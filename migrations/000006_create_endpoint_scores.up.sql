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
    LEFT JOIN transactions t ON t.recipient_address = po.pay_to
    GROUP BY e.id
),
maxes AS (
    SELECT
        MAX(tx_count) AS max_tx,
        MAX(total_volume_usd) AS max_vol,
        MAX(unique_payers) AS max_payers
    FROM raw
    WHERE tx_count > 0
)
SELECT
    r.endpoint_id,
    r.tx_count,
    r.total_volume_usd,
    r.unique_payers,
    r.last_tx_at,
    r.first_tx_at,
    CASE WHEN r.tx_count = 0 THEN 0.0
         ELSE exp(-0.03 * EXTRACT(EPOCH FROM (NOW() - r.last_tx_at)) / 86400.0)
    END AS recency_score,
    CASE WHEN r.tx_count = 0 OR m.max_tx IS NULL THEN 0
         ELSE ROUND((
             0.30 * (ln(1 + r.tx_count) / NULLIF(ln(1 + m.max_tx), 0)) +
             0.25 * (ln(1 + r.total_volume_usd) / NULLIF(ln(1 + m.max_vol), 0)) +
             0.25 * (ln(1 + r.unique_payers) / NULLIF(ln(1 + m.max_payers), 0)) +
             0.20 * exp(-0.03 * EXTRACT(EPOCH FROM (NOW() - r.last_tx_at)) / 86400.0)
         ) * 100)
    END AS reliability_score
FROM raw r
CROSS JOIN maxes m;

CREATE UNIQUE INDEX idx_endpoint_scores_id ON endpoint_scores (endpoint_id);
