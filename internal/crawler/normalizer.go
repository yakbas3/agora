package crawler

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/yamanakbas/agora/internal/models"
)

// assetDecimals maps known asset contract addresses to their decimal places.
var assetDecimals = map[string]int{
	"0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913": 6, // USDC on Base
	"0x036CbD53842c5426634e7929541eC2318f3dCF7e": 6, // USDC on Base Sepolia
	"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": 6, // USDC on Solana
}

// NormalizeItem converts a raw Bazaar API item into an Endpoint and its PaymentOptions.
func NormalizeItem(item BazaarItem) (models.Endpoint, []models.PaymentOption, error) {
	now := time.Now().UTC()

	parsedURL, err := url.Parse(item.Resource)
	if err != nil {
		return models.Endpoint{}, nil, fmt.Errorf("parse resource URL %q: %w", item.Resource, err)
	}

	lastUpdated, err := time.Parse(time.RFC3339Nano, item.LastUpdated)
	if err != nil {
		return models.Endpoint{}, nil, fmt.Errorf("parse lastUpdated %q: %w", item.LastUpdated, err)
	}

	// Extract HTTP method and schemas from the best (first rich) outputSchema
	httpMethod, inputSchema, outputSchema := extractSchemas(item.Accepts)

	// Pick the longest description across all accepts entries
	description := pickBestDescription(item.Accepts)

	endpoint := models.Endpoint{
		ID:           uuid.New(),
		ResourceURL:  item.Resource,
		Domain:       parsedURL.Hostname(),
		Type:         item.Type,
		X402Version:  item.X402Version,
		Description:  description,
		HTTPMethod:   httpMethod,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		RawMetadata:  item.Metadata,
		LastUpdated:  lastUpdated,
		FirstSeen:    now,
		LastCrawled:  now,
	}

	options := make([]models.PaymentOption, 0, len(item.Accepts))
	for _, accept := range item.Accepts {
		priceUSD := computePriceUSD(accept.MaxAmountRequired, accept.Asset)

		option := models.PaymentOption{
			ID:                uuid.New(),
			Scheme:            accept.Scheme,
			NetworkRaw:        accept.Network,
			NetworkNormalized: NormalizeNetwork(accept.Network),
			AssetAddress:      accept.Asset,
			AssetName:         accept.Extra.Name,
			MaxAmountRaw:      accept.MaxAmountRequired,
			PriceUSD:          priceUSD,
			PayTo:             accept.PayTo,
			MaxTimeoutSeconds: accept.MaxTimeoutSeconds,
			MimeType:          accept.MimeType,
			Description:       accept.Description,
			OutputSchemaRaw:   accept.OutputSchema,
		}
		options = append(options, option)
	}

	return endpoint, options, nil
}

// pickBestDescription returns the longest description from the accepts list.
func pickBestDescription(accepts []BazaarAccept) string {
	best := ""
	for _, a := range accepts {
		if len(a.Description) > len(best) {
			best = a.Description
		}
	}
	return best
}

// extractSchemas extracts HTTP method, input schema, and output schema
// from the first accepts entry that has a non-minimal outputSchema.
func extractSchemas(accepts []BazaarAccept) (string, json.RawMessage, json.RawMessage) {
	method := ""

	for _, a := range accepts {
		var schema map[string]json.RawMessage
		if err := json.Unmarshal(a.OutputSchema, &schema); err != nil {
			continue
		}

		// Extract method from input
		if inputRaw, ok := schema["input"]; ok {
			var input struct {
				Method string `json:"method"`
			}
			if err := json.Unmarshal(inputRaw, &input); err == nil && input.Method != "" {
				method = input.Method
			}
		}

		// Check if this has an output section (rich schema)
		outputRaw, hasOutput := schema["output"]
		if hasOutput {
			inputRaw := schema["input"]
			return method, inputRaw, outputRaw
		}
	}

	// Fallback: use the first entry's input schema even if no output
	if len(accepts) > 0 {
		var schema map[string]json.RawMessage
		if err := json.Unmarshal(accepts[0].OutputSchema, &schema); err == nil {
			if inputRaw, ok := schema["input"]; ok {
				return method, inputRaw, nil
			}
		}
	}

	return method, nil, nil
}

// computePriceUSD converts the raw amount string to a USD value
// based on the asset's known decimal places. Defaults to 6 decimals (USDC).
func computePriceUSD(rawAmount string, assetAddress string) float64 {
	amount, err := strconv.ParseFloat(rawAmount, 64)
	if err != nil {
		return 0
	}
	decimals := 6 // default to USDC
	if d, ok := assetDecimals[assetAddress]; ok {
		decimals = d
	}
	divisor := 1.0
	for i := 0; i < decimals; i++ {
		divisor *= 10
	}
	return amount / divisor
}
