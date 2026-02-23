# Transaction Indexer & Endpoint Scoring Design

**Date:** 2026-02-23
**Status:** Approved

## Problem

Our Bazaar crawl captured 12,571 endpoints, but many are registered-but-never-used. The Bazaar is just one facilitator's (Coinbase CDP) catalog — 18 other facilitators exist. To find the *most useful* endpoints, we need to track actual on-chain money flow and score endpoints by real usage.

## Goals

1. **Index on-chain x402 settlements** on Base into a `transactions` table
2. **Score existing endpoints** by transaction volume, unique payers, and recency
3. **Discover new sellers** — addresses receiving x402 payments but not listed in Bazaar

## Approach: API-Based Transaction Indexer

Use direct `eth_getLogs` via Alchemy free tier to query x402 settlement events on Base. Process in 2K-block windows with 2 log queries per window (Settled events + USDC Transfers), joined in memory by tx hash.

### On-Chain Event Identification

x402 settlements on Base happen through two paths:

**Permit2 path (V2):** The facilitator calls `settle()` on proxy contracts, which emit bare `Settled()` or `SettledWithPermit()` marker events. The USDC `Transfer(from, to, value)` event in the same transaction contains payment details.

- x402ExactPermit2Proxy: `0x4020615294c913F045dc10f0a5cdEbd86c280001`
- x402UptoPermit2Proxy: `0x4020633461b2895a48930Ff97eE8fCdE8E520002`

**EIP-3009 path (V1):** The facilitator calls `transferWithAuthorization()` on USDC, emitting `AuthorizationUsed(address indexed authorizer, bytes32 indexed nonce)` plus `Transfer`. Filter by `tx.from` being a known facilitator address.

### Linking Transactions to Endpoints

The `recipient_address` in a transaction (from USDC `Transfer.to`) matches the `pay_to` field in our `payment_options` table. This JOIN gives us per-endpoint usage metrics.

## Data Model

### New Table: `transactions`

| Column | Type | Source |
|--------|------|--------|
| `id` | UUID PK | generated |
| `tx_hash` | TEXT UNIQUE | from event log |
| `block_number` | BIGINT | from event log |
| `block_time` | TIMESTAMPTZ | from block timestamp |
| `event_type` | TEXT | "settled", "settled_with_permit", "authorization_used" |
| `proxy_contract` | TEXT nullable | which proxy emitted Settled, NULL for EIP-3009 |
| `facilitator_address` | TEXT | `tx.from` (gas payer) |
| `payer_address` | TEXT | USDC Transfer `from` |
| `recipient_address` | TEXT | USDC Transfer `to` (matches pay_to) |
| `amount_raw` | TEXT | raw uint256 |
| `amount_usd` | NUMERIC | amount_raw / 10^6 |
| `asset_address` | TEXT | USDC contract |
| `indexed_at` | TIMESTAMPTZ | when stored |

Indexes: `recipient_address`, `block_number`, `facilitator_address`, `block_time`.

### New Table: `indexer_state`

| Column | Type | Purpose |
|--------|------|---------|
| `id` | INTEGER PK | always 1 |
| `last_block` | BIGINT | last fully indexed block |
| `updated_at` | TIMESTAMPTZ | when updated |

### New Materialized View: `endpoint_scores`

```sql
CREATE MATERIALIZED VIEW endpoint_scores AS
SELECT
    e.id AS endpoint_id,
    COUNT(t.id) AS tx_count,
    COALESCE(SUM(t.amount_usd), 0) AS total_volume_usd,
    COUNT(DISTINCT t.payer_address) AS unique_payers,
    MAX(t.block_time) AS last_tx_at,
    MIN(t.block_time) AS first_tx_at
FROM endpoints e
LEFT JOIN payment_options po ON po.endpoint_id = e.id
LEFT JOIN transactions t ON t.recipient_address = po.pay_to
GROUP BY e.id;
```

### New Table: `discovered_sellers`

