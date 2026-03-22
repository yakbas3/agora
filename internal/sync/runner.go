package sync

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
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

	// Skip facilitators synced in the last hour
	skipped := 0
	for i, f := range facilitators {
		if f.LastSyncedAt != nil && now.Sub(*f.LastSyncedAt) < time.Hour {
			skipped++
			continue
		}
		if i > 0 {
			time.Sleep(2 * time.Second) // Rate limit: ~1 req per 2s
		}
		log.Printf("[%d/%d] Syncing %s (%s)...", i+1, len(facilitators), f.Name, f.Address[:10]+"...")
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
	if skipped > 0 {
		log.Printf("Skipped %d recently-synced facilitators", skipped)
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
	totalInserted := 0

	// Query in monthly chunks to avoid CDP scan limits
	chunkStart := since
	for chunkStart.Before(now) {
		chunkEnd := chunkStart.AddDate(0, 1, 0) // +1 month
		if chunkEnd.After(now) {
			chunkEnd = now
		}

		log.Printf("    chunk %s → %s", chunkStart.Format("2006-01-02"), chunkEnd.Format("2006-01-02"))
		transfers, err := r.cdpClient.QueryTransfers(f.Address, chunkStart, chunkEnd)
		if err != nil {
			// If scan limit exceeded, try weekly chunks
			if strings.Contains(err.Error(), "Limit for rows or bytes") {
				weekInserted, weekErr := r.syncFacilitatorWeekly(ctx, f, chunkStart, chunkEnd, now)
				if weekErr != nil {
					return totalInserted, weekErr
				}
				totalInserted += weekInserted
				chunkStart = chunkEnd
				continue
			}
			return totalInserted, fmt.Errorf("query transfers: %w", err)
		}

		inserted, err := r.insertTransfers(ctx, f, transfers, now)
		if err != nil {
			return totalInserted, err
		}
		totalInserted += inserted
		chunkStart = chunkEnd
	}

	if err := r.repo.UpdateFacilitatorSyncTime(ctx, f.ID, now); err != nil {
		return totalInserted, fmt.Errorf("update sync time: %w", err)
	}
	return totalInserted, nil
}

func (r *Runner) syncFacilitatorWeekly(ctx context.Context, f models.Facilitator, start, end, now time.Time) (int, error) {
	totalInserted := 0
	chunkStart := start
	for chunkStart.Before(end) {
		chunkEnd := chunkStart.AddDate(0, 0, 7) // +1 week
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		transfers, err := r.cdpClient.QueryTransfers(f.Address, chunkStart, chunkEnd)
		if err != nil {
			return totalInserted, fmt.Errorf("weekly query transfers: %w", err)
		}
		inserted, err := r.insertTransfers(ctx, f, transfers, now)
		if err != nil {
			return totalInserted, err
		}
		totalInserted += inserted
		chunkStart = chunkEnd
	}
	return totalInserted, nil
}

func (r *Runner) insertTransfers(ctx context.Context, f models.Facilitator, transfers []cdp.Transfer, now time.Time) (int, error) {
	if len(transfers) == 0 {
		return 0, nil
	}

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
