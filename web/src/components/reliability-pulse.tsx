interface ReliabilityPulseProps {
  score: number;
}

export function ReliabilityPulse({ score }: ReliabilityPulseProps) {
  if (score > 70) {
    return <span className="text-signal animate-pulse text-xs">●</span>;
  }
  if (score > 30) {
    return <span className="text-caution text-xs">◐</span>;
  }
  return <span className="text-ink-muted text-xs">○</span>;
}
