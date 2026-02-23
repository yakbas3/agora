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
  return (
    <div className="border border-border rounded-md overflow-hidden">
      {endpoints.map((ep, i) => (
        <div
          key={ep.id}
          className={`flex items-start gap-3 px-4 py-3 hover:bg-void-hover transition-colors ${
            i > 0 ? "border-t border-border-soft" : ""
          }`}
        >
          <div className="pt-1">
            <ReliabilityPulse score={ep.reliabilityScore} />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-3">
              <span className="font-mono text-sm text-ink-primary truncate">{ep.resourceUrl}</span>
              <span className={`text-xs font-mono font-medium ${methodColor(ep.httpMethod)}`}>
                {ep.httpMethod}
              </span>
            </div>
            <div className="flex items-center gap-3 mt-1">
              <span className="text-xs text-ink-secondary truncate">{ep.description}</span>
              <span className="text-xs text-ink-tertiary">
                {ep.paymentOptions[0]?.networkNormalized ?? "\u2014"}
              </span>
              <span className="text-xs text-ink-tertiary">
                {ep.paymentOptions[0]?.assetName ?? "\u2014"}
              </span>
              <span className="text-xs text-ink-tertiary">{timeAgo(ep.lastCrawled)}</span>
            </div>
          </div>
          <div className="flex items-center gap-4 shrink-0">
            <span className="text-xs font-mono text-ink-secondary">
              ${ep.paymentOptions[0]?.priceUsd.toFixed(4) ?? "\u2014"}/req
            </span>
            <ReliabilityBar score={ep.reliabilityScore} />
          </div>
        </div>
      ))}
    </div>
  );
}
