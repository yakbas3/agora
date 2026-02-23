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
