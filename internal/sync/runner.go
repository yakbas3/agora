package sync

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/yamanakbas/agora/internal/cdp"
	"github.com/yamanakbas/agora/internal/database"
	"github.com/yamanakbas/agora/internal/models"
)

const (
	// Default start date for facilitators that have never been synced.
	defaultStartDate = "2024-06-01"
	// USDC has 6 decimals.
	usdcDecimals = 6
	// USDC contract address on Base.
	usdcBaseAddress = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
)

type Runner struct {
	cdpClient *cdp.Client
	repo      *database.Repository
}

func NewRunner(cdpClient *cdp.Client, repo *database.Repository) *Runner {
	return &Runner{cdpClient: cdpClient, repo: repo}
}

func (r *Runner) Run(ctx context.Context) error {
	facilitators, err := r.repo.GetBaseFacilitators(ctx)
	if err != nil {
		return fmt.Errorf("load facilitators: %w", err)
	}
	log.Printf("Syncing %d Base facilitators", len(facilitators))

	now := time.Now().UTC()
	totalInserted := 0

	for _, f := range facilitators {
		inserted, err := r.syncFacilitator(ctx, f, now)
		if err != nil {
			log.Printf("ERROR syncing %s (%s): %v", f.Name, f.Address, err)
			continue
		}
		totalInserted += inserted
		if inserted > 0 {
			log.Printf("  %s (%s): %d new transactions", f.Name, f.Address[:10]+"...", inserted)
		}
	}

	log.Printf("Sync complete: %d new transactions total", totalInserted)

	// Refresh materialized views
	if totalInserted > 0 {
		log.Println("Refreshing endpoint_scores...")
		if err := r.repo.RefreshEndpointScores(ctx); err != nil {
			log.Printf("WARNING: refresh endpoint_scores: %v", err)
		}
		log.Println("Refreshing discovered_sellers...")
		if err := r.repo.RefreshDiscoveredSellers(ctx); err != nil {
			log.Printf("WARNING: refresh discovered_sellers: %v", err)
		}
	}

	return nil
}

func (r *Runner) syncFacilitator(ctx context.Context, f models.Facilitator, now time.Time) (int, error) {
	since := defaultSinceTime(f.LastSyncedAt)

	transfers, err := r.cdpClient.QueryTransfers(f.Address, since, now)
	if err != nil {
		return 0, fmt.Errorf("query transfers: %w", err)
	}

	if len(transfers) == 0 {
		// Update sync time even if no transfers, so we don't re-query the same window
		if err := r.repo.UpdateFacilitatorSyncTime(ctx, f.ID, now); err != nil {
			return 0, fmt.Errorf("update sync time: %w", err)
		}
		return 0, nil
	}

	// Convert CDP transfers to model transactions
	txs := make([]models.Transaction, 0, len(transfers))
	for _, t := range transfers {
		txs = append(txs, models.Transaction{
			ID:                 uuid.New(),
			TxHash:             t.TransactionHash,
			BlockNumber:        t.BlockNumber,
			BlockTime:          t.BlockTimestamp,
			EventType:          "Transfer",
			ProxyContract:      "",
			FacilitatorAddress: f.Address,
			PayerAddress:       t.Sender,
			RecipientAddress:   t.ToAddress,
			AmountRaw:          t.Amount,
			AmountUSD:          usdcToFloat(t.Amount),
			AssetAddress:       usdcBaseAddress,
			IndexedAt:          now,
		})
	}

	inserted, err := r.repo.InsertTransactions(ctx, txs)
	if err != nil {
		return 0, fmt.Errorf("insert transactions: %w", err)
	}

	if err := r.repo.UpdateFacilitatorSyncTime(ctx, f.ID, now); err != nil {
		return inserted, fmt.Errorf("update sync time: %w", err)
	}

	return inserted, nil
}

func defaultSinceTime(lastSynced *time.Time) time.Time {
	if lastSynced != nil {
		return *lastSynced
	}
	t, _ := time.Parse("2006-01-02", defaultStartDate)
	return t
}

// usdcToFloat converts a raw USDC amount string (6 decimals) to a float64 USD value.
func usdcToFloat(raw string) float64 {
	amount, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		return 0
	}
	f := new(big.Float).SetInt(amount)
	divisor := new(big.Float).SetFloat64(1e6)
	result, _ := new(big.Float).Quo(f, divisor).Float64()
	return result
}
