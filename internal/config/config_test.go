// internal/config/config_test.go
package config

import (
	"os"
	"testing"
)

func TestLoad_RequiredFields(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb")
	os.Setenv("BAZAAR_API_URL", "https://example.com/api")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("BAZAAR_API_URL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://test:test@localhost:5432/testdb" {
		t.Errorf("DatabaseURL = %q, want postgres://test:test@localhost:5432/testdb", cfg.DatabaseURL)
	}
	if cfg.BazaarAPIURL != "https://example.com/api" {
		t.Errorf("BazaarAPIURL = %q, want https://example.com/api", cfg.BazaarAPIURL)
	}
}

func TestLoad_Defaults(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb")
	os.Setenv("BAZAAR_API_URL", "https://example.com/api")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("BAZAAR_API_URL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BazaarPageSize != 100 {
		t.Errorf("BazaarPageSize = %d, want 100", cfg.BazaarPageSize)
	}
	if cfg.CrawlerConcurrency != 3 {
		t.Errorf("CrawlerConcurrency = %d, want 3", cfg.CrawlerConcurrency)
	}
	if cfg.CrawlerMaxRetries != 5 {
		t.Errorf("CrawlerMaxRetries = %d, want 5", cfg.CrawlerMaxRetries)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("BAZAAR_API_URL")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required fields, got nil")
	}
}
