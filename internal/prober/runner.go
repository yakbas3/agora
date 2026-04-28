package prober

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yamanakbas/agora/internal/database"
	"github.com/yamanakbas/agora/internal/models"
	"golang.org/x/sync/errgroup"
)

// Repository is the subset of database.Repository used by the runner.
type Repository interface {
	GetEndpointsForProbing(ctx context.Context, limit, offset int) ([]database.ProbeTarget, error)
	GetTotalEndpointCount(ctx context.Context) (int, error)
	InsertProbeResults(ctx context.Context, results []models.ProbeResult) (int, error)
	ApplyProbeUpdates(ctx context.Context, results []models.ProbeResult) (int, error)
	RefreshEndpointScores(ctx context.Context) error
}

// Config controls probe runner behaviour.
type Config struct {
	Concurrency int
	BatchSize   int
	DomainDelay time.Duration
	// MaxEndpoints, when > 0, caps the total number of endpoints probed in this
	// run. Useful for smoke-testing the pipeline without waiting 15-30 minutes
	// for a full 12k sweep. 0 means unlimited.
	MaxEndpoints int
}

// Runner orchestrates the full probe cycle over all endpoints.
type Runner struct {
	client *Client
	repo   Repository
	cfg    Config
}

// NewRunner creates a Runner.
func NewRunner(client *Client, repo Repository, cfg Config) *Runner {
	return &Runner{client: client, repo: repo, cfg: cfg}
}

// Run probes all endpoints and refreshes the materialized view when done.
func (r *Runner) Run(ctx context.Context) error {
	total, err := r.repo.GetTotalEndpointCount(ctx)
	if err != nil {
		return fmt.Errorf("count endpoints: %w", err)
	}
	if r.cfg.MaxEndpoints > 0 && r.cfg.MaxEndpoints < total {
		log.Printf("Probing %d of %d endpoints (concurrency=%d, domainDelay=%s) — MaxEndpoints cap",
			r.cfg.MaxEndpoints, total, r.cfg.Concurrency, r.cfg.DomainDelay)
		total = r.cfg.MaxEndpoints
	} else {
		log.Printf("Probing %d endpoints (concurrency=%d, domainDelay=%s)",
			total, r.cfg.Concurrency, r.cfg.DomainDelay)
	}

	// domainLast tracks when we last sent a request to a given domain.
	var domainLast sync.Map

	// mu guards the results slice collected across goroutines.
	var mu sync.Mutex
	var pending []models.ProbeResult

	flushPending := func() error {
		mu.Lock()
		batch := pending
		pending = nil
		mu.Unlock()
		if len(batch) == 0 {
			return nil
		}
		n, err := r.repo.InsertProbeResults(ctx, batch)
		if err != nil {
			return fmt.Errorf("insert probe results: %w", err)
		}
		updated, err := r.repo.ApplyProbeUpdates(ctx, batch)
		if err != nil {
			return fmt.Errorf("apply probe updates: %w", err)
		}
		log.Printf("  Inserted %d probe results, updated %d payment_options rows", n, updated)
		return nil
	}

	probed := 0
	for offset := 0; offset < total; offset += r.cfg.BatchSize {
		batchLimit := r.cfg.BatchSize
		if remaining := total - offset; remaining < batchLimit {
			batchLimit = remaining
		}
		dbTargets, err := r.repo.GetEndpointsForProbing(ctx, batchLimit, offset)
		if err != nil {
			return fmt.Errorf("load batch at offset %d: %w", offset, err)
		}
		if len(dbTargets) == 0 {
			break
		}

		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(r.cfg.Concurrency)

		for _, dt := range dbTargets {
			dt := dt // capture loop variable
			g.Go(func() error {
				t := toProbeTarget(dt)

				// Per-domain rate limiting.
				domain := extractDomain(t.ResourceURL)
				if domain != "" && r.cfg.DomainDelay > 0 {
					for {
						now := time.Now()
						raw, _ := domainLast.LoadOrStore(domain, now)
						last := raw.(time.Time)
						elapsed := now.Sub(last)
						if elapsed >= r.cfg.DomainDelay {
							domainLast.Store(domain, now)
							break
						}
						select {
						case <-gctx.Done():
							return gctx.Err()
						case <-time.After(r.cfg.DomainDelay - elapsed):
						}
					}
				}

				outcome := r.client.Probe(gctx, t)
				result := outcomeToModel(outcome)

				mu.Lock()
				pending = append(pending, result)
				mu.Unlock()
				return nil
			})
		}

		if err := g.Wait(); err != nil && err != context.Canceled {
			return fmt.Errorf("probe batch: %w", err)
		}

		if err := flushPending(); err != nil {
			return err
		}

		probed += len(dbTargets)
		log.Printf("Progress: %d / %d endpoints probed", probed, total)
	}

	// Final flush.
	if err := flushPending(); err != nil {
		return err
	}

	log.Println("Refreshing endpoint_scores materialized view...")
	if err := r.repo.RefreshEndpointScores(ctx); err != nil {
		return fmt.Errorf("refresh scores: %w", err)
	}

	log.Printf("Probe complete. %d endpoints probed.", probed)
	return nil
}

func toProbeTarget(dt database.ProbeTarget) ProbeTarget {
	refs := make([]PaymentRef, len(dt.PaymentOptions))
	for i, r := range dt.PaymentOptions {
		refs[i] = PaymentRef{
			PayTo:     r.PayTo,
			AmountRaw: r.AmountRaw,
			Network:   r.Network,
			Asset:     r.Asset,
			PriceUSD:  r.PriceUSD,
		}
	}
	return ProbeTarget{
		EndpointID:     dt.EndpointID,
		ResourceURL:    dt.ResourceURL,
		HTTPMethod:     dt.HTTPMethod,
		Domain:         dt.Domain,
		PaymentOptions: refs,
	}
}

func outcomeToModel(o ProbeOutcome) models.ProbeResult {
	return models.ProbeResult{
		ID:                 uuid.New(),
		EndpointID:         o.EndpointID,
		ProbedAt:           time.Now().UTC(),
		StatusCode:         o.StatusCode,
		LatencyMs:          o.LatencyMs,
		HealthStatus:       o.HealthStatus,
		IsValid402:         o.IsValid402,
		ErrorMessage:       o.ErrorMessage,
		ResponsePayTo:      o.ResponsePayTo,
		ResponseAmountRaw:  o.ResponseAmountRaw,
		ResponseNetwork:    o.ResponseNetwork,
		ResponseAsset:      o.ResponseAsset,
		PriceMatch:         o.PriceMatch,
		DiscrepancyDetails: o.DiscrepancyDetails,
	}
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}
