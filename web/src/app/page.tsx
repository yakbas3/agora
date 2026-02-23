"use client";

import { useState, useMemo } from "react";
import { SearchBar } from "@/components/search-bar";
import { FilterChips } from "@/components/filter-chips";
import { EndpointsTable } from "@/components/endpoints-table";
import { endpoints } from "@/lib/dummy-data";

const filterGroups = [
  { label: "Network", options: ["Base", "Ethereum", "Arbitrum"] },
  { label: "Asset", options: ["USDC", "ETH", "DAI"] },
  { label: "Reliability", options: [">90%", ">70%", ">50%"] },
  { label: "Method", options: ["GET", "POST"] },
];

export default function EndpointsPage() {
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState<Record<string, string | null>>({});

  const filtered = useMemo(() => {
    let result = endpoints;
    if (query) {
      const q = query.toLowerCase();
      result = result.filter(
        (ep) =>
          ep.resourceUrl.toLowerCase().includes(q) ||
          ep.description.toLowerCase().includes(q) ||
          ep.domain.toLowerCase().includes(q)
      );
    }
    if (filters.Network) {
      result = result.filter((ep) =>
        ep.paymentOptions.some((po) => po.networkNormalized.toLowerCase() === filters.Network!.toLowerCase())
      );
    }
    if (filters.Asset) {
      result = result.filter((ep) =>
        ep.paymentOptions.some((po) => po.assetName === filters.Asset)
      );
    }
    if (filters.Method) {
      result = result.filter((ep) => ep.httpMethod === filters.Method);
    }
    if (filters.Reliability) {
      const threshold = parseInt(filters.Reliability!.replace(/[>%]/g, ""));
      result = result.filter((ep) => ep.reliabilityScore >= threshold);
    }
    return result;
  }, [query, filters]);

  return (
    <div className="space-y-4">
      <SearchBar onSearch={setQuery} />
      <FilterChips groups={filterGroups} onFilterChange={setFilters} />
      <div className="flex items-center justify-between">
        <p className="text-xs text-ink-tertiary">
          {filtered.length} endpoint{filtered.length !== 1 ? "s" : ""}
        </p>
      </div>
      <EndpointsTable endpoints={filtered} />
    </div>
  );
}
