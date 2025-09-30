package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

// ──────────────────────────────────────────────────────────────────────────────
// persistent run sessions for manual review
// ──────────────────────────────────────────────────────────────────────────────

type RunSession struct {
	ContainerName string
	TmpDir        string
	StartedAt     time.Time
	LastActive    time.Time
	TTL           time.Duration
	Running       bool
	Ended         bool
	TimedOut      bool
	ExitCode      int
	AttachCount   int

	// process and IO
	Cmd   *exec.Cmd
	Stdin io.WriteCloser

	// output buffers (accumulated for replay on reattach)
	BufOut []byte
	BufErr []byte

	// subscribers receive JSON-like maps already typed for the WS
	Subs map[chan map[string]any]struct{}

	Timer *time.Timer

	Mu sync.Mutex

	// GUI (noVNC) session information for Tkinter apps
	GuiContainerName string
	GuiHostPort      int // localhost port for container's noVNC HTTP
	GuiEnabled       bool
}

var runSessionsMu sync.Mutex
var runSessions = map[string]*RunSession{}

// ──────────────────────────────────────────────────────────────────────────────
// utilities
// ──────────────────────────────────────────────────────────────────────────────

func getClass(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	detail, err := GetClassDetail(id, c.GetString("role"), getUserID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func getClassProgress(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		var x int
		if err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, id, getUserID(c)); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	prog, err := GetClassProgress(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, prog)
}

// ──────────────────────────────────────────
// basic user helpers (used from auth.go)
// ──────────────────────────────────────────

func CreateStudent(email, hash string, name, bkClass, bkUID *string) error {
	_, err := DB.Exec(
		`INSERT INTO users (email, password_hash, name, role, email_verified, bk_class, bk_uid)
                 VALUES ($1,$2,$3,'student',TRUE,$4,$5)`,
		email, hash, name, bkClass, bkUID,
	)
	return err
}

func FindUserByEmail(email string) (*User, error) {
	var u User
	err := DB.Get(&u, `
            SELECT id, email, password_hash, name, role, email_verified, email_verified_at, bk_class, bk_uid
              FROM users
             WHERE email = $1`,
		email,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func listStudents(c *gin.Context) {
	list, err := ListAllStudents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func deleteUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := DeleteUser(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func listSubs(c *gin.Context) {
	uid := getUserID(c)
	list, err := ListSubmissionsForStudent(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}

// createAssignment: POST /api/classes/:id/assignments
func createAssignment(c *gin.Context) {
	// NEW: pull the class id from the URL and validate it
	classID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid class id"})
		return
	}
	if c.GetString("role") == "teacher" {
		var x int
		if err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, classID, getUserID(c)); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	var req struct {
		Title         string `json:"title" binding:"required"`
		Description   string `json:"description"`
		ShowTraceback bool   `json:"show_traceback"`
		ManualReview  bool   `json:"manual_review"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a := &Assignment{
		ClassID:          classID,
		Title:            req.Title,
		Description:      req.Description,
		Deadline:         time.Now().Add(24 * time.Hour),
		MaxPoints:        100,
		GradingPolicy:    "all_or_nothing",
		Published:        false,
		ShowTraceback:    req.ShowTraceback,
		ManualReview:     req.ManualReview,
		CreatedBy:        getUserID(c),
		SecondDeadline:   nil,
		LatePenaltyRatio: 0.5,
	}
	if err := CreateAssignment(a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create assignment"})
		return
	}
	c.JSON(http.StatusCreated, a)
}

// listAssignments: GET /api/assignments
func listAssignments(c *gin.Context) {
	list, err := ListAssignments(c.GetString("role"), getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list"})
		return
	}
	c.JSON(http.StatusOK, list)
}

// getAssignment: GET /api/assignments/:id
func getAssignment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := GetAssignment(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
    role := c.GetString("role")
    if role == "student" {
        if ok, err := IsStudentOfAssignment(id, getUserID(c)); err != nil || !ok {
            c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
            return
        }
        if !a.Published {
            c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
            return
        }
        // If a per-student override exists, surface it as the effective deadline
        if o, err := GetDeadlineOverride(id, getUserID(c)); err == nil && o != nil {
            a.Deadline = o.NewDeadline
        }
        subs, _ := ListSubmissionsForAssignmentAndStudent(id, getUserID(c))
        tests, _ := ListTestCases(id)
        c.JSON(http.StatusOK, gin.H{"assignment": a, "submissions": subs, "tests_count": len(tests)})
        return
    } else if role == "teacher" {
		if ok, err := IsTeacherOfAssignment(id, getUserID(c)); err != nil || !ok {
			// Allow preview if the assignment is shared in Teachers' group
			var x int
			if err := DB.Get(&x, `SELECT 1 FROM class_files WHERE class_id=$1 AND assignment_id=$2 LIMIT 1`, TeacherGroupID, id); err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
	}
	tests, _ := ListTestCases(id)
	if a.GradingPolicy == "weighted" {
		sum := 0.0
		for _, t := range tests {
			sum += t.Weight
		}
		a.MaxPoints = int(sum)
	}
	resp := gin.H{"assignment": a, "tests": tests}
	if role == "teacher" || role == "admin" {
		subs, _ := ListSubmissionsForAssignment(id)
		var tsubs []SubmissionWithStudent
		if role == "teacher" {
			// Show only this teacher's runs
			tsubs, _ = ListTeacherRunsForAssignmentByUser(id, getUserID(c))
		} else {
			// Admins can see all teacher runs
			tsubs, _ = ListTeacherRunsForAssignment(id)
		}
		resp["submissions"] = subs
		resp["teacher_runs"] = tsubs
	}
	c.JSON(http.StatusOK, resp)
}

// updateAssignment: PUT /api/assignments/:id
func updateAssignment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(id, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	// Debug: log the entire request body
	bodyBytes, _ := c.GetRawData()
	fmt.Printf("updateAssignment received body: %s\n", string(bodyBytes))
	// Re-inject the body for ShouldBindJSON
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req struct {
		Title              string   `json:"title" binding:"required"`
		Description        string   `json:"description"`
		Deadline           string   `json:"deadline" binding:"required"`
		MaxPoints          int      `json:"max_points" binding:"required"`
		GradingPolicy      string   `json:"grading_policy" binding:"required"`
		ShowTraceback      bool     `json:"show_traceback"`
		ManualReview       bool     `json:"manual_review"`
		LLMInteractive     bool     `json:"llm_interactive"`
		LLMFeedback        bool     `json:"llm_feedback"`
		LLMAutoAward       bool     `json:"llm_auto_award"`
		LLMScenariosRaw    *string  `json:"llm_scenarios_json"`
		LLMStrictness      *int     `json:"llm_strictness"`
		LLMRubric          *string  `json:"llm_rubric"`
		LLMTeacherBaseline *string  `json:"llm_teacher_baseline_json"`
		SecondDeadline     *string  `json:"second_deadline"`
		LatePenaltyRatio   *float64 `json:"late_penalty_ratio"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dl, err := time.Parse(time.RFC3339Nano, req.Deadline)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deadline"})
		return
	}

	a := &Assignment{
		ID:              id,
		Title:           req.Title,
		Description:     req.Description,
		Deadline:        dl,
		MaxPoints:       req.MaxPoints,
		GradingPolicy:   req.GradingPolicy,
		ShowTraceback:   req.ShowTraceback,
		ManualReview:    req.ManualReview,
		LLMInteractive:  req.LLMInteractive,
		LLMFeedback:     req.LLMFeedback,
		LLMAutoAward:    req.LLMAutoAward,
		LLMScenariosRaw: req.LLMScenariosRaw,
	}
	if req.LLMStrictness != nil {
		a.LLMStrictness = *req.LLMStrictness
	} else {
		a.LLMStrictness = 50
	}
	if req.LLMRubric != nil {
		a.LLMRubric = req.LLMRubric
	}
	if req.LLMTeacherBaseline != nil {
		a.LLMTeacherBaseline = req.LLMTeacherBaseline
	}
	if req.SecondDeadline != nil {
		dl, err := time.Parse(time.RFC3339Nano, *req.SecondDeadline)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid second deadline"})
			return
		}
		a.SecondDeadline = &dl
	}
	if req.LatePenaltyRatio != nil {
		a.LatePenaltyRatio = *req.LatePenaltyRatio
	}
	if err := UpdateAssignment(a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update"})
		return
	}
	c.JSON(http.StatusOK, a)
}

// deleteAssignment: DELETE /api/assignments/:id
func deleteAssignment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(id, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	if err := DeleteAssignment(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete"})
		return
	}
	c.Status(http.StatusNoContent)
}

// publishAssignment: PUT /api/assignments/:id/publish
func publishAssignment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(id, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	if err := SetAssignmentPublished(id, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	queueAssignmentPublishedEmail(id)
	c.Status(http.StatusNoContent)
}

// uploadTemplate: POST /api/assignments/:id/template
func uploadTemplate(c *gin.Context) {
	aid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}
	if err := os.MkdirAll("templates", 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}
	path := fmt.Sprintf("templates/%d_%s", aid, filepath.Base(file.Filename))
	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot save"})
		return
	}
	if err := UpdateAssignmentTemplate(aid, &path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// uploadUnitTests: POST /api/assignments/:id/tests/upload
func uploadUnitTests(c *gin.Context) {
	aid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read"})
		return
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	methods := parseUnittestMethods(string(data))
	if len(methods) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no tests found"})
		return
	}
	for _, m := range methods {
		code := string(data)
		name := m
		tc := &TestCase{AssignmentID: aid, Weight: 1, Stdin: "", ExpectedStdout: "", UnittestCode: &code, UnittestName: &name}
		if err := CreateTestCase(tc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
	}
	c.Status(http.StatusCreated)
}

func parseUnittestMethods(src string) []string {
	lines := strings.Split(src, "\n")
	classRE := regexp.MustCompile(`^class\s+(\w+)\(.*unittest\.TestCase.*\):`)
	methodRE := regexp.MustCompile(`^\s*def\s+(test_[a-zA-Z0-9_]+)\s*\(`)
	var methods []string
	var current string
	var indent int
	for _, line := range lines {
		if m := classRE.FindStringSubmatch(line); m != nil {
			current = m[1]
			indent = len(line) - len(strings.TrimLeft(line, " \t"))
			continue
		}
		if current != "" {
			if len(line)-len(strings.TrimLeft(line, " \t")) <= indent && strings.TrimSpace(line) != "" {
				current = ""
				continue
			}
			if m := methodRE.FindStringSubmatch(line); m != nil {
				methods = append(methods, current+"."+m[1])
			}
		}
	}
	return methods
}

// generateAITests: POST /api/assignments/:id/tests/ai-generate
// Calls an LLM (default: GPT-5 via OpenAI API) to generate a Python unittest file
// and a corresponding builder-friendly JSON plan from the assignment title/description.
func generateAITests(c *gin.Context) {
	aid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// Only teachers/admins and must own the assignment if teacher
	if role := c.GetString("role"); role == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	assign, err := GetAssignment(aid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "assignment not found"})
		return
	}

	var req struct {
		Instructions string `json:"instructions"`
		NumTests     int    `json:"num_tests"`
		AutoTests    bool   `json:"auto_tests"`
	}
	_ = c.ShouldBindJSON(&req)
	if !req.AutoTests && req.NumTests <= 0 {
		req.NumTests = 5
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if strings.TrimSpace(apiKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OPENAI_API_KEY not configured on server"})
		return
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5"
	}
	base := os.Getenv("OPENAI_API_BASE")
	if base == "" {
		base = "https://api.openai.com/v1"
	}

	// Build prompt
	sys := "You are an expert Python educator and testing assistant. Generate high-quality unit tests."
	// When auto_tests is enabled, we let the model decide how many tests are needed.
	constraint := ""
	if req.AutoTests {
		constraint = "- Decide the appropriate number of test methods to ensure thorough coverage (typical cases, edge cases, and error handling)."
	} else {
		constraint = fmt.Sprintf("- Cover edge cases and typical cases. Add at least %d test methods.", req.NumTests)
	}
	basePrompt := `Create a Python unittest module for the following programming assignment.

Constraints:
- Use Python's unittest module and a single test class.
- Each test must call student_code(...) to execute the student's program, passing input values as separate arguments. student_code returns the program's stdout string without trailing newlines.
- Prefer small, independent tests. Avoid flaky or slow tests.
%s

Return a single JSON object with fields:
{
  "python": "<full .py file contents>",
  "builder": {
    "class_name": "<TestClassName>",
    "tests": [
      {
        "name": "test_...",
        "description": "...",
        "weight": "1",
        "timeLimit": "1",
        "assertions": [
          // Allowed assertion objects (match exactly these shapes):
          {"kind": "equals", "args": ["..."], "expected": "..."},
          {"kind": "notEquals", "args": ["..."], "expected": "..."},
          {"kind": "contains", "args": ["..."], "expected": "..."},
          {"kind": "notContains", "args": ["..."], "expected": "..."},
          {"kind": "regex", "args": ["..."], "pattern": "^...$"},
          {"kind": "raises", "args": ["..."], "exception": "ValueError"},
          {"kind": "custom", "code": "self.assertTrue(...)"}
        ]
      }
    ]
  }
}

Assignment title: %s
Assignment description:\n%s

Additional guidance (optional): %s
`
	user := fmt.Sprintf(basePrompt, constraint, assign.Title, assign.Description, req.Instructions)

	// Call OpenAI Chat Completions
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": user},
		},
	}
	body, _ := json.Marshal(payload)
	endpoint := strings.TrimRight(base, "/") + "/chat/completions"
	reqHTTP, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	reqHTTP.Header.Set("Authorization", "Bearer "+apiKey)
	reqHTTP.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(reqHTTP)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "llm request failed"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("llm error: %s", strings.TrimSpace(string(data)))})
		return
	}
	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil || len(raw.Choices) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid llm response"})
		return
	}
	content := strings.TrimSpace(raw.Choices[0].Message.Content)

	// Try to parse as JSON bundle
	var bundle struct {
		Python  string `json:"python"`
		Builder struct {
			ClassName string          `json:"class_name"`
			Tests     json.RawMessage `json:"tests"`
		} `json:"builder"`
	}
	parsed := json.Unmarshal([]byte(content), &bundle) == nil && strings.TrimSpace(bundle.Python) != ""
	if !parsed {
		// try to extract fenced JSON``` blocks
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start >= 0 && end > start {
			_ = json.Unmarshal([]byte(content[start:end+1]), &bundle)
			parsed = strings.TrimSpace(bundle.Python) != ""
		}
	}

	if !parsed {
		// Fall back: return raw content as python code
		bundle.Python = content
		bundle.Builder.ClassName = "TestAssignment"
		bundle.Builder.Tests = json.RawMessage([]byte("[]"))
	}

	c.JSON(http.StatusOK, gin.H{
		"python": bundle.Python,
		"builder": gin.H{
			"class_name": bundle.Builder.ClassName,
			"tests":      json.RawMessage(bundle.Builder.Tests),
		},
	})
}

