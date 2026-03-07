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
  lastUpdated: string;
  firstSeen: string;
  lastCrawled: string;
  reliabilityScore?: number;
  reliabilityTrend?: number[];
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
  reliabilityTrend: number[];
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
  endpointsByNetwork: { name: string; count: number }[];
  endpointsByAsset: { name: string; count: number }[];
  endpointsByPriceBracket: { name: string; count: number }[];
  endpointsOverTime: { date: string; count: number }[];
  crawlHistory: CrawlRun[];
}
