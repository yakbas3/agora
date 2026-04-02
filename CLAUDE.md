CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Agora is a Go-based crawler and search engine for the x402 protocol ecosystem. It crawls the Coinbase Bazaar discovery API, indexes on-chain USDC payment transactions via the CDP SQL API, probes endpoints for x402 health compliance, computes reliability scores, and exposes a REST API with semantic search (pgvector). A Next.js frontend provides a dashboard UI.

Course project for CS 4365/6365 at Georgia Tech, Spring 2026.

## Commands

```bash
# Start PostgreSQL (pgvector on port 5433, NOT 5432)
docker compose up -d

# Production stack (postgres + embed sidecar + API + web frontend)
docker compose -f docker-compose.prod.yml up -d

# Build Go binary
go build -o agora.exe ./cmd/agora

# CLI commands
./agora.exe migrate    # Run database migrations
./agora.exe crawl      # Crawl Bazaar API → endpoints + payment options
./agora.exe sync       # Sync V1 transactions from CDP SQL API
./agora.exe index      # Index V2 on-chain transactions (Alchemy RPC)
./agora.exe probe      # Probe all endpoints for x402 health + compliance (~15-30 min)
./agora.exe serve      # Start REST API server (default :8080)

# Tests
go test ./...                                          # All tests
go test ./internal/crawler/...                         # Single package
go test ./internal/crawler/... -run TestNormalizeNetwork  # Single test

# Web frontend (Next.js + Tailwind)
cd web && npm install && npm run dev    # Dev server on :3000
cd web && npm run build                 # Production build
cd web && npm run lint                  # ESLint

# Embed sidecar (Python, sentence-transformers)
cd embed && pip install -r requirements.txt
cd embed && uvicorn server:app --port 8100
```

## Architecture

The system has four main components:

**Go CLI** (`cmd/agora/main.go`) — Dispatches to six subcommands: `migrate`, `crawl`, `sync`, `index`, `probe`, `serve`.

**Data pipeline** — Four independent ingestion paths, all writing to the same PostgreSQL database:
1. **Bazaar Crawler** (`internal/crawler/`) — Paginates the Coinbase Bazaar discovery API with exponential backoff, normalizes responses, upserts endpoints + payment options. Rate-limited: 1 req/2s, page size 1000.
2. **CDP Sync** (`internal/sync/` + `internal/cdp/`) — Queries the CDP SQL API for USDC Transfer events by known facilitator addresses. Chunks by month (falls back to weekly if scan limits exceeded). Updates facilitator sync timestamps and refreshes materialized views (`endpoint_scores`, `discovered_sellers`).
3. **V2 Indexer** (`internal/indexer/`) — Queries Base via Alchemy RPC for `Settled`/`SettledWithPermit` events. Finds near-zero transactions (V2 barely used). Keep code, don't delete.
4. **Health Prober** (`internal/prober/`) — Makes unauthenticated HTTP requests to all 12,571 endpoints to check x402 compliance. Records HTTP status, latency, 402 response validity, and price match. Runs concurrently (20 workers) with per-domain rate limiting (200ms). Results stored in `probe_results`, blended into `endpoint_scores` materialized view.

**REST API** (`internal/api/`) — net/http server with CORS middleware. Endpoints:
- `POST /api/search` — Semantic search via embedding sidecar → pgvector cosine similarity
- `GET /api/endpoints`, `GET /api/endpoints/{id}`, `GET /api/stats`
- `GET /api/facilitators`, `GET /api/transactions`
- `GET /api/endpoints/{id}/probes` — Probe history for a specific endpoint (last 10 results)

**Embed Sidecar** (`embed/`) — FastAPI Python service using `all-MiniLM-L6-v2` (384-dim vectors). The Go API calls `POST /embed` to vectorize search queries.

**Web Frontend** (`web/`) — Next.js 16 + React 19 + Tailwind 4 + Recharts. Talks to the Go API.

### Key Internal Packages

- `internal/config/` — envconfig + godotenv. All config via env vars (see `.env.example`).
- `internal/models/` — Domain structs: Endpoint, PaymentOption, CrawlRun, Transaction, Facilitator, ProbeResult.
- `internal/database/` — pgxpool connection with pgvector type registration, golang-migrate runner, repository with transaction-based upserts and vector search.
- `internal/prober/` — x402 health check prober: Client (HTTP probe + 402 parser + price comparison), Runner (batched concurrency + per-domain rate limiting), types.
- `migrations/` — SQL files + `embed.go` that exports `var FS embed.FS`. This pattern exists because Go's `//go:embed` doesn't allow `../` paths, so the embed lives in the migrations directory itself.

