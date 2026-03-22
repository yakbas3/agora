# Frontend Integration Design

**Date:** 2026-03-07
**Scope:** Endpoints page + Network page wired to real Go API data

## Decisions

- **Pages in scope:** Endpoints (`/`) and Network (`/network`). Facilitators page stays on dummy data.
- **Search behavior:** Hybrid — paginated list on load, semantic search when query is entered, back to list when cleared.
- **Network stats:** Show all aggregations including time series and crawl history (even if sparse).
- **Frontend-to-API:** Direct client-side fetch with `NEXT_PUBLIC_API_URL` env var. CORS already handled.

## Backend Changes

### New endpoint: `GET /api/stats`

Returns all network-wide aggregations:

```json
{
  "total_endpoints": 12571,
  "total_domains": 847,
  "endpoints_by_network": [{"network": "base", "count": 9842}],
  "endpoints_by_asset": [{"asset": "USDC", "count": 10234}],
  "endpoints_by_price_bracket": [{"bracket": "$0-0.001", "count": 4821}],
  "endpoints_over_time": [{"date": "2026-02-01", "count": 12100}],
  "crawl_history": [{"id": "...", "started_at": "...", "status": "completed"}]
}
```

SQL queries:
- `COUNT(*)` from endpoints, `COUNT(DISTINCT domain)` from endpoints
- `GROUP BY network_normalized` from payment_options
- `GROUP BY asset_name` from payment_options
- `CASE/WHEN` on `price_usd` for price brackets from payment_options
- `GROUP BY DATE(first_seen)` from endpoints for time series
- `SELECT *` from crawl_runs ordered by started_at DESC

### Modify: `GET /api/endpoints`

Include payment options inline (currently returns endpoints without them). The frontend table needs network/asset/price from payment options.

Response becomes array of:
```json
{
  "id": "...",
  "resource_url": "...",
  "payment_options": [{"network_normalized": "base", "asset_name": "USDC", "price_usd": 0.002}]
}
```

## Frontend Changes

### API client (`web/src/lib/api.ts`)

- `API_URL` from `NEXT_PUBLIC_API_URL` (default `http://localhost:8080`)
- `fetchEndpoints(limit, offset)` → `GET /api/endpoints`
- `searchEndpoints(query, filters)` → `POST /api/search`
- `fetchStats()` → `GET /api/stats`

### Types (`web/src/lib/types.ts`)

- Update to match API response shapes
- `reliabilityScore` and `reliabilityTrend` become optional (API doesn't have them yet)
- Components show placeholder when reliability data is missing

### Endpoints page (`/`)

- On mount: `GET /api/endpoints?limit=20`
- On search (300ms debounce): `POST /api/search`
- On clear: back to paginated list
- Filter chips map to search API filters
- Loading state while fetching

### Network page (`/network`)

- On mount: `GET /api/stats`
- Feed real data into existing chart components
- Loading state while fetching
