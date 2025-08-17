package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Job represents a grading task for one submission.
type Job struct{ SubmissionID int }

var taskQueue chan Job

const (
	pythonImage  = "python:3.11"
	dockerUser   = "65534" // run containers as 'nobody'
	dockerCPUs   = "1"     // limit CPU shares
	dockerMemory = "256m"  // memory limit
	// additional grace period for docker startup/shutdown
	dockerExtraTime = 10 * time.Second
)

// StartWorker starts n workers processing the grading queue.
func StartWorker(n int) {
	taskQueue = make(chan Job, 100)
	ensureDockerImage(pythonImage)
	for i := 0; i < n; i++ {
		go workerLoop()
	}
}

// EnqueueJob enqueues a submission for grading.
func EnqueueJob(j Job) { taskQueue <- j }

func workerLoop() {
	for j := range taskQueue {
		runSubmission(j.SubmissionID)
	}
}

func ensureDockerImage(img string) {
	if err := exec.Command("docker", "inspect", "--type=image", img).Run(); err != nil {
		exec.Command("docker", "pull", img).Run()
	}
}

func runSubmission(id int) {
	sub, err := GetSubmission(id)
	if err != nil {
		return
	}
	// Determine assignment mode
	if a, err := GetAssignment(sub.AssignmentID); err == nil {
		if a.LLMInteractive {
			UpdateSubmissionStatus(id, "running")
			runLLMInteractive(sub, a)
			return
		}
		// Early exit for manual-review assignments when not using LLM
		if a.ManualReview {
			return
		}
	}

	UpdateSubmissionStatus(id, "running")

	// Recreate submitted files from the stored archive
	tmpDir, err := os.MkdirTemp("", "grader-")
	if err != nil {
		UpdateSubmissionStatus(id, "failed")
		return
	}
	defer os.RemoveAll(tmpDir)

	data, err := base64.StdEncoding.DecodeString(sub.CodeContent)
	if err != nil {
		UpdateSubmissionStatus(id, "failed")
		return
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		UpdateSubmissionStatus(id, "failed")
		return
	}
	for _, f := range zr.File {
		fpath := filepath.Join(tmpDir, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(fpath), 0755)
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(out, rc)
		out.Close()
		os.Chmod(fpath, 0644)
		rc.Close()
	}

	// enforce permissions after extraction
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			os.Chmod(path, 0755)
		} else {
			os.Chmod(path, 0644)
		}
		return nil
	})
	var mainFile string
	var firstPy string
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".py") {
			rel, _ := filepath.Rel(tmpDir, path)
			if firstPy == "" {
				firstPy = rel
			}
			content, _ := os.ReadFile(path)
			if strings.Contains(string(content), "__main__") {
				mainFile = rel
				return io.EOF
			}
		}
		return nil
	})
	if mainFile == "" {
		if _, err := os.Stat(filepath.Join(tmpDir, "main.py")); err == nil {
			mainFile = "main.py"
		} else {
			mainFile = firstPy
		}
	}
	if mainFile == "" {
		UpdateSubmissionStatus(id, "failed")
		return
	}

	tests, err := ListTestCases(sub.AssignmentID)
	if err != nil {
		UpdateSubmissionStatus(id, "failed")
		return
	}

	allPass := true
	totalWeight := 0.0
	earnedWeight := 0.0
	for _, tc := range tests {
		timeout := time.Duration(tc.TimeLimitSec * float64(time.Second))
		var stdout, stderr string
		var exitCode int
		var timedOut bool
		var runtime time.Duration
		if tc.UnittestCode != nil && tc.UnittestName != nil {
			stdout, stderr, exitCode, timedOut, runtime = executePythonUnit(tmpDir, mainFile, *tc.UnittestCode, *tc.UnittestName, timeout)
		} else {
			stdout, stderr, exitCode, timedOut, runtime = executePythonDir(tmpDir, mainFile, tc.Stdin, timeout)
		}

		status := "passed"
		if tc.UnittestCode != nil && tc.UnittestName != nil {
			if timedOut {
				status = "time_limit_exceeded"
			} else if exitCode != 0 {
				status = "wrong_output"
			}
		} else {
			switch {
			case timedOut:
				status = "time_limit_exceeded"
			case exitCode != 0:
				status = "runtime_error"
			case strings.TrimSpace(stdout) != strings.TrimSpace(tc.ExpectedStdout):
				status = "wrong_output"
			}
		}

		res := &Result{SubmissionID: id, TestCaseID: tc.ID, Status: status, ActualStdout: stdout, Stderr: stderr, ExitCode: exitCode, RuntimeMS: int(runtime.Milliseconds())}
		CreateResult(res)
		totalWeight += tc.Weight
		if status != "passed" {
			allPass = false
		} else {
			earnedWeight += tc.Weight
		}
	}

	a, err := GetAssignment(sub.AssignmentID)
	if err == nil {
		score := 0.0
		switch a.GradingPolicy {
		case "all_or_nothing":
			if allPass {
				score = float64(a.MaxPoints)
			}
		case "weighted":
			score = earnedWeight
		}
		if sub.CreatedAt.After(a.Deadline) {
			score = 0
		}
		SetSubmissionPoints(id, score)
	}

	if allPass {
		UpdateSubmissionStatus(id, "completed")
	} else {
		UpdateSubmissionStatus(id, "failed")
	}
}

