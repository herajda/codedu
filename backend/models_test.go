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
