// internal/crawler/network_test.go
package crawler

import "testing"

func TestNormalizeNetwork(t *testing.T) {
	tests := []struct {
		raw      string
		expected string
	}{
		{"base", "base"},
		{"eip155:8453", "base"},
		{"base-sepolia", "base-sepolia"},
		{"eip155:84532", "base-sepolia"},
		{"solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp", "solana"},
		{"ethereum", "ethereum"},
		{"eip155:1", "ethereum"},
		{"unknown-network", "unknown-network"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := NormalizeNetwork(tt.raw)
			if got != tt.expected {
				t.Errorf("NormalizeNetwork(%q) = %q, want %q", tt.raw, got, tt.expected)
			}
		})
	}
}
