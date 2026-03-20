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

export interface FacilitatorStats {
  id: string;
  name: string;
  chain: string;
  address: string;
  last_synced_at: string | null;
  tx_count: number;
  total_volume_usd: number;
  unique_payers: number;
}

export interface Transaction {
  id: string;
  tx_hash: string;
  block_number: number;
  block_time: string;
  event_type: string;
  facilitator_address: string;
  payer_address: string;
  recipient_address: string;
  amount_raw: string;
  amount_usd: number;
  asset_address: string;
  indexed_at: string;
  facilitator_name: string;
}

export interface TransactionsResponse {
  transactions: Transaction[];
  total: number;
  limit: number;
  offset: number;
}
