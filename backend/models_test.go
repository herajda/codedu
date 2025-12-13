package main

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func TestListClassesForTeacher(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	DB = sqlx.NewDb(db, "sqlmock")

	now := time.Now()
	teacherID := uuid.New()
	classID := uuid.New()
	rows := sqlmock.NewRows([]string{"id", "name", "teacher_id", "created_at", "updated_at"}).
		AddRow(classID.String(), "Class A", teacherID.String(), now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM classes WHERE teacher_id = $1 ORDER BY created_at DESC`)).
		WithArgs(teacherID).WillReturnRows(rows)

	cls, err := ListClassesForTeacher(teacherID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cls) != 1 {
		t.Fatalf("expected 1 class, got %d", len(cls))
	}
	if cls[0].TeacherID != teacherID {
		t.Fatalf("wrong teacher id: %s", cls[0].TeacherID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListAssignmentsForStudent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	DB = sqlx.NewDb(db, "sqlmock")

	now := time.Now()
	studentID := uuid.New()
	creatorID := uuid.New()
	assignmentID := uuid.New()
	classID := uuid.New()
	rows := sqlmock.NewRows([]string{"id", "title", "description", "created_by", "deadline", "max_points", "grading_policy", "published", "show_traceback", "show_test_details", "template_path", "created_at", "updated_at", "class_id"}).
		AddRow(assignmentID.String(), "A", "desc", creatorID.String(), now, 100, "all_or_nothing", true, false, false, nil, now, now, classID.String())

		// Relaxed regex: accept any selected columns as our query may include additional fields
	q := `(?s)SELECT.+FROM assignments a.+JOIN class_students cs ON cs.class_id = a.class_id\s+WHERE cs.student_id = \$1 AND a.published = true ORDER BY a\.created_at DESC`
	mock.ExpectQuery(q).WithArgs(studentID).WillReturnRows(rows)

	list, err := ListAssignments("student", studentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(list))
	}
	if list[0].ID != assignmentID {
		t.Fatalf("wrong assignment id: %s", list[0].ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListAssignmentsForTeacher(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	DB = sqlx.NewDb(db, "sqlmock")

	now := time.Now()
	teacherID := uuid.New()
	creatorID := uuid.New()
	assignmentID := uuid.New()
	classID := uuid.New()
	rows := sqlmock.NewRows([]string{"id", "title", "description", "created_by", "deadline", "max_points", "grading_policy", "published", "show_traceback", "show_test_details", "template_path", "created_at", "updated_at", "class_id"}).
		AddRow(assignmentID.String(), "B", "desc", creatorID.String(), now, 100, "all_or_nothing", false, false, false, nil, now, now, classID.String())

	q := `SELECT\s+.*\s+FROM assignments a JOIN classes c ON c.id = a.class_id\s+WHERE c.teacher_id = \$1 ORDER BY a.created_at DESC`
	mock.ExpectQuery(q).WithArgs(teacherID).WillReturnRows(rows)

	list, err := ListAssignments("teacher", teacherID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(list))
	}
	if list[0].ID != assignmentID {
		t.Fatalf("wrong assignment id: %s", list[0].ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateTestCaseDefaultsFunctionMode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	DB = sqlx.NewDb(db, "sqlmock")

	assignmentID := uuid.New()
	fn := "multiply"
	args := "[2, 3]"
	kwargs := `{"scale": 1}`
	expected := "6"
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "weight", "time_limit_sec", "memory_limit_kb", "unittest_code", "unittest_name", "execution_mode", "function_name", "function_args", "function_kwargs", "expected_return", "created_at", "updated_at"}).
		AddRow(uuid.New().String(), 1.0, 1.0, 0, nil, nil, "function", fn, args, kwargs, expected, now, now)

	insertRE := regexp.QuoteMeta(`
         INSERT INTO test_cases (assignment_id, stdin, expected_stdout, weight, time_limit_sec, memory_limit_kb, unittest_code, unittest_name,
                                 execution_mode, function_name, function_args, function_kwargs, expected_return)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
         RETURNING id, weight, time_limit_sec, memory_limit_kb, unittest_code, unittest_name,
                   execution_mode, function_name, function_args, function_kwargs, expected_return, created_at, updated_at`)

	mock.ExpectQuery(insertRE).
		WithArgs(assignmentID, "", "", 1.0, 1.0, 65536, nil, nil, "function", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	tc := &TestCase{AssignmentID: assignmentID, Weight: 1}
	tc.FunctionName = &fn
	tc.FunctionArgs = &args
	tc.FunctionKwargs = &kwargs
	tc.ExpectedReturn = &expected

	if err := CreateTestCase(tc); err != nil {
		t.Fatalf("CreateTestCase returned error: %v", err)
	}
	if tc.ExecutionMode != "function" {
		t.Fatalf("expected execution_mode 'function', got %q", tc.ExecutionMode)
	}
	if tc.TimeLimitSec != 1 {
		t.Fatalf("expected default time limit of 1 second, got %v", tc.TimeLimitSec)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
