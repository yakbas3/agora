package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
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

// SearchFilters holds optional filters for semantic search.
type SearchFilters struct {
	Network  string
	Method   string
	MinPrice *float64
	MaxPrice *float64
}

// SearchResult holds an endpoint with its similarity score.
type SearchResult struct {
	Endpoint   models.Endpoint
	Similarity float64
}

func buildSearchQuery(filters SearchFilters, limit int) (string, []any) {
	// $1 is always the query vector, filled by caller
	args := []any{nil} // placeholder for vector
	argIdx := 2

	joins := ""
	where := "WHERE e.embedding IS NOT NULL"

	if filters.Network != "" {
		joins = "JOIN payment_options po ON po.endpoint_id = e.id"
		where += fmt.Sprintf(" AND po.network_normalized = $%d", argIdx)
		args = append(args, filters.Network)
		argIdx++
	}

	if filters.Method != "" {
		where += fmt.Sprintf(" AND e.http_method = $%d", argIdx)
		args = append(args, filters.Method)
		argIdx++
	}

	needsPO := filters.MinPrice != nil || filters.MaxPrice != nil
	if needsPO && joins == "" {
		joins = "JOIN payment_options po ON po.endpoint_id = e.id"
	}

	if filters.MinPrice != nil {
		where += fmt.Sprintf(" AND po.price_usd >= $%d", argIdx)
		args = append(args, *filters.MinPrice)
		argIdx++
	}

	if filters.MaxPrice != nil {
		where += fmt.Sprintf(" AND po.price_usd <= $%d", argIdx)
		args = append(args, *filters.MaxPrice)
		argIdx++
	}

	q := fmt.Sprintf(`
		SELECT DISTINCT ON (e.id)
			e.id, e.resource_url, e.domain, e.type, e.x402_version,
			e.description, e.http_method, e.input_schema, e.output_schema,
			e.raw_metadata, e.last_updated, e.first_seen, e.last_crawled,
			1 - (e.embedding <=> $1) AS similarity,
			COALESCE(es.reliability_score, 0) AS reliability_score,
			COALESCE(es.health_status, 'unknown') AS health_status,
			COALESCE(es.latency_ms, 0) AS latency_ms,
			es.last_probed_at
		FROM endpoints e
		LEFT JOIN endpoint_scores es ON es.endpoint_id = e.id
		%s
		%s
		ORDER BY e.id, similarity DESC
	`, joins, where)

	// Wrap to re-order by similarity and apply limit
	q = fmt.Sprintf(`SELECT * FROM (%s) sub ORDER BY similarity DESC LIMIT $%d`, q, argIdx)
	args = append(args, limit)

	return q, args
}

// SearchByVector finds the most similar endpoints to the given vector.
func (r *Repository) SearchByVector(ctx context.Context, vector pgvector.Vector, filters SearchFilters, limit int) ([]SearchResult, error) {
	q, args := buildSearchQuery(filters, limit)
	args[0] = vector // fill the vector placeholder

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var sr SearchResult
		err := rows.Scan(
			&sr.Endpoint.ID, &sr.Endpoint.ResourceURL, &sr.Endpoint.Domain,
			&sr.Endpoint.Type, &sr.Endpoint.X402Version, &sr.Endpoint.Description,
			&sr.Endpoint.HTTPMethod, &sr.Endpoint.InputSchema, &sr.Endpoint.OutputSchema,
			&sr.Endpoint.RawMetadata, &sr.Endpoint.LastUpdated, &sr.Endpoint.FirstSeen,
			&sr.Endpoint.LastCrawled, &sr.Similarity, &sr.Endpoint.ReliabilityScore,
			&sr.Endpoint.HealthStatus, &sr.Endpoint.LatencyMs, &sr.Endpoint.LastProbedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, sr)
	}
	return results, rows.Err()
}

