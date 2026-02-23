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

	settledLog := types.Log{
		Address:     proxyAddr,
		Topics:      []common.Hash{SettledTopic},
		TxHash:      txHash,
		BlockNumber: 25000100,
	}

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
		[]types.Log{},
		map[uint64]time.Time{25000200: time.Now()},
		map[common.Hash]common.Address{txHash: common.HexToAddress("0x3333333333333333333333333333333333333333")},
	)

	if len(results) != 0 {
		t.Errorf("expected 0 matched transactions for orphan Settled, got %d", len(results))
	}
}
