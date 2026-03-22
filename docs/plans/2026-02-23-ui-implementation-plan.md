# Agora UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the Agora frontend — a dark, observatory-themed x402 endpoint explorer with Next.js + Tailwind, using dummy data.

**Architecture:** Next.js App Router in a `web/` subdirectory (monorepo alongside Go backend). Three pages: Endpoints (home), Facilitators, Network. All data is dummy/hardcoded — no API calls yet. Design tokens from `docs/plans/2026-02-23-ui-design.md`.

**Tech Stack:** Next.js 15 (App Router), React 19, Tailwind CSS v4, Recharts (charts), TypeScript

**Design Reference:** `docs/plans/2026-02-23-ui-design.md`

---

### Task 1: Scaffold Next.js Project

**Files:**
- Create: `web/` (entire Next.js project via create-next-app)
- Modify: `.gitignore`

**Step 1: Create the Next.js app**

```bash
cd c:/Users/yaman/Desktop/agora
npx create-next-app@latest web --typescript --tailwind --eslint --app --src-dir --no-import-alias --use-npm
```

When prompted, accept defaults. This creates `web/` with App Router, TypeScript, Tailwind, ESLint, and `src/` directory.

**Step 2: Verify it runs**

```bash
cd web && npm run dev
```

Open http://localhost:3000 — should see the Next.js welcome page.

**Step 3: Update root .gitignore**

Append to `.gitignore`:

```
# Frontend
web/node_modules/
web/.next/
web/out/
```

**Step 4: Install additional dependencies**

```bash
cd c:/Users/yaman/Desktop/agora/web
npm install recharts
```

**Step 5: Commit**

```bash
git add web/ .gitignore
git commit -m "feat(web): scaffold Next.js project with Tailwind and Recharts"
```

---

### Task 2: Design Tokens — Tailwind Config + Global CSS

**Files:**
- Modify: `web/src/app/globals.css`
- Modify: `web/tailwind.config.ts` (if it exists) OR `web/src/app/globals.css` (Tailwind v4 uses CSS-based config)

**Step 1: Check Tailwind version and config approach**

Tailwind v4 uses CSS-based configuration in `globals.css` with `@theme`. Tailwind v3 uses `tailwind.config.ts`. Check which was scaffolded:

```bash
cat web/package.json | grep tailwind
```

**Step 2: Configure design tokens**

If Tailwind v4 (CSS-based), replace `web/src/app/globals.css` with:

```css
@import "tailwindcss";

@theme {
  /* Surfaces */
  --color-void: hsl(220 20% 7%);
  --color-void-elevated: hsl(220 18% 10%);
  --color-void-hover: hsl(220 16% 13%);

  /* Signals */
  --color-signal: hsl(168 80% 50%);
  --color-signal-dim: hsl(168 40% 25%);
  --color-caution: hsl(38 85% 55%);
  --color-failure: hsl(0 60% 55%);
  --color-base-blue: hsl(222 100% 50%);

  /* Ink */
  --color-ink-primary: hsl(220 15% 90%);
  --color-ink-secondary: hsl(220 12% 62%);
  --color-ink-tertiary: hsl(220 10% 42%);
  --color-ink-muted: hsl(220 8% 28%);

  /* Borders */
  --color-border: hsla(220 15% 50% / 0.12);
  --color-border-soft: hsla(220 15% 50% / 0.06);
  --color-border-focus: hsla(168 80% 50% / 0.5);

  /* Border radius */
  --radius-sm: 4px;
  --radius-md: 6px;
  --radius-lg: 8px;

  /* Fonts */
  --font-sans: "Inter", ui-sans-serif, system-ui, sans-serif;
  --font-mono: "JetBrains Mono", ui-monospace, monospace;
}

@layer base {
  body {
    background-color: var(--color-void);
    color: var(--color-ink-primary);
    font-family: var(--font-sans);
  }
}
```

If Tailwind v3 (JS config), add colors/fonts to `tailwind.config.ts` `theme.extend` and write equivalent CSS variables in globals.css.

**Step 3: Add Google Fonts**

Modify `web/src/app/layout.tsx` to load Inter and JetBrains Mono via `next/font/google`:

```tsx
import { Inter, JetBrains_Mono } from "next/font/google";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-sans",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
});

// In the html tag:
<html lang="en" className={`${inter.variable} ${jetbrainsMono.variable}`}>
```

**Step 4: Verify tokens work**

Replace `web/src/app/page.tsx` with a minimal test:

```tsx
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
```

Run `npm run dev` and verify colors, fonts, surfaces render correctly.

**Step 5: Commit**

```bash
git add web/
git commit -m "feat(web): configure design tokens — palette, typography, spacing"
```

---

### Task 3: Dummy Data Module

**Files:**
- Create: `web/src/lib/types.ts`
- Create: `web/src/lib/dummy-data.ts`

**Step 1: Define TypeScript types matching Go models**

Create `web/src/lib/types.ts`:

```typescript
export interface Endpoint {
  id: string;
  resourceUrl: string;
  domain: string;
  type: string;
  x402Version: number;
  description: string;
  httpMethod: string;
  inputSchema: Record<string, unknown> | null;
  outputSchema: Record<string, unknown> | null;
  lastUpdated: string;   // ISO date
  firstSeen: string;     // ISO date
  lastCrawled: string;   // ISO date
  // UI-only fields (will come from reliability layer later)
  reliabilityScore: number;  // 0-100
  reliabilityTrend: number[];  // last 30 days, 0-100 each
  paymentOptions: PaymentOption[];
}

export interface PaymentOption {
  id: string;
  endpointId: string;
  scheme: string;
  networkRaw: string;
  networkNormalized: string;
  assetAddress: string;
  assetName: string;
  maxAmountRaw: string;
  priceUsd: number;
  payTo: string;
  maxTimeoutSeconds: number;
  mimeType: string;
  description: string;
}

export interface Facilitator {
  domain: string;
  endpointCount: number;
  avgReliability: number;
  reliabilityTrend: number[];  // last 30 days
  networks: string[];
  assets: string[];
  status: "healthy" | "degraded" | "inactive";
}

export interface CrawlRun {
  id: string;
  startedAt: string;
  completedAt: string | null;
  totalFetched: number;
  newEndpoints: number;
  updatedEndpoints: number;
  status: string;
}

export interface NetworkStats {
  totalEndpoints: number;
  totalDomains: number;
  totalFacilitators: number;
  lastCrawl: string;
  endpointsByNetwork: { network: string; count: number }[];
  endpointsByAsset: { asset: string; count: number }[];
  endpointsByPriceBracket: { bracket: string; count: number }[];
  endpointsOverTime: { date: string; count: number }[];
  crawlHistory: CrawlRun[];
}
```

**Step 2: Create dummy data**

Create `web/src/lib/dummy-data.ts` with ~20 realistic endpoints, 5 facilitators, and network stats. Use real-looking domains (ai-translate.x402.dev, weather-api.base.org, etc.) and realistic x402 pricing. Generate reliability trends as arrays of 30 numbers.

This file should export:

```typescript
export const endpoints: Endpoint[] = [/* 20 items */];
export const facilitators: Facilitator[] = [/* 5 items */];
export const networkStats: NetworkStats = {/* realistic aggregates */};
```

**Step 3: Commit**

```bash
git add web/src/lib/
git commit -m "feat(web): add TypeScript types and dummy data matching Go models"
```

---

### Task 4: Layout Shell — Nav + Search

**Files:**
- Create: `web/src/components/nav.tsx`
- Create: `web/src/components/search-bar.tsx`
- Modify: `web/src/app/layout.tsx`

**Step 1: Build the nav component**

Create `web/src/components/nav.tsx`:

```tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const links = [
  { href: "/", label: "Endpoints" },
  { href: "/facilitators", label: "Facilitators" },
  { href: "/network", label: "Network" },
];

export function Nav() {
  const pathname = usePathname();

  return (
    <nav className="border-b border-border bg-void">
      <div className="max-w-7xl mx-auto px-4 h-12 flex items-center gap-8">
        <Link href="/" className="text-signal font-semibold tracking-tight">
          Agora
        </Link>
        <div className="flex gap-6">
          {links.map((link) => {
            const active = link.href === "/"
              ? pathname === "/"
              : pathname.startsWith(link.href);
            return (
              <Link
                key={link.href}
                href={link.href}
                className={`text-sm relative py-4 transition-colors ${
                  active
                    ? "text-ink-primary after:absolute after:bottom-0 after:left-0 after:right-0 after:h-px after:bg-signal"
                    : "text-ink-secondary hover:text-ink-primary"
                }`}
              >
                {link.label}
              </Link>
            );
          })}
        </div>
      </div>
    </nav>
  );
}
```

