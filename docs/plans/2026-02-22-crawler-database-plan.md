# Phase 1: Crawler & Database Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go-based crawler that paginates the Coinbase Bazaar discovery API, normalizes x402 endpoint metadata, and stores it in PostgreSQL with pgvector support.

**Architecture:** Async batch crawler using goroutines + errgroup. Raw Bazaar API responses are normalized into `endpoints` and `payment_options` tables via pgx batch upserts. Crawl runs are tracked for observability. The crawler is triggered via CLI command.

**Tech Stack:** Go 1.22+, pgx v5, golang-migrate, pgvector-go, envconfig, godotenv, errgroup

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/agora/main.go`
- Create: `.env.example`
- Create: `.gitignore`
- Create: all `internal/` directory stubs

**Step 1: Initialize Go module and install dependencies**

Run:
```bash
cd c:/Users/yaman/Desktop/agora
go mod init github.com/yamanakbas/agora
go get github.com/jackc/pgx/v5
go get github.com/pgvector/pgvector-go
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/postgres
go get github.com/golang-migrate/migrate/v4/source/iofs
go get github.com/kelseyhightower/envconfig
go get github.com/joho/godotenv
go get golang.org/x/sync
go get github.com/google/uuid
```

**Step 2: Create directory structure**

```bash
mkdir -p cmd/agora
mkdir -p internal/config
mkdir -p internal/database
mkdir -p internal/models
mkdir -p internal/crawler
mkdir -p internal/api
mkdir -p migrations
```

**Step 3: Create `.env.example`**

```
# Database
DATABASE_URL=postgres://user:password@localhost:5432/agora?sslmode=disable

# Bazaar API
BAZAAR_API_URL=https://api.cdp.coinbase.com/platform/v2/x402/discovery/resources
BAZAAR_PAGE_SIZE=100

# Crawler
CRAWLER_CONCURRENCY=3
CRAWLER_MAX_RETRIES=5

# OpenAI (future - for embeddings)
# OPENAI_API_KEY=sk-...
```

**Step 4: Create `.gitignore`**

```
# Binaries
/agora
*.exe

# Environment
.env

# IDE
.idea/
.vscode/

# OS
.DS_Store
Thumbs.db

# Go
/vendor/
```

**Step 5: Create stub `cmd/agora/main.go`**

```go
package main

import "fmt"

func main() {
	fmt.Println("agora")
}
```

**Step 6: Verify it compiles**

Run: `go build ./cmd/agora/`
Expected: no errors, produces `agora.exe` binary

**Step 7: Commit**

```bash
git add go.mod go.sum cmd/ internal/ migrations/ .env.example .gitignore
git commit -m "scaffold: init Go module with dependencies and directory structure"
```

---

### Task 2: Configuration

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing test**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"testing"
)

func TestLoad_RequiredFields(t *testing.T) {
	// Set required env vars
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL — `Load` not defined

**Step 3: Write minimal implementation**

```go
// internal/config/config.go
package config

import (
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DatabaseURL        string `envconfig:"DATABASE_URL" required:"true"`
	BazaarAPIURL       string `envconfig:"BAZAAR_API_URL" required:"true"`
	BazaarPageSize     int    `envconfig:"BAZAAR_PAGE_SIZE" default:"100"`
	CrawlerConcurrency int    `envconfig:"CRAWLER_CONCURRENCY" default:"3"`
	CrawlerMaxRetries  int    `envconfig:"CRAWLER_MAX_RETRIES" default:"5"`
	OpenAIAPIKey       string `envconfig:"OPENAI_API_KEY"`
}

