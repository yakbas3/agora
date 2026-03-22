# V1 Transaction Indexer — Design

## Goal

Index real x402 V1 payment transactions from Base chain using the Coinbase CDP SQL API. Surface this data through API endpoints and the frontend to enable transaction browsing, facilitator analytics, and endpoint reliability scores backed by on-chain data.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Primary goal | Both endpoint scores + transaction explorer | Marginal effort is small once transactions are stored |
| Facilitator storage | Database table | Queryable for frontend, joinable for aggregation |
| Sync strategy | Timestamp-based per facilitator | Matches x402scan's proven approach, independent progress per facilitator |
| Sync execution | CLI command (`./agora.exe sync`), single pass | Simple, testable, no daemon needed for course project |
| Schema approach | Reuse existing `transactions` table as-is | All needed columns exist; `endpoint_scores` view works immediately |
| Frontend | Real facilitator data + new transactions page | Replace dummy data, add browsable transaction history |

## Database Changes

### New Migration: `facilitators` table

```sql
CREATE TABLE facilitators (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            TEXT NOT NULL,
    chain           TEXT NOT NULL,
    address         TEXT NOT NULL,
    last_synced_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_facilitators_chain_address ON facilitators (chain, lower(address));
```

- Seeded with 102 Base addresses from 26 known facilitators (sourced from x402scan registry)
- `last_synced_at` = NULL means never synced; sync runner reads this per facilitator
- Solana/Polygon addresses stored for future use but only Base is synced (CDP API limitation)

### Reused As-Is

- `transactions` table (migration 004) — V1 rows use `event_type` = "Transfer", `proxy_contract` = NULL
- `endpoint_scores` materialized view (migration 006) — joins `transactions.recipient_address` to `payment_options.pay_to`
- `indexer_state` table (migration 005) — unused by V1 sync but kept for V2 indexer

## CDP SQL API Client (`internal/cdp/`)

### `client.go`

- Generates short-lived JWTs (ES256, 120s expiry) signed with `CDP_API_KEY_SECRET`
- Method: `QueryTransfers(facilitatorAddress, since, until) -> []Transfer`
- SQL query (same as x402scan):

```sql
SELECT
  address AS contract_address,
  parameters['from']::String AS sender,
  transaction_from,
  parameters['to']::String AS to_address,
  transaction_hash,
  block_timestamp,
  parameters['value']::UInt256 AS amount,
  log_index
FROM base.events
WHERE event_signature = 'Transfer(address,address,uint256)'
  AND address = '0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913'
  AND transaction_from = '<facilitator_address>'
  AND block_timestamp >= '<since>'
  AND block_timestamp < '<until>'
ORDER BY block_timestamp DESC
LIMIT 10000
OFFSET <offset>;
```

- Handles pagination via OFFSET when results exceed 10,000 rows
- USDC contract address on Base: `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913` (hardcoded constant)

### `types.go`

- CDP API response structs
- `Transfer` struct mapping CDP columns to Go fields

### Config Additions

- `CDP_API_KEY_ID` — API key identifier
- `CDP_API_KEY_SECRET` — ES256 private key for JWT signing

## Sync Runner (`internal/sync/`)

### `runner.go`

1. Load all facilitators where `chain = 'base'` from DB
2. For each facilitator:
   - Read `last_synced_at` (NULL -> default to `2024-06-01`, when x402 launched)
   - Query CDP API for USDC Transfers from `last_synced_at` to now
   - Convert `cdp.Transfer` -> `models.Transaction`:
     - `event_type` = "Transfer"
     - `proxy_contract` = NULL
     - `payer_address` = sender from event parameters
     - `amount_usd` = amount_raw / 1,000,000 (USDC has 6 decimals)
     - `block_number` extracted from CDP response
   - Batch INSERT with ON CONFLICT (tx_hash) DO NOTHING
   - Update facilitator's `last_synced_at`
3. Refresh `endpoint_scores` materialized view
4. Log summary: total new transactions, per-facilitator counts

**Error handling:** If one facilitator fails, log the error and continue to the next.

### CLI Command

`./agora.exe sync` — runs one full sync pass, then exits.

## API Endpoints

### `GET /api/transactions`

- Query params: `limit` (default 50), `offset`, `facilitator`, `recipient`
- Returns: array of transactions with facilitator name joined
- Sorted by `block_time DESC`

### `GET /api/facilitators`

- Returns: array of facilitators with aggregated stats (tx count, total volume, unique payers, last active)
- Computed via JOIN to `transactions` table, GROUP BY facilitator

### `GET /api/stats` (enhanced)

- Add: total transactions, total volume USD, transactions over time

## Frontend

### Facilitators Page (`/facilitators`) — Enhanced

- Replace dummy data with `GET /api/facilitators`
- Each card: name, address, tx count, total volume USD, unique payers, last active
- Real sparkline/chart data from transaction history

### Transactions Page (`/transactions`) — New

- Table: tx hash (linked to BaseScan), facilitator name, sender, recipient, amount USD, timestamp
- Filter by facilitator (dropdown)
- Pagination (limit/offset)

### Endpoints Table — Enhanced

- Show tx count / volume badge from `endpoint_scores` data

### Nav

- Add "Transactions" link

## Data Flow

```
CDP SQL API
    ↓ (query USDC Transfers by facilitator)
internal/cdp/client.go
    ↓ ([]Transfer)
internal/sync/runner.go
    ↓ (convert to []Transaction, batch insert)
PostgreSQL transactions table
    ↓ (materialized view refresh)
endpoint_scores view
    ↓ (API queries)
internal/api/handlers.go
    ↓ (JSON responses)
Next.js frontend
```
