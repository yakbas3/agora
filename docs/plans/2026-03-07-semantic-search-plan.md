# Semantic Search Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add embedding-based semantic search over 12,571 x402 endpoints using a free local model (`all-MiniLM-L6-v2`), a Python FastAPI sidecar, and a Go REST API with pgvector KNN search and SQL filters.

**Architecture:** Python sidecar serves a single `/embed` endpoint to convert text to 384-dim vectors. A batch Python script pre-computes embeddings for all endpoints. Go API server accepts search queries, calls the sidecar, and runs pgvector cosine similarity search with optional filters (network, method, price range). Results include similarity scores with configurable K.

**Tech Stack:** Go 1.24, Python 3.12, FastAPI, sentence-transformers, pgvector, PostgreSQL 16, `net/http` (Go 1.22+ ServeMux)

**Design doc:** `docs/plans/2026-03-07-semantic-search-design.md`

---

### Task 1: Database Migration -- Alter Embedding Column and Add Index

**Files:**
- Create: `migrations/000008_alter_embedding_dimension.up.sql`
- Create: `migrations/000008_alter_embedding_dimension.down.sql`

**Step 1: Write the up migration**

```sql
-- Alter embedding column from vector(1536) to vector(384) for all-MiniLM-L6-v2
ALTER TABLE endpoints ALTER COLUMN embedding TYPE vector(384);

-- Add HNSW index for fast cosine similarity search
CREATE INDEX idx_endpoints_embedding ON endpoints USING hnsw (embedding vector_cosine_ops);
```

**Step 2: Write the down migration**

```sql
DROP INDEX IF EXISTS idx_endpoints_embedding;
ALTER TABLE endpoints ALTER COLUMN embedding TYPE vector(1536);
```

**Step 3: Run the migration**

Run: `./agora.exe migrate`
Expected: Migration 000008 applied successfully.

**Step 4: Verify the column change**

Run:
```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "\d endpoints" | grep embedding
```
Expected: `embedding | vector(384)`

**Step 5: Commit**

```bash
git add migrations/000008_alter_embedding_dimension.up.sql migrations/000008_alter_embedding_dimension.down.sql
git commit -m "feat: migrate embedding column to vector(384) with HNSW index"
```

---

### Task 2: Python Sidecar -- Embedding Server

**Files:**
- Create: `embed/server.py`
- Create: `embed/requirements.txt`

**Step 1: Write requirements.txt**

```
sentence-transformers==4.1.0
fastapi==0.115.0
uvicorn==0.34.0
```

**Step 2: Write the sidecar server**

```python
from contextlib import asynccontextmanager
from fastapi import FastAPI
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer

model = None

@asynccontextmanager
async def lifespan(app: FastAPI):
    global model
    model = SentenceTransformer("all-MiniLM-L6-v2")
    yield

app = FastAPI(lifespan=lifespan)

class EmbedRequest(BaseModel):
    text: str

class EmbedResponse(BaseModel):
    embedding: list[float]

@app.post("/embed", response_model=EmbedResponse)
def embed(req: EmbedRequest):
    vec = model.encode(req.text).tolist()
    return EmbedResponse(embedding=vec)

@app.get("/health")
def health():
    return {"status": "ok", "model": "all-MiniLM-L6-v2", "dimensions": 384}
```

**Step 3: Install dependencies and start the server**

Run:
```bash
cd embed && pip install -r requirements.txt
```

Then:
```bash
python -m uvicorn server:app --host 0.0.0.0 --port 8100
```

Expected: Server starts, first request downloads model (~80MB), subsequent requests are instant.

**Step 4: Test the sidecar manually**

Run:
```bash
curl -X POST http://localhost:8100/embed -H "Content-Type: application/json" -d '{"text": "crypto price feed API"}'
```

Expected: JSON response with `embedding` array of 384 floats.

**Step 5: Commit**

```bash
git add embed/server.py embed/requirements.txt
git commit -m "feat: add Python embedding sidecar with all-MiniLM-L6-v2"
```

---

### Task 3: Python Batch Embedder

**Files:**
- Create: `embed/batch_embed.py`

**Step 1: Write the batch embedding script**

