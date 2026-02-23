CREATE TABLE indexer_state (
    id          INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    last_block  BIGINT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO indexer_state (id, last_block) VALUES (1, 0);