// getTemplate: GET /api/assignments/:id/template
func getTemplate(c *gin.Context) {
	aid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := GetAssignment(aid)
	if err != nil || a.TemplatePath == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	role := c.GetString("role")
	if role == "student" {
		if ok, err := IsStudentOfAssignment(aid, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else if role == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	c.FileAttachment(*a.TemplatePath, filepath.Base(*a.TemplatePath))
}

// ──────────────────────────────────────────────────────────────────────────────
// Per-student deadline extensions
// ──────────────────────────────────────────────────────────────────────────────

// listAssignmentExtensions: GET /api/assignments/:id/extensions
func listAssignmentExtensions(c *gin.Context) {
    aid, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
        return
    }
    // Only the owning teacher/admin may view
    if c.GetString("role") == "teacher" {
        if ok, err := IsTeacherOfAssignment(aid, getUserID(c)); err != nil || !ok {
            c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
            return
        }
    }
    list, err := ListDeadlineOverridesForAssignment(aid)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
        return
    }
    c.JSON(http.StatusOK, list)
}

// upsertAssignmentExtension: PUT /api/assignments/:id/extensions/:student_id
func upsertAssignmentExtension(c *gin.Context) {
    aid, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
        return
    }
    sid, err := uuid.Parse(c.Param("student_id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid student id"})
        return
    }
    if c.GetString("role") == "teacher" {
        if ok, err := IsTeacherOfAssignment(aid, getUserID(c)); err != nil || !ok {
            c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
            return
        }
    }
    // Ensure student is enrolled in assignment's class
    if ok, err := IsStudentOfAssignment(aid, sid); err != nil || !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "student not in class"})
        return
    }
    var req struct {
        NewDeadline string  `json:"new_deadline" binding:"required"`
        Note        *string `json:"note"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    dl, err := time.Parse(time.RFC3339Nano, req.NewDeadline)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deadline"})
        return
    }
    if err := UpsertDeadlineOverride(aid, sid, dl, req.Note, getUserID(c)); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
        return
    }
    c.Status(http.StatusNoContent)
}

// deleteAssignmentExtension: DELETE /api/assignments/:id/extensions/:student_id
func deleteAssignmentExtension(c *gin.Context) {
    aid, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
        return
    }
    sid, err := uuid.Parse(c.Param("student_id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid student id"})
        return
    }
    if c.GetString("role") == "teacher" {
        if ok, err := IsTeacherOfAssignment(aid, getUserID(c)); err != nil || !ok {
            c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
            return
        }
    }
    if err := DeleteDeadlineOverride(aid, sid); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
        return
    }
    c.Status(http.StatusNoContent)
}

// createTestCase: POST /api/assignments/:id/tests
func createTestCase(c *gin.Context) {
	aid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Stdin          string  `json:"stdin" binding:"required"`
		ExpectedStdout string  `json:"expected_stdout" binding:"required"`
		Weight         float64 `json:"weight"`
		TimeLimitSec   float64 `json:"time_limit_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Weight == 0 {
		req.Weight = 1
	}
	tc := &TestCase{
		AssignmentID:   aid,
		Stdin:          req.Stdin,
		ExpectedStdout: req.ExpectedStdout,
		Weight:         req.Weight,
		TimeLimitSec:   req.TimeLimitSec,
	}
	if err := CreateTestCase(tc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusCreated, tc)
}

