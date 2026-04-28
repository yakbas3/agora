"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { SearchBar } from "@/components/search-bar";
import { FilterChips } from "@/components/filter-chips";
import { EndpointsTable } from "@/components/endpoints-table";
import { fetchEndpoints, fetchStats, searchEndpoints } from "@/lib/api";
import { transformEndpointWithPayments, transformSearchResult } from "@/lib/transforms";
import type { Endpoint } from "@/lib/types";

const filterGroups = [
  { label: "Network", options: ["base", "ethereum", "arbitrum"] },
  { label: "Method", options: ["GET", "POST"] },
];

export default function EndpointsPage() {
  const [endpoints, setEndpoints] = useState<Endpoint[]>([]);
  const [totalIndexed, setTotalIndexed] = useState<number | null>(null);
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState<Record<string, string | null>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => {
    fetchStats()
      .then((s) => setTotalIndexed(s.total_endpoints))
      .catch(() => {
        // Silent fail — the count is a nice-to-have, not critical.
      });
  }, []);

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
      } else {
        const res = await fetchEndpoints(20, 0);
        setEndpoints((res || []).map(transformEndpointWithPayments));
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
          ) : query ? (
            <>
              <span className="font-mono text-ink-primary">{endpoints.length}</span>{" "}
              results for &ldquo;{query}&rdquo;{" "}
              <span className="text-ink-tertiary">
                (semantic search over {totalIndexed !== null ? totalIndexed.toLocaleString() : "—"} indexed endpoints)
              </span>
            </>
          ) : (
            <>
              Showing{" "}
              <span className="font-mono text-ink-primary">{endpoints.length}</span>
              {totalIndexed !== null && (
                <>
                  {" "}of{" "}
                  <span className="font-mono text-ink-primary">{totalIndexed.toLocaleString()}</span>
                </>
              )}{" "}
              endpoints{" "}
              <span className="text-ink-tertiary">
                — search to find specific capabilities across the full index
              </span>
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
