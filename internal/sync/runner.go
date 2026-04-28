package sync

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/yamanakbas/agora/internal/cdp"
	"github.com/yamanakbas/agora/internal/database"
	"github.com/yamanakbas/agora/internal/models"
)

const (
	// Default start date for facilitators that have never been synced. x402
	// activity before this was near-zero, and starting earlier just wastes CDP
	// scan budget.
	defaultStartDate = "2024-06-01"
	// USDC contract address on Base.
	usdcBaseAddress = "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
	// Max facilitator addresses synced in parallel. CDP's rate limit appears to
	// be ~1-2 QPS per API key, so going higher just makes every request collide
	// on 429 retries. Keep this at 2 — the real speedup comes from the cursor
	// eliminating redundant month scans, not from raw parallelism.
	defaultConcurrency = 2
)

// scanLimitFragment identifies the CDP "scan budget exceeded" 400 error. The
// query itself is valid; the issue is the underlying columnar store can't scan
// enough bytes to satisfy it. Fix: narrow the time window.
const scanLimitFragment = "Limit for rows or bytes"

type Runner struct {
	cdpClient   *cdp.Client
	repo        *database.Repository
	concurrency int
}

func NewRunner(cdpClient *cdp.Client, repo *database.Repository) *Runner {
	return &Runner{cdpClient: cdpClient, repo: repo, concurrency: defaultConcurrency}
}

func (r *Runner) Run(ctx context.Context) error {
	return r.RunFiltered(ctx, "")
}

// RunMissing syncs only Base facilitators that have zero transactions in the
// DB. Use this to resume an interrupted import without re-scanning the
// facilitators we already have data for.
func (r *Runner) RunMissing(ctx context.Context) error {
	facilitators, err := r.repo.GetBaseFacilitatorsMissingTransactions(ctx)
	if err != nil {
		return fmt.Errorf("load missing facilitators: %w", err)
	}
	if len(facilitators) == 0 {
		log.Println("No missing facilitators — all Base facilitators already have transactions.")
		return nil
	}
	return r.runSet(ctx, facilitators)
}

// RunFiltered syncs all Base facilitators in parallel, optionally filtered to a
// single facilitator name (case-insensitive, exact match). An empty nameFilter
// syncs all facilitators.
//
// Each facilitator is synced incrementally using a per-address timestamp cursor:
// we query MAX(block_time) from the transactions table for that address and
// resume from there. This means partial/interrupted runs resume transparently.
// Within the [cursor, now) range we still chunk by month (with a weekly
// fallback on CDP scan-limit errors) because base.events is large enough that a
// single long query exceeds the 93 GiB scan budget even when the result is tiny.
//
// Pattern borrowed from x402scan's sync/transfers/trigger/sync.ts (cursor and
// parallelism) combined with our pre-existing month/week chunking.
func (r *Runner) RunFiltered(ctx context.Context, nameFilter string) error {
	facilitators, err := r.repo.GetBaseFacilitators(ctx)
	if err != nil {
		return fmt.Errorf("load facilitators: %w", err)
	}

	if nameFilter != "" {
		filtered := facilitators[:0]
		for _, f := range facilitators {
			if strings.EqualFold(f.Name, nameFilter) {
				filtered = append(filtered, f)
			}
		}
		facilitators = filtered
		if len(facilitators) == 0 {
			return fmt.Errorf("no facilitator found matching name %q", nameFilter)
		}
	}

	return r.runSet(ctx, facilitators)
}

// runSet syncs a pre-selected list of facilitators in parallel and refreshes
// materialized views at the end if any transactions were inserted.
func (r *Runner) runSet(ctx context.Context, facilitators []models.Facilitator) error {
	log.Printf("Syncing %d Base facilitators (concurrency=%d)", len(facilitators), r.concurrency)

	now := time.Now().UTC()
	var totalInserted int64
	var completed int64

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(r.concurrency)

	for _, f := range facilitators {
		f := f
		g.Go(func() error {
			inserted, err := r.syncFacilitator(gctx, f, now)
			n := atomic.AddInt64(&completed, 1)
			if err != nil {
				log.Printf("  [%d/%d] %s (%s): ERROR: %v",
					n, len(facilitators), f.Name, shortAddr(f.Address), err)
				return nil // don't abort the whole run on a single failure
			}
			atomic.AddInt64(&totalInserted, int64(inserted))
			log.Printf("  [%d/%d] %s (%s): +%d transactions",
				n, len(facilitators), f.Name, shortAddr(f.Address), inserted)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	log.Printf("Sync complete: %d new transactions total", totalInserted)

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

// syncFacilitator fetches all USDC Transfer events for a single facilitator
// address since MAX(block_time) in our DB (or the default start date) up to
// `now`, in monthly chunks. Falls back to weekly chunks on CDP scan-limit
// errors. Bumps last_synced_at once done.
func (r *Runner) syncFacilitator(ctx context.Context, f models.Facilitator, now time.Time) (int, error) {
	since, err := r.cursor(ctx, f)
	if err != nil {
		return 0, fmt.Errorf("read cursor: %w", err)
	}

	// If we're within 30s of now, skip the API call entirely and just bump
	// last_synced_at for visibility.
	if now.Sub(since) < 30*time.Second {
		return 0, r.repo.UpdateFacilitatorSyncTime(ctx, f.ID, now)
	}

	totalInserted := 0
	chunkStart := since
	for chunkStart.Before(now) {
		chunkEnd := chunkStart.AddDate(0, 1, 0)
		if chunkEnd.After(now) {
			chunkEnd = now
		}

		transfers, err := r.cdpClient.QueryTransfers(f.Address, chunkStart, chunkEnd)
		if err != nil {
			if strings.Contains(err.Error(), scanLimitFragment) {
				weekInserted, werr := r.syncWeekly(ctx, f, chunkStart, chunkEnd, now)
				if werr != nil {
					return totalInserted, werr
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

// syncWeekly chunks a failing monthly window into 7-day windows. If those also
// fail, we give up on the chunk — narrower than weekly is probably pointless
// since a single week that can't be scanned means the facilitator has huge
// volume and needs a different data source anyway.
func (r *Runner) syncWeekly(ctx context.Context, f models.Facilitator, start, end, now time.Time) (int, error) {
	totalInserted := 0
	chunkStart := start
	for chunkStart.Before(end) {
		chunkEnd := chunkStart.AddDate(0, 0, 7)
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

// cursor returns the start time for a sync run: MAX(block_time) + 1s for the
// facilitator, or the default start date if we've never seen a transaction
// from this address.
func (r *Runner) cursor(ctx context.Context, f models.Facilitator) (time.Time, error) {
	maxBlockTime, err := r.repo.GetMaxBlockTimeForFacilitator(ctx, f.Address)
	if err != nil {
		return time.Time{}, err
	}
	if maxBlockTime != nil {
		return maxBlockTime.Add(time.Second), nil
	}
	start, _ := time.Parse("2006-01-02", defaultStartDate)
	return start, nil
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
	return inserted, nil
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

func shortAddr(a string) string {
	if len(a) < 10 {
		return a
	}
	return a[:10] + "..."
}
