package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
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
		if !a.Published {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		subs, _ := ListSubmissionsForAssignmentAndStudent(id, c.GetInt("userID"))
		c.JSON(http.StatusOK, gin.H{"assignment": a, "submissions": subs})
		return
	}
	tests, _ := ListTestCases(id)
	resp := gin.H{"assignment": a, "tests": tests}
	if role == "teacher" || role == "admin" {
		subs, _ := ListSubmissionsForAssignment(id)
		resp["submissions"] = subs
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

	var req struct {
		Title         string    `json:"title" binding:"required"`
		Description   string    `json:"description" binding:"required"`
		Deadline      time.Time `json:"deadline" binding:"required"`
		MaxPoints     int       `json:"max_points" binding:"required"`
		GradingPolicy string    `json:"grading_policy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a := &Assignment{
		ID:            id,
		Title:         req.Title,
		Description:   req.Description,
		Deadline:      req.Deadline,
		MaxPoints:     req.MaxPoints,
		GradingPolicy: req.GradingPolicy,
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
		TimeLimitSec   float64 `json:"time_limit_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tc := &TestCase{
		AssignmentID:   aid,
		Stdin:          req.Stdin,
		ExpectedStdout: req.ExpectedStdout,
		TimeLimitSec:   req.TimeLimitSec,
	}
	if err := CreateTestCase(tc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db fail"})
		return
	}
	c.JSON(http.StatusCreated, tc)
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
	// enqueue for grading
	EnqueueJob(Job{SubmissionID: sub.ID})
	c.JSON(http.StatusCreated, sub)
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

// bakalariLogin performs a simple username/password login and returns the access token.
func bakalariLogin(username, password string) (string, error) {
	form := url.Values{}
	form.Set("client_id", "ANDR")
	form.Set("grant_type", "password")
	form.Set("username", username)
	form.Set("password", password)
	resp, err := http.PostForm(bakalariBaseURL+"/api/login", form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed")
	}
	var r struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil || r.AccessToken == "" {
		return "", fmt.Errorf("login failed")
	}
	return r.AccessToken, nil
}

// bakalariAtoms fetches teacher's marking atoms from Bakaláři.
func bakalariAtoms(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := bakalariLogin(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	httpReq, _ := http.NewRequest("GET", bakalariBaseURL+"/api/3/marking/atoms", nil)
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bakalari request failed"})
		return
	}
	defer resp.Body.Close()
	var data struct {
		Atoms []struct {
			Id   string `json:"Id"`
			Name string `json:"Name"`
		} `json:"Atoms"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode"})
		return
	}
	c.JSON(http.StatusOK, data.Atoms)
}

// importBakalariStudents imports students from a Bakaláři class atom and adds them to a local class.
func importBakalariStudents(c *gin.Context) {
	localID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		AtomID   string `json:"atom_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := bakalariLogin(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	httpReq, _ := http.NewRequest("GET", bakalariBaseURL+"/api/3/marking/marks/"+req.AtomID, nil)
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bakalari request failed"})
		return
	}
	defer resp.Body.Close()
	var data struct {
		Students []struct {
			Id         string `json:"Id"`
			ClassId    string `json:"ClassId"`
			FirstName  string `json:"FirstName"`
			MiddleName string `json:"MiddleName"`
			LastName   string `json:"LastName"`
		} `json:"Students"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode"})
		return
	}
	var ids []int
	for _, s := range data.Students {
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
