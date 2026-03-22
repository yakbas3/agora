# Coinbase CDP SQL API — Reference

## What Is It?

A REST API from Coinbase that lets you run SQL queries against indexed Base blockchain data. Think of it as "the entire Base blockchain in a SQL database." No need for Alchemy RPC or BaseScan — just write SQL.

## Access

- **SQL Playground (no key needed):** https://portal.cdp.coinbase.com/products/data/playground
- **REST API (needs free key):** https://portal.cdp.coinbase.com/ → API Keys → Create Secret API Key
- **Requires a Coinbase account** (free)

## Pricing

**Free.** No paid tier mentioned for the SQL API. Suitable for course projects.

## Rate Limits

| Limit | Value |
|-------|-------|
| POST requests | 500 per 10 seconds (~50/sec) |
| Max rows per query | 100,000 |
| Query timeout | 30 seconds |
| Max query length | 100,000 characters |

## Available Tables (Base chain only)

| Table | Description |
|-------|------------|
| `base.events` | Every decoded event log with parameters |
| `base.transactions` | All transactions with gas, from/to |
| `base.blocks` | Block metadata |
| `base.transfers` | Pre-filtered Transfer events (ERC-20, 721, 1155) |
| `base.encoded_logs` | Raw undecoded logs |

## Key Columns in `base.events`

| Column | Description |
|--------|------------|
| `event_signature` | e.g. `Transfer(address,address,uint256)` |
| `address` | Contract that emitted the event |
| `transaction_from` | Address that submitted the tx (gas payer) |
| `transaction_hash` | Tx hash |
| `block_timestamp` | When block was mined |
| `parameters` | Map of decoded event args, accessed via `parameters['from']`, `parameters['to']`, etc. |
| `log_index` | Log position within transaction |
| `block_number` | Block number |

## Authentication

Every REST API request needs a JWT:
1. Take your `CDP_API_KEY_ID` and `CDP_API_KEY_SECRET`
2. Create JWT with: sub=key_id, iss="cdp", aud=["cdp_service"], exp=now+120s
3. Sign with ES256 algorithm
4. Send as: `Authorization: Bearer <jwt>`

## REST API Usage

```
POST https://api.cdp.coinbase.com/platform/v2/data/query/run
Content-Type: application/json
Authorization: Bearer <jwt>

{"sql": "SELECT * FROM base.events WHERE event_signature = 'Transfer(address,address,uint256)' AND address = '0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913' AND block_timestamp >= '2026-02-23' LIMIT 10"}
```

## Example Queries

**Recent USDC transfers on Base:**
```sql
SELECT
  parameters['from'] AS sender,
  parameters['to'] AS recipient,
  parameters['value'] AS amount,
  transaction_from AS gas_payer,
  transaction_hash,
  block_timestamp
FROM base.events
WHERE event_signature = 'Transfer(address,address,uint256)'
  AND address = '0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913'
  AND block_timestamp >= '2026-02-23'
LIMIT 10
```

**x402 payments by a specific facilitator:**
```sql
SELECT *
FROM base.events
WHERE event_signature = 'Transfer(address,address,uint256)'
  AND address = '0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913'
  AND transaction_from = '0xFACILITATOR_ADDRESS_HERE'
  AND block_timestamp >= '2025-06-01'
LIMIT 100
```

## Gotchas

- **Base chain only** — no Ethereum, Solana, Polygon support in SQL API
- **ClickHouse SQL dialect** — read-only SELECT only, no DDL/DML
- **GROUP BY on large tables can timeout** (30s limit) — always add time filters
- **Address case sensitivity** — use `lower()` if unsure about case
- **Tested and confirmed working** as of Feb 2026 in the SQL Playground

## Documentation

- SQL API Welcome: https://docs.cdp.coinbase.com/data/sql-api/welcome
- SQL API Quickstart: https://docs.cdp.coinbase.com/data/sql-api/quickstart
- API Reference: https://docs.cdp.coinbase.com/api-reference/v2/rest-api/sql-api/run-sql-query
- Rate Limits: https://docs.cdp.coinbase.com/api-reference/v2/rate-limits
- API Key Setup: https://docs.cdp.coinbase.com/get-started/docs/cdp-api-keys/