func Load() (*Config, error) {
	// Best-effort .env loading — ignore error if file doesn't exist
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS (3/3 tests)

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config loading from environment variables"
```

---

### Task 3: Models

**Files:**
- Create: `internal/models/endpoint.go`
- Create: `internal/models/payment_option.go`
- Create: `internal/models/crawl_run.go`

**Step 1: Write endpoint model**

```go
// internal/models/endpoint.go
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Endpoint struct {
	ID           uuid.UUID       `db:"id"`
	ResourceURL  string          `db:"resource_url"`
	Domain       string          `db:"domain"`
	Type         string          `db:"type"`
	X402Version  int             `db:"x402_version"`
	Description  string          `db:"description"`
	HTTPMethod   string          `db:"http_method"`
	InputSchema  json.RawMessage `db:"input_schema"`
	OutputSchema json.RawMessage `db:"output_schema"`
	RawMetadata  json.RawMessage `db:"raw_metadata"`
	LastUpdated  time.Time       `db:"last_updated"`
	FirstSeen    time.Time       `db:"first_seen"`
	LastCrawled  time.Time       `db:"last_crawled"`
}
```

**Step 2: Write payment option model**

```go
// internal/models/payment_option.go
package models

import (
	"encoding/json"

	"github.com/google/uuid"
)

type PaymentOption struct {
	ID                uuid.UUID       `db:"id"`
	EndpointID        uuid.UUID       `db:"endpoint_id"`
	Scheme            string          `db:"scheme"`
	NetworkRaw        string          `db:"network_raw"`
	NetworkNormalized string          `db:"network_normalized"`
	AssetAddress      string          `db:"asset_address"`
	AssetName         string          `db:"asset_name"`
	MaxAmountRaw      string          `db:"max_amount_raw"`
	PriceUSD          float64         `db:"price_usd"`
	PayTo             string          `db:"pay_to"`
	MaxTimeoutSeconds int             `db:"max_timeout_seconds"`
	MimeType          string          `db:"mime_type"`
	Description       string          `db:"description"`
	OutputSchemaRaw   json.RawMessage `db:"output_schema_raw"`
}
```

**Step 3: Write crawl run model**

```go
// internal/models/crawl_run.go
package models

import (
	"time"

	"github.com/google/uuid"
)

type CrawlRun struct {
	ID               uuid.UUID  `db:"id"`
	StartedAt        time.Time  `db:"started_at"`
	CompletedAt      *time.Time `db:"completed_at"`
	TotalFetched     int        `db:"total_fetched"`
	NewEndpoints     int        `db:"new_endpoints"`
	UpdatedEndpoints int        `db:"updated_endpoints"`
	Status           string     `db:"status"`
	Error            *string    `db:"error"`
}
```

**Step 4: Verify it compiles**

Run: `go build ./internal/models/`
Expected: no errors

**Step 5: Commit**

```bash
git add internal/models/
git commit -m "feat: add database models for endpoints, payment options, and crawl runs"
```

---

### Task 4: Network Normalization

**Files:**
- Create: `internal/crawler/network.go`
- Create: `internal/crawler/network_test.go`

**Step 1: Write the failing test**

```go
// internal/crawler/network_test.go
package crawler

import "testing"

func TestNormalizeNetwork(t *testing.T) {
	tests := []struct {
		raw      string
		expected string
	}{
		{"base", "base"},
		{"eip155:8453", "base"},
		{"base-sepolia", "base-sepolia"},
		{"eip155:84532", "base-sepolia"},
		{"solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp", "solana"},
		{"ethereum", "ethereum"},
		{"eip155:1", "ethereum"},
		{"unknown-network", "unknown-network"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := NormalizeNetwork(tt.raw)
			if got != tt.expected {
				t.Errorf("NormalizeNetwork(%q) = %q, want %q", tt.raw, got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/crawler/ -run TestNormalizeNetwork -v`
Expected: FAIL — `NormalizeNetwork` not defined

**Step 3: Write minimal implementation**

```go
// internal/crawler/network.go
package crawler

import "strings"

// networkMap maps chain IDs and aliases to canonical network names.
var networkMap = map[string]string{
	"eip155:8453":  "base",
	"eip155:84532": "base-sepolia",
	"eip155:1":     "ethereum",
	"eip155:11155111": "ethereum-sepolia",
}

// NormalizeNetwork converts raw network identifiers to canonical names.
// Solana network strings (starting with "solana:") are normalized to "solana".
// Known EIP-155 chain IDs are mapped to human-readable names.
// Unknown values are returned as-is.
func NormalizeNetwork(raw string) string {
	if raw == "" {
		return ""
	}
	if mapped, ok := networkMap[raw]; ok {
		return mapped
	}
	if strings.HasPrefix(raw, "solana:") {
		return "solana"
	}
	return raw
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/crawler/ -run TestNormalizeNetwork -v`
Expected: PASS (all 9 sub-tests)

**Step 5: Commit**

```bash
git add internal/crawler/network.go internal/crawler/network_test.go
git commit -m "feat: add network name normalization for chain IDs"
```

---

### Task 5: Bazaar API Types & Normalizer

**Files:**
- Create: `internal/crawler/types.go`
- Create: `internal/crawler/normalizer.go`
- Create: `internal/crawler/normalizer_test.go`

**Step 1: Write Bazaar API response types**

These match the exact JSON structure from the Bazaar API.

```go
// internal/crawler/types.go
package crawler

import "encoding/json"

// BazaarResponse is the top-level response from the Bazaar discovery API.
type BazaarResponse struct {
	Items      []BazaarItem    `json:"items"`
	Pagination BazaarPagination `json:"pagination"`
	X402Version int             `json:"x402Version"`
}

type BazaarPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// BazaarItem is a single endpoint entry from the Bazaar API.
type BazaarItem struct {
	Resource    string          `json:"resource"`
	Type        string          `json:"type"`
	X402Version int             `json:"x402Version"`
	LastUpdated string          `json:"lastUpdated"`
	Metadata    json.RawMessage `json:"metadata"`
	Accepts     []BazaarAccept  `json:"accepts"`
}

// BazaarAccept is one payment option within an endpoint.
type BazaarAccept struct {
	Asset              string          `json:"asset"`
	Description        string          `json:"description"`
	Extra              BazaarExtra     `json:"extra"`
	MaxAmountRequired  string          `json:"maxAmountRequired"`
	MaxTimeoutSeconds  int             `json:"maxTimeoutSeconds"`
	MimeType           string          `json:"mimeType"`
	Network            string          `json:"network"`
	OutputSchema       json.RawMessage `json:"outputSchema"`
	PayTo              string          `json:"payTo"`
	Resource           string          `json:"resource"`
	Scheme             string          `json:"scheme"`
}

type BazaarExtra struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	FeePayer string `json:"feePayer"`
}
```

**Step 2: Write the failing normalizer test**

```go
// internal/crawler/normalizer_test.go
package crawler

import (
	"encoding/json"
	"testing"
)

const testItemJSON = `{
	"resource": "https://weather.hugen.tokyo/weather/current",
	"type": "http",
	"x402Version": 2,
	"lastUpdated": "2026-02-23T00:21:38.181Z",
	"metadata": {},
	"accepts": [
		{
			"asset": "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
			"description": "Get current weather conditions for any city worldwide.",
			"extra": {"name": "USD Coin", "version": "2"},
			"maxAmountRequired": "1000",
			"maxTimeoutSeconds": 300,
			"mimeType": "application/json",
			"network": "eip155:8453",
			"outputSchema": {
				"input": {"method": "GET", "queryParams": {"city": "Tokyo"}, "type": "http"},
				"output": {"example": {"temperature_c": 12.5}, "type": "json"}
			},
			"payTo": "0x29322Ea7EcB34aA6164cb2ddeB9CE650902E4f60",
			"resource": "https://weather.hugen.tokyo/weather/current",
			"scheme": "exact"
		}
	]
}`

const testMultiAcceptJSON = `{
	"resource": "https://img402.dev/api/upload",
	"type": "http",
	"x402Version": 2,
	"lastUpdated": "2026-02-20T14:13:40.271Z",
	"metadata": {},
	"accepts": [
		{
			"asset": "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
			"description": "Upload an image with payment and get a public URL. 5MB max, 1-year retention.",
			"extra": {"name": "USD Coin", "version": "2"},
			"maxAmountRequired": "10000",
			"maxTimeoutSeconds": 300,
			"mimeType": "application/json",
			"network": "eip155:8453",
			"outputSchema": {"input": {"method": "POST", "type": "http"}},
			"payTo": "0x77f3c9bCb898Ad1d30e9a336E2cC3108d88D6c09",
			"resource": "https://img402.dev/api/upload",
			"scheme": "exact"
		},
		{
			"asset": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"description": "Upload an image with payment and get a public URL. 5MB max, 1-year retention.",
			"extra": {"feePayer": "BENrLoUbndxoNMUS5JXApGMtNykLjFXXixMtpDwDR9SP"},
			"maxAmountRequired": "10000",
			"maxTimeoutSeconds": 300,
			"mimeType": "application/json",
			"network": "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp",
			"outputSchema": {"input": {"method": "POST", "type": "http"}},
			"payTo": "9xuipz9Y13v6Z1FnLR3XkrknoBLngwUSstjxcqK6deYX",
			"resource": "https://img402.dev/api/upload",
			"scheme": "exact"
		}
	]
}`

func TestNormalizeItem_BasicFields(t *testing.T) {
	var item BazaarItem
	if err := json.Unmarshal([]byte(testItemJSON), &item); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	endpoint, options, err := NormalizeItem(item)
	if err != nil {
		t.Fatalf("NormalizeItem error: %v", err)
	}

	if endpoint.ResourceURL != "https://weather.hugen.tokyo/weather/current" {
		t.Errorf("ResourceURL = %q", endpoint.ResourceURL)
	}
	if endpoint.Domain != "weather.hugen.tokyo" {
		t.Errorf("Domain = %q, want weather.hugen.tokyo", endpoint.Domain)
	}
	if endpoint.X402Version != 2 {
		t.Errorf("X402Version = %d, want 2", endpoint.X402Version)
	}
	if endpoint.HTTPMethod != "GET" {
		t.Errorf("HTTPMethod = %q, want GET", endpoint.HTTPMethod)
	}
	if endpoint.Description != "Get current weather conditions for any city worldwide." {
		t.Errorf("Description = %q", endpoint.Description)
	}
	if len(options) != 1 {
		t.Fatalf("got %d options, want 1", len(options))
	}
	if options[0].NetworkNormalized != "base" {
		t.Errorf("NetworkNormalized = %q, want base", options[0].NetworkNormalized)
	}
	if options[0].AssetName != "USD Coin" {
		t.Errorf("AssetName = %q, want USD Coin", options[0].AssetName)
	}
	if options[0].PriceUSD != 0.001 {
		t.Errorf("PriceUSD = %f, want 0.001", options[0].PriceUSD)
	}
}

func TestNormalizeItem_MultiAccept(t *testing.T) {
	var item BazaarItem
	if err := json.Unmarshal([]byte(testMultiAcceptJSON), &item); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	endpoint, options, err := NormalizeItem(item)
	if err != nil {
		t.Fatalf("NormalizeItem error: %v", err)
	}

	if endpoint.Domain != "img402.dev" {
		t.Errorf("Domain = %q, want img402.dev", endpoint.Domain)
	}
	if endpoint.HTTPMethod != "POST" {
		t.Errorf("HTTPMethod = %q, want POST", endpoint.HTTPMethod)
	}
	if len(options) != 2 {
		t.Fatalf("got %d options, want 2", len(options))
	}
	if options[0].NetworkNormalized != "base" {
		t.Errorf("options[0].NetworkNormalized = %q, want base", options[0].NetworkNormalized)
	}
	if options[1].NetworkNormalized != "solana" {
		t.Errorf("options[1].NetworkNormalized = %q, want solana", options[1].NetworkNormalized)
	}
	// Solana entry should use feePayer as asset name fallback
	if options[1].AssetName != "" {
		t.Errorf("options[1].AssetName = %q, want empty (no name in extra)", options[1].AssetName)
	}
}

func TestNormalizeItem_PicksLongestDescription(t *testing.T) {
	var item BazaarItem
	if err := json.Unmarshal([]byte(testMultiAcceptJSON), &item); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	endpoint, _, err := NormalizeItem(item)
	if err != nil {
		t.Fatalf("NormalizeItem error: %v", err)
	}

	expected := "Upload an image with payment and get a public URL. 5MB max, 1-year retention."
	if endpoint.Description != expected {
		t.Errorf("Description = %q, want %q", endpoint.Description, expected)
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/crawler/ -run TestNormalizeItem -v`
Expected: FAIL — `NormalizeItem` not defined

**Step 4: Write normalizer implementation**

```go
// internal/crawler/normalizer.go
package crawler

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/yamanakbas/agora/internal/models"
)

// assetDecimals maps known asset contract addresses to their decimal places.
var assetDecimals = map[string]int{
	"0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913": 6, // USDC on Base
	"0x036CbD53842c5426634e7929541eC2318f3dCF7e": 6, // USDC on Base Sepolia
	"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": 6, // USDC on Solana
}

// NormalizeItem converts a raw Bazaar API item into an Endpoint and its PaymentOptions.
func NormalizeItem(item BazaarItem) (models.Endpoint, []models.PaymentOption, error) {
	now := time.Now().UTC()

	parsedURL, err := url.Parse(item.Resource)
	if err != nil {
		return models.Endpoint{}, nil, fmt.Errorf("parse resource URL %q: %w", item.Resource, err)
	}

	lastUpdated, err := time.Parse(time.RFC3339Nano, item.LastUpdated)
	if err != nil {
		return models.Endpoint{}, nil, fmt.Errorf("parse lastUpdated %q: %w", item.LastUpdated, err)
	}

	// Extract HTTP method and schemas from the best (first rich) outputSchema
	httpMethod, inputSchema, outputSchema := extractSchemas(item.Accepts)

	// Pick the longest description across all accepts entries
	description := pickBestDescription(item.Accepts)

	endpoint := models.Endpoint{
		ID:           uuid.New(),
		ResourceURL:  item.Resource,
		Domain:       parsedURL.Hostname(),
		Type:         item.Type,
		X402Version:  item.X402Version,
		Description:  description,
		HTTPMethod:   httpMethod,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		RawMetadata:  item.Metadata,
		LastUpdated:  lastUpdated,
		FirstSeen:    now,
		LastCrawled:  now,
	}

	options := make([]models.PaymentOption, 0, len(item.Accepts))
	for _, accept := range item.Accepts {
		priceUSD := computePriceUSD(accept.MaxAmountRequired, accept.Asset)

		option := models.PaymentOption{
			ID:                uuid.New(),
			Scheme:            accept.Scheme,
			NetworkRaw:        accept.Network,
			NetworkNormalized: NormalizeNetwork(accept.Network),
			AssetAddress:      accept.Asset,
			AssetName:         accept.Extra.Name,
			MaxAmountRaw:      accept.MaxAmountRequired,
			PriceUSD:          priceUSD,
			PayTo:             accept.PayTo,
			MaxTimeoutSeconds: accept.MaxTimeoutSeconds,
			MimeType:          accept.MimeType,
			Description:       accept.Description,
			OutputSchemaRaw:   accept.OutputSchema,
		}
		options = append(options, option)
	}

	return endpoint, options, nil
}

// pickBestDescription returns the longest description from the accepts list.
func pickBestDescription(accepts []BazaarAccept) string {
	best := ""
	for _, a := range accepts {
		if len(a.Description) > len(best) {
			best = a.Description
		}
	}
	return best
}

// extractSchemas extracts HTTP method, input schema, and output schema
// from the first accepts entry that has a non-minimal outputSchema.
func extractSchemas(accepts []BazaarAccept) (string, json.RawMessage, json.RawMessage) {
	method := ""

	for _, a := range accepts {
		var schema map[string]json.RawMessage
		if err := json.Unmarshal(a.OutputSchema, &schema); err != nil {
			continue
		}

		// Extract method from input
		if inputRaw, ok := schema["input"]; ok {
			var input struct {
				Method string `json:"method"`
			}
			if err := json.Unmarshal(inputRaw, &input); err == nil && input.Method != "" {
				method = input.Method
			}
		}

		// Check if this has an output section (rich schema)
		outputRaw, hasOutput := schema["output"]
		if hasOutput {
			inputRaw := schema["input"]
			return method, inputRaw, outputRaw
		}
	}

	// Fallback: use the first entry's input schema even if no output
	if len(accepts) > 0 {
		var schema map[string]json.RawMessage
		if err := json.Unmarshal(accepts[0].OutputSchema, &schema); err == nil {
			if inputRaw, ok := schema["input"]; ok {
				return method, inputRaw, nil
			}
		}
	}

	return method, nil, nil
}

// computePriceUSD converts the raw amount string to a USD value
// based on the asset's known decimal places. Defaults to 6 decimals (USDC).
func computePriceUSD(rawAmount string, assetAddress string) float64 {
	amount, err := strconv.ParseFloat(rawAmount, 64)
	if err != nil {
		return 0
	}
	decimals := 6 // default to USDC
	if d, ok := assetDecimals[assetAddress]; ok {
		decimals = d
	}
	divisor := 1.0
	for i := 0; i < decimals; i++ {
		divisor *= 10
	}
	return amount / divisor
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/crawler/ -v`
Expected: PASS (all normalizer + network tests)

**Step 6: Commit**

```bash
git add internal/crawler/types.go internal/crawler/normalizer.go internal/crawler/normalizer_test.go
git commit -m "feat: add Bazaar API types and normalizer with price computation"
```

---

### Task 6: Database Connection & Migrations

**Files:**
- Create: `internal/database/db.go`
- Create: `migrations/000001_create_endpoints.up.sql`
- Create: `migrations/000001_create_endpoints.down.sql`
- Create: `migrations/000002_create_payment_options.up.sql`
- Create: `migrations/000002_create_payment_options.down.sql`
- Create: `migrations/000003_create_crawl_runs.up.sql`
- Create: `migrations/000003_create_crawl_runs.down.sql`

**Step 1: Write database connection module**

```go
// internal/database/db.go
package database

import (
	"context"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

//go:embed ../../../migrations/*.sql
var migrationsFS embed.FS

// NewPool creates a PostgreSQL connection pool with pgvector type support.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// RunMigrations applies all pending database migrations.
func RunMigrations(databaseURL string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
```

> **Note:** The embed path `../../../migrations/*.sql` assumes `db.go` is at `internal/database/db.go` and migrations are at the project root `migrations/`. If the Go embed directive doesn't resolve at compile time, we will adjust to use a different migration loading strategy (e.g., passing the path at runtime). This should be verified in Step 5.

**Step 2: Write migration — create endpoints table**

```sql
-- migrations/000001_create_endpoints.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

CREATE TABLE endpoints (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resource_url   TEXT NOT NULL UNIQUE,
    domain         TEXT NOT NULL,
    type           TEXT NOT NULL DEFAULT 'http',
    x402_version   INTEGER NOT NULL DEFAULT 1,
    description    TEXT NOT NULL DEFAULT '',
    http_method    TEXT NOT NULL DEFAULT '',
    input_schema   JSONB,
    output_schema  JSONB,
    raw_metadata   JSONB,
    last_updated   TIMESTAMPTZ NOT NULL,
    first_seen     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_crawled   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    embedding      vector(1536)
);

CREATE INDEX idx_endpoints_domain ON endpoints (domain);
CREATE INDEX idx_endpoints_http_method ON endpoints (http_method);
```

```sql
-- migrations/000001_create_endpoints.down.sql
DROP TABLE IF EXISTS endpoints;
```

**Step 3: Write migration — create payment_options table**

```sql
-- migrations/000002_create_payment_options.up.sql
CREATE TABLE payment_options (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    endpoint_id         UUID NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
    scheme              TEXT NOT NULL DEFAULT 'exact',
    network_raw         TEXT NOT NULL,
    network_normalized  TEXT NOT NULL,
    asset_address       TEXT NOT NULL,
    asset_name          TEXT NOT NULL DEFAULT '',
    max_amount_raw      TEXT NOT NULL,
    price_usd           NUMERIC(20, 10) NOT NULL DEFAULT 0,
    pay_to              TEXT NOT NULL,
    max_timeout_seconds INTEGER NOT NULL DEFAULT 300,
    mime_type           TEXT NOT NULL DEFAULT '',
    description         TEXT NOT NULL DEFAULT '',
    output_schema_raw   JSONB
);

CREATE INDEX idx_payment_options_endpoint_id ON payment_options (endpoint_id);
CREATE INDEX idx_payment_options_network ON payment_options (network_normalized);
CREATE INDEX idx_payment_options_price ON payment_options (price_usd);
```

```sql
-- migrations/000002_create_payment_options.down.sql
DROP TABLE IF EXISTS payment_options;
```

**Step 4: Write migration — create crawl_runs table**

```sql
-- migrations/000003_create_crawl_runs.up.sql
CREATE TABLE crawl_runs (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    started_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at       TIMESTAMPTZ,
    total_fetched      INTEGER NOT NULL DEFAULT 0,
    new_endpoints      INTEGER NOT NULL DEFAULT 0,
    updated_endpoints  INTEGER NOT NULL DEFAULT 0,
    status             TEXT NOT NULL DEFAULT 'running',
    error              TEXT
);
```

```sql
-- migrations/000003_create_crawl_runs.down.sql
DROP TABLE IF EXISTS crawl_runs;
```

**Step 5: Verify it compiles**

Run: `go build ./internal/database/`

> If the `//go:embed` directive fails because Go doesn't allow embedding files outside the module with relative `../` paths, switch to this approach: remove the embed and make `RunMigrations` accept a filesystem path, using `migrate.New("file://"+path, databaseURL)` instead. Adjust the code accordingly.

**Step 6: Commit**

```bash
git add internal/database/ migrations/
git commit -m "feat: add database connection pool, pgvector support, and schema migrations"
```

---

### Task 7: Database Repository

**Files:**
- Create: `internal/database/repository.go`

**Step 1: Write repository for upserts and crawl run management**

```go
// internal/database/repository.go
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yamanakbas/agora/internal/models"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// StartCrawlRun inserts a new crawl run record and returns its ID.
func (r *Repository) StartCrawlRun(ctx context.Context) (uuid.UUID, error) {
	id := uuid.New()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO crawl_runs (id, started_at, status) VALUES ($1, $2, 'running')`,
		id, time.Now().UTC(),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("start crawl run: %w", err)
	}
	return id, nil
}

// CompleteCrawlRun marks a crawl run as completed with stats.
func (r *Repository) CompleteCrawlRun(ctx context.Context, id uuid.UUID, totalFetched, newEndpoints, updatedEndpoints int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE crawl_runs
		 SET completed_at = $2, total_fetched = $3, new_endpoints = $4,
		     updated_endpoints = $5, status = 'completed'
		 WHERE id = $1`,
		id, time.Now().UTC(), totalFetched, newEndpoints, updatedEndpoints,
	)
	return err
}

// FailCrawlRun marks a crawl run as failed with an error message.
func (r *Repository) FailCrawlRun(ctx context.Context, id uuid.UUID, crawlErr string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE crawl_runs
		 SET completed_at = $2, status = 'failed', error = $3
		 WHERE id = $1`,
		id, time.Now().UTC(), crawlErr,
	)
	return err
}

// UpsertEndpoint inserts or updates an endpoint and replaces its payment options.
// Returns (isNew, isUpdated, error).
func (r *Repository) UpsertEndpoint(ctx context.Context, endpoint models.Endpoint, options []models.PaymentOption) (bool, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Try to get existing endpoint
	var existingID uuid.UUID
	var existingLastUpdated time.Time
	err = tx.QueryRow(ctx,
		`SELECT id, last_updated FROM endpoints WHERE resource_url = $1`,
		endpoint.ResourceURL,
	).Scan(&existingID, &existingLastUpdated)

	isNew := false
	isUpdated := false

	if err == pgx.ErrNoRows {
		// Insert new endpoint
		isNew = true
		_, err = tx.Exec(ctx,
			`INSERT INTO endpoints (id, resource_url, domain, type, x402_version,
			   description, http_method, input_schema, output_schema, raw_metadata,
			   last_updated, first_seen, last_crawled)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			endpoint.ID, endpoint.ResourceURL, endpoint.Domain, endpoint.Type,
			endpoint.X402Version, endpoint.Description, endpoint.HTTPMethod,
			endpoint.InputSchema, endpoint.OutputSchema, endpoint.RawMetadata,
			endpoint.LastUpdated, endpoint.FirstSeen, endpoint.LastCrawled,
		)
		if err != nil {
			return false, false, fmt.Errorf("insert endpoint: %w", err)
		}
	} else if err != nil {
		return false, false, fmt.Errorf("query existing endpoint: %w", err)
	} else {
		// Existing endpoint — update if lastUpdated changed
		endpoint.ID = existingID
		if endpoint.LastUpdated.After(existingLastUpdated) {
			isUpdated = true
			_, err = tx.Exec(ctx,
				`UPDATE endpoints
				 SET domain = $2, type = $3, x402_version = $4, description = $5,
				     http_method = $6, input_schema = $7, output_schema = $8,
				     raw_metadata = $9, last_updated = $10, last_crawled = $11
				 WHERE id = $1`,
				endpoint.ID, endpoint.Domain, endpoint.Type, endpoint.X402Version,
				endpoint.Description, endpoint.HTTPMethod, endpoint.InputSchema,
				endpoint.OutputSchema, endpoint.RawMetadata, endpoint.LastUpdated,
				endpoint.LastCrawled,
			)
			if err != nil {
				return false, false, fmt.Errorf("update endpoint: %w", err)
			}
		} else {
			// Just touch last_crawled
			_, err = tx.Exec(ctx,
				`UPDATE endpoints SET last_crawled = $2 WHERE id = $1`,
				endpoint.ID, endpoint.LastCrawled,
			)
			if err != nil {
				return false, false, fmt.Errorf("touch last_crawled: %w", err)
			}
		}

		// Delete old payment options
		_, err = tx.Exec(ctx,
			`DELETE FROM payment_options WHERE endpoint_id = $1`,
			endpoint.ID,
		)
		if err != nil {
			return false, false, fmt.Errorf("delete old payment options: %w", err)
		}
	}

	// Insert new payment options
	if len(options) > 0 {
		batch := &pgx.Batch{}
		for _, opt := range options {
			opt.EndpointID = endpoint.ID
			batch.Queue(
				`INSERT INTO payment_options (id, endpoint_id, scheme, network_raw,
				   network_normalized, asset_address, asset_name, max_amount_raw,
				   price_usd, pay_to, max_timeout_seconds, mime_type, description,
				   output_schema_raw)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
				opt.ID, endpoint.ID, opt.Scheme, opt.NetworkRaw,
				opt.NetworkNormalized, opt.AssetAddress, opt.AssetName,
				opt.MaxAmountRaw, opt.PriceUSD, opt.PayTo,
				opt.MaxTimeoutSeconds, opt.MimeType, opt.Description,
				opt.OutputSchemaRaw,
			)
		}
		br := tx.SendBatch(ctx, batch)
		for range options {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return false, false, fmt.Errorf("insert payment option: %w", err)
			}
		}
		br.Close()
	}

	if err := tx.Commit(ctx); err != nil {
		return false, false, fmt.Errorf("commit tx: %w", err)
	}

	return isNew, isUpdated, nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/database/`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/database/repository.go
git commit -m "feat: add repository with endpoint upsert and crawl run management"
```

---

### Task 8: Bazaar API Client

**Files:**
- Create: `internal/crawler/client.go`
- Create: `internal/crawler/client_test.go`

**Step 1: Write the failing test using a mock HTTP server**

```go
// internal/crawler/client_test.go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/crawler/ -run TestClient -v`
Expected: FAIL — `NewClient`, `FetchPage`, `FetchAllPages` not defined

**Step 3: Write client implementation**

```go
// internal/crawler/client.go
package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type Client struct {
	baseURL    string
	pageSize   int
	maxRetries int
	httpClient *http.Client
}

func NewClient(baseURL string, pageSize, maxRetries int) *Client {
	return &Client{
		baseURL:    baseURL,
		pageSize:   pageSize,
		maxRetries: maxRetries,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchPage fetches a single page from the Bazaar API with retry and backoff.
func (c *Client) FetchPage(ctx context.Context, offset int) (*BazaarResponse, error) {
	url := fmt.Sprintf("%s?limit=%d&offset=%d", c.baseURL, c.pageSize, offset)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected HTTP %d from %s", resp.StatusCode, url)
		}

		if err != nil {
			lastErr = fmt.Errorf("read response body: %w", err)
			continue
		}

		var bazaarResp BazaarResponse
		if err := json.Unmarshal(body, &bazaarResp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		return &bazaarResp, nil
	}

	return nil, fmt.Errorf("exhausted %d retries: %w", c.maxRetries, lastErr)
}

// FetchAllPages fetches all pages from the Bazaar API using concurrent goroutines.
// concurrency controls how many pages are fetched in parallel.
func (c *Client) FetchAllPages(ctx context.Context, concurrency int) ([]BazaarItem, error) {
	// First, fetch page 0 to learn the total count
	firstResp, err := c.FetchPage(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("fetch first page: %w", err)
	}

	total := firstResp.Pagination.Total
	log.Printf("Bazaar API reports %d total endpoints", total)

	allItems := make([]BazaarItem, 0, total)
	allItems = append(allItems, firstResp.Items...)

	if total <= c.pageSize {
		return allItems, nil
	}

	// Build list of remaining offsets
	var offsets []int
	for offset := c.pageSize; offset < total; offset += c.pageSize {
		offsets = append(offsets, offset)
	}

	// Fetch remaining pages concurrently
	type pageResult struct {
		offset int
		items  []BazaarItem
	}
	results := make([]pageResult, len(offsets))
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for i, offset := range offsets {
		i, offset := i, offset
		g.Go(func() error {
			resp, err := c.FetchPage(ctx, offset)
			if err != nil {
				return fmt.Errorf("fetch page at offset %d: %w", offset, err)
			}
			mu.Lock()
			results[i] = pageResult{offset: offset, items: resp.Items}
			mu.Unlock()
			log.Printf("Fetched page at offset %d (%d items)", offset, len(resp.Items))
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	for _, r := range results {
		allItems = append(allItems, r.items...)
	}

	return allItems, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/crawler/ -v`
Expected: PASS (all tests)

**Step 5: Commit**

```bash
git add internal/crawler/client.go internal/crawler/client_test.go
git commit -m "feat: add Bazaar API client with pagination, retry, and concurrent fetching"
```

---

### Task 9: Crawler Runner

**Files:**
- Create: `internal/crawler/runner.go`

**Step 1: Write runner that orchestrates a full crawl**

```go
// internal/crawler/runner.go
package crawler

import (
	"context"
	"fmt"
	"log"

	"github.com/yamanakbas/agora/internal/database"
)

type Runner struct {
	client *Client
	repo   *database.Repository
}

func NewRunner(client *Client, repo *database.Repository) *Runner {
	return &Runner{client: client, repo: repo}
}

// Run executes a full crawl: fetches all pages, normalizes, and upserts into the database.
func (r *Runner) Run(ctx context.Context, concurrency int) error {
	crawlID, err := r.repo.StartCrawlRun(ctx)
	if err != nil {
		return fmt.Errorf("start crawl run: %w", err)
	}
	log.Printf("Started crawl run %s", crawlID)

	items, err := r.client.FetchAllPages(ctx, concurrency)
	if err != nil {
		_ = r.repo.FailCrawlRun(ctx, crawlID, err.Error())
		return fmt.Errorf("fetch pages: %w", err)
	}
	log.Printf("Fetched %d items from Bazaar API", len(items))

	newCount := 0
	updatedCount := 0
	skippedCount := 0

	for i, item := range items {
		endpoint, options, err := NormalizeItem(item)
		if err != nil {
			log.Printf("WARN: skip item %q: %v", item.Resource, err)
			skippedCount++
			continue
		}

		isNew, isUpdated, err := r.repo.UpsertEndpoint(ctx, endpoint, options)
		if err != nil {
			log.Printf("WARN: upsert failed for %q: %v", item.Resource, err)
			skippedCount++
			continue
		}

		if isNew {
			newCount++
		} else if isUpdated {
			updatedCount++
		}

		if (i+1)%500 == 0 {
			log.Printf("Progress: %d/%d items processed (%d new, %d updated, %d skipped)",
				i+1, len(items), newCount, updatedCount, skippedCount)
		}
	}

	log.Printf("Crawl complete: %d total, %d new, %d updated, %d skipped",
		len(items), newCount, updatedCount, skippedCount)

	if err := r.repo.CompleteCrawlRun(ctx, crawlID, len(items), newCount, updatedCount); err != nil {
		return fmt.Errorf("complete crawl run: %w", err)
	}

	return nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/crawler/`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/crawler/runner.go
git commit -m "feat: add crawler runner that orchestrates full crawl with progress logging"
```

---

### Task 10: CLI Entrypoint

**Files:**
- Modify: `cmd/agora/main.go`

**Step 1: Write the CLI with crawl and migrate commands**

```go
// cmd/agora/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yamanakbas/agora/internal/config"
	"github.com/yamanakbas/agora/internal/crawler"
	"github.com/yamanakbas/agora/internal/database"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	switch os.Args[1] {
	case "migrate":
		runMigrate(cfg)
	case "crawl":
		runCrawl(cfg)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: agora <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  migrate   Run database migrations")
	fmt.Fprintln(os.Stderr, "  crawl     Crawl the Bazaar API and populate the database")
}

func runMigrate(cfg *config.Config) {
	log.Println("Running database migrations...")
	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Migrations complete.")
}

func runCrawl(cfg *config.Config) {
	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	repo := database.NewRepository(pool)
	client := crawler.NewClient(cfg.BazaarAPIURL, cfg.BazaarPageSize, cfg.CrawlerMaxRetries)
	runner := crawler.NewRunner(client, repo)

	log.Printf("Starting crawl (concurrency=%d, pageSize=%d)",
		cfg.CrawlerConcurrency, cfg.BazaarPageSize)

	if err := runner.Run(ctx, cfg.CrawlerConcurrency); err != nil {
		log.Fatalf("Crawl failed: %v", err)
	}

	log.Println("Done.")
}
```

**Step 2: Verify it compiles**

Run: `go build ./cmd/agora/`
Expected: no errors (produces `agora.exe`)

**Step 3: Commit**

```bash
git add cmd/agora/main.go
git commit -m "feat: add CLI entrypoint with migrate and crawl commands"
```

---

### Task 11: End-to-End Verification

**Step 1: Set up cloud PostgreSQL**

1. Create a free Supabase or Neon PostgreSQL instance
2. Enable the `vector` and `uuid-ossp` extensions (Supabase has these pre-enabled)
3. Copy the connection string to `.env`:
   ```
   DATABASE_URL=postgres://user:pass@host:5432/postgres
   BAZAAR_API_URL=https://api.cdp.coinbase.com/platform/v2/x402/discovery/resources
   ```

**Step 2: Run migrations**

Run: `go run cmd/agora/main.go migrate`
Expected: "Migrations complete." — three tables created

**Step 3: Run a crawl**

Run: `go run cmd/agora/main.go crawl`
Expected:
- Logs show ~12,500+ endpoints fetched
- Progress updates every 500 items
- "Crawl complete" summary at the end
- Completes in under 5 minutes

**Step 4: Verify data in database**

Connect to PostgreSQL and run:
```sql
SELECT COUNT(*) FROM endpoints;
-- Expected: ~12,500+

SELECT COUNT(*) FROM payment_options;
-- Expected: ~13,000+ (some endpoints have multiple payment options)

SELECT COUNT(*) FROM crawl_runs WHERE status = 'completed';
-- Expected: 1

-- Spot check a well-structured endpoint
SELECT resource_url, domain, description, http_method
FROM endpoints
WHERE domain = 'weather.hugen.tokyo';

-- Check network distribution
SELECT network_normalized, COUNT(*)
FROM payment_options
GROUP BY network_normalized
ORDER BY COUNT(*) DESC;
```

**Step 5: Final commit**

```bash
git add .env.example
git commit -m "chore: verify end-to-end crawl works against live Bazaar API"
```
