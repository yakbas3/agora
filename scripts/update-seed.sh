#!/bin/bash
# update-seed.sh — Regenerate data/seed.sql.gz from the running database.
#
# Run this after:
#   ./agora.exe migrate
#   ./agora.exe probe      (takes ~15-30 minutes)
#
# Usage: bash scripts/update-seed.sh

set -euo pipefail

CONTAINER=$(docker ps -q --filter ancestor=pgvector/pgvector:pg16)

if [ -z "$CONTAINER" ]; then
  echo "ERROR: No running pgvector container found."
  echo "Start the database first: docker compose up -d"
  exit 1
fi

echo "Dumping database from container $CONTAINER..."
docker exec "$CONTAINER" pg_dump -U agora agora | gzip > data/seed.sql.gz

echo "Verifying seed..."
PROBE_COUNT=$(zcat data/seed.sql.gz | grep -c "INSERT INTO probe_results" || echo "0")
echo "  Probe result inserts in seed: $PROBE_COUNT"

SIZE=$(du -sh data/seed.sql.gz | cut -f1)
echo "  Seed size: $SIZE"

echo ""
echo "Done. Commit with:"
echo "  git add data/seed.sql.gz"
echo "  git commit -m \"chore: update seed with probe results\""
