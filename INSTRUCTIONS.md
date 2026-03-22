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

| Service    | Port  | Description                              |
|------------|-------|------------------------------------------|
| PostgreSQL | 5433  | pgvector database, auto-seeded with 12,571 endpoints |
| Embed      | 8100  | Python sidecar for text embeddings (all-MiniLM-L6-v2) |
| API        | 8081  | Go REST API (search, list, stats)        |
| Web        | 3000  | Next.js frontend with search and charts  |

**First startup takes ~2 minutes** (model download ~400MB). Subsequent starts are faster (model cached in Docker volume).

## Verify It Works

### Web UI
- http://localhost:3000 — Endpoints page (paginated, 12,571 endpoints)
- http://localhost:3000/network — Network analytics (charts by chain, asset, price)
- http://localhost:3000/facilitators — Facilitator information

### API Endpoints
```bash
# Stats overview (total endpoints, domains, distributions)
curl http://localhost:8081/api/stats

# List endpoints with pagination
curl "http://localhost:8081/api/endpoints?limit=5&offset=0"

# Get a single endpoint by ID
curl http://localhost:8081/api/endpoints/<uuid>

# Semantic search (natural language)
curl -X POST http://localhost:8081/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "weather API", "limit": 5}'
```

### Search Examples
Try these queries in the web UI search bar or via the API:
- "weather API" — finds weather-related endpoints
- "image generation" — finds AI image service endpoints
- "cheap endpoints under a penny" — finds low-cost services
- "crypto price feed" — finds cryptocurrency data endpoints

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

# Run migrations
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

## Project Structure

```
agora/
├── cmd/agora/          # CLI entrypoint (migrate, crawl, index, serve)
├── internal/
│   ├── api/            # REST API server, handlers, embed client
│   ├── config/         # Environment-based configuration
│   ├── crawler/        # Bazaar API client, normalizer, runner
│   ├── database/       # PostgreSQL pool, migrations, repository
│   ├── indexer/        # On-chain V2 transaction indexer
│   └── models/         # Domain models (Endpoint, PaymentOption, etc.)
├── migrations/         # SQL migration files
├── embed/              # Python embedding sidecar (FastAPI + sentence-transformers)
├── web/                # Next.js frontend (React, Tailwind, Recharts)
├── data/               # Database seed dump (seed.sql.gz)
├── docs/               # Design documents and implementation plans
├── docker-compose.yml  # Dev database only
└── docker-compose.prod.yml  # Full production stack (4 services)
```

## Key Design Decisions

- **Semantic search** uses all-MiniLM-L6-v2 (384-dim vectors) via a Python sidecar, stored in pgvector with HNSW indexing for fast cosine similarity search
- **Go API** handles all business logic; Python is only used for ML inference
- **Database seed** (`data/seed.sql.gz`) is committed so reviewers get the full dataset without needing to crawl
- **Multi-stage Docker builds** keep images small (Go: ~100MB, Web: ~150MB)
