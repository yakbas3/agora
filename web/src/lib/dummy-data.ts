import type {
  Endpoint,
  Facilitator,
  NetworkStats,
  CrawlRun,
  PaymentOption,
} from "./types";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** ISO timestamp relative to "now" (page-load time). */
function ago(hours: number): string {
  return new Date(Date.now() - hours * 3_600_000).toISOString();
}

/** ISO date string for N days ago (midnight UTC). */
function daysAgo(days: number): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() - days);
  d.setUTCHours(0, 0, 0, 0);
  return d.toISOString();
}

/** Generate a 30-day reliability trend array. */
function trend(
  base: number,
  volatility: number,
  direction: "up" | "down" | "stable" = "stable",
): number[] {
  const points: number[] = [];
  let val = base - (direction === "up" ? 10 : direction === "down" ? -10 : 0);
  for (let i = 0; i < 30; i++) {
    const drift =
      direction === "up" ? 0.35 : direction === "down" ? -0.35 : 0;
    val += drift + (Math.random() - 0.5) * volatility;
    points.push(Math.round(Math.min(100, Math.max(0, val))));
  }
  return points;
}

/** USDC token address on Base. */
const USDC_BASE = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913";
/** USDC token address on Ethereum. */
const USDC_ETH = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48";
/** WETH address on Base. */
const WETH_BASE = "0x4200000000000000000000000000000000000006";
/** DAI address on Ethereum. */
const DAI_ETH = "0x6B175474E89094C44Da98b954EedeAC495271d0F";

/** Deterministic UUID-like id from a seed number. */
function eid(n: number): string {
  const hex = n.toString(16).padStart(4, "0");
  return `a0e1f2d3-${hex}-4b5c-9d8e-f0a1b2c3d4e5`;
}
function pid(n: number): string {
  const hex = n.toString(16).padStart(4, "0");
  return `b1f2e3d4-${hex}-4a6b-8c7d-e0f1a2b3c4d5`;
}

// ---------------------------------------------------------------------------
// Payment option builder
// ---------------------------------------------------------------------------

function usdcBasePayment(
  endpointId: string,
  paymentId: string,
  priceUsd: number,
  payTo: string,
  timeout = 30,
): PaymentOption {
  return {
    id: paymentId,
    endpointId,
    scheme: "exact",
    networkRaw: "eip155:8453",
    networkNormalized: "base",
    assetAddress: USDC_BASE,
    assetName: "USDC",
    maxAmountRaw: String(Math.round(priceUsd * 1e6)),
    priceUsd,
    payTo,
    maxTimeoutSeconds: timeout,
    mimeType: "application/json",
    description: "USDC on Base",
  };
}

function ethBasePayment(
  endpointId: string,
  paymentId: string,
  priceUsd: number,
  payTo: string,
  timeout = 30,
): PaymentOption {
  return {
    id: paymentId,
    endpointId,
    scheme: "exact",
    networkRaw: "eip155:8453",
    networkNormalized: "base",
    assetAddress: WETH_BASE,
    assetName: "WETH",
    maxAmountRaw: String(Math.round((priceUsd / 3200) * 1e18)),
    priceUsd,
    payTo,
    maxTimeoutSeconds: timeout,
    mimeType: "application/json",
    description: "WETH on Base",
  };
}

function usdcEthPayment(
  endpointId: string,
  paymentId: string,
  priceUsd: number,
  payTo: string,
  timeout = 60,
): PaymentOption {
  return {
    id: paymentId,
    endpointId,
    scheme: "exact",
    networkRaw: "eip155:1",
    networkNormalized: "ethereum",
    assetAddress: USDC_ETH,
    assetName: "USDC",
    maxAmountRaw: String(Math.round(priceUsd * 1e6)),
    priceUsd,
    payTo,
    maxTimeoutSeconds: timeout,
    mimeType: "application/json",
    description: "USDC on Ethereum",
  };
}

