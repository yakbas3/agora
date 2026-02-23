package models

import (
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID                 uuid.UUID `db:"id"`
	TxHash             string    `db:"tx_hash"`
	BlockNumber        int64     `db:"block_number"`
	BlockTime          time.Time `db:"block_time"`
	EventType          string    `db:"event_type"`
	ProxyContract      string    `db:"proxy_contract"`
	FacilitatorAddress string    `db:"facilitator_address"`
	PayerAddress       string    `db:"payer_address"`
	RecipientAddress   string    `db:"recipient_address"`
	AmountRaw          string    `db:"amount_raw"`
	AmountUSD          float64   `db:"amount_usd"`
	AssetAddress       string    `db:"asset_address"`
	IndexedAt          time.Time `db:"indexed_at"`
}
