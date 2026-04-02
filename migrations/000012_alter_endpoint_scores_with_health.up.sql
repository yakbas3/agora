DROP MATERIALIZED VIEW IF EXISTS endpoint_scores;

CREATE MATERIALIZED VIEW endpoint_scores AS
WITH raw AS (
    SELECT
        e.id AS endpoint_id,
        COUNT(t.id) AS tx_count,
        COALESCE(SUM(t.amount_usd), 0) AS total_volume_usd,
        COUNT(DISTINCT t.payer_address) AS unique_payers,
        MAX(t.block_time) AS last_tx_at,
        MIN(t.block_time) AS first_tx_at
    FROM endpoints e
    LEFT JOIN payment_options po ON po.endpoint_id = e.id
    LEFT JOIN transactions t ON lower(t.recipient_address) = lower(po.pay_to)
    GROUP BY e.id
),
maxes AS (
    SELECT
        MAX(tx_count) AS max_tx,
        MAX(total_volume_usd) AS max_vol,
        MAX(unique_payers) AS max_payers
    FROM raw
    WHERE tx_count > 0
),
latest_probe AS (
    SELECT DISTINCT ON (endpoint_id)
        endpoint_id,
        health_status,
        is_valid_402,
        latency_ms,
        price_match,
        probed_at
    FROM probe_results
    ORDER BY endpoint_id, probed_at DESC
),
health AS (
    SELECT
        lp.endpoint_id,
        lp.health_status,
        lp.is_valid_402,
        lp.latency_ms,
        lp.price_match,
        lp.probed_at,
        CASE
            WHEN lp.health_status = 'alive' AND lp.is_valid_402 THEN
                0.70
                + CASE WHEN lp.price_match THEN 0.15 ELSE 0.0 END
                + CASE
                    WHEN lp.latency_ms IS NULL THEN 0.05
                    WHEN lp.latency_ms < 2000   THEN 0.15
                    WHEN lp.latency_ms < 5000   THEN 0.10
                    ELSE 0.05
                  END
            WHEN lp.health_status = 'alive' AND NOT lp.is_valid_402 THEN 0.30
            ELSE 0.0
        END AS health_score
    FROM latest_probe lp
)
SELECT
    r.endpoint_id,
    r.tx_count,
    r.total_volume_usd,
    r.unique_payers,
    r.last_tx_at,
    r.first_tx_at,
    COALESCE(h.health_status, 'unknown') AS health_status,
    COALESCE(h.is_valid_402, FALSE)      AS is_valid_402,
    COALESCE(h.latency_ms, 0)            AS latency_ms,
    COALESCE(h.price_match, FALSE)       AS price_match,
    h.probed_at                          AS last_probed_at,
    COALESCE(h.health_score, 0.0)        AS health_score,
    CASE
        WHEN r.tx_count = 0 THEN 0.0
        ELSE exp(-0.03 * EXTRACT(EPOCH FROM (NOW() - r.last_tx_at)) / 86400.0)
    END AS recency_score,
    CASE
        WHEN r.tx_count > 0 AND m.max_tx IS NOT NULL THEN
            LEAST(100, ROUND((
                0.70 * (
                    0.30 * (ln(1 + r.tx_count)          / NULLIF(ln(1 + m.max_tx),   0)) +
                    0.25 * (ln(1 + r.total_volume_usd)  / NULLIF(ln(1 + m.max_vol),  0)) +
                    0.25 * (ln(1 + r.unique_payers)     / NULLIF(ln(1 + m.max_payers),0)) +
                    0.20 * exp(-0.03 * EXTRACT(EPOCH FROM (NOW() - r.last_tx_at)) / 86400.0)
                )
                + 0.30 * COALESCE(h.health_score, 0.0)
            ) * 100))
        WHEN COALESCE(h.health_score, 0.0) > 0 THEN
            ROUND(COALESCE(h.health_score, 0.0) * 50)
        ELSE 0
    END AS reliability_score
FROM raw r
CROSS JOIN maxes m
LEFT JOIN health h ON h.endpoint_id = r.endpoint_id;

CREATE UNIQUE INDEX idx_endpoint_scores_id ON endpoint_scores (endpoint_id);
