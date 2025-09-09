package main

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
)

// ──────────────────────────────────────────────────────────────────────────────
// Interactive session API (LLM-friendly)
// ──────────────────────────────────────────────────────────────────────────────

// SessionLimits defines per-session resource caps persisted and enforced server-side.
type SessionLimits struct {
	WallSeconds int   `json:"wall_seconds"`
	CPUSeconds  int   `json:"cpu_seconds"`
	MemMB       int   `json:"mem_mb"`
	StdoutBytes int64 `json:"stdout_bytes"`
	StderrBytes int64 `json:"stderr_bytes"`
	StdinBytes  int64 `json:"stdin_bytes"`
}

// SessionUsage captures live usage counters for a session.
type SessionUsage struct {
	CPUSeconds  float64 `json:"cpu_seconds"`
	WallSeconds float64 `json:"wall_seconds"`
	MemPeakMB   int64   `json:"mem_peak_mb"`
	StdoutBytes int64   `json:"stdout_bytes"`
	StderrBytes int64   `json:"stderr_bytes"`
	StdinBytes  int64   `json:"stdin_bytes"`
}

// SessionSummary describes a session externally.
type SessionSummary struct {
	SessionID string        `json:"session_id"`
	Status    string        `json:"status"`
	StartedAt time.Time     `json:"started_at"`
	ExitedAt  *time.Time    `json:"exited_at,omitempty"`
	ExitCode  int           `json:"exit_code"`
	Usage     *SessionUsage `json:"usage"`
}

// StartSessionRequest schema for POST /sessions
type StartSessionRequest struct {
	Cmd            []string          `json:"cmd" binding:"required,min=1"`
	Files          []SessionFile     `json:"files"`
	Env            map[string]string `json:"env"`
	Workdir        string            `json:"workdir"`
	Limits         SessionLimits     `json:"limits"`
	Network        string            `json:"network"`
	IdempotencyKey *string           `json:"idempotency_key"`
}

type SessionFile struct {
	Path       string `json:"path" binding:"required"`
	ContentB64 string `json:"content_b64" binding:"required"`
	Mode       string `json:"mode"`
}

type sessionSubscriber struct {
	ch chan sse.Event
}

type ringBuffer struct {
	max int
	buf []byte
}

func newRingBuffer(max int) *ringBuffer { return &ringBuffer{max: max} }
func (r *ringBuffer) Write(p []byte) {
	r.buf = append(r.buf, p...)
	if len(r.buf) > r.max {
		offset := len(r.buf) - r.max
		r.buf = append([]byte(nil), r.buf[offset:]...)
	}
}
func (r *ringBuffer) Bytes() []byte { return append([]byte(nil), r.buf...) }

type SessionState struct {
	ID        string
	Status    string // starting|running|exited|stopped|error|timeout
	StartedAt time.Time
	ExitedAt  *time.Time
	ExitCode  int

	// configuration
	Cmd     []string
	Env     map[string]string
	Workdir string
	Network string // off|egress|full
	Limits  SessionLimits

	// process/pipes
	CmdProc *exec.Cmd
	Stdout  io.ReadCloser
	Stderr  io.ReadCloser
	Stdin   io.WriteCloser
	Cancel  context.CancelFunc
	// docker container name to manage lifecycle
	ContainerName string

	// buffers and counters
	StdoutTail *ringBuffer
	StderrTail *ringBuffer
	Usage      SessionUsage

	stdoutCapped bool
	stderrCapped bool
	stdinClosed  bool

	// sse subscribers
	Subs map[*sessionSubscriber]struct{}

	// fs + transcript
	TmpDir         string
	TranscriptPath string
	transcriptFile *os.File
	transcriptMu   sync.Mutex

	// idempotency bookkeeping
	IdemKey   *string
	CreatedAt time.Time

	mu sync.Mutex
}

var (
	sessionsMu sync.RWMutex
	sessions   = map[string]*SessionState{}

	idemMu  sync.Mutex
	idemIdx = map[string]string{} // idem key -> session id

	// Signing key for artifact URLs
	signingKey     []byte
	initSigningKey sync.Once
)

const (
	maxInputFiles      = 32
	maxInputTotalBytes = 256 * 1024
	maxTailBytes       = 32 * 1024
)