## Environment Variables

All config is loaded from `.env` (gitignored) via godotenv, then parsed by envconfig. Copy `.env.example` to `.env`.

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | `postgres://agora:agora@localhost:5433/agora?sslmode=disable` |
| `BAZAAR_API_URL` | For crawl | — | Coinbase Bazaar discovery API URL |
| `BAZAAR_PAGE_SIZE` | No | 100 | Use 1000 (max allowed) in production |
| `CRAWLER_CONCURRENCY` | No | 3 | Use 1 — Bazaar rate limits are aggressive |
| `CRAWLER_MAX_RETRIES` | No | 5 | Use 10 for reliability |
| `BASE_RPC_URL` | For index | — | Alchemy RPC URL for Base mainnet |
| `INDEXER_BLOCK_RANGE` | No | 10 | Blocks per query window |
| `INDEXER_START_BLOCK` | No | 25000000 | Where to start scanning |
| `EMBED_URL` | No | `http://localhost:8100` | Embed sidecar URL |
| `API_PORT` | No | 8080 | REST API listen port |
| `CDP_API_KEY_ID` | For sync | — | Coinbase Developer Platform key ID |
| `CDP_API_KEY_SECRET` | For sync | — | CDP Ed25519 private key |
| `PROBER_CONCURRENCY` | No | 20 | Parallel probe requests |
| `PROBER_TIMEOUT_SECS` | No | 10 | Per-request HTTP timeout |
| `PROBER_BATCH_SIZE` | No | 500 | Endpoints loaded from DB per batch |
| `PROBER_DOMAIN_DELAY_MS` | No | 200 | Min ms between requests to same domain |

## Database Schema

PostgreSQL with pgvector extension. All migrations in `migrations/` using golang-migrate.

### Core Tables

**`endpoints`** — Crawled x402 service endpoints (12,571 rows in seed data)
- `id` UUID PK, `resource_url` TEXT UNIQUE, `domain` TEXT (indexed), `type` TEXT, `x402_version` INT
- `description` TEXT, `http_method` TEXT (indexed), `input_schema` JSONB, `output_schema` JSONB
- `raw_metadata` JSONB, `last_updated` TIMESTAMPTZ, `first_seen` TIMESTAMPTZ, `last_crawled` TIMESTAMPTZ
- `embedding` vector(384) — HNSW index for cosine similarity search

**`payment_options`** — Payment methods per endpoint (12,803 rows)
- `endpoint_id` UUID FK→endpoints (CASCADE), `scheme` TEXT, `network_raw`/`network_normalized` TEXT
- `asset_address` TEXT, `asset_name` TEXT, `max_amount_raw` TEXT, `price_usd` NUMERIC(20,10) (indexed)
- `pay_to` TEXT (recipient wallet), `max_timeout_seconds` INT, `mime_type` TEXT

**`transactions`** — Indexed on-chain USDC payments (148,106 rows)
- `tx_hash` TEXT UNIQUE, `block_number` BIGINT (indexed), `block_time` TIMESTAMPTZ (indexed)
- `event_type` TEXT (`Transfer`, `Settled`, `SettledWithPermit`)
- `facilitator_address` TEXT (indexed), `payer_address` TEXT, `recipient_address` TEXT (indexed)
- `amount_raw` TEXT, `amount_usd` NUMERIC(20,10), `asset_address` TEXT (always USDC on Base)

**`facilitators`** — Known x402 facilitator addresses (102 rows, seeded in migration 000010)
- `name` TEXT, `chain` TEXT (all `base`), `address` TEXT, `last_synced_at` TIMESTAMPTZ
- Unique index on `(chain, lower(address))` — address matching is case-insensitive

**`crawl_runs`** — Crawl execution history
- `status` TEXT (`running`, `completed`, `failed`), `total_fetched`/`new_endpoints`/`updated_endpoints` INT

**`indexer_state`** — Singleton row tracking V2 indexer progress
- `id` INT CHECK(id=1), `last_block` BIGINT

