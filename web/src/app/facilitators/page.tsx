"use client";

import { useEffect, useState } from "react";
import { fetchFacilitators } from "@/lib/api";
import { FacilitatorStats } from "@/lib/types";

interface GroupedFacilitator {
  name: string;
  addresses: string[];
  totalTxCount: number;
  totalVolume: number;
  uniquePayers: number;
}

function groupFacilitators(stats: FacilitatorStats[]): GroupedFacilitator[] {
  const map = new Map<string, GroupedFacilitator>();
  for (const s of stats) {
    const existing = map.get(s.name);
    if (existing) {
      existing.addresses.push(s.address);
      existing.totalTxCount += s.tx_count;
      existing.totalVolume += s.total_volume_usd;
      existing.uniquePayers += s.unique_payers;
    } else {
      map.set(s.name, {
        name: s.name,
        addresses: [s.address],
        totalTxCount: s.tx_count,
        totalVolume: s.total_volume_usd,
        uniquePayers: s.unique_payers,
      });
    }
  }
  return Array.from(map.values()).sort((a, b) => b.totalVolume - a.totalVolume);
}

export default function FacilitatorsPage() {
  const [facilitators, setFacilitators] = useState<GroupedFacilitator[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchFacilitators()
      .then((data) => setFacilitators(groupFacilitators(data)))
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return <div className="text-sm text-ink-secondary">Loading facilitators...</div>;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-lg font-semibold tracking-tight">Facilitators</h1>
        <p className="text-sm text-ink-secondary mt-1">
          <span className="font-mono text-ink-primary">{facilitators.length}</span> known facilitators on the x402 network
        </p>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {facilitators.map((f) => (
          <div
            key={f.name}
            className="bg-void-elevated border border-border rounded-md p-4 hover:border-border-focus transition-colors"
          >
            <div className="font-mono text-sm text-ink-primary font-medium">{f.name}</div>
            <div className="mt-2 grid grid-cols-2 gap-2 text-xs text-ink-secondary">
              <div>
                <span className="font-mono text-ink-primary">{f.addresses.length}</span> address{f.addresses.length !== 1 ? "es" : ""}
              </div>
              <div>
                <span className="font-mono text-ink-primary">{f.totalTxCount.toLocaleString()}</span> txns
              </div>
              <div>
                <span className="font-mono text-ink-primary">${f.totalVolume.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span> volume
              </div>
              <div>
                <span className="font-mono text-ink-primary">{f.uniquePayers.toLocaleString()}</span> payers
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
