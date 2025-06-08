package main

import (
	"fmt"
	"time"
)

type User struct {
	ID           int       `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Role         string    `db:"role"`
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
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

type TestCase struct {
	ID             int       `db:"id" json:"id"`
	AssignmentID   int       `db:"assignment_id" json:"assignment_id"`
	Stdin          string    `db:"stdin" json:"stdin"`
	ExpectedStdout string    `db:"expected_stdout" json:"expected_stdout"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// ──────────────────────────────────────────────────────
// admin helpers
// ──────────────────────────────────────────────────────

type UserSummary struct {
	ID        int       `db:"id"         json:"id"`
	Email     string    `db:"email"      json:"email"`
	Role      string    `db:"role"       json:"role"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func ListUsers() ([]UserSummary, error) {
	var list []UserSummary
	err := DB.Select(&list,
		`SELECT id,email,role,created_at
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
          INSERT INTO assignments (title, description, created_by, deadline, max_points, grading_policy, class_id)
          VALUES ($1,$2,$3,$4,$5,$6,$7)
          RETURNING id, created_at, updated_at`
	return DB.QueryRow(q,
		a.Title, a.Description, a.CreatedBy, a.Deadline,
		a.MaxPoints, a.GradingPolicy, a.ClassID,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

// ListAssignments returns all assignments.
func ListAssignments() ([]Assignment, error) {
	var list []Assignment
	err := DB.Select(&list, `
    SELECT id, title, description, created_by, deadline, max_points, grading_policy, created_at, updated_at, class_id
      FROM assignments
     ORDER BY created_at DESC`)
	return list, err
}

// GetAssignment looks up one assignment by ID.
func GetAssignment(id int) (*Assignment, error) {
	var a Assignment
	err := DB.Get(&a, `
    SELECT id, title, description, created_by, deadline, max_points, grading_policy, created_at, updated_at, class_id
      FROM assignments
     WHERE id = $1`, id)
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
           max_points=$4, grading_policy=$5,
           updated_at=now()
     WHERE id=$6`,
		a.Title, a.Description, a.Deadline,
		a.MaxPoints, a.GradingPolicy,
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

func CreateTeacher(email, hash string) error {
	_, err := DB.Exec(`
        INSERT INTO users (email, password_hash, role)
        VALUES ($1,$2,'teacher')`, email, hash)
	return err
}

func CreateClass(c *Class) error {
	return DB.QueryRow(`
        INSERT INTO classes (name, teacher_id)
        VALUES ($1,$2)
        RETURNING id, created_at, updated_at`,
		c.Name, c.TeacherID,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func AddStudentsToClass(classID int, studentIDs []int) error {
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
	var list []Student
	err := DB.Select(&list, `
	    SELECT id, email FROM users
	     WHERE role = 'student'
	     ORDER BY email`)
	return list, err
}

// ──────────────────────────────────────────────────────────────────────────────
// classes – helpers for detail view
// ──────────────────────────────────────────────────────────────────────────────
type Student struct {
	ID    int    `db:"id"    json:"id"`
	Email string `db:"email" json:"email"`
}

type ClassDetail struct {
	Class       `json:"class"`
	Teacher     Student      `json:"teacher"`
	Students    []Student    `json:"students"`
	Assignments []Assignment `json:"assignments"`
}

func GetClassDetail(id int) (*ClassDetail, error) {
	// 1) Class meta -----------------------------------------------------------
	var cls Class
	if err := DB.Get(&cls,
		`SELECT * FROM classes WHERE id = $1`, id); err != nil {
		return nil, err
	}

	// 2) Teacher (one row) -----------------------------------------------------
	var teacher Student // reuse tiny struct {id,email}
	if err := DB.Get(&teacher,
		`SELECT id, email FROM users WHERE id = $1`,
		cls.TeacherID); err != nil {
		return nil, err
	}

	// 3) Students (many) -------------------------------------------------------
	var students []Student
	if err := DB.Select(&students, `
		SELECT u.id, u.email
		  FROM users u
		  JOIN class_students cs ON cs.student_id = u.id
		 WHERE cs.class_id = $1
		 ORDER BY u.email`,
		id); err != nil {
		return nil, err
	}

	// 4) Assignments (many) ----------------------------------------------------
	var asg []Assignment
	if err := DB.Select(&asg, `
                SELECT id, title, description, created_by, deadline,
                       max_points, grading_policy,
                       created_at, updated_at, class_id
                  FROM assignments
                 WHERE class_id = $1
                 ORDER BY deadline ASC`,
		id); err != nil {
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

func RemoveStudentFromClass(classID, studentID int) error {
	_, err := DB.Exec(`DELETE FROM class_students
                        WHERE class_id=$1 AND student_id=$2`,
		classID, studentID)
	return err
}

func DeleteUser(id int) error {
	_, err := DB.Exec(`DELETE FROM users WHERE id=$1`, id)
	return err
}

func ListSubmissionsForStudent(studentID int) ([]Submission, error) {
	var subs []Submission
	err := DB.Select(&subs, `
               SELECT id, assignment_id, student_id, code_path, code_content, status, created_at, updated_at
                 FROM submissions
                WHERE student_id = $1
                ORDER BY created_at DESC`, studentID)
	return subs, err
}

func CreateSubmission(s *Submission) error {
	const q = `
          INSERT INTO submissions (assignment_id, student_id, code_path, code_content)
          VALUES ($1,$2,$3,$4)
          RETURNING id, status, created_at, updated_at`
	return DB.QueryRow(q, s.AssignmentID, s.StudentID, s.CodePath, s.CodeContent).
		Scan(&s.ID, &s.Status, &s.CreatedAt, &s.UpdatedAt)
}

func ListSubmissionsForAssignmentAndStudent(aid, sid int) ([]Submission, error) {
	var subs []Submission
	err := DB.Select(&subs, `
               SELECT id, assignment_id, student_id, code_path, code_content, status, created_at, updated_at
                 FROM submissions
                WHERE assignment_id=$1 AND student_id=$2
                ORDER BY created_at DESC`, aid, sid)
	return subs, err
}

func CreateTestCase(tc *TestCase) error {
	const q = `
          INSERT INTO test_cases (assignment_id, stdin, expected_stdout)
          VALUES ($1,$2,$3)
          RETURNING id, created_at, updated_at`
	return DB.QueryRow(q, tc.AssignmentID, tc.Stdin, tc.ExpectedStdout).
		Scan(&tc.ID, &tc.CreatedAt, &tc.UpdatedAt)
}

func ListTestCases(assignmentID int) ([]TestCase, error) {
	var list []TestCase
	err := DB.Select(&list, `
                SELECT id, assignment_id, stdin, expected_stdout, created_at, updated_at
                  FROM test_cases
                 WHERE assignment_id = $1
                 ORDER BY id`, assignmentID)
	return list, err
}
