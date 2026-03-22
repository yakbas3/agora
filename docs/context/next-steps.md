# Next Steps — V1 Transaction Indexer

## Where We Left Off (Feb 23, 2026)

We completed Phase 1 (Bazaar crawler) and attempted a V2 indexer using Alchemy RPC to watch for Settled/SettledWithPermit events on proxy contracts. It found 0 transactions because V2 is barely used.

After researching x402scan, we now understand the correct approach: track V1 payments by monitoring USDC Transfer events where `transaction_from` is a known facilitator address, using the Coinbase CDP SQL API.

## What Already Exists in Our Codebase

- **V2 indexer code** in `internal/indexer/` — client.go, decoder.go, facilitators.go, runner.go. Keep this, don't delete.
- **Bazaar crawler** in `internal/crawler/` — fully working, 12,571 endpoints crawled
- **PostgreSQL** with pgvector on port 5433

## The Plan: Build a V1 Indexer

### Approach (imitating x402scan)

1. **Use CDP SQL API** instead of Alchemy RPC — query `base.events` table for USDC Transfers by facilitator
2. **Maintain a facilitator address list** — pull from x402scan's open-source list (~20+ addresses)
3. **Timestamp-based sync** — store last sync time per facilitator, poll forward every N minutes
4. **Store in our PostgreSQL** — new table for transfer events with dedup on (tx_hash, log_index)

### What We Need

- [ ] CDP API key (free, from portal.cdp.coinbase.com)
- [ ] Facilitator addresses (from x402scan repo: github.com/Merit-Systems/x402scan packages/facilitators/)
- [ ] New migration for transfer_events table
- [ ] CDP SQL API client in Go (JWT auth + query execution)
- [ ] Sync runner (poll per facilitator, timestamp bookmark, batch insert)
- [ ] CLI command: `./agora.exe sync` or similar

### Brainstorming Status

We were in the middle of the brainstorming skill flow:
- [x] Explored x402scan architecture
- [x] Understood CDP SQL API
- [x] User tested CDP Playground (confirmed working)
- [ ] Propose 2-3 approaches with trade-offs
- [ ] Present design for user approval
- [ ] Write design doc
- [ ] Create implementation plan

### Key Decision: What to do with existing V2 indexer

Keep it. Don't delete. The V2 code in `internal/indexer/` stays as-is. The new V1 sync code can live alongside it, possibly in `internal/sync/` or `internal/v1indexer/`.

## Failed Approaches (Don't Retry)

- **BaseScan/Etherscan API for Base** — free tier does NOT cover Base chain. Confirmed with two API calls. Don't waste time on this.
- **Alchemy RPC for V2 events** — works but V2 has no traffic. Keep code but it won't find transactions.
- **Blockscout** — works and is free, but CDP SQL API is simpler (SQL vs REST pagination).
