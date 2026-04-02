package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ProbeResult stores the outcome of a single health check probe.
type ProbeResult struct {
	ID                 uuid.UUID       `db:"id"                  json:"id"`
	EndpointID         uuid.UUID       `db:"endpoint_id"         json:"endpoint_id"`
	ProbedAt           time.Time       `db:"probed_at"           json:"probed_at"`
	StatusCode         *int            `db:"status_code"         json:"status_code"`
	LatencyMs          *int            `db:"latency_ms"          json:"latency_ms"`
	HealthStatus       string          `db:"health_status"       json:"health_status"`
	IsValid402         bool            `db:"is_valid_402"        json:"is_valid_402"`
	ErrorMessage       string          `db:"error_message"       json:"error_message,omitempty"`
	ResponsePayTo      string          `db:"response_pay_to"     json:"response_pay_to,omitempty"`
	ResponseAmountRaw  string          `db:"response_amount_raw" json:"response_amount_raw,omitempty"`
	ResponseNetwork    string          `db:"response_network"    json:"response_network,omitempty"`
	ResponseAsset      string          `db:"response_asset"      json:"response_asset,omitempty"`
	PriceMatch         bool            `db:"price_match"         json:"price_match"`
	DiscrepancyDetails json.RawMessage `db:"discrepancy_details" json:"discrepancy_details,omitempty"`
}
