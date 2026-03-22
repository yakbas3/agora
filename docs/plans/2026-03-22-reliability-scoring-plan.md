# Reliability Scoring Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Compute a 0–100 reliability score for each endpoint from on-chain transaction signals and display it in the frontend.

**Architecture:** Extend the existing `endpoint_scores` materialized view with a weighted composite formula (30% tx_count, 25% volume, 25% unique_payers, 20% recency). Serve scores via existing API endpoints. Frontend UI components already exist — just wire real data.

**Tech Stack:** PostgreSQL (materialized view), Go (API handlers/repository), TypeScript/Next.js (frontend transforms)

---

### Task 1: Update the materialized view migration

**Files:**
- Modify: `migrations/000006_create_endpoint_scores.up.sql`
- Modify: `migrations/000006_create_endpoint_scores.down.sql`

**Step 1: Replace the up migration with the scoring formula**

Replace contents of `migrations/000006_create_endpoint_scores.up.sql` with:

```sql
CREATE MATERIALIZED VIEW endpoint_scores AS
WITH raw AS (
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
    GROUP BY e.id
),
maxes AS (
    SELECT
        MAX(tx_count) AS max_tx,
        MAX(total_volume_usd) AS max_vol,
        MAX(unique_payers) AS max_payers
    FROM raw
    WHERE tx_count > 0
)
SELECT
    r.endpoint_id,
    r.tx_count,
    r.total_volume_usd,
    r.unique_payers,
    r.last_tx_at,
    r.first_tx_at,
    CASE WHEN r.tx_count = 0 THEN 0.0
         ELSE exp(-0.03 * EXTRACT(EPOCH FROM (NOW() - r.last_tx_at)) / 86400.0)
    END AS recency_score,
    CASE WHEN r.tx_count = 0 OR m.max_tx IS NULL THEN 0
         ELSE ROUND((
             0.30 * (ln(1 + r.tx_count) / NULLIF(ln(1 + m.max_tx), 0)) +
             0.25 * (ln(1 + r.total_volume_usd) / NULLIF(ln(1 + m.max_vol), 0)) +
             0.25 * (ln(1 + r.unique_payers) / NULLIF(ln(1 + m.max_payers), 0)) +
             0.20 * exp(-0.03 * EXTRACT(EPOCH FROM (NOW() - r.last_tx_at)) / 86400.0)
         ) * 100)
    END AS reliability_score
FROM raw r
CROSS JOIN maxes m;

CREATE UNIQUE INDEX idx_endpoint_scores_id ON endpoint_scores (endpoint_id);
```

**Step 2: Verify down migration is unchanged**

`migrations/000006_create_endpoint_scores.down.sql` should already contain:
```sql
DROP MATERIALIZED VIEW IF EXISTS endpoint_scores;
```

No changes needed.

**Step 3: Commit**

```bash
git add migrations/000006_create_endpoint_scores.up.sql
git commit -m "feat: add reliability score formula to endpoint_scores materialized view"
```

---

### Task 2: Run migration to apply new view

**Step 1: Drop and recreate the materialized view**

Since materialized views can't be altered in-place, we need to drop and recreate. Connect to the database and run:

```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "DROP MATERIALIZED VIEW IF EXISTS endpoint_scores;"
```

**Step 2: Re-run the migration**

```bash
./agora.exe migrate
```

**Step 3: Refresh the view with current data**

```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "REFRESH MATERIALIZED VIEW CONCURRENTLY endpoint_scores;"
```

**Step 4: Verify scores are computed**

```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT endpoint_id, tx_count, total_volume_usd, reliability_score FROM endpoint_scores WHERE tx_count > 0 ORDER BY reliability_score DESC LIMIT 10;"
```

Expected: rows with `reliability_score` values between 0 and 100.

**Step 5: Verify zero-tx endpoints score 0**

```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT COUNT(*) AS zero_score_count FROM endpoint_scores WHERE reliability_score = 0;"
```

Expected: large number (most endpoints have no transactions).

---

### Task 3: Update Go repository to include reliability scores

**Files:**
- Modify: `internal/database/repository.go`

**Step 1: Add reliability_score to GetEndpoints query**

