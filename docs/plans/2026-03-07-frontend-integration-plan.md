# Frontend Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire the Next.js web frontend to the real Go API backend, replacing dummy data with live data from 12,571 crawled endpoints.

**Architecture:** The Next.js app (port 3000) fetches directly from the Go API (port 8080) using client-side fetch calls. CORS is already configured. We add a `GET /api/stats` endpoint for network aggregations, modify `GET /api/endpoints` to include payment options inline, create a TypeScript API client, and update the Endpoints and Network pages to use real data.

**Tech Stack:** Go 1.22+ (net/http), Next.js 16, React 19, TypeScript, Recharts, Tailwind CSS 4

---

### Task 1: Add `GetStats` repository method

**Files:**
- Modify: `internal/database/repository.go` (append after `GetEndpointByID`)

**Step 1: Add stats types and `GetStats` method**

Add these types and the method at the end of `repository.go`:

```go
// StatsResult holds network-wide aggregation data.
type StatsResult struct {
	TotalEndpoints     int                `json:"total_endpoints"`
	TotalDomains       int                `json:"total_domains"`
	EndpointsByNetwork []NameCount        `json:"endpoints_by_network"`
	EndpointsByAsset   []NameCount        `json:"endpoints_by_asset"`
	EndpointsByPrice   []NameCount        `json:"endpoints_by_price_bracket"`
	EndpointsOverTime  []DateCount        `json:"endpoints_over_time"`
	CrawlHistory       []models.CrawlRun  `json:"crawl_history"`
}

type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type DateCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// GetStats returns network-wide aggregation data.
func (r *Repository) GetStats(ctx context.Context) (*StatsResult, error) {
	s := &StatsResult{}

	// Total endpoints and domains
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(DISTINCT domain) FROM endpoints`,
	).Scan(&s.TotalEndpoints, &s.TotalDomains)
	if err != nil {
		return nil, fmt.Errorf("stats totals: %w", err)
	}

	// Endpoints by network
	rows, err := r.pool.Query(ctx,
		`SELECT po.network_normalized, COUNT(DISTINCT po.endpoint_id)
		 FROM payment_options po
		 GROUP BY po.network_normalized
		 ORDER BY COUNT(DISTINCT po.endpoint_id) DESC`)
	if err != nil {
		return nil, fmt.Errorf("stats by network: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var nc NameCount
		if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, fmt.Errorf("scan network: %w", err)
		}
		s.EndpointsByNetwork = append(s.EndpointsByNetwork, nc)
	}

	// Endpoints by asset
	rows2, err := r.pool.Query(ctx,
		`SELECT po.asset_name, COUNT(DISTINCT po.endpoint_id)
		 FROM payment_options po
		 GROUP BY po.asset_name
		 ORDER BY COUNT(DISTINCT po.endpoint_id) DESC`)
	if err != nil {
		return nil, fmt.Errorf("stats by asset: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var nc NameCount
		if err := rows2.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		s.EndpointsByAsset = append(s.EndpointsByAsset, nc)
	}

	// Endpoints by price bracket
	rows3, err := r.pool.Query(ctx,
		`SELECT bracket, COUNT(*) FROM (
			SELECT CASE
				WHEN po.price_usd < 0.001 THEN '$0-0.001'
				WHEN po.price_usd < 0.01 THEN '$0.001-0.01'
				WHEN po.price_usd < 0.1 THEN '$0.01-0.1'
				ELSE '$0.1+'
			END AS bracket
			FROM payment_options po
		) sub GROUP BY bracket ORDER BY bracket`)
	if err != nil {
		return nil, fmt.Errorf("stats by price: %w", err)
	}
	defer rows3.Close()
	for rows3.Next() {
		var nc NameCount
		if err := rows3.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, fmt.Errorf("scan price: %w", err)
		}
		s.EndpointsByPrice = append(s.EndpointsByPrice, nc)
	}

	// Endpoints over time (by first_seen date)
	rows4, err := r.pool.Query(ctx,
		`SELECT DATE(first_seen) AS d, COUNT(*)
		 FROM endpoints
		 GROUP BY d
		 ORDER BY d`)
	if err != nil {
		return nil, fmt.Errorf("stats over time: %w", err)
	}
	defer rows4.Close()
	cumulative := 0
	for rows4.Next() {
		var dc DateCount
		var dailyCount int
		if err := rows4.Scan(&dc.Date, &dailyCount); err != nil {
			return nil, fmt.Errorf("scan date: %w", err)
		}
		cumulative += dailyCount
		dc.Count = cumulative
		s.EndpointsOverTime = append(s.EndpointsOverTime, dc)
	}

	// Crawl history (last 10)
	rows5, err := r.pool.Query(ctx,
		`SELECT id, started_at, completed_at, total_fetched,
		        new_endpoints, updated_endpoints, status, error
		 FROM crawl_runs
		 ORDER BY started_at DESC
		 LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("stats crawl history: %w", err)
	}
	defer rows5.Close()
	for rows5.Next() {
		var cr models.CrawlRun
		if err := rows5.Scan(&cr.ID, &cr.StartedAt, &cr.CompletedAt,
			&cr.TotalFetched, &cr.NewEndpoints, &cr.UpdatedEndpoints,
			&cr.Status, &cr.Error); err != nil {
			return nil, fmt.Errorf("scan crawl run: %w", err)
		}
		s.CrawlHistory = append(s.CrawlHistory, cr)
	}

	return s, nil
}
```

**Step 2: Verify build**

Run: `cd c:/Users/yaman/Desktop/agora && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/database/repository.go
git commit -m "feat(api): add GetStats repository method for network aggregations"
```

---

### Task 2: Add `GetEndpointsWithPayments` repository method

**Files:**
- Modify: `internal/database/repository.go`

**Step 1: Add method after `GetEndpoints`**

This returns endpoints with their payment options pre-loaded, needed for the list API.

```go
// EndpointWithPayments holds an endpoint with its payment options.
type EndpointWithPayments struct {
	Endpoint       models.Endpoint        `json:"endpoint"`
	PaymentOptions []models.PaymentOption  `json:"payment_options"`
}

// GetEndpointsWithPayments returns a paginated list of endpoints with payment options.
func (r *Repository) GetEndpointsWithPayments(ctx context.Context, limit, offset int) ([]EndpointWithPayments, error) {
	endpoints, err := r.GetEndpoints(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	if len(endpoints) == 0 {
		return nil, nil
	}

	// Collect endpoint IDs
	ids := make([]uuid.UUID, len(endpoints))
	for i, e := range endpoints {
		ids[i] = e.ID
	}

	// Batch-fetch payment options
	rows, err := r.pool.Query(ctx,
		`SELECT id, endpoint_id, scheme, network_raw, network_normalized,
		        asset_address, asset_name, max_amount_raw, price_usd,
		        pay_to, max_timeout_seconds, mime_type, description, output_schema_raw
		 FROM payment_options
		 WHERE endpoint_id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("get payment options batch: %w", err)
	}
	defer rows.Close()

	// Group by endpoint ID
	poMap := make(map[uuid.UUID][]models.PaymentOption)
	for rows.Next() {
		var po models.PaymentOption
		err := rows.Scan(
			&po.ID, &po.EndpointID, &po.Scheme, &po.NetworkRaw, &po.NetworkNormalized,
			&po.AssetAddress, &po.AssetName, &po.MaxAmountRaw, &po.PriceUSD,
			&po.PayTo, &po.MaxTimeoutSeconds, &po.MimeType, &po.Description, &po.OutputSchemaRaw,
		)
		if err != nil {
			return nil, fmt.Errorf("scan payment option: %w", err)
		}
		poMap[po.EndpointID] = append(poMap[po.EndpointID], po)
	}

	result := make([]EndpointWithPayments, len(endpoints))
	for i, e := range endpoints {
		result[i] = EndpointWithPayments{
			Endpoint:       e,
			PaymentOptions: poMap[e.ID],
		}
	}
	return result, nil
}
```

**Step 2: Verify build**

Run: `cd c:/Users/yaman/Desktop/agora && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/database/repository.go
git commit -m "feat(api): add GetEndpointsWithPayments for list with payment options"
```

---

### Task 3: Add `handleStats` handler and update `handleEndpoints`

**Files:**
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/server.go`

