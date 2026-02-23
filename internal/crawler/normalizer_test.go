package crawler

import (
	"encoding/json"
	"testing"
)

const testItemJSON = `{
	"resource": "https://weather.hugen.tokyo/weather/current",
	"type": "http",
	"x402Version": 2,
	"lastUpdated": "2026-02-23T00:21:38.181Z",
	"metadata": {},
	"accepts": [
		{
			"asset": "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
			"description": "Get current weather conditions for any city worldwide.",
			"extra": {"name": "USD Coin", "version": "2"},
			"maxAmountRequired": "1000",
			"maxTimeoutSeconds": 300,
			"mimeType": "application/json",
			"network": "eip155:8453",
			"outputSchema": {
				"input": {"method": "GET", "queryParams": {"city": "Tokyo"}, "type": "http"},
				"output": {"example": {"temperature_c": 12.5}, "type": "json"}
			},
			"payTo": "0x29322Ea7EcB34aA6164cb2ddeB9CE650902E4f60",
			"resource": "https://weather.hugen.tokyo/weather/current",
			"scheme": "exact"
		}
	]
}`

const testMultiAcceptJSON = `{
	"resource": "https://img402.dev/api/upload",
	"type": "http",
	"x402Version": 2,
	"lastUpdated": "2026-02-20T14:13:40.271Z",
	"metadata": {},
	"accepts": [
		{
			"asset": "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
			"description": "Upload an image with payment and get a public URL. 5MB max, 1-year retention.",
			"extra": {"name": "USD Coin", "version": "2"},
			"maxAmountRequired": "10000",
			"maxTimeoutSeconds": 300,
			"mimeType": "application/json",
			"network": "eip155:8453",
			"outputSchema": {"input": {"method": "POST", "type": "http"}},
			"payTo": "0x77f3c9bCb898Ad1d30e9a336E2cC3108d88D6c09",
			"resource": "https://img402.dev/api/upload",
			"scheme": "exact"
		},
		{
			"asset": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"description": "Upload an image with payment and get a public URL. 5MB max, 1-year retention.",
			"extra": {"feePayer": "BENrLoUbndxoNMUS5JXApGMtNykLjFXXixMtpDwDR9SP"},
			"maxAmountRequired": "10000",
			"maxTimeoutSeconds": 300,
			"mimeType": "application/json",
			"network": "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp",
			"outputSchema": {"input": {"method": "POST", "type": "http"}},
			"payTo": "9xuipz9Y13v6Z1FnLR3XkrknoBLngwUSstjxcqK6deYX",
			"resource": "https://img402.dev/api/upload",
			"scheme": "exact"
		}
	]
}`

func TestNormalizeItem_BasicFields(t *testing.T) {
	var item BazaarItem
	if err := json.Unmarshal([]byte(testItemJSON), &item); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	endpoint, options, err := NormalizeItem(item)
	if err != nil {
		t.Fatalf("NormalizeItem error: %v", err)
	}

	if endpoint.ResourceURL != "https://weather.hugen.tokyo/weather/current" {
		t.Errorf("ResourceURL = %q", endpoint.ResourceURL)
	}
	if endpoint.Domain != "weather.hugen.tokyo" {
		t.Errorf("Domain = %q, want weather.hugen.tokyo", endpoint.Domain)
	}
	if endpoint.X402Version != 2 {
		t.Errorf("X402Version = %d, want 2", endpoint.X402Version)
	}
	if endpoint.HTTPMethod != "GET" {
		t.Errorf("HTTPMethod = %q, want GET", endpoint.HTTPMethod)
	}
	if endpoint.Description != "Get current weather conditions for any city worldwide." {
		t.Errorf("Description = %q", endpoint.Description)
	}
	if len(options) != 1 {
		t.Fatalf("got %d options, want 1", len(options))
	}
	if options[0].NetworkNormalized != "base" {
		t.Errorf("NetworkNormalized = %q, want base", options[0].NetworkNormalized)
	}
	if options[0].AssetName != "USD Coin" {
		t.Errorf("AssetName = %q, want USD Coin", options[0].AssetName)
	}
	if options[0].PriceUSD != 0.001 {
		t.Errorf("PriceUSD = %f, want 0.001", options[0].PriceUSD)
	}
}

func TestNormalizeItem_MultiAccept(t *testing.T) {
	var item BazaarItem
	if err := json.Unmarshal([]byte(testMultiAcceptJSON), &item); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	endpoint, options, err := NormalizeItem(item)
	if err != nil {
		t.Fatalf("NormalizeItem error: %v", err)
	}

	if endpoint.Domain != "img402.dev" {
		t.Errorf("Domain = %q, want img402.dev", endpoint.Domain)
	}
	if endpoint.HTTPMethod != "POST" {
		t.Errorf("HTTPMethod = %q, want POST", endpoint.HTTPMethod)
	}
	if len(options) != 2 {
		t.Fatalf("got %d options, want 2", len(options))
	}
	if options[0].NetworkNormalized != "base" {
		t.Errorf("options[0].NetworkNormalized = %q, want base", options[0].NetworkNormalized)
	}
	if options[1].NetworkNormalized != "solana" {
		t.Errorf("options[1].NetworkNormalized = %q, want solana", options[1].NetworkNormalized)
	}
	// Solana entry should use feePayer as asset name fallback
	if options[1].AssetName != "" {
		t.Errorf("options[1].AssetName = %q, want empty (no name in extra)", options[1].AssetName)
	}
}

func TestNormalizeItem_PicksLongestDescription(t *testing.T) {
	var item BazaarItem
	if err := json.Unmarshal([]byte(testMultiAcceptJSON), &item); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	endpoint, _, err := NormalizeItem(item)
	if err != nil {
		t.Fatalf("NormalizeItem error: %v", err)
	}

	expected := "Upload an image with payment and get a public URL. 5MB max, 1-year retention."
	if endpoint.Description != expected {
		t.Errorf("Description = %q, want %q", endpoint.Description, expected)
	}
}
