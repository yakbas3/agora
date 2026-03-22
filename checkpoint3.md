Georgia Institute of Technology
CS 4365/6365: IEC Spring 2026
Project Checkpoint Report

Group: 2
Name(s): Yaman Akbas
Project: AGORA

## Point A: Current State and Context

At the time of Checkpoint 2, AGORA had a fully functional crawl-and-search pipeline: 12,571 x402 endpoints ingested from the Bazaar API, semantic search via pgvector embeddings, a Go REST API, and a Next.js frontend — all deployable with a single `docker compose` command. The main gaps were on-chain transaction data and reliability scoring.

Since Checkpoint 2, the project has added two major capabilities. First, a **V1 transaction indexer** that queries the Coinbase Developer Platform (CDP) SQL API for USDC Transfer events by known facilitator addresses — this brought in 148,106 real on-chain transactions across 102 facilitator addresses. Second, a **reliability scoring system** that computes a 0–100 composite score for each endpoint based on four on-chain signals: transaction count, payment volume, payer diversity, and recency. The scores are computed entirely in PostgreSQL via a materialized view and served through all existing API endpoints. The frontend's existing `ReliabilityPulse` and `ReliabilityBar` UI components now display real scores. Of the 12,571 endpoints, 215 have non-zero reliability scores, reflecting the fact that most Bazaar-listed endpoints have never received an on-chain payment.

## Point B: End-of-Semester Deliverables

The planned deliverables remain the same as Checkpoint 1, with updated status:

| # | Deliverable | Status | Notes |
|---|------------|--------|-------|
| 1 | Complete x402 endpoint index | DONE | 12,571 endpoints and 12,803 payment options crawled and stored in PostgreSQL with pgvector |
| 2 | Semantic search | DONE | Natural language queries via all-MiniLM-L6-v2 embeddings (384-dim, HNSW indexed) |
| 3 | REST API for agent consumption | DONE | 7 endpoints: search, list, detail, stats, facilitators, transactions, endpoint-by-id |
| 4 | Frontend dashboard (bonus) | DONE | 4-page Next.js app with search, analytics charts, facilitator info, transaction explorer |
| 5 | Endpoint reliability scores | DONE | Weighted composite scoring (tx count 30%, volume 25%, payer diversity 25%, recency 20%) computed in PostgreSQL materialized view, served via API, displayed in frontend |
| 6 | Metadata enrichment | PLANNED | URL pattern analysis and 402 response probing — scheduled for final checkpoint |
| 7 | One-command deployment | DONE | `docker compose` starts all 4 services with pre-seeded data |

### Milestone Chart

| Week | Planned (from Checkpoint 1) | Actual |
|------|---------------------------|--------|
| 5 | Research & literature review on x402 protocol, Bazaar API, and existing discovery tools; initial project proposal, Set up the Github Repo | Completed as planned |
| 6 | Set up project infrastructure: database schema design, initial API scaffolding | Completed — DB schema with 8 migration files, Go API scaffolded with CLI commands (migrate, crawl, index, serve) |
| 7 | Build Bazaar API crawler: paginate and cache all 12,000+ endpoints into local index | Completed — full crawler operational, 12,571 endpoints and 12,803 payment options ingested |
| 8 | Implement semantic search using embeddings over endpoint descriptions and schemas | Completed — Python embedding sidecar (all-MiniLM-L6-v2), 384-dim vectors, pgvector HNSW index, cosine similarity search |
| 9 | Implement keyword/filter search (by price, network, method, domain) | Completed early — filter support integrated into vector search; also built REST API with 4 endpoints and full Next.js frontend (bonus) |
| 10 | Build reliability scoring system: periodic health checks, liveness probing, response time tracking | Completed — reliability scoring implemented using on-chain transaction signals rather than health checks (pivot due to data availability). Weighted composite formula (30% tx count, 25% volume, 25% payer diversity, 20% recency) computed in PostgreSQL materialized view. 215 of 12,571 endpoints scored. |
| 11 | Build REST API for agents: natural language query endpoint returning ranked results with metadata | Completed early (week 8) — POST /api/search with semantic ranking, GET /api/endpoints, GET /api/endpoints/:id, GET /api/stats, GET /api/facilitators, GET /api/transactions |
| 12 | Integrate multi-facilitator support (CDP, PayAI, x402.org testnet); enrichment of low-metadata endpoints | Partially completed — V1 transaction indexer built using CDP SQL API, 102 facilitator addresses seeded from x402scan registry, 148,106 transactions indexed. Metadata enrichment still planned. |
| 13 | End-to-end integration testing with real x402 endpoints; handle edge cases (stale endpoints, duplicates) | In progress — address casing normalization (EIP-55 vs lowercase) discovered and fixed during scoring implementation |
| 14 | Performance optimization, caching layer, documentation, and demo preparation | Planned |
| 15 | Final testing, demo, and project presentation | Planned |

## Current Progress Report (Match)

Over the past two weeks, the project added two significant new subsystems:

**(1) V1 Transaction Indexer (Mar 19):** Built a Go client for the Coinbase Developer Platform SQL API that authenticates via CDP API keys (ED25519 JWT signing), queries the `base.events` table for USDC Transfer events matching known facilitator addresses, and inserts the results into PostgreSQL. Seeded 102 facilitator addresses from the x402scan open-source registry. Indexed 148,106 real on-chain transactions. Added API endpoints for facilitators (`GET /api/facilitators`) and transactions (`GET /api/transactions`) with filtering and pagination. Updated the frontend with dedicated Facilitators and Transactions pages showing real data.

**(2) Reliability Scoring System (Mar 22):** Designed and implemented a weighted composite scoring formula that computes a 0–100 reliability score for each endpoint from four normalized on-chain signals: transaction count (30%), payment volume (25%), unique payer diversity (25%), and recency with exponential decay (20%). Log normalization (`ln(1+x)/ln(1+max)`) prevents high-volume endpoints from dominating. Scores are computed in a PostgreSQL materialized view (`endpoint_scores`), joined into all API queries via LEFT JOIN, and displayed in the frontend via existing UI components. Found and fixed a case-sensitivity bug where Ethereum addresses from the Bazaar API (EIP-55 checksummed, mixed case) didn't match CDP transaction data (lowercase), which initially caused only 5 of 215 endpoints to score.

Compared to the originally planned milestone chart, the project is on track. Week 10's reliability scoring is complete, though implemented via on-chain signals rather than health checks — a pragmatic pivot since we now have real transaction data. Health checks (endpoint probing, 402 response verification) are designed as a future scoring dimension that can be blended in. Week 12's multi-facilitator support is partially done with the CDP SQL API integration. The remaining work is metadata enrichment and final polish.

## Supporting Evidence (Factual)

**GitHub Repository:** https://github.com/yakbas3/agora

**Total:** 82 commits, ~120 files, ~5,200 lines of source code.

**Work Done in the Past 2 Weeks (26 commits since Checkpoint 2):**

### Phase 5: V1 Transaction Indexer (new since Checkpoint 2)
- **CDP SQL API client:** https://github.com/yakbas3/agora/tree/main/internal/cdp — Go client with ED25519 JWT authentication, SQL query builder for Base chain events.
- **Sync runner:** https://github.com/yakbas3/agora/blob/main/internal/sync/runner.go — Orchestrates facilitator-by-facilitator transaction indexing with pagination.
- **Facilitator seeding migration:** https://github.com/yakbas3/agora/blob/main/migrations/000009_seed_facilitators.up.sql — 102 Base facilitator addresses from x402scan registry.
- **Repository methods:** https://github.com/yakbas3/agora/blob/main/internal/database/repository.go — `GetFacilitatorStats`, `GetTransactions`, `InsertTransactions` with batch upsert.
- **API endpoints:** `GET /api/facilitators` (with tx stats), `GET /api/transactions` (with pagination and facilitator filter).
- **Frontend pages:** Facilitators page with grouped stats, Transactions page with filtering and pagination.

### Phase 6: Reliability Scoring (new since Checkpoint 2)
- **Design document:** https://github.com/yakbas3/agora/blob/main/docs/plans/2026-03-22-reliability-scoring-design.md
- **Implementation plan:** https://github.com/yakbas3/agora/blob/main/docs/plans/2026-03-22-reliability-scoring-plan.md
- **Materialized view:** https://github.com/yakbas3/agora/blob/main/migrations/000006_create_endpoint_scores.up.sql — CTE-based query with log-normalized weighted composite formula.
- **Go model + repository:** `ReliabilityScore` field added to Endpoint model, all queries (`GetEndpoints`, `GetEndpointByID`, `SearchByVector`, `GetStats`) updated with LEFT JOIN on `endpoint_scores`.
- **API handlers:** `EndpointJSON` includes `reliability_score`, stats endpoint returns `avg_reliability`.
- **Frontend transforms:** `ApiEndpoint.reliability_score` mapped to `Endpoint.reliabilityScore`, consumed by existing `ReliabilityPulse` and `ReliabilityBar` components.

### Database Stats
| Table | Count |
|-------|-------|
| endpoints | 12,571 |
| payment_options | 12,803 |
| transactions | 148,106 |
| facilitators | 102 |
| scored endpoints (reliability > 0) | 215 |

## Skill Learning Report

**On-Chain Data Indexing via CDP SQL API:** This was the most substantial new learning. I had to understand how Coinbase's Developer Platform SQL API works — it's essentially a SQL interface over blockchain event data. The authentication is non-trivial: you generate ED25519 key pairs, sign JWTs with the private key, and include them as bearer tokens. The actual queries target the `base.events` table filtering by contract address (USDC) and event signature (Transfer), with the `transaction_from` field matching known facilitator addresses. I learned this approach by studying how x402scan.com (the existing community explorer) gets its data — they use the exact same CDP SQL API and facilitator filtering strategy.

