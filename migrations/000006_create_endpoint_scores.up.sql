CREATE MATERIALIZED VIEW endpoint_scores AS
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
GROUP BY e.id;

CREATE UNIQUE INDEX idx_endpoint_scores_id ON endpoint_scores (endpoint_id);