**Step 1: Add `handleStats` to handlers.go**

Add this method at the end of `handlers.go`:

```go
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.repo.GetStats(r.Context())
	if err != nil {
		log.Printf("get stats error: %v", err)
		http.Error(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
```

**Step 2: Update `handleEndpoints` to return payment options inline**

Replace the existing `handleEndpoints` method in `handlers.go`:

```go
func (h *Handlers) handleEndpoints(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	endpoints, err := h.repo.GetEndpointsWithPayments(r.Context(), limit, offset)
	if err != nil {
		log.Printf("get endpoints error: %v", err)
		http.Error(w, "failed to get endpoints", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoints)
}
```

**Step 3: Register the stats route in `server.go`**

Add this line after the existing routes in `server.go` `Start()` method:

```go
mux.HandleFunc("GET /api/stats", s.handlers.handleStats)
```

**Step 4: Verify build**

Run: `cd c:/Users/yaman/Desktop/agora && go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/api/handlers.go internal/api/server.go
git commit -m "feat(api): add /api/stats endpoint and include payment options in /api/endpoints"
```

---

### Task 4: Run Go tests

**Step 1: Run all tests**

Run: `cd c:/Users/yaman/Desktop/agora && go test ./...`
Expected: All tests pass (the handler tests should still work since we only changed the list endpoint's repo call, and the search/method tests don't hit the DB)

