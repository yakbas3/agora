"use client";

import { BarChart, Bar, XAxis, YAxis, ResponsiveContainer, Cell } from "recharts";

interface HorizontalBarChartProps {
  data: { name: string; value: number }[];
  title: string;
  color?: string;
}

export function HorizontalBarChart({ data, title, color = "hsl(168, 80%, 50%)" }: HorizontalBarChartProps) {
  return (
    <div className="border border-border-soft rounded-md p-4">
      <h3 className="text-sm font-medium text-ink-secondary mb-4">{title}</h3>
      <ResponsiveContainer width="100%" height={data.length * 32 + 16}>
        <BarChart data={data} layout="vertical" margin={{ left: 0, right: 16, top: 0, bottom: 0 }}>
          <XAxis type="number" hide />
          <YAxis
            type="category"
            dataKey="name"
            width={100}
            tick={{ fill: "hsl(220, 12%, 62%)", fontSize: 12, fontFamily: "var(--font-mono)" }}
            axisLine={false}
            tickLine={false}
          />
          <Bar dataKey="value" radius={[0, 2, 2, 0]} barSize={16}>
            {data.map((_, i) => (
              <Cell key={i} fill={color} fillOpacity={0.8} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
