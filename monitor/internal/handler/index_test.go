package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestIndexHandler_GET_ReturnsHTML(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<html><body>hello</body></html>`)},
	}

	h := IndexHandler(fsys, "index.html")
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
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<html><body>hello</body></html>`)},
	}

	h := IndexHandler(fsys, "index.html")
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
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<html><body>hello</body></html>`)},
	}

	h := IndexHandler(fsys, "index.html")
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestIndexHandler_MissingTemplate_Returns500(t *testing.T) {
	// Empty FS — template file does not exist. Should not panic.
	fsys := fstest.MapFS{}

	h := IndexHandler(fsys, "nonexistent.html")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// This must not panic.
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestIndexHandler_GET_XContentTypeOptions(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<html><body>hello</body></html>`)},
	}

	h := IndexHandler(fsys, "index.html")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	xcto := rec.Header().Get("X-Content-Type-Options")
	if xcto != "nosniff" {
		t.Errorf("X-Content-Type-Options: got %q, want %q", xcto, "nosniff")
	}
}

func TestIndexHandler_InvalidTemplate_Returns500(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<html>{{ .Invalid`)},
	}

	h := IndexHandler(fsys, "index.html")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestIndexHandler_NestedPath(t *testing.T) {
	// Verify it works with nested paths like "templates/index.html"
	fsys := fstest.MapFS{
		"templates/index.html": &fstest.MapFile{Data: []byte(`<html><body>nested</body></html>`)},
	}

	h := IndexHandler(fsys, "templates/index.html")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}
