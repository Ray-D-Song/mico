#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"

echo "=== Starting test containers ==="
cd "$SCRIPT_DIR"
docker compose -f docker-compose.yml up -d

echo "=== Waiting for containers to start ==="
sleep 5

echo "=== Running containers ==="
docker ps --filter "name=mico-"

echo "=== Creating test data for bind mount ==="
mkdir -p "$SCRIPT_DIR/data"
echo "test data from host" > "$SCRIPT_DIR/data/test.txt"

mkdir -p "$SCRIPT_DIR/html"
echo "<h1>Hello from nginx</h1>" > "$SCRIPT_DIR/html/index.html"

echo "=== Building mico ==="
cd "$PROJECT_DIR"
go build -o mico .

echo "=== Running mico pack ==="
./mico pack -o "$SCRIPT_DIR/migration.zst" -c mico-web,mico-db,mico-redis -j 3

echo "=== Pack result ==="
ls -lh "$SCRIPT_DIR/migration.zst"*
sha256sum "$SCRIPT_DIR/migration.zst"

echo "=== Test completed successfully ==="

echo "=== Cleaning up containers (keeping images) ==="
cd "$SCRIPT_DIR"
docker compose -f docker-compose.yml down

echo "=== Done ==="
