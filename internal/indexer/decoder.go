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

// Event topic hashes for x402 settlement and ERC-20 transfer events.
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
