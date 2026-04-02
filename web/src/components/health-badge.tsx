interface HealthBadgeProps {
  status?: "alive" | "dead" | "unknown";
  latencyMs?: number;
}

export function HealthBadge({ status, latencyMs }: HealthBadgeProps) {
  if (!status || status === "unknown") {
    return (
      <span className="inline-flex items-center gap-1 text-[10px] font-mono text-ink-tertiary">
        <span className="w-1.5 h-1.5 rounded-full bg-ink-tertiary/40 shrink-0" />
        unprobed
      </span>
    );
  }

  if (status === "alive") {
    const latencyLabel = latencyMs && latencyMs > 0
      ? `${latencyMs}ms`
      : null;

    return (
      <span className="inline-flex items-center gap-1 text-[10px] font-mono text-signal">
        <span className="w-1.5 h-1.5 rounded-full bg-signal shrink-0" />
        {latencyLabel ? `alive · ${latencyLabel}` : "alive"}
      </span>
    );
  }

  return (
    <span className="inline-flex items-center gap-1 text-[10px] font-mono text-failure">
      <span className="w-1.5 h-1.5 rounded-full bg-failure shrink-0" />
      dead
    </span>
  );
}
