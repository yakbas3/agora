interface ReliabilityPulseProps {
  score?: number;
  size?: "sm" | "base";
}

export function ReliabilityPulse({ score, size = "sm" }: ReliabilityPulseProps) {
  const sizeClass = size === "base" ? "text-sm" : "text-xs";

  if (score === undefined) {
    return <span className={`text-ink-muted ${sizeClass} leading-none`}>●</span>;
  }
  if (score > 70) {
    return <span className={`text-signal animate-signal-pulse ${sizeClass} leading-none`}>●</span>;
  }
  if (score > 30) {
    return <span className={`text-caution ${sizeClass} leading-none`}>◐</span>;
  }
  return <span className={`text-ink-muted ${sizeClass} leading-none`}>○</span>;
}
