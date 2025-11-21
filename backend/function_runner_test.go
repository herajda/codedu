package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFunctionRunnerFilesValid(t *testing.T) {
	dir := t.TempDir()
	mainFile := filepath.Join("student", "solution.py")
	args := "[2, 3]"
	kwargs := `{"scale": 2}`
	expected := "12"
	cfg := functionCallConfig{
		FunctionName: "multiply",
		ArgsJSON:     &args,
		KwargsJSON:   &kwargs,
		ExpectedJSON: &expected,
	}

	configPath, runnerPath, err := writeFunctionRunnerFiles(dir, mainFile, cfg)
	if err != nil {
		t.Fatalf("writeFunctionRunnerFiles returned error: %v", err)
	}
	if configPath == "" || runnerPath == "" {
		t.Fatalf("expected config and runner paths, got %q and %q", configPath, runnerPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	expectedModule := vmWorkspacePath() + "/student/solution.py"
	if payload["module_path"] != expectedModule {
		t.Fatalf("unexpected module_path: %#v", payload["module_path"])
	}
	if payload["function_name"] != "multiply" {
		t.Fatalf("unexpected function name: %#v", payload["function_name"])
	}

	script, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("failed to read runner: %v", err)
	}
	if len(script) == 0 {
		t.Fatalf("runner script is empty")
	}
	if !strings.Contains(string(script), "===GRADER_JSON===") {
		t.Fatalf("runner script missing sentinel marker")
	}
}

func TestWriteFunctionRunnerFilesInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	bad := "[1,"
	cfg := functionCallConfig{FunctionName: "foo", ArgsJSON: &bad}
	if _, _, err := writeFunctionRunnerFiles(dir, "main.py", cfg); err == nil {
		t.Fatalf("expected error for invalid JSON, got nil")
	}
}
