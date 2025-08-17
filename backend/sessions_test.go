package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStartSessionBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/sessions", nil)
	startSessionHandler(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestStageFilesPolicy(t *testing.T) {
	// forbidden path
	_, err := stageFiles([]SessionFile{{Path: "../x", ContentB64: base64.StdEncoding.EncodeToString([]byte("a"))}})
	if err == nil {
		t.Fatalf("expected error")
	}
}