// LLM-interactive flow
func runLLMInteractive(sub *Submission, a *Assignment) {
	// Recreate submitted files from the stored archive
	tmpDir, err := os.MkdirTemp("", "grader-llm-")
	if err != nil {
		UpdateSubmissionStatus(sub.ID, "failed")
		return
	}
	defer os.RemoveAll(tmpDir)

	data, err := base64.StdEncoding.DecodeString(sub.CodeContent)
	if err != nil {
		UpdateSubmissionStatus(sub.ID, "failed")
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		UpdateSubmissionStatus(sub.ID, "failed")
		return
	}
	for _, f := range zr.File {
		fpath := filepath.Join(tmpDir, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(fpath), 0755)
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(out, rc)
		out.Close()
		os.Chmod(fpath, 0644)
		rc.Close()
	}
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			os.Chmod(path, 0755)
		} else {
			os.Chmod(path, 0644)
		}
		return nil
	})

	// Detect main file
	var mainFile, firstPy string
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".py") {
			rel, _ := filepath.Rel(tmpDir, path)
			if firstPy == "" {
				firstPy = rel
			}
			content, _ := os.ReadFile(path)
			if strings.Contains(string(content), "__main__") {
				mainFile = rel
				return io.EOF
			}
		}
		return nil
	})
	if mainFile == "" {
		if _, err := os.Stat(filepath.Join(tmpDir, "main.py")); err == nil {
			mainFile = "main.py"
		} else {
			mainFile = firstPy
		}
	}
	if mainFile == "" {
		_ = CreateLLMRun(&LLMRun{SubmissionID: sub.ID, SmokeOK: false, Verdict: strPtr("SMOKE_FAIL"), Reason: strPtr("no python file found")})
		UpdateSubmissionStatus(sub.ID, "failed")
		return
	}

	// Stage 1: Smoke
	smokeOK, smokeReason := smokePythonProgram(tmpDir, mainFile)
	llm := &LLMRun{SubmissionID: sub.ID, SmokeOK: smokeOK}
	if !smokeOK {
		llm.Verdict = strPtr("SMOKE_FAIL")
		llm.Reason = &smokeReason
		_ = CreateLLMRun(llm)
		UpdateSubmissionStatus(sub.ID, "failed")
		return
	}

	// Stage 2: static review (critical)
	review := llmStaticReview(a, tmpDir)
	if review != nil {
		b, _ := json.Marshal(review)
		s := string(b)
		llm.ReviewJSON = &s
	}

	// Extract acceptance gate from static review if present
	var acceptancePresent bool
	var acceptanceOK bool
	var acceptanceReason string
	if review != nil {
		if accRaw, ok := review["acceptance"]; ok {
			if accMap, ok := accRaw.(map[string]any); ok {
				if v, ok := accMap["ok"].(bool); ok {
					acceptancePresent = true
					acceptanceOK = v
				}
				if v, ok := accMap["reason"].(string); ok {
					acceptanceReason = v
				}
			}
		}
	}

	// Stage 3: scenario planning (LLM) + optional teacher scenarios
	planScen, planJSON := llmPlanScenarios(a, tmpDir, review)
	// merge teacher-provided scenarios if any
	var merged []interactiveScenario
	if len(planScen) > 0 {
		merged = append(merged, planScen...)
	}
	if a.LLMScenariosRaw != nil && strings.TrimSpace(*a.LLMScenariosRaw) != "" {
		var teacherScen []interactiveScenario
		_ = json.Unmarshal([]byte(*a.LLMScenariosRaw), &teacherScen)
		if len(teacherScen) > 0 {
			merged = append(merged, teacherScen...)
		}
	}
	if len(merged) == 0 {
		// generic minimal scenario (non-opinionated)
		merged = []interactiveScenario{{Name: "smoke", Steps: []map[string]string{{"send": ""}}}}
	}

	pass, resultsJSON, transcript, verdict, reason := runInteractiveScenarios(tmpDir, mainFile, merged)

	// Apply acceptance gate: explicit rejection from static review forces failure
	if acceptancePresent && !acceptanceOK {
		pass = false
		if verdict == "PASS" {
			verdict = "REJECTED"
		}
		if reason == "" && strings.TrimSpace(acceptanceReason) != "" {
			reason = acceptanceReason
		}
	}
	// Combine plan + results for nicer printing
	var plan any
	if strings.TrimSpace(planJSON) != "" {
		_ = json.Unmarshal([]byte(planJSON), &plan)
	}
	var res any
	_ = json.Unmarshal([]byte(resultsJSON), &res)
	comb := map[string]any{"plan": plan, "results": res}
	combBytes, _ := json.Marshal(comb)
	combStr := string(combBytes)

	llm.InteractiveJSON = &combStr
	llm.Transcript = &transcript
	llm.Verdict = &verdict
	if reason != "" {
		llm.Reason = &reason
	}
	_ = CreateLLMRun(llm)

	if pass {
		if a.LLMAutoAward {
			_ = SetSubmissionPoints(sub.ID, float64(a.MaxPoints))
		}
		UpdateSubmissionStatus(sub.ID, "completed")
	} else {
		UpdateSubmissionStatus(sub.ID, "failed")
	}
}

