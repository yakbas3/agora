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

var (
	SettledTopic           = crypto.Keccak256Hash([]byte("Settled()"))
	SettledWithPermitTopic = crypto.Keccak256Hash([]byte("SettledWithPermit()"))
	TransferTopic          = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
)

const usdcDecimals = 6

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

func usdcToFloat(amount *big.Int) float64 {
	f := new(big.Float).SetInt(amount)
	divisor := new(big.Float).SetFloat64(math.Pow(10, usdcDecimals))
	result, _ := new(big.Float).Quo(f, divisor).Float64()
	return result
}
