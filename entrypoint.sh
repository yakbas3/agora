#!/bin/sh
set -e

echo "Running migrations..."
./agora migrate

echo "Starting API server..."
exec ./agora serve
