package cdp

import "time"

// Transfer represents a single USDC transfer from the CDP SQL API.
type Transfer struct {
	ContractAddress string    `json:"contract_address"`
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
	Result QueryResult `json:"result"`
}

type QueryResult struct {
	Columns []Column        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Status  string          `json:"status"`
}

type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