**Step 2: Build the search bar**

Create `web/src/components/search-bar.tsx`:

```tsx
"use client";

import { useState } from "react";

interface SearchBarProps {
  placeholder?: string;
  onSearch?: (query: string) => void;
}

export function SearchBar({ placeholder = "Search endpoints by description, domain, or schema...", onSearch }: SearchBarProps) {
  const [query, setQuery] = useState("");

  return (
    <div className="w-full">
      <input
        type="text"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          onSearch?.(e.target.value);
        }}
        placeholder={placeholder}
        className="w-full bg-void-elevated border border-border rounded-sm px-4 py-3 text-sm text-ink-primary placeholder:text-ink-muted focus:outline-none focus:border-border-focus transition-colors font-mono"
      />
    </div>
  );
}
```

**Step 3: Wire layout**

Update `web/src/app/layout.tsx` to include Nav:

```tsx
import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";
import { Nav } from "@/components/nav";

const inter = Inter({ subsets: ["latin"], variable: "--font-sans" });
const jetbrainsMono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-mono" });

export const metadata: Metadata = {
  title: "Agora — x402 Endpoint Explorer",
  description: "Search, evaluate, and monitor x402 protocol endpoints",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${inter.variable} ${jetbrainsMono.variable}`}>
      <body className="bg-void text-ink-primary antialiased">
        <Nav />
        <main className="max-w-7xl mx-auto px-4 py-6">
          {children}
        </main>
      </body>
    </html>
  );
}
```

**Step 4: Verify**

Run `npm run dev`. Nav should show "Agora" in teal, three links with active underline, dark background.

**Step 5: Commit**

```bash
git add web/src/
git commit -m "feat(web): add navigation shell and search bar component"
```

---

### Task 5: Endpoints Page (Home)

**Files:**
- Create: `web/src/components/reliability-pulse.tsx`
- Create: `web/src/components/reliability-bar.tsx`
- Create: `web/src/components/endpoints-table.tsx`
- Create: `web/src/components/filter-chips.tsx`
- Modify: `web/src/app/page.tsx`

**Step 1: Build the reliability pulse (signature element)**

Create `web/src/components/reliability-pulse.tsx`:

A small component that renders the ●/◐/○ indicator based on a reliability score. Score > 70 = teal filled, 30-70 = amber half, < 30 = dim empty. Uses a subtle CSS animation (opacity pulse) for healthy endpoints.

```tsx
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
```

Note: The `animate-pulse` is intentionally subtle — Tailwind's default pulse is a gentle opacity fade. This IS the signature — a breathing glow for healthy endpoints.

**Step 2: Build the reliability bar**

Create `web/src/components/reliability-bar.tsx`:

Horizontal fill bar. Width = score%. Color transitions: signal (>70), caution (30-70), failure (<30). Background is `--ink-muted` at low opacity.

```tsx
interface ReliabilityBarProps {
  score: number;
}

