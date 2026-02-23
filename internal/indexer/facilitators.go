package indexer

import "github.com/ethereum/go-ethereum/common"

var (
	exactPermit2Proxy = common.HexToAddress("0x4020615294c913F045dc10f0a5cdEbd86c280001")
	uptoPermit2Proxy  = common.HexToAddress("0x4020633461b2895a48930Ff97eE8fCdE8E520002")
	usdcBase          = common.HexToAddress("0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913")
)

var knownFacilitators = map[common.Address]bool{
	exactPermit2Proxy: true,
	uptoPermit2Proxy:  true,
}

func KnownFacilitatorAddresses() []common.Address {
	addrs := make([]common.Address, 0, len(knownFacilitators))
	for addr := range knownFacilitators {
		addrs = append(addrs, addr)
	}
	return addrs
}

func IsFacilitator(addr common.Address) bool {
	return knownFacilitators[addr]
}

func ProxyContracts() (exact, upto common.Address) {
	return exactPermit2Proxy, uptoPermit2Proxy
}

func USDCAddress() common.Address {
	return usdcBase
}
