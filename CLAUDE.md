# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Agora is a Go-based crawler and search engine for the x402 protocol ecosystem. It crawls the Coinbase Bazaar discovery API, normalizes endpoint metadata, and stores it in PostgreSQL with pgvector. Future phases add semantic search, reliability scoring, and a REST API for AI agents.

Course project for CS 4365/6365 at Georgia Tech, Spring 2026.

## Commands

```bash
# Start PostgreSQL (pgvector on port 5433, not 5432)
docker compose up -d

# Build
go build -o agora.exe ./cmd/agora

# Run migrations
./agora.exe migrate

# Run full crawl
./agora.exe crawl

# Run all tests
go test ./...

# Run a single package's tests
go test ./internal/crawler/...
go test ./internal/config/...

# Run a single test by name
go test ./internal/crawler/... -run TestNormalizeNetwork
```

## Architecture

The CLI (`cmd/agora/main.go`) has two commands: `migrate` and `crawl`. The crawl pipeline flows:

```
Bazaar API → Client (paginate + retry) → Normalizer → Repository (upsert) → PostgreSQL
```

- **`internal/config/`** — envconfig + godotenv. All config via env vars (see `.env.example`).
- **`internal/crawler/`** — API client with exponential backoff and errgroup concurrency, Bazaar response types, normalizer (converts raw API items to domain models), network mapping (chain IDs like `eip155:8453` → `base`), and a runner that orchestrates the full crawl lifecycle.
- **`internal/database/`** — pgxpool connection with pgvector type registration, golang-migrate runner, and repository with transaction-based upserts (check existing → insert or update → replace payment options).
- **`internal/models/`** — Endpoint, PaymentOption, CrawlRun structs.
- **`migrations/`** — SQL files + `embed.go` that exports `var FS embed.FS`. This pattern exists because Go's `//go:embed` doesn't allow `../` paths, so the embed lives in the migrations directory itself and is imported by `internal/database/db.go`.

## Key Gotchas

- **Docker PostgreSQL runs on port 5433** (not 5432) to avoid conflicts with local PostgreSQL.
- **Bazaar API rate limiting is aggressive.** Use `BAZAAR_PAGE_SIZE=1000` (max allowed) and `CRAWLER_CONCURRENCY=1` with a 2-second inter-page delay. See [coinbase/x402#1057](https://github.com/coinbase/x402/issues/1057).
- **`.env` is gitignored.** Copy `.env.example` to `.env` and adjust values. The actual working config uses port 5433, page size 1000, concurrency 1, and max retries 10.

## Database Access (via Docker)

```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT count(*) FROM endpoints;"
```

## Context & Research

**Start here when resuming work:** Read `docs/context/` for full background on x402 protocol, x402scan architecture, and the CDP SQL API approach.

- `docs/context/x402-protocol-overview.md` — How x402 works (V1 vs V2, facilitators, settlement)
- `docs/context/x402scan-architecture.md` — How x402scan gets its data (CDP SQL API, facilitator filtering, sync flow)
- `docs/context/cdp-sql-api.md` — Coinbase Developer Platform SQL API reference (auth, tables, queries, limits)
- `docs/context/next-steps.md` — Where we left off and what to build next (V1 indexer using CDP SQL API)

## Current State (Feb 2026)

- **Phase 1 (Bazaar Crawler):** Complete. 12,571 endpoints + 12,803 payment options in PostgreSQL.
- **V2 Indexer (Alchemy RPC):** Code exists in `internal/indexer/` but finds 0 transactions (V2 barely used). Keep, don't delete.
- **Next:** Build V1 transaction indexer using CDP SQL API (imitating x402scan's approach). Brainstorming in progress — see `docs/context/next-steps.md`.

## x402 Key Facts

- **V1 (95%+ of payments):** Facilitator calls `transferWithAuthorization()` on USDC. Identified by `transaction_from` = known facilitator address on USDC Transfer events.
- **V2 (barely used):** Proxy contracts emit `Settled()`/`SettledWithPermit()`.
- **CDP SQL API:** Free, queries `base.events` table on Base chain. x402scan's primary data source.
- **BaseScan/Etherscan:** Does NOT work for Base on free tier. Don't retry.

## EDA

`eda/bazaar_eda.ipynb` — Jupyter notebook with pandas/matplotlib analysis of crawled data. Requires Python with `jupyter pandas psycopg2-binary matplotlib seaborn`.
