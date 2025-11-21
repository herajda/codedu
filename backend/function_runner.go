package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type functionCallConfig struct {
	FunctionName string
	ArgsJSON     *string
	KwargsJSON   *string
	ExpectedJSON *string
}

type functionCallResult struct {
	Status       string  `json:"status"`
	Passed       bool    `json:"passed"`
	ReturnJSON   *string `json:"return_json"`
	ReturnRepr   string  `json:"return_repr"`
	Stdout       string  `json:"stdout"`
	Exception    string  `json:"exception"`
	Traceback    string  `json:"traceback"`
	ExpectedJSON *string `json:"expected_json"`
	ExpectedRepr *string `json:"expected_repr"`
}

func writeFunctionRunnerFiles(dir, mainFile string, cfg functionCallConfig) (string, string, error) {
	modulePath := fmt.Sprintf("%s/%s", strings.TrimRight(vmWorkspacePath(), "/"), strings.ReplaceAll(mainFile, "\\", "/"))
	payload := map[string]any{
		"module_path":   modulePath,
		"function_name": cfg.FunctionName,
	}
	if cfg.ArgsJSON != nil && strings.TrimSpace(*cfg.ArgsJSON) != "" {
		var parsed any
		if err := json.Unmarshal([]byte(*cfg.ArgsJSON), &parsed); err != nil {
			return "", "", fmt.Errorf("invalid function args JSON: %w", err)
		}
		payload["args"] = parsed
	}
	if cfg.KwargsJSON != nil && strings.TrimSpace(*cfg.KwargsJSON) != "" {
		var parsed any
		if err := json.Unmarshal([]byte(*cfg.KwargsJSON), &parsed); err != nil {
			return "", "", fmt.Errorf("invalid function kwargs JSON: %w", err)
		}
		payload["kwargs"] = parsed
	}
	if cfg.ExpectedJSON != nil && strings.TrimSpace(*cfg.ExpectedJSON) != "" {
		var parsed any
		if err := json.Unmarshal([]byte(*cfg.ExpectedJSON), &parsed); err != nil {
			return "", "", fmt.Errorf("invalid expected return JSON: %w", err)
		}
		payload["expected"] = parsed
	}

	configPath := filepath.Join(dir, "function_config.json")
	runnerPath := filepath.Join(dir, "function_runner.py")

	cfgBytes, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(configPath, cfgBytes, 0644); err != nil {
		return "", "", err
	}

	script := `import contextlib
import importlib.util
import io
import json
import pathlib
import sys
import traceback

MARKER = "===GRADER_JSON==="


def normalize_value(value):
    if isinstance(value, tuple):
        return [normalize_value(v) for v in value]
    if isinstance(value, list):
        return [normalize_value(v) for v in value]
    if isinstance(value, dict):
        return {k: normalize_value(v) for k, v in value.items()}
    return value


def load_module(path: str):
    spec = importlib.util.spec_from_file_location("student_module", path)
    module = importlib.util.module_from_spec(spec)
    loader = spec.loader
    assert loader is not None
    loader.exec_module(module)
    return module


def resolve_attr(root, dotted: str):
    target = root
    for part in dotted.split('.'):
        target = getattr(target, part)
    return target


def main():
    cfg_path = pathlib.Path(__file__).with_name('function_config.json')
    with cfg_path.open('r', encoding='utf-8') as fh:
        cfg = json.load(fh)

    result = {"status": "ok", "passed": False}
    module_stdout = io.StringIO()
    call_stdout = io.StringIO()

    try:
        with contextlib.redirect_stdout(module_stdout):
            module = load_module(cfg['module_path'])
        func = resolve_attr(module, cfg['function_name'])
        args = cfg.get('args') or []
        kwargs = cfg.get('kwargs') or {}

        with contextlib.redirect_stdout(call_stdout):
            value = func(*args, **kwargs)

        result['passed'] = True
        sentinel = object()
        expected = cfg.get('expected', sentinel)
        if expected is not sentinel:
            normalized_expected = normalize_value(expected)
            normalized_value = normalize_value(value)
            try:
                equal = normalized_value == normalized_expected
            except Exception as cmp_exc:  # noqa: BLE001
                equal = False
                result['compare_exception'] = repr(cmp_exc)
            result['passed'] = bool(equal)
            result['expected_repr'] = repr(expected)
            try:
                result['expected_json'] = json.dumps(normalized_expected)
            except TypeError:
                result['expected_json'] = None
            if isinstance(value, tuple):
                result['comparison_note'] = 'Compared after normalizing tuple return to list.'
        else:
            normalized_value = normalize_value(value)

        result['return_repr'] = repr(value)
        try:
            result['return_json'] = json.dumps(value)
        except TypeError:
            result['return_json'] = None
        if normalized_value is not value:
            try:
                result['return_json_normalized'] = json.dumps(normalized_value)
            except TypeError:
                result['return_json_normalized'] = None

    except Exception as exc:  # noqa: BLE001
        result['status'] = 'exception'
        result['exception'] = repr(exc)
        result['traceback'] = traceback.format_exc()

    result['stdout'] = module_stdout.getvalue() + call_stdout.getvalue()

    print(MARKER + json.dumps(result))
    if result['status'] != 'ok':
        sys.exit(2)
    if not result.get('passed', False):
        sys.exit(1)
    sys.exit(0)


if __name__ == '__main__':
    main()
`
	if err := os.WriteFile(runnerPath, []byte(script), 0644); err != nil {
		return "", "", err
	}
	return configPath, runnerPath, nil
}

