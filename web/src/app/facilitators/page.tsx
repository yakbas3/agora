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

  const active = facilitators.filter((f) => f.totalTxCount > 0);
  const notIndexed = facilitators.filter((f) => f.totalTxCount === 0);

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-lg font-semibold tracking-tight">Facilitators</h1>
        <p className="text-sm text-ink-secondary mt-1">
          <span className="font-mono text-ink-primary">{facilitators.length}</span> known facilitators on the x402 network{" "}
          <span className="text-ink-tertiary">
            — <span className="font-mono text-ink-primary">{active.length}</span> with indexed on-chain activity
          </span>
        </p>
      </div>

      <section className="space-y-3">
        <h2 className="text-xs uppercase tracking-wider text-ink-tertiary font-mono">
          Active ({active.length})
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {active.map((f) => (
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
      </section>

      {notIndexed.length > 0 && (
        <section className="space-y-3">
          <div>
            <h2 className="text-xs uppercase tracking-wider text-ink-tertiary font-mono">
              Registered, not yet indexed ({notIndexed.length})
            </h2>
            <p className="text-xs text-ink-tertiary mt-1 max-w-2xl">
              Facilitator addresses from the x402scan registry that have no matching on-chain
              activity in our indexed window. Either they have not transacted on Base, or they
              have not been synced from the CDP SQL API yet. Run{" "}
              <code className="font-mono text-ink-secondary bg-void-elevated px-1 rounded">
                ./agora sync --missing
              </code>{" "}
              to populate.
            </p>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
            {notIndexed.map((f) => (
              <div
                key={f.name}
                className="bg-void-elevated/40 border border-border-soft rounded-md px-3 py-2 flex items-center justify-between"
              >
                <span className="font-mono text-xs text-ink-secondary">{f.name}</span>
                <span className="font-mono text-[10px] text-ink-tertiary">
                  {f.addresses.length} addr
                </span>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
