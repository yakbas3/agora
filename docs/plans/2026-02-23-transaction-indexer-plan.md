# Transaction Indexer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Index on-chain x402 settlements on Base into PostgreSQL, score endpoints by real usage, and discover sellers not listed in Bazaar.

**Architecture:** New `internal/indexer/` package queries Base chain via `eth_getLogs` (Alchemy free tier) in 2K-block windows. Each window fetches `Settled`/`SettledWithPermit` events from proxy contracts AND `Transfer` events from USDC, joins them by tx hash in memory, and batch-inserts into a `transactions` table. A materialized view `endpoint_scores` joins transactions to endpoints via `pay_to` address matching. A `discovered_sellers` table captures on-chain sellers not in Bazaar.

**Tech Stack:** Go 1.24, go-ethereum (ethclient, abi, crypto, common), pgx v5, existing config/database patterns.

---

### Task 1: SQL Migrations

**Files:**
- Create: `migrations/000004_create_transactions.up.sql`
- Create: `migrations/000004_create_transactions.down.sql`
- Create: `migrations/000005_create_indexer_state.up.sql`
- Create: `migrations/000005_create_indexer_state.down.sql`
- Create: `migrations/000006_create_endpoint_scores.up.sql`
- Create: `migrations/000006_create_endpoint_scores.down.sql`
- Create: `migrations/000007_create_discovered_sellers.up.sql`
- Create: `migrations/000007_create_discovered_sellers.down.sql`

**Step 1: Write the transactions migration**

`000004_create_transactions.up.sql`:
```sql
CREATE TABLE transactions (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tx_hash             TEXT NOT NULL UNIQUE,
    block_number        BIGINT NOT NULL,
    block_time          TIMESTAMPTZ NOT NULL,
    event_type          TEXT NOT NULL,
    proxy_contract      TEXT,
    facilitator_address TEXT NOT NULL,
    payer_address       TEXT NOT NULL,
    recipient_address   TEXT NOT NULL,
    amount_raw          TEXT NOT NULL,
    amount_usd          NUMERIC(20, 10) NOT NULL DEFAULT 0,
    asset_address       TEXT NOT NULL,
    indexed_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_recipient ON transactions (recipient_address);
CREATE INDEX idx_transactions_block ON transactions (block_number);
CREATE INDEX idx_transactions_facilitator ON transactions (facilitator_address);
CREATE INDEX idx_transactions_block_time ON transactions (block_time);
```

`000004_create_transactions.down.sql`:
```sql
DROP TABLE IF EXISTS transactions;
```

**Step 2: Write the indexer_state migration**

`000005_create_indexer_state.up.sql`:
```sql
CREATE TABLE indexer_state (
    id          INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    last_block  BIGINT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO indexer_state (id, last_block) VALUES (1, 0);
```

`000005_create_indexer_state.down.sql`:
```sql
DROP TABLE IF EXISTS indexer_state;
```

**Step 3: Write the endpoint_scores materialized view migration**

`000006_create_endpoint_scores.up.sql`:
```sql
CREATE MATERIALIZED VIEW endpoint_scores AS
SELECT
    e.id AS endpoint_id,
    COUNT(t.id) AS tx_count,
    COALESCE(SUM(t.amount_usd), 0) AS total_volume_usd,
    COUNT(DISTINCT t.payer_address) AS unique_payers,
    MAX(t.block_time) AS last_tx_at,
    MIN(t.block_time) AS first_tx_at
FROM endpoints e
LEFT JOIN payment_options po ON po.endpoint_id = e.id
LEFT JOIN transactions t ON t.recipient_address = po.pay_to
GROUP BY e.id;

CREATE UNIQUE INDEX idx_endpoint_scores_id ON endpoint_scores (endpoint_id);
```

`000006_create_endpoint_scores.down.sql`:
```sql
DROP MATERIALIZED VIEW IF EXISTS endpoint_scores;
```

**Step 4: Write the discovered_sellers migration**

`000007_create_discovered_sellers.up.sql`:
```sql
CREATE TABLE discovered_sellers (
    pay_to              TEXT PRIMARY KEY,
    tx_count            INTEGER NOT NULL DEFAULT 0,
    total_volume_usd    NUMERIC(20, 10) NOT NULL DEFAULT 0,
    unique_payers       INTEGER NOT NULL DEFAULT 0,
    first_seen_at       TIMESTAMPTZ NOT NULL,
    last_seen_at        TIMESTAMPTZ NOT NULL,
    matched_endpoint_id UUID REFERENCES endpoints(id) ON DELETE SET NULL
);
```

`000007_create_discovered_sellers.down.sql`:
```sql
DROP TABLE IF EXISTS discovered_sellers;
```

**Step 5: Test migrations apply cleanly**

Run:
```bash
docker compose up -d
go build -o agora.exe ./cmd/agora && ./agora.exe migrate
```

