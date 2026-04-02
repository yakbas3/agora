CREATE TABLE probe_results (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    endpoint_id         UUID NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
    probed_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status_code         INTEGER,
    latency_ms          INTEGER,
    health_status       TEXT NOT NULL DEFAULT 'unknown'
                        CHECK (health_status IN ('alive', 'dead', 'unknown')),
    is_valid_402        BOOLEAN NOT NULL DEFAULT FALSE,
    error_message       TEXT,
    response_pay_to     TEXT,
    response_amount_raw TEXT,
    response_network    TEXT,
    response_asset      TEXT,
    price_match         BOOLEAN NOT NULL DEFAULT FALSE,
    discrepancy_details JSONB
);

CREATE INDEX idx_probe_results_endpoint    ON probe_results (endpoint_id);
CREATE INDEX idx_probe_results_probed_at   ON probe_results (probed_at DESC);
CREATE INDEX idx_probe_results_ep_latest   ON probe_results (endpoint_id, probed_at DESC);