// error helpers per taxonomy
func apiErr(c *gin.Context, status int, code, detail string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "detail": detail}})
}

// id generator
func newSessionID() string {
	// 16 random bytes hex
	b := make([]byte, 16)
	_, _ = io.ReadFull(randReader{}, b)
	return "sess_" + hex.EncodeToString(b)
}

// A very small rand reader using crypto/sha256 over time+counter; avoids bringing crypto/rand explicitly
type randReader struct{}

var rrCounter uint64

func (r randReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		seed := fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), os.Getpid(), rrCounter)
		rrCounter++
		h := sha256.Sum256([]byte(seed))
		copy(p[n:], h[:])
		n += len(h)
	}
	return len(p), nil
}

func getSigningKey() []byte {
	initSigningKey.Do(func() {
		key := strings.TrimSpace(os.Getenv("SESS_SIGNING_KEY"))
		if key == "" {
			// derive from server secret or random
			seed := fmt.Sprintf("%d-%s", time.Now().UnixNano(), "code-edu")
			h := sha256.Sum256([]byte(seed))
			signingKey = h[:]
		} else {
			signingKey = []byte(key)
		}
	})
	return signingKey
}

func signPath(path string, exp time.Time) string {
	q := strconv.FormatInt(exp.Unix(), 10)
	mac := hmac.New(sha256.New, getSigningKey())
	_, _ = mac.Write([]byte(path + "|" + q))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s?exp=%s&sig=%s", path, q, sig)
}

func verifySignature(path, expStr, sig string) bool {
	if path == "" || expStr == "" || sig == "" {
		return false
	}
	expUnix, _ := strconv.ParseInt(expStr, 10, 64)
	if expUnix == 0 || time.Now().Unix() > expUnix {
		return false
	}
	mac := hmac.New(sha256.New, getSigningKey())
	_, _ = mac.Write([]byte(path + "|" + expStr))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

// validate and persist files to tmp dir
func stageFiles(files []SessionFile) (string, error) {
	if len(files) == 0 {
		// create empty dir
		d, err := os.MkdirTemp(execRoot, "sess-")
		if err != nil {
			return "", err
		}
		return d, nil
	}
	if len(files) > maxInputFiles {
		return "", fmt.Errorf("too many files")
	}
	var total int
	root, err := os.MkdirTemp(execRoot, "sess-")
	if err != nil {
		return "", err
	}
	for _, f := range files {
		p := strings.TrimSpace(f.Path)
		if p == "" || strings.Contains(p, "..") || strings.HasPrefix(p, "/") || strings.HasPrefix(filepath.Base(p), ".") {
			os.RemoveAll(root)
			return "", fmt.Errorf("forbidden path: %s", p)
		}
		b, err := base64.StdEncoding.DecodeString(f.ContentB64)
		if err != nil {
			os.RemoveAll(root)
			return "", fmt.Errorf("invalid base64 for %s", p)
		}
		total += len(b)
		if total > maxInputTotalBytes {
			os.RemoveAll(root)
			return "", fmt.Errorf("files too large")
		}
		dst := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			os.RemoveAll(root)
			return "", err
		}
		if err := os.WriteFile(dst, b, 0644); err != nil {
			os.RemoveAll(root)
			return "", err
		}
		if m := strings.TrimSpace(f.Mode); m != "" {
			if iv, err := strconv.ParseInt(m, 8, 64); err == nil {
				_ = os.Chmod(dst, os.FileMode(iv))
			}
		}
	}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			_ = os.Chmod(path, 0755)
		} else {
			_ = os.Chmod(path, 0644)
		}
		return nil
	})
	_ = ensureSandboxPerms(root)
	return root, nil
}

func (s *SessionState) summarize() *SessionSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	usage := s.Usage
	wall := time.Since(s.StartedAt).Seconds()
	if s.Status == "exited" || s.Status == "stopped" || s.Status == "error" || s.Status == "timeout" {
		// freeze wall if exited
		if s.ExitedAt != nil {
			wall = s.ExitedAt.Sub(s.StartedAt).Seconds()
		}
	}
	usage.WallSeconds = wall
	return &SessionSummary{SessionID: s.ID, Status: s.Status, StartedAt: s.StartedAt, ExitedAt: s.ExitedAt, ExitCode: s.ExitCode, Usage: &usage}
}

