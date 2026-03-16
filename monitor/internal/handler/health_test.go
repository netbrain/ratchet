package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_StatusCode(t *testing.T) {
	h := HealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHealthHandler_ContentType(t *testing.T) {
	h := HealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

func TestHealthHandler_Body(t *testing.T) {
	h := HealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	status, ok := body["status"]
	if !ok {
		t.Fatal("response missing 'status' key")
	}
	if status != "ok" {
		t.Errorf("status: got %q, want %q", status, "ok")
	}
}