func runFunctionCall(dir, mainFile string, cfg functionCallConfig, timeout time.Duration) (string, string, int, bool, time.Duration, *functionCallResult, error) {
	configPath, runnerPath, err := writeFunctionRunnerFiles(dir, mainFile, cfg)
	if err != nil {
		return "", "", 0, false, 0, nil, err
	}
	defer os.Remove(configPath)
	defer os.Remove(runnerPath)
	_ = os.Chmod(configPath, 0644)
	_ = os.Chmod(runnerPath, 0644)
	_ = ensureSandboxPerms(dir)

	script := fmt.Sprintf("start=$(date +%%s%%N); PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 HOME=/tmp LANG=C.UTF-8 python -u '%s'; status=$?; end=$(date +%%s%%N); echo '===RUNTIME_MS===' $(((end-start)/1000000)); exit $status", filepath.Base(runnerPath))

	ctx, cancel := context.WithTimeout(context.Background(), timeout+vmBootTimeout+vmExtraTimeout)
	defer cancel()

	vm, remoteDir, err := startVMWithWorkspace(ctx, dir, nil)
	if err != nil {
		return "", "", -1, ctx.Err() == context.DeadlineExceeded, 0, nil, fmt.Errorf("vm start failed: %w", err)
	}
	defer vm.Close()

	var stdoutBuf, stderrBuf strings.Builder
	start := time.Now()
	rawOut, rawErr, exitCode, runErr := vm.runCommand(ctx, remoteDir, script, nil)
	duration := time.Since(start)
	stdoutBuf.WriteString(rawOut)
	stderrBuf.WriteString(rawErr)

	ctxTimedOut := ctx.Err() == context.DeadlineExceeded
	combinedOut := stdoutBuf.String()
	var runtime time.Duration
	out := combinedOut
	const runtimeMarker = "===RUNTIME_MS==="
	if idx := strings.LastIndex(out, runtimeMarker); idx != -1 {
		tail := out[idx+len(runtimeMarker):]
		if fields := strings.Fields(tail); len(fields) > 0 {
			if ms, perr := strconv.Atoi(fields[0]); perr == nil {
				runtime = time.Duration(ms) * time.Millisecond
			} else {
				runtime = duration
			}
		} else {
			runtime = duration
		}
		out = out[:idx]
	} else {
		runtime = duration
	}

	if runErr != nil && exitCode == 0 {
		exitCode = -1
	}

	const marker = "===GRADER_JSON==="
	var meta *functionCallResult
	if idx := strings.LastIndex(out, marker); idx != -1 {
		payload := strings.TrimSpace(out[idx+len(marker):])
		out = out[:idx]
		var tmp functionCallResult
		if payload != "" {
			if err := json.Unmarshal([]byte(payload), &tmp); err == nil {
				meta = &tmp
			}
		}
	}

	timedOut := ctxTimedOut || runtime > timeout
	stdout := out
	if meta != nil && meta.Stdout != "" {
		stdout = meta.Stdout
	}

	return stdout, strings.TrimSpace(stderrBuf.String()), exitCode, timedOut, runtime, meta, nil
}
