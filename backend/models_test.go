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
	rows := sqlmock.NewRows([]string{"id", "title", "description", "created_by", "deadline", "max_points", "grading_policy", "published", "show_traceback", "template_path", "created_at", "updated_at", "class_id"}).
		AddRow(1, "A", "desc", 5, now, 100, "all_or_nothing", true, false, nil, now, now, 3)

	q := `SELECT a.id, a.title, a.description, a.created_by, a.deadline,
                       a.max_points, a.grading_policy, a.published, a.show_traceback, a.template_path,
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
	rows := sqlmock.NewRows([]string{"id", "title", "description", "created_by", "deadline", "max_points", "grading_policy", "published", "show_traceback", "template_path", "created_at", "updated_at", "class_id"}).
		AddRow(2, "B", "desc", 7, now, 100, "all_or_nothing", false, false, nil, now, now, 4)

	q := `SELECT a.id, a.title, a.description, a.created_by, a.deadline,
                       a.max_points, a.grading_policy, a.published, a.show_traceback, a.template_path,
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

func TestCreateAndGetNote(t *testing.T) {
        db, mock, err := sqlmock.New()
        if err != nil {
                t.Fatalf("failed to open sqlmock: %v", err)
        }
        defer db.Close()

        DB = sqlx.NewDb(db, "sqlmock")

        now := time.Now()
        mock.ExpectQuery(regexp.QuoteMeta(`
        INSERT INTO class_notes (class_id, author_id, content)
        VALUES ($1,$2,$3)
        RETURNING id, created_at`)).
                WithArgs(3, 5, "hello").
                WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(7, now))

        n := &ClassNote{ClassID: 3, AuthorID: 5, Content: "hello"}
        if err := CreateNote(n); err != nil {
                t.Fatalf("unexpected error: %v", err)
        }
        if n.ID != 7 {
                t.Fatalf("expected id 7, got %d", n.ID)
        }

        q := `SELECT id, class_id, author_id, content, created_at FROM class_notes WHERE id=$1`
        mock.ExpectQuery(regexp.QuoteMeta(q)).WithArgs(7).
                WillReturnRows(sqlmock.NewRows([]string{"id", "class_id", "author_id", "content", "created_at"}).
                        AddRow(7, 3, 5, "hello", now))

        got, err := GetNote(7)
        if err != nil {
                t.Fatalf("unexpected error: %v", err)
        }
        if got.Content != "hello" || got.ClassID != 3 {
                t.Fatalf("wrong note returned: %+v", got)
        }

        if err := mock.ExpectationsWereMet(); err != nil {
                t.Fatalf("unmet expectations: %v", err)
        }
}

func TestListNotes(t *testing.T) {
        db, mock, err := sqlmock.New()
        if err != nil {
                t.Fatalf("failed to open sqlmock: %v", err)
        }
        defer db.Close()

        DB = sqlx.NewDb(db, "sqlmock")

        now := time.Now()
        q := `SELECT id, class_id, author_id, content, created_at FROM class_notes WHERE class_id=$1 ORDER BY created_at DESC`
        rows := sqlmock.NewRows([]string{"id", "class_id", "author_id", "content", "created_at"}).
                AddRow(1, 3, 5, "a", now).
                AddRow(2, 3, 5, "b", now)
        mock.ExpectQuery(regexp.QuoteMeta(q)).WithArgs(3).WillReturnRows(rows)

        list, err := ListNotes(3)
        if err != nil {
                t.Fatalf("unexpected error: %v", err)
        }
        if len(list) != 2 {
                t.Fatalf("expected 2 notes, got %d", len(list))
        }

        if err := mock.ExpectationsWereMet(); err != nil {
                t.Fatalf("unmet expectations: %v", err)
        }
}
