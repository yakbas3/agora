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
	for _, addr := range addrs {
		if addr == (common.Address{}) {
			t.Error("got zero address in facilitator list")
		}
	}
}

func TestIsFacilitator(t *testing.T) {
	proxyAddr := common.HexToAddress("0x4020615294c913F045dc10f0a5cdEbd86c280001")
	if !IsFacilitator(proxyAddr) {
		t.Errorf("expected proxy %s to be recognized as facilitator", proxyAddr.Hex())
	}

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
