package prober

import (
	"encoding/json"

	"github.com/google/uuid"
)

// ProbeTarget holds the minimal data needed to probe one endpoint.
type ProbeTarget struct {
	EndpointID     uuid.UUID
	ResourceURL    string
	HTTPMethod     string
	Domain         string
	PaymentOptions []PaymentRef
}

// PaymentRef is a lightweight snapshot of one stored payment option.
type PaymentRef struct {
	PayTo     string
	AmountRaw string
	Network   string
	Asset     string
	PriceUSD  float64
}

// ProbeOutcome is the result of probing a single endpoint.
type ProbeOutcome struct {
	EndpointID         uuid.UUID
	StatusCode         *int
	LatencyMs          *int
	HealthStatus       string // "alive", "dead", "unknown"
	IsValid402         bool
	ErrorMessage       string
	ResponsePayTo      string
	ResponseAmountRaw  string
	ResponseNetwork    string
	ResponseAsset      string
	PriceMatch         bool
	DiscrepancyDetails json.RawMessage
}

// x402Body is the JSON body an x402 server returns on HTTP 402.
type x402Body struct {
	Accepts []x402Accept `json:"accepts"`
	// Some servers embed accepts at the top level as well
	PayTo             string `json:"payTo"`
	MaxAmountRequired string `json:"maxAmountRequired"`
	Network           string `json:"network"`
	Asset             string `json:"asset"`
}

type x402Accept struct {
	PayTo             string `json:"payTo"`
	MaxAmountRequired string `json:"maxAmountRequired"`
	Network           string `json:"network"`
	Asset             string `json:"asset"`
	Scheme            string `json:"scheme"`
}
