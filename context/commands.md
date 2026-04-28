## Commands

### Infrastructure

```bash
# Start PostgreSQL with pgvector (port 5433, NOT 5432 — avoids local PG conflicts)
docker compose up -d

# Full production stack (postgres + embed sidecar + Go API + Next.js web)
docker compose -f docker-compose.prod.yml up -d

# Tear down containers
docker compose down
```

### Building

```bash
# Build the Go binary
go build -o agora.exe ./cmd/agora
```

### CLI Subcommands

```bash
# Run database migrations (creates/updates tables, materialized views)
./agora.exe migrate

# Crawl the Bazaar discovery API → upserts endpoints + payment options
./agora.exe crawl

# Sync V1 transactions from Coinbase CDP SQL API (by facilitator address)
./agora.exe sync

# Index V2 on-chain transactions via Alchemy RPC (Settled/SettledWithPermit events)
./agora.exe index

# Start the REST API server (default :8080)
./agora.exe serve
```

### Tests

```bash
# Run all tests across the project
go test ./...

# Run tests for a single package
go test ./internal/crawler/...
go test ./internal/config/...
go test ./internal/sync/...

# Run a specific test by name
go test ./internal/crawler/... -run TestNormalizeNetwork
```

### Web Frontend (Next.js)

```bash
cd web && npm install       # Install dependencies
cd web && npm run dev       # Dev server on :3000
cd web && npm run build     # Production build
cd web && npm run lint      # ESLint check
```

### Embed Sidecar (Python)

```bash
cd embed && pip install -r requirements.txt       # Install dependencies
cd embed && uvicorn server:app --port 8100         # Start sidecar on :8100
```

### Database Access

```bash
# Connect to PostgreSQL inside Docker and run a query
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) \
  psql -U agora -d agora -c "SELECT count(*) FROM endpoints;"

# Interactive psql session
docker exec -it $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) \
  psql -U agora -d agora
```

### Environment Setup

```bash
# Copy example env (required before running any CLI command)
cp .env.example .env
# Then edit .env with your CDP API key and any local overrides
```
