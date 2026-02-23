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
	settledLogs, err := r.eth.FetchSettledEvents(ctx, fromBlock, toBlock)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch settled events: %w", err)
	}

	if len(settledLogs) == 0 {
		return 0, 0, nil
	}

	transferLogs, err := r.eth.FetchUSDCTransfers(ctx, fromBlock, toBlock)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch USDC transfers: %w", err)
	}

	blockNums := make(map[uint64]bool)
	txHashes := make(map[common.Hash]bool)
	for _, sl := range settledLogs {
		blockNums[sl.BlockNumber] = true
		txHashes[sl.TxHash] = true
	}

	blockTimes := make(map[uint64]time.Time)
	for bn := range blockNums {
		ts, err := r.eth.BlockTimestamp(ctx, int64(bn))
		if err != nil {
			return 0, 0, fmt.Errorf("get block timestamp %d: %w", bn, err)
		}
		blockTimes[bn] = time.Unix(int64(ts), 0).UTC()
	}

	txSenders := make(map[common.Hash]common.Address)
	for txHash := range txHashes {
		sender, err := r.eth.TransactionSender(ctx, txHash)
		if err != nil {
			log.Printf("WARN: could not get sender for %s: %v", txHash.Hex(), err)
			continue
		}
		txSenders[txHash] = sender
	}

	transactions := MatchSettledWithTransfers(settledLogs, transferLogs, blockTimes, txSenders)

	inserted, err := r.repo.InsertTransactions(ctx, transactions)
	if err != nil {
		return 0, 0, fmt.Errorf("insert transactions: %w", err)
	}

	return inserted, len(settledLogs), nil
}