func strPtr(s string) *string { return &s }

// smokePythonProgram tries to run the program briefly (expecting input). Timeout is OK.
func smokePythonProgram(dir, file string) (bool, string) {
	abs, _ := filepath.Abs(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond+dockerExtraTime)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-i", "--network=none", "--user", dockerUser, "--cpus", dockerCPUs, "--memory", dockerMemory, "-v", fmt.Sprintf("%s:/code:ro,z", abs), pythonImage, "bash", "-lc", fmt.Sprintf("python /code/%s", strings.ReplaceAll(file, "'", "'\\''")))
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	// Keep stdin open so input() blocks instead of seeing EOF
	pr, pw := io.Pipe()
	cmd.Stdin = pr
	_ = cmd.Start()
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = pw.Close()
		if ctx.Err() == context.DeadlineExceeded {
			return true, "timeout while waiting for input"
		}
	case <-done:
		_ = pw.Close()
	}
	if stderrBuf.Len() > 0 {
		return false, strings.TrimSpace(stderrBuf.String())
	}
	return true, "exited cleanly"
}

// llmStaticReview calls an LLM to produce a critical review JSON; returns nil on failure.
func llmStaticReview(a *Assignment, dir string) map[string]any {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5"
	}
	base := os.Getenv("OPENAI_API_BASE")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	// collect small code excerpts (truncated)
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".py") {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		b, _ := os.ReadFile(path)
		s := string(b)
		if len(s) > 1200 {
			s = s[:1200] + "\n..."
		}
		files = append(files, fmt.Sprintf("# %s\n%s", rel, s))
		return nil
	})
	// Critical, adversarial mindset and strict JSON schema
	sys := "You are an code reviewer for CLI programs. Be moderately critical and practical. Don't accept any code that has some mistake in it that falls into category of wrong edge-case handling, wrong output for input. Return ONLY strict JSON."
	user := fmt.Sprintf(`Assignment title: %s
Assignment description:
%s

Student code (truncated excerpts):
%s

Return STRICT JSON exactly in this shape (no extra keys, no markdown, no comments):
{
  "summary": "string",
  "issues": [
    {"title":"string","severity":"low|medium|high|critical","rationale":"string",
     "reproduction": {"inputs": ["string"], "expect_regex": "^.*$", "notes": "string"}}
  ],
  "suggestions": ["string"],
  "risk_based_tests": [
    {"name":"string","steps":[{"send":"string"},{"send":"string","expect_regex":"^.*$"}]}
  ],
  "acceptance": {"ok": true, "reason": "string"}
}

Rules:
- Severity reflects impact. Prefer concrete, reproducible risks.
- risk_based_tests should turn suspected failures into runnable steps.
- If unknown, use empty arrays.`, a.Title, a.Description, strings.Join(files, "\n\n"))
	payload := map[string]any{"model": model, "messages": []map[string]string{{"role": "system", "content": sys}, {"role": "user", "content": user}}}
	body, _ := json.Marshal(payload)
	endpoint := strings.TrimRight(base, "/") + "/chat/completions"
	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode >= 300 {
		return nil
	}
	defer resp.Body.Close()
	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.NewDecoder(resp.Body).Decode(&raw) != nil || len(raw.Choices) == 0 {
		return nil
	}
	txt := strings.TrimSpace(raw.Choices[0].Message.Content)
	var out map[string]any
	if json.Unmarshal([]byte(txt), &out) == nil {
		return out
	}
	return map[string]any{"summary": txt}
}

