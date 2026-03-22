# Reliability Scoring Design

**Date:** 2026-03-22
**Status:** Approved

## Overview

Compute a 0–100 reliability score for each endpoint based on on-chain transaction signals. Scores are computed entirely in PostgreSQL via the existing `endpoint_scores` materialized view. Frontend UI components (`ReliabilityPulse`, `ReliabilityBar`) already exist and will be wired to real data.

A future checkpoint will add active health checks (endpoint probing) as a second scoring dimension.

## Scoring Algorithm

Weighted composite of 4 normalized signals:

| Signal | Weight | Normalization |
|--------|--------|---------------|
| `tx_count` | 30% | `ln(1 + tx_count) / ln(1 + max_tx_count)` |
| `total_volume_usd` | 25% | `ln(1 + volume) / ln(1 + max_volume)` |
| `unique_payers` | 25% | `ln(1 + payers) / ln(1 + max_payers)` |
| `recency` | 20% | `exp(-0.03 * days_since_last_tx)` — half-life ~23 days |

**Formula:**
```
reliability_score = round((0.30 * tx_norm + 0.25 * vol_norm + 0.25 * payer_norm + 0.20 * recency_norm) * 100)
```

Log normalization prevents whale endpoints from dominating. Endpoints with zero transactions score 0.

## Database Changes

Replace the existing `endpoint_scores` materialized view with an updated version that adds:

- `reliability_score` (numeric, 0–100) — the weighted composite
- `recency_score` (numeric, 0–1) — the exponential decay component

No new tables. The existing `RefreshEndpointScores()` Go method continues to work unchanged — the view just returns more columns.

Endpoints with no matching transactions get NULL (treated as 0 when joined).

## API Changes

No new routes. Existing endpoints gain reliability data:

- `GET /api/endpoints` — each endpoint object gains `reliabilityScore` (0–100)
- `GET /api/endpoints/{id}` — same
- `POST /api/search` — same
- `GET /api/stats` — add `avg_reliability` (network-wide average)
- `GET /api/facilitators` — compute `avgReliability` per facilitator (average of their endpoints' scores)

`ApiEndpoint` struct gets a `ReliabilityScore *float64` field. Repository queries LEFT JOIN on the refreshed `endpoint_scores` view.

## Frontend Wiring

Minimal changes — UI components already exist:

- Map `reliabilityScore` from API response to the existing optional `Endpoint.reliabilityScore` field
- `ReliabilityPulse` and `ReliabilityBar` already consume the score — no component changes
- `EndpointsTable` already renders both components
- Facilitator card: wire `avgReliability`, derive `status` from score bands (>70% healthy, 30–70% degraded, <30% inactive)
- Remove any remaining hardcoded dummy reliability data

## Future Extension: Active Health Checks

Not built now — documented to avoid design conflicts:

- Background worker probes endpoint URLs periodically
- Measures latency, checks for proper 402 responses, tracks uptime
- New `endpoint_health_checks` table stores probe history
- Reliability score becomes a weighted blend of on-chain signals + health check results
- The materialized view approach extends naturally with additional signals
