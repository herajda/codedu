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
	"syscall"
	"time"
)

// QEMU-backed sandbox configuration. Base image must have an SSH server running
// with the configured user/key whitelisted for passwordless login.
var (
	qemuBinary    = getenvOr("QEMU_BINARY", "qemu-system-x86_64")
	qemuImgBinary = getenvOr("QEMU_IMG_BINARY", "qemu-img")
	qemuBaseImage = getenvOr("QEMU_BASE_IMAGE", "vm/base.img")
	qemuSSHUser   = getenvOr("QEMU_SSH_USER", "runner")
	qemuSSHKey    = getenvOr("QEMU_SSH_KEY", "vm/rsa_key")
	qemuCPUs      = getenvOr("QEMU_CPUS", "2")
	qemuMemory    = getenvOr("QEMU_MEMORY", "1024M")
	qemuAccel     = strings.TrimSpace(os.Getenv("QEMU_ACCEL"))
	qemuEnableKVM = strings.TrimSpace(getenvOr("QEMU_ENABLE_KVM", "1"))
	qemuEnableVNC = strings.TrimSpace(getenvOr("QEMU_ENABLE_VNC", "1"))
	qemuVNCPort   = strings.TrimSpace(getenvOr("QEMU_VNC_PORT", "5900"))
	// Default to 0.0.0.0 so the VNC port can be published from containers; override to 127.0.0.1 for local-only.
	qemuVNCBind    = getenvOr("QEMU_VNC_BIND", "0.0.0.0")
	vmBootTimeout  = getenvDurationOr("QEMU_BOOT_TIMEOUT", 2*time.Minute)
	vmExtraTimeout = 5 * time.Second // small buffer on top of caller timeouts
)

func vmWorkspacePath() string {
	return fmt.Sprintf("/home/%s/code", qemuSSHUser)
}

