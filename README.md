# AGORA

**Discovery, Search, and Reliability for the x402 Endpoint Ecosystem**

AGORA is a search engine and reliability layer for [x402](https://www.x402.org/) protocol endpoints. It crawls, indexes, and enriches metadata from multiple facilitator registries — starting with the [Coinbase Bazaar API](https://docs.cdp.coinbase.com/x402/bazaar) — and exposes a unified REST API that lets AI agents discover callable endpoints using natural language queries.

> **Course project** for CS 4365/6365 — Introduction to Enterprise Computing, Spring 2026, Georgia Tech.

---

## Quick Start (One Command)

The entire system — database with pre-seeded data, embedding sidecar, Go API, and Next.js frontend — runs with a single command:

```bash
git clone https://github.com/yakbas3/agora.git
cd agora
docker compose -f docker-compose.prod.yml up -d
```

This starts 4 services:
- **PostgreSQL** (port 5433) — pre-seeded with 12,571 endpoints, 148,106 transactions, 102 facilitators, and reliability scores
- **Embedding sidecar** (port 8100) — Python/FastAPI with all-MiniLM-L6-v2 for semantic search (first start downloads the model, ~1 min)
- **Go API** (port 8081) — REST API with 7 endpoints
- **Next.js frontend** (port 3000) — 4-page dashboard

Once running, open http://localhost:3000 for the frontend or query the API directly:

```bash
# Search endpoints using natural language
curl -X POST http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "image generation API", "limit": 5}'

# Get network statistics
curl http://localhost:8081/api/stats

# List endpoints with reliability scores
curl http://localhost:8081/api/endpoints?limit=10

# Get facilitator stats
curl http://localhost:8081/api/facilitators

# Browse transactions
curl http://localhost:8081/api/transactions?limit=10
```

To stop everything: `docker compose -f docker-compose.prod.yml down`

---

## What AGORA Does

| Capability | Description |
|---|---|
| **Crawl & Index** | Paginates the Bazaar API, deduplicates, normalizes metadata, and stores locally to avoid upstream rate limits. 12,571 endpoints indexed. |
| **Transaction Indexing** | Queries the CDP SQL API for USDC Transfer events by known facilitator addresses on Base chain. 148,106 transactions indexed across 102 facilitators. |
| **Semantic Search** | Embeds endpoint descriptions via all-MiniLM-L6-v2 (384-dim vectors, pgvector HNSW index) for natural language queries. |
| **Keyword & Filter Search** | Structured filters for price range, network, HTTP method, and domain. |
| **Reliability Scoring** | Weighted composite score (0–100) from on-chain signals: transaction count (30%), payment volume (25%), payer diversity (25%), recency with exponential decay (20%). 215 endpoints scored. |
| **REST API** | 7 endpoints: search, list, detail, stats, facilitators, transactions, endpoint-by-id. |
| **Frontend Dashboard** | 4-page Next.js app with search, analytics charts, facilitator info, and transaction explorer. |

## Architecture

```
┌─────────────────┐
│  Coinbase Bazaar │──▶ Crawler ──▶ PostgreSQL + pgvector
└─────────────────┘                     ▲
                                        │
┌─────────────────┐                     │
│  CDP SQL API    │──▶ V1 Indexer ──────┘
│  (Base chain)   │         │
└─────────────────┘         ▼
                    endpoint_scores (materialized view)
                            │
                            ▼
                    Go REST API (chi router)
                     ├── /api/search (POST)
                     ├── /api/endpoints (GET)
                     ├── /api/endpoints/:id (GET)
                     ├── /api/stats (GET)
                     ├── /api/facilitators (GET)
                     └── /api/transactions (GET)
                            │
                            ▼
                    Next.js Frontend
                     ├── / (Search + Endpoints)
                     ├── /network (Analytics)
                     ├── /facilitators
                     └── /transactions
```

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.24 |
| Database | PostgreSQL 16 + pgvector |
| Embeddings | all-MiniLM-L6-v2 (384-dim, Python sidecar) |
| Frontend | Next.js 16, React 19, Tailwind CSS 4, Recharts |
| Deployment | Docker Compose (4 services) |
| On-chain data | Coinbase Developer Platform SQL API |

## Database

| Table | Count |
|---|---|
| endpoints | 12,571 |
| payment_options | 12,803 |
| transactions | 148,106 |
| facilitators | 102 |
| scored endpoints (reliability > 0) | 215 |

The `endpoint_scores` materialized view computes reliability scores using a weighted composite formula with log normalization:

```sql
reliability_score = round((
    0.30 * ln(1 + tx_count)    / ln(1 + max_tx_count) +
    0.25 * ln(1 + volume)      / ln(1 + max_volume) +
    0.25 * ln(1 + payers)      / ln(1 + max_payers) +
    0.20 * exp(-0.03 * days_since_last_tx)
) * 100)
```

## Development Setup

For local development (without Docker for the application services):

```bash
# Start PostgreSQL with pgvector (runs on port 5433)
docker compose up -d

# Copy and configure environment
cp .env.example .env

# Build
go build -o agora.exe ./cmd/agora

# Run migrations
./agora.exe migrate

# Crawl the Bazaar API (~12,500 endpoints)
./agora.exe crawl

# Sync V1 transactions (requires CDP API keys in .env)
./agora.exe sync

# Start API server
./agora.exe serve

# Start frontend (separate terminal)
cd web && npm install && npm run dev

# Run tests
go test ./...
```

## Repository Structure

```
agora/
├── cmd/agora/              # CLI entrypoint (migrate, crawl, sync, index, serve)
├── internal/
│   ├── api/                # REST API handlers (chi router)
│   ├── cdp/                # CDP SQL API client (ED25519 JWT auth)
│   ├── config/             # Environment-based configuration
│   ├── crawler/            # Bazaar API client, normalizer, runner
│   ├── database/           # pgxpool connection, migrations, repository
│   ├── indexer/            # V2 indexer (Alchemy RPC, barely used)
│   ├── models/             # Endpoint, PaymentOption, Transaction, Facilitator
│   └── sync/               # V1 transaction sync runner
├── migrations/             # SQL schema files (10 migrations) + embed.go
├── embed/                  # Python embedding sidecar (FastAPI + sentence-transformers)
├── web/                    # Next.js frontend
├── data/                   # Pre-seeded database dump (seed.sql.gz)
├── eda/                    # Jupyter notebook for exploratory data analysis
├── docs/                   # Design documents and implementation plans
├── docker-compose.yml      # Development: PostgreSQL only
├── docker-compose.prod.yml # Production: all 4 services with pre-seeded data
├── Dockerfile.api          # Go API multi-stage build
├── Dockerfile.embed        # Python embedding sidecar
└── Dockerfile.web          # Next.js frontend
```

## License

MIT

## Author

**Yaman Akbas** — Georgia Institute of Technology, Spring 2026
