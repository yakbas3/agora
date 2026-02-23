package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClient_FetchPage(t *testing.T) {
	items := []BazaarItem{
		{
			Resource:    "https://example.com/api/test",
			Type:        "http",
			X402Version: 1,
			LastUpdated: "2026-01-01T00:00:00Z",
			Accepts: []BazaarAccept{
				{
					Description:       "Test endpoint",
					Network:           "base",
					MaxAmountRequired: "1000",
					Scheme:            "exact",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := BazaarResponse{
			Items:      items,
			Pagination: BazaarPagination{Limit: 100, Offset: 0, Total: 1},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100, 3)
	resp, err := client.FetchPage(context.Background(), 0)
	if err != nil {
		t.Fatalf("FetchPage error: %v", err)
	}
	if resp.Pagination.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Pagination.Total)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(resp.Items))
	}
	if resp.Items[0].Resource != "https://example.com/api/test" {
		t.Errorf("Resource = %q", resp.Items[0].Resource)
	}
}

func TestClient_FetchPage_RetryOnServerError(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := BazaarResponse{
			Items:      []BazaarItem{},
			Pagination: BazaarPagination{Limit: 100, Offset: 0, Total: 0},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100, 5)
	resp, err := client.FetchPage(context.Background(), 0)
	if err != nil {
		t.Fatalf("FetchPage error after retries: %v", err)
	}
	if resp.Pagination.Total != 0 {
		t.Errorf("Total = %d, want 0", resp.Pagination.Total)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}
}

func TestClient_FetchPage_ExhaustsRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, 100, 3)
	_, err := client.FetchPage(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
}

func TestClient_FetchAllPages(t *testing.T) {
	total := 250
	pageSize := 100
	callCount := atomic.Int32{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		offset := 0
		fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)

		count := pageSize
		if offset+count > total {
			count = total - offset
		}
		items := make([]BazaarItem, count)
		for i := range items {
			items[i] = BazaarItem{
				Resource:    fmt.Sprintf("https://example.com/%d", offset+i),
				Type:        "http",
				X402Version: 1,
				LastUpdated: "2026-01-01T00:00:00Z",
				Accepts:     []BazaarAccept{{Description: "test", Network: "base", MaxAmountRequired: "1000", Scheme: "exact"}},
			}
		}
		resp := BazaarResponse{
			Items:      items,
			Pagination: BazaarPagination{Limit: pageSize, Offset: offset, Total: total},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, pageSize, 3)
	items, err := client.FetchAllPages(context.Background(), 2)
	if err != nil {
		t.Fatalf("FetchAllPages error: %v", err)
	}
	if len(items) != total {
		t.Errorf("got %d items, want %d", len(items), total)
	}
	if callCount.Load() != 3 {
		t.Errorf("page fetches = %d, want 3", callCount.Load())
	}
}
