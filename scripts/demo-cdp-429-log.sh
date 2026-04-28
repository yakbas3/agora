#!/usr/bin/env bash
# demo-cdp-429-log.sh
#
# Captures real CDP 429 backoff output for Figure 11 of the final report
# WITHOUT touching the production `agora` database.
#
# Strategy:
#   1. Create a temporary database `agora_cdpdemo` inside the same Postgres
#      container.
#   2. Run AGORA's migrations into it (this seeds the 102 facilitator
#      addresses from migration 000010 but no transactions).
#   3. Run `./agora sync Coinbase` against the demo DB. Because the demo DB
#      starts with an empty `transactions` table and no per-facilitator
#      cursor, the sync will try to pull Coinbase's full history from CDP
#      from scratch. Coinbase has the heaviest address in the registry, so
#      this reliably fires enough CDP queries in succession to trip CDP's
#      rate limiter, producing the "CDP 429, retry in Xms (attempt N/M)"
#      log lines from internal/cdp/client.go:160.
#   4. Save the captured output to logs/sync-429-demo.log.
#   5. Drop the temporary database.
#
# Usage:
#   ./scripts/demo-cdp-429-log.sh
#
# Output:
#   logs/sync-429-demo.log
#
# Notes:
#   - This script makes real CDP API calls (read-only SQL queries against
#     base.events). It does NOT cost money but it does consume your CDP
#     rate-limit budget for the duration of the sync.
#   - You can ctrl-C at any time once you see a few "CDP 429, retry in Xms"
#     lines — the cleanup trap drops the demo DB.

set -euo pipefail
cd "$(dirname "$0")/.."

DEMO_DB="agora_cdpdemo"
PG_CONTAINER="agora-postgres-1"
LOG_FILE="logs/sync-429-demo.log"
FACILITATOR_FILTER="Coinbase"

# Sanity check: refuse to run if the prod container isn't up
if ! docker ps --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
  echo "ERROR: $PG_CONTAINER is not running. Bring up docker-compose.prod.yml first." >&2
  exit 1
fi

# Sanity check: CDP credentials must be present in .env
if ! grep -qE '^CDP_API_KEY_ID=' .env || ! grep -qE '^CDP_API_KEY_SECRET=' .env; then
  echo "ERROR: CDP_API_KEY_ID and CDP_API_KEY_SECRET must be set in .env." >&2
  exit 1
fi

mkdir -p logs

cleanup() {
  echo
  echo "Cleaning up demo database..."
  docker exec "$PG_CONTAINER" psql -U agora -d postgres -c "DROP DATABASE IF EXISTS $DEMO_DB;" >/dev/null 2>&1 || true
  echo "Done."
}
trap cleanup EXIT

echo "=== Demo CDP-429 sync run (isolated DB) ==="
echo "Production DB '$PG_CONTAINER:agora' will NOT be modified."
echo "Watching for: 'CDP 429, retry in Xms (attempt N/M)'"
echo

# 1. Create the temporary DB
echo "[1/3] Creating temporary database $DEMO_DB..."
docker exec "$PG_CONTAINER" psql -U agora -d postgres -c "DROP DATABASE IF EXISTS $DEMO_DB;" >/dev/null
docker exec "$PG_CONTAINER" psql -U agora -d postgres -c "CREATE DATABASE $DEMO_DB OWNER agora;" >/dev/null

# 2. Run migrations only (no seed). Migration 000010 inserts the 102 facilitator
#    address rows we need for the sync to have something to iterate over. No
#    transactions exist yet, so the sync will start from MAX(block_time)=NULL.
echo "[2/3] Running migrations into $DEMO_DB (no seed, just facilitators)..."
DATABASE_URL="postgres://agora:agora@localhost:5433/${DEMO_DB}?sslmode=disable" \
  ./agora.exe migrate

# 3. Run the filtered sync. Cursor is empty so the sync will scan from the
#    project's default start date for every Coinbase facilitator address,
#    which fires enough back-to-back CDP queries to trip the rate limiter.
echo "[3/3] Running sync against demo DB (filter: '$FACILITATOR_FILTER')..."
echo "      Output -> $LOG_FILE"
echo "      Press Ctrl-C once you see a few '429' lines for Figure 11."
echo

DATABASE_URL="postgres://agora:agora@localhost:5433/${DEMO_DB}?sslmode=disable" \
  ./agora.exe sync "$FACILITATOR_FILTER" 2>&1 | tee "$LOG_FILE"

echo
echo "Demo complete."
echo "Real CDP 429 log lines saved to $LOG_FILE — screenshot for Figure 11."
