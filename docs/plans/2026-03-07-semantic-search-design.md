# Semantic Search Design

**Date:** 2026-03-07
**Status:** Approved

## Overview

Add semantic search to AGORA so users and agents can query x402 endpoints with natural language (e.g. "crypto price feed API under $0.005") and get ranked results with similarity scores.

## Architecture

```
User/Agent                     Go API Server              PostgreSQL (pgvector)
    |                              |                            |
    |  POST /api/search            |                            |
    |  { query, filters, limit }   |                            |
    |----------------------------->|                            |
    |                              |  POST /embed               |
    |                              |----------->Python Sidecar  |
    |                              |<-----------(vector 384)    |
    |                              |                            |
    |                              |  SELECT ... ORDER BY       |
    |                              |  embedding <=> $query_vec  |
    |                              |  WHERE filters...          |
    |                              |  LIMIT $k                  |
    |                              |--------------------------->|
    |                              |<---------------------------|
    |  { results[], scores[] }     |                            |
    |<-----------------------------|                            |
```

Three components:

1. **Python sidecar** (`embed/server.py`) -- FastAPI, loads `all-MiniLM-L6-v2` at startup, serves `POST /embed`.
2. **Python batch script** (`embed/batch_embed.py`) -- Reads endpoints from DB, generates embeddings, writes them back. Run once initially, then after each crawl.
3. **Go API server** -- New `serve` command in `cmd/agora/main.go`. REST endpoints, calls sidecar for query embedding, runs pgvector search with SQL filters.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Embedding model | `all-MiniLM-L6-v2` (384 dims) | Free, local, good enough for short technical text |
| Sidecar protocol | HTTP (FastAPI) | Simple, one endpoint, negligible latency difference vs gRPC |
| Vector index | HNSW | Better recall than IVFFlat at 12K rows, no training step |
| HTTP framework (Go) | `net/http` + `http.ServeMux` | Go 1.22+ routing is sufficient for 3 endpoints |
| Search type | Semantic + SQL filters | Matches checkpoint promise of hybrid queries |
| Result format | Configurable K with similarity scores | Flexible for frontend and agent consumers |

## Database Changes

Migration to alter existing embedding column and add index:

```sql
ALTER TABLE endpoints ALTER COLUMN embedding TYPE vector(384);
CREATE INDEX idx_endpoints_embedding ON endpoints USING hnsw (embedding vector_cosine_ops);
```

Go model: add `Embedding pgvector.Vector` to `Endpoint` struct.

New repository methods:
- `SearchByVector(ctx, vector, filters, limit)` -- KNN search with optional WHERE clauses
- `UpdateEmbedding(ctx, endpointID, vector)` -- for batch script (direct DB, not through Go)

## Python Sidecar

**Location:** `embed/`

```
embed/
  server.py          # FastAPI app, POST /embed
  batch_embed.py     # Batch embedder for all endpoints
  requirements.txt   # sentence-transformers, fastapi, uvicorn, psycopg2-binary
```

**Sidecar API contract:**

```
POST /embed
Request:  { "text": "crypto price feed API" }
Response: { "embedding": [0.012, -0.034, ...] }  // 384 floats
```

**Batch script behavior:**
1. Connect to PostgreSQL (port 5433)
2. Query endpoints where `embedding IS NULL` (or all with `--force`)
3. Build enriched string per endpoint
4. Generate embedding, write back to DB
5. Log progress (~2-3 min for 12K endpoints locally)

Model is ~80MB, downloaded once on first run, cached by sentence-transformers.

## Enriched Text Construction

Each endpoint's embedding is generated from a concatenated string:

```
{http_method} {domain}{path} - {description}. Networks: {networks}. Assets: {assets}. Price: ${price}
```

Example:
```
GET api.example.com/v1/prices - Real-time cryptocurrency price feed API with historical data. Networks: base, ethereum. Assets: USDC. Price: $0.001
```

Rules:
- Networks/Assets: deduplicated from payment_options
- Price: minimum `price_usd` across payment_options
- Missing description: use `"{method} {domain}{path}"` alone
- No payment options: omit Networks/Assets/Price

Same logic in `batch_embed.py`. The Go API only embeds the user's raw query string.

## Go REST API

New CLI command: `./agora.exe serve`

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/search` | Semantic search with optional filters |
| GET | `/api/endpoints` | List/browse with pagination and filters |
| GET | `/api/endpoints/{id}` | Single endpoint with payment options |

**Search request/response:**

```json
// POST /api/search
{
  "query": "crypto price feed under a penny",
  "filters": {
    "network": "base",
    "method": "GET",
    "min_price": 0,
    "max_price": 0.01
  },
  "limit": 10
}

// Response
{
  "results": [
    {
      "endpoint": { "id": "...", "resource_url": "...", "description": "...", "..." },
      "payment_options": [ "..." ],
      "similarity": 0.87
    }
  ],
  "total": 10,
  "query_time_ms": 42
}
```

**Go package structure:**
- `internal/api/server.go` -- HTTP server setup, routes
- `internal/api/handlers.go` -- handler functions
- `internal/api/embed_client.go` -- HTTP client for Python sidecar

## Testing & Evaluation

**Unit tests:**
- Go: handler tests with mock sidecar, repository tests for vector search
- Python: test sidecar returns 384-dim vector

**Integration test:**
- Start sidecar + Go server + PostgreSQL
- Embed test endpoints, run search, verify ranking

**Evaluation (checkpoint target: 80% top-5 accuracy):**
- Curate 20-30 natural language queries with expected endpoint IDs
- Store in `eval/queries.json`
- Script reports accuracy: correct endpoint in top 5 results
- Build eval set after seeing real search results