**`probe_results`** — Health check results per endpoint (one row per probe run)
- `id` UUID PK, `endpoint_id` UUID FK→endpoints (CASCADE)
- `probed_at` TIMESTAMPTZ, `status_code` INT (nullable), `latency_ms` INT (nullable)
- `health_status` TEXT CHECK ('alive', 'dead', 'unknown'), `is_valid_402` BOOLEAN
- `error_message` TEXT (nullable), `response_pay_to`/`response_amount_raw`/`response_network`/`response_asset` TEXT (nullable)
- `price_match` BOOLEAN — true if 402 response matches DB payment option
- `discrepancy_details` JSONB — structured diff when prices mismatch
- Index `(endpoint_id, probed_at DESC)` for efficient latest-probe-per-endpoint queries

### Materialized Views

**`endpoint_scores`** — Reliability metrics per endpoint (refreshed after sync or probe)
- `tx_count`, `total_volume_usd`, `unique_payers`, `first_tx_at`, `last_tx_at`
- `health_status` TEXT, `is_valid_402` BOOL, `latency_ms` INT, `price_match` BOOL, `last_probed_at` TIMESTAMPTZ
- `health_score` FLOAT 0–1: 0.70 base if alive+valid402, +0.15 if price_match, +0.05–0.15 latency bonus
- `recency_score` FLOAT 0–1: `exp(-0.03 * days_since_last_tx)`
- `reliability_score` INT 0–100: blended formula — endpoints with tx data: 70% tx-score + 30% health-score; endpoints with probe data only: health-score × 50 (max 50); neither: 0. Tx-score = `log(tx_count)` (30%) + `log(volume)` (25%) + `log(unique_payers)` (25%) + recency (20%)

**`discovered_sellers`** — On-chain payment recipients not yet matched to crawled endpoints
- `pay_to` TEXT PK, `tx_count`, `total_volume_usd`, `unique_payers`, `matched_endpoint_id` UUID nullable

## REST API Reference

Base URL: `http://localhost:8080/api`. CORS enabled (`*` origin). No authentication.

### POST /api/search
Semantic search via embed sidecar → pgvector cosine similarity.
```json
// Request
{ "query": "string", "filters": { "network": "base-sepolia", "method": "GET", "min_price": 0, "max_price": 1.0 }, "limit": 10 }
// Response
{ "results": [{ "endpoint": { ... }, "similarity": 0.85 }], "total": 42, "query_time_ms": 120 }
```

### GET /api/endpoints?limit=20&offset=0
Returns array of `{ endpoint, payment_options[] }` objects. Includes `reliability_score`, `health_status`, `latency_ms`, `last_probed_at` from materialized view.

### GET /api/endpoints/{id}
Single endpoint with its payment options.

### GET /api/endpoints/{id}/probes
Last 10 probe results for a specific endpoint. Returns array of `probe_results` rows.

### GET /api/stats
Network-wide aggregations: total endpoints, domains, by-network/asset/price-bracket breakdowns, time-series, crawl history, transaction stats, average reliability score, `alive_count`/`dead_count`/`unknown_count`.

### GET /api/facilitators
Facilitator list with tx_count, total_volume_usd, unique_payers from joined transaction data.

### GET /api/transactions?limit=50&offset=0&facilitator=Coinbase
Paginated transactions with facilitator name. Filter by facilitator name substring.

## Frontend Structure

Next.js 16 app in `web/`. Standalone output mode for Docker deployment.

### Pages (`web/src/app/`)
- `/` — Search bar + endpoints table with reliability scores
- `/facilitators` — Facilitator cards with stats
- `/transactions` — Transaction explorer with facilitator filter

### Key Components (`web/src/components/`)
- `search-bar.tsx` — Query input for semantic search
- `filter-chips.tsx` — Network/method filter toggles
- `endpoints-table.tsx` — Paginated endpoint list with health badges
- `health-badge.tsx` — Green/red/gray dot showing alive/dead/unprobed status + latency
- `reliability-bar.tsx` / `reliability-pulse.tsx` — Score visualization
- `area-chart-panel.tsx` / `horizontal-bar-chart.tsx` / `sparkline.tsx` — Recharts wrappers
- `facilitator-card.tsx` — Individual facilitator display
- `stat-line.tsx` — KPI metric display
- `nav.tsx` — Navigation bar

