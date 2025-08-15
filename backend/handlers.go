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
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ──────────────────────────────────────────────────────────────────────────────
// utilities
// ──────────────────────────────────────────────────────────────────────────────

func getClass(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	detail, err := GetClassDetail(id, c.GetString("role"), c.GetInt("userID"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func getClassProgress(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		var x int
		if err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, id, c.GetInt("userID")); err != nil {
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
		`INSERT INTO users (email, password_hash, name, role, bk_class, bk_uid)
                 VALUES ($1,$2,$3,'student',$4,$5)`,
		email, hash, name, bkClass, bkUID,
	)
	return err
}

func FindUserByEmail(email string) (*User, error) {
	var u User
	err := DB.Get(&u, `
            SELECT id, email, password_hash, name, role, bk_class, bk_uid
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
	id, err := strconv.Atoi(c.Param("id"))
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
	uid := c.GetInt("userID")
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
	classID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid class id"})
		return
	}
	if c.GetString("role") == "teacher" {
		var x int
		if err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, classID, c.GetInt("userID")); err != nil {
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
		ClassID:       classID,
		Title:         req.Title,
		Description:   req.Description,
		Deadline:      time.Now().Add(24 * time.Hour),
		MaxPoints:     100,
		GradingPolicy: "all_or_nothing",
		Published:     false,
		ShowTraceback: req.ShowTraceback,
		ManualReview:  req.ManualReview,
		CreatedBy:     c.GetInt("userID"),
	}
	if err := CreateAssignment(a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create assignment"})
		return
	}
	c.JSON(http.StatusCreated, a)
}

// listAssignments: GET /api/assignments
func listAssignments(c *gin.Context) {
	list, err := ListAssignments(c.GetString("role"), c.GetInt("userID"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list"})
		return
	}
	c.JSON(http.StatusOK, list)
}

// getAssignment: GET /api/assignments/:id
func getAssignment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
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
		if ok, err := IsStudentOfAssignment(id, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if !a.Published {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		subs, _ := ListSubmissionsForAssignmentAndStudent(id, c.GetInt("userID"))
		c.JSON(http.StatusOK, gin.H{"assignment": a, "submissions": subs})
		return
	} else if role == "teacher" {
		if ok, err := IsTeacherOfAssignment(id, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
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
		tsubs, _ := ListTeacherRunsForAssignment(id)
		resp["submissions"] = subs
		resp["teacher_runs"] = tsubs
	}
	c.JSON(http.StatusOK, resp)
}

// updateAssignment: PUT /api/assignments/:id
func updateAssignment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(id, c.GetInt("userID")); err != nil || !ok {
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
		Title         string `json:"title" binding:"required"`
		Description   string `json:"description"`
		Deadline      string `json:"deadline" binding:"required"`
		MaxPoints     int    `json:"max_points" binding:"required"`
		GradingPolicy string `json:"grading_policy" binding:"required"`
		ShowTraceback bool   `json:"show_traceback"`
		ManualReview  bool   `json:"manual_review"`
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
		ID:            id,
		Title:         req.Title,
		Description:   req.Description,
		Deadline:      dl,
		MaxPoints:     req.MaxPoints,
		GradingPolicy: req.GradingPolicy,
		ShowTraceback: req.ShowTraceback,
		ManualReview:  req.ManualReview,
	}
	if err := UpdateAssignment(a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update"})
		return
	}
	c.JSON(http.StatusOK, a)
}

// deleteAssignment: DELETE /api/assignments/:id
func deleteAssignment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(id, c.GetInt("userID")); err != nil || !ok {
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
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(id, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	if err := SetAssignmentPublished(id, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// uploadTemplate: POST /api/assignments/:id/template
func uploadTemplate(c *gin.Context) {
	aid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, c.GetInt("userID")); err != nil || !ok {
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
	aid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, c.GetInt("userID")); err != nil || !ok {
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
	aid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// Only teachers/admins and must own the assignment if teacher
	if role := c.GetString("role"); role == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, c.GetInt("userID")); err != nil || !ok {
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
	aid, err := strconv.Atoi(c.Param("id"))
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
		if ok, err := IsStudentOfAssignment(aid, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else if role == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	c.FileAttachment(*a.TemplatePath, filepath.Base(*a.TemplatePath))
}

// createTestCase: POST /api/assignments/:id/tests
func createTestCase(c *gin.Context) {
	aid, err := strconv.Atoi(c.Param("id"))
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
	id, err := strconv.Atoi(c.Param("id"))
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
	id, err := strconv.Atoi(c.Param("id"))
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
	aid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, c.GetInt("userID")); err != nil || !ok {
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
	aid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var tmp int
	if err := DB.Get(&tmp, `SELECT 1 FROM assignments a JOIN class_students cs ON cs.class_id=a.class_id WHERE a.id=$1 AND cs.student_id=$2`, aid, c.GetInt("userID")); err != nil {
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

	tmpDir, err := os.MkdirTemp("", "upload-")
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

	// Ensure directory and files are readable by the container user (nobody)
	// The container mounts this directory read-only and needs execute perms on dirs
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
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

	name := fmt.Sprintf("%d_%d_%d.zip", aid, c.GetInt("userID"), time.Now().UnixNano())
	path := filepath.Join("uploads", name)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot save"})
		return
	}

	sub := &Submission{
		AssignmentID: aid,
		StudentID:    c.GetInt("userID"),
		CodePath:     path,
		CodeContent:  base64.StdEncoding.EncodeToString(buf.Bytes()),
	}
	if err := CreateSubmission(sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	// enqueue for grading unless manual review is enabled
	if a, err := GetAssignment(aid); err == nil && !a.ManualReview {
		EnqueueJob(Job{SubmissionID: sub.ID})
	}
	c.JSON(http.StatusCreated, sub)
}

// runTeacherSolution: POST /api/assignments/:id/solution-run
// Allows a teacher/admin to upload a reference solution and run all tests.
// Does not persist a submission or results; returns a summary JSON immediately.
func runTeacherSolution(c *gin.Context) {
	aid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	// only teachers/admins and must own the assignment if teacher
	if role := c.GetString("role"); role == "teacher" {
		if ok, err := IsTeacherOfAssignment(aid, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
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

	tmpDir, err := os.MkdirTemp("", "teacher-solution-")
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "no python files found"})
		return
	}

	tests, err := ListTestCases(aid)
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
	name := fmt.Sprintf("%d_%d_%d_teacher.zip", aid, c.GetInt("userID"), time.Now().UnixNano())
	path := filepath.Join("uploads", name)
	_ = os.WriteFile(path, buf.Bytes(), 0644)
	sub := &Submission{AssignmentID: aid, StudentID: c.GetInt("userID"), CodePath: path, CodeContent: base64.StdEncoding.EncodeToString(buf.Bytes()), IsTeacherRun: true}
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
	if a, err := GetAssignment(aid); err == nil {
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
		_ = SetSubmissionPoints(sub.ID, score)
	}
	if allPass {
		_ = UpdateSubmissionStatus(sub.ID, "completed")
	} else {
		_ = UpdateSubmissionStatus(sub.ID, "failed")
	}

	c.JSON(http.StatusOK, gin.H{
		"submission_id": sub.ID,
		"total":         len(tests),
		"passed":        passed,
		"failed":        len(tests) - passed,
		"results":       results,
	})
}

// getSubmission: GET /api/submissions/:id
func getSubmission(c *gin.Context) {
	sid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	sub, err := GetSubmission(sid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if c.GetString("role") == "student" && c.GetInt("userID") != sub.StudentID {
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
	c.JSON(http.StatusOK, gin.H{"submission": sub, "results": results})
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
	if err := CreateTeacher(req.Email, string(hash), nil); err != nil {
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
	cl := &Class{Name: req.Name, TeacherID: c.GetInt("userID")}
	if err := CreateClass(cl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusCreated, cl)
}

// ---- TEACHER renames a class ----
func updateClass(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	teacherID := 0
	if c.GetString("role") == "teacher" {
		teacherID = c.GetInt("userID")
	}
	if err := UpdateClassName(id, teacherID, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- TEACHER deletes a class ----
func deleteClass(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	teacherID := 0
	if c.GetString("role") == "teacher" {
		teacherID = c.GetInt("userID")
	}
	if err := DeleteClass(id, teacherID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- TEACHER adds students to an existing class ----
func addStudents(c *gin.Context) {
	classID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		StudentIDs []int `json:"student_ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	teacherID := 0
	if c.GetString("role") == "teacher" {
		teacherID = c.GetInt("userID")
	}
	if err := AddStudentsToClass(classID, teacherID, req.StudentIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// importBakalariStudents imports students provided by the frontend from a Bakaláři class atom and adds them to a local class.
func importBakalariStudents(c *gin.Context) {
	localID, _ := strconv.Atoi(c.Param("id"))
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
	var ids []int
	for _, s := range req.Students {
		full := strings.TrimSpace(strings.Join([]string{s.FirstName, s.MiddleName, s.LastName}, " "))
		id, err := EnsureStudentForBk(s.Id, s.ClassId, full)
		if err == nil {
			ids = append(ids, id)
		}
	}
	if err := AddStudentsToClass(localID, c.GetInt("userID"), ids); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"added": len(ids)})
}

// ---- STUDENT / TEACHER common list ----
func myClasses(c *gin.Context) {
	uid := c.GetInt("userID")
	role := c.GetString("role")
	var (
		list []Class
		err  error
	)
	if role == "teacher" {
		list, err = ListClassesForTeacher(uid)
	} else {
		list, err = ListClassesForStudent(uid)
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
	uid, err := strconv.Atoi(c.Param("id"))
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
	classID, _ := strconv.Atoi(c.Param("id"))
	studentID, _ := strconv.Atoi(c.Param("sid"))
	teacherID := 0
	if c.GetString("role") == "teacher" {
		teacherID = c.GetInt("userID")
	}
	if err := RemoveStudentFromClass(classID, teacherID, studentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

// overrideSubmissionPoints allows a teacher or admin to set custom points for a submission.
func overrideSubmissionPoints(c *gin.Context) {
	sid, err := strconv.Atoi(c.Param("id"))
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
		if ok, err := IsTeacherOfAssignment(a.ID, c.GetInt("userID")); err != nil || !ok {
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
	cid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	role := c.GetString("role")
	if role == "teacher" {
		if ok, err := IsTeacherOfClass(cid, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else if role == "student" {
		if ok, err := IsStudentOfClass(cid, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
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

	var parentID *int
	if pidStr := c.Query("parent"); pidStr != "" {
		if pid, err := strconv.Atoi(pidStr); err == nil {
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
	cid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	role := c.GetString("role")
	if role == "teacher" {
		if ok, err := IsTeacherOfClass(cid, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else if role == "student" {
		if ok, err := IsStudentOfClass(cid, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
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
	cid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfClass(cid, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	var parentID *int
	pidStr := c.Request.FormValue("parent_id")
	if pidStr != "" {
		if pid, err := strconv.Atoi(pidStr); err == nil {
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
		Name     string `json:"name" binding:"required"`
		ParentID *int   `json:"parent_id"`
		IsDir    bool   `json:"is_dir"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cf, err := SaveFile(cid, req.ParentID, req.Name, nil, req.IsDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusCreated, cf)
}

func downloadClassFile(c *gin.Context) {
	fid, err := strconv.Atoi(c.Param("id"))
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
	if role == "teacher" {
		if ok, err := IsTeacherOfClass(f.ClassID, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	} else if role == "student" {
		if ok, err := IsStudentOfClass(f.ClassID, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
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
	fid, err := strconv.Atoi(c.Param("id"))
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
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfClass(f.ClassID, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	if err := RenameFile(fid, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteClassFile(c *gin.Context) {
	fid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	f, err := GetFile(fid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfClass(f.ClassID, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	if err := DeleteFile(fid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func updateFileContent(c *gin.Context) {
	fid, err := strconv.Atoi(c.Param("id"))
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
	if c.GetString("role") == "teacher" {
		if ok, err := IsTeacherOfClass(f.ClassID, c.GetInt("userID")); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
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

	dst := imaging.Resize(img, avatarSize, avatarSize, imaging.Lanczos)

	buf := bytes.Buffer{}
	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 90})
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
	uid := c.GetInt("userID")
	var req struct {
		Name   *string `json:"name"`
		Avatar *string `json:"avatar"`
		Theme  *string `json:"theme"`
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
	if req.Theme != nil {
		t := strings.ToLower(strings.TrimSpace(*req.Theme))
		if t != "light" && t != "dark" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid theme"})
			return
		}
		req.Theme = &t
	}
	if err := UpdateUserProfile(uid, req.Name, req.Avatar, req.Theme); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func changePassword(c *gin.Context) {
	uid := c.GetInt("userID")
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

func linkLocalAccount(c *gin.Context) {
	uid := c.GetInt("userID")
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
	if _, err := mail.ParseAddress(user.Email); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "already linked"})
		return
	}
	if _, err := FindUserByEmail(req.Email); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email already in use"})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err := LinkLocalAccount(uid, req.Email, string(hash)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
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
	list, err := ListRecentConversations(c.GetInt("userID"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func getUserPublic(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
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
		To    int     `json:"to" binding:"required"`
		Text  string  `json:"text"`
		Image *string `json:"image"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Text) == "" && (req.Image == nil || *req.Image == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty message"})
		return
	}
	if req.Image != nil && len(*req.Image) > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image too large"})
		return
	}
	msg := &Message{SenderID: c.GetInt("userID"), RecipientID: req.To, Text: req.Text, Image: req.Image}
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
	otherID, err := strconv.Atoi(c.Param("id"))
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
	uid := c.GetInt("userID")
	msgs, err := ListMessages(uid, otherID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	_ = MarkMessagesRead(uid, otherID)
	c.JSON(http.StatusOK, msgs)
}

func markMessagesReadHandler(c *gin.Context) {
	otherID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := MarkMessagesRead(c.GetInt("userID"), otherID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func starConversation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := StarConversation(c.GetInt("userID"), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func unstarConversation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := UnstarConversation(c.GetInt("userID"), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func archiveConversation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := ArchiveConversation(c.GetInt("userID"), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func unarchiveConversation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := UnarchiveConversation(c.GetInt("userID"), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func blockUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := BlockUser(c.GetInt("userID"), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func unblockUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := UnblockUser(c.GetInt("userID"), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.Status(http.StatusNoContent)
}

func listBlockedUsers(c *gin.Context) {
	list, err := ListBlockedUsers(c.GetInt("userID"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusOK, list)
}
