package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Job represents a grading task for one submission.
type Job struct{ SubmissionID int }

var taskQueue chan Job

const pythonImage = "python:3.11"

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

	tests, err := ListTestCases(sub.AssignmentID)
	if err != nil {
		UpdateSubmissionStatus(id, "failed")
		return
	}

	allPass := true
	for _, tc := range tests {
		out, err, timedOut, runtime := executePython(sub.CodePath, tc.Stdin, time.Duration(tc.TimeLimitMS)*time.Millisecond)

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

func executePython(path, stdin string, timeout time.Duration) (string, error, bool, time.Duration) {
	abs, _ := filepath.Abs(path)
	fmt.Printf("[worker] Running: %s with timeout %v\n", abs, timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-i", "-v", fmt.Sprintf("%s:/code/main.py:ro", abs), pythonImage, "python", "/code/main.py")
	cmd.Stdin = strings.NewReader(stdin)

	start := time.Now()
	out, err := cmd.CombinedOutput()
	runtime := time.Since(start)

	timedOut := ctx.Err() == context.DeadlineExceeded
	return string(out), err, timedOut, runtime
}
