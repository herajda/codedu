package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// QEMU-backed sandbox configuration. Base image must have an SSH server running
// with the configured user/key whitelisted for passwordless login.
var (
	qemuBinary     = getenvOr("QEMU_BINARY", "qemu-system-x86_64")
	qemuImgBinary  = getenvOr("QEMU_IMG_BINARY", "qemu-img")
	qemuBaseImage  = getenvOr("QEMU_BASE_IMAGE", "vm/base.img")
	qemuSSHUser    = getenvOr("QEMU_SSH_USER", "runner")
	qemuSSHKey     = getenvOr("QEMU_SSH_KEY", "vm/rsa_key")
	qemuCPUs       = getenvOr("QEMU_CPUS", "2")
	qemuMemory     = getenvOr("QEMU_MEMORY", "1024M")
	qemuAccel      = strings.TrimSpace(os.Getenv("QEMU_ACCEL"))
	qemuEnableKVM  = strings.TrimSpace(getenvOr("QEMU_ENABLE_KVM", "1"))
	vmBootTimeout  = getenvDurationOr("QEMU_BOOT_TIMEOUT", 2*time.Minute)
	vmExtraTimeout = 5 * time.Second // small buffer on top of caller timeouts
	// Global VM/test execution throttling
	maxParallelVMs = getenvIntOr("MAX_PARALLEL_TESTS", 8)
	vmQueueTimeout = getenvDurationOr("VM_QUEUE_TIMEOUT", 10*time.Minute)
	// CPU isolation via cgroups (best-effort)
	qemuCPUQuotaUS   = strings.TrimSpace(os.Getenv("QEMU_CPU_QUOTA_US"))
	qemuCPUPeriodUS  = strings.TrimSpace(getenvOr("QEMU_CPU_PERIOD_US", "100000"))
	qemuCPUWeight    = strings.TrimSpace(os.Getenv("QEMU_CPU_WEIGHT"))
	qemuCPUSet       = strings.TrimSpace(os.Getenv("QEMU_CPUSET"))
	qemuCgroupRoot   = filepath.Clean(getenvOr("QEMU_CGROUP_ROOT", "/sys/fs/cgroup/codedu-vm"))
	qemuCPUIsolation = strings.TrimSpace(getenvOr("QEMU_CPU_ISOLATION", "1"))
)

func vmWorkspacePath() string {
	return fmt.Sprintf("/home/%s/code", qemuSSHUser)
}

type vmInstance struct {
	tmpDir      string
	overlayPath string
	sshPort     int
	additional  map[int]int // guest port -> host port
	cmd         *exec.Cmd
	sshKeyPath  string
	qemuStdout  *bytes.Buffer
	qemuStderr  *bytes.Buffer
	cgroupPath  string
	slotHeld    bool
}

func getenvDurationOr(k string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(k))
	if raw == "" {
		return def
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return def
}

func getenvIntOr(k string, def int) int {
	raw := strings.TrimSpace(os.Getenv(k))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v == 0 {
		return def
	}
	return v
}

var (
	vmSlotOnce sync.Once
	vmSlotChan chan struct{}
)

func initVMLimiter() {
	vmSlotOnce.Do(func() {
		if maxParallelVMs < 1 {
			maxParallelVMs = 1
		}
		vmSlotChan = make(chan struct{}, maxParallelVMs)
	})
}