**Step 2: If any tests fail, fix them before proceeding**

---

### Task 5: Create frontend API client

**Files:**
- Create: `web/src/lib/api.ts`

**Step 1: Create the API client module**

```typescript
const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export interface ApiPaymentOption {
  id: string;
  endpoint_id: string;
  scheme: string;
  network_raw: string;
  network_normalized: string;
  asset_address: string;
  asset_name: string;
  max_amount_raw: string;
  price_usd: number;
  pay_to: string;
  max_timeout_seconds: number;
  mime_type: string;
  description: string;
}

export interface ApiEndpoint {
  id: string;
  resource_url: string;
  domain: string;
  type: string;
  x402_version: number;
  description: string;
  http_method: string;
  input_schema: Record<string, unknown> | null;
  output_schema: Record<string, unknown> | null;
  last_updated: string;
  first_seen: string;
  last_crawled: string;
}

export interface ApiEndpointWithPayments {
  endpoint: ApiEndpoint;
  payment_options: ApiPaymentOption[] | null;
}

export interface ApiSearchResult {
  endpoint: ApiEndpoint;
  similarity: number;
}

export interface ApiSearchResponse {
  results: ApiSearchResult[] | null;
  total: number;
  query_time_ms: number;
}

export interface ApiStats {
  total_endpoints: number;
  total_domains: number;
  endpoints_by_network: { name: string; count: number }[];
  endpoints_by_asset: { name: string; count: number }[];
  endpoints_by_price_bracket: { name: string; count: number }[];
  endpoints_over_time: { date: string; count: number }[];
  crawl_history: {
    id: string;
    started_at: string;
    completed_at: string | null;
    total_fetched: number;
    new_endpoints: number;
    updated_endpoints: number;
    status: string;
  }[];
}

export async function fetchEndpoints(
  limit = 20,
  offset = 0
): Promise<ApiEndpointWithPayments[]> {
  const res = await fetch(
    `${API_URL}/api/endpoints?limit=${limit}&offset=${offset}`
  );
  if (!res.ok) throw new Error(`Failed to fetch endpoints: ${res.status}`);
  return res.json();
}

export async function searchEndpoints(
  query: string,
  filters?: { network?: string; method?: string; min_price?: number; max_price?: number },
  limit = 20
): Promise<ApiSearchResponse> {
  const res = await fetch(`${API_URL}/api/search`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query, filters: filters || {}, limit }),
  });
  if (!res.ok) throw new Error(`Search failed: ${res.status}`);
  return res.json();
}

export async function fetchStats(): Promise<ApiStats> {
  const res = await fetch(`${API_URL}/api/stats`);
  if (!res.ok) throw new Error(`Failed to fetch stats: ${res.status}`);
  return res.json();
}
```

**Step 2: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "feat(web): add API client for Go backend"
```

---

### Task 6: Update frontend types

**Files:**
- Modify: `web/src/lib/types.ts`

**Step 1: Make reliability fields optional and add payment options**

Replace the contents of `types.ts` with:

```typescript
export interface Endpoint {
  id: string;
  resourceUrl: string;
  domain: string;
  type: string;
  x402Version: number;
  description: string;
  httpMethod: string;
  inputSchema: Record<string, unknown> | null;
  outputSchema: Record<string, unknown> | null;
  lastUpdated: string;
  firstSeen: string;
  lastCrawled: string;
  reliabilityScore?: number;
  reliabilityTrend?: number[];
  paymentOptions: PaymentOption[];
}

