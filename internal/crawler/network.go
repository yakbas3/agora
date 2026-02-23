// internal/crawler/network.go
package crawler

import "strings"

// networkMap maps chain IDs and aliases to canonical network names.
var networkMap = map[string]string{
	"eip155:8453":      "base",
	"eip155:84532":     "base-sepolia",
	"eip155:1":         "ethereum",
	"eip155:11155111":  "ethereum-sepolia",
}

// NormalizeNetwork converts raw network identifiers to canonical names.
// Solana network strings (starting with "solana:") are normalized to "solana".
// Known EIP-155 chain IDs are mapped to human-readable names.
// Unknown values are returned as-is.
func NormalizeNetwork(raw string) string {
	if raw == "" {
		return ""
	}
	if mapped, ok := networkMap[raw]; ok {
		return mapped
	}
	if strings.HasPrefix(raw, "solana:") {
		return "solana"
	}
	return raw
}
