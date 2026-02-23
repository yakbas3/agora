CREATE TABLE discovered_sellers (
    pay_to              TEXT PRIMARY KEY,
    tx_count            INTEGER NOT NULL DEFAULT 0,
    total_volume_usd    NUMERIC(20, 10) NOT NULL DEFAULT 0,
    unique_payers       INTEGER NOT NULL DEFAULT 0,
    first_seen_at       TIMESTAMPTZ NOT NULL,
    last_seen_at        TIMESTAMPTZ NOT NULL,
    matched_endpoint_id UUID REFERENCES endpoints(id) ON DELETE SET NULL
);