**Reliability Scoring Design:** Designing the scoring algorithm taught me about normalization techniques for heterogeneous signals. The key insight was using log normalization (`ln(1+x)/ln(1+max)`) instead of linear normalization — this prevents a single whale endpoint from compressing all other scores to near-zero. I also learned about exponential decay functions for modeling recency (half-life of ~23 days means an endpoint that was active a month ago still gets partial credit). The most educational bug was the Ethereum address casing issue: the Bazaar API returns EIP-55 checksummed addresses (mixed case like `0x1A2B...`) while on-chain data is lowercase (`0x1a2b...`). This caused a silent data loss where the JOIN matched only 5 of 215 endpoints — a reminder that blockchain address comparison must always be case-insensitive.

**PostgreSQL Materialized Views for Analytics:** I used materialized views to pre-compute expensive aggregations. The `endpoint_scores` view joins endpoints → payment_options → transactions with CTEs for normalization, and the `REFRESH MATERIALIZED VIEW CONCURRENTLY` command allows zero-downtime refreshes. This pattern is powerful for dashboard-style analytics where you need fast reads but can tolerate slightly stale data.

## Self-Evaluation

**Scope: 115%.** Since Checkpoint 2, the project has delivered both the V1 transaction indexer and the reliability scoring system — two non-trivial subsystems that significantly deepen AGORA's capabilities beyond a simple search wrapper. The transaction indexer brings real on-chain data into the system (148K transactions), and the reliability scoring turns that data into actionable trust signals. The only remaining planned deliverable is metadata enrichment, which I'm confident I can complete in the final weeks. The scoring implementation took a different approach than originally planned (on-chain signals instead of health checks), but I believe this is better — it's based on real payment activity rather than synthetic probes.

**Match: 110%.** The project is tracking close to the original timeline. Weeks 5–9 were completed on or ahead of schedule (as reported in Checkpoint 2). Week 10's reliability scoring is now complete. Week 12's multi-facilitator support is partially done — the CDP SQL API integration and facilitator seeding are working, though the metadata enrichment piece is deferred. The main deviation from the plan is the approach to reliability scoring (on-chain signals vs. health checks), but this was a data-driven decision after building the transaction indexer gave us real payment data to work with. Health checks remain in the design as a future scoring dimension.

**Factual: 100%.** Everything claimed is verifiable through the GitHub repository. The 148,106 transactions are queryable in the database. The 215 scored endpoints can be confirmed via the API. The scoring formula is visible in the materialized view SQL. All 82 commits have descriptive messages. The production Docker deployment from Checkpoint 2 continues to work.

## LLM-Generated Feedback (Claude Opus 4.6)

*Project: AGORA — Advanced Search and Indexing for x402 Pay-Per-Use Resource Network*
*Student: Yaman Akbas*
*Checkpoint: 3*

**Scope: 115/120**

Strengths: The project now covers the full stack from on-chain data ingestion to user-facing reliability scores. The V1 transaction indexer is a substantial addition — integrating with the CDP SQL API required understanding blockchain event structures, JWT authentication, and facilitator-based filtering. The reliability scoring system demonstrates thoughtful algorithm design with log normalization and exponential decay. The pivot from health checks to on-chain signals was well-reasoned and produced a more grounded scoring system. The EIP-55 case sensitivity bug catch shows attention to real-world data quality issues.

Areas for improvement: The metadata enrichment deliverable remains unimplemented. Additionally, the current reliability scores are only meaningful for the 215 endpoints (1.7%) that have matching on-chain transactions — the health check dimension (planned for a future checkpoint) would extend scoring to all endpoints. The scoring algorithm's reliance on relative normalization means scores will shift as new data comes in, which could be confusing for users expecting stable scores.

**Match: 110/120**

Strengths: The milestone chart shows steady progress with week 10's reliability scoring completed on schedule. The V1 transaction indexer (week 12's multi-facilitator support) was partially delivered ahead of schedule. The pivot from health checks to on-chain scoring signals was a justified adaptation based on available data. 26 commits in the checkpoint period show continuous development.

Areas for improvement: Metadata enrichment (originally week 12) is still unstarted. The project has consistently added bonus work (frontend pages, Docker deployment) which is impressive but has come at the cost of some originally planned features. Completing metadata enrichment in the final checkpoint would fully close the gap with the original proposal.

**Factual: 100/100**

Strengths: All claims are directly verifiable. The GitHub repository contains 82 commits with descriptive messages. Database counts (12,571 endpoints, 148,106 transactions, 102 facilitators, 215 scored endpoints) are queryable. The scoring formula is visible in the committed SQL migration. The design documents and implementation plans provide clear audit trails for technical decisions. The EIP-55 bug fix is documented in commit history.

| Dimension | Score | Notes |
|-----------|-------|-------|
| Scope | 115/120 | Full-stack reliability scoring complete; metadata enrichment still pending |
| Match | 110/120 | On track with timeline; on-chain scoring pivot well-justified |
| Factual | 100/100 | All claims verifiable through repository and database |

Overall assessment: Strong checkpoint. The combination of on-chain data indexing and reliability scoring adds real analytical depth to AGORA. The primary recommendation for the final checkpoint is to deliver metadata enrichment and the health check scoring dimension, as these would extend reliability scores to all 12,571 endpoints rather than just the 215 with on-chain activity.
