# AGORA — Build, Run, and Test Instructions

## Overview

AGORA is a search engine and analytics platform for the x402 protocol ecosystem. It crawls the Coinbase Bazaar discovery API, stores endpoint metadata in PostgreSQL with pgvector embeddings, and serves a REST API + Next.js frontend for semantic search and network analytics.

**Tech Stack:** Go 1.24, Python 3.12, Node.js 22, PostgreSQL 16 (pgvector), Docker

YOU NEED DOCKER FOR THIS

## Quick Start (One Command)

```bash
git clone https://github.com/yakbas3/agora.git
cd agora
docker compose -f docker-compose.prod.yml up --build
```

This starts all 4 services:

| Service    | Port  | Description                                          |
|------------|-------|------------------------------------------------------|
| PostgreSQL | 5433  | pgvector database, auto-seeded with full dataset     |
| Embed      | 8100  | Python sidecar for text embeddings (all-MiniLM-L6-v2)|
| API        | 8081  | Go REST API (search, list, stats, health data)       |
| Web        | 3000  | Next.js frontend with search, charts, health badges  |

**First startup takes ~2 minutes** (model download ~400MB). Subsequent starts are faster (model cached in Docker volume).

The API container automatically runs database migrations on startup before serving. No manual migration step is needed.

## Verify It Works

### Web UI
- http://localhost:3000 — Endpoints page (search + reliability + health status badges)
- http://localhost:3000/facilitators — Facilitator information
- http://localhost:3000/transactions — Transaction explorer

### API Endpoints
```bash
# Stats overview (includes alive_count, dead_count, unknown_count)
curl http://localhost:8081/api/stats

# List endpoints (includes health_status, latency_ms, last_probed_at)
curl "http://localhost:8081/api/endpoints?limit=5&offset=0"

# Semantic search
curl -X POST http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "weather API", "limit": 5}'

# Probe history for a specific endpoint
curl http://localhost:8081/api/endpoints/<uuid>/probes
```

### Verify Seed Data
```bash
# Connect to the database
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) \
  psql -U agora -d agora -c "
    SELECT
      COUNT(*) AS endpoints,
      COUNT(*) FILTER (WHERE reliability_score > 0) AS scored,
      COUNT(*) FILTER (WHERE health_status = 'alive') AS alive,
      COUNT(*) FILTER (WHERE health_status = 'dead') AS dead
    FROM endpoint_scores;
  "
```

Expected output (approximate):
```
 endpoints | scored | alive | dead
-----------+--------+-------+------
     12571 |   3000+|  2000+|  500+
```

```bash
# Verify probe results exist
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) \
  psql -U agora -d agora -c "SELECT COUNT(*) FROM probe_results;"
```

## Tear Down

```bash
# Stop containers (keep data)
docker compose -f docker-compose.prod.yml down

# Stop and remove all data (full reset)
docker compose -f docker-compose.prod.yml down -v
```

## Development Setup (Without Docker)

### Prerequisites
- Go 1.24+
- Python 3.12+
- Node.js 22+
- Docker (for PostgreSQL only)

### Database
```bash
# Start PostgreSQL with pgvector (port 5433)
docker compose up -d

# Copy environment config
cp .env.example .env
# Edit .env if needed (defaults work for local dev)
```

### Go API
```bash
# Build
go build -o agora.exe ./cmd/agora

# Run migrations (required before first use and after pulling new code)
./agora.exe migrate

# Start API server
./agora.exe serve
```

### Python Embedding Sidecar
```bash
cd embed
pip install -r requirements.txt
uvicorn server:app --host 0.0.0.0 --port 8100
```

### Next.js Frontend
```bash
cd web
npm install
npm run dev
```

### Run Tests
```bash
go test ./...
```

## CLI Commands

| Command              | Description                                                   |
|----------------------|---------------------------------------------------------------|
| `./agora.exe migrate`| Run database migrations (run once after pulling new code)     |
| `./agora.exe crawl`  | Crawl Bazaar API → 12,571 endpoints + payment options         |
| `./agora.exe sync`   | Sync V1 USDC transactions from CDP SQL API                    |
| `./agora.exe index`  | Index V2 on-chain transactions (Alchemy RPC)                  |
| `./agora.exe probe`  | Probe all endpoints for x402 health + compliance (see below)  |
| `./agora.exe serve`  | Start REST API server on :8080                                |

