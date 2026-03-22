import { Facilitator } from "@/lib/types";
import { ReliabilityPulse } from "./reliability-pulse";
import { Sparkline } from "./sparkline";

const statusColor: Record<string, string> = {
  healthy: "var(--color-signal)",
  degraded: "var(--color-caution)",
  inactive: "var(--color-ink-muted)",
};

export function FacilitatorCard({ facilitator }: { facilitator: Facilitator }) {
  return (
    <div className="bg-void-elevated border border-border rounded-md p-4 hover:border-border-focus transition-colors group">
      {/* Header: pulse + domain */}
      <div className="flex items-center gap-2">
        <ReliabilityPulse score={facilitator.avgReliability} size="base" />
        <span className="font-mono text-sm text-ink-primary group-hover:text-signal transition-colors">
          {facilitator.domain}
        </span>
      </div>

      {/* Stats */}
      <div className="mt-2 flex items-center gap-1.5 text-xs text-ink-secondary">
        <span className="font-mono text-ink-primary">{facilitator.endpointCount}</span>
        <span>endpoints</span>
        <span className="text-ink-tertiary">&middot;</span>
        <span className="font-mono text-ink-primary">{facilitator.avgReliability.toFixed(1)}%</span>
        <span>uptime</span>
      </div>

      {/* Sparkline with caption */}
      <div className="mt-3 flex items-end gap-2">
        <Sparkline data={facilitator.reliabilityTrend} color={statusColor[facilitator.status]} />
        <span className="text-[10px] text-ink-muted leading-none pb-px">30d</span>
      </div>

      {/* Network/asset chips */}
      <div className="mt-3 flex flex-wrap gap-1.5">
        {facilitator.networks.map((net) => (
          <span
            key={net}
            className="text-[11px] font-mono text-ink-tertiary border border-border-soft rounded-sm px-1.5 py-0.5"
          >
            {net}
          </span>
        ))}
        {facilitator.assets.map((asset) => (
          <span
            key={asset}
            className="text-[11px] font-mono text-ink-tertiary border border-border-soft rounded-sm px-1.5 py-0.5"
          >
            {asset}
          </span>
        ))}
      </div>
    </div>
  );
}