function daiEthPayment(
  endpointId: string,
  paymentId: string,
  priceUsd: number,
  payTo: string,
  timeout = 60,
): PaymentOption {
  return {
    id: paymentId,
    endpointId,
    scheme: "exact",
    networkRaw: "eip155:1",
    networkNormalized: "ethereum",
    assetAddress: DAI_ETH,
    assetName: "DAI",
    maxAmountRaw: String(Math.round(priceUsd * 1e18)),
    priceUsd,
    payTo,
    maxTimeoutSeconds: timeout,
    mimeType: "application/json",
    description: "DAI on Ethereum",
  };
}

// ---------------------------------------------------------------------------
// Wallet addresses (deterministic fakes)
// ---------------------------------------------------------------------------

const wallets = {
  coinbase: "0x1a2B3c4D5e6F7890AbCdEf1234567890aBcDeF12",
  x402Pay: "0x2b3C4d5E6f7A8901BcDeF01234567890AbCdEf23",
  basePay: "0x3c4D5e6F7a8B9012CdEf012345678901BcDeF034",
  pay402: "0x4d5E6f7A8b9C0123DeF0123456789012CdEf0145",
  ethLink: "0x5e6F7a8B9c0D1234Ef01234567890123DeF01256",
};

// ---------------------------------------------------------------------------
// Endpoints (20 total)
// ---------------------------------------------------------------------------

