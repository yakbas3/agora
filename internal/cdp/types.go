package cdp

import "time"

// Transfer represents a single USDC transfer from the CDP SQL API.
type Transfer struct {
	ContractAddress string    `json:"address"`
	Sender          string    `json:"sender"`
	TransactionFrom string    `json:"transaction_from"`
	ToAddress       string    `json:"to_address"`
	TransactionHash string    `json:"transaction_hash"`
	BlockTimestamp  time.Time `json:"block_timestamp"`
	Amount          string    `json:"amount"`
	LogIndex        int       `json:"log_index"`
	BlockNumber     int64     `json:"block_number"`
}

// QueryResponse is the CDP SQL API response envelope.
type QueryResponse struct {
	Metadata QueryMetadata            `json:"metadata"`
	Result   []map[string]interface{} `json:"result"`
}

type QueryMetadata struct {
	RowCount       int    `json:"rowCount"`
	ExecutionTimeMs int   `json:"executionTimeMs"`
	Cached         bool   `json:"cached"`
}