func acquireVMSlot(ctx context.Context) error {
	initVMLimiter()
	queueCtx, cancel := context.WithTimeout(context.Background(), vmQueueTimeout)
	defer cancel()
	for {
		select {
		case vmSlotChan <- struct{}{}:
			return nil
		case <-queueCtx.Done():
			return queueCtx.Err()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func releaseVMSlot() {
	initVMLimiter()
	select {
	case <-vmSlotChan:
	default:
	}
}

// acquireEphemeralPort reserves a TCP port on localhost and then closes the
// listener. This avoids racy manual scanning for free ports.
func acquireEphemeralPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// resolveVMPath tries to find the given path relative to common roots (cwd, parents, exe dir).
func resolveVMPath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	clean := filepath.Clean(p)
	// Try as-is (relative to cwd)
	if abs, err := filepath.Abs(clean); err == nil {
		if _, statErr := os.Stat(abs); statErr == nil {
			return abs
		}
	}
	// Walk up parents from cwd
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; dir != "" && dir != string(filepath.Separator); {
			candidate := filepath.Join(dir, clean)
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	// Walk up parents from executable dir
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for dir != "" && dir != string(filepath.Separator) {
			candidate := filepath.Join(dir, clean)
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return clean
}

func cpuIsolationEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(qemuCPUIsolation))
	return !(v == "0" || v == "false" || v == "no")
}

func normalizedCgroupRoot() string {
	root := qemuCgroupRoot
	if !strings.HasPrefix(root, "/sys/fs/cgroup") {
		root = filepath.Join("/sys/fs/cgroup", strings.TrimLeft(root, "/"))
	}
	return root
}

func defaultCPUQuota() string {
	if strings.TrimSpace(qemuCPUQuotaUS) != "" {
		return qemuCPUQuotaUS
	}
	if cpus, err := strconv.Atoi(qemuCPUs); err == nil && cpus > 0 {
		return strconv.Itoa(cpus * 100000)
	}
	return ""
}

func enableV2Controllers(root string, controllers ...string) error {
	ctrlPath := filepath.Join(root, "cgroup.controllers")
	data, err := os.ReadFile(ctrlPath)
	if err != nil {
		return err
	}
	available := map[string]struct{}{}
	for _, c := range strings.Fields(string(data)) {
		available[c] = struct{}{}
	}
	subtree := filepath.Join(root, "cgroup.subtree_control")
	for _, c := range controllers {
		if _, ok := available[c]; !ok {
			return fmt.Errorf("controller %s not available at %s", c, root)
		}
		current, _ := os.ReadFile(subtree)
		if bytes.Contains(current, []byte(c)) {
			continue
		}
		f, ferr := os.OpenFile(subtree, os.O_WRONLY|os.O_APPEND, 0644)
		if ferr != nil {
			return ferr
		}
		if _, ferr = f.WriteString("+" + c); ferr != nil {
			f.Close()
			return ferr
		}
		f.Close()
	}
	return nil
}

func configureV2CPUGroup(root string, pid int) (string, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	if err := enableV2Controllers(root, "cpu"); err != nil {
		return "", err
	}

	group := filepath.Join(root, fmt.Sprintf("vm-%d", time.Now().UnixNano()))
	if err := os.Mkdir(group, 0755); err != nil {
		return "", err
	}

	period := strings.TrimSpace(qemuCPUPeriodUS)
	if period == "" {
		period = "100000"
	}
	quota := strings.TrimSpace(defaultCPUQuota())
	if quota != "" {
		val := fmt.Sprintf("%s %s", quota, period)
		if err := os.WriteFile(filepath.Join(group, "cpu.max"), []byte(val), 0644); err != nil {
			fmt.Printf("[vm] warn: cpu.max apply failed: %v\n", err)
		}
	}
	if strings.TrimSpace(qemuCPUWeight) != "" {
		if err := os.WriteFile(filepath.Join(group, "cpu.weight"), []byte(strings.TrimSpace(qemuCPUWeight)), 0644); err != nil {
			fmt.Printf("[vm] warn: cpu.weight apply failed: %v\n", err)
		}
	}

	// Optional cpuset pinning (best-effort).
	if strings.TrimSpace(qemuCPUSet) != "" {
		if err := enableV2Controllers(root, "cpuset"); err == nil {
			parentMems, _ := os.ReadFile(filepath.Join(root, "cpuset.mems"))
			parentMems = bytes.TrimSpace(parentMems)
			if len(parentMems) == 0 {
				parentMems = []byte("0")
			}
			_ = os.WriteFile(filepath.Join(group, "cpuset.mems"), parentMems, 0644)
			if err := os.WriteFile(filepath.Join(group, "cpuset.cpus"), []byte(qemuCPUSet), 0644); err != nil {
				fmt.Printf("[vm] warn: cpuset apply failed: %v\n", err)
			}
		}
	}

	if err := os.WriteFile(filepath.Join(group, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return "", err
	}
	return group, nil
}

func configureV1CPUGroup(root string, pid int) (string, error) {
	cpuRoot := filepath.Join("/sys/fs/cgroup/cpu", strings.TrimPrefix(root, "/sys/fs/cgroup/"))
	if err := os.MkdirAll(cpuRoot, 0755); err != nil {
		return "", err
	}
	group := filepath.Join(cpuRoot, fmt.Sprintf("vm-%d", time.Now().UnixNano()))
	if err := os.Mkdir(group, 0755); err != nil {
		return "", err
	}

	if quota := strings.TrimSpace(defaultCPUQuota()); quota != "" {
		if quota == "max" {
			quota = "-1"
		}
		if err := os.WriteFile(filepath.Join(group, "cpu.cfs_quota_us"), []byte(quota), 0644); err != nil {
			fmt.Printf("[vm] warn: cpu quota apply failed: %v\n", err)
		}
	}
	if period := strings.TrimSpace(qemuCPUPeriodUS); period != "" {
		if err := os.WriteFile(filepath.Join(group, "cpu.cfs_period_us"), []byte(period), 0644); err != nil {
			fmt.Printf("[vm] warn: cpu period apply failed: %v\n", err)
		}
	}
	if shares := strings.TrimSpace(qemuCPUWeight); shares != "" {
		if err := os.WriteFile(filepath.Join(group, "cpu.shares"), []byte(shares), 0644); err != nil {
			fmt.Printf("[vm] warn: cpu shares apply failed: %v\n", err)
		}
	}

	if err := os.WriteFile(filepath.Join(group, "tasks"), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return "", err
	}
	return group, nil
}

func createCPUGroup(pid int) (string, error) {
	if !cpuIsolationEnabled() {
		return "", nil
	}
	if _, err := os.Stat("/sys/fs/cgroup"); err != nil {
		return "", fmt.Errorf("cgroup filesystem missing: %w", err)
	}
	root := normalizedCgroupRoot()
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return configureV2CPUGroup(root, pid)
	}
	return configureV1CPUGroup(root, pid)
}

// startVM boots a QEMU instance with user networking and SSH port forwarding.
// optionalForwards maps guest ports to specific host ports (e.g., 6080->host).
func startVM(ctx context.Context, optionalForwards map[int]int) (*vmInstance, error) {
	baseImg := resolveVMPath(qemuBaseImage)
	sshKey := resolveVMPath(qemuSSHKey)

	if _, err := os.Stat(baseImg); err != nil {
		return nil, fmt.Errorf("qemu base image not found: %w", err)
	}
	if _, err := os.Stat(sshKey); err != nil {
		return nil, fmt.Errorf("qemu ssh key not found: %w", err)
	}
	fmt.Printf("[vm] resolved paths base=%s key=%s\n", baseImg, sshKey)

	if err := acquireVMSlot(ctx); err != nil {
		return nil, fmt.Errorf("waiting for VM slot: %w", err)
	}
	slotHeld := true

	tmp, err := os.MkdirTemp(execRoot, "vm-")
	if err != nil {
		releaseVMSlot()
		return nil, err
	}
	overlay := filepath.Join(tmp, "overlay.qcow2")
	imgArgs := []string{"create", "-f", "qcow2", "-b", baseImg, "-F", "qcow2", overlay}
	if err := exec.CommandContext(ctx, qemuImgBinary, imgArgs...).Run(); err != nil {
		os.RemoveAll(tmp)
		releaseVMSlot()
		return nil, fmt.Errorf("qemu-img create: %w", err)
	}
	fmt.Printf("[vm] created overlay %s\n", overlay)

	sshPort, err := acquireEphemeralPort()
	if err != nil {
		os.RemoveAll(tmp)
		releaseVMSlot()
		return nil, fmt.Errorf("allocate ssh port: %w", err)
	}

	additional := map[int]int{}
	for guest, host := range optionalForwards {
		if host == 0 {
			h, herr := acquireEphemeralPort()
			if herr != nil {
				os.RemoveAll(tmp)
				releaseVMSlot()
				return nil, fmt.Errorf("allocate host port for guest %d: %w", guest, herr)
			}
			host = h
		}
		additional[guest] = host
	}

	forwards := []string{fmt.Sprintf("hostfwd=tcp::%d-:22", sshPort)}
	for guest, host := range additional {
		forwards = append(forwards, fmt.Sprintf("hostfwd=tcp::%d-:%d", host, guest))
	}

	// Check for "vm.state" file
	vmState := filepath.Join(filepath.Dir(baseImg), "vm.state")
	hasSnapshot := false
	if _, err := os.Stat(vmState); err == nil {
		hasSnapshot = true
	}

	args := []string{
		"-M", "pc-i440fx-7.2",
		"-m", qemuMemory,
		"-smp", qemuCPUs,
		"-drive", fmt.Sprintf("file=%s,if=virtio,cache=writeback", overlay),
		"-netdev", fmt.Sprintf("user,id=net0,%s", strings.Join(forwards, ",")),
		"-device", "virtio-net-pci,netdev=net0,romfile=",
		"-serial", "stdio",
		"-monitor", "none",
		"-nographic",
		"-display", "none",
	}
	if hasSnapshot {
		args = append(args, "-incoming", fmt.Sprintf("exec:cat %s", vmState))
		fmt.Println("[vm] using snapshot state from vm.state")
	}

	if qemuAccel != "" {
		args = append([]string{"-accel", qemuAccel}, args...)
	}
	if strings.ToLower(qemuEnableKVM) == "1" || strings.ToLower(qemuEnableKVM) == "true" {
		args = append(args, "-enable-kvm")
	}

	fmt.Printf("[vm] starting qemu with image=%s overlay=%s ssh_port=%d forwards=%v accel=%s snapshot=%v\n", baseImg, overlay, sshPort, additional, qemuAccel, hasSnapshot)
	cmd := exec.CommandContext(ctx, qemuBinary, args...)
	// Avoid cluttering logs; QEMU stays attached to the process for lifecycle control.
	qStdout := &bytes.Buffer{}
	qStderr := &bytes.Buffer{}
	cmd.Stdout = qStdout
	cmd.Stderr = qStderr

	vm := &vmInstance{
		tmpDir:      tmp,
		overlayPath: overlay,
		sshPort:     sshPort,
		additional:  additional,
		cmd:         cmd,
		sshKeyPath:  sshKey,
		qemuStdout:  qStdout,
		qemuStderr:  qStderr,
		slotHeld:    slotHeld,
	}

	if err := cmd.Start(); err != nil {
		vm.Close()
		return nil, fmt.Errorf("qemu start: %w", err)
	}

	if err := vm.applyCPUConstraints(); err != nil {
		fmt.Printf("[vm] warn: CPU limits not applied: %v\n", err)
	}
	if err := vm.waitForSSH(ctx); err != nil {
		vm.Close()
		return nil, err
	}
	return vm, nil
}

// PrepareSnapshot boots the base image, waits for SSH, and saves a "vm.state" file.
func PrepareSnapshot() error {
	baseImg := resolveVMPath(qemuBaseImage)
	sshKey := resolveVMPath(qemuSSHKey)

	if _, err := os.Stat(baseImg); err != nil {
		return fmt.Errorf("qemu base image not found: %w", err)
	}
	if _, err := os.Stat(sshKey); err != nil {
		return fmt.Errorf("qemu ssh key not found: %w", err)
	}

	// We need a monitor socket to send migrate command
	monitorSock := filepath.Join(os.TempDir(), fmt.Sprintf("qemu-monitor-%d.sock", time.Now().UnixNano()))
	sshPort, err := acquireEphemeralPort()
	if err != nil {
		return fmt.Errorf("allocate ssh port: %w", err)
	}

	// Important: Use exact same device configuration as startVM (except hostfwd port)
	args := []string{
		"-M", "pc-i440fx-7.2",
		"-m", qemuMemory,
		"-smp", qemuCPUs,
		"-drive", fmt.Sprintf("file=%s,if=virtio,cache=writeback", baseImg), // Direct RW access to base
		"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", sshPort),
		"-device", "virtio-net-pci,netdev=net0,romfile=",
		"-serial", "stdio",
		"-monitor", fmt.Sprintf("unix:%s,server,nowait", monitorSock),
		"-nographic",
		"-display", "none",
	}
	if qemuAccel != "" {
		args = append([]string{"-accel", qemuAccel}, args...)
	}
	if strings.ToLower(qemuEnableKVM) == "1" || strings.ToLower(qemuEnableKVM) == "true" {
		args = append(args, "-enable-kvm")
	}

	fmt.Printf("[snapshot] starting qemu for snapshotting image=%s ssh_port=%d\n", baseImg, sshPort)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, qemuBinary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("qemu start: %w", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	// Wait for SSH to be ready
	vm := &vmInstance{
		sshPort:    sshPort,
		sshKeyPath: sshKey,
		qemuStdout: bytes.NewBuffer(nil),
		qemuStderr: bytes.NewBuffer(nil),
	}
	fmt.Println("[snapshot] waiting for SSH...")
	if err := vm.waitForSSH(ctx); err != nil {
		return fmt.Errorf("ssh wait failed: %w", err)
	}
	fmt.Println("[snapshot] SSH ready, saving snapshot...")

	// Connect to monitor and save snapshot
	conn, err := net.Dial("unix", monitorSock)
	if err != nil {
		return fmt.Errorf("monitor dial: %w", err)
	}
	defer conn.Close()

	// Read banner
	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	fmt.Printf("[snapshot] monitor banner: %s\n", string(buf[:n]))

	// Migrate to file
	vmState := filepath.Join(filepath.Dir(baseImg), "vm.state")
	fmt.Printf("[snapshot] migrating to %s\n", vmState)
	cmdStr := fmt.Sprintf("migrate \"exec:cat > %s\"\n", vmState)
	fmt.Fprintf(conn, cmdStr)

	// Poll for completion
	for {
		fmt.Fprintf(conn, "info migrate\n")
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("[snapshot] read error: %v\n", err)
			break
		}
		out := string(buf[:n])
		if strings.Contains(out, "completed") {
			fmt.Println("[snapshot] migration completed")
			break
		}
		if strings.Contains(out, "failed") {
			return fmt.Errorf("migration failed: %s", out)
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Println("[snapshot] sending quit...")
	fmt.Fprintf(conn, "quit\n")

	// Wait for process to exit
	if err := cmd.Wait(); err != nil {
		fmt.Printf("[snapshot] qemu exit: %v\n", err)
	}
	fmt.Println("[snapshot] snapshot state created successfully")
	return nil
}

func (v *vmInstance) applyCPUConstraints() error {
	if v.cmd == nil || v.cmd.Process == nil {
		return nil
	}
	group, err := createCPUGroup(v.cmd.Process.Pid)
	if err != nil {
		return err
	}
	if group != "" {
		v.cgroupPath = group
		fmt.Printf("[vm] CPU group applied at %s\n", group)
	}
	return nil
}

func (v *vmInstance) cleanupCPUGroup() {
	if v.cgroupPath == "" {
		return
	}
	if err := os.Remove(v.cgroupPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("[vm] warn: cleanup cgroup %s failed: %v\n", v.cgroupPath, err)
	}
	v.cgroupPath = ""
}

func (v *vmInstance) waitForSSH(ctx context.Context) error {
	deadline := time.Now().Add(vmBootTimeout)
	attempt := 0
	for time.Now().Before(deadline) {
		attempt++
		// Single probe timeout (per attempt) — allow a bit more time on slow boots.
		pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

		// If QEMU has already exited, surface that immediately with logs.
		if v.cmd != nil && v.cmd.Process != nil {
			if err := v.cmd.Process.Signal(syscall.Signal(0)); err != nil {
				fmt.Printf("[vm] qemu process appears exited (signal 0 failed: %v); dumping output\n", err)
				stdoutTail := strings.TrimSpace(v.qemuStdout.String())
				stderrTail := strings.TrimSpace(v.qemuStderr.String())
				if stdoutTail != "" {
					fmt.Printf("[vm] qemu stdout:\n%s\n", stdoutTail)
				}
				if stderrTail != "" {
					fmt.Printf("[vm] qemu stderr:\n%s\n", stderrTail)
				}
				cancel()
				return fmt.Errorf("qemu exited before ssh was ready: %w", err)
			}
		}

		cmd := v.sshCommand(pingCtx, "echo ok")
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		err := cmd.Run()
		if err == nil {
			cancel()
			return nil
		}
		fmt.Printf("[vm] ssh probe attempt=%d failed: err=%v stdout=%q stderr=%q\n", attempt, err, strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()))
		cancel()
		if ctx.Err() != nil {
			fmt.Printf("[vm] ssh probe context error: %v\n", ctx.Err())
			return ctx.Err()
		}
		time.Sleep(500 * time.Millisecond)
	}
	stdoutTail := strings.TrimSpace(v.qemuStdout.String())
	stderrTail := strings.TrimSpace(v.qemuStderr.String())
	if stdoutTail != "" {
		//fmt.Printf("[vm] qemu stdout:\n%s\n", stdoutTail)
	}
	if stderrTail != "" {
		fmt.Printf("[vm] qemu stderr:\n%s\n", stderrTail)
	}
	return fmt.Errorf("vm ssh not ready within %s", vmBootTimeout)
}

func (v *vmInstance) sshArgs() []string {
	return []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		"-o", "LogLevel=ERROR",
		"-i", v.sshKeyPath,
		"-p", strconv.Itoa(v.sshPort),
		fmt.Sprintf("%s@127.0.0.1", qemuSSHUser),
	}
}

func (v *vmInstance) sshCommand(ctx context.Context, remoteCmd string) *exec.Cmd {
	args := append(v.sshArgs(), "bash", "-lc", remoteCmd)
	return exec.CommandContext(ctx, "ssh", args...)
}

// prepareWorkspaceDir ensures the workspace path exists (removing any leftovers)
// and returns stdout/stderr alongside the error for debugging.
func (v *vmInstance) prepareWorkspaceDir(ctx context.Context, dest string) (string, string, error) {
	// Guard against empty/unsafe destinations and quote the path.
	clean := fmt.Sprintf(`
dest=%q
if [ -z "$dest" ] || [ "$dest" = "/" ]; then
  echo "refusing to clean unsafe dest: '$dest'" >&2
  exit 1
fi
rm -rf -- "$dest" && mkdir -p -- "$dest"
`, dest)
	cmd := v.sshCommand(ctx, clean)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	return strings.TrimSpace(stdoutBuf.String()), strings.TrimSpace(stderrBuf.String()), err
}

// syncWorkspace copies the given directory into the VM under /home/<user>/code.
func (v *vmInstance) syncWorkspace(ctx context.Context, dir string) (string, error) {
	dest := vmWorkspacePath()
	fmt.Printf("[vm] syncing workspace %s -> %s\n", dir, dest)
	// Some images may have restrictive permissions under /home; fall back to /tmp if prep fails.
	candidates := []string{dest, fmt.Sprintf("/tmp/code-%d", time.Now().UnixNano())}
	var prepStdout, prepStderr string
	var prepErr error
	for _, candidate := range candidates {
		dest = candidate
		for attempt := 1; attempt <= 5; attempt++ {
			prepStdout, prepStderr, prepErr = v.prepareWorkspaceDir(ctx, dest)
			if prepErr != nil {
				fmt.Printf("[vm] prepare workspace attempt=%d path=%s failed: %v stdout=%q stderr=%q\n", attempt, dest, prepErr, prepStdout, prepStderr)
				time.Sleep(1 * time.Second)
				continue
			}
			prepErr = nil
			break
		}
		if prepErr == nil {
			break
		}
	}
	if prepErr != nil {
		stdoutTail := strings.TrimSpace(v.qemuStdout.String())
		stderrTail := strings.TrimSpace(v.qemuStderr.String())
		if stdoutTail != "" {
			fmt.Printf("[vm] qemu stdout (prep failure):\n%s\n", stdoutTail)
		}
		if stderrTail != "" {
			fmt.Printf("[vm] qemu stderr (prep failure):\n%s\n", stderrTail)
		}
		return "", fmt.Errorf("prepare workspace: %w (stdout=%q stderr=%q)", prepErr, prepStdout, prepStderr)
	}

	target := fmt.Sprintf("%s@127.0.0.1:%s", qemuSSHUser, dest)
	args := []string{
		"-q",
		"-r",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", v.sshKeyPath,
		"-P", strconv.Itoa(v.sshPort),
		filepath.Clean(dir) + "/.",
		target,
	}
	copyCtx, cancel := context.WithTimeout(ctx, vmBootTimeout)
	defer cancel()
	var scpErr error
	for attempt := 1; attempt <= 5; attempt++ {
		cmd := exec.CommandContext(copyCtx, "scp", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			scpErr = err
			fmt.Printf("[vm] scp attempt=%d failed: %v output=%q\n", attempt, err, string(out))
			time.Sleep(1 * time.Second)
			continue
		}
		scpErr = nil
		break
	}
	if scpErr != nil {
		stdoutTail := strings.TrimSpace(v.qemuStdout.String())
		stderrTail := strings.TrimSpace(v.qemuStderr.String())
		if stdoutTail != "" {
			fmt.Printf("[vm] qemu stdout (scp failure):\n%s\n", stdoutTail)
		}
		if stderrTail != "" {
			fmt.Printf("[vm] qemu stderr (scp failure):\n%s\n", stderrTail)
		}
		return "", fmt.Errorf("copy workspace: %w", scpErr)
	}
	fmt.Printf("[vm] workspace synced to %s\n", dest)
	return dest, nil
}

// startVMWithWorkspace boots a VM and copies the workspace into it.
func startVMWithWorkspace(ctx context.Context, dir string, forwards map[int]int) (*vmInstance, string, error) {
	vm, err := startVM(ctx, forwards)
	if err != nil {
		return nil, "", err
	}
	remoteDir, err := vm.syncWorkspace(ctx, dir)
	if err != nil {
		vm.Close()
		return nil, "", err
	}
	return vm, remoteDir, nil
}

// runCommand executes a command inside the VM in the provided workdir.
func (v *vmInstance) runCommand(ctx context.Context, workdir, script string, stdin *strings.Reader) (string, string, int, error) {
	fmt.Printf("[vm] runCommand workdir=%s script=%s\n", workdir, script)
	cmd := v.sshCommand(ctx, fmt.Sprintf("cd %s && %s", workdir, script))
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode, err
}

// startInteractive starts a long-running command and returns pipes for streaming IO.
func (v *vmInstance) startInteractive(ctx context.Context, workdir, script string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	fmt.Printf("[vm] startInteractive workdir=%s script=%s\n", workdir, script)
	cmd := v.sshCommand(ctx, fmt.Sprintf("cd %s && %s", workdir, script))
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, nil, err
	}
	return cmd, stdinPipe, stdoutPipe, stderrPipe, nil
}

func (v *vmInstance) Close() {
	if v.cmd != nil && v.cmd.Process != nil {
		_ = v.cmd.Process.Kill()
		_, _ = v.cmd.Process.Wait()
	}
	v.cleanupCPUGroup()
	if v.tmpDir != "" {
		_ = os.RemoveAll(v.tmpDir)
	}
	if v.slotHeld {
		releaseVMSlot()
		v.slotHeld = false
	}
}
