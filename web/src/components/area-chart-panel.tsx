"use client";

import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip } from "recharts";

interface AreaChartPanelProps {
  data: { date: string; count: number }[];
  title: string;
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function formatTooltipDate(label: unknown): string {
  const d = new Date(String(label));
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

export function AreaChartPanel({ data, title }: AreaChartPanelProps) {
  return (
    <div className="border border-border-soft rounded-md p-4">
      <h3 className="text-sm font-medium text-ink-secondary mb-4">{title}</h3>
      <ResponsiveContainer width="100%" height={200}>
        <AreaChart data={data} margin={{ left: 0, right: 0, top: 4, bottom: 0 }}>
          <defs>
            <linearGradient id="signalGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="hsl(168, 80%, 50%)" stopOpacity={0.3} />
              <stop offset="100%" stopColor="hsl(168, 80%, 50%)" stopOpacity={0} />
            </linearGradient>
          </defs>
          <XAxis
            dataKey="date"
            tickFormatter={formatDate}
            tick={{ fill: "hsl(220, 10%, 42%)", fontSize: 11, fontFamily: "var(--font-mono)" }}
            axisLine={false}
            tickLine={false}
            interval="preserveStartEnd"
          />
          <YAxis hide />
          <Tooltip
            labelFormatter={formatTooltipDate}
            contentStyle={{
              backgroundColor: "hsl(220, 18%, 10%)",
              border: "1px solid hsla(220, 15%, 50%, 0.12)",
              borderRadius: "4px",
              color: "hsl(220, 15%, 90%)",
              fontSize: "12px",
              fontFamily: "var(--font-mono)",
            }}
            formatter={(value) => [Number(value).toLocaleString(), "endpoints"]}
          />
          <Area
            type="monotone"
            dataKey="count"
            stroke="hsl(168, 80%, 50%)"
            strokeWidth={1.5}
            fill="url(#signalGradient)"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
