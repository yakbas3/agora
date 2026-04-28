package models

import (
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID                 uuid.UUID `db:"id" json:"id"`
	TxHash             string    `db:"tx_hash" json:"tx_hash"`
	BlockNumber        int64     `db:"block_number" json:"block_number"`
	BlockTime          time.Time `db:"block_time" json:"block_time"`
	EventType          string    `db:"event_type" json:"event_type"`
	ProxyContract      string    `db:"proxy_contract" json:"proxy_contract"`
	FacilitatorAddress string    `db:"facilitator_address" json:"facilitator_address"`
	PayerAddress       string    `db:"payer_address" json:"payer_address"`
	RecipientAddress   string    `db:"recipient_address" json:"recipient_address"`
	AmountRaw          string    `db:"amount_raw" json:"amount_raw"`
	AmountUSD          float64   `db:"amount_usd" json:"amount_usd"`
	AssetAddress       string    `db:"asset_address" json:"asset_address"`
	IndexedAt          time.Time `db:"indexed_at" json:"indexed_at"`
}
