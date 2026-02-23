CREATE TABLE crawl_runs (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    started_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at       TIMESTAMPTZ,
    total_fetched      INTEGER NOT NULL DEFAULT 0,
    new_endpoints      INTEGER NOT NULL DEFAULT 0,
    updated_endpoints  INTEGER NOT NULL DEFAULT 0,
    status             TEXT NOT NULL DEFAULT 'running',
    error              TEXT
);
