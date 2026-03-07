package database

import (
	"testing"
)

func TestBuildSearchQuery_NoFilters(t *testing.T) {
	q, args := buildSearchQuery(SearchFilters{}, 10)
	if len(args) != 2 {
		t.Fatalf("expected 2 args (vector placeholder + limit), got %d", len(args))
	}
	if q == "" {
		t.Fatal("expected non-empty query")
	}
}

func TestBuildSearchQuery_AllFilters(t *testing.T) {
	maxPrice := 0.01
	q, args := buildSearchQuery(SearchFilters{
		Network:  "base",
		Method:   "GET",
		MinPrice: nil,
		MaxPrice: &maxPrice,
	}, 5)
	if q == "" {
		t.Fatal("expected non-empty query")
	}
	// vector placeholder + network + method + maxPrice + limit = 5
	if len(args) != 5 {
		t.Fatalf("expected 5 args, got %d", len(args))
	}
}
