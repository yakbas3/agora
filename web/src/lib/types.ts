// TypeScript types mirroring Go models in internal/models/
// with additional UI-specific fields for the frontend.

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
  lastUpdated: string; // ISO date
  firstSeen: string; // ISO date
  lastCrawled: string; // ISO date
  // UI-only fields (will come from reliability layer later)
  reliabilityScore: number; // 0-100
  reliabilityTrend: number[]; // last 30 days, 0-100 each
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
  reliabilityTrend: number[]; // last 30 days
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
