package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStaticHandler_ServesFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"test.js": {Data: []byte("console.log('hello')")},
	}

	h := StaticHandler(fsys)
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
	fsys := fstest.MapFS{}

	h := StaticHandler(fsys)
	req := httptest.NewRequest(http.MethodGet, "/static/nonexistent.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}