export const endpoints: Endpoint[] = [
  // ── AI / ML endpoints ──────────────────────────────────────────────────
  {
    id: eid(1),
    resourceUrl: "https://translate-api.coinbase.com/v1/translate",
    domain: "translate-api.coinbase.com",
    type: "paywall",
    x402Version: 1,
    description:
      "Neural machine translation supporting 48 language pairs. Includes auto-detection, formality controls, and domain-specific glossaries.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        text: { type: "string" },
        source_lang: { type: "string" },
        target_lang: { type: "string" },
      },
      required: ["text", "target_lang"],
    },
    outputSchema: {
      type: "object",
      properties: {
        translated_text: { type: "string" },
        detected_lang: { type: "string" },
      },
    },
    lastUpdated: ago(2),
    firstSeen: daysAgo(45),
    lastCrawled: ago(1),
    reliabilityScore: 97,
    reliabilityTrend: trend(97, 3, "stable"),
    paymentOptions: [
      usdcBasePayment(eid(1), pid(1), 0.002, wallets.coinbase),
      ethBasePayment(eid(1), pid(2), 0.002, wallets.coinbase),
    ],
  },
  {
    id: eid(2),
    resourceUrl: "https://images.x402-pay.dev/v2/generate",
    domain: "images.x402-pay.dev",
    type: "paywall",
    x402Version: 1,
    description:
      "Text-to-image generation powered by Stable Diffusion XL. Supports 512-1024px, multiple styles, and negative prompts.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        prompt: { type: "string" },
        width: { type: "number" },
        height: { type: "number" },
        style: { type: "string" },
      },
      required: ["prompt"],
    },
    outputSchema: {
      type: "object",
      properties: {
        image_url: { type: "string" },
        seed: { type: "number" },
      },
    },
    lastUpdated: ago(5),
    firstSeen: daysAgo(30),
    lastCrawled: ago(2),
    reliabilityScore: 93,
    reliabilityTrend: trend(93, 5, "up"),
    paymentOptions: [
      usdcBasePayment(eid(2), pid(3), 0.01, wallets.x402Pay),
    ],
  },
  {
    id: eid(3),
    resourceUrl: "https://nlp.coinbase.com/v1/summarize",
    domain: "nlp.coinbase.com",
    type: "paywall",
    x402Version: 1,
    description:
      "Extractive and abstractive text summarization. Handles documents up to 50,000 tokens with adjustable compression ratio.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        text: { type: "string" },
        max_length: { type: "number" },
        mode: { type: "string", enum: ["extractive", "abstractive"] },
      },
      required: ["text"],
    },
    outputSchema: {
      type: "object",
      properties: {
        summary: { type: "string" },
        token_count: { type: "number" },
      },
    },
    lastUpdated: ago(3),
    firstSeen: daysAgo(60),
    lastCrawled: ago(1),
    reliabilityScore: 99,
    reliabilityTrend: trend(99, 1, "stable"),
    paymentOptions: [
      usdcBasePayment(eid(3), pid(4), 0.005, wallets.coinbase),
    ],
  },
  {
    id: eid(4),
    resourceUrl: "https://ml.basepay.io/sentiment",
    domain: "ml.basepay.io",
    type: "paywall",
    x402Version: 1,
    description:
      "Sentiment analysis for English text. Returns positive/negative/neutral scores plus per-sentence breakdown.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        text: { type: "string" },
        granularity: { type: "string", enum: ["document", "sentence"] },
      },
      required: ["text"],
    },
    outputSchema: {
      type: "object",
      properties: {
        sentiment: { type: "string" },
        confidence: { type: "number" },
      },
    },
    lastUpdated: ago(48),
    firstSeen: daysAgo(90),
    lastCrawled: ago(26),
    reliabilityScore: 45,
    reliabilityTrend: trend(45, 20, "down"),
    paymentOptions: [
      usdcBasePayment(eid(4), pid(5), 0.001, wallets.basePay),
    ],
  },
  {
    id: eid(5),
    resourceUrl: "https://speech.x402-pay.dev/v1/tts",
    domain: "speech.x402-pay.dev",
    type: "paywall",
    x402Version: 1,
    description:
      "Text-to-speech synthesis with 12 natural voices. Returns mp3 or wav audio at 22kHz-48kHz sample rates.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        text: { type: "string" },
        voice: { type: "string" },
        format: { type: "string", enum: ["mp3", "wav"] },
      },
      required: ["text"],
    },
    outputSchema: {
      type: "object",
      properties: {
        audio_url: { type: "string" },
        duration_seconds: { type: "number" },
      },
    },
    lastUpdated: ago(8),
    firstSeen: daysAgo(21),
    lastCrawled: ago(3),
    reliabilityScore: 91,
    reliabilityTrend: trend(91, 4, "up"),
    paymentOptions: [
      usdcBasePayment(eid(5), pid(6), 0.008, wallets.x402Pay),
      usdcEthPayment(eid(5), pid(7), 0.008, wallets.x402Pay),
    ],
  },
  {
    id: eid(6),
    resourceUrl: "https://vision.coinbase.com/v1/ocr",
    domain: "vision.coinbase.com",
    type: "paywall",
    x402Version: 1,
    description:
      "Optical character recognition for images and PDFs. Supports 25 languages, handwriting detection, and table extraction.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        image_url: { type: "string" },
        languages: { type: "array", items: { type: "string" } },
      },
      required: ["image_url"],
    },
    outputSchema: {
      type: "object",
      properties: {
        text: { type: "string" },
        confidence: { type: "number" },
        blocks: { type: "array" },
      },
    },
    lastUpdated: ago(6),
    firstSeen: daysAgo(38),
    lastCrawled: ago(2),
    reliabilityScore: 95,
    reliabilityTrend: trend(95, 3, "stable"),
    paymentOptions: [
      usdcBasePayment(eid(6), pid(8), 0.003, wallets.coinbase),
    ],
  },

  // ── Data endpoints ─────────────────────────────────────────────────────
  {
    id: eid(7),
    resourceUrl: "https://weather.pay402.xyz/v2/forecast",
    domain: "weather.pay402.xyz",
    type: "paywall",
    x402Version: 1,
    description:
      "7-day weather forecast with hourly granularity. Covers 200,000+ locations worldwide with precipitation probability, UV index, and air quality.",
    httpMethod: "GET",
    inputSchema: null,
    outputSchema: {
      type: "object",
      properties: {
        location: { type: "object" },
        forecast: { type: "array" },
      },
    },
    lastUpdated: ago(1),
    firstSeen: daysAgo(120),
    lastCrawled: ago(1),
    reliabilityScore: 94,
    reliabilityTrend: trend(94, 3, "stable"),
    paymentOptions: [
      usdcBasePayment(eid(7), pid(9), 0.0005, wallets.pay402),
    ],
  },
  {
    id: eid(8),
    resourceUrl: "https://market.coinbase.com/v1/stocks/quote",
    domain: "market.coinbase.com",
    type: "paywall",
    x402Version: 1,
    description:
      "Real-time stock quotes for US exchanges. Includes bid/ask spread, volume, market cap, and after-hours data. 15-minute delay for free tier.",
    httpMethod: "GET",
    inputSchema: null,
    outputSchema: {
      type: "object",
      properties: {
        symbol: { type: "string" },
        price: { type: "number" },
        change: { type: "number" },
      },
    },
    lastUpdated: ago(0.5),
    firstSeen: daysAgo(55),
    lastCrawled: ago(1),
    reliabilityScore: 98,
    reliabilityTrend: trend(98, 2, "stable"),
    paymentOptions: [
      usdcBasePayment(eid(8), pid(10), 0.001, wallets.coinbase),
      ethBasePayment(eid(8), pid(11), 0.001, wallets.coinbase),
    ],
  },
  {
    id: eid(9),
    resourceUrl: "https://data.pay402.xyz/v1/crypto/rates",
    domain: "data.pay402.xyz",
    type: "paywall",
    x402Version: 1,
    description:
      "Live cryptocurrency exchange rates aggregated from 15 DEXs and 8 CEXs. Supports 2,000+ token pairs with VWAP pricing.",
    httpMethod: "GET",
    inputSchema: null,
    outputSchema: {
      type: "object",
      properties: {
        base: { type: "string" },
        quote: { type: "string" },
        rate: { type: "number" },
        sources: { type: "array" },
      },
    },
    lastUpdated: ago(0.25),
    firstSeen: daysAgo(90),
    lastCrawled: ago(1),
    reliabilityScore: 96,
    reliabilityTrend: trend(96, 2, "stable"),
    paymentOptions: [
      usdcBasePayment(eid(9), pid(12), 0.0002, wallets.pay402),
    ],
  },
  {
    id: eid(10),
    resourceUrl: "https://geo.x402-pay.dev/v1/geocode",
    domain: "geo.x402-pay.dev",
    type: "paywall",
    x402Version: 1,
    description:
      "Forward and reverse geocoding. Converts addresses to coordinates and vice versa. Covers 250+ countries with sub-building precision.",
    httpMethod: "GET",
    inputSchema: null,
    outputSchema: {
      type: "object",
      properties: {
        lat: { type: "number" },
        lng: { type: "number" },
        formatted_address: { type: "string" },
      },
    },
    lastUpdated: ago(12),
    firstSeen: daysAgo(25),
    lastCrawled: ago(3),
    reliabilityScore: 88,
    reliabilityTrend: trend(88, 6, "up"),
    paymentOptions: [
      usdcBasePayment(eid(10), pid(13), 0.0003, wallets.x402Pay),
    ],
  },
  {
    id: eid(11),
    resourceUrl: "https://feeds.facilitator.eth.link/v1/nft-floor",
    domain: "feeds.facilitator.eth.link",
    type: "paywall",
    x402Version: 1,
    description:
      "NFT collection floor price feeds from OpenSea, Blur, and X2Y2. Updated every 5 minutes for top 500 collections.",
    httpMethod: "GET",
    inputSchema: null,
    outputSchema: {
      type: "object",
      properties: {
        collection: { type: "string" },
        floor_eth: { type: "number" },
        volume_24h: { type: "number" },
      },
    },
    lastUpdated: ago(336),
    firstSeen: daysAgo(180),
    lastCrawled: ago(168),
    reliabilityScore: 8,
    reliabilityTrend: trend(8, 10, "down"),
    paymentOptions: [
      daiEthPayment(eid(11), pid(14), 0.001, wallets.ethLink),
    ],
  },

  // ── Utility endpoints ──────────────────────────────────────────────────
  {
    id: eid(12),
    resourceUrl: "https://tools.coinbase.com/v1/pdf/convert",
    domain: "tools.coinbase.com",
    type: "paywall",
    x402Version: 1,
    description:
      "Convert HTML, DOCX, or Markdown to PDF. Supports custom headers/footers, page sizes, and embedded fonts up to 20 MB input.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        content: { type: "string" },
        format: { type: "string", enum: ["html", "docx", "markdown"] },
        page_size: { type: "string" },
      },
      required: ["content", "format"],
    },
    outputSchema: {
      type: "object",
      properties: { pdf_url: { type: "string" }, pages: { type: "number" } },
    },
    lastUpdated: ago(4),
    firstSeen: daysAgo(40),
    lastCrawled: ago(1),
    reliabilityScore: 96,
    reliabilityTrend: trend(96, 2, "stable"),
    paymentOptions: [
      usdcBasePayment(eid(12), pid(15), 0.003, wallets.coinbase),
    ],
  },
  {
    id: eid(13),
    resourceUrl: "https://verify.pay402.xyz/v1/email",
    domain: "verify.pay402.xyz",
    type: "paywall",
    x402Version: 1,
    description:
      "Email address validation and deliverability check. Verifies MX records, detects disposable providers, catches typos in popular domains.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: { email: { type: "string" } },
      required: ["email"],
    },
    outputSchema: {
      type: "object",
      properties: {
        valid: { type: "boolean" },
        deliverable: { type: "boolean" },
        disposable: { type: "boolean" },
      },
    },
    lastUpdated: ago(10),
    firstSeen: daysAgo(75),
    lastCrawled: ago(2),
    reliabilityScore: 89,
    reliabilityTrend: trend(89, 5, "stable"),
    paymentOptions: [
      usdcBasePayment(eid(13), pid(16), 0.0001, wallets.pay402),
    ],
  },
  {
    id: eid(14),
    resourceUrl: "https://links.x402-pay.dev/v1/shorten",
    domain: "links.x402-pay.dev",
    type: "paywall",
    x402Version: 1,
    description:
      "URL shortening with click analytics. Custom slugs available. Tracks geographic distribution, referrers, and device types for 90 days.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        url: { type: "string" },
        custom_slug: { type: "string" },
      },
      required: ["url"],
    },
    outputSchema: {
      type: "object",
      properties: {
        short_url: { type: "string" },
        expires_at: { type: "string" },
      },
    },
    lastUpdated: ago(24),
    firstSeen: daysAgo(15),
    lastCrawled: ago(5),
    reliabilityScore: 82,
    reliabilityTrend: trend(82, 8, "up"),
    paymentOptions: [
      usdcBasePayment(eid(14), pid(17), 0.0001, wallets.x402Pay),
    ],
  },
  {
    id: eid(15),
    resourceUrl: "https://tools.basepay.io/v1/qr",
    domain: "tools.basepay.io",
    type: "paywall",
    x402Version: 1,
    description:
      "QR code generation with embedded logos. Supports PNG, SVG, and PDF output. Error correction levels L through H.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        data: { type: "string" },
        size: { type: "number" },
        format: { type: "string", enum: ["png", "svg", "pdf"] },
      },
      required: ["data"],
    },
    outputSchema: {
      type: "object",
      properties: { image_url: { type: "string" } },
    },
    lastUpdated: ago(72),
    firstSeen: daysAgo(60),
    lastCrawled: ago(48),
    reliabilityScore: 38,
    reliabilityTrend: trend(38, 18, "down"),
    paymentOptions: [
      usdcBasePayment(eid(15), pid(18), 0.0005, wallets.basePay),
    ],
  },
  {
    id: eid(16),
    resourceUrl: "https://auth.pay402.xyz/v1/captcha/verify",
    domain: "auth.pay402.xyz",
    type: "paywall",
    x402Version: 1,
    description:
      "Privacy-preserving CAPTCHA verification without cookies or fingerprinting. Returns a signed attestation token valid for 10 minutes.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        challenge_id: { type: "string" },
        response: { type: "string" },
      },
      required: ["challenge_id", "response"],
    },
    outputSchema: {
      type: "object",
      properties: {
        valid: { type: "boolean" },
        token: { type: "string" },
      },
    },
    lastUpdated: ago(6),
    firstSeen: daysAgo(50),
    lastCrawled: ago(2),
    reliabilityScore: 92,
    reliabilityTrend: trend(92, 4, "stable"),
    paymentOptions: [
      usdcBasePayment(eid(16), pid(19), 0.0002, wallets.pay402),
    ],
  },
  {
    id: eid(17),
    resourceUrl: "https://search.coinbase.com/v1/web",
    domain: "search.coinbase.com",
    type: "paywall",
    x402Version: 1,
    description:
      "Web search API with 10B+ page index. Returns snippets, related queries, and knowledge graph entities. Rate limit: 100 req/s.",
    httpMethod: "GET",
    inputSchema: null,
    outputSchema: {
      type: "object",
      properties: {
        results: { type: "array" },
        total: { type: "number" },
      },
    },
    lastUpdated: ago(1),
    firstSeen: daysAgo(35),
    lastCrawled: ago(1),
    reliabilityScore: 99,
    reliabilityTrend: trend(99, 1, "stable"),
    paymentOptions: [
      usdcBasePayment(eid(17), pid(20), 0.005, wallets.coinbase),
      ethBasePayment(eid(17), pid(21), 0.005, wallets.coinbase),
    ],
  },
  {
    id: eid(18),
    resourceUrl: "https://defi.facilitator.eth.link/v1/swap-quote",
    domain: "defi.facilitator.eth.link",
    type: "paywall",
    x402Version: 1,
    description:
      "DeFi swap quote aggregator. Finds optimal routes across Uniswap, Curve, and Balancer with MEV protection.",
    httpMethod: "GET",
    inputSchema: null,
    outputSchema: {
      type: "object",
      properties: {
        route: { type: "array" },
        expected_output: { type: "string" },
        price_impact: { type: "number" },
      },
    },
    lastUpdated: ago(480),
    firstSeen: daysAgo(150),
    lastCrawled: ago(336),
    reliabilityScore: 15,
    reliabilityTrend: trend(15, 12, "down"),
    paymentOptions: [
      daiEthPayment(eid(18), pid(22), 0.002, wallets.ethLink),
    ],
  },
  {
    id: eid(19),
    resourceUrl: "https://images.basepay.io/v1/resize",
    domain: "images.basepay.io",
    type: "paywall",
    x402Version: 1,
    description:
      "On-the-fly image resizing and format conversion. Supports JPEG, PNG, WebP, and AVIF. Max input 25 MB.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        image_url: { type: "string" },
        width: { type: "number" },
        height: { type: "number" },
        format: { type: "string" },
      },
      required: ["image_url"],
    },
    outputSchema: {
      type: "object",
      properties: {
        output_url: { type: "string" },
        size_bytes: { type: "number" },
      },
    },
    lastUpdated: ago(96),
    firstSeen: daysAgo(80),
    lastCrawled: ago(72),
    reliabilityScore: 61,
    reliabilityTrend: trend(61, 15, "down"),
    paymentOptions: [
      usdcBasePayment(eid(19), pid(23), 0.001, wallets.basePay),
    ],
  },
  {
    id: eid(20),
    resourceUrl: "https://storage.pay402.xyz/v1/ipfs/pin",
    domain: "storage.pay402.xyz",
    type: "paywall",
    x402Version: 1,
    description:
      "IPFS pinning service with guaranteed 99.9% availability. Supports files up to 1 GB with CID v1. Pins retained for 1 year.",
    httpMethod: "POST",
    inputSchema: {
      type: "object",
      properties: {
        cid: { type: "string" },
        name: { type: "string" },
      },
      required: ["cid"],
    },
    outputSchema: {
      type: "object",
      properties: {
        pin_id: { type: "string" },
        status: { type: "string" },
        gateway_url: { type: "string" },
      },
    },
    lastUpdated: ago(14),
    firstSeen: daysAgo(28),
    lastCrawled: ago(3),
    reliabilityScore: 87,
    reliabilityTrend: trend(87, 6, "up"),
    paymentOptions: [
      usdcBasePayment(eid(20), pid(24), 0.05, wallets.pay402),
      usdcEthPayment(eid(20), pid(25), 0.05, wallets.pay402, 120),
    ],
  },
];