### API Layer (`web/src/lib/`)
- `api.ts` — Fetch functions (`fetchEndpoints`, `searchEndpoints`, `fetchStats`, `fetchFacilitators`, `fetchTransactions`)
- `types.ts` — Frontend TypeScript interfaces (camelCase versions of API types)
- `transforms.ts` — API response → frontend type conversion (snake_case → camelCase)

Environment: `NEXT_PUBLIC_API_URL` sets the Go API target.

## Embed Sidecar

`embed/server.py` — FastAPI service on port 8100.

- `POST /embed` — Takes `{ "text": "..." }`, returns `{ "embedding": [float x 384] }`
- `GET /health` — Returns model name + dimensions
- Model: `all-MiniLM-L6-v2` (384-dimensional vectors, loaded at startup)
- Dependencies: `sentence-transformers==4.1.0`, `fastapi==0.115.0`, `uvicorn==0.34.0`

First startup downloads the model (~90MB) to `~/.cache/huggingface`. Production Docker volume caches this.

## Docker / Production

### Dev (`docker-compose.yml`)
Just PostgreSQL with pgvector on port 5433.

### Production (`docker-compose.prod.yml`)
Full stack with health checks and dependency ordering:
1. **postgres** — pgvector:pg16, seeded from `data/seed.sql.gz` on first run
2. **embed** — Python sidecar (60s startup grace period for model download)
3. **api** — Go binary, waits for postgres + embed healthy
4. **web** — Next.js standalone, waits for api

Ports: postgres 5433, embed 8100, api 8081 (mapped from 8080), web 3000.

### Dockerfiles
- `Dockerfile.api` — Multi-stage Go build → `./agora.exe serve`
- `Dockerfile.embed` — Python + sentence-transformers → uvicorn
- `Dockerfile.web` — Node build → Next.js standalone

## Go Dependencies

Key direct dependencies (Go 1.24.0):
- `github.com/ethereum/go-ethereum` v1.17.0 — Ethereum client (V2 indexer)
- `github.com/golang-jwt/jwt/v5` — JWT signing for CDP API auth
- `github.com/golang-migrate/migrate/v4` — SQL migrations
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `github.com/pgvector/pgvector-go` — pgvector type support
- `github.com/kelseyhightower/envconfig` + `github.com/joho/godotenv` — Config
- `golang.org/x/sync` — errgroup for concurrency

## Data Pipeline Details

### Bazaar Crawler (`internal/crawler/`)
- `client.go` — HTTP client with exponential backoff (max 30s), 30s timeout per request
- `normalizer.go` — Converts BazaarItem → Endpoint + PaymentOption models
- `network.go` — Network string normalization (e.g., `base-sepolia` → `base-sepolia`)
- `runner.go` — Orchestrates pagination, 2s delay between pages, tracks crawl run stats
- `types.go` — BazaarResponse, BazaarItem, BazaarAccept structs matching Bazaar API schema

### CDP Sync (`internal/cdp/` + `internal/sync/`)
- `cdp/client.go` — Ed25519 JWT auth, queries CDP SQL API for USDC Transfer events
  - Queries `base.events` table for Transfer events where `transaction_from` = facilitator
  - Pagination: offset-based, 10,000 rows per page
  - USDC contract: `0x833589fcd6edb6e08f4c7c32d4f71b54bda02913`
- `sync/runner.go` — Iterates facilitators, chunks queries by month (falls back to weekly on scan limits)
  - Skips facilitators synced < 1 hour ago
  - 2s delay between facilitators
  - Default start date: 2024-06-01
  - USDC decimals: 6 (divides raw amount by 10^6 for USD)
  - Refreshes `endpoint_scores` and `discovered_sellers` materialized views after sync

### V2 Indexer (`internal/indexer/`)
- Queries Base via Alchemy RPC for Settled/SettledWithPermit events
- Processes in windows of `INDEXER_BLOCK_RANGE` blocks
- Finds near-zero transactions — V2 is barely used
- **Keep this code; don't delete it**

## Repository Methods (`internal/database/repository.go`)