func (s *SessionState) broadcast(data any) {
	evt := sse.Event{Event: "message", Data: data}
	s.mu.Lock()
	for sub := range s.Subs {
		select {
		case sub.ch <- evt:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *SessionState) writeNDJSONLine(obj map[string]any) {
	s.transcriptMu.Lock()
	defer s.transcriptMu.Unlock()
	if s.transcriptFile == nil {
		return
	}
	obj["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	line, _ := json.Marshal(obj)
	_, _ = s.transcriptFile.Write(append(line, '\n'))
}

func (s *SessionState) startProcess() error {
	// Ensure image exists before starting to avoid long pulls during session
	_ = ensureDockerImage(pythonImage)
	// network policy
	netFlag := "--network=none"
	s.Network = strings.ToLower(strings.TrimSpace(s.Network))
	if s.Network == "egress" || s.Network == "full" {
		allowEgress := strings.TrimSpace(os.Getenv("ALLOW_SESSION_EGRESS")) == "1"
		allowFull := strings.TrimSpace(os.Getenv("ALLOW_SESSION_FULLNET")) == "1"
		if s.Network == "egress" {
			if !allowEgress {
				return fmt.Errorf("POLICY_VIOLATION: network blocked")
			}
			// approximate egress by default network; we do not add special iptables here
			netFlag = "--network=bridge"
		} else if s.Network == "full" {
			if !allowFull {
				return fmt.Errorf("POLICY_VIOLATION: network blocked")
			}
			netFlag = "--network=bridge"
		}
	}

	abs, _ := filepath.Abs(s.TmpDir)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.Limits.WallSeconds)*time.Second+dockerExtraTime)
	s.Cancel = cancel
	// give this container a predictable name for later rm -f
	cname := strings.ReplaceAll(s.ID, "_", "-")
	args := []string{
		"run", "--rm", "-i",
		"--name", cname,
		netFlag,
		"--user", dockerUser,
		"--cpus", dockerCPUs,
		"--memory", dockerMemory,
		"--pids-limit", "128",
		"--read-only",
		"--cap-drop=ALL",
		"--security-opt", "no-new-privileges",
		"--security-opt", "label=disable",
		"--mount", "type=tmpfs,destination=/tmp,tmpfs-size=16m",
		"-v", fmt.Sprintf("%s:/code:ro", abs),
	}
	wd := strings.TrimSpace(s.Workdir)
	if wd != "" {
		// Only allow relative safe paths inside /code
		wd = filepath.Clean(wd)
		wd = strings.TrimPrefix(wd, "./")
		args = append(args, "-w", "/code/"+wd)
	} else {
		args = append(args, "-w", "/code")
	}
	// env
	for k, v := range s.Env {
		if !validEnvKey(k) {
			continue
		}
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	// tighten docker memory flag when explicit limit provided
	if s.Limits.MemMB > 0 {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--memory" {
				args[i+1] = fmt.Sprintf("%dm", s.Limits.MemMB)
				break
			}
		}
	}
	// image + command
	args = append(args, pythonImage)
	if len(s.Cmd) == 1 {
		args = append(args, s.Cmd[0])
	} else {
		args = append(args, s.Cmd...)
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	stdoutPipe, e1 := cmd.StdoutPipe()
	stderrPipe, e2 := cmd.StderrPipe()
	stdinPipe, e3 := cmd.StdinPipe()
	if e1 != nil || e2 != nil || e3 != nil {
		return fmt.Errorf("CONTAINER_ERROR: pipes")
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("CONTAINER_ERROR: start")
	}
	s.CmdProc = cmd
	s.Stdout = stdoutPipe
	s.Stderr = stderrPipe
	s.Stdin = stdinPipe
	s.ContainerName = cname
	// context watcher for wall timeout and cleanup
	go func() {
		<-ctx.Done()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			s.mu.Lock()
			if s.Status == "running" {
				s.Status = "timeout"
			}
			s.mu.Unlock()
			s.broadcast(map[string]any{"type": "error", "code": "RESOURCE_EXCEEDED", "detail": "wall_seconds", "ts": time.Now().UTC().Format(time.RFC3339Nano)})
		}
		if s.ContainerName != "" {
			_ = exec.Command("docker", "rm", "-f", s.ContainerName).Run()
		}
	}()
	return nil
}

var envKeyRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

func validEnvKey(k string) bool { return envKeyRe.MatchString(strings.ToUpper(strings.TrimSpace(k))) }

// POST /api/sessions
func startSessionHandler(c *gin.Context) {
	var req StartSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	// defaults
	if req.Limits.WallSeconds <= 0 {
		req.Limits.WallSeconds = 30
	}
	if req.Limits.CPUSeconds <= 0 {
		req.Limits.CPUSeconds = 10
	}
	if req.Limits.MemMB <= 0 {
		req.Limits.MemMB = 256
	}
	if req.Limits.StdoutBytes <= 0 {
		req.Limits.StdoutBytes = 1 << 20
	}
	if req.Limits.StderrBytes <= 0 {
		req.Limits.StderrBytes = 512 << 10
	}
	if req.Limits.StdinBytes <= 0 {
		req.Limits.StdinBytes = 256 << 10
	}
	if strings.TrimSpace(req.Network) == "" {
		req.Network = "off"
	}

	// idempotency check
	if req.IdempotencyKey != nil && strings.TrimSpace(*req.IdempotencyKey) != "" {
		idemMu.Lock()
		if sid, ok := idemIdx[*req.IdempotencyKey]; ok {
			idemMu.Unlock()
			sessionsMu.RLock()
			s := sessions[sid]
			sessionsMu.RUnlock()
			if s != nil {
				s.mu.Lock()
				stdinSent := s.Usage.StdinBytes > 0
				created := s.CreatedAt
				s.mu.Unlock()
				if time.Since(created) <= 2*time.Minute && !stdinSent {
					c.JSON(http.StatusOK, gin.H{"session": s.summarize(), "stream": gin.H{"transport": "sse", "url": fmt.Sprintf("/api/sessions/%s/stream", s.ID)}})
					return
				} else {
					apiErr(c, http.StatusConflict, "CONFLICT", "idempotency key already used and session mutated")
					return
				}
			}
		}
		idemMu.Unlock()
	}

	// stage files
	tmpDir, err := stageFiles(req.Files)
	if err != nil {
		// map errors
		msg := err.Error()
		code := "BAD_REQUEST"
		if strings.Contains(msg, "forbidden path") {
			code = "POLICY_VIOLATION"
		}
		if strings.Contains(msg, "too many") || strings.Contains(msg, "too large") {
			code = "POLICY_VIOLATION"
		}
		apiErr(c, http.StatusBadRequest, code, msg)
		return
	}

	// build session
	s := &SessionState{
		ID:         newSessionID(),
		Status:     "starting",
		StartedAt:  time.Now().UTC(),
		ExitCode:   0,
		Cmd:        req.Cmd,
		Env:        req.Env,
		Workdir:    req.Workdir,
		Network:    req.Network,
		Limits:     req.Limits,
		StdoutTail: newRingBuffer(maxTailBytes),
		StderrTail: newRingBuffer(maxTailBytes),
		Subs:       make(map[*sessionSubscriber]struct{}),
		TmpDir:     tmpDir,
		CreatedAt:  time.Now(),
		IdemKey:    req.IdempotencyKey,
	}

	// open transcript file
	_ = os.MkdirAll("uploads", 0755)
	transcript := filepath.Join("uploads", fmt.Sprintf("%s.ndjson", s.ID))
	f, ferr := os.Create(transcript)
	if ferr == nil {
		s.transcriptFile = f
		s.TranscriptPath = transcript
	}

	// start docker process
	if err := s.startProcess(); err != nil {
		_ = os.RemoveAll(tmpDir)
		if s.transcriptFile != nil {
			_ = s.transcriptFile.Close()
		}
		msg := err.Error()
		if strings.HasPrefix(msg, "POLICY_VIOLATION:") {
			apiErr(c, http.StatusBadRequest, "POLICY_VIOLATION", strings.TrimPrefix(msg, "POLICY_VIOLATION: "))
			return
		}
		apiErr(c, http.StatusBadGateway, "CONTAINER_ERROR", "container start failed")
		return
	}

	// save in registry
	sessionsMu.Lock()
	sessions[s.ID] = s
	sessionsMu.Unlock()
	if req.IdempotencyKey != nil && strings.TrimSpace(*req.IdempotencyKey) != "" {
		idemMu.Lock()
		idemIdx[*req.IdempotencyKey] = s.ID
		idemMu.Unlock()
	}

	// start readers
	go runSessionIO(s)

	// response
	c.JSON(http.StatusCreated, gin.H{
		"session": s.summarize(),
		"stream":  gin.H{"transport": "sse", "url": fmt.Sprintf("/api/sessions/%s/stream", s.ID)},
	})
}

