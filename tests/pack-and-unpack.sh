#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

cleanup() {
    echo -e "\n${BLUE}--- Cleanup ---${NC}"
    cd "$SCRIPT_DIR"
    docker compose down -v 2>/dev/null || true
    docker network rm tests_mico-net 2>/dev/null || true
    rm -f mico-test-*.zst mico-test-*.zst.sha256 2>/dev/null || true
}

trap cleanup EXIT

echo -e "${BLUE}=== Mico Pack/Unpack E2E Test ===${NC}"

echo -e "\n${BLUE}1. Building mico...${NC}"
cd "$PROJECT_DIR"
go build -o mico .

echo -e "\n${BLUE}2. Starting test containers...${NC}"
cd "$SCRIPT_DIR"
mkdir -p data html
echo "<h1>test</h1>" > html/index.html
docker compose up -d
sleep 3
docker compose ps

echo -e "\n${BLUE}3. Packing containers...${NC}"
ARCHIVE="mico-test-$(date +%s).zst"
"$PROJECT_DIR/mico" pack -o "$SCRIPT_DIR/$ARCHIVE"
ls -lh "$SCRIPT_DIR/$ARCHIVE"*

echo -e "\n${BLUE}4. Stopping and removing all containers and volumes...${NC}"
docker compose down -v
docker ps -a --filter "name=mico-" --format '{{.Names}}' | xargs -r docker rm -f 2>/dev/null || true
docker volume ls --filter "name=mico-" -q | xargs -r docker volume rm 2>/dev/null || true
echo "Remaining containers: $(docker ps -a --filter 'name=mico-' -q | wc -l | tr -d ' ')"
echo "Remaining volumes:   $(docker volume ls --filter 'name=mico-' -q | wc -l | tr -d ' ')"

echo -e "\n${BLUE}5. Unpacking...${NC}"
"$PROJECT_DIR/mico" unpack "$SCRIPT_DIR/$ARCHIVE"

echo -e "\n${BLUE}6. Verifying...${NC}"
sleep 2
CONTAINERS=$(docker ps -a --filter "name=mico-" --format '{{.Names}}')
if [ -z "$CONTAINERS" ]; then
    echo -e "${RED}FAIL: No containers restored${NC}"
    exit 1
fi
echo "Containers restored: $CONTAINERS"

docker compose ps 2>/dev/null || docker ps --filter "name=mico-" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo -e "\n${GREEN}PASS: Pack/unpack round-trip succeeded${NC}"