Key database operations:
- **Crawl:** `StartCrawlRun`, `CompleteCrawlRun`, `FailCrawlRun`, `UpsertEndpoint`
- **Transactions:** `InsertTransactions`, `GetLastIndexedBlock`, `UpdateLastIndexedBlock`
- **Views:** `RefreshEndpointScores`, `RefreshDiscoveredSellers`
- **Search:** `SearchByVector(vector, filters, limit)` — cosine similarity with optional network/method/price filters
- **Read:** `GetEndpoints`, `GetEndpointByID`, `GetEndpointsWithPayments`, `GetStats`, `GetFacilitatorStats`, `GetTransactions`
- **Facilitators:** `GetBaseFacilitators`, `UpdateFacilitatorSyncTime`
- **Probe:** `GetEndpointsForProbing(limit, offset)`, `GetTotalEndpointCount`, `InsertProbeResults`, `GetProbeHistory(endpointID, limit)`

## Testing

Test files exist for:
- `internal/crawler/` — client, network normalization, response normalizer
- `internal/indexer/` — client, event decoder, facilitator matching
- `internal/api/` — embed client, HTTP handlers
- `internal/database/` — vector search
- `internal/prober/` — probe client (10 table-driven tests: 402/200/5xx/timeout/connection-refused, price match/mismatch, case-insensitive addresses)

Tests use standard `go test` with table-driven patterns. No external test database required for unit tests (they mock the DB layer or use httptest.NewServer).

## Seed Data

`data/seed.sql.gz` contains a full database snapshot:
- 12,571 endpoints, 12,803 payment options
- 148,106 transactions from 102 facilitators
- Probe results for all endpoints (health status, latency, 402 compliance)
- Pre-computed reliability scores in materialized views (blended tx + health signals)

Loaded automatically on first `docker-compose.prod.yml` startup via PostgreSQL's `/docker-entrypoint-initdb.d/` mechanism.

**To regenerate the seed** (after running `./agora.exe probe`):
```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) \
  pg_dump -U agora agora | gzip > data/seed.sql.gz
git add data/seed.sql.gz && git commit -m "chore: update seed with probe results"
```

## Key Gotchas

- **Docker PostgreSQL runs on port 5433** (not 5432) to avoid conflicts with local PostgreSQL.
- **Bazaar API rate limiting is aggressive.** Use `BAZAAR_PAGE_SIZE=1000` (max allowed) and `CRAWLER_CONCURRENCY=1`. See [coinbase/x402#1057](https://github.com/coinbase/x402/issues/1057).
- **CDP SQL API has scan limits.** The sync runner chunks queries by month, falling back to weekly. Rate limit: ~1 req/2s with exponential backoff on 429s.
- **`.env` is gitignored.** Copy `.env.example` to `.env`. Working config: port 5433, page size 1000, concurrency 1, max retries 10.
- **Production uses `docker-compose.prod.yml`** which includes postgres (with seed data), embed sidecar, Go API, and Next.js web — all with health checks and proper service dependencies.
- **BaseScan/Etherscan does NOT work for Base on free tier.** Don't retry this approach.
- **Facilitator address matching is case-insensitive.** The DB uses `lower(address)` in unique constraints and joins.
- **USDC has 6 decimals.** Raw amounts from the chain must be divided by 10^6 to get USD values.
- **Embed sidecar needs ~60s on first start** to download the sentence-transformers model. Production Docker uses a volume cache.
- **Probe command takes 15–30 minutes** to run against all 12,571 endpoints. Run it once, then re-dump the seed so reviewers get pre-populated health data without waiting.
- **x402 probe address comparison is case-insensitive.** Bazaar API returns EIP-55 checksummed addresses; on-chain data is lowercase. The prober uses `strings.EqualFold` for all address comparisons.
- **Migrations run automatically in Docker.** The `entrypoint.sh` runs `./agora migrate` before `./agora serve`. New migrations (000011 probe_results, 000012 updated endpoint_scores) apply on first container start after pulling new code.

## x402 Protocol Context

- **V1 (95%+ of payments):** Facilitator calls `transferWithAuthorization()` on USDC. Detected by `transaction_from` = known facilitator address on USDC Transfer events.
- **V2 (barely used):** Proxy contracts emit `Settled()`/`SettledWithPermit()`.
- **CDP SQL API:** Free Coinbase API, queries `base.events` table on Base chain. Primary data source for V1 indexing.
- **USDC on Base:** Contract `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913`.

## Database Access

```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT count(*) FROM endpoints;"
```

## Context & Research

- `context/` — Architecture, commands, database, research, and x402 protocol reference files
- `docs/context/` — Deep-dive research: x402 protocol overview, x402scan architecture, CDP SQL API reference, next steps