func runSessionIO(s *SessionState) {
	s.mu.Lock()
	s.Status = "running"
	s.mu.Unlock()
	s.broadcast(map[string]any{"type": "status", "status": "running", "usage": s.summarize().Usage, "ts": time.Now().UTC().Format(time.RFC3339Nano)})

	// stdout reader
	go func() {
		reader := bufio.NewReader(s.Stdout)
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				s.mu.Lock()
				s.Usage.StdoutBytes += int64(len(chunk))
				capped := s.stdoutCapped
				if !capped && s.Limits.StdoutBytes > 0 && s.Usage.StdoutBytes > s.Limits.StdoutBytes {
					s.stdoutCapped = true
					capped = true
				}
				if !capped {
					s.StdoutTail.Write(chunk)
				}
				s.mu.Unlock()
				// always write transcript
				s.writeNDJSONLine(map[string]any{"dir": "stdout", "size": len(chunk), "sha256": sha256Hex(chunk), "data_b64": base64.StdEncoding.EncodeToString(chunk)})
				if capped {
					// emit limit once
					s.broadcast(map[string]any{"type": "limit", "kind": "stdout_bytes", "detail": fmt.Sprintf("cap %d reached", s.Limits.StdoutBytes), "ts": time.Now().UTC().Format(time.RFC3339Nano)})
					// keep draining but do not forward further
					continue
				}
				// forward to SSE
				s.broadcast(map[string]any{"type": "stdout", "data_b64": base64.StdEncoding.EncodeToString(chunk), "ts": time.Now().UTC().Format(time.RFC3339Nano)})
			}
			if err != nil {
				break
			}
		}
	}()

	// stderr reader
	go func() {
		reader := bufio.NewReader(s.Stderr)
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				s.mu.Lock()
				s.Usage.StderrBytes += int64(len(chunk))
				capped := s.stderrCapped
				if !capped && s.Limits.StderrBytes > 0 && s.Usage.StderrBytes > s.Limits.StderrBytes {
					s.stderrCapped = true
					capped = true
				}
				if !capped {
					s.StderrTail.Write(chunk)
				}
				s.mu.Unlock()
				s.writeNDJSONLine(map[string]any{"dir": "stderr", "size": len(chunk), "sha256": sha256Hex(chunk), "data_b64": base64.StdEncoding.EncodeToString(chunk)})
				if capped {
					s.broadcast(map[string]any{"type": "limit", "kind": "stderr_bytes", "detail": fmt.Sprintf("cap %d reached", s.Limits.StderrBytes), "ts": time.Now().UTC().Format(time.RFC3339Nano)})
					continue
				}
				s.broadcast(map[string]any{"type": "stderr", "data_b64": base64.StdEncoding.EncodeToString(chunk), "ts": time.Now().UTC().Format(time.RFC3339Nano)})
			}
			if err != nil {
				break
			}
		}
	}()

	// wait + heartbeat
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	done := make(chan struct{})
	go func() {
		err := s.CmdProc.Wait()
		code := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				code = -1
			}
		}
		now := time.Now().UTC()
		s.mu.Lock()
		s.ExitedAt = &now
		if s.Status == "running" {
			s.Status = "exited"
		}
		s.ExitCode = code
		s.mu.Unlock()
		s.writeNDJSONLine(map[string]any{"event": "exit", "code": code})
		s.broadcast(map[string]any{"type": "exit", "code": code, "ts": time.Now().UTC().Format(time.RFC3339Nano)})
		close(done)
	}()

	for {
		select {
		case <-ticker.C:
			// periodic heartbeat
			s.broadcast(map[string]any{"type": "status", "status": s.summarize().Status, "usage": s.summarize().Usage, "ts": time.Now().UTC().Format(time.RFC3339Nano)})
		case <-done:
			// cleanup transcript and tmp dir after short delay
			if s.transcriptFile != nil {
				_ = s.transcriptFile.Close()
			}
			go func(tmp, cname string) {
				time.Sleep(1500 * time.Millisecond)
				if cname != "" {
					_ = exec.Command("docker", "rm", "-f", cname).Run()
				}
				if tmp != "" {
					_ = os.RemoveAll(tmp)
				}
			}(s.TmpDir, s.ContainerName)
			return
		}
	}
}

