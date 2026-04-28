## Context & Research

This file indexes the research documents that informed Agora's design decisions.
The detailed write-ups live in `docs/context/`. Start there when onboarding or
resuming work after a break.

### Research Documents

- **`docs/context/x402-protocol-overview.md`**
  How x402 works end to end. Covers the HTTP 402 payment flow, the role of
  facilitators, and the two settlement versions (V1: EIP-3009 transferWithAuthorization,
  V2: Permit2 proxy contracts). Essential reading for understanding what Agora indexes.

- **`docs/context/x402scan-architecture.md`**
  How x402scan (Merit Systems' open-source explorer) gets its data. Documents their
  CDP SQL API usage, sync flow (5-minute cron per facilitator), facilitator registry,
  and tech stack (Next.js, tRPC, Trigger.dev, Neon Postgres). Agora's CDP sync module
  was directly inspired by this architecture.

- **`docs/context/cdp-sql-api.md`**
  Full reference for the Coinbase Developer Platform SQL API. Covers authentication
  (JWT with ES256), available tables (`base.events`, `base.transactions`, `base.transfers`),
  rate limits (500 req/10s, 100k row max, 30s timeout), example queries for USDC
  transfers, and gotchas (ClickHouse dialect, case sensitivity, GROUP BY timeouts).

- **`docs/context/next-steps.md`**
  Snapshot of where the project stood after Phase 1 (Bazaar crawler) and the failed
  V2 indexer attempt. Documents the decision to build a V1 indexer using CDP SQL API,
  the facilitator-based detection approach, and the brainstorming status at that time.
  Note: much of this has since been implemented.

### Key Insights from Research

1. **V1 detection is simple:** Filter USDC Transfer events where `transaction_from`
   is a known facilitator wallet address. No complex ABI decoding needed.

2. **CDP SQL API is the right data source:** Free, SQL-based, covers all of Base chain.
   Better than Alchemy RPC (which requires log filtering), BaseScan (no Base on free tier),
   or Blockscout (REST pagination complexity).

3. **Facilitator addresses are the linchpin:** Without a facilitator list, you can't
   distinguish x402 payments from regular USDC transfers. x402scan maintains a static
   list of ~20+ addresses in their repo. Agora stores these in the `facilitators` table.

4. **Time-chunked sync is necessary:** The CDP SQL API has scan limits on large queries.
   Agora chunks by month and falls back to weekly windows when limits are hit.

5. **V2 is not worth active investment:** Near-zero on-chain activity as of early 2026.
   The V2 indexer code exists for completeness but finds no transactions.

### Failed Approaches (Do Not Retry)

- **BaseScan/Etherscan for Base** — Free tier does not cover Base. Confirmed Feb 2026.
- **Alchemy RPC for V2 events** — Works technically but V2 has no traffic.
- **Blockscout** — Functional but CDP SQL API is simpler for our use case.