type interactiveScenario struct {
	Name  string              `json:"name"`
	Steps []map[string]string `json:"steps"`
	Notes string              `json:"notes"`
}

// llmPlanScenarios asks the model to generate assignment-specific scenarios, leveraging review risks.
func llmPlanScenarios(a *Assignment, dir string, review map[string]any) ([]interactiveScenario, string) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return nil, ""
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5"
	}
	base := os.Getenv("OPENAI_API_BASE")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".py") {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		b, _ := os.ReadFile(path)
		s := string(b)
		if len(s) > 1000 {
			s = s[:1000] + "\n..."
		}
		files = append(files, fmt.Sprintf("# %s\n%s", rel, s))
		return nil
	})
	var reviewPart string
	if review != nil {
		if b, err := json.Marshal(review); err == nil {
			reviewPart = string(b)
		}
	}
	sys := "You design black-box CLI test scenarios. Output ONLY strict JSON that the runner can execute."
	user := fmt.Sprintf(`Assignment title: %s
Assignment description:
%s

Student code (truncated excerpts):
%s

Static review (may include risks):
%s

Return STRICT JSON exactly in this shape (no extras):
{
  "scenarios": [
    {
      "name": "string",
      "rationale": "string",
      "steps": [
        {"send": "string"},
        {"send": "string", "expect_regex": "^.*$"}
      ]
    }
  ]
}
Guidelines:
- 1-5 scenarios, 1-6 steps each.
- steps simulate user typing lines into stdin; expect_regex is optional and must be a compact regex.
- Avoid problem-specific jargon in send values unless clearly present in the assignment.
- Incorporate risk-based tests from the review when present.`, a.Title, a.Description, strings.Join(files, "\n\n"), reviewPart)
	payload := map[string]any{"model": model, "messages": []map[string]string{{"role": "system", "content": sys}, {"role": "user", "content": user}}}
	body, _ := json.Marshal(payload)
	endpoint := strings.TrimRight(base, "/") + "/chat/completions"
	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode >= 300 {
		return nil, ""
	}
	defer resp.Body.Close()
	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.NewDecoder(resp.Body).Decode(&raw) != nil || len(raw.Choices) == 0 {
		return nil, ""
	}
	txt := strings.TrimSpace(raw.Choices[0].Message.Content)
	// Try to parse and coerce to interactiveScenario slice
	type plan struct {
		Scenarios []struct {
			Name      string              `json:"name"`
			Rationale string              `json:"rationale"`
			Steps     []map[string]string `json:"steps"`
		} `json:"scenarios"`
	}
	var p plan
	if err := json.Unmarshal([]byte(txt), &p); err != nil || len(p.Scenarios) == 0 {
		return nil, txt
	}
	var out []interactiveScenario
	for _, s := range p.Scenarios {
		// normalize expect key to expect_after
		steps := make([]map[string]string, 0, len(s.Steps))
		for _, st := range s.Steps {
			m := map[string]string{}
			if v, ok := st["send"]; ok {
				m["send"] = v
			}
			if v, ok := st["expect_regex"]; ok {
				m["expect_after"] = v
			} else if v, ok := st["expect"]; ok {
				m["expect_after"] = v
			}
			steps = append(steps, m)
		}
		out = append(out, interactiveScenario{Name: s.Name, Notes: s.Rationale, Steps: steps})
	}
	return out, txt
}

