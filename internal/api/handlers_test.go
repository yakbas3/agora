package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleSearch_MissingQuery(t *testing.T) {
	h := &Handlers{}
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/search", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.handleSearch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleSearch_InvalidMethod(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()

	h.handleSearch(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