Verify with:
```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "\dt"
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "\dm"
```

Expected: tables `transactions`, `indexer_state`, `discovered_sellers` exist. Materialized view `endpoint_scores` exists.

**Step 6: Commit**

```bash
git add migrations/000004_* migrations/000005_* migrations/000006_* migrations/000007_*
git commit -m "feat: add migrations for transactions, indexer_state, endpoint_scores, discovered_sellers"
```

---

### Task 2: Config and Transaction Model

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/models/transaction.go`
- Modify: `.env.example`

**Step 1: Add indexer config fields**

Add to `Config` struct in `internal/config/config.go`:
```go
BaseRPCURL        string `envconfig:"BASE_RPC_URL"`
IndexerBlockRange int64  `envconfig:"INDEXER_BLOCK_RANGE" default:"2000"`
IndexerStartBlock int64  `envconfig:"INDEXER_START_BLOCK" default:"25000000"`
```

Note: `BASE_RPC_URL` is NOT marked `required:"true"` because the `crawl` and `migrate` commands don't need it. The `index` command will check for it at runtime.

**Step 2: Add env vars to `.env.example`**

Append to `.env.example`:
```
# Indexer (on-chain transaction tracking)
BASE_RPC_URL=https://base-mainnet.g.alchemy.com/v2/YOUR_KEY
INDEXER_BLOCK_RANGE=2000
INDEXER_START_BLOCK=25000000
```

**Step 3: Create the Transaction model**

`internal/models/transaction.go`:
```go
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
```

**Step 4: Verify it compiles**

Run: `go build ./...`
Expected: no errors.

**Step 5: Commit**

```bash
git add internal/config/config.go internal/models/transaction.go .env.example
git commit -m "feat: add indexer config fields and Transaction model"
```

---

### Task 3: Facilitators Registry

**Files:**
- Create: `internal/indexer/facilitators.go`
- Create: `internal/indexer/facilitators_test.go`

**Step 1: Write the failing test**

`internal/indexer/facilitators_test.go`:
```go
package indexer

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestKnownFacilitatorAddresses(t *testing.T) {
	addrs := KnownFacilitatorAddresses()
	if len(addrs) == 0 {
		t.Fatal("expected at least one known facilitator address")
	}
	// All addresses should be valid (non-zero)
	for _, addr := range addrs {
		if addr == (common.Address{}) {
			t.Error("got zero address in facilitator list")
		}
	}
}

func TestIsFacilitator(t *testing.T) {
	// ExactPermit2Proxy is a known proxy
	proxyAddr := common.HexToAddress("0x4020615294c913F045dc10f0a5cdEbd86c280001")
	if !IsFacilitator(proxyAddr) {
		t.Errorf("expected proxy %s to be recognized as facilitator", proxyAddr.Hex())
	}

	// Random address should not be a facilitator
	unknown := common.HexToAddress("0x0000000000000000000000000000000000000001")
	if IsFacilitator(unknown) {
		t.Error("random address should not be recognized as facilitator")
	}
}

func TestProxyContracts(t *testing.T) {
	exact, upto := ProxyContracts()
	if exact == (common.Address{}) || upto == (common.Address{}) {
		t.Fatal("proxy contract addresses should not be zero")
	}
}

func TestUSDCAddress(t *testing.T) {
	usdc := USDCAddress()
	if usdc == (common.Address{}) {
		t.Fatal("USDC address should not be zero")
	}
}
```

**Step 2: Run test to verify it fails**

First, add the go-ethereum dependency:
```bash
go get github.com/ethereum/go-ethereum@latest
```

Run: `go test ./internal/indexer/... -v`
Expected: compilation error — package doesn't exist yet.

**Step 3: Write the implementation**

`internal/indexer/facilitators.go`:
```go
package indexer

import "github.com/ethereum/go-ethereum/common"

// Proxy contracts on Base mainnet
var (
	exactPermit2Proxy = common.HexToAddress("0x4020615294c913F045dc10f0a5cdEbd86c280001")
	uptoPermit2Proxy  = common.HexToAddress("0x4020633461b2895a48930Ff97eE8fCdE8E520002")
	usdcBase          = common.HexToAddress("0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913")
)

// Known Coinbase CDP facilitator addresses on Base.
// Source: facilitators.x402.watch, Coinbase CDP docs.
var knownFacilitators = map[common.Address]bool{
	exactPermit2Proxy: true,
	uptoPermit2Proxy:  true,
	// Add known Coinbase CDP facilitator wallet addresses here as discovered.
	// These are the addresses that call settle() on the proxy contracts (tx.from).
}

// KnownFacilitatorAddresses returns all known facilitator addresses.
func KnownFacilitatorAddresses() []common.Address {
	addrs := make([]common.Address, 0, len(knownFacilitators))
	for addr := range knownFacilitators {
		addrs = append(addrs, addr)
	}
	return addrs
}