Change the `GetEndpoints` method to LEFT JOIN on `endpoint_scores` and return the score. Update the query from:

```go
q := `
    SELECT id, resource_url, domain, type, x402_version, description,
           http_method, input_schema, output_schema, raw_metadata,
           last_updated, first_seen, last_crawled
    FROM endpoints
    ORDER BY last_crawled DESC
    LIMIT $1 OFFSET $2
`
```

To:

```go
q := `
    SELECT e.id, e.resource_url, e.domain, e.type, e.x402_version, e.description,
           e.http_method, e.input_schema, e.output_schema, e.raw_metadata,
           e.last_updated, e.first_seen, e.last_crawled,
           COALESCE(es.reliability_score, 0)
    FROM endpoints e
    LEFT JOIN endpoint_scores es ON es.endpoint_id = e.id
    ORDER BY e.last_crawled DESC
    LIMIT $1 OFFSET $2
`
```

Update the scan to include a new `ReliabilityScore` field. This requires adding the field to the Endpoint model first (see step 2).

**Step 2: Add ReliabilityScore to the Endpoint model**

In `internal/models/endpoint.go`, add:

```go
ReliabilityScore float64 `db:"reliability_score" json:"reliability_score"`
```

after the `Embedding` field.

**Step 3: Update all scans in repository.go**

Every place that scans an `Endpoint` struct needs to also scan `reliability_score`. Update these methods:

1. **`GetEndpoints`** — add `&e.ReliabilityScore` to the Scan call and `COALESCE(es.reliability_score, 0)` to the query with the LEFT JOIN
2. **`GetEndpointByID`** — add LEFT JOIN on `endpoint_scores` to the endpoint query, add `COALESCE(es.reliability_score, 0)` to SELECT, add `&e.ReliabilityScore` to Scan
3. **`SearchByVector`** — add LEFT JOIN and score to `buildSearchQuery`, add `&sr.Endpoint.ReliabilityScore` to Scan

For **`GetEndpoints`**, the full updated scan:
```go
err := rows.Scan(
    &e.ID, &e.ResourceURL, &e.Domain, &e.Type, &e.X402Version,
    &e.Description, &e.HTTPMethod, &e.InputSchema, &e.OutputSchema,
    &e.RawMetadata, &e.LastUpdated, &e.FirstSeen, &e.LastCrawled,
    &e.ReliabilityScore,
)
```

For **`GetEndpointByID`**, update the query:
```go
eq := `
    SELECT e.id, e.resource_url, e.domain, e.type, e.x402_version, e.description,
           e.http_method, e.input_schema, e.output_schema, e.raw_metadata,
           e.last_updated, e.first_seen, e.last_crawled,
           COALESCE(es.reliability_score, 0)
    FROM endpoints e
    LEFT JOIN endpoint_scores es ON es.endpoint_id = e.id
    WHERE e.id = $1
`
```
And the scan:
```go
err := r.pool.QueryRow(ctx, eq, id).Scan(
    &e.ID, &e.ResourceURL, &e.Domain, &e.Type, &e.X402Version,
    &e.Description, &e.HTTPMethod, &e.InputSchema, &e.OutputSchema,
    &e.RawMetadata, &e.LastUpdated, &e.FirstSeen, &e.LastCrawled,
    &e.ReliabilityScore,
)
```

For **`buildSearchQuery`**, update the SELECT in the query template:
```go
q := fmt.Sprintf(`
    SELECT DISTINCT ON (e.id)
        e.id, e.resource_url, e.domain, e.type, e.x402_version,
        e.description, e.http_method, e.input_schema, e.output_schema,
        e.raw_metadata, e.last_updated, e.first_seen, e.last_crawled,
        1 - (e.embedding <=> $1) AS similarity,
        COALESCE(es.reliability_score, 0) AS reliability_score
    FROM endpoints e
    LEFT JOIN endpoint_scores es ON es.endpoint_id = e.id
    %s
    %s
    ORDER BY e.id, similarity DESC
`, joins, where)
```