```python
import argparse
import psycopg2
from sentence_transformers import SentenceTransformer

DB_URL = "postgresql://agora:agora@localhost:5433/agora"
BATCH_SIZE = 256

def build_text(row):
    method, domain, resource_url, description, networks, assets, price = row
    parts = []
    if method:
        parts.append(method)
    parts.append(resource_url or domain)
    if description:
        parts.append(f"- {description}")
    if networks:
        parts.append(f"Networks: {networks}")
    if assets:
        parts.append(f"Assets: {assets}")
    if price is not None:
        parts.append(f"Price: ${price:.6f}")
    return " ".join(parts)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--force", action="store_true", help="Re-embed all endpoints, not just nulls")
    args = parser.parse_args()

    model = SentenceTransformer("all-MiniLM-L6-v2")
    conn = psycopg2.connect(DB_URL)

    where = "" if args.force else "WHERE e.embedding IS NULL"
    query = f"""
        SELECT e.id, e.http_method, e.domain, e.resource_url, e.description,
               STRING_AGG(DISTINCT po.network_normalized, ', ') AS networks,
               STRING_AGG(DISTINCT po.asset_name, ', ') AS assets,
               MIN(po.price_usd) AS price
        FROM endpoints e
        LEFT JOIN payment_options po ON po.endpoint_id = e.id
        {where}
        GROUP BY e.id
    """

    cur = conn.cursor()
    cur.execute(query)
    rows = cur.fetchall()
    print(f"Found {len(rows)} endpoints to embed")

    for i in range(0, len(rows), BATCH_SIZE):
        batch = rows[i:i + BATCH_SIZE]
        texts = [build_text(r[1:]) for r in batch]
        embeddings = model.encode(texts)

        for row, emb in zip(batch, embeddings):
            eid = row[0]
            vec = emb.tolist()
            cur.execute("UPDATE endpoints SET embedding = %s::vector WHERE id = %s", (str(vec), eid))

        conn.commit()
        print(f"  Embedded {min(i + BATCH_SIZE, len(rows))}/{len(rows)}")

    cur.close()
    conn.close()
    print("Done!")

if __name__ == "__main__":
    main()
```

**Step 2: Run the batch embedder**

Run (with sidecar NOT needed -- this script loads the model directly):
```bash
cd embed && python batch_embed.py
```

Expected: Embeds all 12,571 endpoints in ~2-3 minutes, prints progress.

**Step 3: Verify embeddings in database**

Run:
```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT count(*) FROM endpoints WHERE embedding IS NOT NULL;"
```

Expected: `12571` (or current endpoint count).

**Step 4: Commit**

```bash
git add embed/batch_embed.py
git commit -m "feat: add batch embedding script for endpoint corpus"
```

---

### Task 4: Go Model -- Add Embedding Field

**Files:**
- Modify: `internal/models/endpoint.go` (add Embedding field after LastCrawled, ~line 23)

**Step 1: Add the Embedding field to the Endpoint struct**

Add this import and field:

```go
// Add to imports:
import "github.com/pgvector/pgvector-go"

// Add field after LastCrawled:
Embedding  pgvector.Vector `db:"embedding"`
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors.

**Step 3: Commit**

```bash
git add internal/models/endpoint.go
git commit -m "feat: add Embedding field to Endpoint model"
```

---

### Task 5: Repository -- Add Search Method

**Files:**
- Modify: `internal/database/repository.go` (add methods after existing ones, ~line 276)

**Step 1: Write the test**

Create: `internal/database/repository_search_test.go`

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/database/... -run TestBuildSearchQuery -v`
Expected: FAIL -- `buildSearchQuery` not defined.

**Step 3: Write the search implementation**

Add to `internal/database/repository.go`:

```go
// SearchFilters holds optional filters for semantic search.
type SearchFilters struct {
	Network  string
	Method   string
	MinPrice *float64
	MaxPrice *float64
}

// SearchResult holds an endpoint with its similarity score.
type SearchResult struct {
	Endpoint   models.Endpoint
	Similarity float64
}

func buildSearchQuery(filters SearchFilters, limit int) (string, []any) {
	// $1 is always the query vector, filled by caller
	args := []any{nil} // placeholder for vector
	argIdx := 2

	joins := ""
	where := "WHERE e.embedding IS NOT NULL"

	if filters.Network != "" {
		joins = "JOIN payment_options po ON po.endpoint_id = e.id"
		where += fmt.Sprintf(" AND po.network_normalized = $%d", argIdx)
		args = append(args, filters.Network)
		argIdx++
	}

	if filters.Method != "" {
		where += fmt.Sprintf(" AND e.http_method = $%d", argIdx)
		args = append(args, filters.Method)
		argIdx++
	}

	needsPO := filters.MinPrice != nil || filters.MaxPrice != nil
	if needsPO && joins == "" {
		joins = "JOIN payment_options po ON po.endpoint_id = e.id"
	}

	if filters.MinPrice != nil {
		where += fmt.Sprintf(" AND po.price_usd >= $%d", argIdx)
		args = append(args, *filters.MinPrice)
		argIdx++
	}

	if filters.MaxPrice != nil {
		where += fmt.Sprintf(" AND po.price_usd <= $%d", argIdx)
		args = append(args, *filters.MaxPrice)
		argIdx++
	}

	q := fmt.Sprintf(`
		SELECT DISTINCT ON (e.id)
			e.id, e.resource_url, e.domain, e.type, e.x402_version,
			e.description, e.http_method, e.input_schema, e.output_schema,
			e.raw_metadata, e.last_updated, e.first_seen, e.last_crawled,
			1 - (e.embedding <=> $1) AS similarity
		FROM endpoints e
		%s
		%s
		ORDER BY e.id, similarity DESC
	`, joins, where)

	// Wrap to re-order by similarity and apply limit
	q = fmt.Sprintf(`SELECT * FROM (%s) sub ORDER BY similarity DESC LIMIT $%d`, q, argIdx)
	args = append(args, limit)

	return q, args
}

// SearchByVector finds the most similar endpoints to the given vector.
func (r *Repository) SearchByVector(ctx context.Context, vector pgvector.Vector, filters SearchFilters, limit int) ([]SearchResult, error) {
	q, args := buildSearchQuery(filters, limit)
	args[0] = vector // fill the vector placeholder

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var sr SearchResult
		err := rows.Scan(
			&sr.Endpoint.ID, &sr.Endpoint.ResourceURL, &sr.Endpoint.Domain,
			&sr.Endpoint.Type, &sr.Endpoint.X402Version, &sr.Endpoint.Description,
			&sr.Endpoint.HTTPMethod, &sr.Endpoint.InputSchema, &sr.Endpoint.OutputSchema,
			&sr.Endpoint.RawMetadata, &sr.Endpoint.LastUpdated, &sr.Endpoint.FirstSeen,
			&sr.Endpoint.LastCrawled, &sr.Similarity,
		)
		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, sr)
	}
	return results, rows.Err()
}
```

