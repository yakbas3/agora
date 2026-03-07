package models

import (
	"encoding/json"

	"github.com/google/uuid"
)

type PaymentOption struct {
	ID                uuid.UUID       `db:"id" json:"id"`
	EndpointID        uuid.UUID       `db:"endpoint_id" json:"endpoint_id"`
	Scheme            string          `db:"scheme" json:"scheme"`
	NetworkRaw        string          `db:"network_raw" json:"network_raw"`
	NetworkNormalized string          `db:"network_normalized" json:"network_normalized"`
	AssetAddress      string          `db:"asset_address" json:"asset_address"`
	AssetName         string          `db:"asset_name" json:"asset_name"`
	MaxAmountRaw      string          `db:"max_amount_raw" json:"max_amount_raw"`
	PriceUSD          float64         `db:"price_usd" json:"price_usd"`
	PayTo             string          `db:"pay_to" json:"pay_to"`
	MaxTimeoutSeconds int             `db:"max_timeout_seconds" json:"max_timeout_seconds"`
	MimeType          string          `db:"mime_type" json:"mime_type"`
	Description       string          `db:"description" json:"description"`
	OutputSchemaRaw   json.RawMessage `db:"output_schema_raw" json:"output_schema_raw,omitempty"`
}
