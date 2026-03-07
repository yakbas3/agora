# Production Docker Compose Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a production Docker Compose setup that starts all 4 services (Postgres, Go API, Python embed sidecar, Next.js frontend) with pre-loaded data in one command.

**Architecture:** Each service gets its own Dockerfile with multi-stage builds where applicable. A gzipped pg_dump seeds the database on first start. The Go container runs migrations before serving. Services communicate via Docker DNS on an internal network.

**Tech Stack:** Docker, Docker Compose, Go 1.24, Python 3.12, Node 22, pgvector/pg16

---

### Task 1: Export database seed dump

**Files:**
- Create: `data/seed.sql.gz`

**Step 1: Export the database with embeddings**

Run:
```bash
docker exec $(docker ps -q --filter ancestor=pgvector/pgvector:pg16) pg_dump -U agora --no-owner --no-privileges agora | gzip > data/seed.sql.gz
```

Create the `data/` directory first if needed:
```bash
mkdir -p data
```

**Step 2: Verify the dump is valid**

Run:
```bash
ls -lh data/seed.sql.gz
```
Expected: File exists, size should be a few MB (varies depending on whether embeddings are populated).

**Step 3: Verify dump contents**

Run:
```bash
gunzip -c data/seed.sql.gz | head -50
```
Expected: Should show SQL statements starting with `SET`, `CREATE TABLE`, etc.

**Step 4: Add data/ to .gitignore check**

