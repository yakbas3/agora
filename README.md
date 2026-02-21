# AGORA

**Discovery, Search, and Reliability for the x402 Endpoint Ecosystem**

AGORA is a search engine and reliability layer for [x402](https://www.x402.org/) protocol endpoints. It crawls, indexes, and enriches metadata from multiple facilitator registries — starting with the [Coinbase Bazaar API](https://docs.cdp.coinbase.com/x402/bazaar) — and exposes a unified REST API that lets AI agents discover callable endpoints using natural language queries.

> **Course project** for CS 4365/6365 — Introduction to Enterprise Computing, Spring 2026, Georgia Tech.

---

## Problem

The x402 ecosystem has 12,000+ registered endpoints and growing, but the Bazaar discovery API has no server-side search or filtering. Agents must paginate the entire catalog client-side, with rate limits that prevent full traversal. Metadata quality varies wildly — roughly a third of endpoints have rich input/output schemas, another third have only basic descriptions, and the rest are generic or spam-like registrations.

Existing tools address fragments of this:

- **SentEdge AI** — Bazaar dashboard with keyword filters and trending leaderboards
- **x402 Index** — Community-submitted directory behind an x402-gated API
- **QuickNode** — Interactive testing tool with Bazaar search
- **Standalone health monitors** — Liveness probes for individual endpoints

None combine semantic search, reliability scoring, and a programmatic API designed for autonomous agent consumption into a single system.

## What AGORA Does

| Capability | Description |
|---|---|
| **Crawl & Index** | Paginates the Bazaar API (and other facilitator registries), deduplicates, normalizes metadata, and stores it locally to avoid upstream rate limits. |
| **Semantic Search** | Embeds endpoint descriptions into vector space so agents can query in natural language (e.g., *"image generation API under $0.05 on Base"*). |
| **Keyword & Filter Search** | Structured filters for price range, network, HTTP method, domain, and facilitator. |
| **Reliability Scoring** | Periodic health checks — liveness probes, latency measurement, x402 compliance validation — surfaced as a reliability score per endpoint. |
| **Agent-First REST API** | `POST /search` accepts a natural language query and returns ranked results with metadata, pricing, and reliability scores. |

## Architecture

```
┌─────────────────┐
│  Coinbase Bazaar │─┐
│  PayAI           │─┼──▶ Crawler / Indexer ──▶ Index Database
│  Other facilit.  │─┘         │                  ▲    │
└─────────────────┘            │           Health  │    │
                               │           Monitor─┘    ▼
                               │                   Search Engine
                               │                   (semantic + keyword
                               │                    + ranking)
                               │                        │
                               │                        ▼
                          ┌─────────┐           AGORA REST API
                          │   x402  │◀─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─  AI Agents
                          │Endpoints│    agent discovers, then    (natural
                          └─────────┘    calls endpoint directly  language)
```

AGORA is a **discovery and reliability layer** — it does not proxy x402 payments. Agents use AGORA to find the right endpoint, then call and pay for it directly via the x402 protocol.

A higher-fidelity diagram is included in the project checkpoint report.

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Python 3.12+ |
| API Framework | FastAPI |
| Database | PostgreSQL + pgvector |
| Embeddings | OpenAI `text-embedding-3-small` (or local alternative) |
| Health Checks | `httpx` async probes |
| Crawler | Custom async paginator with backoff and caching |

## Project Roadmap

| Week | Milestone |
|---|---|
| 5 | Research, literature review, initial proposal |
| 6 | GitHub repo, database schema, API scaffolding |
| 7 | Bazaar API crawler (pagination, rate limits, caching) |
| 8 | Semantic search with embeddings |
| 9 | Keyword / filter search (price, network, method, domain) |
| 10 | Reliability scoring system (health checks, liveness probing) |
| 11 | REST API for agents (natural language queries → ranked results) |
| 12 | Multi-facilitator support, metadata enrichment |
| 13 | Integration testing, edge case handling |
| 14 | Performance optimization, caching, documentation |
| 15 | Final testing, demo, presentation |

**Target metric:** 80% accuracy — correct endpoint in the top 5 results for natural language queries across a test suite of 50+ queries.

## Getting Started

> 🚧 Under active development — setup instructions will be added as components are built.

```bash
git clone https://github.com/yamanakbas/agora.git
cd agora
# setup instructions coming soon
```

## Repository Structure

```
agora/
├── src/
│   ├── crawler/       # Bazaar + multi-facilitator crawlers
│   ├── indexer/        # Metadata normalization, embedding generation
│   ├── search/         # Semantic + keyword search engine
│   ├── health/         # Endpoint reliability monitoring
│   └── api/            # FastAPI REST interface
├── tests/
├── docs/
├── README.md
└── requirements.txt
```

## License

MIT

## Author

**Yaman Akbas** — Georgia Institute of Technology, Spring 2026