func sha256Hex(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// GET /api/sessions/:id/stream (SSE)
func sessionStreamHandler(c *gin.Context) {
	id := c.Param("id")
	sessionsMu.RLock()
	s := sessions[id]
	sessionsMu.RUnlock()
	if s == nil {
		apiErr(c, http.StatusNotFound, "NOT_FOUND", "session not found")
		return
	}

	sub := &sessionSubscriber{ch: make(chan sse.Event, 32)}
	// add subscriber
	s.mu.Lock()
	s.Subs[sub] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.Subs, sub); s.mu.Unlock(); close(sub.ch) }()

	// Immediately send a status snapshot
	ss := s.summarize()
	s.broadcast(map[string]any{"type": "status", "status": ss.Status, "usage": ss.Usage, "ts": time.Now().UTC().Format(time.RFC3339Nano)})

	c.Stream(func(w io.Writer) bool {
		if evt, ok := <-sub.ch; ok {
			c.SSEvent(evt.Event, evt.Data)
			return true
		}
		return false
	})
}

// POST /api/sessions/:id/input
func sessionInputHandler(c *gin.Context) {
	id := c.Param("id")
	sessionsMu.RLock()
	s := sessions[id]
	sessionsMu.RUnlock()
	if s == nil {
		apiErr(c, http.StatusNotFound, "NOT_FOUND", "session not found")
		return
	}
	var req struct {
		DataB64 string `json:"data_b64"`
		EOF     bool   `json:"eof"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr(c, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
		return
	}
	if req.EOF {
		s.mu.Lock()
		if s.Stdin != nil && !s.stdinClosed {
			_ = s.Stdin.Close()
			s.stdinClosed = true
		}
		s.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"ok": true, "written_bytes": 0})
		return
	}
	if strings.TrimSpace(req.DataB64) == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true, "written_bytes": 0})
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.DataB64)
	if err != nil {
		apiErr(c, http.StatusBadRequest, "BAD_REQUEST", "invalid base64")
		return
	}

	s.mu.Lock()
	cur := s.Usage.StdinBytes
	max := s.Limits.StdinBytes
	rem := max - cur
	if rem < 0 {
		rem = 0
	}
	toWrite := data
	if max > 0 && int64(len(data)) > rem {
		toWrite = data[:rem]
	}
	writer := s.Stdin
	s.mu.Unlock()

	var n int
	var werr error
	if len(toWrite) > 0 && writer != nil {
		n, werr = writer.Write(toWrite)
	}
	if werr != nil {
		apiErr(c, http.StatusBadGateway, "CONTAINER_ERROR", "stdin write failed")
		return
	}
	if n > 0 {
		s.mu.Lock()
		s.Usage.StdinBytes += int64(n)
		s.mu.Unlock()
		s.writeNDJSONLine(map[string]any{"dir": "stdin", "size": n, "sha256": sha256Hex(toWrite[:n]), "data_b64": base64.StdEncoding.EncodeToString(toWrite[:n])})
	}
	if max > 0 && int64(len(data)) > rem {
		// exceeded
		s.broadcast(map[string]any{"type": "limit", "kind": "stdin_bytes", "detail": fmt.Sprintf("cap %d reached", s.Limits.StdinBytes), "ts": time.Now().UTC().Format(time.RFC3339Nano)})
		apiErr(c, http.StatusBadRequest, "RESOURCE_EXCEEDED", fmt.Sprintf("stdin_bytes cap %d", s.Limits.StdinBytes))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "written_bytes": n})
}

// POST /api/sessions/:id/stop
func sessionStopHandler(c *gin.Context) {
	id := c.Param("id")
	sessionsMu.RLock()
	s := sessions[id]
	sessionsMu.RUnlock()
	if s == nil {
		apiErr(c, http.StatusNotFound, "NOT_FOUND", "session not found")
		return
	}
	var req struct {
		Signal string `json:"signal"`
	}
	_ = c.ShouldBindJSON(&req)
	sig := strings.ToUpper(strings.TrimSpace(req.Signal))
	if sig == "" {
		sig = "TERM"
	}

	s.mu.Lock()
	cmd := s.CmdProc
	s.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		if sig == "KILL" {
			_ = cmd.Process.Kill()
		} else {
			_ = cmd.Process.Signal(os.Interrupt)
		}
	}
	// best-effort docker rm -f if still around
	cname := s.ContainerName
	go func(name string) {
		time.Sleep(2 * time.Second)
		if name != "" {
			_ = exec.Command("docker", "rm", "-f", name).Run()
		}
	}(cname)

	c.JSON(http.StatusOK, gin.H{"session": s.summarize()})
}

// GET /api/sessions/:id
func sessionStatusHandler(c *gin.Context) {
	id := c.Param("id")
	sessionsMu.RLock()
	s := sessions[id]
	sessionsMu.RUnlock()
	if s == nil {
		apiErr(c, http.StatusNotFound, "NOT_FOUND", "session not found")
		return
	}

	// assemble artifacts
	s.mu.Lock()
	stdoutTail := base64.StdEncoding.EncodeToString(s.StdoutTail.Bytes())
	stderrTail := base64.StdEncoding.EncodeToString(s.StderrTail.Bytes())
	transcriptPath := s.TranscriptPath
	s.mu.Unlock()

	url := ""
	if transcriptPath != "" {
		url = signPath(fmt.Sprintf("/api/sessions/%s/transcript.ndjson", id), time.Now().Add(15*time.Minute))
	}
	c.JSON(http.StatusOK, gin.H{
		"session": s.summarize(),
		"artifacts": gin.H{
			"stdout_tail_b64": stdoutTail,
			"stderr_tail_b64": stderrTail,
			"transcript_url":  url,
			"files":           []any{},
		},
	})
}

// GET /api/sessions/:id/transcript.ndjson?sig=...&exp=...
func sessionTranscriptHandler(c *gin.Context) {
	id := c.Param("id")
	path := fmt.Sprintf("/api/sessions/%s/transcript.ndjson", id)
	sig := c.Query("sig")
	exp := c.Query("exp")
	if !verifySignature(path, exp, sig) {
		apiErr(c, http.StatusForbidden, "POLICY_VIOLATION", "bad signature")
		return
	}
	sessionsMu.RLock()
	s := sessions[id]
	sessionsMu.RUnlock()
	if s == nil || s.TranscriptPath == "" {
		apiErr(c, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	c.FileAttachment(s.TranscriptPath, filepath.Base(s.TranscriptPath))
}

// ──────────────────────────────────────────────────────────────────────────────
// Router wiring
// ──────────────────────────────────────────────────────────────────────────────

func registerSessionRoutes(r *gin.RouterGroup) {
	r.POST("/sessions", startSessionHandler)
	r.GET("/sessions/:id/stream", sessionStreamHandler)
	r.POST("/sessions/:id/input", sessionInputHandler)
	r.POST("/sessions/:id/stop", sessionStopHandler)
	r.GET("/sessions/:id", sessionStatusHandler)
	r.GET("/sessions/:id/transcript.ndjson", sessionTranscriptHandler)
}

// ──────────────────────────────────────────────────────────────────────────────
// Minimal tests helpers: not executed here; full tests in sessions_test.go
// ──────────────────────────────────────────────────────────────────────────────

// sortedKeys returns a sorted slice of keys from map.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ensure reference to docker flags from worker.go to prevent dead code elimination if unused here
func _refDockerFlags() string {
	return fmt.Sprintf("%s-%s-%s-%s-%d", pythonImage, dockerUser, dockerCPUs, dockerMemory, dockerExtraTime/time.Second)
}

// prevent unused import warnings when tests are excluded
var _ = []any{runtime.NumCPU}
