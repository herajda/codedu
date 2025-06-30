package main

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func TestGetAssignmentStudentForbidden(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()
	DB = sqlx.NewDb(db, "sqlmock")

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "title", "description", "created_by", "deadline", "max_points", "grading_policy", "published", "show_traceback", "template_path", "created_at", "updated_at", "class_id"}).
		AddRow(4, "A", "d", 1, now, 100, "all_or_nothing", true, false, nil, now, now, 2)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, created_by, deadline, max_points, grading_policy, published, show_traceback, template_path, created_at, updated_at, class_id FROM assignments WHERE id = $1`)).
		WithArgs(4).WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT 1 FROM assignments a JOIN class_students cs ON cs.class_id=a.class_id WHERE a.id=$1 AND cs.student_id=$2`)).
		WithArgs(4, 9).WillReturnError(sql.ErrNoRows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "4"}}
	c.Request, _ = http.NewRequest("GET", "/assignments/4", nil)
	c.Set("role", "student")
	c.Set("userID", 9)

	getAssignment(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateAssignmentTeacherForbidden(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()
	DB = sqlx.NewDb(db, "sqlmock")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT 1 FROM assignments a JOIN classes c ON c.id=a.class_id WHERE a.id=$1 AND c.teacher_id=$2`)).
		WithArgs(5, 7).WillReturnError(sql.ErrNoRows)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	body := `{"title":"t","description":"","deadline":"2023-01-01T00:00:00Z","max_points":10,"grading_policy":"all_or_nothing"}`
	c.Request, _ = http.NewRequest("PUT", "/assignments/5", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", "teacher")
	c.Set("userID", 7)

	updateAssignment(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
