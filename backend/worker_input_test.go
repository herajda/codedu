package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecutePythonDirInputSuppression(t *testing.T) {
	// Skip if no docker/vm environment (mocking check)
	if os.Getenv("SKIP_VM_TESTS") != "" {
		t.Skip("Skipping VM tests")
	}

	// Set up environment for VM
	tempExecRoot := t.TempDir()
	// Save original execRoot and restore it after test
	origExecRoot := execRoot
	execRoot = tempExecRoot
	defer func() { execRoot = origExecRoot }()

	t.Setenv("DOCKER_USER", "1000:1000") // Use current user
	t.Setenv("PYTHON_BIN", "python3")

	dir := t.TempDir()
	mainFile := "main.py"
	code := `
try:
    val = input("PROMPT_TEXT")
    print(f"OUTPUT:{val}")
except EOFError:
    print("EOF")
`
	if err := os.WriteFile(filepath.Join(dir, mainFile), []byte(code), 0644); err != nil {
		t.Fatalf("failed to write main file: %v", err)
	}

	// Input to be fed to stdin
	stdin := "user_input_value"
	timeout := 5 * time.Second

	// Call executePythonDir
	stdout, stderr, exitCode, timedOut, _ := executePythonDir(dir, mainFile, stdin, timeout)

	if timedOut {
		t.Fatalf("execution timed out")
	}
	if exitCode != 0 {
		t.Fatalf("execution failed with exit code %d. Stderr: %s", exitCode, stderr)
	}

	// Verify stdout does NOT contain "PROMPT_TEXT"
	if strings.Contains(stdout, "PROMPT_TEXT") {
		t.Errorf("stdout should not contain input prompt. Got: %q", stdout)
	}

	// Verify stdout DOES contain "OUTPUT:user_input_value"
	expectedOutput := "OUTPUT:user_input_value"
	if !strings.Contains(stdout, expectedOutput) {
		t.Errorf("stdout missing expected output. Got: %q", stdout)
	}
}