func runInteractiveScenarios(dir, mainFile string, scenarios []interactiveScenario) (bool, string, string, string, string) {
	const maxCalls = 30
	const perStep = 1500 * time.Millisecond
	const maxWall = 90 * time.Second
	const maxOut = 64 * 1024

	abs, _ := filepath.Abs(dir)
	transcript := &strings.Builder{}
	calls := 0
	overallDeadline := time.Now().Add(maxWall)

	type stepRes struct {
		Step   int    `json:"step"`
		Sent   string `json:"sent"`
		Expect string `json:"expect"`
		Pass   bool   `json:"pass"`
		Notes  string `json:"notes"`
	}
	type scenRes struct {
		Name  string    `json:"name"`
		Pass  bool      `json:"pass"`
		Steps []stepRes `json:"steps"`
		Notes string    `json:"notes"`
	}
	var results []scenRes
	overallPass := true
	verdict := "PASS"
	reason := ""

	for _, sc := range scenarios {
		sr := scenRes{Name: sc.Name, Notes: sc.Notes}
		scenPass := true

		// Check overall wall time
		remaining := time.Until(overallDeadline)
		if remaining <= 0 {
			verdict = "INTERACTIVE_TIMEOUT"
			reason = "max wall time"
			overallPass = false
			scenPass = false
			results = append(results, sr)
			break
		}

		// Start fresh container for this scenario
		ctx, cancel := context.WithTimeout(context.Background(), remaining+dockerExtraTime)
		defer cancel()
		script := fmt.Sprintf("PYTHONUNBUFFERED=1 python -u /code/%s", strings.ReplaceAll(mainFile, "'", "'\\''"))
		cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-i", "--network=none", "--user", dockerUser, "--cpus", dockerCPUs, "--memory", dockerMemory, "-v", fmt.Sprintf("%s:/code:ro,z", abs), pythonImage, "bash", "-lc", script)
		stdoutPipe, _ := cmd.StdoutPipe()
		stderrPipe, _ := cmd.StderrPipe()
		stdinPipe, _ := cmd.StdinPipe()
		if err := cmd.Start(); err != nil {
			verdict = "RUNTIME_ERROR"
			reason = "container start failed"
			overallPass = false
			scenPass = false
			results = append(results, sr)
			break
		}

		outReader := bufio.NewReader(stdoutPipe)
		errReader := bufio.NewReader(stderrPipe)
		var bufOut, bufErr bytes.Buffer
		var mu sync.Mutex
		prevOutLen := 0
		prevErrLen := 0
		drainNewOutput := func() string {
			mu.Lock()
			defer mu.Unlock()
			ob := bufOut.Bytes()
			eb := bufErr.Bytes()
			var b strings.Builder
			if len(ob) > prevOutLen {
				b.Write(ob[prevOutLen:])
				prevOutLen = len(ob)
			}
			if len(eb) > prevErrLen {
				b.Write(eb[prevErrLen:])
				prevErrLen = len(eb)
			}
			return b.String()
		}

		go func() {
			b := make([]byte, 1024)
			for {
				n, err := outReader.Read(b)
				if n > 0 {
					mu.Lock()
					bufOut.Write(b[:n])
					mu.Unlock()
				}
				if err != nil {
					return
				}
			}
		}()
		go func() {
			b := make([]byte, 1024)
			for {
				n, err := errReader.Read(b)
				if n > 0 {
					mu.Lock()
					bufErr.Write(b[:n])
					mu.Unlock()
				}
				if err != nil {
					return
				}
			}
		}()

		readUntil := func(re *regexp.Regexp, timeout time.Duration) bool {
			mu.Lock()
			baseOut := bufOut.Len()
			baseErr := bufErr.Len()
			mu.Unlock()
			deadline := time.Now().Add(timeout)
			for time.Now().Before(deadline) {
				mu.Lock()
				ob := bufOut.Bytes()
				eb := bufErr.Bytes()
				ol := len(ob)
				el := len(eb)
				mu.Unlock()
				if ol > maxOut || el > maxOut {
					return false
				}
				if re == nil {
					if ol > baseOut || el > baseErr {
						return true
					}
				} else {
					if re.Match(ob) || re.Match(eb) {
						return true
					}
				}
				time.Sleep(30 * time.Millisecond)
			}
			return re == nil
		}

		// wait briefly for initial prompt/banner
		_ = readUntil(nil, perStep/2)
		if initOut := drainNewOutput(); initOut != "" {
			transcript.WriteString("PROGRAM> " + initOut + "\n")
		}
		for i, st := range sc.Steps {
			if calls >= maxCalls {
				verdict = "INTERACTIVE_TIMEOUT"
				reason = "max tool calls"
				overallPass = false
				scenPass = false
				break
			}
			sent := strings.TrimSpace(st["send"])
			expect := strings.TrimSpace(st["expect_after"])
			exp := strings.TrimSpace(st["expect"])
			if exp != "" {
				expect = exp
			}
			// wait a short time for prompt/output produced by previous step
			_ = readUntil(nil, perStep/6)
			// log only new output available before we send the next input
			if pre := drainNewOutput(); pre != "" {
				transcript.WriteString("PROGRAM> " + pre + "\n")
			}
			if sent != "" {
				_, _ = io.WriteString(stdinPipe, sent+"\n")
				transcript.WriteString("AI> " + sent + "\n")
				calls++
			}
			var re *regexp.Regexp
			if expect != "" {
				re = regexp.MustCompile(expect)
			}
			pass := readUntil(re, perStep)
			if post := drainNewOutput(); post != "" {
				transcript.WriteString("PROGRAM> " + post + "\n")
			}
			sr.Steps = append(sr.Steps, stepRes{Step: i + 1, Sent: sent, Expect: expect, Pass: pass})
			mu.Lock()
			if bufOut.Len() > maxOut || bufErr.Len() > maxOut {
				mu.Unlock()
				overallPass = false
				scenPass = false
				verdict = "OUTPUT_TRUNCATED"
				reason = "output cap exceeded"
				break
			}
			mu.Unlock()
		}

		_ = stdinPipe.Close()
		_ = cmd.Wait()
		if tail := drainNewOutput(); tail != "" {
			transcript.WriteString("PROGRAM> " + tail + "\n")
		}

		sr.Pass = scenPass
		results = append(results, sr)
		if !scenPass {
			break
		}
	}

	// If any scenario failed without a more specific verdict, mark as interactive failure
	if !overallPass && verdict == "PASS" {
		verdict = "INTERACTIVE_FAIL"
		reason = "scenario expectations not met"
	}

	inter := map[string]any{"scenarios": results, "overall_pass": overallPass}
	interJSON, _ := json.Marshal(inter)
	return overallPass, string(interJSON), transcript.String(), verdict, reason
}

