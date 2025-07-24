package main

import (
	"fmt"
	"time"

	"github.com/gin-contrib/sse"
	"golang.org/x/crypto/bcrypt"
)

const maxFileSize = 20 * 1024 * 1024 // 20 MB

type User struct {
	ID           int       `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Name         *string   `db:"name"`
	Avatar       *string   `db:"avatar"`
	Role         string    `db:"role"`
	BkClass      *string   `db:"bk_class"`
	BkUID        *string   `db:"bk_uid"`
	CreatedAt    time.Time `db:"created_at"`
}

type Assignment struct {
	ID            int       `db:"id" json:"id"`
	Title         string    `db:"title" json:"title"`
	Description   string    `db:"description" json:"description"`
	CreatedBy     int       `db:"created_by" json:"created_by"`
	Deadline      time.Time `db:"deadline" json:"deadline"`
	MaxPoints     int       `db:"max_points" json:"max_points"`
	GradingPolicy string    `db:"grading_policy" json:"grading_policy"`
	Published     bool      `db:"published" json:"published"`
	ShowTraceback bool      `db:"show_traceback" json:"show_traceback"`
	TemplatePath  *string   `db:"template_path" json:"template_path"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
	ClassID       int       `db:"class_id" json:"class_id"`
}
type Class struct {
	ID        int       `db:"id"        json:"id"`
	Name      string    `db:"name"      json:"name"`
	TeacherID int       `db:"teacher_id" json:"teacher_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type Submission struct {
	ID           int       `db:"id" json:"id"`
	AssignmentID int       `db:"assignment_id" json:"assignment_id"`
	StudentID    int       `db:"student_id" json:"student_id"`
	CodePath     string    `db:"code_path" json:"code_path"`
	CodeContent  string    `db:"code_content" json:"code_content"`
	Status       string    `db:"status" json:"status"`
	Points       *float64  `db:"points" json:"points"`
	OverridePts  *float64  `db:"override_points" json:"override_points"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

type TestCase struct {
	ID             int       `db:"id" json:"id"`
	AssignmentID   int       `db:"assignment_id" json:"assignment_id"`
	Stdin          string    `db:"stdin" json:"stdin"`
	ExpectedStdout string    `db:"expected_stdout" json:"expected_stdout"`
	Weight         float64   `db:"weight" json:"weight"`
	TimeLimitSec   float64   `db:"time_limit_sec" json:"time_limit_sec"`
	MemoryLimitKB  int       `db:"memory_limit_kb" json:"memory_limit_kb"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// ──────────────────────────────────────────────────────
// admin helpers
// ──────────────────────────────────────────────────────

type UserSummary struct {
	ID        int       `db:"id"         json:"id"`
	Email     string    `db:"email"      json:"email"`
	Name      *string   `db:"name"       json:"name"`
	Role      string    `db:"role"       json:"role"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func ListUsers() ([]UserSummary, error) {
	list := []UserSummary{}
	err := DB.Select(&list,
		`SELECT id,email,name,role,created_at
                  FROM users
             ORDER BY created_at`)
	return list, err
}

func UpdateUserRole(id int, role string) error {
	// only three legal roles
	switch role {
	case "student", "teacher", "admin":
	default:
		return fmt.Errorf("invalid role")
	}
	_, err := DB.Exec(`UPDATE users SET role=$1 WHERE id=$2`, role, id)
	return err
}

func GetUser(id int) (*User, error) {
	var u User
	err := DB.Get(&u, `SELECT id, email, password_hash, name, avatar, role, bk_class, bk_uid, created_at
                FROM users WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func UpdateUserProfile(id int, name, avatar *string) error {
	_, err := DB.Exec(`UPDATE users SET name=COALESCE($1,name), avatar=COALESCE($2,avatar) WHERE id=$3`, name, avatar, id)
	return err
}

func UpdateUserPassword(id int, hash string) error {
	_, err := DB.Exec(`UPDATE users SET password_hash=$1 WHERE id=$2`, hash, id)
	return err
}

func ListAllClasses() ([]Class, error) {
	var cls []Class
	err := DB.Select(&cls,
		`SELECT * FROM classes ORDER BY created_at DESC`)
	return cls, err
}

// ──────────────────────────────────────────────────────────────────────────────
// assignments
// ──────────────────────────────────────────────────────────────────────────────
func CreateAssignment(a *Assignment) error {
	const q = `
          INSERT INTO assignments (title, description, created_by, deadline, max_points, grading_policy, published, show_traceback, template_path, class_id)
          VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
          RETURNING id, created_at, updated_at`
	return DB.QueryRow(q,
		a.Title, a.Description, a.CreatedBy, a.Deadline,
		a.MaxPoints, a.GradingPolicy, a.Published, a.ShowTraceback, a.TemplatePath, a.ClassID,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

// ListAssignments returns all assignments.
func ListAssignments(role string, userID int) ([]Assignment, error) {
	list := []Assignment{}
	query := `
    SELECT a.id, a.title, a.description, a.created_by, a.deadline,
           a.max_points, a.grading_policy, a.published, a.show_traceback, a.template_path,
           a.created_at, a.updated_at, a.class_id
      FROM assignments a`
	var args []any
	switch role {
	case "teacher":
		query += ` JOIN classes c ON c.id = a.class_id
                WHERE c.teacher_id = $1`
		args = append(args, userID)
	case "student":
		query += ` JOIN class_students cs ON cs.class_id = a.class_id
                WHERE cs.student_id = $1 AND a.published = true`
		args = append(args, userID)
	default:
		// admin gets everything
	}
	query += " ORDER BY a.created_at DESC"
	err := DB.Select(&list, query, args...)
	return list, err
}

// GetAssignment looks up one assignment by ID.
func GetAssignment(id int) (*Assignment, error) {
	var a Assignment
	err := DB.Get(&a, `
    SELECT id, title, description, created_by, deadline, max_points, grading_policy, published, show_traceback, template_path, created_at, updated_at, class_id
      FROM assignments
     WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAssignmentForSubmission retrieves the assignment associated with a submission.
func GetAssignmentForSubmission(subID int) (*Assignment, error) {
	var a Assignment
	err := DB.Get(&a, `
        SELECT a.id, a.title, a.description, a.created_by, a.deadline,
               a.max_points, a.grading_policy, a.published, a.show_traceback, a.template_path,
               a.created_at, a.updated_at, a.class_id
          FROM assignments a
          JOIN submissions s ON s.assignment_id = a.id
         WHERE s.id=$1`, subID)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// UpdateAssignment modifies title/description/deadline of an existing assignment.
func UpdateAssignment(a *Assignment) error {
	res, err := DB.Exec(`
    UPDATE assignments
       SET title=$1, description=$2, deadline=$3,
           max_points=$4, grading_policy=$5, show_traceback=$6,
           updated_at=now()
     WHERE id=$7`,
		a.Title, a.Description, a.Deadline,
		a.MaxPoints, a.GradingPolicy, a.ShowTraceback,
		a.ID)
	if err != nil {
		return err
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return fmt.Errorf("no rows updated")
	}
	return nil
}

// DeleteAssignment removes an assignment (and cascades test_cases/submissions).
func DeleteAssignment(id int) error {
	_, err := DB.Exec(`DELETE FROM assignments WHERE id=$1`, id)
	return err
}

// SetAssignmentPublished updates the published flag on an assignment.
func SetAssignmentPublished(id int, published bool) error {
	_, err := DB.Exec(`UPDATE assignments SET published=$1, updated_at=now() WHERE id=$2`, published, id)
	return err
}

func UpdateAssignmentTemplate(id int, path *string) error {
	_, err := DB.Exec(`UPDATE assignments SET template_path=$1, updated_at=now() WHERE id=$2`, path, id)
	return err
}

// IsTeacherOfAssignment checks whether the given teacher owns the class the
// assignment belongs to.
func IsTeacherOfAssignment(aid, teacherID int) (bool, error) {
	var x int
	err := DB.Get(&x, `SELECT 1 FROM assignments a JOIN classes c ON c.id=a.class_id
                WHERE a.id=$1 AND c.teacher_id=$2`, aid, teacherID)
	if err != nil {
		return false, err
	}
	return true, nil
}

// IsStudentOfAssignment checks whether the student is enrolled in the class the
// assignment belongs to.
func IsStudentOfAssignment(aid, studentID int) (bool, error) {
	var x int
	err := DB.Get(&x, `SELECT 1 FROM assignments a JOIN class_students cs ON cs.class_id=a.class_id
                WHERE a.id=$1 AND cs.student_id=$2`, aid, studentID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func CreateTeacher(email, hash string, bkUID *string) error {
	_, err := DB.Exec(`
        INSERT INTO users (email, password_hash, role, bk_uid)
        VALUES ($1,$2,'teacher',$3)`, email, hash, bkUID)
	return err
}

// FindUserByBkUID returns a user identified by the Bakaláři UID.
func FindUserByBkUID(uid string) (*User, error) {
	var u User
	err := DB.Get(&u, `SELECT id, email, password_hash, name, role, bk_class, bk_uid, created_at
                            FROM users WHERE bk_uid=$1`, uid)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// createStudentWithID inserts a new student and returns its database ID.
func createStudentWithID(email, hash string, name, bkClass, bkUID *string) (int, error) {
	var id int
	err := DB.QueryRow(`
                INSERT INTO users (email, password_hash, name, role, bk_class, bk_uid)
                VALUES ($1,$2,$3,'student',$4,$5)
                RETURNING id`, email, hash, name, bkClass, bkUID).Scan(&id)
	return id, err
}

// EnsureStudentForBk ensures a student exists for the given Bakaláři UID
// and returns the local user ID.
func EnsureStudentForBk(uid, cls, name string) (int, error) {
	u, err := FindUserByBkUID(uid)
	if err == nil {
		if cls != "" && (u.BkClass == nil || *u.BkClass != cls) {
			_, _ = DB.Exec(`UPDATE users SET bk_class=$1 WHERE id=$2`, cls, u.ID)
			u.BkClass = &cls
		}
		if name != "" && (u.Name == nil || *u.Name != name) {
			_, _ = DB.Exec(`UPDATE users SET name=$1 WHERE id=$2`, name, u.ID)
			u.Name = &name
		}
		return u.ID, nil
	}
	// not found
	hash, _ := bcrypt.GenerateFromPassword([]byte(uid), bcrypt.DefaultCost)
	return createStudentWithID(uid, string(hash), &name, &cls, &uid)
}

func CreateClass(c *Class) error {
	return DB.QueryRow(`
        INSERT INTO classes (name, teacher_id)
        VALUES ($1,$2)
        RETURNING id, created_at, updated_at`,
		c.Name, c.TeacherID,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func UpdateClassName(id, teacherID int, name string) error {
	if teacherID != 0 {
		var x int
		if err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, id, teacherID); err != nil {
			return err
		}
	}
	res, err := DB.Exec(`UPDATE classes SET name=$1, updated_at=now() WHERE id=$2`, name, id)
	if err != nil {
		return err
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return fmt.Errorf("no rows updated")
	}
	return nil
}

func DeleteClass(id, teacherID int) error {
	if teacherID != 0 {
		var x int
		if err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, id, teacherID); err != nil {
			return err
		}
	}
	_, err := DB.Exec(`DELETE FROM classes WHERE id=$1`, id)
	return err
}

func AddStudentsToClass(classID, teacherID int, studentIDs []int) error {
	if teacherID != 0 {
		var x int
		if err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, classID, teacherID); err != nil {
			return err
		}
	}
	tx, err := DB.Beginx()
	if err != nil {
		return err
	}
	for _, sid := range studentIDs {
		if _, err = tx.Exec(`
            INSERT INTO class_students (class_id, student_id)
            VALUES ($1,$2) ON CONFLICT DO NOTHING`, classID, sid); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func ListClassesForTeacher(teacherID int) ([]Class, error) {
	var cls []Class
	err := DB.Select(&cls, `
                SELECT * FROM classes
                 WHERE teacher_id = $1
                 ORDER BY created_at DESC`, teacherID)
	return cls, err
}

func ListClassesForStudent(studentID int) ([]Class, error) {
	var cls []Class
	err := DB.Select(&cls, `
        SELECT c.* FROM classes c
        JOIN class_students cs ON cs.class_id = c.id
        WHERE cs.student_id = $1
        ORDER BY c.created_at DESC`, studentID)
	return cls, err
}

func ListAllStudents() ([]Student, error) {
	list := []Student{}
	err := DB.Select(&list, `
            SELECT id, email, name FROM users
             WHERE role = 'student'
             ORDER BY email`)
	return list, err
}

// ──────────────────────────────────────────────────────────────────────────────
// classes – helpers for detail view
// ──────────────────────────────────────────────────────────────────────────────
type Student struct {
	ID    int     `db:"id"    json:"id"`
	Email string  `db:"email" json:"email"`
	Name  *string `db:"name"  json:"name"`
}

type ClassDetail struct {
	Class       `json:"class"`
	Teacher     Student      `json:"teacher"`
	Students    []Student    `json:"students"`
	Assignments []Assignment `json:"assignments"`
}

func GetClassDetail(id int, role string, userID int) (*ClassDetail, error) {
	// 1) Class meta -------------------------------------------------------
	var cls Class
	switch role {
	case "teacher":
		if err := DB.Get(&cls, `SELECT * FROM classes WHERE id=$1 AND teacher_id=$2`, id, userID); err != nil {
			return nil, err
		}
	case "student":
		if err := DB.Get(&cls, `SELECT c.* FROM classes c JOIN class_students cs ON cs.class_id=c.id WHERE c.id=$1 AND cs.student_id=$2`, id, userID); err != nil {
			return nil, err
		}
	default:
		if err := DB.Get(&cls, `SELECT * FROM classes WHERE id = $1`, id); err != nil {
			return nil, err
		}
	}

	// 2) Teacher (one row) -----------------------------------------------------
	var teacher Student // reuse tiny struct {id,email,name}
	if err := DB.Get(&teacher,
		`SELECT id, email, name FROM users WHERE id = $1`,
		cls.TeacherID); err != nil {
		return nil, err
	}

	// 3) Students (many) -------------------------------------------------------
	var students []Student
	if err := DB.Select(&students, `
               SELECT u.id, u.email, u.name
                 FROM users u
                 JOIN class_students cs ON cs.student_id = u.id
                WHERE cs.class_id = $1
                ORDER BY u.email`,
		id); err != nil {
		return nil, err
	}

	// 4) Assignments (many) ----------------------------------------------------
	var asg []Assignment
	query := `
                SELECT id, title, description, created_by, deadline,
                       max_points, grading_policy, published, template_path,
                       created_at, updated_at, class_id
                  FROM assignments
                 WHERE class_id = $1`
	if role == "student" {
		query += " AND published = true"
	}
	query += " ORDER BY deadline ASC"
	if err := DB.Select(&asg, query, id); err != nil {
		return nil, err
	}

	// 5) Assemble --------------------------------------------------------------
	return &ClassDetail{
		Class:       cls,
		Teacher:     teacher,
		Students:    students,
		Assignments: asg,
	}, nil
}

func RemoveStudentFromClass(classID, teacherID, studentID int) error {
	if teacherID == 0 {
		_, err := DB.Exec(`DELETE FROM class_students WHERE class_id=$1 AND student_id=$2`, classID, studentID)
		return err
	}
	_, err := DB.Exec(`DELETE FROM class_students cs USING classes c
                        WHERE cs.class_id=$1 AND cs.student_id=$2 AND c.id=cs.class_id AND c.teacher_id=$3`,
		classID, studentID, teacherID)
	return err
}

func DeleteUser(id int) error {
	_, err := DB.Exec(`DELETE FROM users WHERE id=$1`, id)
	return err
}

func ListSubmissionsForStudent(studentID int) ([]Submission, error) {
	subs := []Submission{}
	err := DB.Select(&subs, `
               SELECT id, assignment_id, student_id, code_path, code_content, status, points, override_points, created_at, updated_at
                 FROM submissions
                WHERE student_id = $1
                ORDER BY created_at DESC`, studentID)
	return subs, err
}

func CreateSubmission(s *Submission) error {
	const q = `
          INSERT INTO submissions (assignment_id, student_id, code_path, code_content)
          SELECT $1,$2,$3,$4
            WHERE EXISTS (
                SELECT 1 FROM assignments a
                JOIN class_students cs ON cs.class_id = a.class_id
               WHERE a.id=$1 AND cs.student_id=$2)
          RETURNING id, status, created_at, updated_at`
	return DB.QueryRow(q, s.AssignmentID, s.StudentID, s.CodePath, s.CodeContent).
		Scan(&s.ID, &s.Status, &s.CreatedAt, &s.UpdatedAt)
}

type SubmissionWithReason struct {
	Submission
	FailureReason *string `db:"failure_reason" json:"failure_reason,omitempty"`
}

// SubmissionWithStudent includes the submitting student's email.
type SubmissionWithStudent struct {
	Submission
	StudentEmail  string  `db:"email" json:"student_email"`
	StudentName   *string `db:"name" json:"student_name"`
	FailureReason *string `db:"failure_reason" json:"failure_reason,omitempty"`
}

func ListSubmissionsForAssignmentAndStudent(aid, sid int) ([]SubmissionWithReason, error) {
	subs := []SubmissionWithReason{}
	err := DB.Select(&subs, `
               SELECT id, assignment_id, student_id, code_path, code_content, status, points, override_points, created_at, updated_at,
                      (SELECT r.status FROM results r
                         WHERE r.submission_id = submissions.id AND r.status <> 'passed'
                         ORDER BY r.id LIMIT 1) AS failure_reason
                 FROM submissions
                WHERE assignment_id=$1 AND student_id=$2
                ORDER BY created_at DESC`, aid, sid)
	return subs, err
}

// ListSubmissionsForAssignment returns all submissions for a given assignment
// along with each student's email and first failing result.
func ListSubmissionsForAssignment(aid int) ([]SubmissionWithStudent, error) {
	subs := []SubmissionWithStudent{}
	err := DB.Select(&subs, `
               SELECT s.id, s.assignment_id, s.student_id, s.code_path, s.code_content, s.status, s.points, s.override_points, s.created_at, s.updated_at,
                     u.email, u.name,
                     (SELECT r.status FROM results r
                        WHERE r.submission_id = s.id AND r.status <> 'passed'
                         ORDER BY r.id LIMIT 1) AS failure_reason
                 FROM submissions s
                 JOIN users u ON u.id = s.student_id
                WHERE s.assignment_id = $1
                ORDER BY s.created_at DESC`, aid)
	return subs, err
}

func CreateTestCase(tc *TestCase) error {
	if tc.TimeLimitSec == 0 {
		tc.TimeLimitSec = 1
	}
	const q = `
          INSERT INTO test_cases (assignment_id, stdin, expected_stdout, weight, time_limit_sec)
          VALUES ($1,$2,$3,$4,$5)
          RETURNING id, weight, time_limit_sec, memory_limit_kb, created_at, updated_at`
	return DB.QueryRow(q, tc.AssignmentID, tc.Stdin, tc.ExpectedStdout, tc.Weight, tc.TimeLimitSec).
		Scan(&tc.ID, &tc.Weight, &tc.TimeLimitSec, &tc.MemoryLimitKB, &tc.CreatedAt, &tc.UpdatedAt)
}

// UpdateTestCase modifies stdin/stdout/time limit of an existing test case.
func UpdateTestCase(tc *TestCase) error {
	if tc.TimeLimitSec == 0 {
		tc.TimeLimitSec = 1
	}
	res, err := DB.Exec(`
                UPDATE test_cases
                   SET stdin=$1, expected_stdout=$2, weight=$3, time_limit_sec=$4,
                       updated_at=now()
                 WHERE id=$5`,
		tc.Stdin, tc.ExpectedStdout, tc.Weight, tc.TimeLimitSec, tc.ID)
	if err != nil {
		return err
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return fmt.Errorf("no rows updated")
	}
	return nil
}

func ListTestCases(assignmentID int) ([]TestCase, error) {
	list := []TestCase{}
	err := DB.Select(&list, `
                SELECT id, assignment_id, stdin, expected_stdout, weight, time_limit_sec, memory_limit_kb, created_at, updated_at
                  FROM test_cases
                 WHERE assignment_id = $1
                 ORDER BY id`, assignmentID)
	return list, err
}

func DeleteTestCase(id int) error {
	_, err := DB.Exec(`DELETE FROM test_cases WHERE id=$1`, id)
	return err
}

// ──────────────────────────────────────────────────────
// submissions – helpers for grading
// ──────────────────────────────────────────────────────

// Result represents outcome of one test case execution.
type Result struct {
	ID           int       `db:"id" json:"id"`
	SubmissionID int       `db:"submission_id" json:"submission_id"`
	TestCaseID   int       `db:"test_case_id" json:"test_case_id"`
	Status       string    `db:"status" json:"status"`
	ActualStdout string    `db:"actual_stdout" json:"actual_stdout"`
	Stderr       string    `db:"stderr" json:"stderr"`
	ExitCode     int       `db:"exit_code" json:"exit_code"`
	RuntimeMS    int       `db:"runtime_ms" json:"runtime_ms"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

func GetSubmission(id int) (*Submission, error) {
	var s Submission
	err := DB.Get(&s, `
        SELECT id, assignment_id, student_id, code_path, code_content, status, points, override_points, created_at, updated_at
          FROM submissions
         WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func UpdateSubmissionStatus(id int, status string) error {
	_, err := DB.Exec(`UPDATE submissions SET status=$1, updated_at=now() WHERE id=$2`, status, id)
	if err == nil {
		broadcast(sse.Event{Event: "status", Data: map[string]any{"submission_id": id, "status": status}})
	}
	return err
}

func CreateResult(r *Result) error {
	const q = `
        INSERT INTO results (submission_id, test_case_id, status, actual_stdout, stderr, exit_code, runtime_ms)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        RETURNING id, created_at`
	err := DB.QueryRow(q, r.SubmissionID, r.TestCaseID, r.Status, r.ActualStdout, r.Stderr, r.ExitCode, r.RuntimeMS).
		Scan(&r.ID, &r.CreatedAt)
	if err == nil {
		broadcast(sse.Event{Event: "result", Data: r})
	}
	return err
}

func ListResultsForSubmission(subID int) ([]Result, error) {
	list := []Result{}
	err := DB.Select(&list, `
        SELECT id, submission_id, test_case_id, status, actual_stdout, stderr, exit_code, runtime_ms, created_at
          FROM results
         WHERE submission_id=$1
         ORDER BY id`, subID)
	return list, err
}

func SetSubmissionPoints(id int, pts float64) error {
	_, err := DB.Exec(`UPDATE submissions SET points=$1 WHERE id=$2`, pts, id)
	return err
}

func SetSubmissionOverridePoints(id int, pts *float64) error {
	_, err := DB.Exec(`UPDATE submissions SET override_points=$1 WHERE id=$2`, pts, id)
	return err
}

type ScoreCell struct {
	StudentID    int      `db:"student_id" json:"student_id"`
	AssignmentID int      `db:"assignment_id" json:"assignment_id"`
	Points       *float64 `db:"points" json:"points"`
}

type ClassProgress struct {
	Students    []Student    `json:"students"`
	Assignments []Assignment `json:"assignments"`
	Scores      []ScoreCell  `json:"scores"`
}

// GetClassProgress returns score cells for each student/assignment pair in a class.
func GetClassProgress(classID int) (*ClassProgress, error) {
	var students []Student
	if err := DB.Select(&students, `
                SELECT u.id, u.email, u.name
                  FROM users u
                  JOIN class_students cs ON cs.student_id=u.id
                 WHERE cs.class_id=$1
                 ORDER BY u.email`, classID); err != nil {
		return nil, err
	}

	var asg []Assignment
	if err := DB.Select(&asg, `
                SELECT id, title, description, created_by, deadline,
                       max_points, grading_policy, published, template_path,
                       created_at, updated_at, class_id
                  FROM assignments
                 WHERE class_id=$1
                 ORDER BY deadline ASC`, classID); err != nil {
		return nil, err
	}

	var cells []ScoreCell
	if err := DB.Select(&cells, `
                SELECT cs.student_id, a.id AS assignment_id,
                       MAX(COALESCE(s.override_points, s.points)) AS points
                  FROM class_students cs
                  JOIN assignments a ON a.class_id=cs.class_id
                  LEFT JOIN submissions s ON s.assignment_id=a.id AND s.student_id=cs.student_id
                 WHERE cs.class_id=$1
                 GROUP BY cs.student_id, a.id
                 ORDER BY cs.student_id, a.id`, classID); err != nil {
		return nil, err
	}

	return &ClassProgress{Students: students, Assignments: asg, Scores: cells}, nil
}

// ──────────────────────────────────────────
// File management
// ──────────────────────────────────────────

type ClassFile struct {
	ID        int       `db:"id" json:"id"`
	ClassID   int       `db:"class_id" json:"class_id"`
	ParentID  *int      `db:"parent_id" json:"parent_id"`
	Name      string    `db:"name" json:"name"`
	Path      string    `db:"path" json:"path"`
	IsDir     bool      `db:"is_dir" json:"is_dir"`
	Size      int       `db:"size" json:"size"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type ClassFileWithContent struct {
	ClassFile
	Content []byte `db:"content" json:"content"`
}

func buildFilePath(parentID *int, name string) (string, error) {
	if parentID == nil {
		return "/" + name, nil
	}
	var p string
	if err := DB.Get(&p, `SELECT path FROM class_files WHERE id=$1`, *parentID); err != nil {
		return "", err
	}
	return p + "/" + name, nil
}

func ListFiles(classID int, parentID *int) ([]ClassFile, error) {
	list := []ClassFile{}
	query := `SELECT id,class_id,parent_id,name,path,is_dir,size,created_at,updated_at
                   FROM class_files WHERE class_id=$1`
	args := []any{classID}
	if parentID == nil {
		query += ` AND parent_id IS NULL`
	} else {
		query += ` AND parent_id=$2`
		args = append(args, *parentID)
	}
	query += ` ORDER BY is_dir DESC, name`
	err := DB.Select(&list, query, args...)
	return list, err
}

func SearchFiles(classID int, term string) ([]ClassFile, error) {
	list := []ClassFile{}
	err := DB.Select(&list, `
                SELECT id,class_id,parent_id,name,path,is_dir,size,created_at,updated_at
                  FROM class_files
                 WHERE class_id=$1 AND (name ILIKE $2 OR path ILIKE $2)
                 ORDER BY is_dir DESC, path`,
		classID, "%"+term+"%")
	return list, err
}

func ListNotebooks(classID int) ([]ClassFile, error) {
	list := []ClassFile{}
	err := DB.Select(&list, `SELECT id,class_id,parent_id,name,path,is_dir,size,created_at,updated_at
                FROM class_files
               WHERE class_id=$1 AND NOT is_dir AND lower(name) LIKE '%.ipynb'
               ORDER BY updated_at DESC`, classID)
	return list, err
}

func SaveFile(classID int, parentID *int, name string, data []byte, isDir bool) (*ClassFile, error) {
	if !isDir && len(data) > maxFileSize {
		return nil, fmt.Errorf("file too large")
	}
	path, err := buildFilePath(parentID, name)
	if err != nil {
		return nil, err
	}
	size := len(data)
	var cf ClassFile
	err = DB.QueryRow(`INSERT INTO class_files (class_id,parent_id,name,path,is_dir,content,size)
                        VALUES ($1,$2,$3,$4,$5,$6,$7)
                        RETURNING id,class_id,parent_id,name,path,is_dir,size,created_at,updated_at`,
		classID, parentID, name, path, isDir, data, size).Scan(
		&cf.ID, &cf.ClassID, &cf.ParentID, &cf.Name, &cf.Path, &cf.IsDir, &cf.Size, &cf.CreatedAt, &cf.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &cf, nil
}

func GetFile(id int) (*ClassFileWithContent, error) {
	var cf ClassFileWithContent
	err := DB.Get(&cf, `SELECT id,class_id,parent_id,name,path,is_dir,size,created_at,updated_at,content FROM class_files WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return &cf, nil
}

func RenameFile(id int, newName string) error {
	var f ClassFile
	if err := DB.Get(&f, `SELECT id,class_id,parent_id,name,path FROM class_files WHERE id=$1`, id); err != nil {
		return err
	}
	newPath, err := buildFilePath(f.ParentID, newName)
	if err != nil {
		return err
	}
	tx, err := DB.Beginx()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE class_files SET name=$1, path=$2 WHERE id=$3`, newName, newPath, id); err != nil {
		tx.Rollback()
		return err
	}
	oldPrefix := f.Path + "/"
	newPrefix := newPath + "/"
	rows := []ClassFile{}
	if err := tx.Select(&rows, `SELECT id,path FROM class_files WHERE class_id=$1 AND path LIKE $2`, f.ClassID, oldPrefix+"%"); err == nil {
		for _, r := range rows {
			np := newPrefix + r.Path[len(oldPrefix):]
			if _, err := tx.Exec(`UPDATE class_files SET path=$1 WHERE id=$2`, np, r.ID); err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit()
}

func DeleteFile(id int) error {
	var f ClassFile
	if err := DB.Get(&f, `SELECT class_id,path FROM class_files WHERE id=$1`, id); err != nil {
		return err
	}
	_, err := DB.Exec(`DELETE FROM class_files WHERE class_id=$1 AND (id=$2 OR path LIKE $3)`, f.ClassID, id, f.Path+"/%")
	return err
}

func UpdateFileContent(id int, data []byte) error {
	if len(data) > maxFileSize {
		return fmt.Errorf("file too large")
	}
	_, err := DB.Exec(`UPDATE class_files SET content=$1, size=$2, updated_at=now() WHERE id=$3`, data, len(data), id)
	return err
}

func IsTeacherOfClass(cid, teacherID int) (bool, error) {
	var x int
	err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, cid, teacherID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func IsStudentOfClass(cid, studentID int) (bool, error) {
	var x int
	err := DB.Get(&x, `SELECT 1 FROM class_students WHERE class_id=$1 AND student_id=$2`, cid, studentID)
	if err != nil {
		return false, err
	}
	return true, nil
}

// ──────────────────────────────────────────────────────
// messaging
// ──────────────────────────────────────────────────────

type Message struct {
	ID          int       `db:"id" json:"id"`
	SenderID    int       `db:"sender_id" json:"sender_id"`
	RecipientID int       `db:"recipient_id" json:"recipient_id"`
	Content     string    `db:"content" json:"content"`
	Image       *string   `db:"image" json:"image,omitempty"`
	IsRead      bool      `db:"is_read" json:"is_read"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

func CreateMessage(m *Message) error {
	const q = `INSERT INTO messages (sender_id, recipient_id, content, image)
                    VALUES ($1,$2,$3,$4)
                    RETURNING id, created_at, is_read`
	err := DB.QueryRow(q, m.SenderID, m.RecipientID, m.Content, m.Image).
		Scan(&m.ID, &m.CreatedAt, &m.IsRead)
	if err == nil {
		broadcastMsg(sse.Event{Event: "message", Data: m})
	}
	return err
}

func ListMessages(userID, otherID, limit, offset int) ([]Message, error) {
	msgs := []Message{}
	err := DB.Select(&msgs, `SELECT id,sender_id,recipient_id,content,image,created_at,is_read
                                 FROM messages
                                WHERE (sender_id=$1 AND recipient_id=$2)
                                   OR (sender_id=$2 AND recipient_id=$1)
                                ORDER BY created_at DESC
                                LIMIT $3 OFFSET $4`,
		userID, otherID, limit, offset)
	return msgs, err
}

func MarkMessagesRead(userID, otherID int) error {
	res, err := DB.Exec(`UPDATE messages SET is_read=TRUE
                           WHERE sender_id=$1 AND recipient_id=$2 AND is_read=FALSE`,
		otherID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		broadcastRead(otherID, userID)
	}
	return nil
}

type UserSearch struct {
	ID     int     `db:"id" json:"id"`
	Email  string  `db:"email" json:"email"`
	Name   *string `db:"name" json:"name"`
	Avatar *string `db:"avatar" json:"avatar"`
}

func SearchUsers(term string) ([]UserSearch, error) {
	list := []UserSearch{}
	like := "%" + term + "%"
	err := DB.Select(&list,
		`SELECT id,email,name,avatar FROM users
                  WHERE LOWER(email) LIKE LOWER($1) OR LOWER(COALESCE(name,'')) LIKE LOWER($1)
                  ORDER BY email LIMIT 20`, like)
	return list, err
}

type Conversation struct {
	OtherID     int     `db:"other_id" json:"other_id"`
	Name        *string `db:"name" json:"name"`
	Avatar      *string `db:"avatar" json:"avatar"`
	Email       string  `db:"email" json:"email"`
	UnreadCount int     `db:"unread_count" json:"unread_count"`
	Message
}

func ListRecentConversations(userID, limit int) ([]Conversation, error) {
	list := []Conversation{}
	if limit <= 0 {
		limit = 20
	}
	const q = `
       WITH latest AS (
               SELECT *,
                      CASE WHEN sender_id=$1 THEN recipient_id ELSE sender_id END AS other_id,
                      ROW_NUMBER() OVER (
                              PARTITION BY CASE WHEN sender_id=$1 THEN recipient_id ELSE sender_id END
                              ORDER BY created_at DESC
                      ) AS rn
                 FROM messages
                WHERE sender_id=$1 OR recipient_id=$1
       ), unread AS (
               SELECT sender_id AS other_id, COUNT(*) AS unread_count
                 FROM messages
                WHERE recipient_id=$1 AND is_read=FALSE
                GROUP BY sender_id
       )
       SELECT l.other_id, u.name, u.avatar, u.email,
              l.id, l.sender_id, l.recipient_id, l.content, l.image, l.created_at,
              COALESCE(un.unread_count,0) AS unread_count
         FROM latest l
         JOIN users u ON u.id = l.other_id
         LEFT JOIN unread un ON un.other_id=l.other_id
        WHERE l.rn = 1
        ORDER BY (COALESCE(un.unread_count,0) > 0) DESC, l.created_at DESC
        LIMIT $2`
	err := DB.Select(&list, q, userID, limit)
	return list, err
}