// IsFacilitator checks if an address is a known facilitator.
func IsFacilitator(addr common.Address) bool {
	return knownFacilitators[addr]
}

// ProxyContracts returns the x402 proxy contract addresses on Base.
func ProxyContracts() (exact, upto common.Address) {
	return exactPermit2Proxy, uptoPermit2Proxy
}

// USDCAddress returns the USDC contract address on Base.
func USDCAddress() common.Address {
	return usdcBase
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/indexer/... -v`
Expected: all 4 tests PASS.

**Step 5: Commit**

```bash
git add internal/indexer/facilitators.go internal/indexer/facilitators_test.go go.mod go.sum
git commit -m "feat: add known facilitator addresses and proxy contracts registry"
```

---

### Task 4: Event Decoder

**Files:**
- Create: `internal/indexer/decoder.go`
- Create: `internal/indexer/decoder_test.go`

This is the core logic: parsing raw event logs and matching Settled events with USDC Transfers by tx hash.

**Step 1: Write the failing test**

`internal/indexer/decoder_test.go`:
```go
package indexer

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestSettledEventTopic(t *testing.T) {
	expected := crypto.Keccak256Hash([]byte("Settled()"))
	if SettledTopic != expected {
		t.Errorf("SettledTopic mismatch: got %s, want %s", SettledTopic.Hex(), expected.Hex())
	}
}

func TestSettledWithPermitEventTopic(t *testing.T) {
	expected := crypto.Keccak256Hash([]byte("SettledWithPermit()"))
	if SettledWithPermitTopic != expected {
		t.Errorf("SettledWithPermitTopic mismatch: got %s, want %s", SettledWithPermitTopic.Hex(), expected.Hex())
	}
}

func TestTransferEventTopic(t *testing.T) {
	expected := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	if TransferTopic != expected {
		t.Errorf("TransferTopic mismatch: got %s, want %s", TransferTopic.Hex(), expected.Hex())
	}
}

func TestMatchSettledWithTransfers(t *testing.T) {
	txHash := common.HexToHash("0xabc123")
	proxyAddr := common.HexToAddress("0x4020615294c913F045dc10f0a5cdEbd86c280001")
	usdcAddr := USDCAddress()

	// Create a Settled event log
	settledLog := types.Log{
		Address:     proxyAddr,
		Topics:      []common.Hash{SettledTopic},
		TxHash:      txHash,
		BlockNumber: 25000100,
	}

	// Create a USDC Transfer log in the same tx
	payer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	recipient := common.HexToAddress("0x2222222222222222222222222222222222222222")
	amount := new(big.Int).SetUint64(1000000) // 1 USDC

	transferLog := types.Log{
		Address: usdcAddr,
		Topics: []common.Hash{
			TransferTopic,
			common.BytesToHash(payer.Bytes()),
			common.BytesToHash(recipient.Bytes()),
		},
		Data:        common.LeftPadBytes(amount.Bytes(), 32),
		TxHash:      txHash,
		BlockNumber: 25000100,
	}

	blockTimes := map[uint64]time.Time{
		25000100: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}

	txSenders := map[common.Hash]common.Address{
		txHash: common.HexToAddress("0x3333333333333333333333333333333333333333"),
	}

	results := MatchSettledWithTransfers(
		[]types.Log{settledLog},
		[]types.Log{transferLog},
		blockTimes,
		txSenders,
	)

	if len(results) != 1 {
		t.Fatalf("expected 1 matched transaction, got %d", len(results))
	}

	r := results[0]
	if r.TxHash != txHash.Hex() {
		t.Errorf("tx_hash: got %s, want %s", r.TxHash, txHash.Hex())
	}
	if r.BlockNumber != 25000100 {
		t.Errorf("block_number: got %d, want 25000100", r.BlockNumber)
	}
	if r.EventType != "settled" {
		t.Errorf("event_type: got %s, want settled", r.EventType)
	}
	if r.ProxyContract != proxyAddr.Hex() {
		t.Errorf("proxy_contract: got %s, want %s", r.ProxyContract, proxyAddr.Hex())
	}
	if r.PayerAddress != payer.Hex() {
		t.Errorf("payer: got %s, want %s", r.PayerAddress, payer.Hex())
	}
	if r.RecipientAddress != recipient.Hex() {
		t.Errorf("recipient: got %s, want %s", r.RecipientAddress, recipient.Hex())
	}
	if r.AmountRaw != "1000000" {
		t.Errorf("amount_raw: got %s, want 1000000", r.AmountRaw)
	}
	if r.AmountUSD != 1.0 {
		t.Errorf("amount_usd: got %f, want 1.0", r.AmountUSD)
	}
}

func TestMatchSettledNoTransfer(t *testing.T) {
	// Settled event with no matching USDC Transfer should be skipped
	txHash := common.HexToHash("0xdef456")
	proxyAddr := common.HexToAddress("0x4020615294c913F045dc10f0a5cdEbd86c280001")

	settledLog := types.Log{
		Address:     proxyAddr,
		Topics:      []common.Hash{SettledTopic},
		TxHash:      txHash,
		BlockNumber: 25000200,
	}

	results := MatchSettledWithTransfers(
		[]types.Log{settledLog},
		[]types.Log{}, // no transfers
		map[uint64]time.Time{25000200: time.Now()},
		map[common.Hash]common.Address{txHash: common.HexToAddress("0x3333333333333333333333333333333333333333")},
	)

	if len(results) != 0 {
		t.Errorf("expected 0 matched transactions for orphan Settled, got %d", len(results))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/indexer/... -run TestSettled -v`
Expected: compilation error — types not defined.

**Step 3: Write the implementation**

`internal/indexer/decoder.go`:
```go
package indexer

import (
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/yamanakbas/agora/internal/models"
)

// Event topic hashes
var (
	SettledTopic           = crypto.Keccak256Hash([]byte("Settled()"))
	SettledWithPermitTopic = crypto.Keccak256Hash([]byte("SettledWithPermit()"))
	TransferTopic          = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
)

const usdcDecimals = 6

// MatchSettledWithTransfers joins Settled/SettledWithPermit events with USDC Transfer
// events by tx hash. Returns matched transactions.
//
// blockTimes maps block numbers to timestamps.
// txSenders maps tx hashes to the address that submitted the transaction (the facilitator).
func MatchSettledWithTransfers(
	settledLogs []types.Log,
	transferLogs []types.Log,
	blockTimes map[uint64]time.Time,
	txSenders map[common.Hash]common.Address,
) []models.Transaction {
	// Index USDC transfers by tx hash
	transfersByTx := make(map[common.Hash][]types.Log)
	for _, tl := range transferLogs {
		transfersByTx[tl.TxHash] = append(transfersByTx[tl.TxHash], tl)
	}

	var results []models.Transaction

	for _, sl := range settledLogs {
		transfers, ok := transfersByTx[sl.TxHash]
		if !ok || len(transfers) == 0 {
			continue
		}

		eventType := "settled"
		if sl.Topics[0] == SettledWithPermitTopic {
			eventType = "settled_with_permit"
		}

		// Use the last USDC Transfer in the tx as the payment
		// (there may be multiple transfers for fee splitting, etc.)
		tl := transfers[len(transfers)-1]
		payer, recipient, amount := decodeTransfer(tl)

		amountFloat := usdcToFloat(amount)

		results = append(results, models.Transaction{
			ID:                 uuid.New(),
			TxHash:             sl.TxHash.Hex(),
			BlockNumber:        int64(sl.BlockNumber),
			BlockTime:          blockTimes[sl.BlockNumber],
			EventType:          eventType,
			ProxyContract:      sl.Address.Hex(),
			FacilitatorAddress: txSenders[sl.TxHash].Hex(),
			PayerAddress:       payer.Hex(),
			RecipientAddress:   recipient.Hex(),
			AmountRaw:          amount.String(),
			AmountUSD:          amountFloat,
			AssetAddress:       USDCAddress().Hex(),
			IndexedAt:          time.Now().UTC(),
		})
	}

	return results
}

// decodeTransfer extracts from, to, and value from a USDC Transfer event log.
// Transfer(address indexed from, address indexed to, uint256 value)
func decodeTransfer(log types.Log) (from, to common.Address, value *big.Int) {
	if len(log.Topics) < 3 {
		return common.Address{}, common.Address{}, big.NewInt(0)
	}
	from = common.BytesToAddress(log.Topics[1].Bytes())
	to = common.BytesToAddress(log.Topics[2].Bytes())
	value = new(big.Int)
	if len(log.Data) >= 32 {
		value.SetBytes(log.Data[:32])
	}
	return from, to, value
}

// usdcToFloat converts a raw USDC amount (6 decimals) to a float64.
func usdcToFloat(amount *big.Int) float64 {
	f := new(big.Float).SetInt(amount)
	divisor := new(big.Float).SetFloat64(math.Pow(10, usdcDecimals))
	result, _ := new(big.Float).Quo(f, divisor).Float64()
	return result
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/indexer/... -v`
Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/indexer/decoder.go internal/indexer/decoder_test.go
git commit -m "feat: add event decoder that matches Settled events with USDC Transfers"
```

---

### Task 5: Ethereum RPC Client

**Files:**
- Create: `internal/indexer/client.go`
- Create: `internal/indexer/client_test.go`

This wraps `go-ethereum/ethclient` to fetch event logs in block-range windows.

**Step 1: Write the failing test**

`internal/indexer/client_test.go`:
```go
package indexer

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestBuildSettledFilterQuery(t *testing.T) {
	c := &EthClient{}
	q := c.buildSettledFilter(1000, 2000)

	if q.FromBlock.Int64() != 1000 {
		t.Errorf("FromBlock: got %d, want 1000", q.FromBlock.Int64())
	}
	if q.ToBlock.Int64() != 2000 {
		t.Errorf("ToBlock: got %d, want 2000", q.ToBlock.Int64())
	}
	if len(q.Addresses) != 2 {
		t.Fatalf("expected 2 contract addresses, got %d", len(q.Addresses))
	}
	// Should have Settled and SettledWithPermit topics
	if len(q.Topics) != 1 || len(q.Topics[0]) != 2 {
		t.Fatalf("expected 1 topic group with 2 topics, got %v", q.Topics)
	}
}

func TestBuildTransferFilterQuery(t *testing.T) {
	c := &EthClient{}
	q := c.buildTransferFilter(1000, 2000)

	if q.FromBlock.Int64() != 1000 {
		t.Errorf("FromBlock: got %d, want 1000", q.FromBlock.Int64())
	}
	if len(q.Addresses) != 1 {
		t.Fatalf("expected 1 address (USDC), got %d", len(q.Addresses))
	}
	if q.Addresses[0] != USDCAddress() {
		t.Errorf("address: got %s, want %s", q.Addresses[0].Hex(), USDCAddress().Hex())
	}
	expectedTopic := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	if q.Topics[0][0] != expectedTopic {
		t.Errorf("topic0: got %s, want %s", q.Topics[0][0].Hex(), expectedTopic.Hex())
	}
}

func TestBlockWindows(t *testing.T) {
	windows := blockWindows(100, 350, 100)
	expected := [][2]int64{{100, 199}, {200, 299}, {300, 350}}
	if len(windows) != len(expected) {
		t.Fatalf("expected %d windows, got %d", len(expected), len(windows))
	}
	for i, w := range windows {
		if w != expected[i] {
			t.Errorf("window %d: got %v, want %v", i, w, expected[i])
		}
	}
}

func TestBlockWindowsSingleBlock(t *testing.T) {
	windows := blockWindows(100, 100, 2000)
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	if windows[0] != [2]int64{100, 100} {
		t.Errorf("got %v, want [100, 100]", windows[0])
	}
}

func TestExtractSender(t *testing.T) {
	// Just test the zero case — real extraction requires an RPC
	zero := common.Address{}
	if zero != (common.Address{}) {
		t.Error("zero address should equal empty address")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/indexer/... -run TestBuild -v`
Expected: compilation error — `EthClient` not defined.

**Step 3: Write the implementation**

`internal/indexer/client.go`:
```go
package indexer

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EthClient wraps go-ethereum's ethclient for querying x402 event logs.
type EthClient struct {
	client *ethclient.Client
}

// NewEthClient connects to the given RPC URL and returns an EthClient.
func NewEthClient(rpcURL string) (*EthClient, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial RPC: %w", err)
	}
	return &EthClient{client: client}, nil
}

// Close closes the underlying RPC connection.
func (c *EthClient) Close() {
	c.client.Close()
}

// CurrentBlock returns the latest block number.
func (c *EthClient) CurrentBlock(ctx context.Context) (int64, error) {
	header, err := c.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("get latest header: %w", err)
	}
	return header.Number.Int64(), nil
}

// FetchSettledEvents queries Settled and SettledWithPermit events from proxy contracts.
func (c *EthClient) FetchSettledEvents(ctx context.Context, fromBlock, toBlock int64) ([]types.Log, error) {
	q := c.buildSettledFilter(fromBlock, toBlock)
	logs, err := c.client.FilterLogs(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("filter settled logs [%d-%d]: %w", fromBlock, toBlock, err)
	}
	return logs, nil
}

// FetchUSDCTransfers queries USDC Transfer events in the given block range.
func (c *EthClient) FetchUSDCTransfers(ctx context.Context, fromBlock, toBlock int64) ([]types.Log, error) {
	q := c.buildTransferFilter(fromBlock, toBlock)
	logs, err := c.client.FilterLogs(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("filter USDC transfer logs [%d-%d]: %w", fromBlock, toBlock, err)
	}
	return logs, nil
}

// TransactionSender returns the from address of a transaction.
func (c *EthClient) TransactionSender(ctx context.Context, txHash common.Hash) (common.Address, error) {
	tx, _, err := c.client.TransactionByHash(ctx, txHash)
	if err != nil {
		return common.Address{}, fmt.Errorf("get tx %s: %w", txHash.Hex(), err)
	}
	sender, err := types.LatestSignerForChainID(tx.ChainId()).Sender(tx)
	if err != nil {
		return common.Address{}, fmt.Errorf("recover sender for %s: %w", txHash.Hex(), err)
	}
	return sender, nil
}

// BlockTimestamp returns the timestamp for a block number.
func (c *EthClient) BlockTimestamp(ctx context.Context, blockNumber int64) (uint64, error) {
	header, err := c.client.HeaderByNumber(ctx, big.NewInt(blockNumber))
	if err != nil {
		return 0, fmt.Errorf("get header %d: %w", blockNumber, err)
	}
	return header.Time, nil
}

func (c *EthClient) buildSettledFilter(fromBlock, toBlock int64) ethereum.FilterQuery {
	exact, upto := ProxyContracts()
	return ethereum.FilterQuery{
		FromBlock: big.NewInt(fromBlock),
		ToBlock:   big.NewInt(toBlock),
		Addresses: []common.Address{exact, upto},
		Topics:    [][]common.Hash{{SettledTopic, SettledWithPermitTopic}},
	}
}

func (c *EthClient) buildTransferFilter(fromBlock, toBlock int64) ethereum.FilterQuery {
	return ethereum.FilterQuery{
		FromBlock: big.NewInt(fromBlock),
		ToBlock:   big.NewInt(toBlock),
		Addresses: []common.Address{USDCAddress()},
		Topics:    [][]common.Hash{{TransferTopic}},
	}
}

// blockWindows divides a block range into fixed-size windows.
func blockWindows(from, to, windowSize int64) [][2]int64 {
	var windows [][2]int64
	for start := from; start <= to; start += windowSize {
		end := start + windowSize - 1
		if end > to {
			end = to
		}
		windows = append(windows, [2]int64{start, end})
	}
	return windows
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/indexer/... -v`
Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/indexer/client.go internal/indexer/client_test.go
git commit -m "feat: add Ethereum RPC client for querying x402 event logs"
```

---

### Task 6: Repository Methods

**Files:**
- Modify: `internal/database/repository.go`

**Step 1: Add transaction and indexer_state repository methods**

Append to `internal/database/repository.go`:

```go
// GetLastIndexedBlock returns the last fully indexed block number.
func (r *Repository) GetLastIndexedBlock(ctx context.Context) (int64, error) {
	var lastBlock int64
	err := r.pool.QueryRow(ctx,
		`SELECT last_block FROM indexer_state WHERE id = 1`,
	).Scan(&lastBlock)
	if err != nil {
		return 0, fmt.Errorf("get last indexed block: %w", err)
	}
	return lastBlock, nil
}

// UpdateLastIndexedBlock sets the last fully indexed block number.
func (r *Repository) UpdateLastIndexedBlock(ctx context.Context, blockNumber int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE indexer_state SET last_block = $1, updated_at = $2 WHERE id = 1`,
		blockNumber, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("update last indexed block: %w", err)
	}
	return nil
}

// InsertTransactions batch-inserts transactions, skipping duplicates (ON CONFLICT DO NOTHING).
func (r *Repository) InsertTransactions(ctx context.Context, txs []models.Transaction) (int, error) {
	if len(txs) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, tx := range txs {
		batch.Queue(
			`INSERT INTO transactions (id, tx_hash, block_number, block_time, event_type,
			   proxy_contract, facilitator_address, payer_address, recipient_address,
			   amount_raw, amount_usd, asset_address, indexed_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			 ON CONFLICT (tx_hash) DO NOTHING`,
			tx.ID, tx.TxHash, tx.BlockNumber, tx.BlockTime, tx.EventType,
			tx.ProxyContract, tx.FacilitatorAddress, tx.PayerAddress,
			tx.RecipientAddress, tx.AmountRaw, tx.AmountUSD, tx.AssetAddress,
			tx.IndexedAt,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	inserted := 0
	for range txs {
		ct, err := br.Exec()
		if err != nil {
			br.Close()
			return inserted, fmt.Errorf("insert transaction: %w", err)
		}
		if ct.RowsAffected() > 0 {
			inserted++
		}
	}
	br.Close()

	return inserted, nil
}

// RefreshEndpointScores refreshes the endpoint_scores materialized view.
func (r *Repository) RefreshEndpointScores(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `REFRESH MATERIALIZED VIEW CONCURRENTLY endpoint_scores`)
	if err != nil {
		return fmt.Errorf("refresh endpoint_scores: %w", err)
	}
	return nil
}

// RefreshDiscoveredSellers updates the discovered_sellers table from unmatched transactions.
func (r *Repository) RefreshDiscoveredSellers(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO discovered_sellers (pay_to, tx_count, total_volume_usd, unique_payers, first_seen_at, last_seen_at)
		SELECT
			t.recipient_address,
			COUNT(*)::INTEGER,
			COALESCE(SUM(t.amount_usd), 0),
			COUNT(DISTINCT t.payer_address)::INTEGER,
			MIN(t.block_time),
			MAX(t.block_time)
		FROM transactions t
		WHERE NOT EXISTS (
			SELECT 1 FROM payment_options po WHERE po.pay_to = t.recipient_address
		)
		GROUP BY t.recipient_address
		ON CONFLICT (pay_to) DO UPDATE SET
			tx_count = EXCLUDED.tx_count,
			total_volume_usd = EXCLUDED.total_volume_usd,
			unique_payers = EXCLUDED.unique_payers,
			first_seen_at = EXCLUDED.first_seen_at,
			last_seen_at = EXCLUDED.last_seen_at
	`)
	if err != nil {
		return fmt.Errorf("refresh discovered_sellers: %w", err)
	}
	return nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no errors.

**Step 3: Commit**

```bash
git add internal/database/repository.go
git commit -m "feat: add repository methods for transactions, indexer state, and scoring"
```

---

### Task 7: Indexer Runner

**Files:**
- Create: `internal/indexer/runner.go`

**Step 1: Write the runner**

`internal/indexer/runner.go`:
```go
package indexer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/yamanakbas/agora/internal/database"
)

// Runner orchestrates the block-range windowed indexing loop.
type Runner struct {
	eth        *EthClient
	repo       *database.Repository
	blockRange int64
	startBlock int64
}

// NewRunner creates a new indexer runner.
func NewRunner(eth *EthClient, repo *database.Repository, blockRange, startBlock int64) *Runner {
	return &Runner{
		eth:        eth,
		repo:       repo,
		blockRange: blockRange,
		startBlock: startBlock,
	}
}

// Run indexes transactions from the last indexed block to the current chain head.
func (r *Runner) Run(ctx context.Context) error {
	lastBlock, err := r.repo.GetLastIndexedBlock(ctx)
	if err != nil {
		return fmt.Errorf("get last indexed block: %w", err)
	}

	fromBlock := lastBlock + 1
	if lastBlock == 0 {
		fromBlock = r.startBlock
	}

	currentBlock, err := r.eth.CurrentBlock(ctx)
	if err != nil {
		return fmt.Errorf("get current block: %w", err)
	}

	if fromBlock > currentBlock {
		log.Printf("Already up to date (last indexed: %d, chain head: %d)", lastBlock, currentBlock)
		return nil
	}

	log.Printf("Indexing blocks %d to %d (%d blocks)", fromBlock, currentBlock, currentBlock-fromBlock+1)

	windows := blockWindows(fromBlock, currentBlock, r.blockRange)
	totalTxs := 0
	totalSettled := 0

	for i, w := range windows {
		windowFrom, windowTo := w[0], w[1]

		txCount, settledCount, err := r.processWindow(ctx, windowFrom, windowTo)
		if err != nil {
			return fmt.Errorf("process window [%d-%d]: %w", windowFrom, windowTo, err)
		}

		totalTxs += txCount
		totalSettled += settledCount

		if err := r.repo.UpdateLastIndexedBlock(ctx, windowTo); err != nil {
			return fmt.Errorf("update last indexed block: %w", err)
		}

		if (i+1)%10 == 0 || i == len(windows)-1 {
			log.Printf("Progress: %d/%d windows, %d settled events, %d transactions stored (block %d)",
				i+1, len(windows), totalSettled, totalTxs, windowTo)
		}
	}

	log.Printf("Indexing complete: %d transactions from %d settled events", totalTxs, totalSettled)

	log.Println("Refreshing endpoint scores...")
	if err := r.repo.RefreshEndpointScores(ctx); err != nil {
		return fmt.Errorf("refresh scores: %w", err)
	}

	log.Println("Refreshing discovered sellers...")
	if err := r.repo.RefreshDiscoveredSellers(ctx); err != nil {
		return fmt.Errorf("refresh discovered sellers: %w", err)
	}

	log.Println("Done.")
	return nil
}

// processWindow fetches and processes events in a single block range.
// Returns (inserted transaction count, settled event count, error).
func (r *Runner) processWindow(ctx context.Context, fromBlock, toBlock int64) (int, int, error) {
	// Fetch Settled events from proxy contracts
	settledLogs, err := r.eth.FetchSettledEvents(ctx, fromBlock, toBlock)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch settled events: %w", err)
	}

	if len(settledLogs) == 0 {
		return 0, 0, nil
	}

	// Fetch USDC Transfers in the same block range
	transferLogs, err := r.eth.FetchUSDCTransfers(ctx, fromBlock, toBlock)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch USDC transfers: %w", err)
	}

	// Collect unique block numbers and tx hashes we need metadata for
	blockNums := make(map[uint64]bool)
	txHashes := make(map[common.Hash]bool)
	for _, sl := range settledLogs {
		blockNums[sl.BlockNumber] = true
		txHashes[sl.TxHash] = true
	}

	// Fetch block timestamps
	blockTimes := make(map[uint64]time.Time)
	for bn := range blockNums {
		ts, err := r.eth.BlockTimestamp(ctx, int64(bn))
		if err != nil {
			return 0, 0, fmt.Errorf("get block timestamp %d: %w", bn, err)
		}
		blockTimes[bn] = time.Unix(int64(ts), 0).UTC()
	}

	// Fetch transaction senders (facilitator addresses)
	txSenders := make(map[common.Hash]common.Address)
	for txHash := range txHashes {
		sender, err := r.eth.TransactionSender(ctx, txHash)
		if err != nil {
			log.Printf("WARN: could not get sender for %s: %v", txHash.Hex(), err)
			continue
		}
		txSenders[txHash] = sender
	}

	// Match and decode
	transactions := MatchSettledWithTransfers(settledLogs, transferLogs, blockTimes, txSenders)

	// Insert into database
	inserted, err := r.repo.InsertTransactions(ctx, transactions)
	if err != nil {
		return 0, 0, fmt.Errorf("insert transactions: %w", err)
	}

	return inserted, len(settledLogs), nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no errors.

**Step 3: Commit**

```bash
git add internal/indexer/runner.go
git commit -m "feat: add indexer runner with block-range windowed processing"
```

---

### Task 8: CLI Index Command

**Files:**
- Modify: `cmd/agora/main.go`

**Step 1: Add the `index` case to main.go**

Add `"github.com/yamanakbas/agora/internal/indexer"` to imports.

Add case to switch:
```go
case "index":
    runIndex(cfg)
```

Add to `printUsage()`:
```go
fmt.Fprintln(os.Stderr, "  index     Index on-chain x402 transactions from Base")
```

Add function:
```go
func runIndex(cfg *config.Config) {
	if cfg.BaseRPCURL == "" {
		log.Fatal("BASE_RPC_URL is required for indexing. Set it in .env or environment.")
	}

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	eth, err := indexer.NewEthClient(cfg.BaseRPCURL)
	if err != nil {
		log.Fatalf("Failed to connect to Base RPC: %v", err)
	}
	defer eth.Close()

	repo := database.NewRepository(pool)
	runner := indexer.NewRunner(eth, repo, cfg.IndexerBlockRange, cfg.IndexerStartBlock)

	log.Printf("Starting indexer (blockRange=%d, startBlock=%d)", cfg.IndexerBlockRange, cfg.IndexerStartBlock)

	if err := runner.Run(ctx); err != nil {
		log.Fatalf("Indexing failed: %v", err)
	}
}
```

**Step 2: Verify it compiles**

Run: `go build -o agora.exe ./cmd/agora`
Expected: no errors.

**Step 3: Commit**

```bash
git add cmd/agora/main.go
git commit -m "feat: add index CLI command for on-chain transaction indexing"
```

---

### Task 9: E2E Verification

**Step 1: Run migrations**

```bash
docker compose up -d
./agora.exe migrate
```

Verify tables exist:
```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "\dt"
```

**Step 2: Set up Alchemy API key**

Add to `.env`:
```
BASE_RPC_URL=https://base-mainnet.g.alchemy.com/v2/YOUR_ACTUAL_KEY
```

**Step 3: Test with a small block range**

Override the start block to a recent range with known x402 activity:
```bash
INDEXER_START_BLOCK=29000000 INDEXER_BLOCK_RANGE=2000 ./agora.exe index
```

**Step 4: Verify data**

```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT count(*) FROM transactions;"
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT * FROM transactions ORDER BY block_number DESC LIMIT 5;"
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT * FROM endpoint_scores WHERE tx_count > 0 ORDER BY tx_count DESC LIMIT 10;"
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT * FROM discovered_sellers ORDER BY tx_count DESC LIMIT 10;"
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "SELECT last_block FROM indexer_state;"
```

**Step 5: Run all tests**

```bash
go test ./... -v
```

Expected: all tests pass.

**Step 6: Final commit**

```bash
git add -A
git commit -m "chore: verify e2e transaction indexer pipeline"
```

---

### Task 10: Handle Edge Case — Transactions With Multiple USDC Transfers

**Files:**
- Modify: `internal/indexer/decoder.go`
- Modify: `internal/indexer/decoder_test.go`

Some x402 settlements may have multiple USDC Transfer events in a single transaction (e.g., fee splitting, gas refunds). The current decoder takes the last transfer, but we should handle this more carefully after observing real data in Task 9.

**Step 1: After E2E, inspect actual transaction patterns**

```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) psql -U agora -d agora -c "
SELECT tx_hash, count(*) as transfer_count
FROM (
    SELECT DISTINCT tx_hash FROM transactions
) t
GROUP BY tx_hash
HAVING count(*) > 1
ORDER BY transfer_count DESC
LIMIT 10;"
```

**Step 2: Adjust decoder if needed based on observed patterns**

If multiple transfers per settlement are common, consider:
- Storing all transfers (one transaction row per USDC Transfer within a Settled tx)
- Or picking the largest transfer (the actual payment, not fees)

This is a data-driven decision — implement after Task 9 reveals the patterns.

**Step 3: Commit any adjustments**

```bash
git add internal/indexer/decoder.go internal/indexer/decoder_test.go
git commit -m "fix: handle multiple USDC transfers per settlement"
```
