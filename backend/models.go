package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const maxFileSize = 20 * 1024 * 1024 // 20 MB

var ErrBlocked = errors.New("blocked")

type User struct {
	ID                 uuid.UUID  `db:"id"`
	Email              string     `db:"email"`
	PasswordHash       string     `db:"password_hash"`
	Name               *string    `db:"name"`
	Avatar             *string    `db:"avatar"`
	Role               string     `db:"role"`
	Theme              string     `db:"theme"`
	PreferredLocale    *string    `db:"preferred_locale"`
	EmailNotifications bool       `db:"email_notifications"`
	EmailMessageDigest bool       `db:"email_message_digest"`
	EmailVerified      bool       `db:"email_verified"`
	EmailVerifiedAt    *time.Time `db:"email_verified_at"`
	BkClass            *string    `db:"bk_class"`
	BkUID              *string    `db:"bk_uid"`
	MsOID              *string    `db:"ms_oid"`
	CreatedAt          time.Time  `db:"created_at"`
}

type Assignment struct {
	ID              uuid.UUID      `db:"id" json:"id"`
	Title           string         `db:"title" json:"title"`
	Description     string         `db:"description" json:"description"`
	CreatedBy       uuid.UUID      `db:"created_by" json:"created_by"`
	Deadline        time.Time      `db:"deadline" json:"deadline"`
	MaxPoints       int            `db:"max_points" json:"max_points"`
	GradingPolicy   string         `db:"grading_policy" json:"grading_policy"`
	Published       bool           `db:"published" json:"published"`
	ShowTraceback   bool           `db:"show_traceback" json:"show_traceback"`
	ShowTestDetails bool           `db:"show_test_details" json:"show_test_details"`
	ManualReview    bool           `db:"manual_review" json:"manual_review"`
	BannedFunctions pq.StringArray `db:"banned_functions" json:"banned_functions"`
	BannedModules   pq.StringArray `db:"banned_modules" json:"banned_modules"`
	BannedToolRules *string        `db:"banned_tool_rules" json:"banned_tool_rules,omitempty"`
	TemplatePath    *string        `db:"template_path" json:"template_path"`
	CreatedAt       time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at" json:"updated_at"`
	ClassID         uuid.UUID      `db:"class_id" json:"class_id"`

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

// AssignmentClone links a cloned assignment back to its source and target class.
type AssignmentClone struct {
	SourceAssignmentID uuid.UUID  `db:"source_assignment_id" json:"source_assignment_id"`
	ClonedAssignmentID uuid.UUID  `db:"cloned_assignment_id" json:"cloned_assignment_id"`
	TargetClassID      uuid.UUID  `db:"target_class_id" json:"target_class_id"`
	CreatedBy          *uuid.UUID `db:"created_by" json:"created_by,omitempty"`
	CreatedAt          time.Time  `db:"created_at" json:"created_at"`
}

func copyStringArray(arr pq.StringArray) []string {
	if len(arr) == 0 {
		return []string{}
	}
	out := make([]string, len(arr))
	copy(out, arr)
	return out
}

func cloneStrings(list []string) []string {
	if len(list) == 0 {
		return []string{}
	}
	out := make([]string, len(list))
	copy(out, list)
	return out
}

// AssignmentDeadlineOverride stores a per-student custom deadline for an assignment
type AssignmentDeadlineOverride struct {
	AssignmentID uuid.UUID `db:"assignment_id" json:"assignment_id"`
	StudentID    uuid.UUID `db:"student_id" json:"student_id"`
	NewDeadline  time.Time `db:"new_deadline" json:"new_deadline"`
	Note         *string   `db:"note" json:"note"`
	CreatedBy    uuid.UUID `db:"created_by" json:"created_by"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}
type Class struct {
	ID          uuid.UUID `db:"id"        json:"id"`
	Name        string    `db:"name"      json:"name"`
	TeacherID   uuid.UUID `db:"teacher_id" json:"teacher_id"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
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
	StudentName      *string   `db:"student_name" json:"student_name,omitempty"`
}

type TestCase struct {
	ID               uuid.UUID `db:"id" json:"id"`
	AssignmentID     uuid.UUID `db:"assignment_id" json:"assignment_id"`
	Stdin            string    `db:"stdin" json:"stdin"`
	ExpectedStdout   string    `db:"expected_stdout" json:"expected_stdout"`
	Weight           float64   `db:"weight" json:"weight"`
	TimeLimitSec     float64   `db:"time_limit_sec" json:"time_limit_sec"`
	MemoryLimitKB    int       `db:"memory_limit_kb" json:"memory_limit_kb"`
	UnittestCode     *string   `db:"unittest_code" json:"unittest_code"`
	UnittestName     *string   `db:"unittest_name" json:"unittest_name"`
	ExecutionMode    string    `db:"execution_mode" json:"execution_mode"`
	FunctionName     *string   `db:"function_name" json:"function_name,omitempty"`
	FunctionArgs     *string   `db:"function_args" json:"function_args,omitempty"`
	FunctionKwargs   *string   `db:"function_kwargs" json:"function_kwargs,omitempty"`
	FunctionArgNames *string   `db:"function_arg_names" json:"function_arg_names,omitempty"`
	ExpectedReturn   *string   `db:"expected_return" json:"expected_return,omitempty"`
	FileName         *string   `db:"file_name" json:"file_name,omitempty"`
	FileBase64       *string   `db:"file_base64" json:"file_base64,omitempty"`
	FilesJSON        *string   `db:"files_json" json:"files_json,omitempty"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

// ──────────────────────────────────────────────────────
// admin helpers
// ──────────────────────────────────────────────────────

type UserSummary struct {
	ID        uuid.UUID `db:"id"         json:"id"`
	Email     string    `db:"email"      json:"email"`
	Name      *string   `db:"name"       json:"name"`
	Role      string    `db:"role"       json:"role"`
	BkUID     *string   `db:"bk_uid"     json:"bk_uid"`
	MsOID     *string   `db:"ms_oid"     json:"ms_oid"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func ListUsers() ([]UserSummary, error) {
	list := []UserSummary{}
	err := DB.Select(&list,
		`SELECT id,email,name,role,bk_uid,ms_oid,created_at
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
	err := DB.Get(&u, `SELECT id, email, password_hash, name, avatar, role, theme, preferred_locale, email_notifications, email_message_digest, email_verified, email_verified_at, bk_class, bk_uid, created_at
                FROM users WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func UpdateUserProfile(id uuid.UUID, name, avatar, theme *string, notifications, messageDigest *bool, preferredLocale *string, preferredLocaleProvided bool) error {
	if preferredLocaleProvided {
		_, err := DB.Exec(`UPDATE users
		SET name=COALESCE($1,name),
		    avatar=COALESCE($2,avatar),
		    theme=COALESCE($3,theme),
		    email_notifications=COALESCE($4,email_notifications),
		    email_message_digest=COALESCE($5,email_message_digest),
		    preferred_locale=$6
	 WHERE id=$7`, name, avatar, theme, notifications, messageDigest, preferredLocale, id)
		return err
	}

	_, err := DB.Exec(`UPDATE users
		SET name=COALESCE($1,name),
		    avatar=COALESCE($2,avatar),
		    theme=COALESCE($3,theme),
		    email_notifications=COALESCE($4,email_notifications),
		    email_message_digest=COALESCE($5,email_message_digest)
	 WHERE id=$6`, name, avatar, theme, notifications, messageDigest, id)
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
          INSERT INTO assignments (title, description, created_by, deadline, max_points, grading_policy, published, show_traceback, show_test_details, manual_review, banned_functions, banned_modules, banned_tool_rules, template_path, class_id, second_deadline, late_penalty_ratio)
          VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
          RETURNING id, created_at, updated_at`
	return DB.QueryRow(q,
		a.Title, a.Description, a.CreatedBy, a.Deadline,
		a.MaxPoints, a.GradingPolicy, a.Published, a.ShowTraceback, a.ShowTestDetails, a.ManualReview,
		pq.Array(copyStringArray(a.BannedFunctions)), pq.Array(copyStringArray(a.BannedModules)),
		a.BannedToolRules,
		a.TemplatePath, a.ClassID,
		a.SecondDeadline, a.LatePenaltyRatio,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

// SaveAssignmentClone records a link between a source assignment and its cloned copy.
func SaveAssignmentClone(sourceID, clonedID, targetClassID, createdBy uuid.UUID) error {
	_, err := DB.Exec(`
        INSERT INTO assignment_clones (source_assignment_id, cloned_assignment_id, target_class_id, created_by)
        VALUES ($1,$2,$3,$4)
        ON CONFLICT (cloned_assignment_id) DO NOTHING`,
		sourceID, clonedID, targetClassID, createdBy)
	return err
}

// ListAssignmentClonesForSourceAndTarget returns all clones of a source within a given target class.
func ListAssignmentClonesForSourceAndTarget(sourceID, targetClassID uuid.UUID) ([]AssignmentClone, error) {
	list := []AssignmentClone{}
	err := DB.Select(&list, `
                SELECT source_assignment_id, cloned_assignment_id, target_class_id, created_by, created_at
                  FROM assignment_clones
                 WHERE source_assignment_id=$1 AND target_class_id=$2
                 ORDER BY created_at DESC`,
		sourceID, targetClassID)
	return list, err
}

// ListAssignments returns all assignments.
func ListAssignments(role string, userID uuid.UUID) ([]Assignment, error) {
	list := []Assignment{}
	// Select deadline expression varies by role to reflect per-student overrides
	deadlineExpr := "a.deadline"
	joins := ""
	var args []any
	query := `
    SELECT a.id, a.title, a.description, a.created_by, ` + deadlineExpr + ` AS deadline,
           a.max_points, a.grading_policy, a.published, a.show_traceback, a.show_test_details, a.manual_review,
           COALESCE(a.banned_functions,'{}') AS banned_functions,
           COALESCE(a.banned_modules,'{}')   AS banned_modules,
           a.banned_tool_rules,
           a.template_path,
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
	switch role {
	case "teacher":
		query += ` JOIN classes c ON c.id = a.class_id
                WHERE c.teacher_id = $1`
		args = append(args, userID)
	case "student":
		// Left join per-student override, and coalesce deadline
		joins = ` LEFT JOIN assignment_deadline_overrides ado
                     ON ado.assignment_id = a.id AND ado.student_id = $1`
		// rebuild query with override-aware deadline
		query = `
    SELECT a.id, a.title, a.description, a.created_by, COALESCE(ado.new_deadline, a.deadline) AS deadline,
           a.max_points, a.grading_policy, a.published, a.show_traceback, a.show_test_details, a.manual_review,
           COALESCE(a.banned_functions,'{}') AS banned_functions,
           COALESCE(a.banned_modules,'{}')   AS banned_modules,
           a.banned_tool_rules,
           a.template_path,
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
      FROM assignments a` + joins + ` JOIN class_students cs ON cs.class_id = a.class_id
     WHERE cs.student_id = $1 AND a.published = true`
		args = append(args, userID)
	default:
		// admin gets everything
	}
	if role != "student" {
		query += joins
	}
	query += " ORDER BY a.created_at DESC"
	err := DB.Select(&list, query, args...)
	return list, err
}

// UpsertDeadlineOverride creates or updates a per-student deadline override
func UpsertDeadlineOverride(aid, studentID uuid.UUID, newDeadline time.Time, note *string, createdBy uuid.UUID) error {
	_, err := DB.Exec(`
        INSERT INTO assignment_deadline_overrides (assignment_id, student_id, new_deadline, note, created_by)
        VALUES ($1,$2,$3,$4,$5)
        ON CONFLICT (assignment_id, student_id) DO UPDATE
           SET new_deadline = EXCLUDED.new_deadline,
               note = EXCLUDED.note,
               created_by = EXCLUDED.created_by,
               updated_at = now()`,
		aid, studentID, newDeadline, note, createdBy,
	)
	return err
}

// GetDeadlineOverride returns the override for one student if present
func GetDeadlineOverride(aid, studentID uuid.UUID) (*AssignmentDeadlineOverride, error) {
	var o AssignmentDeadlineOverride
	if err := DB.Get(&o, `SELECT assignment_id, student_id, new_deadline, note, created_by, created_at, updated_at
                            FROM assignment_deadline_overrides
                           WHERE assignment_id=$1 AND student_id=$2`, aid, studentID); err != nil {
		return nil, err
	}
	return &o, nil
}

// ListDeadlineOverridesForAssignment returns overrides with student identity for UI
type DeadlineOverrideWithStudent struct {
	AssignmentID uuid.UUID `db:"assignment_id" json:"assignment_id"`
	StudentID    uuid.UUID `db:"student_id" json:"student_id"`
	NewDeadline  time.Time `db:"new_deadline" json:"new_deadline"`
	Note         *string   `db:"note" json:"note"`
	CreatedBy    uuid.UUID `db:"created_by" json:"created_by"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
	Email        string    `db:"email" json:"email"`
	Name         *string   `db:"name" json:"name"`
}

func ListDeadlineOverridesForAssignment(aid uuid.UUID) ([]DeadlineOverrideWithStudent, error) {
	list := []DeadlineOverrideWithStudent{}
	err := DB.Select(&list, `
        SELECT ado.assignment_id, ado.student_id, ado.new_deadline, ado.note, ado.created_by, ado.created_at, ado.updated_at,
               u.email, u.name
          FROM assignment_deadline_overrides ado
          JOIN users u ON u.id = ado.student_id
         WHERE ado.assignment_id = $1
         ORDER BY ado.new_deadline ASC`, aid)
	return list, err
}

// DeleteDeadlineOverride removes a per-student override
func DeleteDeadlineOverride(aid, studentID uuid.UUID) error {
	_, err := DB.Exec(`DELETE FROM assignment_deadline_overrides WHERE assignment_id=$1 AND student_id=$2`, aid, studentID)
	return err
}

// GetAssignment looks up one assignment by ID.
func GetAssignment(id uuid.UUID) (*Assignment, error) {
	var a Assignment
	err := DB.Get(&a, `
    SELECT id, title, description, created_by, deadline, max_points, grading_policy, published, show_traceback, show_test_details, manual_review,
           COALESCE(banned_functions,'{}') AS banned_functions,
           COALESCE(banned_modules,'{}')   AS banned_modules,
           banned_tool_rules,
           template_path, created_at, updated_at, class_id,
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
               a.max_points, a.grading_policy, a.published, a.show_traceback, a.show_test_details, a.manual_review,
               COALESCE(a.banned_functions,'{}') AS banned_functions,
               COALESCE(a.banned_modules,'{}')   AS banned_modules,
               a.banned_tool_rules,
               a.template_path,
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
           max_points=$4, grading_policy=$5, show_traceback=$6, show_test_details=$7, manual_review=$8,
           banned_functions=$9, banned_modules=$10, banned_tool_rules=$11,
           llm_interactive=$12, llm_feedback=$13, llm_auto_award=$14, llm_scenarios_json=$15,
           llm_strictness=$16, llm_rubric=$17, llm_teacher_baseline_json=$18,
           second_deadline=$19, late_penalty_ratio=$20,
           updated_at=now()
     WHERE id=$21`,
		a.Title, a.Description, a.Deadline,
		a.MaxPoints, a.GradingPolicy, a.ShowTraceback, a.ShowTestDetails, a.ManualReview,
		pq.Array(copyStringArray(a.BannedFunctions)), pq.Array(copyStringArray(a.BannedModules)), a.BannedToolRules,
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

func UpdateAssignmentConstraints(id uuid.UUID, bannedFunctions, bannedModules []string, configJSON *string) error {
	_, err := DB.Exec(`
        UPDATE assignments
           SET banned_functions=$1,
               banned_modules=$2,
               banned_tool_rules=$3,
               updated_at=now()
         WHERE id=$4`,
		pq.Array(cloneStrings(bannedFunctions)),
		pq.Array(cloneStrings(bannedModules)),
		configJSON,
		id,
	)
	return err
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
		ShowTestDetails:  src.ShowTestDetails,
		ManualReview:     src.ManualReview,
		BannedFunctions:  append(pq.StringArray(nil), src.BannedFunctions...),
		BannedModules:    append(pq.StringArray(nil), src.BannedModules...),
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
	if src.BannedToolRules != nil {
		clone := *src.BannedToolRules
		dst.BannedToolRules = &clone
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
			AssignmentID:     dst.ID,
			Stdin:            t.Stdin,
			ExpectedStdout:   t.ExpectedStdout,
			Weight:           t.Weight,
			TimeLimitSec:     t.TimeLimitSec,
			MemoryLimitKB:    t.MemoryLimitKB,
			UnittestCode:     t.UnittestCode,
			UnittestName:     t.UnittestName,
			ExecutionMode:    t.ExecutionMode,
			FunctionName:     t.FunctionName,
			FunctionArgs:     t.FunctionArgs,
			FunctionKwargs:   t.FunctionKwargs,
			FunctionArgNames: t.FunctionArgNames,
			ExpectedReturn:   t.ExpectedReturn,
			FileName:         t.FileName,
			FileBase64:       t.FileBase64,
			FilesJSON:        t.FilesJSON,
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
        INSERT INTO users (email, password_hash, name, role, email_verified, bk_uid)
        VALUES ($1,$2,$3,'teacher',TRUE,$4)`, email, hash, name, bkUID)
	return err
}

// FindUserByBkUID returns a user identified by the Bakaláři UID.
func FindUserByBkUID(uid string) (*User, error) {
	var u User
	err := DB.Get(&u, `SELECT id, email, password_hash, name, role, bk_class, bk_uid, avatar, theme, email_verified, email_verified_at, created_at
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
                INSERT INTO users (email, password_hash, name, role, email_verified, bk_class, bk_uid)
                VALUES ($1,$2,$3,'student',TRUE,$4,$5)
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

func mergeUsersTx(tx *sqlx.Tx, targetID, sourceID uuid.UUID) error {
	if targetID == sourceID {
		return nil
	}

	if _, err := tx.Exec(`UPDATE classes SET teacher_id=$1 WHERE teacher_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE assignments SET created_by=$1 WHERE created_by=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM class_students cs
		WHERE cs.student_id=$2
		  AND EXISTS (
			SELECT 1 FROM class_students cs2
			WHERE cs2.class_id = cs.class_id AND cs2.student_id = $1
		)`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE class_students SET student_id=$1 WHERE student_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM assignment_deadline_overrides ado
		WHERE ado.student_id=$2
		  AND EXISTS (
			SELECT 1 FROM assignment_deadline_overrides ado2
			WHERE ado2.assignment_id = ado.assignment_id AND ado2.student_id = $1
		)`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE assignment_deadline_overrides SET student_id=$1 WHERE student_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE assignment_deadline_overrides SET created_by=$1 WHERE created_by=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE submissions SET student_id=$1 WHERE student_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE messages SET sender_id=$1 WHERE sender_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE messages SET recipient_id=$1 WHERE recipient_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM blocked_users WHERE blocker_id=$2 AND blocked_id=$1`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM blocked_users WHERE blocker_id=$1 AND blocked_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM blocked_users bu
		WHERE bu.blocker_id=$2
		  AND EXISTS (
			SELECT 1 FROM blocked_users bu2
			WHERE bu2.blocker_id=$1 AND bu2.blocked_id=bu.blocked_id
		)`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE blocked_users SET blocker_id=$1 WHERE blocker_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM blocked_users bu
		WHERE bu.blocked_id=$2
		  AND EXISTS (
			SELECT 1 FROM blocked_users bu2
			WHERE bu2.blocker_id=bu.blocker_id AND bu2.blocked_id=$1
		)`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE blocked_users SET blocked_id=$1 WHERE blocked_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM starred_conversations WHERE user_id=$2 AND other_id=$1`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM starred_conversations WHERE user_id=$1 AND other_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM starred_conversations sc
		WHERE sc.user_id=$2
		  AND EXISTS (
			SELECT 1 FROM starred_conversations sc2
			WHERE sc2.user_id=$1 AND sc2.other_id=sc.other_id
		)`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE starred_conversations SET user_id=$1 WHERE user_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM starred_conversations sc
		WHERE sc.other_id=$2
		  AND EXISTS (
			SELECT 1 FROM starred_conversations sc2
			WHERE sc2.user_id=sc.user_id AND sc2.other_id=$1
		)`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE starred_conversations SET other_id=$1 WHERE other_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM archived_conversations WHERE user_id=$2 AND other_id=$1`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM archived_conversations WHERE user_id=$1 AND other_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM archived_conversations ac
		WHERE ac.user_id=$2
		  AND EXISTS (
			SELECT 1 FROM archived_conversations ac2
			WHERE ac2.user_id=$1 AND ac2.other_id=ac.other_id
		)`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE archived_conversations SET user_id=$1 WHERE user_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM archived_conversations ac
		WHERE ac.other_id=$2
		  AND EXISTS (
			SELECT 1 FROM archived_conversations ac2
			WHERE ac2.user_id=ac.user_id AND ac2.other_id=$1
		)`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE archived_conversations SET other_id=$1 WHERE other_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE forum_messages SET user_id=$1 WHERE user_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM notification_log nl
		WHERE nl.user_id=$2
		  AND EXISTS (
			SELECT 1 FROM notification_log nl2
			WHERE nl2.user_id=$1 AND nl2.notification_type=nl.notification_type AND nl2.context=nl.context
		)`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE notification_log SET user_id=$1 WHERE user_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO user_presence (user_id, is_online, last_seen, created_at, updated_at)
		SELECT $1, src.is_online, src.last_seen, src.created_at, src.updated_at
		FROM user_presence src
		WHERE src.user_id=$2
		ON CONFLICT (user_id) DO UPDATE
			SET is_online = user_presence.is_online OR EXCLUDED.is_online,
			    last_seen = GREATEST(user_presence.last_seen, EXCLUDED.last_seen),
			    updated_at = now()
	`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_presence WHERE user_id=$1`, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE password_reset_tokens SET user_id=$1 WHERE user_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE email_verification_tokens SET user_id=$1 WHERE user_id=$2`, targetID, sourceID); err != nil {
		return err
	}
	return nil
}

func CreateClass(c *Class) error {
	return DB.QueryRow(`
        INSERT INTO classes (name, teacher_id, description)
        VALUES ($1,$2,$3)
        RETURNING id, created_at, updated_at`,
		c.Name, c.TeacherID, c.Description,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func UpdateClassName(id uuid.UUID, teacherID uuid.UUID, name string) error {
	trimmed := strings.TrimSpace(name)
	return UpdateClassMeta(id, teacherID, &trimmed, nil)
}

func UpdateClassMeta(id uuid.UUID, teacherID uuid.UUID, name, description *string) error {
	if teacherID != uuid.Nil {
		var x int
		if err := DB.Get(&x, `SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2`, id, teacherID); err != nil {
			return err
		}
	}

	var (
		setClauses []string
		args       []any
	)

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return fmt.Errorf("name cannot be empty")
		}
		nameArg := trimmed
		setClauses = append(setClauses, fmt.Sprintf("name=$%d", len(args)+1))
		args = append(args, nameArg)
	}

	if description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description=$%d", len(args)+1))
		args = append(args, *description)
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at=now()"))

	args = append(args, id)
	query := fmt.Sprintf(`UPDATE classes SET %s WHERE id=$%d`,
		strings.Join(setClauses, ", "),
		len(args),
	)

	res, err := DB.Exec(query, args...)
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
            SELECT id, email, name, avatar FROM users
             WHERE role = 'student'
             ORDER BY email`)
	return list, err
}

// ──────────────────────────────────────────────────────────────────────────────
// classes – helpers for detail view
// ──────────────────────────────────────────────────────────────────────────────
type Student struct {
	ID     uuid.UUID `db:"id"     json:"id"`
	Email  string    `db:"email"  json:"email"`
	Name   *string   `db:"name"   json:"name"`
	Avatar *string   `db:"avatar" json:"avatar"`
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
		`SELECT id, email, name, avatar FROM users WHERE id = $1`,
		cls.TeacherID); err != nil {
		return nil, err
	}

	// 3) Students (many) -------------------------------------------------------
	var students []Student
	if err := DB.Select(&students, `
               SELECT u.id, u.email, u.name, u.avatar
                 FROM users u
                 JOIN class_students cs ON cs.student_id = u.id
                WHERE cs.class_id = $1
                ORDER BY u.email`,
		id); err != nil {
		return nil, err
	}

	// 4) Assignments (many) ----------------------------------------------------
	var asg []Assignment
	if role == "student" {
		// For students, reflect personal overrides in the returned deadline
		query := `
                SELECT a.id, a.title, a.description, a.created_by, COALESCE(ado.new_deadline, a.deadline) AS deadline,
                       a.max_points, a.grading_policy, a.published, a.template_path,
                       a.created_at, a.updated_at, a.class_id,
                       COALESCE(a.llm_interactive,false) AS llm_interactive,
                       COALESCE(a.llm_feedback,false) AS llm_feedback,
                       COALESCE(a.llm_auto_award,true) AS llm_auto_award,
                       a.llm_scenarios_json
                  FROM assignments a
             LEFT JOIN assignment_deadline_overrides ado ON ado.assignment_id=a.id AND ado.student_id=$2
                 WHERE a.class_id = $1 AND a.published = true
              ORDER BY deadline ASC`
		if err := DB.Select(&asg, query, id, userID); err != nil {
			return nil, err
		}
	} else {
		query := `
                SELECT id, title, description, created_by, deadline,
                       max_points, grading_policy, published, template_path,
                       created_at, updated_at, class_id,
                       COALESCE(llm_interactive,false) AS llm_interactive,
                       COALESCE(llm_feedback,false) AS llm_feedback,
                       COALESCE(llm_auto_award,true) AS llm_auto_award,
                       llm_scenarios_json
                  FROM assignments
                 WHERE class_id = $1
              ORDER BY deadline ASC`
		if err := DB.Select(&asg, query, id); err != nil {
			return nil, err
		}
	}
	// end assignments selection

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
	if tc.MemoryLimitKB == 0 {
		tc.MemoryLimitKB = 65536
	}
	if strings.TrimSpace(tc.ExecutionMode) == "" {
		if tc.UnittestName != nil {
			tc.ExecutionMode = "unittest"
		} else if tc.FunctionName != nil {
			tc.ExecutionMode = "function"
		} else {
			tc.ExecutionMode = "stdin_stdout"
		}
	}
	const q = `
         INSERT INTO test_cases (assignment_id, stdin, expected_stdout, weight, time_limit_sec, memory_limit_kb, unittest_code, unittest_name,
                                 execution_mode, function_name, function_args, function_kwargs, function_arg_names, expected_return, file_name, file_base64, files_json)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
         RETURNING id, weight, time_limit_sec, memory_limit_kb, unittest_code, unittest_name,
                   execution_mode, function_name, function_args, function_kwargs, function_arg_names, expected_return, file_name, file_base64, files_json, created_at, updated_at`
	return DB.QueryRow(q, tc.AssignmentID, tc.Stdin, tc.ExpectedStdout, tc.Weight, tc.TimeLimitSec, tc.MemoryLimitKB, tc.UnittestCode, tc.UnittestName,
		tc.ExecutionMode, tc.FunctionName, tc.FunctionArgs, tc.FunctionKwargs, tc.FunctionArgNames, tc.ExpectedReturn, tc.FileName, tc.FileBase64, tc.FilesJSON).
		Scan(&tc.ID, &tc.Weight, &tc.TimeLimitSec, &tc.MemoryLimitKB, &tc.UnittestCode, &tc.UnittestName,
			&tc.ExecutionMode, &tc.FunctionName, &tc.FunctionArgs, &tc.FunctionKwargs, &tc.FunctionArgNames, &tc.ExpectedReturn, &tc.FileName, &tc.FileBase64, &tc.FilesJSON, &tc.CreatedAt, &tc.UpdatedAt)
}

// UpdateTestCase modifies stdin/stdout/time limit of an existing test case.
func UpdateTestCase(tc *TestCase) error {
	if tc.TimeLimitSec == 0 {
		tc.TimeLimitSec = 1
	}
	if strings.TrimSpace(tc.ExecutionMode) == "" {
		if tc.UnittestName != nil {
			tc.ExecutionMode = "unittest"
		} else if tc.FunctionName != nil {
			tc.ExecutionMode = "function"
		} else {
			tc.ExecutionMode = "stdin_stdout"
		}
	}
	res, err := DB.Exec(`
                UPDATE test_cases
                   SET stdin=$1, expected_stdout=$2, weight=$3, time_limit_sec=$4,
                       unittest_code=$5, unittest_name=$6, execution_mode=$7,
                       function_name=$8, function_args=$9, function_kwargs=$10, function_arg_names=$11, expected_return=$12,
                       file_name=$13, file_base64=$14, files_json=$15,
                       updated_at=now()
                 WHERE id=$16`,
		tc.Stdin, tc.ExpectedStdout, tc.Weight, tc.TimeLimitSec, tc.UnittestCode, tc.UnittestName, tc.ExecutionMode,
		tc.FunctionName, tc.FunctionArgs, tc.FunctionKwargs, tc.FunctionArgNames, tc.ExpectedReturn, tc.FileName, tc.FileBase64, tc.FilesJSON, tc.ID)
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
               SELECT id, assignment_id, stdin, expected_stdout, weight, time_limit_sec, memory_limit_kb,
                      unittest_code, unittest_name, execution_mode, function_name, function_args, function_kwargs,
                      function_arg_names, expected_return, file_name, file_base64, files_json, created_at, updated_at
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

type testFingerprint struct {
	ExecutionMode    string  `json:"execution_mode"`
	Stdin            string  `json:"stdin"`
	ExpectedOut      string  `json:"expected_stdout"`
	Weight           float64 `json:"weight"`
	TimeLimit        float64 `json:"time_limit_sec"`
	MemoryKB         int     `json:"memory_limit_kb"`
	UnittestCode     *string `json:"unittest_code,omitempty"`
	UnittestName     *string `json:"unittest_name,omitempty"`
	FunctionName     *string `json:"function_name,omitempty"`
	FunctionArgs     *string `json:"function_args,omitempty"`
	FunctionKw       *string `json:"function_kwargs,omitempty"`
	FunctionArgNames *string `json:"function_arg_names,omitempty"`
	ExpectedRet      *string `json:"expected_return,omitempty"`
	FileName         *string `json:"file_name,omitempty"`
	FileBase64       *string `json:"file_base64,omitempty"`
	FilesJSON        *string `json:"files_json,omitempty"`
}

func fingerprintTests(list []TestCase) ([]string, error) {
	fps := make([]string, 0, len(list))
	for _, t := range list {
		fp := testFingerprint{
			ExecutionMode:    t.ExecutionMode,
			Stdin:            t.Stdin,
			ExpectedOut:      t.ExpectedStdout,
			Weight:           t.Weight,
			TimeLimit:        t.TimeLimitSec,
			MemoryKB:         t.MemoryLimitKB,
			UnittestCode:     t.UnittestCode,
			UnittestName:     t.UnittestName,
			FunctionName:     t.FunctionName,
			FunctionArgs:     t.FunctionArgs,
			FunctionKw:       t.FunctionKwargs,
			FunctionArgNames: t.FunctionArgNames,
			ExpectedRet:      t.ExpectedReturn,
			FileName:         t.FileName,
			FileBase64:       t.FileBase64,
			FilesJSON:        t.FilesJSON,
		}
		js, err := json.Marshal(fp)
		if err != nil {
			return nil, err
		}
		fps = append(fps, string(js))
	}
	sort.Strings(fps)
	return fps, nil
}

// teacherGroupCloneDiffers reports whether a Teachers' group clone is out of sync
// with the source assignment for the fields we care about (excluding deadlines).
func teacherGroupCloneDiffers(src, clone *Assignment, srcTests, cloneTests []TestCase) bool {
	if src.Title != clone.Title ||
		src.Description != clone.Description ||
		src.GradingPolicy != clone.GradingPolicy ||
		src.MaxPoints != clone.MaxPoints ||
		src.ShowTraceback != clone.ShowTraceback ||
		src.ShowTestDetails != clone.ShowTestDetails ||
		src.ManualReview != clone.ManualReview ||
		src.LLMInteractive != clone.LLMInteractive ||
		src.LLMFeedback != clone.LLMFeedback ||
		src.LLMAutoAward != clone.LLMAutoAward {
		return true
	}
	if (src.LLMScenariosRaw == nil) != (clone.LLMScenariosRaw == nil) {
		return true
	}
	if src.LLMScenariosRaw != nil && clone.LLMScenariosRaw != nil && *src.LLMScenariosRaw != *clone.LLMScenariosRaw {
		return true
	}
	if src.LLMStrictness != clone.LLMStrictness {
		return true
	}
	if (src.LLMRubric == nil) != (clone.LLMRubric == nil) {
		return true
	}
	if src.LLMRubric != nil && clone.LLMRubric != nil && *src.LLMRubric != *clone.LLMRubric {
		return true
	}
	if (src.LLMTeacherBaseline == nil) != (clone.LLMTeacherBaseline == nil) {
		return true
	}
	if src.LLMTeacherBaseline != nil && clone.LLMTeacherBaseline != nil && *src.LLMTeacherBaseline != *clone.LLMTeacherBaseline {
		return true
	}

	if len(srcTests) != len(cloneTests) {
		return true
	}
	srcFP, err := fingerprintTests(srcTests)
	if err != nil {
		return true
	}
	cloneFP, err := fingerprintTests(cloneTests)
	if err != nil {
		return true
	}
	for i := range srcFP {
		if srcFP[i] != cloneFP[i] {
			return true
		}
	}
	return false
}

// NeedsTeacherGroupSync reports whether any Teachers' group clone of the source assignment
// is out of sync. It returns the clones to avoid re-querying.
func NeedsTeacherGroupSync(sourceID uuid.UUID) (bool, []AssignmentClone, error) {
	clones, err := ListAssignmentClonesForSourceAndTarget(sourceID, TeacherGroupID)
	if err != nil || len(clones) == 0 {
		return false, clones, err
	}
	src, err := GetAssignment(sourceID)
	if err != nil {
		return false, clones, err
	}
	srcTests, _ := ListTestCases(sourceID)
	for _, cl := range clones {
		dst, err := GetAssignment(cl.ClonedAssignmentID)
		if err != nil {
			continue
		}
		dstTests, _ := ListTestCases(cl.ClonedAssignmentID)
		if teacherGroupCloneDiffers(src, dst, srcTests, dstTests) {
			return true, clones, nil
		}
	}
	return false, clones, nil
}

// ──────────────────────────────────────────────────────
// submissions – helpers for grading
// ──────────────────────────────────────────────────────

// Result represents outcome of one test case execution.
type Result struct {
	ID             uuid.UUID `db:"id" json:"id"`
	SubmissionID   uuid.UUID `db:"submission_id" json:"submission_id"`
	TestCaseID     uuid.UUID `db:"test_case_id" json:"test_case_id"`
	Status         string    `db:"status" json:"status"`
	ActualStdout   string    `db:"actual_stdout" json:"actual_stdout"`
	Stderr         string    `db:"stderr" json:"stderr"`
	ExitCode       int       `db:"exit_code" json:"exit_code"`
	RuntimeMS      int       `db:"runtime_ms" json:"runtime_ms"`
	Stdin          *string   `db:"stdin" json:"stdin,omitempty"`
	ExpectedStdout *string   `db:"expected_stdout" json:"expected_stdout,omitempty"`
	UnittestCode   *string   `db:"unittest_code" json:"unittest_code,omitempty"`
	UnittestName   *string   `db:"unittest_name" json:"unittest_name,omitempty"`
	ExecutionMode  *string   `db:"execution_mode" json:"execution_mode,omitempty"`
	FunctionName   *string   `db:"function_name" json:"function_name,omitempty"`
	FunctionArgs   *string   `db:"function_args" json:"function_args,omitempty"`
	FunctionKwargs *string   `db:"function_kwargs" json:"function_kwargs,omitempty"`
	ExpectedReturn *string   `db:"expected_return" json:"expected_return,omitempty"`
	ActualReturn   *string   `db:"actual_return" json:"actual_return,omitempty"`
	TestNumber     *int      `db:"test_number" json:"test_number,omitempty"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
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
               attempt_number, student_name
          FROM (
            SELECT s.id, s.assignment_id, s.student_id, s.code_path, s.code_content, s.status, s.points, s.override_points, s.is_teacher_run, s.manually_accepted, s.late, s.created_at, s.updated_at,
                   ROW_NUMBER() OVER (PARTITION BY s.assignment_id, s.student_id ORDER BY s.created_at ASC, s.id ASC) AS attempt_number,
                   u.name as student_name
              FROM submissions s
              LEFT JOIN users u ON u.id = s.student_id
          ) sub
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
        INSERT INTO results (submission_id, test_case_id, status, actual_stdout, stderr, exit_code, runtime_ms, actual_return)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        RETURNING id, created_at`
	err := DB.QueryRow(q, r.SubmissionID, r.TestCaseID, r.Status, r.ActualStdout, r.Stderr, r.ExitCode, r.RuntimeMS, r.ActualReturn).
		Scan(&r.ID, &r.CreatedAt)
	if err == nil {
		if num, nerr := lookupTestNumber(r.TestCaseID); nerr == nil {
			r.TestNumber = num
		}
		broadcast(sse.Event{Event: "result", Data: r})
	}
	return err
}

func lookupTestNumber(tcID uuid.UUID) (*int, error) {
	var num sql.NullInt64
	err := DB.Get(&num, `
        WITH target AS (
            SELECT assignment_id FROM test_cases WHERE id = $1
        ),
        ordered AS (
            SELECT id, ROW_NUMBER() OVER (ORDER BY id) AS test_number
              FROM test_cases
             WHERE assignment_id = (SELECT assignment_id FROM target)
        )
        SELECT test_number FROM ordered WHERE id = $1
    `, tcID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !num.Valid {
		return nil, nil
	}
	v := int(num.Int64)
	return &v, nil
}

func ListResultsForSubmission(subID uuid.UUID) ([]Result, error) {
	list := []Result{}
	err := DB.Select(&list, `
        WITH sub AS (
            SELECT assignment_id FROM submissions WHERE id = $1
        ),
        ordered_tests AS (
            SELECT id,
                   ROW_NUMBER() OVER (ORDER BY id) AS test_number,
                   stdin,
                   expected_stdout,
                   unittest_code,
                   unittest_name,
                   execution_mode,
                   function_name,
                   function_args,
                   function_kwargs,
                   expected_return
              FROM test_cases
             WHERE assignment_id = (SELECT assignment_id FROM sub)
        )
        SELECT r.id, r.submission_id, r.test_case_id, r.status, r.actual_stdout, r.stderr,
               r.exit_code, r.runtime_ms, r.created_at,
               ot.stdin, ot.expected_stdout, ot.unittest_code, ot.unittest_name,
               ot.execution_mode, ot.function_name, ot.function_args, ot.function_kwargs, ot.expected_return,
               r.actual_return,
               ot.test_number
          FROM results r
          LEFT JOIN ordered_tests ot ON r.test_case_id = ot.id
         WHERE r.submission_id=$1
         ORDER BY COALESCE(ot.test_number, 2147483647), r.id`, subID)
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
                SELECT u.id, u.email, u.name, u.avatar
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
	ID           uuid.UUID  `db:"id" json:"id"`
	ClassID      uuid.UUID  `db:"class_id" json:"class_id"`
	ParentID     *uuid.UUID `db:"parent_id" json:"parent_id"`
	Name         string     `db:"name" json:"name"`
	Path         string     `db:"path" json:"path"`
	IsDir        bool       `db:"is_dir" json:"is_dir"`
	AssignmentID *uuid.UUID `db:"assignment_id" json:"assignment_id,omitempty"`
	Size         int        `db:"size" json:"size"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
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
	Structured  bool      `db:"structured" json:"structured"`
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
	const q = `INSERT INTO messages (sender_id, recipient_id, content, image, file_name, file, structured)
                    VALUES ($1,$2,$3,$4,$5,$6,$7)
                    RETURNING id, created_at, is_read`
	err = DB.QueryRow(q, m.SenderID, m.RecipientID, m.Text, m.Image, m.FileName, m.File, m.Structured).
		Scan(&m.ID, &m.CreatedAt, &m.IsRead)
	if err == nil {
		broadcastMsg(sse.Event{Event: "message", Data: m})
	}
	return err
}

func ListMessages(userID, otherID uuid.UUID, limit, offset int) ([]Message, error) {
	msgs := []Message{}
	err := DB.Select(&msgs, `SELECT id,sender_id,recipient_id,content,image,file_name,file,created_at,is_read,structured
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
              l.id, l.sender_id, l.recipient_id, l.content, l.image, l.file_name, l.file, l.created_at, l.structured,
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
	ID         uuid.UUID `db:"id" json:"id"`
	ClassID    uuid.UUID `db:"class_id" json:"class_id"`
	UserID     uuid.UUID `db:"user_id" json:"user_id"`
	Text       string    `db:"content" json:"text"`
	Image      *string   `db:"image" json:"image,omitempty"`
	FileName   *string   `db:"file_name" json:"file_name,omitempty"`
	File       *string   `db:"file" json:"file,omitempty"`
	Structured bool      `db:"structured" json:"structured"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	Name       *string   `db:"name" json:"name"`
	Email      string    `db:"email" json:"email"`
	Avatar     *string   `db:"avatar" json:"avatar"`
}

func CreateForumMessage(m *ForumMessage) error {
	const q = `INSERT INTO forum_messages (class_id, user_id, content, image, file_name, file, structured)
                   VALUES ($1,$2,$3,$4,$5,$6,$7)
                   RETURNING id, created_at`
	if err := DB.QueryRow(q, m.ClassID, m.UserID, m.Text, m.Image, m.FileName, m.File, m.Structured).Scan(&m.ID, &m.CreatedAt); err != nil {
		return err
	}
	_ = DB.QueryRow(`SELECT name, email, avatar FROM users WHERE id=$1`, m.UserID).Scan(&m.Name, &m.Email, &m.Avatar)
	broadcastForumMsg(m)
	return nil
}

func ListForumMessages(classID uuid.UUID, limit, offset int) ([]ForumMessage, error) {
	msgs := []ForumMessage{}
	err := DB.Select(&msgs, `SELECT fm.id, fm.class_id, fm.user_id, fm.content, fm.image, fm.file_name, fm.file, fm.created_at, fm.structured,
                                       u.name, u.email, u.avatar
                                  FROM forum_messages fm
                                  JOIN users u ON u.id=fm.user_id
                                 WHERE fm.class_id=$1
                               ORDER BY fm.created_at DESC
                                  LIMIT $2 OFFSET $3`,
		classID, limit, offset)
	return msgs, err
}

func DeleteForumMessage(classID, messageID uuid.UUID) error {
	res, err := DB.Exec(`DELETE FROM forum_messages WHERE id=$1 AND class_id=$2`, messageID, classID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	broadcastForumDelete(classID, messageID)
	return nil
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
func FindUserByMsOID(oid string) (*User, error) {
	var u User
	err := DB.Get(&u, `SELECT id, email, password_hash, name, role, email_verified, email_verified_at, bk_class, bk_uid, ms_oid, avatar, theme, preferred_locale, email_notifications, email_message_digest, created_at
                            FROM users WHERE ms_oid=$1`, oid)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func LinkMsOID(userID uuid.UUID, oid string) error {
	_, err := DB.Exec(`UPDATE users SET ms_oid=$1 WHERE id=$2`, oid, userID)
	return err
}

func CreateUserFromOIDC(u *User) error {
	_, err := DB.Exec(`INSERT INTO users (email, password_hash, name, role, email_verified, ms_oid) VALUES ($1, $2, $3, $4, $5, $6)`, u.Email, u.PasswordHash, u.Name, u.Role, u.EmailVerified, u.MsOID)
	return err
}

type TeacherWhitelist struct {
	Email     string    `db:"email" json:"email"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func IsEmailWhitelisted(email string) (bool, error) {
	var count int
	// specific casing in DB shouldn't matter if we lowercase both sides or if we enforce lowercase on insert
	// For robustness: compare LOWER(db) with lower(input)
	err := DB.Get(&count, "SELECT count(*) FROM teacher_whitelist WHERE LOWER(email)=$1", strings.ToLower(email))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func AddTeacherWhitelist(email string) error {
	// Always store as lowercase to keep it clean
	_, err := DB.Exec("INSERT INTO teacher_whitelist (email) VALUES ($1) ON CONFLICT (email) DO NOTHING", strings.ToLower(email))
	return err
}

func RemoveTeacherWhitelist(email string) error {
	_, err := DB.Exec("DELETE FROM teacher_whitelist WHERE LOWER(email)=$1", strings.ToLower(email))
	return err
}

func ListTeacherWhitelist() ([]TeacherWhitelist, error) {
	var list []TeacherWhitelist
	err := DB.Select(&list, "SELECT * FROM teacher_whitelist ORDER BY created_at DESC")
	return list, err
}
