import { StatLine } from "@/components/stat-line";
import { HorizontalBarChart } from "@/components/horizontal-bar-chart";
import { AreaChartPanel } from "@/components/area-chart-panel";
import { networkStats } from "@/lib/dummy-data";

export default function NetworkPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-lg font-semibold tracking-tight">Network</h1>
        <div className="mt-2">
          <StatLine stats={networkStats} />
        </div>
      </div>
      <AreaChartPanel
        data={networkStats.endpointsOverTime}
        title="Endpoints discovered over time"
      />
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <HorizontalBarChart
          data={networkStats.endpointsByNetwork.map((d) => ({ name: d.network, value: d.count }))}
          title="Endpoints by network"
        />
        <HorizontalBarChart
          data={networkStats.endpointsByAsset.map((d) => ({ name: d.asset, value: d.count }))}
          title="Endpoints by asset"
          color="hsl(222, 100%, 50%)"
        />
        <HorizontalBarChart
          data={networkStats.endpointsByPriceBracket.map((d) => ({ name: d.bracket, value: d.count }))}
          title="Endpoints by price bracket"
          color="hsl(38, 85%, 55%)"
        />
      </div>
    </div>
  );
}
