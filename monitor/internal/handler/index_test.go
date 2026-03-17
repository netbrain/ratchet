package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestIndexHandler_GET_ReturnsHTML(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "index.html")
	os.WriteFile(tmplPath, []byte(`<html><body>hello</body></html>`), 0o644)

	h := IndexHandler(tmplPath)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type: got %q, want %q", ct, "text/html; charset=utf-8")
	}
}

func TestIndexHandler_POST_Returns405(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "index.html")
	os.WriteFile(tmplPath, []byte(`<html><body>hello</body></html>`), 0o644)

	h := IndexHandler(tmplPath)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if allow := rec.Header().Get("Allow"); allow == "" {
		t.Error("missing Allow header on 405 response")
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body["error"] != "method not allowed" {
		t.Errorf("error message: got %q", body["error"])
	}
}

func TestIndexHandler_PUT_Returns405(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "index.html")
	os.WriteFile(tmplPath, []byte(`<html><body>hello</body></html>`), 0o644)

	h := IndexHandler(tmplPath)
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestIndexHandler_MissingTemplate_Returns500(t *testing.T) {
	// Provide a path to a non-existent file — should not panic.
	h := IndexHandler("/nonexistent/path/index.html")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// This must not panic.
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestIndexHandler_InvalidTemplate_Returns500(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "index.html")
	// Invalid Go template syntax
	os.WriteFile(tmplPath, []byte(`<html>{{ .Invalid`), 0o644)

	h := IndexHandler(tmplPath)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