// lastN helper removed (unused)

func executePythonDir(dir, file, stdin string, timeout time.Duration) (string, string, int, bool, time.Duration) {
	abs, _ := filepath.Abs(dir)
	fmt.Printf("[worker] Running: %s/%s with timeout %v\n", abs, file, timeout)
	// allow some extra time for container startup and shutdown
	ctx, cancel := context.WithTimeout(context.Background(), timeout+dockerExtraTime)
	defer cancel()

	mount := fmt.Sprintf("%s:/code:ro,z", abs)

	// Measure runtime inside the container. A shell script records timestamps
	// before and after executing the Python program and prints the elapsed
	// milliseconds as the last line of stdout with a unique prefix.
	script := fmt.Sprintf("start=$(date +%%s%%N); python /code/%s; status=$?; end=$(date +%%s%%N); echo '===RUNTIME_MS===' $(((end-start)/1000000)); exit $status", file)

	cmd := exec.CommandContext(ctx, "docker", "run",
		"--rm",
		"-i",
		"--network=none",
		"--user", dockerUser,
		"--cpus", dockerCPUs,
		"--memory", dockerMemory,
		"-v", mount,
		pythonImage, "bash", "-c", script)
	cmd.Stdin = strings.NewReader(stdin)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	ctxTimedOut := ctx.Err() == context.DeadlineExceeded

	out := strings.TrimSpace(stdoutBuf.String())
	var runtime time.Duration
	if lines := strings.Split(out, "\n"); len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "===RUNTIME_MS===") {
		rstr := strings.TrimSpace(strings.TrimPrefix(lines[len(lines)-1], "===RUNTIME_MS==="))
		if ms, perr := strconv.Atoi(rstr); perr == nil {
			runtime = time.Duration(ms) * time.Millisecond
			out = strings.Join(lines[:len(lines)-1], "\n")
		} else {
			runtime = duration
		}
	} else {
		runtime = duration
	}
	runtimeExceeded := runtime > timeout
	timedOut := ctxTimedOut || runtimeExceeded

	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return out, strings.TrimSpace(stderrBuf.String()), exitCode, timedOut, runtime
}

