package crawler

import "encoding/json"

// BazaarResponse is the top-level response from the Bazaar discovery API.
type BazaarResponse struct {
	Items       []BazaarItem     `json:"items"`
	Pagination  BazaarPagination `json:"pagination"`
	X402Version int              `json:"x402Version"`
}

type BazaarPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// BazaarItem is a single endpoint entry from the Bazaar API.
type BazaarItem struct {
	Resource    string          `json:"resource"`
	Type        string          `json:"type"`
	X402Version int             `json:"x402Version"`
	LastUpdated string          `json:"lastUpdated"`
	Metadata    json.RawMessage `json:"metadata"`
	Accepts     []BazaarAccept  `json:"accepts"`
}

// BazaarAccept is one payment option within an endpoint.
type BazaarAccept struct {
	Asset             string          `json:"asset"`
	Description       string          `json:"description"`
	Extra             BazaarExtra     `json:"extra"`
	MaxAmountRequired string          `json:"maxAmountRequired"`
	MaxTimeoutSeconds int             `json:"maxTimeoutSeconds"`
	MimeType          string          `json:"mimeType"`
	Network           string          `json:"network"`
	OutputSchema      json.RawMessage `json:"outputSchema"`
	PayTo             string          `json:"payTo"`
	Resource          string          `json:"resource"`
	Scheme            string          `json:"scheme"`
}

type BazaarExtra struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	FeePayer string `json:"feePayer"`
}