| Column | Type |
|--------|------|
| `pay_to` | TEXT UNIQUE |
| `tx_count` | INTEGER |
| `total_volume_usd` | NUMERIC |
| `unique_payers` | INTEGER |
| `first_seen_at` | TIMESTAMPTZ |
| `last_seen_at` | TIMESTAMPTZ |
| `matched_endpoint_id` | UUID nullable |

Populated by aggregating `transactions.recipient_address` values not matching any `payment_options.pay_to`.

## Architecture

### New Package: `internal/indexer/`

- **`client.go`** — Wraps `go-ethereum/ethclient`. Methods: `FetchSettledEvents(fromBlock, toBlock)`, `FetchUSDCTransfers(fromBlock, toBlock)`, `GetCurrentBlock()`.
- **`decoder.go`** — Parses raw event logs into domain types. Matches Settled events with USDC Transfers by tx hash.
- **`facilitators.go`** — Hardcoded known facilitator addresses on Base (Coinbase CDP ~11 addresses) and proxy contract addresses.
- **`runner.go`** — Block-range windowed indexing loop: read last_block → process 2K-block windows → store transactions → update state → refresh scores.
- **`types.go`** — `Transaction` domain type for decoded on-chain data.

### CLI Addition

New `index` command in `cmd/agora/main.go`:
```bash
./agora.exe index   # indexes from last_block to chain head
```

### Config Additions

```
BASE_RPC_URL          (required, e.g., https://base-mainnet.g.alchemy.com/v2/KEY)
INDEXER_BLOCK_RANGE   (default: 2000)
INDEXER_START_BLOCK   (default: 25000000)
```

## Indexer Flow

```
Start
  → Read last_block from indexer_state (or INDEXER_START_BLOCK if first run)
  → Get current chain head from RPC
  → For each 2K-block window [fromBlock, toBlock]:
      → eth_getLogs: Settled/SettledWithPermit from proxy contracts
      → eth_getLogs: USDC Transfer events in same block range
      → Join in memory by tx_hash
      → Batch INSERT matched transactions
      → UPDATE indexer_state.last_block = toBlock
  → REFRESH MATERIALIZED VIEW endpoint_scores
  → INSERT/UPDATE discovered_sellers from unmatched recipients
  → Log summary stats
```

## Cost Estimate (Alchemy Free Tier: 30M CU/month)

| Scenario | Blocks | Windows | API Calls | CU |
|----------|--------|---------|-----------|-----|
| Full backfill (25M→30M) | 5,000,000 | 2,500 | 5,000 | ~375K (1.25%) |
| Daily ongoing | ~43,200 | ~22 | ~44 | ~3.3K (0.01%) |
| Monthly ongoing | ~1.3M | ~650 | ~1,300 | ~100K (0.33%) |

## Known Facilitator Addresses (Base)

Source: facilitators.x402.watch, Coinbase CDP docs.

Hardcoded in `facilitators.go`. Can be expanded later.

### Proxy Contracts
- `0x4020615294c913F045dc10f0a5cdEbd86c280001` (ExactPermit2Proxy)
- `0x4020633461b2895a48930Ff97eE8fCdE8E520002` (UptoPermit2Proxy)

### USDC on Base
- `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913`

## New Dependency

- `github.com/ethereum/go-ethereum` — for ethclient, types, crypto, common, abi

## File Changes Summary

**New files:**
- `internal/indexer/client.go`
- `internal/indexer/decoder.go`
- `internal/indexer/facilitators.go`
- `internal/indexer/runner.go`
- `internal/indexer/types.go`
- `migrations/000002_add_transactions.up.sql`
- `migrations/000002_add_transactions.down.sql`

**Modified files:**
- `cmd/agora/main.go` — add `index` command
- `internal/config/config.go` — add `BASE_RPC_URL`, `INDEXER_*` fields
- `internal/database/repository.go` — add transaction insert, indexer_state CRUD, score refresh
- `.env.example` — add new env vars

## Success Criteria

1. `./agora.exe index` backfills transactions from block 25M to current head
2. `endpoint_scores` view shows per-endpoint tx_count, volume, unique_payers
3. `discovered_sellers` table captures pay_to addresses not in Bazaar
4. Subsequent `index` runs pick up from last_block incrementally
5. Full backfill completes within Alchemy free tier limits