// GetEndpoints returns a paginated list of endpoints.
func (r *Repository) GetEndpoints(ctx context.Context, limit, offset int) ([]models.Endpoint, error) {
	q := `
		SELECT e.id, e.resource_url, e.domain, e.type, e.x402_version, e.description,
		       e.http_method, e.input_schema, e.output_schema, e.raw_metadata,
		       e.last_updated, e.first_seen, e.last_crawled,
		       COALESCE(es.reliability_score, 0),
		       COALESCE(es.health_status, 'unknown'),
		       COALESCE(es.latency_ms, 0),
		       es.last_probed_at
		FROM endpoints e
		LEFT JOIN endpoint_scores es ON es.endpoint_id = e.id
		ORDER BY e.last_crawled DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []models.Endpoint
	for rows.Next() {
		var e models.Endpoint
		err := rows.Scan(
			&e.ID, &e.ResourceURL, &e.Domain, &e.Type, &e.X402Version,
			&e.Description, &e.HTTPMethod, &e.InputSchema, &e.OutputSchema,
			&e.RawMetadata, &e.LastUpdated, &e.FirstSeen, &e.LastCrawled,
			&e.ReliabilityScore, &e.HealthStatus, &e.LatencyMs, &e.LastProbedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan endpoint: %w", err)
		}
		endpoints = append(endpoints, e)
	}
	return endpoints, rows.Err()
}

// GetEndpointByID returns a single endpoint with its payment options.
func (r *Repository) GetEndpointByID(ctx context.Context, id uuid.UUID) (*models.Endpoint, []models.PaymentOption, error) {
	eq := `
		SELECT e.id, e.resource_url, e.domain, e.type, e.x402_version, e.description,
		       e.http_method, e.input_schema, e.output_schema, e.raw_metadata,
		       e.last_updated, e.first_seen, e.last_crawled,
		       COALESCE(es.reliability_score, 0),
		       COALESCE(es.health_status, 'unknown'),
		       COALESCE(es.latency_ms, 0),
		       es.last_probed_at
		FROM endpoints e
		LEFT JOIN endpoint_scores es ON es.endpoint_id = e.id
		WHERE e.id = $1
	`
	var e models.Endpoint
	err := r.pool.QueryRow(ctx, eq, id).Scan(
		&e.ID, &e.ResourceURL, &e.Domain, &e.Type, &e.X402Version,
		&e.Description, &e.HTTPMethod, &e.InputSchema, &e.OutputSchema,
		&e.RawMetadata, &e.LastUpdated, &e.FirstSeen, &e.LastCrawled,
		&e.ReliabilityScore, &e.HealthStatus, &e.LatencyMs, &e.LastProbedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get endpoint: %w", err)
	}

	pq := `
		SELECT id, endpoint_id, scheme, network_raw, network_normalized,
		       asset_address, asset_name, max_amount_raw, price_usd,
		       pay_to, max_timeout_seconds, mime_type, description, output_schema_raw
		FROM payment_options WHERE endpoint_id = $1
	`
	rows, err := r.pool.Query(ctx, pq, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get payment options: %w", err)
	}
	defer rows.Close()

	var pos []models.PaymentOption
	for rows.Next() {
		var po models.PaymentOption
		err := rows.Scan(
			&po.ID, &po.EndpointID, &po.Scheme, &po.NetworkRaw, &po.NetworkNormalized,
			&po.AssetAddress, &po.AssetName, &po.MaxAmountRaw, &po.PriceUSD,
			&po.PayTo, &po.MaxTimeoutSeconds, &po.MimeType, &po.Description, &po.OutputSchemaRaw,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("scan payment option: %w", err)
		}
		pos = append(pos, po)
	}
	return &e, pos, rows.Err()
}

// StatsResult holds network-wide aggregation data.
type StatsResult struct {
	TotalEndpoints     int                `json:"total_endpoints"`
	TotalDomains       int                `json:"total_domains"`
	EndpointsByNetwork []NameCount        `json:"endpoints_by_network"`
	EndpointsByAsset   []NameCount        `json:"endpoints_by_asset"`
	EndpointsByPrice   []NameCount        `json:"endpoints_by_price_bracket"`
	EndpointsOverTime  []DateCount        `json:"endpoints_over_time"`
	CrawlHistory         []models.CrawlRun  `json:"crawl_history"`
	TotalTransactions    int                `json:"total_transactions"`
	TotalVolumeUSD       float64            `json:"total_volume_usd"`
	TransactionsOverTime []DateCount        `json:"transactions_over_time"`
	AvgReliability       float64            `json:"avg_reliability"`
	AliveCount           int                `json:"alive_count"`
	DeadCount            int                `json:"dead_count"`
	UnknownCount         int                `json:"unknown_count"`
}

type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type DateCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// GetStats returns network-wide aggregation data.
func (r *Repository) GetStats(ctx context.Context) (*StatsResult, error) {
	s := &StatsResult{}

	// Total endpoints and domains
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(DISTINCT domain) FROM endpoints`,
	).Scan(&s.TotalEndpoints, &s.TotalDomains)
	if err != nil {
		return nil, fmt.Errorf("stats totals: %w", err)
	}

	// Endpoints by network
	rows, err := r.pool.Query(ctx,
		`SELECT po.network_normalized, COUNT(DISTINCT po.endpoint_id)
		 FROM payment_options po
		 GROUP BY po.network_normalized
		 ORDER BY COUNT(DISTINCT po.endpoint_id) DESC`)
	if err != nil {
		return nil, fmt.Errorf("stats by network: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var nc NameCount
		if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, fmt.Errorf("scan network: %w", err)
		}
		s.EndpointsByNetwork = append(s.EndpointsByNetwork, nc)
	}

	// Endpoints by asset
	rows2, err := r.pool.Query(ctx,
		`SELECT po.asset_name, COUNT(DISTINCT po.endpoint_id)
		 FROM payment_options po
		 GROUP BY po.asset_name
		 ORDER BY COUNT(DISTINCT po.endpoint_id) DESC`)
	if err != nil {
		return nil, fmt.Errorf("stats by asset: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var nc NameCount
		if err := rows2.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		s.EndpointsByAsset = append(s.EndpointsByAsset, nc)
	}

	// Endpoints by price bracket
	rows3, err := r.pool.Query(ctx,
		`SELECT bracket, COUNT(*) FROM (
			SELECT CASE
				WHEN po.price_usd < 0.001 THEN '$0-0.001'
				WHEN po.price_usd < 0.01 THEN '$0.001-0.01'
				WHEN po.price_usd < 0.1 THEN '$0.01-0.1'
				ELSE '$0.1+'
			END AS bracket
			FROM payment_options po
		) sub GROUP BY bracket ORDER BY bracket`)
	if err != nil {
		return nil, fmt.Errorf("stats by price: %w", err)
	}
	defer rows3.Close()
	for rows3.Next() {
		var nc NameCount
		if err := rows3.Scan(&nc.Name, &nc.Count); err != nil {
			return nil, fmt.Errorf("scan price: %w", err)
		}
		s.EndpointsByPrice = append(s.EndpointsByPrice, nc)
	}

	// Endpoints over time (by first_seen date)
	rows4, err := r.pool.Query(ctx,
		`SELECT DATE(first_seen)::text AS d, COUNT(*)
		 FROM endpoints
		 GROUP BY d
		 ORDER BY d`)
	if err != nil {
		return nil, fmt.Errorf("stats over time: %w", err)
	}
	defer rows4.Close()
	cumulative := 0
	for rows4.Next() {
		var dc DateCount
		var dailyCount int
		if err := rows4.Scan(&dc.Date, &dailyCount); err != nil {
			return nil, fmt.Errorf("scan date: %w", err)
		}
		cumulative += dailyCount
		dc.Count = cumulative
		s.EndpointsOverTime = append(s.EndpointsOverTime, dc)
	}

	// Crawl history (last 10)
	rows5, err := r.pool.Query(ctx,
		`SELECT id, started_at, completed_at, total_fetched,
		        new_endpoints, updated_endpoints, status, error
		 FROM crawl_runs
		 ORDER BY started_at DESC
		 LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("stats crawl history: %w", err)
	}
	defer rows5.Close()
	for rows5.Next() {
		var cr models.CrawlRun
		if err := rows5.Scan(&cr.ID, &cr.StartedAt, &cr.CompletedAt,
			&cr.TotalFetched, &cr.NewEndpoints, &cr.UpdatedEndpoints,
			&cr.Status, &cr.Error); err != nil {
			return nil, fmt.Errorf("scan crawl run: %w", err)
		}
		s.CrawlHistory = append(s.CrawlHistory, cr)
	}

	// Transaction totals
	r.pool.QueryRow(ctx,
		`SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(amount_usd), 0) FROM transactions`,
	).Scan(&s.TotalTransactions, &s.TotalVolumeUSD)

	// Average reliability score
	r.pool.QueryRow(ctx,
		`SELECT COALESCE(AVG(reliability_score), 0) FROM endpoint_scores WHERE reliability_score > 0`,
	).Scan(&s.AvgReliability)

	// Health status breakdown
	r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE health_status = 'alive'),
			COUNT(*) FILTER (WHERE health_status = 'dead'),
			COUNT(*) FILTER (WHERE health_status = 'unknown')
		FROM endpoint_scores
	`).Scan(&s.AliveCount, &s.DeadCount, &s.UnknownCount)

	// Transactions over time (daily, last 30 days)
	rows6, err := r.pool.Query(ctx,
		`SELECT DATE(block_time)::text AS d, COUNT(*)
		 FROM transactions
		 WHERE block_time >= NOW() - INTERVAL '30 days'
		 GROUP BY d ORDER BY d`)
	if err == nil {
		defer rows6.Close()
		for rows6.Next() {
			var dc DateCount
			if err := rows6.Scan(&dc.Date, &dc.Count); err == nil {
				s.TransactionsOverTime = append(s.TransactionsOverTime, dc)
			}
		}
	}

	return s, nil
}

