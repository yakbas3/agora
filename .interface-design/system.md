# Agora Design System

## Direction

**Observatory** — a network operations console looking into a live x402 endpoint network. Dark canvas where data points glow as signals. Dense but clear. Precise, alive, technical.

**Signature:** Reliability pulse — a breathing ●/◐/○ indicator on every endpoint and facilitator. Teal = healthy, amber = degraded, dim = inactive.

## Depth Strategy

**Borders-only.** No shadows. Dark backgrounds swallow shadows — low-opacity hsla borders provide quiet structure that glows faintly against the void.

## Palette

| Token | Value | Role |
|-------|-------|------|
| `void` | `hsl(220 20% 7%)` | Canvas |
| `void-elevated` | `hsl(220 18% 10%)` | Cards, panels, nav |
| `void-hover` | `hsl(220 16% 13%)` | Hover, active, dropdowns |
| `signal` | `hsl(168 80% 50%)` | Teal — healthy, active, primary accent |
| `signal-dim` | `hsl(168 40% 25%)` | Muted signal backgrounds |
| `caution` | `hsl(38 85% 55%)` | Amber — degraded reliability |
| `failure` | `hsl(0 60% 55%)` | Coral — down/error |
| `base-blue` | `hsl(222 100% 50%)` | BASE identity, used sparingly |
| `ink-primary` | `hsl(220 15% 90%)` | Primary text |
| `ink-secondary` | `hsl(220 12% 62%)` | Supporting text |
| `ink-tertiary` | `hsl(220 10% 42%)` | Metadata, timestamps |
| `ink-muted` | `hsl(220 8% 28%)` | Disabled, placeholder |
| `border` | `hsla(220 15% 50% / 0.12)` | Standard separation |
| `border-soft` | `hsla(220 15% 50% / 0.06)` | Subtle separation |
| `border-focus` | `hsla(168 80% 50% / 0.5)` | Focus rings |

## Typography

| Role | Font | Weight | Tracking |
|------|------|--------|----------|
| Headlines | Inter | 600 | -0.02em |
| Body/Labels | Inter | 400/500 | normal |
| Data (URLs, hashes, prices) | JetBrains Mono | 400 | tabular-nums |

## Spacing

Base unit: **4px**. Scale: 4, 8, 12, 16, 24, 32, 48, 64.

## Border Radius

| Context | Value |
|---------|-------|
| Inputs, buttons | 4px (`rounded-sm`) |
| Cards, panels | 6px (`rounded-md`) |
| Modals, popovers | 8px (`rounded-lg`) |

## Layout

- **Top nav**, no sidebar — only 3-4 pages, maximize horizontal space for data tables
- Active link: `ink-primary` text + `signal` underline
- Logo "Agora" in `signal` teal
- Max width: `max-w-7xl` (1280px) centered

## Component Patterns

### Reliability Pulse
- `score > 70`: teal ● with `animate-pulse`
- `score > 30`: amber ◐
- `score <= 30`: dim ○

### Reliability Bar
- Horizontal fill, width = score%
- Color: `signal` (>70), `caution` (30-70), `failure` (<30)
- Background: `ink-muted/20`
- Height: 6px (`h-1.5`)

### Endpoints Table Row
- Two-line dense rows (py-3)
- Leading: reliability pulse
- Line 1: URL (mono) + method badge (color-coded)
- Line 2: description + network + asset + relative time
- Right: price/req (mono) + reliability bar
- Row separator: `border-border-soft`
- Hover: `bg-void-hover`

### Facilitator Card
- Surface: `bg-void-elevated`, `border-border`, `rounded-md`
- Hover: `border-border-focus`
- Content: pulse + domain, stats, sparkline (30d), networks/assets

### Sparkline
- Inline SVG, 80x20px default
- Polyline, stroke 1.5px, rounded caps
- Color matches status

### Filter Chips
- Groups with label + toggle buttons
- Active: `border-signal/40`, `bg-signal-dim/30`, `text-signal`
- Inactive: `border-border`, `text-ink-secondary`

### Stat Line
- Inline text, not card grid
- Numbers in `font-mono text-ink-primary`
- Labels in `text-ink-secondary`
- Separators: `·` in `text-ink-tertiary`

### Charts (Recharts)
- Area chart: teal stroke, gradient fill 30%→0% opacity
- Bar chart: horizontal layout, mono font labels
- Tooltip: `void-elevated` background, `border` border, mono font
- Each chart in `border-border-soft rounded-md p-4` container

## Search Bar
- Full-width, `bg-void-elevated`, `border-border`
- Mono font for input text
- Focus: `border-border-focus`
- Placeholder: `text-ink-muted`

## HTTP Method Colors
- GET: `text-signal`
- POST: `text-base-blue`
- PUT: `text-caution`
- DELETE: `text-failure`