type vmInstance struct {
	tmpDir      string
	overlayPath string
	sshPort     int
	additional  map[int]int // guest port -> host port
	vncPort     int         // host port for QEMU's VNC server (0 if disabled)
	vncDisplay  int         // QEMU VNC display number (port = 5900 + display)
	cmd         *exec.Cmd
	sshKeyPath  string
	qemuStdout  *bytes.Buffer
	qemuStderr  *bytes.Buffer
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

// acquireVNCPort reserves a TCP port suitable for QEMU's VNC display in the 5900-5999 range.
// QEMU expects "-vnc :<display>" where display = port-5900 (typically 0-99).
func acquireVNCPort() (port int, display int, err error) {
	// Helper that validates range and checks availability on the requested bind host.
	check := func(p int) (int, int, error) {
		if p < 5900 || p > 5999 {
			return 0, 0, fmt.Errorf("VNC port must be in 5900-5999 (got %d)", p)
		}
		ln, lerr := net.Listen("tcp", fmt.Sprintf("%s:%d", qemuVNCBind, p))
		if lerr != nil {
			return 0, 0, lerr
		}
		_ = ln.Close()
		return p, p - 5900, nil
	}

	if qemuVNCPort != "" {
		p, perr := strconv.Atoi(qemuVNCPort)
		if perr != nil || p <= 0 {
			return 0, 0, fmt.Errorf("invalid QEMU_VNC_PORT %q", qemuVNCPort)
		}
		return check(p)
	}

	for p := 5900; p <= 5999; p++ {
		if hp, disp, herr := check(p); herr == nil {
			return hp, disp, nil
		}
	}
	return 0, 0, fmt.Errorf("no free VNC port found in 5900-5999 on %s", qemuVNCBind)
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

	tmp, err := os.MkdirTemp(execRoot, "vm-")
	if err != nil {
		return nil, err
	}
	overlay := filepath.Join(tmp, "overlay.qcow2")
	imgArgs := []string{"create", "-f", "qcow2", "-b", baseImg, "-F", "qcow2", overlay}
	if err := exec.CommandContext(ctx, qemuImgBinary, imgArgs...).Run(); err != nil {
		os.RemoveAll(tmp)
		return nil, fmt.Errorf("qemu-img create: %w", err)
	}
	fmt.Printf("[vm] created overlay %s\n", overlay)

	sshPort, err := acquireEphemeralPort()
	if err != nil {
		os.RemoveAll(tmp)
		return nil, fmt.Errorf("allocate ssh port: %w", err)
	}

	additional := map[int]int{}
	for guest, host := range optionalForwards {
		if host == 0 {
			h, herr := acquireEphemeralPort()
			if herr != nil {
				os.RemoveAll(tmp)
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

	enableVNC := strings.ToLower(qemuEnableVNC) == "1" || strings.ToLower(qemuEnableVNC) == "true" || strings.ToLower(qemuEnableVNC) == "yes"
	vncPort := 0
	vncDisplay := 0
	if enableVNC {
		vncPort, vncDisplay, err = acquireVNCPort()
		if err != nil {
			os.RemoveAll(tmp)
			return nil, fmt.Errorf("allocate vnc port: %w", err)
		}
		fmt.Printf("[vm] VNC binding %s:%d (display=%d)\n", qemuVNCBind, vncPort, vncDisplay)
	}

	args := []string{
		"-m", qemuMemory,
		"-smp", qemuCPUs,
		"-drive", fmt.Sprintf("file=%s,if=virtio,cache=writeback", overlay),
		"-netdev", fmt.Sprintf("user,id=net0,%s", strings.Join(forwards, ",")),
		"-device", "virtio-net-pci,netdev=net0",
		"-serial", "stdio",
		"-monitor", "none",
	}
	if enableVNC {
		args = append(args,
			"-vnc", fmt.Sprintf("%s:%d", qemuVNCBind, vncDisplay),
			"-vga", "std",
		)
	} else {
		args = append(args,
			"-nographic",
			"-display", "none",
		)
	}
	if qemuAccel != "" {
		args = append([]string{"-accel", qemuAccel}, args...)
	}
	if strings.ToLower(qemuEnableKVM) == "1" || strings.ToLower(qemuEnableKVM) == "true" {
		args = append(args, "-enable-kvm")
	}

	vncInfo := "disabled"
	if enableVNC {
		vncInfo = fmt.Sprintf("%s:%d (display=%d)", qemuVNCBind, vncPort, vncDisplay)
	}
	fmt.Printf("[vm] starting qemu with image=%s overlay=%s ssh_port=%d forwards=%v accel=%s vnc=%s\n", baseImg, overlay, sshPort, additional, qemuAccel, vncInfo)
	cmd := exec.CommandContext(ctx, qemuBinary, args...)
	// Avoid cluttering logs; QEMU stays attached to the process for lifecycle control.
	qStdout := &bytes.Buffer{}
	qStderr := &bytes.Buffer{}
	cmd.Stdout = qStdout
	cmd.Stderr = qStderr

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmp)
		return nil, fmt.Errorf("qemu start: %w", err)
	}
	if enableVNC {
		fmt.Printf("[vm] VNC listening on %s:%d (display=%d)\n", qemuVNCBind, vncPort, vncDisplay)
	}

	vm := &vmInstance{
		tmpDir:      tmp,
		overlayPath: overlay,
		sshPort:     sshPort,
		additional:  additional,
		vncPort:     vncPort,
		vncDisplay:  vncDisplay,
		cmd:         cmd,
		sshKeyPath:  sshKey,
		qemuStdout:  qStdout,
		qemuStderr:  qStderr,
	}
	if err := vm.waitForSSH(ctx); err != nil {
		vm.Close()
		return nil, err
	}
	return vm, nil
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
		"-i", v.sshKeyPath,
		"-P", strconv.Itoa(v.sshPort),
		filepath.Clean(dir) + "/.",
		target,
	}
	copyCtx, cancel := context.WithTimeout(ctx, vmBootTimeout)
	defer cancel()
	var scpErr error
	for attempt := 1; attempt <= 5; attempt++ {
		if err := exec.CommandContext(copyCtx, "scp", args...).Run(); err != nil {
			scpErr = err
			fmt.Printf("[vm] scp attempt=%d failed: %v\n", attempt, err)
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
	if v.tmpDir != "" {
		_ = os.RemoveAll(v.tmpDir)
	}
}
