package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedClient_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/embed" {
			t.Fatalf("expected /embed, got %s", r.URL.Path)
		}

		var req embedRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Text == "" {
			t.Fatal("expected non-empty text")
		}

		resp := embedResponse{Embedding: make([]float32, 384)}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewEmbedClient(server.URL)
	vec, err := client.Embed("test query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != 384 {
		t.Fatalf("expected 384 dims, got %d", len(vec))
	}
}