// ---------------------------------------------------------------------------
// Facilitators (5)
// ---------------------------------------------------------------------------

export const facilitators: Facilitator[] = [
  {
    domain: "coinbase.com",
    endpointCount: 15,
    avgReliability: 98.5,
    reliabilityTrend: trend(98.5, 1, "stable"),
    networks: ["base", "ethereum"],
    assets: ["USDC", "WETH"],
    status: "healthy",
  },
  {
    domain: "x402-pay.dev",
    endpointCount: 8,
    avgReliability: 95.2,
    reliabilityTrend: trend(95.2, 3, "up"),
    networks: ["base", "ethereum"],
    assets: ["USDC"],
    status: "healthy",
  },
  {
    domain: "basepay.io",
    endpointCount: 4,
    avgReliability: 52.3,
    reliabilityTrend: trend(52.3, 18, "down"),
    networks: ["base"],
    assets: ["USDC"],
    status: "degraded",
  },
  {
    domain: "pay402.xyz",
    endpointCount: 6,
    avgReliability: 91.7,
    reliabilityTrend: trend(91.7, 4, "stable"),
    networks: ["base", "ethereum"],
    assets: ["USDC"],
    status: "healthy",
  },
  {
    domain: "facilitator.eth.link",
    endpointCount: 2,
    avgReliability: 12.1,
    reliabilityTrend: trend(12.1, 10, "down"),
    networks: ["ethereum"],
    assets: ["DAI"],
    status: "inactive",
  },
];