Add these imports to `repository.go` if not already present: `"fmt"`, `"github.com/pgvector/pgvector-go"`, `"github.com/yamanakbas/agora/internal/models"`.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/database/... -run TestBuildSearchQuery -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/database/repository.go internal/database/repository_search_test.go
git commit -m "feat: add SearchByVector repository method with filter support"
```

---

### Task 6: Repository -- Add Endpoint Listing Methods

**Files:**
- Modify: `internal/database/repository.go`

**Step 1: Add GetEndpoints and GetEndpointByID methods**

```go
// GetEndpoints returns a paginated list of endpoints.
func (r *Repository) GetEndpoints(ctx context.Context, limit, offset int) ([]models.Endpoint, error) {
	q := `
		SELECT id, resource_url, domain, type, x402_version, description,
		       http_method, input_schema, output_schema, raw_metadata,
		       last_updated, first_seen, last_crawled
		FROM endpoints
		ORDER BY last_crawled DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []models.Endpoint
	for rows.Next() {
		var e models.Endpoint
		err := rows.Scan(
			&e.ID, &e.ResourceURL, &e.Domain, &e.Type, &e.X402Version,
			&e.Description, &e.HTTPMethod, &e.InputSchema, &e.OutputSchema,
			&e.RawMetadata, &e.LastUpdated, &e.FirstSeen, &e.LastCrawled,
		)
		if err != nil {
			return nil, fmt.Errorf("scan endpoint: %w", err)
		}
		endpoints = append(endpoints, e)
	}
	return endpoints, rows.Err()
}

// GetEndpointByID returns a single endpoint with its payment options.
func (r *Repository) GetEndpointByID(ctx context.Context, id uuid.UUID) (*models.Endpoint, []models.PaymentOption, error) {
	eq := `
		SELECT id, resource_url, domain, type, x402_version, description,
		       http_method, input_schema, output_schema, raw_metadata,
		       last_updated, first_seen, last_crawled
		FROM endpoints WHERE id = $1
	`
	var e models.Endpoint
	err := r.pool.QueryRow(ctx, eq, id).Scan(
		&e.ID, &e.ResourceURL, &e.Domain, &e.Type, &e.X402Version,
		&e.Description, &e.HTTPMethod, &e.InputSchema, &e.OutputSchema,
		&e.RawMetadata, &e.LastUpdated, &e.FirstSeen, &e.LastCrawled,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get endpoint: %w", err)
	}

	pq := `
		SELECT id, endpoint_id, scheme, network_raw, network_normalized,
		       asset_address, asset_name, max_amount_raw, price_usd,
		       pay_to, max_timeout_seconds, mime_type, description, output_schema
		FROM payment_options WHERE endpoint_id = $1
	`
	rows, err := r.pool.Query(ctx, pq, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get payment options: %w", err)
	}
	defer rows.Close()

	var pos []models.PaymentOption
	for rows.Next() {
		var po models.PaymentOption
		err := rows.Scan(
			&po.ID, &po.EndpointID, &po.Scheme, &po.NetworkRaw, &po.NetworkNormalized,
			&po.AssetAddress, &po.AssetName, &po.MaxAmountRaw, &po.PriceUSD,
			&po.PayTo, &po.MaxTimeoutSeconds, &po.MimeType, &po.Description, &po.OutputSchemaRaw,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("scan payment option: %w", err)
		}
		pos = append(pos, po)
	}
	return &e, pos, rows.Err()
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors.

**Step 3: Commit**

```bash
git add internal/database/repository.go
git commit -m "feat: add GetEndpoints and GetEndpointByID repository methods"
```

---

### Task 7: Embed Client -- Go HTTP Client for Python Sidecar

**Files:**
- Create: `internal/api/embed_client.go`
- Create: `internal/api/embed_client_test.go`

**Step 1: Write the test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/api/... -run TestEmbedClient -v`
Expected: FAIL -- `NewEmbedClient` not defined.

**Step 3: Write the embed client**

```go
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type EmbedClient struct {
	baseURL    string
	httpClient *http.Client
}

type embedRequest struct {
	Text string `json:"text"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func NewEmbedClient(baseURL string) *EmbedClient {
	return &EmbedClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

func (c *EmbedClient) Embed(text string) ([]float32, error) {
	body, err := json.Marshal(embedRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/embed", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed request returned status %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	return result.Embedding, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/api/... -run TestEmbedClient -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/api/embed_client.go internal/api/embed_client_test.go
git commit -m "feat: add Go HTTP client for Python embedding sidecar"
```

---

### Task 8: API Handlers

**Files:**
- Create: `internal/api/handlers.go`
- Create: `internal/api/handlers_test.go`

**Step 1: Write handler tests**

```go
package api

import (
	"bytes"
	"encoding/json"
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/... -run TestHandle -v`
Expected: FAIL -- `Handlers` not defined.

**Step 3: Write the handlers**

```go
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"github.com/yamanakbas/agora/internal/database"
)

type Handlers struct {
	repo        *database.Repository
	embedClient *EmbedClient
}

func NewHandlers(repo *database.Repository, embedClient *EmbedClient) *Handlers {
	return &Handlers{repo: repo, embedClient: embedClient}
}

type SearchRequest struct {
	Query   string        `json:"query"`
	Filters SearchFilters `json:"filters"`
	Limit   int           `json:"limit"`
}

type SearchFilters struct {
	Network  string   `json:"network"`
	Method   string   `json:"method"`
	MinPrice *float64 `json:"min_price"`
	MaxPrice *float64 `json:"max_price"`
}

type SearchResponse struct {
	Results     []SearchResultJSON `json:"results"`
	Total       int                `json:"total"`
	QueryTimeMs int64              `json:"query_time_ms"`
}

type SearchResultJSON struct {
	Endpoint       EndpointJSON `json:"endpoint"`
	Similarity     float64      `json:"similarity"`
}

type EndpointJSON struct {
	ID           string          `json:"id"`
	ResourceURL  string          `json:"resource_url"`
	Domain       string          `json:"domain"`
	Type         string          `json:"type"`
	X402Version  int             `json:"x402_version"`
	Description  string          `json:"description"`
	HTTPMethod   string          `json:"http_method"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
}

func (h *Handlers) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}

	start := time.Now()

	vec, err := h.embedClient.Embed(req.Query)
	if err != nil {
		log.Printf("embedding error: %v", err)
		http.Error(w, "embedding service unavailable", http.StatusServiceUnavailable)
		return
	}

	dbFilters := database.SearchFilters{
		Network:  req.Filters.Network,
		Method:   req.Filters.Method,
		MinPrice: req.Filters.MinPrice,
		MaxPrice: req.Filters.MaxPrice,
	}

	results, err := h.repo.SearchByVector(r.Context(), pgvector.NewVector(vec), dbFilters, req.Limit)
	if err != nil {
		log.Printf("search error: %v", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	resp := SearchResponse{
		Total:       len(results),
		QueryTimeMs: time.Since(start).Milliseconds(),
	}
	for _, sr := range results {
		resp.Results = append(resp.Results, SearchResultJSON{
			Endpoint: EndpointJSON{
				ID:           sr.Endpoint.ID.String(),
				ResourceURL:  sr.Endpoint.ResourceURL,
				Domain:       sr.Endpoint.Domain,
				Type:         sr.Endpoint.Type,
				X402Version:  sr.Endpoint.X402Version,
				Description:  sr.Endpoint.Description,
				HTTPMethod:   sr.Endpoint.HTTPMethod,
				InputSchema:  sr.Endpoint.InputSchema,
				OutputSchema: sr.Endpoint.OutputSchema,
			},
			Similarity: sr.Similarity,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) handleEndpoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	endpoints, err := h.repo.GetEndpoints(r.Context(), limit, offset)
	if err != nil {
		log.Printf("get endpoints error: %v", err)
		http.Error(w, "failed to get endpoints", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}

func (h *Handlers) handleEndpointByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid endpoint ID", http.StatusBadRequest)
		return
	}

	endpoint, paymentOptions, err := h.repo.GetEndpointByID(r.Context(), id)
	if err != nil {
		log.Printf("get endpoint error: %v", err)
		http.Error(w, "endpoint not found", http.StatusNotFound)
		return
	}

	resp := map[string]any{
		"endpoint":        endpoint,
		"payment_options": paymentOptions,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/api/... -run TestHandle -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add REST API handlers for search, list, and detail endpoints"
```

---

### Task 9: API Server and CLI Command

**Files:**
- Create: `internal/api/server.go`
- Modify: `cmd/agora/main.go` (add `serve` command, ~line 34 in the switch)
- Modify: `internal/config/config.go` (add `EmbedURL` field)

**Step 1: Add config field**

Add to `internal/config/config.go` Config struct:

```go
EmbedURL string `envconfig:"EMBED_URL" default:"http://localhost:8100"`
APIPort  string `envconfig:"API_PORT" default:"8080"`
```

**Step 2: Write the server**

```go
package api

import (
	"log"
	"net/http"

	"github.com/yamanakbas/agora/internal/database"
)

type Server struct {
	handlers *Handlers
	port     string
}

func NewServer(repo *database.Repository, embedURL string, port string) *Server {
	return &Server{
		handlers: NewHandlers(repo, NewEmbedClient(embedURL)),
		port:     port,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/search", s.handlers.handleSearch)
	mux.HandleFunc("GET /api/endpoints", s.handlers.handleEndpoints)
	mux.HandleFunc("GET /api/endpoints/{id}", s.handlers.handleEndpointByID)

	log.Printf("API server starting on :%s", s.port)
	return http.ListenAndServe(":"+s.port, mux)
}
```

**Step 3: Add the `serve` command to main.go**

Add a new case in the switch statement in `cmd/agora/main.go` (after the `index` case):

```go
case "serve":
	runServe()
```

Add the `runServe` function:

```go
func runServe() {
	cfg := config.MustLoad()
	pool := database.MustNewPool(cfg.DatabaseURL)
	defer pool.Close()

	repo := database.NewRepository(pool)
	srv := api.NewServer(repo, cfg.EmbedURL, cfg.APIPort)

	log.Printf("Embed sidecar URL: %s", cfg.EmbedURL)
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

Add import: `"github.com/yamanakbas/agora/internal/api"`

**Step 4: Verify it compiles**

Run: `go build -o agora.exe ./cmd/agora`
Expected: No errors.

**Step 5: Commit**

```bash
git add internal/api/server.go internal/config/config.go cmd/agora/main.go
git commit -m "feat: add serve command with REST API server"
```

---

### Task 10: Update .env.example and Add CORS

**Files:**
- Modify: `.env.example` (add EMBED_URL and API_PORT)
- Modify: `internal/api/server.go` (add CORS middleware for frontend)

**Step 1: Add env vars to .env.example**

Append:

```
# Embedding Sidecar
EMBED_URL=http://localhost:8100

# API Server
API_PORT=8080
```

**Step 2: Add CORS middleware to server.go**

Wrap the mux in a CORS handler so the Next.js frontend (port 3000) can call the API:

```go
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

Update `Start()` to use it:

```go
return http.ListenAndServe(":"+s.port, corsMiddleware(mux))
```

**Step 3: Verify it compiles**

Run: `go build -o agora.exe ./cmd/agora`
Expected: No errors.

**Step 4: Commit**

```bash
git add .env.example internal/api/server.go
git commit -m "feat: add CORS middleware and env config for API server"
```

---

### Task 11: End-to-End Integration Test

**Files:**
- None created -- this is a manual verification task.

**Step 1: Start all services**

Terminal 1 (Docker should already be running):
```bash
docker compose up -d
```

Terminal 2 (Python sidecar):
```bash
cd embed && python -m uvicorn server:app --host 0.0.0.0 --port 8100
```

Terminal 3 (Go API):
```bash
./agora.exe serve
```

**Step 2: Run the batch embedder (if not done already)**

```bash
cd embed && python batch_embed.py
```

**Step 3: Test semantic search**

```bash
curl -s -X POST http://localhost:8080/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "crypto price feed API", "limit": 5}' | python -m json.tool
```

Expected: 5 results with similarity scores, ordered by relevance.

**Step 4: Test with filters**

```bash
curl -s -X POST http://localhost:8080/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "image generation", "filters": {"network": "base"}, "limit": 3}' | python -m json.tool
```

Expected: 3 results filtered to base network.

**Step 5: Test list and detail endpoints**

```bash
curl -s "http://localhost:8080/api/endpoints?limit=2" | python -m json.tool
```

Then pick an ID from the result and:
```bash
curl -s "http://localhost:8080/api/endpoints/{id}" | python -m json.tool
```

Expected: Endpoint details with payment options.

**Step 6: Commit any fixes if needed**

---

### Task 12: Run All Tests

**Step 1: Run the full test suite**

Run: `go test ./... -v`
Expected: All tests pass.

**Step 2: Final commit if any fixes were needed**

```bash
git add -A
git commit -m "fix: address test failures from semantic search integration"
```

---

## Summary

| Task | What | Files |
|------|------|-------|
| 1 | DB migration (vector 384 + HNSW index) | `migrations/000008_*` |
| 2 | Python sidecar (FastAPI + all-MiniLM-L6-v2) | `embed/server.py`, `embed/requirements.txt` |
| 3 | Python batch embedder | `embed/batch_embed.py` |
| 4 | Go model (add Embedding field) | `internal/models/endpoint.go` |
| 5 | Repository search method | `internal/database/repository.go`, `*_test.go` |
| 6 | Repository list/detail methods | `internal/database/repository.go` |
| 7 | Go embed client (HTTP → sidecar) | `internal/api/embed_client.go`, `*_test.go` |
| 8 | API handlers (search, list, detail) | `internal/api/handlers.go`, `*_test.go` |
| 9 | API server + CLI `serve` command | `internal/api/server.go`, `cmd/agora/main.go`, `config.go` |
| 10 | CORS + env config | `internal/api/server.go`, `.env.example` |
| 11 | E2E integration test | manual |
| 12 | Run all tests | verification |