### Running the Probe Command

The probe command makes HTTP requests to all 12,571 indexed endpoints, checks whether they respond with a proper x402 `402 Payment Required` response, records latency and payment metadata, and blends results into reliability scores.

```bash
./agora.exe probe
```

Expected output:
```
Probing 12571 endpoints (concurrency=20, timeout=10s, domainDelay=200ms)
Progress: 500 / 12571 endpoints probed
Progress: 1000 / 12571 endpoints probed
...
Refreshing endpoint_scores materialized view...
Probe complete. 12571 endpoints probed.
```

**Duration:** ~15–30 minutes for the full dataset (20 concurrent requests, 10s timeout per request).

**Configuration via .env:**
```
PROBER_CONCURRENCY=20      # Parallel HTTP requests (default: 20)
PROBER_TIMEOUT_SECS=10     # Per-request timeout in seconds (default: 10)
PROBER_BATCH_SIZE=500      # Endpoints loaded from DB per batch (default: 500)
PROBER_DOMAIN_DELAY_MS=200 # Min milliseconds between requests to same domain (default: 200)
```

## Reproducibility: Updating the Seed

The `data/seed.sql.gz` file is committed to the repository and loaded automatically by Docker on first startup. It contains a complete database snapshot including endpoints, transactions, probe results, and pre-computed scores.

**To regenerate the seed after running the probe command:**

```bash
# 1. Start the database
docker compose up -d

# 2. Run migrations
./agora.exe migrate

# 3. Run the probe (takes 15-30 minutes)
./agora.exe probe

# 4. Dump the updated database to seed.sql.gz
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) \
  pg_dump -U agora agora | gzip > data/seed.sql.gz

# 5. Verify the seed includes probe results
zcat data/seed.sql.gz | grep -c "INSERT INTO probe_results"

# 6. Commit the new seed
git add data/seed.sql.gz
git commit -m "chore: update seed with probe results"
```

After committing, `docker compose -f docker-compose.prod.yml up --build` will load the seed with all probe data pre-populated — TAs see health badges and alive/dead counts immediately without needing to run `probe` themselves.

## Project Structure

```
agora/
├── cmd/agora/          # CLI entrypoint (migrate, crawl, index, sync, probe, serve)
├── internal/
│   ├── api/            # REST API server, handlers, embed client
│   ├── config/         # Environment-based configuration
│   ├── crawler/        # Bazaar API client, normalizer, runner
│   ├── database/       # PostgreSQL pool, migrations, repository
│   ├── indexer/        # On-chain V2 transaction indexer
│   ├── prober/         # x402 health check prober (client, runner, types)
│   └── models/         # Domain models (Endpoint, PaymentOption, ProbeResult, etc.)
├── migrations/         # SQL migration files (000001–000012)
├── embed/              # Python embedding sidecar (FastAPI + sentence-transformers)
├── web/                # Next.js frontend (React, Tailwind, Recharts)
├── data/               # Database seed dump (seed.sql.gz) — includes probe results
├── docs/               # Design documents and implementation plans
├── docker-compose.yml  # Dev database only
└── docker-compose.prod.yml  # Full production stack (4 services)
```

## Key Design Decisions

- **Semantic search** uses all-MiniLM-L6-v2 (384-dim vectors) via a Python sidecar, stored in pgvector with HNSW indexing for fast cosine similarity search
- **Go API** handles all business logic; Python is only used for ML inference
- **Reliability scoring** blends on-chain transaction signals (tx count, volume, payer diversity, recency) with x402 health probe results into a 0–100 composite score
- **Health probing** makes unauthenticated HTTP requests to endpoints, verifies they return HTTP 402 with valid x402 payment instructions, detects price discrepancies
- **Database seed** (`data/seed.sql.gz`) is committed so reviewers get the full dataset without needing to crawl, sync, or probe
- **Multi-stage Docker builds** keep images small (Go: ~100MB, Web: ~150MB)
- **Migrations run automatically** inside the API Docker container via `entrypoint.sh` before starting the server
