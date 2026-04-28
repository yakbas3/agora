#!/usr/bin/env bash
# demo-probe-log.sh
#
# Captures real probe output for Figure 10 of the final report WITHOUT
# touching the production `agora` database.
#
# Strategy: create a temporary database `agora_probedemo` inside the same
# Postgres container, restore the project seed into it, run the probe
# against that DB only, save the output to logs/probe-demo.log, then drop
# the temporary database.
#
# Usage:
#   ./scripts/demo-probe-log.sh
#
# Output:
#   logs/probe-demo.log

set -euo pipefail
cd "$(dirname "$0")/.."

DEMO_DB="agora_probedemo"
PG_CONTAINER="agora-postgres-1"
LOG_FILE="logs/probe-demo.log"

# Sanity check: refuse to run if the prod container isn't up
if ! docker ps --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
  echo "ERROR: $PG_CONTAINER is not running. Bring up docker-compose.prod.yml first." >&2
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

echo "=== Demo probe run (isolated DB) ==="
echo "Production DB '$PG_CONTAINER:agora' will NOT be modified."
echo

# 1. Create the temporary DB
echo "[1/4] Creating temporary database $DEMO_DB..."
docker exec "$PG_CONTAINER" psql -U agora -d postgres -c "DROP DATABASE IF EXISTS $DEMO_DB;" >/dev/null
docker exec "$PG_CONTAINER" psql -U agora -d postgres -c "CREATE DATABASE $DEMO_DB OWNER agora;" >/dev/null

# 2. Restore the seed into the demo DB
echo "[2/4] Restoring seed into $DEMO_DB (this can take a minute)..."
gunzip -c data/seed.sql.gz | docker exec -i "$PG_CONTAINER" psql -U agora -d "$DEMO_DB" -q >/dev/null

# 3. Run the probe against the demo DB only — override DATABASE_URL via env.
#    godotenv.Load() respects existing environment variables, so this DOES NOT
#    fall back to the .env DATABASE_URL.
echo "[3/4] Running probe against demo DB (50 endpoints)..."
echo "      Output -> $LOG_FILE"
echo

DATABASE_URL="postgres://agora:agora@localhost:5433/${DEMO_DB}?sslmode=disable" \
  ./agora.exe probe 50 2>&1 | tee "$LOG_FILE"

echo
echo "[4/4] Demo complete."
echo "Real probe output saved to $LOG_FILE — screenshot it for Figure 10."
