import { FacilitatorCard } from "@/components/facilitator-card";
import { facilitators } from "@/lib/dummy-data";

export default function FacilitatorsPage() {
  return (
    <div className="space-y-4">
      <h1 className="text-lg font-semibold tracking-tight">Facilitators</h1>
      <p className="text-sm text-ink-secondary">
        {facilitators.length} known facilitators on the x402 network
      </p>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {facilitators.map((f) => (
          <FacilitatorCard key={f.domain} facilitator={f} />
        ))}
      </div>
    </div>
  );
}
