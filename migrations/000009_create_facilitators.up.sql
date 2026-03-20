CREATE TABLE facilitators (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            TEXT NOT NULL,
    chain           TEXT NOT NULL,
    address         TEXT NOT NULL,
    last_synced_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_facilitators_chain_address ON facilitators (chain, lower(address));
