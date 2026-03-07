import type { Endpoint, PaymentOption, NetworkStats } from "./types";
import type {
  ApiEndpointWithPayments,
  ApiSearchResult,
  ApiPaymentOption,
  ApiStats,
} from "./api";

function transformPaymentOption(po: ApiPaymentOption): PaymentOption {
  return {
    id: po.id,
    endpointId: po.endpoint_id,
    scheme: po.scheme,
    networkRaw: po.network_raw,
    networkNormalized: po.network_normalized,
    assetAddress: po.asset_address,
    assetName: po.asset_name,
    maxAmountRaw: po.max_amount_raw,
    priceUsd: po.price_usd,
    payTo: po.pay_to,
    maxTimeoutSeconds: po.max_timeout_seconds,
    mimeType: po.mime_type,
    description: po.description,
  };
}

function transformEndpoint(
  api: { id: string; resource_url: string; domain: string; type: string; x402_version: number; description: string; http_method: string; input_schema: Record<string, unknown> | null; output_schema: Record<string, unknown> | null; last_updated: string; first_seen: string; last_crawled: string },
  paymentOptions: PaymentOption[]
): Endpoint {
  return {
    id: api.id,
    resourceUrl: api.resource_url,
    domain: api.domain,
    type: api.type,
    x402Version: api.x402_version,
    description: api.description,
    httpMethod: api.http_method,
    inputSchema: api.input_schema,
    outputSchema: api.output_schema,
    lastUpdated: api.last_updated,
    firstSeen: api.first_seen,
    lastCrawled: api.last_crawled,
    paymentOptions,
  };
}

export function transformEndpointWithPayments(
  ewp: ApiEndpointWithPayments
): Endpoint {
  const pos = (ewp.payment_options || []).map(transformPaymentOption);
  return transformEndpoint(ewp.endpoint, pos);
}

export function transformSearchResult(sr: ApiSearchResult): Endpoint {
  return transformEndpoint(sr.endpoint, []);
}

export function transformStats(api: ApiStats): NetworkStats {
  return {
    totalEndpoints: api.total_endpoints,
    totalDomains: api.total_domains,
    endpointsByNetwork: api.endpoints_by_network || [],
    endpointsByAsset: api.endpoints_by_asset || [],
    endpointsByPriceBracket: api.endpoints_by_price_bracket || [],
    endpointsOverTime: api.endpoints_over_time || [],
    crawlHistory: (api.crawl_history || []).map((cr) => ({
      id: cr.id,
      startedAt: cr.started_at,
      completedAt: cr.completed_at,
      totalFetched: cr.total_fetched,
      newEndpoints: cr.new_endpoints,
      updatedEndpoints: cr.updated_endpoints,
      status: cr.status,
    })),
  };
}
