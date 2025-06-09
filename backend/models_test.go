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

func TestSettings(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	DB = sqlx.NewDb(db, "sqlmock")

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO settings (key, value) VALUES ($1,$2)
        ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`)).
		WithArgs("bakalari_url", "http://x").WillReturnResult(sqlmock.NewResult(1, 1))

	if err := SetSetting("bakalari_url", "http://x"); err != nil {
		t.Fatalf("set: %v", err)
	}

	rows := sqlmock.NewRows([]string{"value"}).AddRow("http://x")
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT value FROM settings WHERE key=$1`)).
		WithArgs("bakalari_url").WillReturnRows(rows)

	val, err := GetSetting("bakalari_url")
	if err != nil || val != "http://x" {
		t.Fatalf("got %q err %v", val, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
