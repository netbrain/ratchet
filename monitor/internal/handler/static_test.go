package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticHandler_ServesFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.js"), []byte("console.log('hello')"), 0o644)

	h := StaticHandler(dir)
	req := httptest.NewRequest(http.MethodGet, "/static/test.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if body != "console.log('hello')" {
		t.Errorf("body: got %q", body)
	}
}

func TestStaticHandler_NotFound(t *testing.T) {
	dir := t.TempDir()

	h := StaticHandler(dir)
	req := httptest.NewRequest(http.MethodGet, "/static/nonexistent.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}
