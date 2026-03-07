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