// ---------------------------------------------------------------------------
// Crawl history (5 recent runs)
// ---------------------------------------------------------------------------

const crawlHistory: CrawlRun[] = [
  {
    id: "c0000001-aaaa-4bbb-8ccc-ddddeeee0001",
    startedAt: ago(2),
    completedAt: ago(1.5),
    totalFetched: 12571,
    newEndpoints: 23,
    updatedEndpoints: 412,
    status: "completed",
  },
  {
    id: "c0000001-aaaa-4bbb-8ccc-ddddeeee0002",
    startedAt: ago(26),
    completedAt: ago(25.5),
    totalFetched: 12548,
    newEndpoints: 47,
    updatedEndpoints: 389,
    status: "completed",
  },
  {
    id: "c0000001-aaaa-4bbb-8ccc-ddddeeee0003",
    startedAt: ago(50),
    completedAt: ago(49.3),
    totalFetched: 12501,
    newEndpoints: 112,
    updatedEndpoints: 356,
    status: "completed",
  },
  {
    id: "c0000001-aaaa-4bbb-8ccc-ddddeeee0004",
    startedAt: ago(74),
    completedAt: ago(73.6),
    totalFetched: 12389,
    newEndpoints: 89,
    updatedEndpoints: 401,
    status: "completed",
  },
  {
    id: "c0000001-aaaa-4bbb-8ccc-ddddeeee0005",
    startedAt: ago(98),
    completedAt: null,
    totalFetched: 11203,
    newEndpoints: 0,
    updatedEndpoints: 0,
    status: "failed",
  },
];

