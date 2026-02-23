CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

CREATE TABLE endpoints (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resource_url   TEXT NOT NULL UNIQUE,
    domain         TEXT NOT NULL,
    type           TEXT NOT NULL DEFAULT 'http',
    x402_version   INTEGER NOT NULL DEFAULT 1,
    description    TEXT NOT NULL DEFAULT '',
    http_method    TEXT NOT NULL DEFAULT '',
    input_schema   JSONB,
    output_schema  JSONB,
    raw_metadata   JSONB,
    last_updated   TIMESTAMPTZ NOT NULL,
    first_seen     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_crawled   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    embedding      vector(1536)
);

CREATE INDEX idx_endpoints_domain ON endpoints (domain);
CREATE INDEX idx_endpoints_http_method ON endpoints (http_method);
