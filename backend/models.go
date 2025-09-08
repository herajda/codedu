package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const maxFileSize = 20 * 1024 * 1024 // 20 MB

var ErrBlocked = errors.New("blocked")

type User struct {
	ID           uuid.UUID `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Name         *string   `db:"name"`
	Avatar       *string   `db:"avatar"`
	Role         string    `db:"role"`
	Theme        string    `db:"theme"`
	BkClass      *string   `db:"bk_class"`
	BkUID        *string   `db:"bk_uid"`
	CreatedAt    time.Time `db:"created_at"`
}

type Assignment struct {
	ID            uuid.UUID `db:"id" json:"id"`
	Title         string    `db:"title" json:"title"`
	Description   string    `db:"description" json:"description"`
	CreatedBy     uuid.UUID `db:"created_by" json:"created_by"`
	Deadline      time.Time `db:"deadline" json:"deadline"`
	MaxPoints     int       `db:"max_points" json:"max_points"`
	GradingPolicy string    `db:"grading_policy" json:"grading_policy"`
	Published     bool      `db:"published" json:"published"`
	ShowTraceback bool      `db:"show_traceback" json:"show_traceback"`
	ManualReview  bool      `db:"manual_review" json:"manual_review"`
	TemplatePath  *string   `db:"template_path" json:"template_path"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
	ClassID       uuid.UUID `db:"class_id" json:"class_id"`

	// LLM-interactive testing configuration
	LLMInteractive     bool    `db:"llm_interactive" json:"llm_interactive"`
	LLMFeedback        bool    `db:"llm_feedback" json:"llm_feedback"`
	LLMAutoAward       bool    `db:"llm_auto_award" json:"llm_auto_award"`
	LLMScenariosRaw    *string `db:"llm_scenarios_json" json:"llm_scenarios_json"`
	LLMStrictness      int     `db:"llm_strictness" json:"llm_strictness"`
	LLMRubric          *string `db:"llm_rubric" json:"llm_rubric"`
	LLMTeacherBaseline *string `db:"llm_teacher_baseline_json" json:"llm_teacher_baseline_json"`

	// Second deadline feature
	SecondDeadline   *time.Time `db:"second_deadline" json:"second_deadline"`
	LatePenaltyRatio float64    `db:"late_penalty_ratio" json:"late_penalty_ratio"`
}
type Class struct {
	ID        uuid.UUID `db:"id"        json:"id"`
	Name      string    `db:"name"      json:"name"`
	TeacherID uuid.UUID `db:"teacher_id" json:"teacher_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type Submission struct {
	ID               uuid.UUID `db:"id" json:"id"`
	AssignmentID     uuid.UUID `db:"assignment_id" json:"assignment_id"`
	StudentID        uuid.UUID `db:"student_id" json:"student_id"`
	CodePath         string    `db:"code_path" json:"code_path"`
	CodeContent      string    `db:"code_content" json:"code_content"`
	Status           string    `db:"status" json:"status"`
	Points           *float64  `db:"points" json:"points"`
	OverridePts      *float64  `db:"override_points" json:"override_points"`
	IsTeacherRun     bool      `db:"is_teacher_run" json:"is_teacher_run"`
	ManuallyAccepted bool      `db:"manually_accepted" json:"manually_accepted"`
	Late             bool      `db:"late" json:"late"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
	AttemptNumber    *int      `db:"attempt_number" json:"attempt_number,omitempty"`
}