export interface PaymentOption {
  id: string;
  endpointId: string;
  scheme: string;
  networkRaw: string;
  networkNormalized: string;
  assetAddress: string;
  assetName: string;
  maxAmountRaw: string;
  priceUsd: number;
  payTo: string;
  maxTimeoutSeconds: number;
  mimeType: string;
  description: string;
}

export interface Facilitator {
  domain: string;
  endpointCount: number;
  avgReliability: number;
  reliabilityTrend: number[];
  networks: string[];
  assets: string[];
  status: "healthy" | "degraded" | "inactive";
}

export interface CrawlRun {
  id: string;
  startedAt: string;
  completedAt: string | null;
  totalFetched: number;
  newEndpoints: number;
  updatedEndpoints: number;
  status: string;
}

export interface NetworkStats {
  totalEndpoints: number;
  totalDomains: number;
  endpointsByNetwork: { name: string; count: number }[];
  endpointsByAsset: { name: string; count: number }[];
  endpointsByPriceBracket: { name: string; count: number }[];
  endpointsOverTime: { date: string; count: number }[];
  crawlHistory: CrawlRun[];
}
```

**Step 2: Commit**

```bash
git add web/src/lib/types.ts
git commit -m "feat(web): update types for real API data, make reliability optional"
```

---

### Task 7: Create transform helpers

**Files:**
- Create: `web/src/lib/transforms.ts`

**Step 1: Create transform functions from API types to frontend types**

```typescript
import type { Endpoint, PaymentOption, NetworkStats } from "./types";
import type {
  ApiEndpointWithPayments,
  ApiSearchResult,
  ApiPaymentOption,
  ApiStats,
} from "./api";

function transformPaymentOption(po: ApiPaymentOption): PaymentOption {
  return {
    id: po.id,
    endpointId: po.endpoint_id,
    scheme: po.scheme,
    networkRaw: po.network_raw,
    networkNormalized: po.network_normalized,
    assetAddress: po.asset_address,
    assetName: po.asset_name,
    maxAmountRaw: po.max_amount_raw,
    priceUsd: po.price_usd,
    payTo: po.pay_to,
    maxTimeoutSeconds: po.max_timeout_seconds,
    mimeType: po.mime_type,
    description: po.description,
  };
}

function transformEndpoint(
  api: { id: string; resource_url: string; domain: string; type: string; x402_version: number; description: string; http_method: string; input_schema: Record<string, unknown> | null; output_schema: Record<string, unknown> | null; last_updated: string; first_seen: string; last_crawled: string },
  paymentOptions: PaymentOption[]
): Endpoint {
  return {
    id: api.id,
    resourceUrl: api.resource_url,
    domain: api.domain,
    type: api.type,
    x402Version: api.x402_version,
    description: api.description,
    httpMethod: api.http_method,
    inputSchema: api.input_schema,
    outputSchema: api.output_schema,
    lastUpdated: api.last_updated,
    firstSeen: api.first_seen,
    lastCrawled: api.last_crawled,
    paymentOptions,
  };
}

export function transformEndpointWithPayments(
  ewp: ApiEndpointWithPayments
): Endpoint {
  const pos = (ewp.payment_options || []).map(transformPaymentOption);
  return transformEndpoint(ewp.endpoint, pos);
}

export function transformSearchResult(sr: ApiSearchResult): Endpoint {
  return transformEndpoint(sr.endpoint, []);
}