And update the `SearchByVector` scan:
```go
err := rows.Scan(
    &sr.Endpoint.ID, &sr.Endpoint.ResourceURL, &sr.Endpoint.Domain,
    &sr.Endpoint.Type, &sr.Endpoint.X402Version, &sr.Endpoint.Description,
    &sr.Endpoint.HTTPMethod, &sr.Endpoint.InputSchema, &sr.Endpoint.OutputSchema,
    &sr.Endpoint.RawMetadata, &sr.Endpoint.LastUpdated, &sr.Endpoint.FirstSeen,
    &sr.Endpoint.LastCrawled, &sr.Similarity, &sr.Endpoint.ReliabilityScore,
)
```

**Step 4: Add avg_reliability to StatsResult**

In the `StatsResult` struct, add:
```go
AvgReliability float64 `json:"avg_reliability"`
```

In `GetStats`, after the transaction totals query, add:
```go
r.pool.QueryRow(ctx,
    `SELECT COALESCE(AVG(reliability_score), 0) FROM endpoint_scores WHERE reliability_score > 0`,
).Scan(&s.AvgReliability)
```

**Step 5: Build and verify**

```bash
go build ./...
```

Expected: no compilation errors.

**Step 6: Commit**

```bash
git add internal/models/endpoint.go internal/database/repository.go
git commit -m "feat: serve reliability scores from endpoint_scores view in all API queries"
```

---

### Task 4: Update API handlers to include reliability score in responses

**Files:**
- Modify: `internal/api/handlers.go`

**Step 1: Add ReliabilityScore to EndpointJSON**

In the `EndpointJSON` struct, add:
```go
ReliabilityScore float64 `json:"reliability_score"`
```

**Step 2: Update handleSearch to pass the score**

In `handleSearch`, where `EndpointJSON` is constructed in the loop, add:
```go
ReliabilityScore: sr.Endpoint.ReliabilityScore,
```

**Step 3: Build and verify**

```bash
go build ./...
```

**Step 4: Commit**

```bash
git add internal/api/handlers.go
git commit -m "feat: include reliability_score in API endpoint responses"
```

---

### Task 5: Update frontend types and transforms

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/transforms.ts`

**Step 1: Add reliability_score to ApiEndpoint**

In `web/src/lib/api.ts`, add to the `ApiEndpoint` interface:
```typescript
reliability_score?: number;
```

**Step 2: Add avg_reliability to ApiStats**

In `web/src/lib/api.ts`, add to the `ApiStats` interface:
```typescript
avg_reliability: number;
```

**Step 3: Update transformEndpoint to pass reliability score**

In `web/src/lib/transforms.ts`, the `transformEndpoint` function's parameter type needs `reliability_score?: number` added, and the return object needs:
```typescript
reliabilityScore: api.reliability_score,
```

**Step 4: Update transformSearchResult to pass reliability score**

The `transformSearchResult` function constructs an Endpoint from `ApiSearchResult`. The `ApiSearchResult.endpoint` already includes the `ApiEndpoint` type which now has `reliability_score`. No extra work needed since `transformEndpoint` handles it.

**Step 5: Commit**

```bash
git add web/src/lib/api.ts web/src/lib/transforms.ts
git commit -m "feat: wire reliability score through frontend API types and transforms"
```

---

### Task 6: Verify end-to-end

**Step 1: Start the backend**

```bash
./agora.exe serve
```

**Step 2: Test the endpoints API**

```bash
curl http://localhost:8080/api/endpoints?limit=5 | jq '.[0].endpoint.reliability_score'
```

Expected: a number between 0 and 100.

**Step 3: Test the stats API**

```bash
curl http://localhost:8080/api/stats | jq '.avg_reliability'
```

Expected: a number > 0.

**Step 4: Start the frontend**

```bash
cd web && npm run dev
```

Open http://localhost:3000 — endpoints should show real reliability pulses and bars.

**Step 5: Commit any fixes if needed**

---

### Task 7: Clean up dummy data (optional)

**Files:**
- Delete: `web/src/lib/dummy-data.ts` (not imported anywhere)

**Step 1: Verify dummy-data.ts is unused**

```bash
grep -r "dummy-data" web/src/
```

Expected: no results (already confirmed).

**Step 2: Delete the file**

```bash
rm web/src/lib/dummy-data.ts
```

**Step 3: Commit**

```bash
git add -u web/src/lib/dummy-data.ts
git commit -m "chore: remove unused dummy data file"
```