// EndpointWithPayments holds an endpoint with its payment options.
type EndpointWithPayments struct {
	Endpoint       models.Endpoint        `json:"endpoint"`
	PaymentOptions []models.PaymentOption  `json:"payment_options"`
}

// GetEndpointsWithPayments returns a paginated list of endpoints with payment options.
func (r *Repository) GetEndpointsWithPayments(ctx context.Context, limit, offset int) ([]EndpointWithPayments, error) {
	endpoints, err := r.GetEndpoints(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	if len(endpoints) == 0 {
		return nil, nil
	}

	// Collect endpoint IDs
	ids := make([]uuid.UUID, len(endpoints))
	for i, e := range endpoints {
		ids[i] = e.ID
	}

	// Batch-fetch payment options
	rows, err := r.pool.Query(ctx,
		`SELECT id, endpoint_id, scheme, network_raw, network_normalized,
		        asset_address, asset_name, max_amount_raw, price_usd,
		        pay_to, max_timeout_seconds, mime_type, description, output_schema_raw
		 FROM payment_options
		 WHERE endpoint_id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("get payment options batch: %w", err)
	}
	defer rows.Close()

	// Group by endpoint ID
	poMap := make(map[uuid.UUID][]models.PaymentOption)
	for rows.Next() {
		var po models.PaymentOption
		err := rows.Scan(
			&po.ID, &po.EndpointID, &po.Scheme, &po.NetworkRaw, &po.NetworkNormalized,
			&po.AssetAddress, &po.AssetName, &po.MaxAmountRaw, &po.PriceUSD,
			&po.PayTo, &po.MaxTimeoutSeconds, &po.MimeType, &po.Description, &po.OutputSchemaRaw,
		)
		if err != nil {
			return nil, fmt.Errorf("scan payment option: %w", err)
		}
		poMap[po.EndpointID] = append(poMap[po.EndpointID], po)
	}

	result := make([]EndpointWithPayments, len(endpoints))
	for i, e := range endpoints {
		result[i] = EndpointWithPayments{
			Endpoint:       e,
			PaymentOptions: poMap[e.ID],
		}
	}
	return result, nil
}

// GetBaseFacilitators returns all facilitators on the Base chain.
func (r *Repository) GetBaseFacilitators(ctx context.Context) ([]models.Facilitator, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, chain, address, last_synced_at, created_at
		 FROM facilitators WHERE chain = 'base'
		 ORDER BY name, address`)
	if err != nil {
		return nil, fmt.Errorf("get base facilitators: %w", err)
	}
	defer rows.Close()

	var out []models.Facilitator
	for rows.Next() {
		var f models.Facilitator
		if err := rows.Scan(&f.ID, &f.Name, &f.Chain, &f.Address, &f.LastSyncedAt, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan facilitator: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetBaseFacilitatorsMissingTransactions returns facilitators on Base that have
// zero matching rows in the transactions table. Useful for resuming a partial
// import without re-hitting CDP for addresses we already indexed.
func (r *Repository) GetBaseFacilitatorsMissingTransactions(ctx context.Context) ([]models.Facilitator, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT f.id, f.name, f.chain, f.address, f.last_synced_at, f.created_at
		FROM facilitators f
		LEFT JOIN transactions t
		  ON lower(t.facilitator_address) = lower(f.address)
		WHERE f.chain = 'base'
		GROUP BY f.id, f.name, f.chain, f.address, f.last_synced_at, f.created_at
		HAVING COUNT(t.id) = 0
		ORDER BY f.name, f.address`)
	if err != nil {
		return nil, fmt.Errorf("get missing facilitators: %w", err)
	}
	defer rows.Close()

	var out []models.Facilitator
	for rows.Next() {
		var f models.Facilitator
		if err := rows.Scan(&f.ID, &f.Name, &f.Chain, &f.Address, &f.LastSyncedAt, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan facilitator: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FacilitatorStats holds a facilitator with aggregated transaction stats.
type FacilitatorStats struct {
	models.Facilitator
	TxCount      int     `json:"tx_count"`
	TotalVolume  float64 `json:"total_volume_usd"`
	UniquePayers int     `json:"unique_payers"`
}

// GetFacilitatorStats returns all facilitators with their transaction stats.
func (r *Repository) GetFacilitatorStats(ctx context.Context) ([]FacilitatorStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT f.id, f.name, f.chain, f.address, f.last_synced_at, f.created_at,
		       COALESCE(s.tx_count, 0),
		       COALESCE(s.total_volume, 0),
		       COALESCE(s.unique_payers, 0)
		FROM facilitators f
		LEFT JOIN (
			SELECT facilitator_address,
			       COUNT(*)::int AS tx_count,
			       SUM(amount_usd) AS total_volume,
			       COUNT(DISTINCT payer_address)::int AS unique_payers
			FROM transactions
			GROUP BY facilitator_address
		) s ON lower(f.address) = lower(s.facilitator_address)
		WHERE f.chain = 'base'
		ORDER BY COALESCE(s.tx_count, 0) DESC, f.name`)
	if err != nil {
		return nil, fmt.Errorf("get facilitator stats: %w", err)
	}
	defer rows.Close()

	var out []FacilitatorStats
	for rows.Next() {
		var fs FacilitatorStats
		if err := rows.Scan(&fs.ID, &fs.Name, &fs.Chain, &fs.Address,
			&fs.LastSyncedAt, &fs.CreatedAt,
			&fs.TxCount, &fs.TotalVolume, &fs.UniquePayers); err != nil {
			return nil, fmt.Errorf("scan facilitator stats: %w", err)
		}
		out = append(out, fs)
	}
	return out, rows.Err()
}

// TransactionWithFacilitator holds a transaction with its facilitator name.
type TransactionWithFacilitator struct {
	models.Transaction
	FacilitatorName string `json:"facilitator_name"`
}

// GetTransactions returns a paginated list of transactions with facilitator names.
func (r *Repository) GetTransactions(ctx context.Context, limit, offset int, facilitatorFilter string) ([]TransactionWithFacilitator, int, error) {
	// Count total
	countQuery := `SELECT COUNT(*) FROM transactions t`
	countArgs := []any{}
	if facilitatorFilter != "" {
		countQuery += ` JOIN facilitators f ON lower(t.facilitator_address) = lower(f.address) AND f.chain = 'base' WHERE lower(f.name) = lower($1)`
		countArgs = append(countArgs, facilitatorFilter)
	}
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	// Fetch page
	query := `
		SELECT t.id, t.tx_hash, t.block_number, t.block_time, t.event_type,
		       t.proxy_contract, t.facilitator_address, t.payer_address,
		       t.recipient_address, t.amount_raw, t.amount_usd, t.asset_address,
		       t.indexed_at, COALESCE(f.name, 'Unknown')
		FROM transactions t
		LEFT JOIN facilitators f ON lower(t.facilitator_address) = lower(f.address) AND f.chain = 'base'`
	args := []any{}
	argIdx := 1

	if facilitatorFilter != "" {
		query += fmt.Sprintf(` WHERE lower(f.name) = lower($%d)`, argIdx)
		args = append(args, facilitatorFilter)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY t.block_time DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("get transactions: %w", err)
	}
	defer rows.Close()

	var out []TransactionWithFacilitator
	for rows.Next() {
		var tw TransactionWithFacilitator
		if err := rows.Scan(&tw.ID, &tw.TxHash, &tw.BlockNumber, &tw.BlockTime,
			&tw.EventType, &tw.ProxyContract, &tw.FacilitatorAddress,
			&tw.PayerAddress, &tw.RecipientAddress, &tw.AmountRaw,
			&tw.AmountUSD, &tw.AssetAddress, &tw.IndexedAt,
			&tw.FacilitatorName); err != nil {
			return nil, 0, fmt.Errorf("scan transaction: %w", err)
		}
		out = append(out, tw)
	}
	return out, total, rows.Err()
}

// UpdateFacilitatorSyncTime sets last_synced_at for a facilitator.
func (r *Repository) UpdateFacilitatorSyncTime(ctx context.Context, id uuid.UUID, syncedAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE facilitators SET last_synced_at = $2 WHERE id = $1`,
		id, syncedAt)
	if err != nil {
		return fmt.Errorf("update facilitator sync time: %w", err)
	}
	return nil
}

// GetMaxBlockTimeForFacilitator returns the most recent block_time stored for a
// given facilitator address (case-insensitive). Returns nil if no transactions
// are stored for that address — the caller should fall back to a default start.
func (r *Repository) GetMaxBlockTimeForFacilitator(ctx context.Context, facilitatorAddress string) (*time.Time, error) {
	var t *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT MAX(block_time) FROM transactions
		 WHERE lower(facilitator_address) = lower($1)`,
		facilitatorAddress).Scan(&t)
	if err != nil {
		return nil, fmt.Errorf("max block_time for facilitator: %w", err)
	}
	return t, nil
}

// GetTotalEndpointCount returns the total number of endpoints.
func (r *Repository) GetTotalEndpointCount(ctx context.Context) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM endpoints`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count endpoints: %w", err)
	}
	return n, nil
}

// ProbeTarget is returned by GetEndpointsForProbing.
// Import cycle avoidance: we define it here rather than in internal/prober.
type ProbeTarget struct {
	EndpointID     uuid.UUID
	ResourceURL    string
	HTTPMethod     string
	Domain         string
	PaymentOptions []PaymentRef
}

// PaymentRef is a lightweight snapshot of a payment option for probe comparison.
type PaymentRef struct {
	PayTo     string
	AmountRaw string
	Network   string
	Asset     string
	PriceUSD  float64
}

// GetEndpointsForProbing returns a batch of endpoints with their payment options.
// Pagination is by endpoint, not by row.
func (r *Repository) GetEndpointsForProbing(ctx context.Context, limit, offset int) ([]ProbeTarget, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.resource_url, e.http_method, e.domain,
		       po.pay_to, po.max_amount_raw, po.network_normalized, po.asset_address, po.price_usd
		FROM (SELECT id, resource_url, http_method, domain FROM endpoints ORDER BY id LIMIT $1 OFFSET $2) e
		LEFT JOIN payment_options po ON po.endpoint_id = e.id
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get endpoints for probing: %w", err)
	}
	defer rows.Close()

	// Collect rows, grouping payment options per endpoint.
	type row struct {
		EndpointID  uuid.UUID
		ResourceURL string
		HTTPMethod  string
		Domain      string
		PayTo       *string
		AmountRaw   *string
		Network     *string
		Asset       *string
		PriceUSD    *float64
	}

	// Use a map to deduplicate endpoints while collecting payment options.
	orderMap := make(map[uuid.UUID]int)
	var order []uuid.UUID
	targets := make(map[uuid.UUID]*ProbeTarget)

	for rows.Next() {
		var rw row
		if err := rows.Scan(
			&rw.EndpointID, &rw.ResourceURL, &rw.HTTPMethod, &rw.Domain,
			&rw.PayTo, &rw.AmountRaw, &rw.Network, &rw.Asset, &rw.PriceUSD,
		); err != nil {
			return nil, fmt.Errorf("scan probe target: %w", err)
		}

		if _, seen := orderMap[rw.EndpointID]; !seen {
			orderMap[rw.EndpointID] = len(order)
			order = append(order, rw.EndpointID)
			targets[rw.EndpointID] = &ProbeTarget{
				EndpointID:  rw.EndpointID,
				ResourceURL: rw.ResourceURL,
				HTTPMethod:  rw.HTTPMethod,
				Domain:      rw.Domain,
			}
		}

		if rw.PayTo != nil {
			ref := PaymentRef{PayTo: *rw.PayTo}
			if rw.AmountRaw != nil {
				ref.AmountRaw = *rw.AmountRaw
			}
			if rw.Network != nil {
				ref.Network = *rw.Network
			}
			if rw.Asset != nil {
				ref.Asset = *rw.Asset
			}
			if rw.PriceUSD != nil {
				ref.PriceUSD = *rw.PriceUSD
			}
			targets[rw.EndpointID].PaymentOptions = append(targets[rw.EndpointID].PaymentOptions, ref)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]ProbeTarget, len(order))
	for i, id := range order {
		result[i] = *targets[id]
	}
	return result, nil
}

// InsertProbeResults batch-inserts probe results, ignoring duplicates.
func (r *Repository) InsertProbeResults(ctx context.Context, results []models.ProbeResult) (int, error) {
	if len(results) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, pr := range results {
		batch.Queue(`
			INSERT INTO probe_results (
				id, endpoint_id, probed_at, status_code, latency_ms,
				health_status, is_valid_402, error_message,
				response_pay_to, response_amount_raw, response_network, response_asset,
				price_match, discrepancy_details
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			pr.ID, pr.EndpointID, pr.ProbedAt, pr.StatusCode, pr.LatencyMs,
			pr.HealthStatus, pr.IsValid402, nullStr(pr.ErrorMessage),
			nullStr(pr.ResponsePayTo), nullStr(pr.ResponseAmountRaw),
			nullStr(pr.ResponseNetwork), nullStr(pr.ResponseAsset),
			pr.PriceMatch, nullJSON(pr.DiscrepancyDetails),
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	inserted := 0
	for range results {
		ct, err := br.Exec()
		if err != nil {
			br.Close()
			return inserted, fmt.Errorf("insert probe result: %w", err)
		}
		if ct.RowsAffected() > 0 {
			inserted++
		}
	}
	br.Close()
	return inserted, nil
}

// ApplyProbeUpdates writes drifted payment details back to the payment_options
// table. For each probe result where is_valid_402 is true, price_match is
// false, and the live response has a pay_to + amount + network + asset, it
// updates the matching payment_options row(s) in place. Scales price_usd by the
// amount ratio so USD-denominated price stays consistent.
//
// Matching is case-insensitive on network_normalized + asset_address. If the
// endpoint has multiple payment options matching those keys, they all get the
// same update — which is the correct behaviour, since they represent the same
// payment route.
//
// Returns the number of payment_options rows updated.
func (r *Repository) ApplyProbeUpdates(ctx context.Context, results []models.ProbeResult) (int, error) {
	if len(results) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	queued := 0
	for _, pr := range results {
		if !pr.IsValid402 || pr.PriceMatch {
			continue
		}
		if pr.ResponsePayTo == "" || pr.ResponseAmountRaw == "" ||
			pr.ResponseNetwork == "" || pr.ResponseAsset == "" {
			continue
		}
		batch.Queue(`
			UPDATE payment_options
			SET pay_to = $2,
			    max_amount_raw = $3,
			    price_usd = CASE
			      WHEN max_amount_raw ~ '^[0-9]+$'
			           AND max_amount_raw::numeric > 0
			           AND $3 ~ '^[0-9]+$'
			      THEN price_usd * ($3::numeric / max_amount_raw::numeric)
			      ELSE price_usd
			    END
			WHERE endpoint_id = $1
			  AND lower(network_normalized) = lower($4)
			  AND lower(asset_address) = lower($5)`,
			pr.EndpointID, pr.ResponsePayTo, pr.ResponseAmountRaw,
			pr.ResponseNetwork, pr.ResponseAsset,
		)
		queued++
	}
	if queued == 0 {
		return 0, nil
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	updated := 0
	for i := 0; i < queued; i++ {
		ct, err := br.Exec()
		if err != nil {
			return updated, fmt.Errorf("apply probe update: %w", err)
		}
		updated += int(ct.RowsAffected())
	}
	return updated, nil
}

// GetProbeHistory returns the last N probe results for an endpoint.
func (r *Repository) GetProbeHistory(ctx context.Context, endpointID uuid.UUID, limit int) ([]models.ProbeResult, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, endpoint_id, probed_at, status_code, latency_ms,
		       health_status, is_valid_402,
		       COALESCE(error_message, ''),
		       COALESCE(response_pay_to, ''), COALESCE(response_amount_raw, ''),
		       COALESCE(response_network, ''), COALESCE(response_asset, ''),
		       price_match, discrepancy_details
		FROM probe_results
		WHERE endpoint_id = $1
		ORDER BY probed_at DESC
		LIMIT $2
	`, endpointID, limit)
	if err != nil {
		return nil, fmt.Errorf("get probe history: %w", err)
	}
	defer rows.Close()

	var out []models.ProbeResult
	for rows.Next() {
		var pr models.ProbeResult
		if err := rows.Scan(
			&pr.ID, &pr.EndpointID, &pr.ProbedAt, &pr.StatusCode, &pr.LatencyMs,
			&pr.HealthStatus, &pr.IsValid402, &pr.ErrorMessage,
			&pr.ResponsePayTo, &pr.ResponseAmountRaw, &pr.ResponseNetwork, &pr.ResponseAsset,
			&pr.PriceMatch, &pr.DiscrepancyDetails,
		); err != nil {
			return nil, fmt.Errorf("scan probe result: %w", err)
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}

// nullStr returns nil for empty strings so the DB stores NULL.
func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nullJSON returns nil for empty/null JSON so the DB stores NULL.
func nullJSON(b []byte) interface{} {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	return b
}
