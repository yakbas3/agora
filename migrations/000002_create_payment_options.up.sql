CREATE TABLE payment_options (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    endpoint_id         UUID NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
    scheme              TEXT NOT NULL DEFAULT 'exact',
    network_raw         TEXT NOT NULL,
    network_normalized  TEXT NOT NULL,
    asset_address       TEXT NOT NULL,
    asset_name          TEXT NOT NULL DEFAULT '',
    max_amount_raw      TEXT NOT NULL,
    price_usd           NUMERIC(20, 10) NOT NULL DEFAULT 0,
    pay_to              TEXT NOT NULL,
    max_timeout_seconds INTEGER NOT NULL DEFAULT 300,
    mime_type           TEXT NOT NULL DEFAULT '',
    description         TEXT NOT NULL DEFAULT '',
    output_schema_raw   JSONB
);

CREATE INDEX idx_payment_options_endpoint_id ON payment_options (endpoint_id);
CREATE INDEX idx_payment_options_network ON payment_options (network_normalized);
CREATE INDEX idx_payment_options_price ON payment_options (price_usd);
