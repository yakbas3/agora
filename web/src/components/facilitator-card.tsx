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
    <div className="bg-void-elevated border border-border rounded-md p-4 hover:border-border-focus transition-colors">
      <div className="flex items-center gap-2">
        <ReliabilityPulse score={facilitator.avgReliability} />
        <span className="font-mono text-sm text-ink-primary">{facilitator.domain}</span>
      </div>
      <div className="mt-2 flex items-center gap-3 text-xs text-ink-secondary">
        <span>{facilitator.endpointCount} endpoints</span>
        <span>·</span>
        <span>{facilitator.avgReliability.toFixed(1)}% uptime</span>
      </div>
      <div className="mt-3">
        <Sparkline data={facilitator.reliabilityTrend} color={statusColor[facilitator.status]} />
        <span className="text-xs text-ink-tertiary ml-2">30d</span>
      </div>
      <div className="mt-3 flex items-center gap-2 text-xs text-ink-tertiary">
        <span>{facilitator.networks.join(", ")}</span>
        <span>·</span>
        <span>{facilitator.assets.join(", ")}</span>
      </div>
    </div>
  );
}
