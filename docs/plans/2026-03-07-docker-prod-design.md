# Production Docker Compose Design

**Goal:** One-command startup (`docker compose -f docker-compose.prod.yml up --build`) that gives a TA the full working Agora stack with pre-loaded data, including semantic search.

**Motivation:** Course project grading requires TAs to reproduce the system. Fewer manual steps = fewer failure points.

## Architecture

```
docker compose -f docker-compose.prod.yml up --build

┌─────────────────────────────────────────────────────────┐
│  Docker Network: agora-net                              │
│                                                         │
│  ┌──────────┐  :3000   ┌──────────┐  :8080             │
│  │  web     │ ───────► │  api     │                     │
│  │ (Next.js)│  fetch   │ (Go)     │                     │
│  │ node:22  │          │ golang   │                     │
│  └──────────┘          └──┬───┬───┘                     │
│                      embed│   │sql                      │
│                  ┌────────▼┐ ┌▼──────────┐              │
│                  │ embed   │ │ postgres  │              │
│                  │(FastAPI)│ │ (pgvector)│              │
│                  │python:3 │ │ :5432     │              │
│                  │ :8100   │ └───────────┘              │
│                  └─────────┘                            │
└─────────────────────────────────────────────────────────┘

Host ports exposed: 3000 (web UI), 8080 (API)
```

Services communicate via Docker DNS (container names). The browser fetches from `localhost:8080` (host-exposed API port).

## Services

### postgres
- Image: `pgvector/pgvector:pg16` (no custom Dockerfile)
- Data seeding: `data/seed.sql.gz` mounted into `/docker-entrypoint-initdb.d/`
- Seed includes all tables with embeddings pre-computed (~15-20MB gzipped)
- Health check: `pg_isready -U agora`
- Volume: `pgdata` persists data across restarts

### api (Go)
- Dockerfile: `Dockerfile.api` — multi-stage (golang:1.24 build → debian:bookworm-slim runtime)
- Entrypoint: `entrypoint.sh` runs `./agora migrate` then `./agora serve`
- Env: `DATABASE_URL=postgres://agora:agora@postgres:5432/agora`, `EMBED_URL=http://embed:8100`, `API_PORT=8080`
- Depends on: postgres (healthy), embed (healthy)

### embed (Python FastAPI)
- Dockerfile: `Dockerfile.embed` — python:3.12-slim, installs requirements, runs uvicorn
- Volume: `model-cache` at `/root/.cache/huggingface` caches the ~90MB sentence-transformers model
- Health check: `curl http://localhost:8100/health`
- First start downloads model (~30s), subsequent starts are instant

### web (Next.js)
- Dockerfile: `Dockerfile.web` — three-stage (deps → build → runtime with standalone output)
- Build-time env: `NEXT_PUBLIC_API_URL=http://localhost:8080`
- Depends on: api (started)

## Startup Order

```
postgres (healthy) ──┐
                     ├── api (migrate → serve)
embed (healthy) ─────┘
                         │
                         └── web (started)
```

## File Layout

```
agora/
├── docker-compose.prod.yml    # All 4 services
├── Dockerfile.api             # Go multi-stage
├── Dockerfile.embed           # Python sidecar
├── Dockerfile.web             # Next.js multi-stage
├── entrypoint.sh              # migrate → serve
├── data/
│   └── seed.sql.gz            # pg_dump with embeddings
├── docker-compose.yml         # Unchanged (dev, Postgres only)
```

## TA Experience

```bash
git clone https://github.com/yamanakbas/agora.git && cd agora
docker compose -f docker-compose.prod.yml up --build
# Open http://localhost:3000 — full UI with 12,571 endpoints, charts, semantic search
```

Optional: `docker compose -f docker-compose.prod.yml exec api ./agora crawl` to run a live crawl.
Reset: `docker compose -f docker-compose.prod.yml down -v` to wipe and re-seed.

## What Stays Unchanged

- `docker-compose.yml` for local dev (Postgres only)
- `.env` / `.env.example` for local dev config
- All existing source code — no modifications needed
