interface ReliabilityBarProps {
  score?: number;
}

export function ReliabilityBar({ score }: ReliabilityBarProps) {
  if (score === undefined) {
    return (
      <div className="flex items-center gap-2">
        <div className="w-20 h-1.5 bg-ink-muted/20 rounded-full overflow-hidden" />
        <span className="text-xs text-ink-tertiary font-mono tabular-nums w-8 text-right">&mdash;</span>
      </div>
    );
  }
  const color = score > 70 ? "bg-signal" : score > 30 ? "bg-caution" : "bg-failure";
  return (
    <div className="flex items-center gap-2">
      <div className="w-20 h-1.5 bg-ink-muted/20 rounded-full overflow-hidden">
        <div className={`h-full ${color} rounded-full transition-all`} style={{ width: `${score}%` }} />
      </div>
      <span className="text-xs text-ink-tertiary font-mono tabular-nums w-8 text-right">{score}%</span>
    </div>
  );
}
