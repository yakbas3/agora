# Crawler & Database Design — Phase 1

**Date:** 2026-02-22
**Status:** Approved

## Overview

AGORA Phase 1: build an async crawler that paginates the Coinbase Bazaar discovery API, normalizes endpoint metadata, and stores it in a PostgreSQL + pgvector database. This is the data foundation for semantic search, keyword filtering, and reliability scoring in later phases.

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.22+ |
| Web Framework | chi (lightweight router) |
| Database Driver | pgx v5 (connection pooling, batch ops) |
| Migrations | golang-migrate |
| Vector Search | pgvector-go |
| HTTP Client | net/http stdlib |
| Config | envconfig (reads .env) |
| Embeddings | OpenAI go-openai (future) |

## Bazaar API Observations

Source: `GET https://api.cdp.coinbase.com/platform/v2/x402/discovery/resources`

- **No authentication required** (public API)
- **Pagination:** `limit` (max 100) + `offset` params, `pagination.total` in response
- **No server-side search or filtering** (only `type` filter)
- **12,573 endpoints** as of 2026-02-22
- **`metadata` field is always empty** `{}` — real data lives in `accepts[]` entries
- **Spam problem:** ~8,000+ endpoints from `lowpaymentfee.com` with generic "Premium API Access" descriptions
- **Network naming inconsistent:** "base", "eip155:8453", "base-sepolia" all appear

### Per-Item Structure

```
Top-level: resource, type, x402Version, lastUpdated, metadata (always {}), accepts[]
Per accepts[]: asset, description, extra, maxAmountRequired, maxTimeoutSeconds,
               mimeType, network, outputSchema, payTo, resource, scheme
```

### Two outputSchema Patterns

1. **Minimal:** `{ input: { discoverable: true, method: "GET", type: "http" } }`
2. **Rich:** `{ input: { method, queryParams/body/bodyFields, type }, output: { example, type } }`

## Database Schema

### `endpoints`

| Column | Type | Notes |
|---|---|---|
| id | UUID (PK) | |
| resource_url | TEXT (UNIQUE) | Dedup key |
| domain | TEXT | Extracted from resource_url |
| type | TEXT | Always "http" |
| x402_version | INTEGER | 1 or 2 |
| description | TEXT | Best description from accepts[] |
| http_method | TEXT | From outputSchema.input.method |
| input_schema | JSONB | Normalized input schema |
| output_schema | JSONB | Normalized output schema |
| raw_metadata | JSONB | Original metadata field |
| last_updated | TIMESTAMPTZ | From Bazaar lastUpdated |
| first_seen | TIMESTAMPTZ | First crawl time |
| last_crawled | TIMESTAMPTZ | Last crawl touch |
| embedding | vector(1536) | For semantic search (future) |

### `payment_options`

| Column | Type | Notes |
|---|---|---|
| id | UUID (PK) | |
| endpoint_id | UUID (FK) | → endpoints |
| scheme | TEXT | "exact" |
| network_raw | TEXT | Raw: "base", "eip155:8453", etc. |
| network_normalized | TEXT | Cleaned: "base", "base-sepolia", "solana" |
| asset_address | TEXT | Contract address |
| asset_name | TEXT | From extra.name |
| max_amount_raw | TEXT | Raw string from API |
| price_usd | NUMERIC | Computed: raw / 10^decimals |
| pay_to | TEXT | Wallet address |
| max_timeout_seconds | INTEGER | |
| mime_type | TEXT | |
| description | TEXT | Per-option description |
| output_schema_raw | JSONB | Raw outputSchema |

### `crawl_runs`

| Column | Type | Notes |
|---|---|---|
| id | UUID (PK) | |
| started_at | TIMESTAMPTZ | |
| completed_at | TIMESTAMPTZ | nullable |
| total_fetched | INTEGER | |
| new_endpoints | INTEGER | |
| updated_endpoints | INTEGER | |
| status | TEXT | running, completed, failed |
| error | TEXT | nullable |

### Indexes

- `endpoints.resource_url` — unique, dedup
- `endpoints.domain` — domain filtering
- `endpoints.embedding` — HNSW for vector search (future)
- `payment_options.endpoint_id` — FK join
- `payment_options.network_normalized` — network filtering
- `payment_options.price_usd` — price range filtering

## Crawler Design

### Strategy

1. Full crawl: paginate offset=0 to total, limit=100 per page (~126 pages)
2. Concurrency: 3-5 goroutines via errgroup
3. Dedup: upsert on resource_url, compare lastUpdated to skip unchanged
4. Network normalization: eip155:8453 → base, eip155:84532 → base-sepolia, etc.
5. Description extraction: pick longest accepts[].description for endpoint-level
6. Price computation: USDC has 6 decimals, maxAmountRequired / 10^6 = USD

### Flow

```
Start crawl run → INSERT into crawl_runs (status=running)
  │
  for offset in range(0, total, 100):
  │   GET /discovery/resources?limit=100&offset={offset}
  │   │
  │   for each item:
  │   │   normalize fields
  │   │   UPSERT into endpoints (ON CONFLICT resource_url)
  │   │   DELETE old payment_options for this endpoint
  │   │   INSERT new payment_options
  │   │
  │   exponential backoff on HTTP errors
  │
  Update crawl_run with stats → status=completed
```

### Error Handling

- HTTP 429/5xx → exponential backoff (1s, 2s, 4s, max 30s), retry up to 5 times
- Individual item parse errors → log and skip
- Network errors → retry with backoff
- Crawl marked "failed" if total fetched < 50% of expected

## Project Structure

```
agora/
├── go.mod
├── go.sum
├── .env.example
├── cmd/
│   └── agora/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── database/
│   │   └── db.go
│   ├── models/
│   │   ├── endpoint.go
│   │   ├── payment_option.go
│   │   └── crawl_run.go
│   ├── crawler/
│   │   ├── client.go
│   │   ├── normalizer.go
│   │   ├── runner.go
│   │   └── network.go
│   └── api/
│       └── router.go
├── migrations/
│   ├── 001_create_endpoints.up.sql
│   ├── 001_create_endpoints.down.sql
│   └── ...
├── tests/
├── docs/
│   └── plans/
└── README.md
```

## Scope

### In Scope (Phase 1)

- PostgreSQL schema with endpoints, payment_options, crawl_runs
- SQL migrations via golang-migrate
- Async crawler with goroutines + errgroup
- Data normalization (network, price, description)
- Upsert with dedup on resource_url
- Crawl run tracking
- CLI trigger: `go run cmd/agora/main.go crawl`
- Unit tests for normalizer and crawler

### Not In Scope (Future)

- Embeddings / vector search (Phase 2)
- Keyword/filter search API (Phase 3)
- Health checks / reliability scoring (Phase 4)
- REST API endpoints (Phase 3-4)
- Multi-facilitator support (Phase 5)
- Scheduled crawling

## Success Criteria

- Crawl all ~12,573 endpoints successfully
- Store in normalized schema with correct dedup
- Handle spam endpoints (stored but flagged)
- Crawl completes in under 5 minutes
- Tests pass