export function transformStats(api: ApiStats): NetworkStats {
  return {
    totalEndpoints: api.total_endpoints,
    totalDomains: api.total_domains,
    endpointsByNetwork: api.endpoints_by_network || [],
    endpointsByAsset: api.endpoints_by_asset || [],
    endpointsByPriceBracket: api.endpoints_by_price_bracket || [],
    endpointsOverTime: api.endpoints_over_time || [],
    crawlHistory: (api.crawl_history || []).map((cr) => ({
      id: cr.id,
      startedAt: cr.started_at,
      completedAt: cr.completed_at,
      totalFetched: cr.total_fetched,
      newEndpoints: cr.new_endpoints,
      updatedEndpoints: cr.updated_endpoints,
      status: cr.status,
    })),
  };
}
```

**Step 2: Commit**

```bash
git add web/src/lib/transforms.ts
git commit -m "feat(web): add API-to-frontend type transform helpers"
```

---

### Task 8: Update EndpointsTable and ReliabilityPulse for optional reliability

**Files:**
- Modify: `web/src/components/endpoints-table.tsx`
- Modify: `web/src/components/reliability-pulse.tsx`

**Step 1: Check current ReliabilityPulse component**

Read `web/src/components/reliability-pulse.tsx` first. Then update it so the `score` prop is optional. When `undefined`, render a neutral gray dot.

**Step 2: Update EndpointsTable to handle optional reliability**

In `endpoints-table.tsx`, the `ReliabilityPulse` and `ReliabilityBar` receive `ep.reliabilityScore`. Since this is now optional, use `ep.reliabilityScore ?? undefined` (already works if the type is optional). For the `ReliabilityBar`, pass `score={ep.reliabilityScore}` — update ReliabilityBar similarly to accept `score?: number`.

Also handle the case where `paymentOptions` might be empty (use optional chaining, which is already partially there).

**Step 3: Commit**

```bash
git add web/src/components/endpoints-table.tsx web/src/components/reliability-pulse.tsx
git commit -m "feat(web): make reliability display optional in endpoint table"
```

---

### Task 9: Rewrite Endpoints page to use real API

**Files:**
- Modify: `web/src/app/page.tsx`

**Step 1: Replace dummy data with API calls**

```typescript
"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { SearchBar } from "@/components/search-bar";
import { FilterChips } from "@/components/filter-chips";
import { EndpointsTable } from "@/components/endpoints-table";
import { fetchEndpoints, searchEndpoints } from "@/lib/api";
import { transformEndpointWithPayments, transformSearchResult } from "@/lib/transforms";
import type { Endpoint } from "@/lib/types";

const filterGroups = [
  { label: "Network", options: ["base", "ethereum", "arbitrum"] },
  { label: "Method", options: ["GET", "POST"] },
];

