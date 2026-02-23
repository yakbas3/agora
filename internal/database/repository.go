package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yamanakbas/agora/internal/models"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// StartCrawlRun inserts a new crawl run record and returns its ID.
func (r *Repository) StartCrawlRun(ctx context.Context) (uuid.UUID, error) {
	id := uuid.New()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO crawl_runs (id, started_at, status) VALUES ($1, $2, 'running')`,
		id, time.Now().UTC(),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("start crawl run: %w", err)
	}
	return id, nil
}

// CompleteCrawlRun marks a crawl run as completed with stats.
func (r *Repository) CompleteCrawlRun(ctx context.Context, id uuid.UUID, totalFetched, newEndpoints, updatedEndpoints int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE crawl_runs
		 SET completed_at = $2, total_fetched = $3, new_endpoints = $4,
		     updated_endpoints = $5, status = 'completed'
		 WHERE id = $1`,
		id, time.Now().UTC(), totalFetched, newEndpoints, updatedEndpoints,
	)
	if err != nil {
		return fmt.Errorf("complete crawl run: %w", err)
	}
	return nil
}

// FailCrawlRun marks a crawl run as failed with an error message.
func (r *Repository) FailCrawlRun(ctx context.Context, id uuid.UUID, crawlErr string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE crawl_runs
		 SET completed_at = $2, status = 'failed', error = $3
		 WHERE id = $1`,
		id, time.Now().UTC(), crawlErr,
	)
	if err != nil {
		return fmt.Errorf("fail crawl run: %w", err)
	}
	return nil
}

// UpsertEndpoint inserts or updates an endpoint and replaces its payment options.
// Returns (isNew, isUpdated, error).
func (r *Repository) UpsertEndpoint(ctx context.Context, endpoint models.Endpoint, options []models.PaymentOption) (bool, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Try to get existing endpoint
	var existingID uuid.UUID
	var existingLastUpdated time.Time
	err = tx.QueryRow(ctx,
		`SELECT id, last_updated FROM endpoints WHERE resource_url = $1`,
		endpoint.ResourceURL,
	).Scan(&existingID, &existingLastUpdated)

	isNew := false
	isUpdated := false

	if err == pgx.ErrNoRows {
		// Insert new endpoint
		isNew = true
		_, err = tx.Exec(ctx,
			`INSERT INTO endpoints (id, resource_url, domain, type, x402_version,
			   description, http_method, input_schema, output_schema, raw_metadata,
			   last_updated, first_seen, last_crawled)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			endpoint.ID, endpoint.ResourceURL, endpoint.Domain, endpoint.Type,
			endpoint.X402Version, endpoint.Description, endpoint.HTTPMethod,
			endpoint.InputSchema, endpoint.OutputSchema, endpoint.RawMetadata,
			endpoint.LastUpdated, endpoint.FirstSeen, endpoint.LastCrawled,
		)
		if err != nil {
			return false, false, fmt.Errorf("insert endpoint: %w", err)
		}
	} else if err != nil {
		return false, false, fmt.Errorf("query existing endpoint: %w", err)
	} else {
		// Existing endpoint — update if lastUpdated changed
		endpoint.ID = existingID
		if endpoint.LastUpdated.After(existingLastUpdated) {
			isUpdated = true
			_, err = tx.Exec(ctx,
				`UPDATE endpoints
				 SET domain = $2, type = $3, x402_version = $4, description = $5,
				     http_method = $6, input_schema = $7, output_schema = $8,
				     raw_metadata = $9, last_updated = $10, last_crawled = $11
				 WHERE id = $1`,
				endpoint.ID, endpoint.Domain, endpoint.Type, endpoint.X402Version,
				endpoint.Description, endpoint.HTTPMethod, endpoint.InputSchema,
				endpoint.OutputSchema, endpoint.RawMetadata, endpoint.LastUpdated,
				endpoint.LastCrawled,
			)
			if err != nil {
				return false, false, fmt.Errorf("update endpoint: %w", err)
			}
		} else {
			// Just touch last_crawled
			_, err = tx.Exec(ctx,
				`UPDATE endpoints SET last_crawled = $2 WHERE id = $1`,
				endpoint.ID, endpoint.LastCrawled,
			)
			if err != nil {
				return false, false, fmt.Errorf("touch last_crawled: %w", err)
			}
		}

		// Delete old payment options
		_, err = tx.Exec(ctx,
			`DELETE FROM payment_options WHERE endpoint_id = $1`,
			endpoint.ID,
		)
		if err != nil {
			return false, false, fmt.Errorf("delete old payment options: %w", err)
		}
	}

	// Insert new payment options
	if len(options) > 0 {
		batch := &pgx.Batch{}
		for _, opt := range options {
			opt.EndpointID = endpoint.ID
			batch.Queue(
				`INSERT INTO payment_options (id, endpoint_id, scheme, network_raw,
				   network_normalized, asset_address, asset_name, max_amount_raw,
				   price_usd, pay_to, max_timeout_seconds, mime_type, description,
				   output_schema_raw)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
				opt.ID, endpoint.ID, opt.Scheme, opt.NetworkRaw,
				opt.NetworkNormalized, opt.AssetAddress, opt.AssetName,
				opt.MaxAmountRaw, opt.PriceUSD, opt.PayTo,
				opt.MaxTimeoutSeconds, opt.MimeType, opt.Description,
				opt.OutputSchemaRaw,
			)
		}
		br := tx.SendBatch(ctx, batch)
		for range options {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return false, false, fmt.Errorf("insert payment option: %w", err)
			}
		}
		br.Close()
	}

	if err := tx.Commit(ctx); err != nil {
		return false, false, fmt.Errorf("commit tx: %w", err)
	}

	return isNew, isUpdated, nil
}

// GetLastIndexedBlock returns the last fully indexed block number.
func (r *Repository) GetLastIndexedBlock(ctx context.Context) (int64, error) {
	var lastBlock int64
	err := r.pool.QueryRow(ctx,
		`SELECT last_block FROM indexer_state WHERE id = 1`,
	).Scan(&lastBlock)
	if err != nil {
		return 0, fmt.Errorf("get last indexed block: %w", err)
	}
	return lastBlock, nil
}

// UpdateLastIndexedBlock sets the last fully indexed block number.
func (r *Repository) UpdateLastIndexedBlock(ctx context.Context, blockNumber int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE indexer_state SET last_block = $1, updated_at = $2 WHERE id = 1`,
		blockNumber, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("update last indexed block: %w", err)
	}
	return nil
}

// InsertTransactions batch-inserts transactions, skipping duplicates (ON CONFLICT DO NOTHING).
func (r *Repository) InsertTransactions(ctx context.Context, txs []models.Transaction) (int, error) {
	if len(txs) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, tx := range txs {
		batch.Queue(
			`INSERT INTO transactions (id, tx_hash, block_number, block_time, event_type,
			   proxy_contract, facilitator_address, payer_address, recipient_address,
			   amount_raw, amount_usd, asset_address, indexed_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			 ON CONFLICT (tx_hash) DO NOTHING`,
			tx.ID, tx.TxHash, tx.BlockNumber, tx.BlockTime, tx.EventType,
			tx.ProxyContract, tx.FacilitatorAddress, tx.PayerAddress,
			tx.RecipientAddress, tx.AmountRaw, tx.AmountUSD, tx.AssetAddress,
			tx.IndexedAt,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	inserted := 0
	for range txs {
		ct, err := br.Exec()
		if err != nil {
			br.Close()
			return inserted, fmt.Errorf("insert transaction: %w", err)
		}
		if ct.RowsAffected() > 0 {
			inserted++
		}
	}
	br.Close()

	return inserted, nil
}

// RefreshEndpointScores refreshes the endpoint_scores materialized view.
func (r *Repository) RefreshEndpointScores(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `REFRESH MATERIALIZED VIEW CONCURRENTLY endpoint_scores`)
	if err != nil {
		return fmt.Errorf("refresh endpoint_scores: %w", err)
	}
	return nil
}

// RefreshDiscoveredSellers updates the discovered_sellers table from unmatched transactions.
func (r *Repository) RefreshDiscoveredSellers(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO discovered_sellers (pay_to, tx_count, total_volume_usd, unique_payers, first_seen_at, last_seen_at)
		SELECT
			t.recipient_address,
			COUNT(*)::INTEGER,
			COALESCE(SUM(t.amount_usd), 0),
			COUNT(DISTINCT t.payer_address)::INTEGER,
			MIN(t.block_time),
			MAX(t.block_time)
		FROM transactions t
		WHERE NOT EXISTS (
			SELECT 1 FROM payment_options po WHERE po.pay_to = t.recipient_address
		)
		GROUP BY t.recipient_address
		ON CONFLICT (pay_to) DO UPDATE SET
			tx_count = EXCLUDED.tx_count,
			total_volume_usd = EXCLUDED.total_volume_usd,
			unique_payers = EXCLUDED.unique_payers,
			first_seen_at = EXCLUDED.first_seen_at,
			last_seen_at = EXCLUDED.last_seen_at
	`)
	if err != nil {
		return fmt.Errorf("refresh discovered_sellers: %w", err)
	}
	return nil
}
