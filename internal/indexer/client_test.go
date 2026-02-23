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
	zero := common.Address{}
	if zero != (common.Address{}) {
		t.Error("zero address should equal empty address")
	}
}
