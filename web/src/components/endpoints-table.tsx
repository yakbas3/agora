import { Endpoint } from "@/lib/types";
import { ReliabilityPulse } from "./reliability-pulse";
import { ReliabilityBar } from "./reliability-bar";

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const hours = Math.floor(diff / 3600000);
  if (hours < 1) return "just now";
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function methodColor(method: string): string {
  switch (method) {
    case "GET": return "text-signal";
    case "POST": return "text-base-blue";
    case "PUT": return "text-caution";
    case "DELETE": return "text-failure";
    default: return "text-ink-secondary";
  }
}

interface EndpointsTableProps {
  endpoints: Endpoint[];
}

export function EndpointsTable({ endpoints }: EndpointsTableProps) {
  if (endpoints.length === 0) {
    return (
      <div className="border border-border rounded-md px-4 py-12 text-center text-sm text-ink-tertiary">
        No endpoints match your filters.
      </div>
    );
  }

  return (
    <div className="border border-border rounded-md overflow-hidden">
      {endpoints.map((ep, i) => (
        <div
          key={ep.id}
          className={`flex items-start gap-3 px-4 py-3 hover:bg-void-hover transition-colors cursor-default ${
            i > 0 ? "border-t border-border-soft" : ""
          }`}
        >
          <div className="pt-0.5 shrink-0">
            <ReliabilityPulse score={ep.reliabilityScore} />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-baseline gap-2">
              <span className="font-mono text-sm text-ink-primary truncate">{ep.resourceUrl}</span>
              <span className={`text-[11px] font-mono font-medium shrink-0 ${methodColor(ep.httpMethod)}`}>
                {ep.httpMethod}
              </span>
            </div>
            <div className="flex items-center gap-1.5 mt-1 text-xs text-ink-tertiary">
              <span className="text-ink-secondary truncate max-w-sm">{ep.description}</span>
              <span className="shrink-0">&middot;</span>
              <span className="shrink-0">{ep.paymentOptions?.[0]?.networkNormalized ?? "\u2014"}</span>
              <span className="shrink-0">&middot;</span>
              <span className="shrink-0">{ep.paymentOptions?.[0]?.assetName ?? "\u2014"}</span>
              <span className="shrink-0">&middot;</span>
              <span className="shrink-0">{timeAgo(ep.lastCrawled)}</span>
            </div>
          </div>
          <div className="flex items-center gap-4 shrink-0 pt-0.5">
            <span className="text-xs font-mono text-ink-secondary tabular-nums">
              ${ep.paymentOptions?.[0]?.priceUsd.toFixed(4) ?? "\u2014"}/req
            </span>
            <ReliabilityBar score={ep.reliabilityScore} />
          </div>
        </div>
      ))}
    </div>
  );
}
