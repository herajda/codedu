package main

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
	rows := sqlmock.NewRows([]string{"id", "name", "teacher_id", "created_at", "updated_at"}).
		AddRow(1, "Class A", 7, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM classes WHERE teacher_id = $1 ORDER BY created_at DESC`)).
		WithArgs(7).WillReturnRows(rows)

	cls, err := ListClassesForTeacher(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cls) != 1 {
		t.Fatalf("expected 1 class, got %d", len(cls))
	}
	if cls[0].TeacherID != 7 {
		t.Fatalf("wrong teacher id: %d", cls[0].TeacherID)
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
	rows := sqlmock.NewRows([]string{"id", "title", "description", "created_by", "deadline", "max_points", "grading_policy", "published", "template_path", "created_at", "updated_at", "class_id"}).
		AddRow(1, "A", "desc", 5, now, 100, "all_or_nothing", true, nil, now, now, 3)

	q := `SELECT a.id, a.title, a.description, a.created_by, a.deadline,
                       a.max_points, a.grading_policy, a.published, a.template_path,
                       a.created_at, a.updated_at, a.class_id
                  FROM assignments a JOIN class_students cs ON cs.class_id = a.class_id
                           WHERE cs.student_id = $1 AND a.published = true ORDER BY a.created_at DESC`
	mock.ExpectQuery(regexp.QuoteMeta(q)).WithArgs(9).WillReturnRows(rows)

	list, err := ListAssignments("student", 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(list))
	}
	if list[0].ID != 1 {
		t.Fatalf("wrong assignment id: %d", list[0].ID)
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
	rows := sqlmock.NewRows([]string{"id", "title", "description", "created_by", "deadline", "max_points", "grading_policy", "published", "template_path", "created_at", "updated_at", "class_id"}).
		AddRow(2, "B", "desc", 7, now, 100, "all_or_nothing", false, nil, now, now, 4)

	q := `SELECT a.id, a.title, a.description, a.created_by, a.deadline,
                       a.max_points, a.grading_policy, a.published, a.template_path,
                       a.created_at, a.updated_at, a.class_id
                  FROM assignments a JOIN classes c ON c.id = a.class_id
                           WHERE c.teacher_id = $1 ORDER BY a.created_at DESC`
	mock.ExpectQuery(regexp.QuoteMeta(q)).WithArgs(7).WillReturnRows(rows)

	list, err := ListAssignments("teacher", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(list))
	}
	if list[0].ID != 2 {
		t.Fatalf("wrong assignment id: %d", list[0].ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
