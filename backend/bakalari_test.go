package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBakalariLogin(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"abc"}`))
	}))
	defer ts.Close()

	client, err := BakalariLogin(ts.URL, "user", "pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if client.AccessToken != "abc" {
		t.Fatalf("token mismatch: %s", client.AccessToken)
	}
}
