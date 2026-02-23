export default function Home() {
  return (
    <div className="min-h-screen bg-void p-8">
      <h1 className="text-2xl font-semibold text-ink-primary tracking-tight">Agora</h1>
      <p className="text-ink-secondary mt-2">x402 Endpoint Explorer</p>
      <p className="text-signal font-mono mt-4">● healthy signal</p>
      <p className="text-caution font-mono">◐ degraded signal</p>
      <p className="text-failure font-mono">○ failed signal</p>
      <div className="mt-4 p-4 bg-void-elevated border border-border rounded-md">
        <p className="text-ink-tertiary text-sm">Elevated surface with border</p>
      </div>
    </div>
  );
}