The seed dump should be committed (it's the seed data for TAs). Do NOT gitignore it. But do add a note:

Run:
```bash
echo "# data/seed.sql.gz is intentionally committed — it seeds the production database" > data/README.md
```

**Step 5: Commit**

```bash
git add data/
git commit -m "data: add database seed dump with endpoints and embeddings"
```

---

### Task 2: Create .dockerignore

**Files:**
- Create: `.dockerignore`

**Step 1: Create root .dockerignore**

This is used by all three Dockerfiles. Exclude things that shouldn't go into build contexts.

```
.git
.env
.env.*
*.log
crawl.log
node_modules
web/node_modules
web/.next
data/
docs/
eda/
.interface-design/
embed/__pycache__
tmp_*
```

**Step 2: Commit**

```bash
git add .dockerignore
git commit -m "chore: add .dockerignore for production builds"
```

---

### Task 3: Enable Next.js standalone output

**Files:**
- Modify: `web/next.config.ts`

**Step 1: Add output: "standalone" to next config**

The standalone output mode creates a self-contained production build that doesn't need `node_modules`. Required for the Docker multi-stage build.

Change `web/next.config.ts` to:

```typescript
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
};

export default nextConfig;
```

**Step 2: Verify build still works**

Run: `cd c:/Users/yaman/Desktop/agora/web && npm run build`
Expected: Build succeeds. Should now produce `.next/standalone/` directory.

**Step 3: Verify standalone output exists**

Run: `ls web/.next/standalone/server.js`
Expected: File exists.

**Step 4: Commit**

```bash
git add web/next.config.ts
git commit -m "chore(web): enable standalone output for Docker builds"
```

---

### Task 4: Create Dockerfile.api (Go)

**Files:**
- Create: `Dockerfile.api`
- Create: `entrypoint.sh`

**Step 1: Create the Go multi-stage Dockerfile**

```dockerfile
FROM golang:1.24 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o agora ./cmd/agora

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/agora .
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
```

**Step 2: Create the entrypoint script**

```bash
#!/bin/sh
set -e

echo "Running migrations..."
./agora migrate

echo "Starting API server..."
exec ./agora serve
```

**Step 3: Verify Dockerfile builds**

Run: `cd c:/Users/yaman/Desktop/agora && docker build -f Dockerfile.api -t agora-api .`
Expected: Build succeeds. Final image should be small (~30MB).

**Step 4: Commit**

```bash
git add Dockerfile.api entrypoint.sh
git commit -m "feat(docker): add Go API Dockerfile with migrate-then-serve entrypoint"
```

---

### Task 5: Create Dockerfile.embed (Python)

**Files:**
- Create: `Dockerfile.embed`

**Step 1: Create the Python Dockerfile**

```dockerfile
FROM python:3.12-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

COPY embed/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY embed/server.py .

EXPOSE 8100
CMD ["uvicorn", "server:app", "--host", "0.0.0.0", "--port", "8100"]
```

**Step 2: Verify Dockerfile builds**

Run: `cd c:/Users/yaman/Desktop/agora && docker build -f Dockerfile.embed -t agora-embed .`
Expected: Build succeeds. Image will be ~1.5GB due to PyTorch (sentence-transformers dependency).

**Step 3: Commit**

```bash
git add Dockerfile.embed
git commit -m "feat(docker): add Python embedding sidecar Dockerfile"
```

---

### Task 6: Create Dockerfile.web (Next.js)

**Files:**
- Create: `Dockerfile.web`

**Step 1: Create the Next.js multi-stage Dockerfile**

```dockerfile
FROM node:22-alpine AS deps
WORKDIR /app
COPY web/package.json web/package-lock.json ./
RUN npm ci

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY web/ .
ENV NEXT_PUBLIC_API_URL=http://localhost:8080
RUN npm run build

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
ENV PORT=3000

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

EXPOSE 3000
CMD ["node", "server.js"]
```

**Step 2: Verify Dockerfile builds**

Run: `cd c:/Users/yaman/Desktop/agora && docker build -f Dockerfile.web -t agora-web .`
Expected: Build succeeds. Final image should be ~150MB.

**Step 3: Commit**

```bash
git add Dockerfile.web
git commit -m "feat(docker): add Next.js frontend Dockerfile with standalone output"
```

---

### Task 7: Create docker-compose.prod.yml

**Files:**
- Create: `docker-compose.prod.yml`

**Step 1: Create the production compose file**

```yaml
services:
  postgres:
    image: pgvector/pgvector:pg16
    ports:
      - "5433:5432"
    environment:
      POSTGRES_USER: agora
      POSTGRES_PASSWORD: agora
      POSTGRES_DB: agora
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./data/seed.sql.gz:/docker-entrypoint-initdb.d/seed.sql.gz:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U agora"]
      interval: 5s
      timeout: 5s
      retries: 10

  embed:
    build:
      context: .
      dockerfile: Dockerfile.embed
    ports:
      - "8100:8100"
    volumes:
      - model-cache:/root/.cache/huggingface
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8100/health"]
      interval: 10s
      timeout: 10s
      retries: 12
      start_period: 60s

  api:
    build:
      context: .
      dockerfile: Dockerfile.api
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://agora:agora@postgres:5432/agora?sslmode=disable
      EMBED_URL: http://embed:8100
      API_PORT: "8080"
    depends_on:
      postgres:
        condition: service_healthy
      embed:
        condition: service_healthy

  web:
    build:
      context: .
      dockerfile: Dockerfile.web
    ports:
      - "3000:3000"
    depends_on:
      - api

volumes:
  pgdata:
  model-cache:
```

**Step 2: Commit**

```bash
git add docker-compose.prod.yml
git commit -m "feat(docker): add production compose with all 4 services"
```

---

### Task 8: Full stack smoke test

**Step 1: Stop any running local services**

Kill any local agora, node, or python processes that might conflict on ports 8080, 8100, 3000, 5433.

**Step 2: Start the full stack**

Run:
```bash
cd c:/Users/yaman/Desktop/agora && docker compose -f docker-compose.prod.yml up --build
```

Watch the logs. Expected startup sequence:
1. Postgres starts, loads seed data from `seed.sql.gz`
2. Embed sidecar starts, downloads model (first time only)
3. Once postgres + embed are healthy, API container starts, runs migrations, starts serving
4. Web container starts

**Step 3: Test API endpoints**

Run (in another terminal):
```bash
curl -s http://localhost:8080/api/stats | head -100
```
Expected: JSON with `total_endpoints` > 12000

```bash
curl -s "http://localhost:8080/api/endpoints?limit=1" | head -100
```
Expected: JSON array with endpoint + payment_options

**Step 4: Test web UI**

Open `http://localhost:3000` in a browser.
Expected: Endpoints page loads with real data from the database.

Open `http://localhost:3000/network`.
Expected: Network stats page with charts showing real aggregation data.

**Step 5: Test semantic search**

Type a query in the search bar on the endpoints page.
Expected: Returns semantically relevant results (requires embeddings in seed dump).

**Step 6: Tear down**

Run:
```bash
docker compose -f docker-compose.prod.yml down
```

**Step 7: If any step fails, fix before proceeding**

Common issues:
- Port conflicts: change the host port mapping or kill conflicting processes
- Seed not loading: verify `data/seed.sql.gz` is valid SQL
- Model download timeout: increase embed health check `start_period`
- Next.js standalone missing: verify `output: "standalone"` in next.config.ts

---

### Task 9: Commit final state and verify clean build

**Step 1: Tear down with volumes to test fresh start**

Run:
```bash
docker compose -f docker-compose.prod.yml down -v
```

**Step 2: Start fresh**

Run:
```bash
docker compose -f docker-compose.prod.yml up --build
```

Expected: Everything starts from scratch — Postgres re-seeds, model re-downloads (cached in volume only if volume wasn't removed, but we removed it with `-v`), all services come up healthy.

**Step 3: Verify endpoints and search work**

Same checks as Task 8 Steps 3-5.

**Step 4: Tear down**

```bash
docker compose -f docker-compose.prod.yml down
```
