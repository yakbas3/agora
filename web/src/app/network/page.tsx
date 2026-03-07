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
