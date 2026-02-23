package models

import (
	"encoding/json"

	"github.com/google/uuid"
)

type PaymentOption struct {
	ID                uuid.UUID       `db:"id"`
	EndpointID        uuid.UUID       `db:"endpoint_id"`
	Scheme            string          `db:"scheme"`
	NetworkRaw        string          `db:"network_raw"`
	NetworkNormalized string          `db:"network_normalized"`
	AssetAddress      string          `db:"asset_address"`
	AssetName         string          `db:"asset_name"`
	MaxAmountRaw      string          `db:"max_amount_raw"`
	PriceUSD          float64         `db:"price_usd"`
	PayTo             string          `db:"pay_to"`
	MaxTimeoutSeconds int             `db:"max_timeout_seconds"`
	MimeType          string          `db:"mime_type"`
	Description       string          `db:"description"`
	OutputSchemaRaw   json.RawMessage `db:"output_schema_raw"`
}
