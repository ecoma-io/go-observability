#!/bin/bash
set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# Config
APP_PORT=8081
PROM_PORT=9099
JAEGER_PORT=16687
METRICS_PORT=9092

echo -e "${GREEN}Starting E2E Tests...${NC}"
echo "Project root: $PROJECT_ROOT"
echo "Script directory: $SCRIPT_DIR"


# 1. Start Infrastructure and Service
echo "[1/3] Building and Starting Docker Infrastructure..."
cd "$SCRIPT_DIR"

# Set build time for Docker build args
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
export BUILD_TIME

docker compose up -d --build
echo "Waiting 5s for services to stabilize..."
sleep 5

# 2. Generate Load
echo "[2/3] Generating Load..."
# Make a few requests
for _ in {1..20}; do
   curl -s "http://localhost:$APP_PORT/ping" > /dev/null || echo "Request failed"
   sleep 0.1
done

# 3. Verify Observability
echo "[3/4] Verifying Observability..."
echo "Waiting 5s for traces to flush..."
sleep 5

# Check service logs for version info (LDFlags verification)
FAILURE=false

echo "[4/4] Verifying Build Metadata (LDFlags)..."

SERVICE_LOGS=$(docker logs simple-service 2>&1)
if echo "$SERVICE_LOGS" | grep -q "version.*v1.0.0-e2e"; then
   echo -e "${GREEN}SUCCESS: Version metadata found in logs!${NC}"
else
   echo -e "${RED}FAILURE: Version metadata not found in logs${NC}"
   echo "Log snippet:"
   echo "$SERVICE_LOGS" | grep -i "version" | head -3
   FAILURE=true
fi

# Check Jaeger (looking for service name 'simple-service')
# Jaeger API: /api/traces?service=simple-service
echo "Checking Jaeger..."
JAEGER_RESPONSE=$(curl -s "http://localhost:$JAEGER_PORT/api/traces?service=simple-service")

if [[ $JAEGER_RESPONSE == *"traceID"* ]]; then
   echo -e "${GREEN}SUCCESS: Traces found in Jaeger!${NC}"
else
   echo -e "${RED}FAILURE: No traces found in Jaeger.${NC}"
   echo "Response snippet: ${JAEGER_RESPONSE:0:100}..."
   FAILURE=true
fi

# Check Prometheus (looking for 'request_count_total')
# Prometheus API: /api/v1/query?query=request_count_total
echo "Checking Prometheus..."
PROM_RESPONSE=$(curl -s "http://localhost:$PROM_PORT/api/v1/query?query=request_count_total")

if [[ $PROM_RESPONSE == *"success"* ]] && [[ $PROM_RESPONSE == *"simple-service"* ]]; then
   echo -e "${GREEN}SUCCESS: Metrics found in Prometheus!${NC}"
else
   echo -e "${RED}FAILURE: Metrics verification failed.${NC}"
   echo "Response snippet: ${PROM_RESPONSE:0:200}..."
   FAILURE=true
   # Debugging help
   echo "Debugging: Checking App Metrics Endpoint manually..."
   curl -v http://localhost:$METRICS_PORT/metrics


fi

if [ "$FAILURE" = true ]; then
    echo -e "${RED}E2E Tests Failed!${NC}"
    exit 1
else
    echo -e "${GREEN}All E2E Tests Passed!${NC}"
fi