type TestCase struct {
	ID             uuid.UUID `db:"id" json:"id"`
	AssignmentID   uuid.UUID `db:"assignment_id" json:"assignment_id"`
	Stdin          string    `db:"stdin" json:"stdin"`
	ExpectedStdout string    `db:"expected_stdout" json:"expected_stdout"`
	Weight         float64   `db:"weight" json:"weight"`
	TimeLimitSec   float64   `db:"time_limit_sec" json:"time_limit_sec"`
	MemoryLimitKB  int       `db:"memory_limit_kb" json:"memory_limit_kb"`
	UnittestCode   *string   `db:"unittest_code" json:"unittest_code"`
	UnittestName   *string   `db:"unittest_name" json:"unittest_name"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// ──────────────────────────────────────────────────────
// admin helpers
// ──────────────────────────────────────────────────────

type UserSummary struct {
	ID        uuid.UUID `db:"id"         json:"id"`
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

func UpdateUserRole(id uuid.UUID, role string) error {
	// only three legal roles
	switch role {
	case "student", "teacher", "admin":
	default:
		return fmt.Errorf("invalid role")
	}
	_, err := DB.Exec(`UPDATE users SET role=$1 WHERE id=$2`, role, id)
	return err
}

func GetUser(id uuid.UUID) (*User, error) {
	var u User
	err := DB.Get(&u, `SELECT id, email, password_hash, name, avatar, role, theme, bk_class, bk_uid, created_at
                FROM users WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func UpdateUserProfile(id uuid.UUID, name, avatar, theme *string) error {
	_, err := DB.Exec(`UPDATE users SET name=COALESCE($1,name), avatar=COALESCE($2,avatar), theme=COALESCE($3,theme) WHERE id=$4`, name, avatar, theme, id)
	return err
}

// AssignRandomAvatarsToUsersWithout assigns a random avatar from the provided list
// to all users whose avatar is NULL. It is safe to call on startup.
func AssignRandomAvatarsToUsersWithout(catalog []string) error {
	if len(catalog) == 0 {
		return nil
	}
	type row struct{ ID uuid.UUID }
	var rows []row
	if err := DB.Select(&rows, `SELECT id FROM users WHERE avatar IS NULL`); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	for i, r := range rows {
		pick := catalog[int(time.Now().UnixNano()+int64(i))%len(catalog)]
		_, _ = DB.Exec(`UPDATE users SET avatar=$1 WHERE id=$2`, pick, r.ID)
	}
	return nil
}

func UpdateUserPassword(id uuid.UUID, hash string) error {
	_, err := DB.Exec(`UPDATE users SET password_hash=$1 WHERE id=$2`, hash, id)
	return err
}

// LinkLocalAccount sets a new email and password hash for an existing user.
func LinkLocalAccount(id uuid.UUID, email, hash string) error {
	_, err := DB.Exec(`UPDATE users SET email=$1, password_hash=$2 WHERE id=$3`, email, hash, id)
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
          INSERT INTO assignments (title, description, created_by, deadline, max_points, grading_policy, published, show_traceback, manual_review, template_path, class_id, second_deadline, late_penalty_ratio)
          VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
          RETURNING id, created_at, updated_at`
	return DB.QueryRow(q,
		a.Title, a.Description, a.CreatedBy, a.Deadline,
		a.MaxPoints, a.GradingPolicy, a.Published, a.ShowTraceback, a.ManualReview, a.TemplatePath, a.ClassID,
		a.SecondDeadline, a.LatePenaltyRatio,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

// ListAssignments returns all assignments.
func ListAssignments(role string, userID uuid.UUID) ([]Assignment, error) {
	list := []Assignment{}
	query := `
    SELECT a.id, a.title, a.description, a.created_by, a.deadline,
           a.max_points, a.grading_policy, a.published, a.show_traceback, a.manual_review, a.template_path,
           a.created_at, a.updated_at, a.class_id,
           COALESCE(a.llm_interactive,false) AS llm_interactive,
           COALESCE(a.llm_feedback,false) AS llm_feedback,
           COALESCE(a.llm_auto_award,true) AS llm_auto_award,
           a.llm_scenarios_json,
           COALESCE(a.llm_strictness,50) AS llm_strictness,
           a.llm_rubric,
           a.llm_teacher_baseline_json,
           a.second_deadline,
           COALESCE(a.late_penalty_ratio,0.5) AS late_penalty_ratio
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
func GetAssignment(id uuid.UUID) (*Assignment, error) {
	var a Assignment
	err := DB.Get(&a, `
    SELECT id, title, description, created_by, deadline, max_points, grading_policy, published, show_traceback, manual_review, template_path, created_at, updated_at, class_id,
           COALESCE(llm_interactive,false) AS llm_interactive,
           COALESCE(llm_feedback,false) AS llm_feedback,
           COALESCE(llm_auto_award,true) AS llm_auto_award,
           llm_scenarios_json,
           COALESCE(llm_strictness,50) AS llm_strictness,
           llm_rubric,
           llm_teacher_baseline_json,
           second_deadline,
           COALESCE(late_penalty_ratio,0.5) AS late_penalty_ratio
      FROM assignments
     WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAssignmentForSubmission retrieves the assignment associated with a submission.
func GetAssignmentForSubmission(subID uuid.UUID) (*Assignment, error) {
	var a Assignment
	err := DB.Get(&a, `
        SELECT a.id, a.title, a.description, a.created_by, a.deadline,
               a.max_points, a.grading_policy, a.published, a.show_traceback, a.manual_review, a.template_path,
               a.created_at, a.updated_at, a.class_id,
               COALESCE(a.llm_interactive,false) AS llm_interactive,
               COALESCE(a.llm_feedback,false) AS llm_feedback,
               COALESCE(a.llm_auto_award,true) AS llm_auto_award,
               a.llm_scenarios_json,
               COALESCE(a.llm_strictness,50) AS llm_strictness,
               a.llm_rubric,
               a.llm_teacher_baseline_json,
               a.second_deadline,
               COALESCE(a.late_penalty_ratio,0.5) AS late_penalty_ratio
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
           max_points=$4, grading_policy=$5, show_traceback=$6, manual_review=$7,
           llm_interactive=$8, llm_feedback=$9, llm_auto_award=$10, llm_scenarios_json=$11,
           llm_strictness=$12, llm_rubric=$13, llm_teacher_baseline_json=$14,
           second_deadline=$15, late_penalty_ratio=$16,
           updated_at=now()
     WHERE id=$17`,
		a.Title, a.Description, a.Deadline,
		a.MaxPoints, a.GradingPolicy, a.ShowTraceback, a.ManualReview,
		a.LLMInteractive, a.LLMFeedback, a.LLMAutoAward, a.LLMScenariosRaw,
		a.LLMStrictness, a.LLMRubric, a.LLMTeacherBaseline,
		a.SecondDeadline, a.LatePenaltyRatio,
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
func DeleteAssignment(id uuid.UUID) error {
    _, err := DB.Exec(`DELETE FROM assignments WHERE id=$1`, id)
    return err
}

// SetAssignmentPublished updates the published flag on an assignment.
func SetAssignmentPublished(id uuid.UUID, published bool) error {
	_, err := DB.Exec(`UPDATE assignments SET published=$1, updated_at=now() WHERE id=$2`, published, id)
	return err
}

func UpdateAssignmentTemplate(id uuid.UUID, path *string) error {
    _, err := DB.Exec(`UPDATE assignments SET template_path=$1, updated_at=now() WHERE id=$2`, path, id)
    return err
}

// CloneAssignmentWithTests duplicates an assignment (including test cases and
// template/settings) into a target class and returns the new assignment ID.
func CloneAssignmentWithTests(sourceID, targetClassID, createdBy uuid.UUID) (uuid.UUID, error) {
    src, err := GetAssignment(sourceID)
    if err != nil {
        return uuid.Nil, err
    }
    // Insert new assignment copying most fields; do not publish by default
    dst := &Assignment{
        ClassID:          targetClassID,
        Title:            src.Title,
        Description:      src.Description,
        Deadline:         src.Deadline,
        MaxPoints:        src.MaxPoints,
        GradingPolicy:    src.GradingPolicy,
        Published:        false,
        ShowTraceback:    src.ShowTraceback,
        ManualReview:     src.ManualReview,
        TemplatePath:     src.TemplatePath,
        CreatedBy:        createdBy,
        SecondDeadline:   src.SecondDeadline,
        LatePenaltyRatio: src.LatePenaltyRatio,
        // LLM fields applied post-insert via UpdateAssignment
        LLMInteractive:     src.LLMInteractive,
        LLMFeedback:        src.LLMFeedback,
        LLMAutoAward:       src.LLMAutoAward,
        LLMScenariosRaw:    src.LLMScenariosRaw,
        LLMStrictness:      src.LLMStrictness,
        LLMRubric:          src.LLMRubric,
        LLMTeacherBaseline: src.LLMTeacherBaseline,
    }
    if err := CreateAssignment(dst); err != nil {
        return uuid.Nil, err
    }
    // Apply LLM fields to match source
    if err := UpdateAssignment(dst); err != nil {
        return uuid.Nil, err
    }
    // Copy test cases
    tests, err := ListTestCases(sourceID)
    if err != nil {
        return uuid.Nil, err
    }
    for _, t := range tests {
        tc := &TestCase{
            AssignmentID:   dst.ID,
            Stdin:          t.Stdin,
            ExpectedStdout: t.ExpectedStdout,
            Weight:         t.Weight,
            TimeLimitSec:   t.TimeLimitSec,
            MemoryLimitKB:  t.MemoryLimitKB,
            UnittestCode:   t.UnittestCode,
            UnittestName:   t.UnittestName,
        }
        if err := CreateTestCase(tc); err != nil {
            return uuid.Nil, err
        }
    }
    return dst.ID, nil
}

// IsTeacherOfAssignment checks whether the given teacher owns the class the
// assignment belongs to.
func IsTeacherOfAssignment(aid, teacherID uuid.UUID) (bool, error) {
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
func IsStudentOfAssignment(aid, studentID uuid.UUID) (bool, error) {
	var x int
	err := DB.Get(&x, `SELECT 1 FROM assignments a JOIN class_students cs ON cs.class_id=a.class_id
                WHERE a.id=$1 AND cs.student_id=$2`, aid, studentID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func CreateTeacher(email, hash string, name, bkUID *string) error {
    _, err := DB.Exec(`
        INSERT INTO users (email, password_hash, name, role, bk_uid)
        VALUES ($1,$2,$3,'teacher',$4)`, email, hash, name, bkUID)
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
func createStudentWithID(email, hash string, name, bkClass, bkUID *string) (uuid.UUID, error) {
	var id uuid.UUID
	err := DB.QueryRow(`
                INSERT INTO users (email, password_hash, name, role, bk_class, bk_uid)
                VALUES ($1,$2,$3,'student',$4,$5)
                RETURNING id`, email, hash, name, bkClass, bkUID).Scan(&id)
	return id, err
}

// EnsureStudentForBk ensures a student exists for the given Bakaláři UID
// and returns the local user ID.
func EnsureStudentForBk(uid, cls, name string) (uuid.UUID, error) {
	if len(uid) > 3 {
		uid = uid[len(uid)-3:]
	}
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

func UpdateClassName(id uuid.UUID, teacherID uuid.UUID, name string) error {
	if teacherID != uuid.Nil {
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

// UpdateClassTeacher changes ownership of a class to a different teacher.
// Admins may transfer any class. When teacherID is provided (non-zero), it is validated by the caller.
func UpdateClassTeacher(id uuid.UUID, newTeacherID uuid.UUID) error {
	// Ensure the target user exists and is a teacher
	var role string
	if err := DB.Get(&role, `SELECT role FROM users WHERE id=$1`, newTeacherID); err != nil {
		return err
	}
	if role != "teacher" {
		return fmt.Errorf("user is not a teacher")
	}
	res, err := DB.Exec(`UPDATE classes SET teacher_id=$1, updated_at=now() WHERE id=$2`, newTeacherID, id)
	if err != nil {
		return err
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return fmt.Errorf("no rows updated")
	}
	return nil
}

func DeleteClass(id uuid.UUID, teacherID uuid.UUID) error {
	if teacherID != uuid.Nil {
		var x int
		if err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, id, teacherID); err != nil {
			return err
		}
	}
	_, err := DB.Exec(`DELETE FROM classes WHERE id=$1`, id)
	return err
}

func AddStudentsToClass(classID, teacherID uuid.UUID, studentIDs []uuid.UUID) error {
	if teacherID != uuid.Nil {
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

func ListClassesForTeacher(teacherID uuid.UUID) ([]Class, error) {
	var cls []Class
	err := DB.Select(&cls, `
                SELECT * FROM classes
                 WHERE teacher_id = $1
                 ORDER BY created_at DESC`, teacherID)
	return cls, err
}

func ListClassesForStudent(studentID uuid.UUID) ([]Class, error) {
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
	ID    uuid.UUID `db:"id"    json:"id"`
	Email string    `db:"email" json:"email"`
	Name  *string   `db:"name"  json:"name"`
}

type ClassDetail struct {
	Class       `json:"class"`
	Teacher     Student      `json:"teacher"`
	Students    []Student    `json:"students"`
	Assignments []Assignment `json:"assignments"`
}

func GetClassDetail(id uuid.UUID, role string, userID uuid.UUID) (*ClassDetail, error) {
    // 1) Class meta -------------------------------------------------------
    var cls Class
    switch role {
    case "teacher":
        if id == TeacherGroupID {
            if err := DB.Get(&cls, `SELECT * FROM classes WHERE id=$1`, id); err != nil {
                return nil, err
            }
        } else {
            if err := DB.Get(&cls, `SELECT * FROM classes WHERE id=$1 AND teacher_id=$2`, id, userID); err != nil {
                return nil, err
            }
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
                       created_at, updated_at, class_id,
                       COALESCE(llm_interactive,false) AS llm_interactive,
                       COALESCE(llm_feedback,false) AS llm_feedback,
                       COALESCE(llm_auto_award,true) AS llm_auto_award,
                       llm_scenarios_json
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

func RemoveStudentFromClass(classID, teacherID, studentID uuid.UUID) error {
	if teacherID == uuid.Nil {
		_, err := DB.Exec(`DELETE FROM class_students WHERE class_id=$1 AND student_id=$2`, classID, studentID)
		return err
	}
	_, err := DB.Exec(`DELETE FROM class_students cs USING classes c
                        WHERE cs.class_id=$1 AND cs.student_id=$2 AND c.id=cs.class_id AND c.teacher_id=$3`,
		classID, studentID, teacherID)
	return err
}

func DeleteUser(id uuid.UUID) error {
	_, err := DB.Exec(`DELETE FROM users WHERE id=$1`, id)
	return err
}

func ListSubmissionsForStudent(studentID uuid.UUID) ([]Submission, error) {
	subs := []Submission{}
	err := DB.Select(&subs, `
               SELECT id, assignment_id, student_id, code_path, code_content, status, points, override_points, is_teacher_run, manually_accepted, late, created_at, updated_at,
                      ROW_NUMBER() OVER (PARTITION BY assignment_id, student_id ORDER BY created_at ASC, id ASC) AS attempt_number
                 FROM submissions
                WHERE student_id = $1
                ORDER BY created_at DESC`, studentID)
	return subs, err
}

func CreateSubmission(s *Submission) error {
	const q = `
          INSERT INTO submissions (assignment_id, student_id, code_path, code_content, is_teacher_run)
          SELECT $1,$2,$3,$4,$5
            WHERE EXISTS (
                SELECT 1 FROM assignments a
                JOIN class_students cs ON cs.class_id = a.class_id
               WHERE a.id=$1 AND cs.student_id=$2)
          RETURNING id, status, created_at, updated_at`
	return DB.QueryRow(q, s.AssignmentID, s.StudentID, s.CodePath, s.CodeContent, s.IsTeacherRun).
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

func ListSubmissionsForAssignmentAndStudent(aid, sid uuid.UUID) ([]SubmissionWithReason, error) {
	subs := []SubmissionWithReason{}
	err := DB.Select(&subs, `
               SELECT id, assignment_id, student_id, code_path, code_content, status, points, override_points, is_teacher_run, manually_accepted, late, created_at, updated_at,
                      ROW_NUMBER() OVER (PARTITION BY assignment_id, student_id ORDER BY created_at ASC, id ASC) AS attempt_number,
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
func ListSubmissionsForAssignment(aid uuid.UUID) ([]SubmissionWithStudent, error) {
	subs := []SubmissionWithStudent{}
	err := DB.Select(&subs, `
               SELECT s.id, s.assignment_id, s.student_id, s.code_path, s.code_content, s.status, s.points, s.override_points, s.is_teacher_run, s.manually_accepted, s.late, s.created_at, s.updated_at,
                     ROW_NUMBER() OVER (PARTITION BY s.assignment_id, s.student_id ORDER BY s.created_at ASC, s.id ASC) AS attempt_number,
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

// Teacher runs listing: include teacher email/name and is_teacher_run filter
func ListTeacherRunsForAssignment(aid uuid.UUID) ([]SubmissionWithStudent, error) {
	subs := []SubmissionWithStudent{}
	err := DB.Select(&subs, `
                SELECT s.id, s.assignment_id, s.student_id, s.code_path, s.code_content, s.status, s.points, s.override_points, s.is_teacher_run, s.manually_accepted, s.late, s.created_at, s.updated_at,
                       ROW_NUMBER() OVER (PARTITION BY s.assignment_id, s.student_id ORDER BY s.created_at ASC, s.id ASC) AS attempt_number,
                       u.email, u.name,
                       (SELECT r.status FROM results r
                          WHERE r.submission_id = s.id AND r.status <> 'passed'
                           ORDER BY r.id LIMIT 1) AS failure_reason
                  FROM submissions s
                  JOIN users u ON u.id = s.student_id
                 WHERE s.assignment_id = $1 AND s.is_teacher_run = TRUE
                 ORDER BY s.created_at DESC`, aid)
	return subs, err
}

// ListTeacherRunsForAssignmentByUser returns teacher runs for a given assignment
// filtered to a specific teacher (user) ID. This ensures teacher runs are not
// shared across teachers in the UI.
func ListTeacherRunsForAssignmentByUser(aid, uid uuid.UUID) ([]SubmissionWithStudent, error) {
    subs := []SubmissionWithStudent{}
    err := DB.Select(&subs, `
                SELECT s.id, s.assignment_id, s.student_id, s.code_path, s.code_content, s.status, s.points, s.override_points, s.is_teacher_run, s.manually_accepted, s.late, s.created_at, s.updated_at,
                       ROW_NUMBER() OVER (PARTITION BY s.assignment_id, s.student_id ORDER BY s.created_at ASC, s.id ASC) AS attempt_number,
                       u.email, u.name,
                       (SELECT r.status FROM results r
                          WHERE r.submission_id = s.id AND r.status <> 'passed'
                           ORDER BY r.id LIMIT 1) AS failure_reason
                  FROM submissions s
                  JOIN users u ON u.id = s.student_id
                 WHERE s.assignment_id = $1 AND s.is_teacher_run = TRUE AND s.student_id = $2
                 ORDER BY s.created_at DESC`, aid, uid)
    return subs, err
}

func CreateTestCase(tc *TestCase) error {
	if tc.TimeLimitSec == 0 {
		tc.TimeLimitSec = 1
	}
	const q = `
         INSERT INTO test_cases (assignment_id, stdin, expected_stdout, weight, time_limit_sec, unittest_code, unittest_name)
         VALUES ($1,$2,$3,$4,$5,$6,$7)
         RETURNING id, weight, time_limit_sec, memory_limit_kb, unittest_code, unittest_name, created_at, updated_at`
	return DB.QueryRow(q, tc.AssignmentID, tc.Stdin, tc.ExpectedStdout, tc.Weight, tc.TimeLimitSec, tc.UnittestCode, tc.UnittestName).
		Scan(&tc.ID, &tc.Weight, &tc.TimeLimitSec, &tc.MemoryLimitKB, &tc.UnittestCode, &tc.UnittestName, &tc.CreatedAt, &tc.UpdatedAt)
}

// UpdateTestCase modifies stdin/stdout/time limit of an existing test case.
func UpdateTestCase(tc *TestCase) error {
	if tc.TimeLimitSec == 0 {
		tc.TimeLimitSec = 1
	}
	res, err := DB.Exec(`
                UPDATE test_cases
                   SET stdin=$1, expected_stdout=$2, weight=$3, time_limit_sec=$4,
                       unittest_code=COALESCE($5, unittest_code), unittest_name=COALESCE($6, unittest_name),
                       updated_at=now()
                 WHERE id=$7`,
		tc.Stdin, tc.ExpectedStdout, tc.Weight, tc.TimeLimitSec, tc.UnittestCode, tc.UnittestName, tc.ID)
	if err != nil {
		return err
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return fmt.Errorf("no rows updated")
	}
	return nil
}

func ListTestCases(assignmentID uuid.UUID) ([]TestCase, error) {
	list := []TestCase{}
	err := DB.Select(&list, `
               SELECT id, assignment_id, stdin, expected_stdout, weight, time_limit_sec, memory_limit_kb, unittest_code, unittest_name, created_at, updated_at
                 FROM test_cases
                 WHERE assignment_id = $1
                 ORDER BY id`, assignmentID)
	return list, err
}

func DeleteTestCase(id uuid.UUID) error {
	_, err := DB.Exec(`DELETE FROM test_cases WHERE id=$1`, id)
	return err
}

// DeleteAllTestCasesForAssignment removes all test cases for a given assignment.
func DeleteAllTestCasesForAssignment(assignmentID uuid.UUID) error {
	_, err := DB.Exec(`DELETE FROM test_cases WHERE assignment_id=$1`, assignmentID)
	return err
}

// ──────────────────────────────────────────────────────
// submissions – helpers for grading
// ──────────────────────────────────────────────────────

// Result represents outcome of one test case execution.
type Result struct {
	ID           uuid.UUID `db:"id" json:"id"`
	SubmissionID uuid.UUID `db:"submission_id" json:"submission_id"`
	TestCaseID   uuid.UUID `db:"test_case_id" json:"test_case_id"`
	Status       string    `db:"status" json:"status"`
	ActualStdout string    `db:"actual_stdout" json:"actual_stdout"`
	Stderr       string    `db:"stderr" json:"stderr"`
	ExitCode     int       `db:"exit_code" json:"exit_code"`
	RuntimeMS    int       `db:"runtime_ms" json:"runtime_ms"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

// LLMRun stores artifacts from an LLM-interactive testing run for a submission.
type LLMRun struct {
	ID              uuid.UUID `db:"id" json:"id"`
	SubmissionID    uuid.UUID `db:"submission_id" json:"submission_id"`
	SmokeOK         bool      `db:"smoke_ok" json:"smoke_ok"`
	ReviewJSON      *string   `db:"review_json" json:"review_json,omitempty"`
	InteractiveJSON *string   `db:"interactive_json" json:"interactive_json,omitempty"`
	Transcript      *string   `db:"transcript" json:"transcript,omitempty"`
	Verdict         *string   `db:"verdict" json:"verdict,omitempty"`
	Reason          *string   `db:"reason" json:"reason,omitempty"`
	ModelName       *string   `db:"model_name" json:"model_name,omitempty"`
	ToolCalls       *int      `db:"tool_calls" json:"tool_calls,omitempty"`
	WallTimeMS      *int      `db:"wall_time_ms" json:"wall_time_ms,omitempty"`
	OutputSize      *int      `db:"output_size" json:"output_size,omitempty"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}

func CreateLLMRun(r *LLMRun) error {
	const q = `
        INSERT INTO llm_runs (submission_id, smoke_ok, review_json, interactive_json, transcript, verdict, reason, model_name, tool_calls, wall_time_ms, output_size)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
        RETURNING id, created_at`
	return DB.QueryRow(q, r.SubmissionID, r.SmokeOK, r.ReviewJSON, r.InteractiveJSON, r.Transcript, r.Verdict, r.Reason, r.ModelName, r.ToolCalls, r.WallTimeMS, r.OutputSize).
		Scan(&r.ID, &r.CreatedAt)
}

func GetLatestLLMRun(subID uuid.UUID) (*LLMRun, error) {
	var r LLMRun
	err := DB.Get(&r, `SELECT id, submission_id, smoke_ok, review_json, interactive_json, transcript, verdict, reason, model_name, tool_calls, wall_time_ms, output_size, created_at
                         FROM llm_runs WHERE submission_id=$1 ORDER BY id DESC LIMIT 1`, subID)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func GetSubmission(id uuid.UUID) (*Submission, error) {
	var s Submission
	err := DB.Get(&s, `
        SELECT id, assignment_id, student_id, code_path, code_content, status, points, override_points, is_teacher_run, manually_accepted, late, created_at, updated_at,
               attempt_number
          FROM (
            SELECT id, assignment_id, student_id, code_path, code_content, status, points, override_points, is_teacher_run, manually_accepted, late, created_at, updated_at,
                   ROW_NUMBER() OVER (PARTITION BY assignment_id, student_id ORDER BY created_at ASC, id ASC) AS attempt_number
              FROM submissions
          ) s
         WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func UpdateSubmissionStatus(id uuid.UUID, status string) error {
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

func ListResultsForSubmission(subID uuid.UUID) ([]Result, error) {
	list := []Result{}
	err := DB.Select(&list, `
        SELECT id, submission_id, test_case_id, status, actual_stdout, stderr, exit_code, runtime_ms, created_at
          FROM results
         WHERE submission_id=$1
         ORDER BY id`, subID)
	return list, err
}

func SetSubmissionPoints(id uuid.UUID, pts float64) error {
	_, err := DB.Exec(`UPDATE submissions SET points=$1 WHERE id=$2`, pts, id)
	return err
}

func SetSubmissionLate(id uuid.UUID, late bool) error {
	_, err := DB.Exec(`UPDATE submissions SET late=$1 WHERE id=$2`, late, id)
	return err
}

func SetSubmissionOverridePoints(id uuid.UUID, pts *float64) error {
	_, err := DB.Exec(`UPDATE submissions SET override_points=$1 WHERE id=$2`, pts, id)
	return err
}

func SetSubmissionManualAccept(id uuid.UUID, accepted bool) error {
	_, err := DB.Exec(`UPDATE submissions SET manually_accepted=$1, updated_at=now() WHERE id=$2`, accepted, id)
	return err
}

type ScoreCell struct {
	StudentID    uuid.UUID `db:"student_id" json:"student_id"`
	AssignmentID uuid.UUID `db:"assignment_id" json:"assignment_id"`
	Points       *float64  `db:"points" json:"points"`
}

type ClassProgress struct {
	Students    []Student    `json:"students"`
	Assignments []Assignment `json:"assignments"`
	Scores      []ScoreCell  `json:"scores"`
}

// GetClassProgress returns score cells for each student/assignment pair in a class.
func GetClassProgress(classID uuid.UUID) (*ClassProgress, error) {
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
                       created_at, updated_at, class_id,
                       COALESCE(llm_interactive,false) AS llm_interactive,
                       COALESCE(llm_feedback,false) AS llm_feedback,
                       COALESCE(llm_auto_award,true) AS llm_auto_award,
                       llm_scenarios_json,
                       second_deadline,
                       COALESCE(late_penalty_ratio,0.5) AS late_penalty_ratio
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
    ID        uuid.UUID  `db:"id" json:"id"`
    ClassID   uuid.UUID  `db:"class_id" json:"class_id"`
    ParentID  *uuid.UUID `db:"parent_id" json:"parent_id"`
    Name      string     `db:"name" json:"name"`
    Path      string     `db:"path" json:"path"`
    IsDir     bool       `db:"is_dir" json:"is_dir"`
    AssignmentID *uuid.UUID `db:"assignment_id" json:"assignment_id,omitempty"`
    Size      int        `db:"size" json:"size"`
    CreatedAt time.Time  `db:"created_at" json:"created_at"`
    UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

type ClassFileWithContent struct {
	ClassFile
	Content []byte `db:"content" json:"content"`
}

func buildFilePath(parentID *uuid.UUID, name string) (string, error) {
	if parentID == nil {
		return "/" + name, nil
	}
	var p string
	if err := DB.Get(&p, `SELECT path FROM class_files WHERE id=$1`, *parentID); err != nil {
		return "", err
	}
	return p + "/" + name, nil
}

func ListFiles(classID uuid.UUID, parentID *uuid.UUID) ([]ClassFile, error) {
    list := []ClassFile{}
    query := `SELECT id,class_id,parent_id,name,path,is_dir,assignment_id,size,created_at,updated_at
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

func SearchFiles(classID uuid.UUID, term string) ([]ClassFile, error) {
    list := []ClassFile{}
    err := DB.Select(&list, `
                SELECT id,class_id,parent_id,name,path,is_dir,assignment_id,size,created_at,updated_at
                  FROM class_files
                 WHERE class_id=$1 AND (name ILIKE $2 OR path ILIKE $2)
                 ORDER BY is_dir DESC, path`,
        classID, "%"+term+"%")
    return list, err
}

func ListNotebooks(classID uuid.UUID) ([]ClassFile, error) {
	list := []ClassFile{}
	err := DB.Select(&list, `SELECT id,class_id,parent_id,name,path,is_dir,size,created_at,updated_at
                FROM class_files
               WHERE class_id=$1 AND NOT is_dir AND lower(name) LIKE '%.ipynb'
               ORDER BY updated_at DESC`, classID)
	return list, err
}

func SaveFile(classID uuid.UUID, parentID *uuid.UUID, name string, data []byte, isDir bool) (*ClassFile, error) {
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
                        RETURNING id,class_id,parent_id,name,path,is_dir,assignment_id,size,created_at,updated_at`,
        classID, parentID, name, path, isDir, data, size).Scan(
        &cf.ID, &cf.ClassID, &cf.ParentID, &cf.Name, &cf.Path, &cf.IsDir, &cf.AssignmentID, &cf.Size, &cf.CreatedAt, &cf.UpdatedAt)
    if err != nil {
        return nil, err
    }
    return &cf, nil
}

// SaveAssignmentRef creates a non-directory file entry that references an assignment.
func SaveAssignmentRef(classID uuid.UUID, parentID *uuid.UUID, name string, assignmentID uuid.UUID) (*ClassFile, error) {
    path, err := buildFilePath(parentID, name)
    if err != nil {
        return nil, err
    }
    var cf ClassFile
    err = DB.QueryRow(`INSERT INTO class_files (class_id,parent_id,name,path,is_dir,assignment_id,content,size)
                        VALUES ($1,$2,$3,$4,FALSE,$5,NULL,0)
                        RETURNING id,class_id,parent_id,name,path,is_dir,assignment_id,size,created_at,updated_at`,
        classID, parentID, name, path, assignmentID).Scan(
        &cf.ID, &cf.ClassID, &cf.ParentID, &cf.Name, &cf.Path, &cf.IsDir, &cf.AssignmentID, &cf.Size, &cf.CreatedAt, &cf.UpdatedAt)
    if err != nil {
        return nil, err
    }
    return &cf, nil
}

func GetFile(id uuid.UUID) (*ClassFileWithContent, error) {
	var cf ClassFileWithContent
	err := DB.Get(&cf, `SELECT id,class_id,parent_id,name,path,is_dir,size,created_at,updated_at,content FROM class_files WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return &cf, nil
}

func RenameFile(id uuid.UUID, newName string) error {
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

func DeleteFile(id uuid.UUID) error {
	var f ClassFile
	if err := DB.Get(&f, `SELECT class_id,path FROM class_files WHERE id=$1`, id); err != nil {
		return err
	}
	_, err := DB.Exec(`DELETE FROM class_files WHERE class_id=$1 AND (id=$2 OR path LIKE $3)`, f.ClassID, id, f.Path+"/%")
	return err
}

func UpdateFileContent(id uuid.UUID, data []byte) error {
	if len(data) > maxFileSize {
		return fmt.Errorf("file too large")
	}
	_, err := DB.Exec(`UPDATE class_files SET content=$1, size=$2, updated_at=now() WHERE id=$3`, data, len(data), id)
	return err
}

func IsTeacherOfClass(cid, teacherID uuid.UUID) (bool, error) {
	var x int
	err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, cid, teacherID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func IsStudentOfClass(cid, studentID uuid.UUID) (bool, error) {
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
	ID          uuid.UUID `db:"id" json:"id"`
	SenderID    uuid.UUID `db:"sender_id" json:"sender_id"`
	RecipientID uuid.UUID `db:"recipient_id" json:"recipient_id"`
	Text        string    `db:"content" json:"text"`
	Image       *string   `db:"image" json:"image,omitempty"`
	FileName    *string   `db:"file_name" json:"file_name,omitempty"`
	File        *string   `db:"file" json:"file,omitempty"`
	IsRead      bool      `db:"is_read" json:"is_read"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

func CreateMessage(m *Message) error {
	blocked, err := IsBlocked(m.SenderID, m.RecipientID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlocked
	}
	const q = `INSERT INTO messages (sender_id, recipient_id, content, image, file_name, file)
                    VALUES ($1,$2,$3,$4,$5,$6)
                    RETURNING id, created_at, is_read`
	err = DB.QueryRow(q, m.SenderID, m.RecipientID, m.Text, m.Image, m.FileName, m.File).
		Scan(&m.ID, &m.CreatedAt, &m.IsRead)
	if err == nil {
		broadcastMsg(sse.Event{Event: "message", Data: m})
	}
	return err
}

func ListMessages(userID, otherID uuid.UUID, limit, offset int) ([]Message, error) {
	msgs := []Message{}
	err := DB.Select(&msgs, `SELECT id,sender_id,recipient_id,content,image,file_name,file,created_at,is_read
                                 FROM messages
                                WHERE (sender_id=$1 AND recipient_id=$2)
                                   OR (sender_id=$2 AND recipient_id=$1)
                                ORDER BY created_at DESC
                                LIMIT $3 OFFSET $4`,
		userID, otherID, limit, offset)
	return msgs, err
}

func MarkMessagesRead(userID, otherID uuid.UUID) error {
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
	ID     uuid.UUID `db:"id" json:"id"`
	Email  string    `db:"email" json:"email"`
	Name   *string   `db:"name" json:"name"`
	Avatar *string   `db:"avatar" json:"avatar"`
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

func BlockUser(blockerID, blockedID uuid.UUID) error {
	_, err := DB.Exec(`INSERT INTO blocked_users (blocker_id, blocked_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, blockerID, blockedID)
	return err
}

func UnblockUser(blockerID, blockedID uuid.UUID) error {
	_, err := DB.Exec(`DELETE FROM blocked_users WHERE blocker_id=$1 AND blocked_id=$2`, blockerID, blockedID)
	return err
}

func ListBlockedUsers(blockerID uuid.UUID) ([]UserSearch, error) {
	list := []UserSearch{}
	err := DB.Select(&list, `SELECT u.id, u.email, u.name, u.avatar
                                   FROM blocked_users b
                                   JOIN users u ON u.id = b.blocked_id
                                  WHERE b.blocker_id=$1
                                  ORDER BY u.email`, blockerID)
	return list, err
}

func IsBlocked(a, b uuid.UUID) (bool, error) {
	var x int
	err := DB.Get(&x, `SELECT 1 FROM blocked_users WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)`, a, b)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

type Conversation struct {
	OtherID     uuid.UUID `db:"other_id" json:"other_id"`
	Name        *string   `db:"name" json:"name"`
	Avatar      *string   `db:"avatar" json:"avatar"`
	Email       string    `db:"email" json:"email"`
	UnreadCount int       `db:"unread_count" json:"unread_count"`
	Starred     bool      `db:"starred" json:"starred"`
	Archived    bool      `db:"archived" json:"archived"`
	Message
}

func ListRecentConversations(userID uuid.UUID, limit int) ([]Conversation, error) {
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
              l.id, l.sender_id, l.recipient_id, l.content, l.image, l.file_name, l.file, l.created_at,
              COALESCE(un.unread_count,0) AS unread_count,
              (sc.other_id IS NOT NULL) AS starred,
              (ac.other_id IS NOT NULL) AS archived
        FROM latest l
        JOIN users u ON u.id = l.other_id
        LEFT JOIN unread un ON un.other_id=l.other_id
       LEFT JOIN starred_conversations sc ON sc.user_id=$1 AND sc.other_id=l.other_id
       LEFT JOIN archived_conversations ac ON ac.user_id=$1 AND ac.other_id=l.other_id
        LEFT JOIN blocked_users b ON b.blocker_id=$1 AND b.blocked_id=l.other_id
       WHERE l.rn = 1 AND b.blocked_id IS NULL
       ORDER BY (COALESCE(un.unread_count,0) > 0) DESC, l.created_at DESC
       LIMIT $2`
	err := DB.Select(&list, q, userID, limit)
	return list, err
}

func StarConversation(userID, otherID uuid.UUID) error {
	_, err := DB.Exec(`INSERT INTO starred_conversations (user_id, other_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, otherID)
	return err
}

func UnstarConversation(userID, otherID uuid.UUID) error {
	_, err := DB.Exec(`DELETE FROM starred_conversations WHERE user_id=$1 AND other_id=$2`, userID, otherID)
	return err
}

func ArchiveConversation(userID, otherID uuid.UUID) error {
	_, err := DB.Exec(`INSERT INTO archived_conversations (user_id, other_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, otherID)
	return err
}

func UnarchiveConversation(userID, otherID uuid.UUID) error {
	_, err := DB.Exec(`DELETE FROM archived_conversations WHERE user_id=$1 AND other_id=$2`, userID, otherID)
	return err
}

// ──────────────────────────────────────────────────────
// class forum messaging
// ──────────────────────────────────────────────────────

type ForumMessage struct {
	ID        uuid.UUID `db:"id" json:"id"`
	ClassID   uuid.UUID `db:"class_id" json:"class_id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	Text      string    `db:"content" json:"text"`
	Image     *string   `db:"image" json:"image,omitempty"`
	FileName  *string   `db:"file_name" json:"file_name,omitempty"`
	File      *string   `db:"file" json:"file,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	Name      *string   `db:"name" json:"name"`
	Email     string    `db:"email" json:"email"`
	Avatar    *string   `db:"avatar" json:"avatar"`
}

func CreateForumMessage(m *ForumMessage) error {
	const q = `INSERT INTO forum_messages (class_id, user_id, content, image, file_name, file)
                   VALUES ($1,$2,$3,$4,$5,$6)
                   RETURNING id, created_at`
	if err := DB.QueryRow(q, m.ClassID, m.UserID, m.Text, m.Image, m.FileName, m.File).Scan(&m.ID, &m.CreatedAt); err != nil {
		return err
	}
	_ = DB.QueryRow(`SELECT name, email, avatar FROM users WHERE id=$1`, m.UserID).Scan(&m.Name, &m.Email, &m.Avatar)
	broadcastForumMsg(m)
	return nil
}

func ListForumMessages(classID uuid.UUID, limit, offset int) ([]ForumMessage, error) {
    msgs := []ForumMessage{}
    err := DB.Select(&msgs, `SELECT fm.id, fm.class_id, fm.user_id, fm.content, fm.image, fm.file_name, fm.file, fm.created_at,
                                       u.name, u.email, u.avatar
                                  FROM forum_messages fm
                                  JOIN users u ON u.id=fm.user_id
                                 WHERE fm.class_id=$1
                               ORDER BY fm.created_at DESC
                                  LIMIT $2 OFFSET $3`,
        classID, limit, offset)
    return msgs, err
}

type UserPresence struct {
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	IsOnline  bool      `db:"is_online" json:"is_online"`
	LastSeen  time.Time `db:"last_seen" json:"last_seen"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// UserPresence functions
func MarkUserOnline(userID uuid.UUID) error {
    // Upsert presence only if the user exists to avoid FK errors when tokens are stale
    _, err := DB.Exec(`
        INSERT INTO user_presence (user_id, is_online, last_seen, updated_at)
        SELECT $1, TRUE, now(), now()
        WHERE EXISTS (SELECT 1 FROM users WHERE id = $1)
        ON CONFLICT (user_id)
        DO UPDATE SET is_online = TRUE, last_seen = now(), updated_at = now()
    `, userID)
    return err
}

func MarkUserOffline(userID uuid.UUID) error {
	_, err := DB.Exec(`
		UPDATE user_presence 
		SET is_online = FALSE, updated_at = now() 
		WHERE user_id = $1
	`, userID)
	return err
}

func UpdateUserLastSeen(userID uuid.UUID) error {
    // Touch presence only if the user exists to avoid FK errors when tokens are stale
    _, err := DB.Exec(`
        INSERT INTO user_presence (user_id, is_online, last_seen, updated_at)
        SELECT $1, TRUE, now(), now()
        WHERE EXISTS (SELECT 1 FROM users WHERE id = $1)
        ON CONFLICT (user_id)
        DO UPDATE SET last_seen = now(), updated_at = now()
    `, userID)
    return err
}

func GetOnlineUsers() ([]UserPresence, error) {
	var users []UserPresence
	err := DB.Select(&users, `
		SELECT user_id, is_online, last_seen, created_at, updated_at
		FROM user_presence 
		WHERE is_online = TRUE 
		ORDER BY last_seen DESC
	`)
	return users, err
}

func IsUserOnline(userID uuid.UUID) (bool, error) {
	var isOnline bool
	err := DB.Get(&isOnline, `
		SELECT is_online 
		FROM user_presence 
		WHERE user_id = $1
	`, userID)
	if err != nil {
		// If no record exists, user is considered offline
		return false, nil
	}
	return isOnline, nil
}

func CleanupInactiveUsers() error {
	// Mark users as offline if they haven't been seen in the last 5 minutes
	_, err := DB.Exec(`
		UPDATE user_presence 
		SET is_online = FALSE, updated_at = now() 
		WHERE last_seen < now() - INTERVAL '5 minutes' AND is_online = TRUE
	`)
	return err
}