// ---------------------------------------------------------------------------
// Network-wide statistics
// ---------------------------------------------------------------------------

function generateEndpointsOverTime(): { date: string; count: number }[] {
  const points: { date: string; count: number }[] = [];
  let count = 11800;
  for (let i = 29; i >= 0; i--) {
    count += Math.round(Math.random() * 40 + 5);
    points.push({ date: daysAgo(i), count });
  }
  // Ensure the last value matches the "real" total
  points[points.length - 1].count = 12571;
  return points;
}

export const networkStats: NetworkStats = {
  totalEndpoints: 12571,
  totalDomains: 847,
  totalFacilitators: 13,
  lastCrawl: ago(2),

  endpointsByNetwork: [
    { network: "base", count: 9842 },
    { network: "ethereum", count: 1653 },
    { network: "arbitrum", count: 724 },
    { network: "optimism", count: 352 },
  ],

  endpointsByAsset: [
    { asset: "USDC", count: 10234 },
    { asset: "ETH", count: 1287 },
    { asset: "DAI", count: 643 },
    { asset: "WETH", count: 407 },
  ],

  endpointsByPriceBracket: [
    { bracket: "$0-0.001", count: 4821 },
    { bracket: "$0.001-0.01", count: 5134 },
    { bracket: "$0.01-0.1", count: 2103 },
    { bracket: "$0.1+", count: 513 },
  ],

  endpointsOverTime: generateEndpointsOverTime(),

  crawlHistory,
};