export function ReliabilityBar({ score }: ReliabilityBarProps) {
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
```

**Step 3: Build filter chips**

Create `web/src/components/filter-chips.tsx`:

Horizontal row of toggle chips for filtering: Network (Base, Ethereum, etc.), Asset (USDC, ETH, etc.), Reliability (All, >70%, >90%), Method (GET, POST).

```tsx
"use client";

import { useState } from "react";

interface FilterGroup {
  label: string;
  options: string[];
}

interface FilterChipsProps {
  groups: FilterGroup[];
  onFilterChange?: (filters: Record<string, string | null>) => void;
}

export function FilterChips({ groups, onFilterChange }: FilterChipsProps) {
  const [active, setActive] = useState<Record<string, string | null>>({});

  const toggle = (group: string, option: string) => {
    const next = { ...active, [group]: active[group] === option ? null : option };
    setActive(next);
    onFilterChange?.(next);
  };

  return (
    <div className="flex flex-wrap gap-4">
      {groups.map((group) => (
        <div key={group.label} className="flex items-center gap-1.5">
          <span className="text-xs text-ink-tertiary">{group.label}:</span>
          {group.options.map((option) => (
            <button
              key={option}
              onClick={() => toggle(group.label, option)}
              className={`text-xs px-2 py-1 rounded-sm border transition-colors ${
                active[group.label] === option
                  ? "border-signal/40 bg-signal-dim/30 text-signal"
                  : "border-border text-ink-secondary hover:text-ink-primary hover:border-border-focus"
              }`}
            >
              {option}
            </button>
          ))}
        </div>
      ))}
    </div>
  );
}
```

**Step 4: Build the endpoints table**

Create `web/src/components/endpoints-table.tsx`:

Two-line dense rows with pulse, URL (mono), method badge, price, reliability bar, and relative time. Accepts filtered endpoints array.

```tsx
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
                {ep.paymentOptions[0]?.networkNormalized ?? "—"}
              </span>
              <span className="text-xs text-ink-tertiary">
                {ep.paymentOptions[0]?.assetName ?? "—"}
              </span>
              <span className="text-xs text-ink-tertiary">{timeAgo(ep.lastCrawled)}</span>
            </div>
          </div>
          <div className="flex items-center gap-4 shrink-0">
            <span className="text-xs font-mono text-ink-secondary">
              ${ep.paymentOptions[0]?.priceUsd.toFixed(4) ?? "—"}/req
            </span>
            <ReliabilityBar score={ep.reliabilityScore} />
          </div>
        </div>
      ))}
    </div>
  );
}
```

**Step 5: Wire up the home page**

Replace `web/src/app/page.tsx`:

```tsx
"use client";

import { useState, useMemo } from "react";
import { SearchBar } from "@/components/search-bar";
import { FilterChips } from "@/components/filter-chips";
import { EndpointsTable } from "@/components/endpoints-table";
import { endpoints } from "@/lib/dummy-data";

const filterGroups = [
  { label: "Network", options: ["Base", "Ethereum", "Arbitrum"] },
  { label: "Asset", options: ["USDC", "ETH", "DAI"] },
  { label: "Reliability", options: [">90%", ">70%", ">50%"] },
  { label: "Method", options: ["GET", "POST"] },
];

export default function EndpointsPage() {
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState<Record<string, string | null>>({});

  const filtered = useMemo(() => {
    let result = endpoints;
    if (query) {
      const q = query.toLowerCase();
      result = result.filter(
        (ep) =>
          ep.resourceUrl.toLowerCase().includes(q) ||
          ep.description.toLowerCase().includes(q) ||
          ep.domain.toLowerCase().includes(q)
      );
    }
    if (filters.Network) {
      result = result.filter((ep) =>
        ep.paymentOptions.some((po) => po.networkNormalized === filters.Network!.toLowerCase())
      );
    }
    if (filters.Asset) {
      result = result.filter((ep) =>
        ep.paymentOptions.some((po) => po.assetName === filters.Asset)
      );
    }
    if (filters.Method) {
      result = result.filter((ep) => ep.httpMethod === filters.Method);
    }
    if (filters.Reliability) {
      const threshold = parseInt(filters.Reliability!.replace(/[>%]/g, ""));
      result = result.filter((ep) => ep.reliabilityScore >= threshold);
    }
    return result;
  }, [query, filters]);

  return (
    <div className="space-y-4">
      <SearchBar onSearch={setQuery} />
      <FilterChips groups={filterGroups} onFilterChange={setFilters} />
      <div className="flex items-center justify-between">
        <p className="text-xs text-ink-tertiary">
          {filtered.length} endpoint{filtered.length !== 1 ? "s" : ""}
        </p>
      </div>
      <EndpointsTable endpoints={filtered} />
    </div>
  );
}
```

**Step 6: Verify**

Run `npm run dev`. Home page should show search bar, filter chips, and the endpoints table with reliability pulses, URLs in monospace, and reliability bars. Search and filters should work.

**Step 7: Commit**

```bash
git add web/src/
git commit -m "feat(web): build endpoints page with search, filters, and reliability table"
```

---

### Task 6: Facilitators Page

**Files:**
- Create: `web/src/components/sparkline.tsx`
- Create: `web/src/components/facilitator-card.tsx`
- Create: `web/src/app/facilitators/page.tsx`

**Step 1: Build inline sparkline**

Create `web/src/components/sparkline.tsx`:

A tiny SVG sparkline (80x20px) that renders a 30-point reliability trend line. Color matches the facilitator's status.

```tsx
interface SparklineProps {
  data: number[];
  color?: string;
  width?: number;
  height?: number;
}

