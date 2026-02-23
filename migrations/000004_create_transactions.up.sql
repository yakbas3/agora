CREATE TABLE transactions (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tx_hash             TEXT NOT NULL UNIQUE,
    block_number        BIGINT NOT NULL,
    block_time          TIMESTAMPTZ NOT NULL,
    event_type          TEXT NOT NULL,
    proxy_contract      TEXT,
    facilitator_address TEXT NOT NULL,
    payer_address       TEXT NOT NULL,
    recipient_address   TEXT NOT NULL,
    amount_raw          TEXT NOT NULL,
    amount_usd          NUMERIC(20, 10) NOT NULL DEFAULT 0,
    asset_address       TEXT NOT NULL,
    indexed_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_recipient ON transactions (recipient_address);
CREATE INDEX idx_transactions_block ON transactions (block_number);
CREATE INDEX idx_transactions_facilitator ON transactions (facilitator_address);
CREATE INDEX idx_transactions_block_time ON transactions (block_time);