export default function EndpointsPage() {
  const [endpoints, setEndpoints] = useState<Endpoint[]>([]);
  const [total, setTotal] = useState(0);
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState<Record<string, string | null>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  const load = useCallback(async (q: string, f: Record<string, string | null>) => {
    setLoading(true);
    setError(null);
    try {
      if (q.trim()) {
        const apiFilters: Record<string, string> = {};
        if (f.Network) apiFilters.network = f.Network;
        if (f.Method) apiFilters.method = f.Method;
        const res = await searchEndpoints(q, apiFilters, 20);
        setEndpoints((res.results || []).map(transformSearchResult));
        setTotal(res.total);
      } else {
        const res = await fetchEndpoints(20, 0);
        const transformed = (res || []).map(transformEndpointWithPayments);
        setEndpoints(transformed);
        setTotal(transformed.length);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load("", {});
  }, [load]);

  const handleSearch = (q: string) => {
    setQuery(q);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => load(q, filters), 300);
  };

  const handleFilterChange = (f: Record<string, string | null>) => {
    setFilters(f);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => load(query, f), 300);
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-lg font-semibold tracking-tight">Endpoints</h1>
        <p className="text-sm text-ink-secondary mt-1">
          {loading ? (
            <span className="text-ink-tertiary">Loading...</span>
          ) : error ? (
            <span className="text-failure">{error}</span>
          ) : (
            <>
              <span className="font-mono text-ink-primary">{endpoints.length}</span>{" "}
              {query ? `results for "${query}"` : "endpoints"}{" "}
              {query && <span className="text-ink-tertiary">(semantic search)</span>}
            </>
          )}
        </p>
      </div>
      <SearchBar onSearch={handleSearch} />
      <FilterChips groups={filterGroups} onFilterChange={handleFilterChange} />
      {!loading && !error && <EndpointsTable endpoints={endpoints} />}
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add web/src/app/page.tsx
git commit -m "feat(web): wire endpoints page to real API with semantic search"
```

---

### Task 10: Rewrite Network page to use real API

**Files:**
- Modify: `web/src/app/network/page.tsx`

**Step 1: Replace dummy data with API call**

```typescript
"use client";

import { useState, useEffect } from "react";
import { StatLine } from "@/components/stat-line";
import { HorizontalBarChart } from "@/components/horizontal-bar-chart";
import { AreaChartPanel } from "@/components/area-chart-panel";
import { fetchStats } from "@/lib/api";
import { transformStats } from "@/lib/transforms";
import type { NetworkStats } from "@/lib/types";

export default function NetworkPage() {
  const [stats, setStats] = useState<NetworkStats | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchStats()
      .then((data) => setStats(transformStats(data)))
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, []);

  if (error) {
    return (
      <div className="space-y-6">
        <h1 className="text-lg font-semibold tracking-tight">Network</h1>
        <p className="text-sm text-failure">{error}</p>
      </div>
    );
  }

  if (!stats) {
    return (
      <div className="space-y-6">
        <h1 className="text-lg font-semibold tracking-tight">Network</h1>
        <p className="text-sm text-ink-tertiary">Loading...</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-lg font-semibold tracking-tight">Network</h1>
        <div className="mt-1">
          <StatLine stats={stats} />
        </div>
      </div>
      {stats.endpointsOverTime.length > 0 && (
        <AreaChartPanel
          data={stats.endpointsOverTime}
          title="Endpoints discovered over time"
        />
      )}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <HorizontalBarChart
          data={stats.endpointsByNetwork.map((d) => ({ name: d.name, value: d.count }))}
          title="Endpoints by network"
        />
        <HorizontalBarChart
          data={stats.endpointsByAsset.map((d) => ({ name: d.name, value: d.count }))}
          title="Endpoints by asset"
          color="hsl(222, 100%, 50%)"
        />
        <HorizontalBarChart
          data={stats.endpointsByPriceBracket.map((d) => ({ name: d.name, value: d.count }))}
          title="Endpoints by price bracket"
          color="hsl(38, 85%, 55%)"
        />
      </div>
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add web/src/app/network/page.tsx
git commit -m "feat(web): wire network page to real API stats"
```

---

### Task 11: Update StatLine and NetworkStats type compatibility

**Files:**
- Modify: `web/src/components/stat-line.tsx`

**Step 1: Remove `totalFacilitators` and `lastCrawl` references**

The API stats don't include facilitator count or last crawl time. Update StatLine to only show what the API provides:

```typescript
import { NetworkStats } from "@/lib/types";

export function StatLine({ stats }: { stats: NetworkStats }) {
  return (
    <p className="text-sm text-ink-secondary">
      <span className="text-ink-primary font-mono">{stats.totalEndpoints.toLocaleString()}</span> endpoints
      <span className="text-ink-tertiary mx-2">&middot;</span>
      <span className="text-ink-primary font-mono">{stats.totalDomains.toLocaleString()}</span> domains
    </p>
  );
}
```

**Step 2: Commit**

```bash
git add web/src/components/stat-line.tsx
git commit -m "feat(web): simplify stat line to match API stats"
```

---

### Task 12: Add `.env.local` for Next.js and verify build

**Files:**
- Create: `web/.env.local`
- Modify: `web/.gitignore` (if it doesn't already ignore `.env.local`)

**Step 1: Create env file**

```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

**Step 2: Verify Next.js build**

Run: `cd c:/Users/yaman/Desktop/agora/web && npm run build`
Expected: Build succeeds with no type errors

**Step 3: If build fails, fix type errors before proceeding**

**Step 4: Commit**

```bash
git add web/.env.local
git commit -m "chore(web): add .env.local for API URL configuration"
```

Note: `.env.local` is gitignored by Next.js by default. If it is, skip the git add and instead document it in the README or `.env.example`.

---

### Task 13: E2E smoke test

**Step 1: Start Docker (if not running)**

Run: `cd c:/Users/yaman/Desktop/agora && docker compose up -d`

**Step 2: Start the Go API**

Run: `cd c:/Users/yaman/Desktop/agora && go run ./cmd/agora serve`
(Needs the embedding sidecar running for search, but endpoints/stats work without it)

**Step 3: Test endpoints list**

Run: `curl http://localhost:8080/api/endpoints?limit=2`
Expected: JSON array of 2 endpoint objects, each with `endpoint` and `payment_options` fields

**Step 4: Test stats**

Run: `curl http://localhost:8080/api/stats`
Expected: JSON with `total_endpoints` > 12000, arrays for `endpoints_by_network`, `endpoints_by_asset`, `endpoints_by_price_bracket`, `endpoints_over_time`

**Step 5: Start the web frontend**

Run: `cd c:/Users/yaman/Desktop/agora/web && npm run dev`

**Step 6: Open browser**

Navigate to `http://localhost:3000` — should show real endpoint data from the database.
Navigate to `http://localhost:3000/network` — should show real aggregate charts.

**Step 7: Test semantic search (requires embedding sidecar)**

Start sidecar if not running: `cd c:/Users/yaman/Desktop/agora/embed && python -m uvicorn server:app --port 8100`
Type a query in the search bar — should return semantically relevant results.