export function Sparkline({ data, color = "var(--color-signal)", width = 80, height = 20 }: SparklineProps) {
  if (data.length < 2) return null;
  const max = Math.max(...data);
  const min = Math.min(...data);
  const range = max - min || 1;
  const points = data
    .map((v, i) => {
      const x = (i / (data.length - 1)) * width;
      const y = height - ((v - min) / range) * height;
      return `${x},${y}`;
    })
    .join(" ");

  return (
    <svg width={width} height={height} className="inline-block">
      <polyline
        points={points}
        fill="none"
        stroke={color}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
```

**Step 2: Build facilitator card**

Create `web/src/components/facilitator-card.tsx`:

Card with pulse, domain, endpoint count, uptime, sparkline, networks/assets.

```tsx
import { Facilitator } from "@/lib/types";
import { ReliabilityPulse } from "./reliability-pulse";
import { Sparkline } from "./sparkline";

const statusColor: Record<string, string> = {
  healthy: "var(--color-signal)",
  degraded: "var(--color-caution)",
  inactive: "var(--color-ink-muted)",
};

export function FacilitatorCard({ facilitator }: { facilitator: Facilitator }) {
  return (
    <div className="bg-void-elevated border border-border rounded-md p-4 hover:border-border-focus transition-colors">
      <div className="flex items-center gap-2">
        <ReliabilityPulse score={facilitator.avgReliability} />
        <span className="font-mono text-sm text-ink-primary">{facilitator.domain}</span>
      </div>
      <div className="mt-2 flex items-center gap-3 text-xs text-ink-secondary">
        <span>{facilitator.endpointCount} endpoints</span>
        <span>·</span>
        <span>{facilitator.avgReliability.toFixed(1)}% uptime</span>
      </div>
      <div className="mt-3">
        <Sparkline data={facilitator.reliabilityTrend} color={statusColor[facilitator.status]} />
        <span className="text-xs text-ink-tertiary ml-2">30d</span>
      </div>
      <div className="mt-3 flex items-center gap-2 text-xs text-ink-tertiary">
        <span>{facilitator.networks.join(", ")}</span>
        <span>·</span>
        <span>{facilitator.assets.join(", ")}</span>
      </div>
    </div>
  );
}
```

**Step 3: Build facilitators page**

Create `web/src/app/facilitators/page.tsx`:

```tsx
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
```

**Step 4: Verify**

Navigate to /facilitators. Should see a responsive grid of cards with pulse indicators, sparklines, and metadata.

**Step 5: Commit**

```bash
git add web/src/
git commit -m "feat(web): add facilitators page with sparkline cards"
```

---

### Task 7: Network Page — Stats + Charts

**Files:**
- Create: `web/src/components/stat-line.tsx`
- Create: `web/src/components/horizontal-bar-chart.tsx`
- Create: `web/src/components/area-chart-panel.tsx`
- Create: `web/src/app/network/page.tsx`

**Step 1: Build inline stat line**

Create `web/src/components/stat-line.tsx`:

Not a card grid — an inline text line: `12,571 endpoints · 847 domains · 13 facilitators · last crawl 2h ago`

```tsx
import { NetworkStats } from "@/lib/types";

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const hours = Math.floor(diff / 3600000);
  if (hours < 1) return "just now";
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function StatLine({ stats }: { stats: NetworkStats }) {
  return (
    <p className="text-sm text-ink-secondary">
      <span className="text-ink-primary font-mono">{stats.totalEndpoints.toLocaleString()}</span> endpoints
      <span className="text-ink-tertiary mx-2">·</span>
      <span className="text-ink-primary font-mono">{stats.totalDomains.toLocaleString()}</span> domains
      <span className="text-ink-tertiary mx-2">·</span>
      <span className="text-ink-primary font-mono">{stats.totalFacilitators}</span> facilitators
      <span className="text-ink-tertiary mx-2">·</span>
      last crawl <span className="text-ink-primary">{timeAgo(stats.lastCrawl)}</span>
    </p>
  );
}
```

**Step 2: Build horizontal bar chart component**

Create `web/src/components/horizontal-bar-chart.tsx`:

Uses Recharts `BarChart` with horizontal layout. Styled with our tokens.

```tsx
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
```

**Step 3: Build area chart panel**

Create `web/src/components/area-chart-panel.tsx`:

Uses Recharts `AreaChart` for endpoints-over-time. Teal fill with low opacity.

```tsx
"use client";

import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip } from "recharts";

interface AreaChartPanelProps {
  data: { date: string; count: number }[];
  title: string;
}

export function AreaChartPanel({ data, title }: AreaChartPanelProps) {
  return (
    <div className="border border-border-soft rounded-md p-4">
      <h3 className="text-sm font-medium text-ink-secondary mb-4">{title}</h3>
      <ResponsiveContainer width="100%" height={200}>
        <AreaChart data={data} margin={{ left: 0, right: 0, top: 0, bottom: 0 }}>
          <defs>
            <linearGradient id="signalGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="hsl(168, 80%, 50%)" stopOpacity={0.3} />
              <stop offset="100%" stopColor="hsl(168, 80%, 50%)" stopOpacity={0} />
            </linearGradient>
          </defs>
          <XAxis
            dataKey="date"
            tick={{ fill: "hsl(220, 10%, 42%)", fontSize: 11, fontFamily: "var(--font-mono)" }}
            axisLine={false}
            tickLine={false}
          />
          <YAxis hide />
          <Tooltip
            contentStyle={{
              backgroundColor: "hsl(220, 18%, 10%)",
              border: "1px solid hsla(220, 15%, 50%, 0.12)",
              borderRadius: "4px",
              color: "hsl(220, 15%, 90%)",
              fontSize: "12px",
              fontFamily: "var(--font-mono)",
            }}
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
```

**Step 4: Build network page**

Create `web/src/app/network/page.tsx`:

```tsx
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
```

**Step 5: Verify**

Navigate to /network. Should see stat line, area chart, and three horizontal bar charts in a responsive grid.

**Step 6: Commit**

```bash
git add web/src/
git commit -m "feat(web): add network page with stats, area chart, and distribution bars"
```

---

### Task 8: Polish + Build Verification

**Files:**
- Modify: Various components for consistency check

**Step 1: Run the build**

```bash
cd c:/Users/yaman/Desktop/agora/web && npm run build
```

Fix any TypeScript or build errors.

**Step 2: Run linting**

```bash
npm run lint
```

Fix any ESLint issues.

**Step 3: Visual review**

Run `npm run dev` and check all three pages:
- Endpoints: search works, filters toggle, reliability pulses animate, table rows show data
- Facilitators: cards render with sparklines, pulse indicators match status
- Network: stat line renders, charts display, responsive grid works

Verify:
- Dark background is consistent (no white flashes on page transition)
- Fonts load correctly (Inter for labels, JetBrains Mono for data)
- Border opacity is subtle (squint test)
- Signal teal is the only accent color used
- Nav underline follows active page

**Step 4: Commit**

```bash
git add -A web/
git commit -m "chore(web): fix build and lint issues, polish UI consistency"
```

---

## Summary

| Task | What | Commit |
|------|------|--------|
| 1 | Scaffold Next.js + Recharts | `feat(web): scaffold Next.js project` |
| 2 | Design tokens (Tailwind + fonts) | `feat(web): configure design tokens` |
| 3 | TypeScript types + dummy data | `feat(web): add types and dummy data` |
| 4 | Nav + search bar shell | `feat(web): add navigation shell` |
| 5 | Endpoints page (table + filters + search) | `feat(web): build endpoints page` |
| 6 | Facilitators page (cards + sparklines) | `feat(web): add facilitators page` |
| 7 | Network page (stats + charts) | `feat(web): add network page` |
| 8 | Build verification + polish | `chore(web): fix build and lint` |