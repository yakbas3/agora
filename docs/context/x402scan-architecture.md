# x402scan — How It Works

## Overview

x402scan (https://x402scan.com) is an open-source ecosystem explorer for x402 payments, built by Merit Systems. Repo: https://github.com/Merit-Systems/x402scan (Apache 2.0, TypeScript).

## How They Get Their Data

x402scan tracks x402 payments by monitoring **USDC Transfer events where `transaction_from` is a known facilitator address**. That's the core insight — they don't watch for fancy contract events, they just filter standard ERC-20 Transfers by who submitted the transaction.

### Data Source: Coinbase CDP SQL API (for Base chain)

Primary data source is the Coinbase Developer Platform SQL API — a REST endpoint where you run SQL queries against indexed Base blockchain data.

**Endpoint:** `POST https://api.cdp.coinbase.com/platform/v2/data/query/run`

**The actual query they run:**
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
  AND address = '<usdc_contract_address>'
  AND transaction_from = '<facilitator_address>'
  AND block_timestamp >= '<since_last_sync>'
  AND block_timestamp < '<now>'
ORDER BY block_timestamp DESC
LIMIT 10000
OFFSET <offset>;
```

### Sync Flow (every 5 minutes)

1. Cron fires (Trigger.dev scheduled task)
2. For each facilitator address, find the last synced timestamp from DB
3. Query CDP SQL API for USDC Transfers by that facilitator since last sync
4. Batch insert into PostgreSQL with `skipDuplicates` (idempotent)
5. Repeat for next facilitator

### Other Data Providers (not Base)
- **Google BigQuery** — for Solana
- **BitQuery GraphQL** — for Polygon

### Facilitator Registry

Static list of ~20+ facilitator wallet addresses maintained in `packages/facilitators/`. Each entry has: name, address, chain, firstTransactionDate. New facilitators added manually via PRs.

## What They Display

| Page | Data |
|------|------|
| Home | Total txns, volume, top servers, top facilitators, top AI agents, latest txns |
| /facilitators | Ranked facilitators with volume charts |
| /facilitator/[id] | Individual facilitator stats + transaction history |
| /transactions | Searchable/filterable transaction list |
| /resources | Registered x402 API endpoints |
| /networks | Per-chain statistics |

## Tech Stack

- **Frontend:** Next.js (App Router), React, Vercel
- **API:** tRPC (type-safe RPC)
- **Background Jobs:** Trigger.dev (cron-based sync)
- **Database:** PostgreSQL via Neon + Prisma ORM
- **Auth:** NextAuth + SIWE (Sign-In With Ethereum)
- **Monorepo:** pnpm workspaces + Turborepo

## Data Model (key fields per transfer)

| Field | Description |
|-------|------------|
| tx_hash | On-chain transaction hash |
| transaction_from | Facilitator address (gas payer) |
| sender | Who sent USDC |
| recipient | Who received USDC |
| amount | USDC amount (6 decimals) |
| block_timestamp | When block was mined |
| chain | "base", "polygon", "solana" |
| facilitator_id | Links to facilitator record |
| log_index | Position within transaction |
