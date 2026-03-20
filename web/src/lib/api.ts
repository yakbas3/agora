import { FacilitatorStats, TransactionsResponse } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export interface ApiPaymentOption {
  id: string;
  endpoint_id: string;
  scheme: string;
  network_raw: string;
  network_normalized: string;
  asset_address: string;
  asset_name: string;
  max_amount_raw: string;
  price_usd: number;
  pay_to: string;
  max_timeout_seconds: number;
  mime_type: string;
  description: string;
}

export interface ApiEndpoint {
  id: string;
  resource_url: string;
  domain: string;
  type: string;
  x402_version: number;
  description: string;
  http_method: string;
  input_schema: Record<string, unknown> | null;
  output_schema: Record<string, unknown> | null;
  last_updated: string;
  first_seen: string;
  last_crawled: string;
}

export interface ApiEndpointWithPayments {
  endpoint: ApiEndpoint;
  payment_options: ApiPaymentOption[] | null;
}

export interface ApiSearchResult {
  endpoint: ApiEndpoint;
  similarity: number;
}

export interface ApiSearchResponse {
  results: ApiSearchResult[] | null;
  total: number;
  query_time_ms: number;
}

export interface ApiStats {
  total_endpoints: number;
  total_domains: number;
  endpoints_by_network: { name: string; count: number }[];
  endpoints_by_asset: { name: string; count: number }[];
  endpoints_by_price_bracket: { name: string; count: number }[];
  endpoints_over_time: { date: string; count: number }[];
  crawl_history: {
    id: string;
    started_at: string;
    completed_at: string | null;
    total_fetched: number;
    new_endpoints: number;
    updated_endpoints: number;
    status: string;
  }[];
}

export async function fetchEndpoints(
  limit = 20,
  offset = 0
): Promise<ApiEndpointWithPayments[]> {
  const res = await fetch(
    `${API_URL}/api/endpoints?limit=${limit}&offset=${offset}`
  );
  if (!res.ok) throw new Error(`Failed to fetch endpoints: ${res.status}`);
  return res.json();
}

export async function searchEndpoints(
  query: string,
  filters?: { network?: string; method?: string; min_price?: number; max_price?: number },
  limit = 20
): Promise<ApiSearchResponse> {
  const res = await fetch(`${API_URL}/api/search`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query, filters: filters || {}, limit }),
  });
  if (!res.ok) throw new Error(`Search failed: ${res.status}`);
  return res.json();
}

export async function fetchStats(): Promise<ApiStats> {
  const res = await fetch(`${API_URL}/api/stats`);
  if (!res.ok) throw new Error(`Failed to fetch stats: ${res.status}`);
  return res.json();
}

export async function fetchFacilitators(): Promise<FacilitatorStats[]> {
  const res = await fetch(`${API_URL}/api/facilitators`);
  if (!res.ok) throw new Error(`Failed to fetch facilitators: ${res.status}`);
  return res.json();
}

export async function fetchTransactions(
  limit = 50,
  offset = 0,
  facilitator?: string
): Promise<TransactionsResponse> {
  let url = `${API_URL}/api/transactions?limit=${limit}&offset=${offset}`;
  if (facilitator) url += `&facilitator=${encodeURIComponent(facilitator)}`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Failed to fetch transactions: ${res.status}`);
  return res.json();
}
