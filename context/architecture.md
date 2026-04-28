## Architecture

Agora is structured as a Go monolith with a Python embedding sidecar and a Next.js frontend.
The CLI (`cmd/agora/main.go`) dispatches to five subcommands: `migrate`, `crawl`, `sync`, `index`, `serve`.

### Data Flow (High Level)

```
Bazaar API ──→ Crawler ──→ PostgreSQL ←── CDP Sync ←── CDP SQL API
                               │
                          REST API (:8080)
                            │      │
                   Embed Sidecar   Web Frontend (:3000)
                     (:8100)
```

### Data Pipeline — Three Independent Ingestion Paths

All three write to the same PostgreSQL database (pgvector on port 5433):

1. **Bazaar Crawler** (`internal/crawler/`)
   - Paginates the Coinbase Bazaar discovery API with exponential backoff
   - Normalizes raw API responses into domain models (endpoints + payment options)
   - Upserts via transaction-based logic (check existing → insert or update → replace payment options)
   - Rate-limited: 1 request per 2 seconds, page size 1000, concurrency 1

2. **CDP Sync** (`internal/sync/` + `internal/cdp/`)
   - Queries the CDP SQL API for USDC Transfer events by known facilitator addresses
   - Chunks queries by month, falls back to weekly if scan limits are exceeded
   - Updates facilitator sync timestamps and refreshes materialized views
   - Materialized views: `endpoint_scores` (reliability scoring), `discovered_sellers`

3. **V2 Indexer** (`internal/indexer/`)
   - Queries Base via Alchemy RPC for `Settled`/`SettledWithPermit` events on proxy contracts
   - Finds near-zero transactions because V2 is barely used as of early 2026
   - Code is kept for completeness — do not delete

### REST API (`internal/api/`)

net/http server with CORS middleware. Endpoints:
- `POST /api/search` — Semantic search via embedding sidecar → pgvector cosine similarity
- `GET /api/endpoints` — Paginated list of all indexed x402 endpoints
- `GET /api/endpoints/{id}` — Single endpoint detail with payment options
- `GET /api/stats` — Aggregate statistics (endpoint counts, transaction volume, etc.)
- `GET /api/facilitators` — List of known facilitator wallets with sync status
- `GET /api/transactions` — Recent x402 payment transactions

### Embed Sidecar (`embed/`)

FastAPI Python service using `all-MiniLM-L6-v2` (384-dimensional vectors).
The Go API calls `POST /embed` to vectorize search queries at runtime.
Runs on port 8100 in both dev and production.

### Web Frontend (`web/`)

Next.js 16 + React 19 + Tailwind 4 + Recharts.
Talks to the Go API on port 8080. Dev server runs on port 3000.

### Key Internal Packages

- **`internal/config/`** — envconfig + godotenv. All config via env vars (see `.env.example`).
- **`internal/models/`** — Domain structs: Endpoint, PaymentOption, CrawlRun, Transaction, Facilitator.
- **`internal/database/`** — pgxpool connection with pgvector type registration, golang-migrate runner, and repository with transaction-based upserts and vector search.
- **`migrations/`** — SQL migration files + `embed.go` that exports `var FS embed.FS`. This pattern exists because Go's `//go:embed` doesn't allow `../` paths, so the embed lives in the migrations directory itself and is imported by `internal/database/db.go`.

### Production Deployment

`docker-compose.prod.yml` orchestrates the full stack:
- PostgreSQL (pgvector, seeded with data)
- Embed sidecar (Python)
- Go API server
- Next.js web frontend
All services have health checks and proper dependency ordering.
