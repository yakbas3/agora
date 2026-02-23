import { NetworkStats } from "@/lib/types";

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const hours = Math.floor(diff / 3600000);
  if (hours < 1) return "just now";
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function StatLine({ stats }: { stats: NetworkStats }) {
  return (
    <p className="text-sm text-ink-secondary">
      <span className="text-ink-primary font-mono">{stats.totalEndpoints.toLocaleString()}</span> endpoints
      <span className="text-ink-tertiary mx-2">&middot;</span>
      <span className="text-ink-primary font-mono">{stats.totalDomains.toLocaleString()}</span> domains
      <span className="text-ink-tertiary mx-2">&middot;</span>
      <span className="text-ink-primary font-mono">{stats.totalFacilitators}</span> facilitators
      <span className="text-ink-tertiary mx-2">&middot;</span>
      last crawl <span className="text-ink-primary">{timeAgo(stats.lastCrawl)}</span>
    </p>
  );
}
