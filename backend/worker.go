package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
		out, err := os.Create(fpath)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(out, rc)
		out.Close()
		rc.Close()
	}

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
	for _, tc := range tests {
		timeout := time.Duration(tc.TimeLimitSec * float64(time.Second))
		out, err, timedOut, runtime := executePythonDir(tmpDir, mainFile, tc.Stdin, timeout)

		status := "passed"
		switch {
		case timedOut:
			status = "time_limit_exceeded"
		case err != nil || strings.TrimSpace(out) != strings.TrimSpace(tc.ExpectedStdout):
			status = "wrong_output"
		}

		res := &Result{SubmissionID: id, TestCaseID: tc.ID, Status: status, ActualStdout: out, RuntimeMS: int(runtime.Milliseconds())}
		CreateResult(res)
		if status != "passed" {
			allPass = false
		}
	}

	if allPass {
		UpdateSubmissionStatus(id, "completed")
	} else {
		UpdateSubmissionStatus(id, "failed")
	}
}

func executePythonDir(dir, file, stdin string, timeout time.Duration) (string, error, bool, time.Duration) {
	abs, _ := filepath.Abs(dir)
	fmt.Printf("[worker] Running: %s/%s with timeout %v\n", abs, file, timeout)
	// allow some extra time for container startup and shutdown
	ctx, cancel := context.WithTimeout(context.Background(), timeout+dockerExtraTime)
	defer cancel()

	mount := fmt.Sprintf("%s:/code:ro", abs)

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

	start := time.Now()
	outBytes, err := cmd.CombinedOutput()
	duration := time.Since(start)

	ctxTimedOut := ctx.Err() == context.DeadlineExceeded

	out := strings.TrimSpace(string(outBytes))
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

	return out, err, timedOut, runtime
}