// updateTestCase: PUT /api/tests/:id
func updateTestCase(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Stdin          string  `json:"stdin"`
		ExpectedStdout string  `json:"expected_stdout"`
		Weight         float64 `json:"weight"`
		TimeLimitSec   float64 `json:"time_limit_sec"`
		UnittestCode   *string `json:"unittest_code"`
		UnittestName   *string `json:"unittest_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Weight == 0 {
		req.Weight = 1
	}
	tc := &TestCase{ID: id, Stdin: req.Stdin, ExpectedStdout: req.ExpectedStdout, Weight: req.Weight, TimeLimitSec: req.TimeLimitSec, UnittestCode: req.UnittestCode, UnittestName: req.UnittestName}
	if err := UpdateTestCase(tc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, tc)
}

// deleteTestCase: DELETE /api/tests/:id
func deleteTestCase(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := DeleteTestCase(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// deleteAllTestCases: DELETE /api/assignments/:id/tests
func deleteAllTestCases(c *gin.Context) {
	aid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	if err := DeleteAllTestCasesForAssignment(aid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// createSubmission: POST /api/assignments/:id/submissions
func createSubmission(c *gin.Context) {
	aid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var tmp int
	if err := DB.Get(&tmp, `SELECT 1 FROM assignments a JOIN class_students cs ON cs.class_id=a.class_id WHERE a.id=$1 AND cs.student_id=$2`, aid, getUserID(c)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form"})
		return
	}
	var files []*multipart.FileHeader
	if c.Request.MultipartForm != nil {
		files = c.Request.MultipartForm.File["files"]
	}
	if len(files) == 0 {
		// fallback to single "file" field for backwards compatibility
		if f, err := c.FormFile("file"); err == nil {
			files = []*multipart.FileHeader{f}
		}
	}
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files"})
		return
	}
	if err := os.MkdirAll("uploads", 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}

	tmpDir, err := os.MkdirTemp(execRoot, "upload-")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}
	defer os.RemoveAll(tmpDir)

	for _, fh := range files {
		dst := filepath.Join(tmpDir, filepath.Base(fh.Filename))
		if err := c.SaveUploadedFile(fh, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot save"})
			return
		}
	}

	// Ensure uploaded files are readable by the container user
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
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
	_ = ensureSandboxPerms(tmpDir)

	// Ensure uploaded files are readable by the container user.
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
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
	_ = ensureSandboxPerms(tmpDir)

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel := filepath.Base(path)
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "zip failed"})
		return
	}
	if err := zw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "zip failed"})
		return
	}

	name := fmt.Sprintf("%d_%d_%d.zip", aid, getUserID(c), time.Now().UnixNano())
	path := filepath.Join("uploads", name)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot save"})
		return
	}

	sub := &Submission{
		AssignmentID: aid,
		StudentID:    getUserID(c),
		CodePath:     path,
		CodeContent:  base64.StdEncoding.EncodeToString(buf.Bytes()),
	}
	if err := CreateSubmission(sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	// enqueue for grading unless manual review is enabled (unless LLM interactive is on)
	if a, err := GetAssignment(aid); err == nil {
		if a.LLMInteractive || !a.ManualReview {
			EnqueueJob(Job{SubmissionID: sub.ID})
		}
	}
	c.JSON(http.StatusCreated, sub)
}

// runTeacherSolution: POST /api/assignments/:id/solution-run
// Allows a teacher/admin to upload a reference solution and run all tests.
// Does not persist a submission or results; returns a summary JSON immediately.
func runTeacherSolution(c *gin.Context) {
	aid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// only teachers/admins; for teachers require ownership OR that the assignment is shared in Teachers' group
	if role := c.GetString("role"); role == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, getUserID(c)); err != nil || !ok {
			// Allow if the assignment is referenced in the Teachers' group tree
			var x int
			if err := DB.Get(&x, `SELECT 1 FROM class_files WHERE class_id=$1 AND assignment_id=$2 LIMIT 1`, TeacherGroupID, aid); err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form"})
		return
	}
	var files []*multipart.FileHeader
	if c.Request.MultipartForm != nil {
		files = c.Request.MultipartForm.File["files"]
	}
	if len(files) == 0 {
		if f, ferr := c.FormFile("file"); ferr == nil {
			files = []*multipart.FileHeader{f}
		}
	}
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files"})
		return
	}

	tmpDir, err := os.MkdirTemp(execRoot, "teacher-solution-")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}
	defer os.RemoveAll(tmpDir)

	for _, fh := range files {
		dst := filepath.Join(tmpDir, filepath.Base(fh.Filename))
		if err := c.SaveUploadedFile(fh, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot save"})
			return
		}
	}

	// Detect main file similarly to worker
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
			if mainGuard.Match(content) {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "no python files found"})
		return
	}

	// Ensure staged code is world-readable so the unprivileged container user can access it
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
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
	_ = ensureSandboxPerms(tmpDir)

        tests, err := ListTestCases(aid)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
                return
        }

        assignment, err := GetAssignment(aid)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
                return
        }

	// Execute all tests and gather results without persisting
	results := make([]map[string]any, 0, len(tests))
	passed := 0
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

		if status == "passed" {
			passed++
			earnedWeight += tc.Weight
		}
		totalWeight += tc.Weight

		item := map[string]any{
			"test_case_id":    tc.ID,
			"unittest_name":   tc.UnittestName,
			"status":          status,
			"runtime_ms":      int(runtime.Milliseconds()),
			"exit_code":       exitCode,
			"actual_stdout":   stdout,
			"expected_stdout": tc.ExpectedStdout,
			"stderr":          stderr,
		}
		results = append(results, item)
	}

	// Persist this run as a teacher submission for later viewing
	// Zip uploaded files in-memory similar to student submission
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel := filepath.Base(path)
		w, e := zw.Create(rel)
		if e != nil {
			return e
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		_, e = w.Write(data)
		return e
	})
	_ = zw.Close()
	_ = os.MkdirAll("uploads", 0755)
	name := fmt.Sprintf("%d_%d_%d_teacher.zip", aid, getUserID(c), time.Now().UnixNano())
	path := filepath.Join("uploads", name)
	_ = os.WriteFile(path, buf.Bytes(), 0644)
	sub := &Submission{AssignmentID: aid, StudentID: getUserID(c), CodePath: path, CodeContent: base64.StdEncoding.EncodeToString(buf.Bytes()), IsTeacherRun: true}
	// Insert without enrollment requirement by bypassing CreateSubmission if teacher
	if c.GetString("role") == "teacher" || c.GetString("role") == "admin" {
		// direct insert
		_ = DB.QueryRow(`INSERT INTO submissions (assignment_id, student_id, code_path, code_content, is_teacher_run)
                          VALUES ($1,$2,$3,$4,TRUE) RETURNING id, status, created_at, updated_at`,
			sub.AssignmentID, sub.StudentID, sub.CodePath, sub.CodeContent).Scan(&sub.ID, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt)
	} else {
		_ = CreateSubmission(sub)
	}
	// Save per-test results to DB (so later details are available)
	for i, tc := range tests {
		item := results[i]
		r := &Result{SubmissionID: sub.ID, TestCaseID: tc.ID, Status: item["status"].(string), ActualStdout: fmt.Sprint(item["actual_stdout"]), Stderr: fmt.Sprint(item["stderr"]), ExitCode: item["exit_code"].(int), RuntimeMS: item["runtime_ms"].(int)}
		_ = CreateResult(r)
	}

        // Compute and persist overall status and points similar to worker
        allPass := passed == len(tests)
        if !assignment.LLMInteractive {
                score := 0.0
                switch assignment.GradingPolicy {
                case "all_or_nothing":
                        if allPass {
                                score = float64(assignment.MaxPoints)
                        }
                case "weighted":
                        // normalize to MaxPoints
                        if totalWeight > 0 {
                                score = earnedWeight * (float64(assignment.MaxPoints) / totalWeight)
                        }
                }

                // Handle late submission logic with second deadline
                if sub.CreatedAt.After(assignment.Deadline) {
                        _ = SetSubmissionLate(sub.ID, true)

                        // Check if there's a second deadline and submission is within it
                        if assignment.SecondDeadline != nil && sub.CreatedAt.Before(*assignment.SecondDeadline) {
                                // Apply penalty ratio for second deadline submissions
                                score = score * assignment.LatePenaltyRatio
                        } else {
                                // No second deadline or submission is after second deadline - no points
                                score = 0.0
                        }
                }

                _ = SetSubmissionPoints(sub.ID, score)
                if allPass {
                        _ = UpdateSubmissionStatus(sub.ID, "completed")
                } else {
                        _ = UpdateSubmissionStatus(sub.ID, "failed")
                }
        }

	// Save teacher baseline (plan+results) on assignment so student runs can use it as standard
	baseline := map[string]any{
		"tests":      results,
		"summary":    map[string]any{"total": len(tests), "passed": passed},
		"created_at": time.Now().Format(time.RFC3339Nano),
	}
        if b, e := json.Marshal(baseline); e == nil {
                s := string(b)
                assignment.LLMTeacherBaseline = &s
                _ = UpdateAssignment(assignment)
        }

        resp := gin.H{
                "submission_id": sub.ID,
                "total":         len(tests),
                "passed":        passed,
                "failed":        len(tests) - passed,
                "results":       results,
        }

        if assignment.LLMInteractive {
                UpdateSubmissionStatus(sub.ID, "running")
                runLLMInteractive(sub, assignment)
                if llm, err := GetLatestLLMRun(sub.ID); err == nil && llm != nil {
                        resp["llm"] = llm
                }
        }

        c.JSON(http.StatusOK, resp)
}

// getSubmission: GET /api/submissions/:id
func getSubmission(c *gin.Context) {
	sid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	sub, err := GetSubmission(sid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if c.GetString("role") == "student" && getUserID(c) != sub.StudentID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	results, _ := ListResultsForSubmission(sid)
	if c.GetString("role") == "student" {
		if a, err := GetAssignmentForSubmission(sub.ID); err == nil && !a.ShowTraceback {
			for i := range results {
				results[i].Stderr = ""
			}
		}
	}
	resp := gin.H{"submission": sub, "results": results}
	// Attach latest LLM run if available
	if llm, err := GetLatestLLMRun(sid); err == nil && llm != nil {
		// apply feedback visibility for students
		if role := c.GetString("role"); role == "student" {
			if a, e := GetAssignmentForSubmission(sub.ID); e == nil {
				if !a.LLMFeedback {
					// hide detailed artifacts for students if disabled
					llm.ReviewJSON = nil
					llm.Transcript = nil
				}
			}
		}
		resp["llm"] = llm
	}
	c.JSON(http.StatusOK, resp)
}

// acceptSubmission: PUT /api/submissions/:id/accept
// Allows a teacher/admin to manually accept a submission and optionally set points (capped to assignment max).
func acceptSubmission(c *gin.Context) {
	sid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := GetAssignmentForSubmission(sid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if role := c.GetString("role"); role == "teacher" {
		if ok, err := IsTeacherOfAssignment(a.ID, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	var req struct {
		Points *float64 `json:"points"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Cap points to [0, max_points]
	if req.Points != nil {
		p := *req.Points
		if p < 0 {
			p = 0
		}
		if p > float64(a.MaxPoints) {
			p = float64(a.MaxPoints)
		}
		*req.Points = p
		if err := SetSubmissionOverridePoints(sid, req.Points); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
	}
	// Mark as manually accepted and completed
	_ = SetSubmissionManualAccept(sid, true)
	if err := UpdateSubmissionStatus(sid, "completed"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// undoManualAccept: PUT /api/submissions/:id/undo-accept
// Allows a teacher/admin to undo manual acceptance of a submission.
func undoManualAccept(c *gin.Context) {
	sid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := GetAssignmentForSubmission(sid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if role := c.GetString("role"); role == "teacher" {
		if ok, err := IsTeacherOfAssignment(a.ID, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	// Mark as not manually accepted
	_ = SetSubmissionManualAccept(sid, false)

	// Reset status to failed if it was manually accepted
	// This will allow the worker to re-grade the submission
	if err := UpdateSubmissionStatus(sid, "failed"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}

	c.Status(http.StatusNoContent)
}

// submissionTerminalWS: GET /api/submissions/:id/terminal (WS)
// Upgrades to a websocket and bridges an interactive shell inside a Docker
// container seeded with the submission's files. Teacher/admin only; also
// validates teacher owns the assignment's class if teacher role.
func submissionTerminalWS(c *gin.Context) {
	sid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	sub, err := GetSubmission(sid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	a, err := GetAssignmentForSubmission(sid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "assignment not found"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(a.ID, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	// Upgrade to WebSocket early so client doesn't time out while we prepare
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := up.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade failed for submission %s: %v", sid, err)
		// If upgrade fails, try to return a plain error for easier diagnosis
		c.Status(http.StatusBadRequest)
		return
	}
	wsFail := func(msg string) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("ERROR: "+msg))
		_ = conn.Close()
	}

	// Decode submission archive to a temp dir
	tmpDir, err := os.MkdirTemp(execRoot, "term-sub-")
	if err != nil {
		wsFail("server error")
		return
	}
	// Ensure cleanup later
	// Note: cannot defer here because we'll upgrade connection and hold.

	// Try to decode either base64 zip or plain text
	data, berr := base64.StdEncoding.DecodeString(sub.CodeContent)
	var isZip bool
	if berr == nil && len(data) > 4 && (string(data[:2]) == "PK") {
		isZip = true
	}
	if isZip {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			os.RemoveAll(tmpDir)
			wsFail("invalid zip")
			return
		}
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			dst := filepath.Join(tmpDir, filepath.Base(f.Name))
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = os.WriteFile(dst, b, 0644)
			_ = rc.Close()
		}
	} else {
		// write single file main.py
		b := data
		if berr != nil {
			b = []byte(sub.CodeContent)
		}
		_ = os.WriteFile(filepath.Join(tmpDir, "main.py"), b, 0644)
	}
	// Permissions for container user
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
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
	_ = ensureSandboxPerms(tmpDir)

	// Prepare docker run with TTY and bash
	abs, _ := filepath.Abs(tmpDir)
	// Ensure image exists to avoid interactive sessions hanging on pulls
	_ = ensureDockerImage(pythonImage)
	// Configure a clean prompt and disable bracketed paste to avoid stray sequences
	// We also set a custom INPUTRC in /tmp to tweak readline behaviour and prevent rc files
	bootstrap := strings.Join([]string{
		"cd /code",
		// Write a minimal inputrc to /tmp and use it for this shell
		"printf 'set enable-bracketed-paste off\nset show-all-if-ambiguous on\nset completion-ignore-case on\n' > /tmp/inputrc",
		"export INPUTRC=/tmp/inputrc",
		// Set a minimal prompt and disable PROMPT_COMMAND hooks.
		"export PS1='~% '",
		"unset PROMPT_COMMAND",
		// Start interactive bash without sourcing system/user rc that may override PS1
		"exec bash --noprofile --norc -i",
	}, " && ")

	// Give the container a unique and Docker-safe name so we can force-remove it on session end
	// Use the UUID string (lowercase) and ensure only allowed characters
	safeID := strings.ToLower(sub.ID.String())
	// UUID contains only [0-9a-f-], which are allowed by Docker names
	containerName := fmt.Sprintf("term-%s-%d", safeID, time.Now().UnixNano())

	cmd := exec.Command("docker", "run",
		"--rm",
		"--name", containerName,
		"-it",
		"--network=none",
		"--user", dockerUser,
		"--cpus", dockerCPUs,
		"--memory", dockerMemory,
		"--pids-limit", "128",
		"--read-only",
		"--cap-drop=ALL",
		"--security-opt", "no-new-privileges",
		"--security-opt", "label=disable",
		"--mount", "type=tmpfs,destination=/tmp,tmpfs-size=16m",
		"-e", "PS1=~% ",
		"-e", "PROMPT_COMMAND=",
		"-v", fmt.Sprintf("%s:/code:rw", abs),
		pythonImage, "bash", "-lc", bootstrap)

	// PTY for interactive session
	ptyFile, err := pty.Start(cmd)
	if err != nil {
		os.RemoveAll(tmpDir)
		wsFail("container start failed")
		return
	}

	// IO pump: container stdout -> ws, ws -> container stdin
	done := make(chan struct{})
	var once sync.Once
	safeClose := func() { once.Do(func() { close(done) }) }
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptyFile.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		safeClose()
	}()

	go func() {
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if mt == websocket.TextMessage || mt == websocket.BinaryMessage {
				_, _ = ptyFile.Write(msg)
			}
		}
		safeClose()
	}()

	// Wait for termination
	<-done
	_ = conn.Close()
	_ = ptyFile.Close()
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	// Best-effort force removal of the named container to avoid leftovers
	_ = exec.Command("docker", "rm", "-f", containerName).Run()
	// cleanup temp files
	go func() { time.Sleep(2 * time.Second); _ = os.RemoveAll(tmpDir) }()
	log.Println("terminal session closed for submission", sub.ID)
}

// submissionRunWS: GET /api/submissions/:id/run (WS)
// A simplified run session for manual review: each "execute" message starts a fresh
// docker container that runs the student's Python program and streams stdout/stderr
// back to the client. The client can send "input" messages to forward stdin and
// "stop" to terminate the current run. Errors are sent as JSON messages.
func submissionRunWS(c *gin.Context) {
	sid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	sub, err := GetSubmission(sid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	a, err := GetAssignmentForSubmission(sid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "assignment not found"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(a.ID, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	// Upgrade WS
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := up.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade failed for submission %s: %v", sid, err)
		c.Status(http.StatusBadRequest)
		return
	}

	// Use a stable per-submission key based on UUID string
	sessionKey := fmt.Sprintf("sub-%s", strings.ToLower(sub.ID.String()))

	// channel to serialize writes
	ch := make(chan map[string]any, 128)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case m := <-ch:
				b, _ := json.Marshal(m)
				if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
					close(done)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// attach to session
	runSessionsMu.Lock()
	sess, exists := runSessions[sessionKey]
	if !exists {
		sess = &RunSession{TTL: 60 * time.Second, Subs: make(map[chan map[string]any]struct{})}
		runSessions[sessionKey] = sess
	}
	sess.Mu.Lock()
	sess.AttachCount++
	sess.LastActive = time.Now()
	if sess.Timer != nil {
		sess.Timer.Stop()
		sess.Timer = nil
	}
	sess.Subs[ch] = struct{}{}
	// replay
	if len(sess.BufOut) > 0 {
		ch <- map[string]any{"type": "stdout", "data": string(sess.BufOut)}
	}
	if len(sess.BufErr) > 0 {
		ch <- map[string]any{"type": "stderr", "data": string(sess.BufErr)}
	}
	if sess.Running {
		ch <- map[string]any{"type": "started"}
	} else if sess.Ended {
		ch <- map[string]any{"type": "exit", "code": sess.ExitCode, "timedOut": sess.TimedOut}
	}
	sess.Mu.Unlock()
	runSessionsMu.Unlock()

	broadcast := func(m map[string]any) {
		runSessionsMu.Lock()
		s := runSessions[sessionKey]
		runSessionsMu.Unlock()
		if s == nil {
			return
		}
		s.Mu.Lock()
		for subCh := range s.Subs {
			select {
			case subCh <- m:
			default:
			}
		}
		s.Mu.Unlock()
	}

	// helper to stage code into tmp dir once per session
	ensureTmp := func() (string, error) {
		runSessionsMu.Lock()
		s := runSessions[sessionKey]
		runSessionsMu.Unlock()
		if s == nil {
			return "", fmt.Errorf("no session")
		}
		s.Mu.Lock()
		td := s.TmpDir
		s.Mu.Unlock()
		if td != "" {
			return td, nil
		}
		tmpDir, err := os.MkdirTemp(execRoot, "run-sub-")
		if err != nil {
			return "", err
		}
		// decode
		data, berr := base64.StdEncoding.DecodeString(sub.CodeContent)
		isZip := berr == nil && len(data) > 4 && (string(data[:2]) == "PK")
		if isZip {
			zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				os.RemoveAll(tmpDir)
				return "", fmt.Errorf("invalid zip")
			}
			for _, f := range zr.File {
				if f.FileInfo().IsDir() {
					continue
				}
				dst := filepath.Join(tmpDir, filepath.Base(f.Name))
				rc, _ := f.Open()
				b, _ := io.ReadAll(rc)
				_ = rc.Close()
				_ = os.WriteFile(dst, b, 0644)
			}
		} else {
			b := data
			if berr != nil {
				b = []byte(sub.CodeContent)
			}
			_ = os.WriteFile(filepath.Join(tmpDir, "main.py"), b, 0644)
		}
		_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
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
		runSessionsMu.Lock()
		s = runSessions[sessionKey]
		if s != nil {
			s.Mu.Lock()
			s.TmpDir = tmpDir
			s.Mu.Unlock()
		}
		runSessionsMu.Unlock()
		return tmpDir, nil
	}

	// read loop from client
	for {
		_, raw, rerr := conn.ReadMessage()
		if rerr != nil {
			break
		}
		var m struct {
			Type      string `json:"type"`
			Data      string `json:"data,omitempty"`
			TimeoutMs *int   `json:"timeout_ms,omitempty"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			ch <- map[string]any{"type": "error", "message": "invalid message"}
			continue
		}
		switch m.Type {
		case "execute":
			// stop existing if any, then start a fresh container
			runSessionsMu.Lock()
			s := runSessions[sessionKey]
			runSessionsMu.Unlock()
			if s == nil {
				ch <- map[string]any{"type": "error", "message": "session not found"}
				continue
			}
			s.Mu.Lock()
			if s.Running {
				if s.Cmd != nil && s.Cmd.Process != nil {
					_ = s.Cmd.Process.Kill()
				}
				if s.ContainerName != "" {
					_ = exec.Command("docker", "rm", "-f", s.ContainerName).Run()
				}
				if s.GuiContainerName != "" {
					_ = exec.Command("docker", "rm", "-f", s.GuiContainerName).Run()
				}
			}
			s.BufOut = nil
			s.BufErr = nil
			s.Ended = false
			s.TimedOut = false
			s.ExitCode = 0
			s.GuiEnabled = false
			s.GuiContainerName = ""
			s.GuiHostPort = 0
			s.Mu.Unlock()

			td, terr := ensureTmp()
			if terr != nil {
				ch <- map[string]any{"type": "error", "message": terr.Error()}
				continue
			}
			abs, _ := filepath.Abs(td)

			// detect main file
			var mainFile, firstPy string
			_ = filepath.Walk(td, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasSuffix(info.Name(), ".py") {
					rel, _ := filepath.Rel(td, path)
					if firstPy == "" {
						firstPy = rel
					}
					content, _ := os.ReadFile(path)
					if mainGuard.Match(content) {
						mainFile = rel
						return io.EOF
					}
				}
				return nil
			})
			if mainFile == "" {
				if _, err := os.Stat(filepath.Join(td, "main.py")); err == nil {
					mainFile = "main.py"
				} else {
					mainFile = firstPy
				}
			}
			if mainFile == "" {
				ch <- map[string]any{"type": "error", "message": "no python files found"}
				continue
			}

			// detect if submission likely uses Tkinter and needs GUI
			guiWanted := false
			_ = filepath.Walk(td, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".py") {
					return nil
				}
				b, _ := os.ReadFile(path)
				text := strings.ToLower(string(b))
				if strings.Contains(text, "import tkinter") || strings.Contains(text, "from tkinter import") {
					guiWanted = true
					return io.EOF
				}
				return nil
			})

			// choose mode: GUI vs headless
			if guiWanted {
				// Start a GUI-capable container exposing noVNC on a random localhost port
				hostPort := 0
				ln, lerr := net.Listen("tcp", "127.0.0.1:0")
				if lerr == nil {
					hostPort = ln.Addr().(*net.TCPAddr).Port
					_ = ln.Close()
				}
				if hostPort == 0 {
					ch <- map[string]any{"type": "error", "message": "no free port for GUI"}
					continue
				}
				safeID := strings.ToLower(sub.ID.String())
				containerName := fmt.Sprintf("gui-%s-%d", safeID, time.Now().UnixNano())
				// Supervisord-inspired approach for robust process management
				sup := strings.Join([]string{
					"[supervisord]",
					"nodaemon=true",
					"",
					"[program:xvfb]",
					"command=/usr/bin/Xvfb :0 -screen 0 1280x800x24 -nolisten tcp",
					"priority=10",
					"autorestart=true",
					"",
					"[program:wm]",
					"command=/usr/bin/fluxbox",
					"environment=DISPLAY=\":0\"",
					"priority=20",
					"autorestart=true",
					"",
					"[program:vnc]",
					"command=/usr/bin/x11vnc -display :0 -forever -shared -nopw -rfbport 5900 -repeat",
					"priority=30",
					"autorestart=true",
					"",
					"[program:web]",
					"command=/usr/bin/websockify --web=/usr/share/novnc 6080 localhost:5900",
					"priority=35",
					"autorestart=true",
					"",
					"[program:app]",
					fmt.Sprintf("command=/usr/local/bin/python /code/%s", strings.ReplaceAll(mainFile, "'", "'\\''")),
					"directory=/code",
					"environment=DISPLAY=\":0\"",
					"priority=40",
					"autorestart=false",
				}, "\n")
				script := fmt.Sprintf(strings.Join([]string{
					"export DEBIAN_FRONTEND=noninteractive",
					"apt-get update >/dev/null 2>&1 || true",
					"apt-get install -y --no-install-recommends xvfb x11vnc fluxbox novnc websockify python3-tk supervisor >/dev/null 2>&1 || true",
					"rm -rf /var/lib/apt/lists/*",
					fmt.Sprintf("cat > /tmp/supervisord.conf << 'EOF'\n%s\nEOF", sup),
					"/usr/bin/supervisord -c /tmp/supervisord.conf",
				}, "\n"))
				cmd := exec.Command("docker", "run", "--rm", "--name", containerName,
					"-p", fmt.Sprintf("127.0.0.1:%d:6080", hostPort),
					"--cpus", dockerCPUs, "--memory", dockerMemory,
					"--security-opt", "label=disable",
					"-v", fmt.Sprintf("%s:/code:ro", abs),
					pythonImage, "bash", "-lc", script)
				stdoutPipe, e1 := cmd.StdoutPipe()
				stderrPipe, e2 := cmd.StderrPipe()
				if e1 != nil || e2 != nil {
					ch <- map[string]any{"type": "error", "message": "container start failed"}
					continue
				}
				if err := cmd.Start(); err != nil {
					ch <- map[string]any{"type": "error", "message": "container start failed"}
					continue
				}

				runSessionsMu.Lock()
				s = runSessions[sessionKey]
				runSessionsMu.Unlock()
				if s == nil {
					_ = cmd.Process.Kill()
					_ = exec.Command("docker", "rm", "-f", containerName).Run()
					continue
				}
				s.Mu.Lock()
				s.Cmd = cmd
				s.Stdin = nil
				s.ContainerName = containerName
				s.GuiContainerName = containerName
				s.GuiHostPort = hostPort
				s.GuiEnabled = true
				s.Running = true
				s.Ended = false
				s.LastActive = time.Now()
				s.Mu.Unlock()

				// Announce start, then probe noVNC HTTP before telling client to load GUI to avoid early 502s
				broadcast(map[string]any{"type": "started"})
				go func(port int, subID uuid.UUID) {
					url := fmt.Sprintf("http://127.0.0.1:%d/vnc.html", port)
					// give the container ample time to install GUI packages and start noVNC
					deadline := time.Now().Add(60 * time.Second)
					for time.Now().Before(deadline) {
						resp, err := http.Get(url)
						if err == nil {
							_, _ = io.Copy(io.Discard, resp.Body)
							_ = resp.Body.Close()
							if resp.StatusCode < 500 {
								// only announce GUI once the web interface is reachable
								broadcast(map[string]any{"type": "gui", "base": fmt.Sprintf("/api/submissions/%s/gui/", subID)})
								return
							}
						}
						time.Sleep(500 * time.Millisecond)
					}
					// if we get here the GUI never became ready; inform client instead of serving 502s
					broadcast(map[string]any{"type": "error", "message": "GUI failed to start"})
				}(hostPort, sub.ID)

				go func() {
					buf := make([]byte, 4096)
					for {
						n, rerr := stdoutPipe.Read(buf)
						if n > 0 {
							chunk := append([]byte(nil), buf[:n]...)
							runSessionsMu.Lock()
							s := runSessions[sessionKey]
							if s != nil {
								s.Mu.Lock()
								s.BufOut = append(s.BufOut, chunk...)
								s.Mu.Unlock()
							}
							runSessionsMu.Unlock()
							broadcast(map[string]any{"type": "stdout", "data": string(chunk)})
						}
						if rerr != nil {
							return
						}
					}
				}()
				go func() {
					buf := make([]byte, 4096)
					for {
						n, rerr := stderrPipe.Read(buf)
						if n > 0 {
							chunk := append([]byte(nil), buf[:n]...)
							runSessionsMu.Lock()
							s := runSessions[sessionKey]
							if s != nil {
								s.Mu.Lock()
								s.BufErr = append(s.BufErr, chunk...)
								s.Mu.Unlock()
							}
							runSessionsMu.Unlock()
							broadcast(map[string]any{"type": "stderr", "data": string(chunk)})
						}
						if rerr != nil {
							return
						}
					}
				}()
				go func() {
					err := cmd.Wait()
					exitCode := 0
					if err != nil {
						if ee, ok := err.(*exec.ExitError); ok {
							exitCode = ee.ExitCode()
						} else {
							exitCode = -1
						}
					}
					runSessionsMu.Lock()
					s := runSessions[sessionKey]
					runSessionsMu.Unlock()
					if s != nil {
						s.Mu.Lock()
						s.Running = false
						s.Ended = true
						s.ExitCode = exitCode
						s.Mu.Unlock()
					}
					broadcast(map[string]any{"type": "exit", "code": exitCode, "timedOut": false})
				}()
				break
			}

			// Headless mode (no GUI)
			safeID := strings.ToLower(sub.ID.String())
			containerName := fmt.Sprintf("run-%s-%d", safeID, time.Now().UnixNano())
			script := fmt.Sprintf("cd /code && python %s", strings.ReplaceAll(mainFile, "'", "'\\''"))
			// Ensure the image is available to avoid long pull hangs during interactive runs
			_ = ensureDockerImage(pythonImage)
			cmd := exec.Command("docker", "run", "--rm", "--name", containerName, "-i",
				"--network=none",
				"--user", dockerUser,
				"--cpus", dockerCPUs,
				"--memory", dockerMemory,
				"--memory-swap", dockerMemory,
				"--pids-limit", "128",
				"--read-only",
				"--cap-drop=ALL",
				"--security-opt", "no-new-privileges",
				"--security-opt", "label=disable",
				"--mount", "type=tmpfs,destination=/tmp,tmpfs-size=16m",
				"-v", fmt.Sprintf("%s:/code:ro", abs),
				pythonImage, "bash", "-lc", script)
			stdoutPipe, e1 := cmd.StdoutPipe()
			stderrPipe, e2 := cmd.StderrPipe()
			stdinPipe, e3 := cmd.StdinPipe()
			if e1 != nil || e2 != nil || e3 != nil {
				ch <- map[string]any{"type": "error", "message": "container start failed"}
				continue
			}
			if err := cmd.Start(); err != nil {
				ch <- map[string]any{"type": "error", "message": "container start failed"}
				continue
			}

			runSessionsMu.Lock()
			s = runSessions[sessionKey]
			runSessionsMu.Unlock()
			if s == nil {
				_ = cmd.Process.Kill()
				_ = exec.Command("docker", "rm", "-f", containerName).Run()
				continue
			}
			s.Mu.Lock()
			s.Cmd = cmd
			s.Stdin = stdinPipe
			s.ContainerName = containerName
			s.Running = true
			s.Ended = false
			s.LastActive = time.Now()
			s.Mu.Unlock()

			broadcast(map[string]any{"type": "started"})

			go func() {
				buf := make([]byte, 4096)
				for {
					n, rerr := stdoutPipe.Read(buf)
					if n > 0 {
						chunk := append([]byte(nil), buf[:n]...)
						runSessionsMu.Lock()
						s := runSessions[sessionKey]
						if s != nil {
							s.Mu.Lock()
							s.BufOut = append(s.BufOut, chunk...)
							s.Mu.Unlock()
						}
						runSessionsMu.Unlock()
						broadcast(map[string]any{"type": "stdout", "data": string(chunk)})
					}
					if rerr != nil {
						return
					}
				}
			}()
			go func() {
				buf := make([]byte, 4096)
				for {
					n, rerr := stderrPipe.Read(buf)
					if n > 0 {
						chunk := append([]byte(nil), buf[:n]...)
						runSessionsMu.Lock()
						s := runSessions[sessionKey]
						if s != nil {
							s.Mu.Lock()
							s.BufErr = append(s.BufErr, chunk...)
							s.Mu.Unlock()
						}
						runSessionsMu.Unlock()
						broadcast(map[string]any{"type": "stderr", "data": string(chunk)})
					}
					if rerr != nil {
						return
					}
				}
			}()
			go func() {
				err := cmd.Wait()
				exitCode := 0
				if err != nil {
					if ee, ok := err.(*exec.ExitError); ok {
						exitCode = ee.ExitCode()
					} else {
						exitCode = -1
					}
				}
				runSessionsMu.Lock()
				s := runSessions[sessionKey]
				runSessionsMu.Unlock()
				if s != nil {
					s.Mu.Lock()
					s.Running = false
					s.Ended = true
					s.ExitCode = exitCode
					s.Mu.Unlock()
				}
				broadcast(map[string]any{"type": "exit", "code": exitCode, "timedOut": false})
			}()

		case "input":
			runSessionsMu.Lock()
			s := runSessions[sessionKey]
			runSessionsMu.Unlock()
			if s != nil {
				s.Mu.Lock()
				in := s.Stdin
				s.LastActive = time.Now()
				s.Mu.Unlock()
				if in != nil {
					_, _ = io.WriteString(in, m.Data)
				}
			}
		case "stop":
			runSessionsMu.Lock()
			s := runSessions[sessionKey]
			runSessionsMu.Unlock()
			if s != nil {
				s.Mu.Lock()
				cmd := s.Cmd
				cname := s.ContainerName
				gname := s.GuiContainerName
				s.Mu.Unlock()
				if cmd != nil && cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				if cname != "" {
					_ = exec.Command("docker", "rm", "-f", cname).Run()
				}
				if gname != "" {
					_ = exec.Command("docker", "rm", "-f", gname).Run()
				}
				runSessionsMu.Lock()
				s = runSessions[sessionKey]
				if s != nil {
					s.Mu.Lock()
					s.Running = false
					s.Ended = true
					s.ExitCode = -1
					s.GuiEnabled = false
					s.GuiHostPort = 0
					s.GuiContainerName = ""
					s.Mu.Unlock()
				}
				runSessionsMu.Unlock()
				broadcast(map[string]any{"type": "exit", "code": -1, "timedOut": false})
			}
		default:
			ch <- map[string]any{"type": "error", "message": "unknown message type"}
		}
	}

	// detach on close, start TTL if no viewers
	runSessionsMu.Lock()
	s := runSessions[sessionKey]
	runSessionsMu.Unlock()
	if s != nil {
		s.Mu.Lock()
		delete(s.Subs, ch)
		s.AttachCount--
		zero := s.AttachCount <= 0
		s.LastActive = time.Now()
		if zero && s.Timer == nil {
			ttl := s.TTL
			s.Timer = time.AfterFunc(ttl, func() {
				runSessionsMu.Lock()
				ss := runSessions[sessionKey]
				runSessionsMu.Unlock()
				if ss == nil {
					return
				}
				ss.Mu.Lock()
				running := ss.Running
				cmd := ss.Cmd
				cname := ss.ContainerName
				tmp := ss.TmpDir
				ss.Running = false
				ss.Ended = true
				ss.TimedOut = true
				ss.ExitCode = -1
				ss.Mu.Unlock()
				if running && cmd != nil && cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				if cname != "" {
					_ = exec.Command("docker", "rm", "-f", cname).Run()
				}
				if tmp != "" {
					_ = os.RemoveAll(tmp)
				}
				runSessionsMu.Lock()
				delete(runSessions, sessionKey)
				runSessionsMu.Unlock()
			})
		}
		s.Mu.Unlock()
	}
	close(done)
	_ = conn.Close()
}

// ---- ADMIN adds a teacher ----
func createTeacher(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err := CreateTeacher(req.Email, string(hash), nil, nil); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "user exists"})
		return
	}
	c.Status(http.StatusCreated)
}

// ---- TEACHER creates a class ----
func createClass(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := &Class{Name: req.Name, TeacherID: getUserID(c)}
	if err := CreateClass(cl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusCreated, cl)
}

// ---- TEACHER renames a class ----
func updateClass(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	teacherID := uuid.Nil
	if c.GetString("role") == "teacher" {
		teacherID = getUserID(c)
	}
	if err := UpdateClassName(id, teacherID, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- TEACHER deletes a class ----
func deleteClass(c *gin.Context) {
	id, _ := uuid.Parse(c.Param("id"))
	teacherID := uuid.Nil
	if c.GetString("role") == "teacher" {
		teacherID = getUserID(c)
	}
	if err := DeleteClass(id, teacherID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- TEACHER adds students to an existing class ----
func addStudents(c *gin.Context) {
	classID, _ := uuid.Parse(c.Param("id"))
	var req struct {
		StudentIDs []uuid.UUID `json:"student_ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	teacherID := uuid.Nil
	if c.GetString("role") == "teacher" {
		teacherID = getUserID(c)
	}
	if err := AddStudentsToClass(classID, teacherID, req.StudentIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// importBakalariStudents imports students provided by the frontend from a Bakaláři class atom and adds them to a local class.
func importBakalariStudents(c *gin.Context) {
	localID, _ := uuid.Parse(c.Param("id"))
	var req struct {
		Students []struct {
			Id         string `json:"Id"`
			ClassId    string `json:"ClassId"`
			FirstName  string `json:"FirstName"`
			MiddleName string `json:"MiddleName"`
			LastName   string `json:"LastName"`
		} `json:"Students" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var ids []uuid.UUID
	for _, s := range req.Students {
		full := strings.TrimSpace(strings.Join([]string{s.FirstName, s.MiddleName, s.LastName}, " "))
		id, err := EnsureStudentForBk(s.Id, s.ClassId, full)
		if err == nil {
			ids = append(ids, id)
		}
	}
	if err := AddStudentsToClass(localID, getUserID(c), ids); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"added": len(ids)})
}

// ---- STUDENT / TEACHER common list ----
func myClasses(c *gin.Context) {
	uid := getUserID(c)
	role := c.GetString("role")
	var (
		list []Class
		err  error
	)
	if role == "teacher" {
		list, err = ListClassesForTeacher(uid)
	} else if role == "student" {
		list, err = ListClassesForStudent(uid)
	} else if role == "admin" {
		list, err = ListAllClasses()
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}

// ──────────────────────────────────────────────────────
// ADMIN handlers
// ──────────────────────────────────────────────────────

func listUsers(c *gin.Context) {
	list, err := ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func updateUserRole(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := UpdateUserRole(uid, req.Role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func listAllClasses(c *gin.Context) {
	list, err := ListAllClasses()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}
func removeStudent(c *gin.Context) {
	classID, _ := uuid.Parse(c.Param("id"))
	studentID, _ := uuid.Parse(c.Param("sid"))
	teacherID := uuid.Nil
	if c.GetString("role") == "teacher" {
		teacherID = getUserID(c)
	}
	if err := RemoveStudentFromClass(classID, teacherID, studentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// overrideSubmissionPoints allows a teacher or admin to set custom points for a submission.
func overrideSubmissionPoints(c *gin.Context) {
	sid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Points *float64 `json:"points"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a, err := GetAssignmentForSubmission(sid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(a.ID, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	if err := SetSubmissionOverridePoints(sid, req.Points); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	// If manual review is enabled and points were set, mark as completed
	if a.ManualReview && req.Points != nil {
		_ = UpdateSubmissionStatus(sid, "completed")
	}
	c.Status(http.StatusNoContent)
}

// ──────────────────────────────────────────
// File system handlers
// ──────────────────────────────────────────

func listClassFiles(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	role := c.GetString("role")
	if cid == TeacherGroupID {
		if !(role == "teacher" || role == "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else {
		if role == "teacher" {
			if ok, err := IsTeacherOfClass(cid, getUserID(c)); err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		} else if role == "student" {
			if ok, err := IsStudentOfClass(cid, getUserID(c)); err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
	}
	search := c.Query("search")
	if search != "" {
		list, err := SearchFiles(cid, search)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
		c.JSON(http.StatusOK, list)
		return
	}

	var parentID *uuid.UUID
	if pidStr := c.Query("parent"); pidStr != "" {
		if pid, err := uuid.Parse(pidStr); err == nil {
			parentID = &pid
		}
	}
	list, err := ListFiles(cid, parentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func listClassNotebooks(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	role := c.GetString("role")
	if cid == TeacherGroupID {
		if !(role == "teacher" || role == "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else {
		if role == "teacher" {
			if ok, err := IsTeacherOfClass(cid, getUserID(c)); err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		} else if role == "student" {
			if ok, err := IsStudentOfClass(cid, getUserID(c)); err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
	}
	list, err := ListNotebooks(cid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func uploadClassFile(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if cid == TeacherGroupID {
		if !(c.GetString("role") == "teacher" || c.GetString("role") == "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else {
		if c.GetString("role") == "teacher" {
			if ok, err := IsTeacherOfClass(cid, getUserID(c)); err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
	}
	var parentID *uuid.UUID
	pidStr := c.Request.FormValue("parent_id")
	if pidStr != "" {
		if pid, err := uuid.Parse(pidStr); err == nil {
			parentID = &pid
		}
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
			return
		}
		f, err := file.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "open"})
			return
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "read"})
			return
		}
		if len(data) > maxFileSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file too large"})
			return
		}
		cf, err := SaveFile(cid, parentID, filepath.Base(file.Filename), data, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
		c.JSON(http.StatusCreated, cf)
		return
	}
	var req struct {
		Name         string     `json:"name"`
		ParentID     *uuid.UUID `json:"parent_id"`
		IsDir        bool       `json:"is_dir"`
		AssignmentID *uuid.UUID `json:"assignment_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Special support: create an assignment reference entry in Teachers' group
	if req.AssignmentID != nil {
		if cid != TeacherGroupID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "assignment refs allowed only in teachers group"})
			return
		}
		// Load source assignment (no ownership requirement; visibility is governed by presence in Teachers group tree)
		a, err := GetAssignment(*req.AssignmentID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "source assignment not found"})
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = a.Title
		}
		cf, err := SaveAssignmentRef(cid, req.ParentID, name, *req.AssignmentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
		c.JSON(http.StatusCreated, cf)
		return
	}

	// Default: create directory or empty file placeholder
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	cf, err := SaveFile(cid, req.ParentID, req.Name, nil, req.IsDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusCreated, cf)
}

// importAssignmentToClass: POST /api/classes/:id/assignments/import
// Allows teacher/admin to clone a shared assignment (referenced in Teachers' group)
// into one of their own classes, including tests and template/settings.
func importAssignmentToClass(c *gin.Context) {
	classID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid class id"})
		return
	}
	// Teachers must own the class (admins bypass)
	if role := c.GetString("role"); role == "teacher" {
		if ok, err := IsTeacherOfClass(classID, getUserID(c)); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	var req struct {
		SourceAssignmentID uuid.UUID `json:"source_assignment_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Validate that the source assignment is actually published in Teachers' group tree (exists as a ref)
	var tmp int
	if err := DB.Get(&tmp, `SELECT 1 FROM class_files WHERE class_id=$1 AND assignment_id=$2 LIMIT 1`, TeacherGroupID, req.SourceAssignmentID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not shared in teachers group"})
		return
	}
	newID, err := CloneAssignmentWithTests(req.SourceAssignmentID, classID, getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "clone failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"assignment_id": newID})
}

func downloadClassFile(c *gin.Context) {
	fid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	f, err := GetFile(fid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	role := c.GetString("role")
	if f.ClassID == TeacherGroupID {
		if !(role == "teacher" || role == "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else {
		if role == "teacher" {
			if ok, err := IsTeacherOfClass(f.ClassID, getUserID(c)); err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		} else if role == "student" {
			if ok, err := IsStudentOfClass(f.ClassID, getUserID(c)); err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
	}
	if f.IsDir {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not a file"})
		return
	}
	ext := strings.ToLower(filepath.Ext(f.Name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		c.Data(http.StatusOK, mime.TypeByExtension(ext), f.Content)
	default:
		c.Writer.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(f.Name))
		c.Data(http.StatusOK, "application/octet-stream", f.Content)
	}
}

func renameClassFile(c *gin.Context) {
	fid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	f, err := GetFile(fid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if f.ClassID == TeacherGroupID {
		if !(c.GetString("role") == "teacher" || c.GetString("role") == "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else {
		if c.GetString("role") == "teacher" {
			if ok, err := IsTeacherOfClass(f.ClassID, getUserID(c)); err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
	}
	if err := RenameFile(fid, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteClassFile(c *gin.Context) {
	fid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	f, err := GetFile(fid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if f.ClassID == TeacherGroupID {
		if !(c.GetString("role") == "teacher" || c.GetString("role") == "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else {
		if c.GetString("role") == "teacher" {
			if ok, err := IsTeacherOfClass(f.ClassID, getUserID(c)); err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
	}
	if err := DeleteFile(fid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func updateFileContent(c *gin.Context) {
	fid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read"})
		return
	}
	if len(data) > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large"})
		return
	}
	f, err := GetFile(fid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if f.ClassID == TeacherGroupID {
		if !(c.GetString("role") == "teacher" || c.GetString("role") == "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else {
		if c.GetString("role") == "teacher" {
			if ok, err := IsTeacherOfClass(f.ClassID, getUserID(c)); err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
	}
	if f.IsDir {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not a file"})
		return
	}
	if err := UpdateFileContent(fid, data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func resizeAvatar(data string) (string, error) {
	const avatarSize = 256

	parts := strings.SplitN(data, ",", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid data url")
	}
	// parts[0] contains the original prefix, but we will compute the correct
	// prefix again after encoding to ensure it matches the actual output format
	enc := parts[1]
	b, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}

	img, format, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return "", err
	}

	// Preserve aspect ratio and crop to square to avoid squeezing
	dst := imaging.Fill(img, avatarSize, avatarSize, imaging.Center, imaging.Lanczos)

	buf := bytes.Buffer{}
	switch format {
	case "jpeg", "jpg":
		// Slightly higher quality to reduce downscaling artifacts
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 95})
		format = "jpeg"
	default:
		err = png.Encode(&buf, dst)
		format = "png"
	}
	if err != nil {
		return "", err
	}

	prefix := "data:image/" + format + ";base64"
	return prefix + "," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func updateProfile(c *gin.Context) {
	uid := getUserID(c)
	var req struct {
		Name               *string `json:"name"`
		Avatar             *string `json:"avatar"`
		Theme              *string `json:"theme"`
		EmailNotifications *bool   `json:"email_notifications"`
		EmailMessageDigest *bool   `json:"email_message_digest"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := GetUser(uid)
	if err != nil {
		log.Printf("[linkBakalariAccount] GetUser error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	if user.BkUID != nil {
		req.Name = nil // Bakalari users cannot change name
	}
	if req.Avatar != nil {
		av := strings.TrimSpace(*req.Avatar)
		if strings.HasPrefix(av, "data:") {
			resized, err := resizeAvatar(av)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid avatar"})
				return
			}
			req.Avatar = &resized
		} else {
			// allow selecting one of the built-in avatars
			isAllowed := false
			for _, a := range defaultAvatars {
				if av == a {
					isAllowed = true
					break
				}
			}
			if !isAllowed {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid avatar selection"})
				return
			}
			req.Avatar = &av
		}
	}
	if req.EmailNotifications != nil && !*req.EmailNotifications {
		falseVal := false
		req.EmailMessageDigest = &falseVal
	}
	if req.Theme != nil {
		t := strings.ToLower(strings.TrimSpace(*req.Theme))
		if t != "light" && t != "dark" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid theme"})
			return
		}
		req.Theme = &t
	}
	if err := UpdateUserProfile(uid, req.Name, req.Avatar, req.Theme, req.EmailNotifications, req.EmailMessageDigest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func changePassword(c *gin.Context) {
	uid := getUserID(c)
	var req struct {
		Old string `json:"old_password" binding:"required"`
		New string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := GetUser(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	if user.BkUID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bakalari account"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Old)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wrong password"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.New), bcrypt.DefaultCost)
	if err := UpdateUserPassword(uid, string(hash)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func linkBakalariAccount(c *gin.Context) {
	uid := getUserID(c)
	var req struct {
		UID   string  `json:"uid" binding:"required"`
		Role  string  `json:"role" binding:"required"`
		Class *string `json:"class"`
		Name  *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := GetUser(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	if user.BkUID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "already linked"})
		return
	}

	bkUID := strings.TrimSpace(req.UID)
	if len(bkUID) > 3 {
		bkUID = bkUID[len(bkUID)-3:]
	}

	role := "student"
	if strings.EqualFold(req.Role, "teacher") {
		role = "teacher"
	}

	var classValue *string
	if role == "student" && req.Class != nil {
		cls := strings.TrimSpace(*req.Class)
		if cls != "" {
			classValue = new(string)
			*classValue = cls
		}
	}

	var nameValue *string
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n != "" {
			nameValue = new(string)
			*nameValue = n
		}
	}

	var existing *User
	existing, err = FindUserByBkUID(bkUID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
		existing = nil
	}

	var existingDetail *User
	if existing != nil {
		existingDetail, err = GetUser(existing.ID)
		if err != nil {
			log.Printf("[linkBakalariAccount] GetUser existing error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
	}

	finalName := user.Name
	if finalName == nil || strings.TrimSpace(*finalName) == "" {
		if nameValue != nil {
			finalName = nameValue
		} else if existingDetail != nil && existingDetail.Name != nil && strings.TrimSpace(*existingDetail.Name) != "" {
			finalName = existingDetail.Name
		}
	}

	finalAvatar := user.Avatar
	if finalAvatar == nil && existingDetail != nil && existingDetail.Avatar != nil {
		finalAvatar = existingDetail.Avatar
	}

	finalBkClass := user.BkClass
	if role == "student" {
		if classValue != nil {
			finalBkClass = classValue
		} else if finalBkClass == nil || strings.TrimSpace(*finalBkClass) == "" {
			if existingDetail != nil && existingDetail.BkClass != nil && strings.TrimSpace(*existingDetail.BkClass) != "" {
				finalBkClass = existingDetail.BkClass
			}
		}
	} else {
		finalBkClass = nil
	}

	finalRole := user.Role
	switch finalRole {
	case "admin":
		// do nothing
	case "teacher":
		// keep teacher role
	default:
		if role == "teacher" || (existingDetail != nil && existingDetail.Role == "teacher") {
			finalRole = "teacher"
		} else {
			finalRole = "student"
		}
	}

	tx, err := DB.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("[linkBakalariAccount] rollback error: %v", err)
		}
	}()

	if existingDetail != nil {
		if err := mergeUsersTx(tx, user.ID, existingDetail.ID); err != nil {
			log.Printf("[linkBakalariAccount] mergeUsersTx error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
		if _, err := tx.Exec(`DELETE FROM users WHERE id=$1`, existingDetail.ID); err != nil {
			log.Printf("[linkBakalariAccount] delete existing user error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
			return
		}
	}

	if _, err := tx.Exec(`
		UPDATE users
		   SET bk_uid=$2,
		       bk_class = CASE
		           WHEN $6 <> 'student' THEN NULL
		           WHEN $3::text IS NOT NULL THEN $3
		           ELSE bk_class
		       END,
		       name = COALESCE($4, name),
		       avatar = COALESCE($5, avatar),
		       role = $6,
		       updated_at = now()
		 WHERE id=$1`,
		user.ID, bkUID, finalBkClass, finalName, finalAvatar, finalRole); err != nil {
		log.Printf("[linkBakalariAccount] update user error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[linkBakalariAccount] commit error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}

	c.Status(http.StatusNoContent)
}

func linkLocalAccount(c *gin.Context) {
	uid := getUserID(c)
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := GetUser(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	if user.BkUID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "non-bakalari account"})
		return
	}
	if _, err := mail.ParseAddress(user.Email); err == nil && user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "already linked"})
		return
	}
	if mailer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Email verification is temporarily unavailable"})
		return
	}
	email := strings.TrimSpace(req.Email)
	existing, err := FindUserByEmail(email)
	if err == nil && existing.ID != uid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email already in use"})
		return
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	tx, err := DB.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE users
	                         SET email=$1,
	                             password_hash=$2,
	                             email_verified=FALSE,
	                             email_verified_at=NULL
	                       WHERE id=$3`, email, string(hash), uid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	token, err := issueEmailVerificationTokenTx(tx, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	if err := mailer.sendVerificationEmail(email, token); err != nil {
		log.Printf("could not send verification email for user %s: %v", uid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not send verification email"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"message":           "Verification email sent",
		"needsVerification": true,
		"email":             email,
	})
}

// ──────────────────────────────────────────
// messaging handlers
// ──────────────────────────────────────────

func searchUsers(c *gin.Context) {
	term := c.Query("q")
	list, err := SearchUsers(term)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func listConversations(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	list, err := ListRecentConversations(getUserID(c), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func getUserPublic(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	u, err := GetUser(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":     u.ID,
		"name":   u.Name,
		"avatar": u.Avatar,
		"email":  u.Email,
	})
}

func createMessage(c *gin.Context) {
	var req struct {
		To       uuid.UUID `json:"to" binding:"required"`
		Text     string    `json:"text"`
		Image    *string   `json:"image"`
		FileName *string   `json:"file_name"`
		File     *string   `json:"file"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Text) == "" && (req.Image == nil || *req.Image == "") && (req.File == nil || *req.File == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty message"})
		return
	}
	if req.Image != nil && len(*req.Image) > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image too large"})
		return
	}
	if req.File != nil && len(*req.File) > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large"})
		return
	}
	msg := &Message{SenderID: getUserID(c), RecipientID: req.To, Text: req.Text, Image: req.Image, FileName: req.FileName, File: req.File}
	if err := CreateMessage(msg); err != nil {
		if errors.Is(err, ErrBlocked) {
			c.JSON(http.StatusForbidden, gin.H{"error": "blocked"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		}
		return
	}
	c.JSON(http.StatusCreated, msg)
}

func listMessages(c *gin.Context) {
	otherID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}
	uid := getUserID(c)
	msgs, err := ListMessages(uid, otherID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	_ = MarkMessagesRead(uid, otherID)
	c.JSON(http.StatusOK, msgs)
}

func markMessagesReadHandler(c *gin.Context) {
	otherID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := MarkMessagesRead(getUserID(c), otherID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func starConversation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := StarConversation(getUserID(c), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func unstarConversation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := UnstarConversation(getUserID(c), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func archiveConversation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := ArchiveConversation(getUserID(c), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func unarchiveConversation(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := UnarchiveConversation(getUserID(c), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func blockUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := BlockUser(getUserID(c), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func unblockUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := UnblockUser(getUserID(c), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func listBlockedUsers(c *gin.Context) {
	list, err := ListBlockedUsers(getUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}

// downloadMessageFile: GET /api/messages/file/:id
func downloadMessageFile(c *gin.Context) {
	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// Get the message to check permissions and get file data
	var msg Message
	err = DB.Get(&msg, `SELECT id, sender_id, recipient_id, file_name, file FROM messages WHERE id=$1`, fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
		return
	}

	// Check if user has permission to download this file (must be sender or recipient)
	userID := getUserID(c)
	if msg.SenderID != userID && msg.RecipientID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if msg.File == nil || *msg.File == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "no file attached"})
		return
	}

	// Decode base64 file data
	fileData, err := base64.StdEncoding.DecodeString(*msg.File)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid file data"})
		return
	}

	// Set appropriate headers
	filename := "file"
	if msg.FileName != nil && *msg.FileName != "" {
		filename = *msg.FileName
	}

	c.Writer.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(filename))
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(fileData)))

	// Determine content type based on file extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		c.Data(http.StatusOK, mime.TypeByExtension(ext), fileData)
	default:
		c.Data(http.StatusOK, "application/octet-stream", fileData)
	}
}

// ──────────────────────────────────────────────────────
// Admin-only utilities
// ──────────────────────────────────────────────────────

// adminCreateClass: POST /api/admin/classes
// Allows admins to create a class for a specified teacher.
func adminCreateClass(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		Name      string    `json:"name" binding:"required"`
		TeacherID uuid.UUID `json:"teacher_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl := &Class{Name: req.Name, TeacherID: req.TeacherID}
	if err := CreateClass(cl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusCreated, cl)
}

// adminTransferClass: PUT /api/admin/classes/:id/transfer
// Transfers class ownership to a new teacher.
func adminTransferClass(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		TeacherID uuid.UUID `json:"teacher_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := UpdateClassTeacher(id, req.TeacherID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// User presence endpoints
func presenceHandler(c *gin.Context) {
	uid := getUserID(c)

	switch c.Request.Method {
	case "POST":
		// Mark user as online
		if err := MarkUserOnline(uid); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update presence"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "online"})

	case "PUT":
		// Update last seen
		if err := UpdateUserLastSeen(uid); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update presence"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated"})

	case "DELETE":
		// Mark user as offline
		if err := MarkUserOffline(uid); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update presence"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "offline"})

	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
	}
}

func onlineUsersHandler(c *gin.Context) {
	users, err := GetOnlineUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get online users"})
		return
	}

	// Get user details for online users
	type OnlineUser struct {
		ID     uuid.UUID `json:"id"`
		Name   string    `json:"name"`
		Avatar string    `json:"avatar"`
		Email  string    `json:"email"`
	}

	var onlineUsers []OnlineUser
	for _, presence := range users {
		user, err := GetUser(presence.UserID)
		if err != nil {
			continue // Skip users that can't be found
		}
		name := ""
		if user.Name != nil {
			name = *user.Name
		}
		avatar := ""
		if user.Avatar != nil {
			avatar = *user.Avatar
		}
		onlineUsers = append(onlineUsers, OnlineUser{
			ID:     user.ID,
			Name:   name,
			Avatar: avatar,
			Email:  user.Email,
		})
	}

	c.JSON(http.StatusOK, onlineUsers)
}