func executePythonUnit(dir, mainFile, testCode, testName string, timeout time.Duration) (string, string, int, bool, time.Duration) {
	abs, _ := filepath.Abs(dir)
	testPath := filepath.Join(dir, "run_test.py")
	content := fmt.Sprintf(`import sys, unittest, builtins, io

# patch assertEqual so comparisons use string values
orig_assertEqual = unittest.TestCase.assertEqual
def _assertEqual(self, first, second, msg=None):
    orig_assertEqual(self, str(first), str(second), msg)
unittest.TestCase.assertEqual = _assertEqual

# prevent provided test modules from auto-running all tests (e.g., unittest.main())
# so that we can selectively run a single test method by name below

def __grader_noop__(*args, **kwargs):
    return None
unittest.main = __grader_noop__

student_source = open('%s').read()

def student_code(*args):
    it = iter(str(a) for a in args)
    builtins.input = lambda prompt=None: next(it)
    out = io.StringIO()
    old = sys.stdout
    sys.stdout = out
    glb = {'__name__':'__main__'}
    exec(student_source, glb)
    sys.stdout = old
    return out.getvalue().strip()

%s

if __name__ == '__main__':
    suite = unittest.defaultTestLoader.loadTestsFromName('__main__.%s')
    result = unittest.TextTestRunner().run(suite)
    sys.exit(0 if result.wasSuccessful() else 1)
`, "/code/"+mainFile, testCode, testName)
	os.WriteFile(testPath, []byte(content), 0644)
	// Ensure permissions are readable by container user (nobody)
	_ = os.Chmod(dir, 0755)
	_ = os.Chmod(testPath, 0644)

	ctx, cancel := context.WithTimeout(context.Background(), timeout+dockerExtraTime)
	defer cancel()
	mount := fmt.Sprintf("%s:/code:ro,z", abs)
	script := fmt.Sprintf("start=$(date +%%s%%N); python /code/run_test.py; status=$?; end=$(date +%%s%%N); echo '===RUNTIME_MS===' $(((end-start)/1000000)); exit $status")
	cmd := exec.CommandContext(ctx, "docker", "run",
		"--rm", "-i", "--network=none", "--user", dockerUser,
		"--cpus", dockerCPUs, "--memory", dockerMemory,
		"-v", mount, pythonImage, "bash", "-c", script)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	ctxTimedOut := ctx.Err() == context.DeadlineExceeded

	out := strings.TrimSpace(stdoutBuf.String())
	var runtime time.Duration
	if lines := strings.Split(out, "\n"); len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "===RUNTIME_MS===") {
		rstr := strings.TrimSpace(strings.TrimPrefix(lines[len(lines)-1], "===RUNTIME_MS==="))
		if ms, perr := strconv.Atoi(rstr); perr == nil {
			runtime = time.Duration(ms) * time.Millisecond
			out = strings.Join(lines[:len(lines)-1], "\n")
		} else {
			runtime = duration
		}
	} else {
		runtime = duration
	}
	runtimeExceeded := runtime > timeout
	timedOut := ctxTimedOut || runtimeExceeded

	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return out, strings.TrimSpace(stderrBuf.String()), exitCode, timedOut, runtime
}
