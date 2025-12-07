package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExecutePythonDirTimeoutLogic(t *testing.T) {
	if os.Getenv("SKIP_VM_TESTS") != "" {
		t.Skip("Skipping VM tests")
	}

	// Setup
	tempExecRoot := t.TempDir()
	origExecRoot := execRoot
	execRoot = tempExecRoot
	defer func() { execRoot = origExecRoot }()

	t.Setenv("DOCKER_USER", "1000:1000")
	t.Setenv("PYTHON_BIN", "python3")

	dir := t.TempDir()
	mainFile := "main.py"
	// Script sleeps for 10 seconds, which is much longer than the 2s timeout
	code := `
import time
time.sleep(10)
print("Done")
`
	if err := os.WriteFile(filepath.Join(dir, mainFile), []byte(code), 0644); err != nil {
		t.Fatalf("failed to write main file: %v", err)
	}

	// Set timeout to 2 seconds
	timeout := 2 * time.Second

	t.Logf("Starting execution with timeout %v...", timeout)
	start := time.Now()
	stdout, stderr, exitCode, timedOut, runtime := executePythonDir(dir, mainFile, "", timeout)
	totalDuration := time.Since(start)

	t.Logf("Execution finished. TimedOut: %v, Runtime: %v, TotalDuration: %v", timedOut, runtime, totalDuration)
	t.Logf("Stdout: %q, Stderr: %q, ExitCode: %d", stdout, stderr, exitCode)

	if !timedOut {
		t.Errorf("Expected timedOut=true, got false")
	}

	// Runtime should be around 2s (the timeout).
	// If it waited for the script to finish (old behavior), runtime would be ~10s.
	if runtime > 5*time.Second {
		t.Errorf("Runtime %v is too long (expected ~2s). The process was not killed immediately.", runtime)
	}
}
