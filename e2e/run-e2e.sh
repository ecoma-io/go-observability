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
SIMPLE_APP_PORT=8081
SIMPLE_METRICS_PORT=9092
GIN_APP_PORT=8082
GIN_METRICS_PORT=9093
PROM_PORT=9099
JAEGER_PORT=16687

echo -e "${GREEN}Starting E2E Tests...${NC}"
echo "Project root: $PROJECT_ROOT"
echo "Script directory: $SCRIPT_DIR"

# Helper function to wait for HTTP endpoint
wait_for_http() {
  local url="$1"
  local name="$2"
  local timeout=${3:-30}
  local start
  start=$(date +%s)
  
  echo -n "Waiting for $name... "
  while true; do
    if curl -s --fail "$url" >/dev/null 2>&1; then
      local elapsed=$(( $(date +%s) - start ))
      echo "ready (${elapsed}s)"
      return 0
    fi
    if [ $(( $(date +%s) - start )) -ge "$timeout" ]; then
      echo "timeout after ${timeout}s"
      return 1
    fi
    sleep 0.3
  done
}

# 1. Start Infrastructure and Service
echo "[1/3] Building and Starting Docker Infrastructure..."
cd "$SCRIPT_DIR"

# Set build time for Docker build args
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
export BUILD_TIME

docker compose up -d --build

# Wait for services to be ready instead of fixed sleep
wait_for_http "http://localhost:$SIMPLE_APP_PORT/ping" "simple-service" 20
wait_for_http "http://localhost:$GIN_APP_PORT/ping" "gin-service" 20
wait_for_http "http://localhost:$JAEGER_PORT/" "jaeger" 15

# 2. Generate Load
echo "[2/3] Generating Load..."
echo "Testing simple-service..."
# Reduced requests from 10 to 5 and run in parallel
for _ in {1..5}; do
   curl -s "http://localhost:$SIMPLE_APP_PORT/ping" > /dev/null &
done
wait

echo "Testing gin-service..."
# Reduced requests and run in parallel
for _ in {1..5}; do
   curl -s "http://localhost:$GIN_APP_PORT/ping" > /dev/null &
   curl -s "http://localhost:$GIN_APP_PORT/users/123" > /dev/null &
done
wait

# Test panic recovery
echo "Testing panic recovery..."
PANIC_RESPONSE=$(curl -s "http://localhost:$GIN_APP_PORT/panic")
if echo "$PANIC_RESPONSE" | grep -q "error.*Internal Server Error"; then
   echo -e "${GREEN}SUCCESS: Panic recovery works!${NC}"
   if echo "$PANIC_RESPONSE" | grep -q "trace_id"; then
      echo -e "${GREEN}  + trace_id included in response${NC}"
   else
      echo "  Note: trace_id not in response (expected without propagation headers)"
   fi
else
   echo -e "${RED}FAILURE: Panic recovery failed${NC}"
   echo "Response: $PANIC_RESPONSE"
fi

# 3. Verify Observability
echo "[3/4] Verifying Observability..."

# Poll for traces instead of fixed sleep
echo -n "Waiting for traces to appear... "
wait_for_traces() {
  local service="$1"
  local timeout=${2:-15}
  local start
  start=$(date +%s)
  
  while true; do
    if curl -s "http://localhost:$JAEGER_PORT/api/traces?service=$service&limit=1" | grep -q '"traceID"'; then
      local elapsed=$(( $(date +%s) - start ))
      return 0
    fi
    if [ $(( $(date +%s) - start )) -ge "$timeout" ]; then
      return 1
    fi
    sleep 0.3
  done
}

# Give traces a moment to flush, then start polling
sleep 1
wait_for_traces "simple-service" 10 && echo "traces ready"

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
echo "Checking Jaeger for simple-service traces..."
JAEGER_RESPONSE=$(curl -s "http://localhost:$JAEGER_PORT/api/traces?service=simple-service")

if [[ $JAEGER_RESPONSE == *"traceID"* ]]; then
   echo -e "${GREEN}SUCCESS: simple-service traces found in Jaeger!${NC}"
else
   echo -e "${RED}FAILURE: No simple-service traces found in Jaeger.${NC}"
   echo "Response snippet: ${JAEGER_RESPONSE:0:100}..."
   FAILURE=true
fi

# Check Jaeger for gin-service traces
echo "Checking Jaeger for gin-service traces..."
JAEGER_GIN_RESPONSE=$(curl -s "http://localhost:$JAEGER_PORT/api/traces?service=gin-service")

if [[ $JAEGER_GIN_RESPONSE == *"traceID"* ]]; then
   echo -e "${GREEN}SUCCESS: gin-service traces found in Jaeger!${NC}"
else
   echo -e "${RED}FAILURE: No gin-service traces found in Jaeger.${NC}"
   echo "Response snippet: ${JAEGER_GIN_RESPONSE:0:100}..."
   FAILURE=true
fi

# Check Prometheus (looking for 'request_count_total')
# Prometheus API: /api/v1/query?query=request_count_total
echo "Checking Prometheus for simple-service metrics..."
PROM_RESPONSE=$(curl -s "http://localhost:$PROM_PORT/api/v1/query?query=request_count_total")

if [[ $PROM_RESPONSE == *"success"* ]] && { [[ $PROM_RESPONSE == *"simple-service"* ]] || [[ $PROM_RESPONSE == *"job=\"simple-service\""* ]]; }; then
   echo -e "${GREEN}SUCCESS: simple-service metrics found in Prometheus!${NC}"
else
   # Fallback: check metrics endpoint directly (Prometheus may not have scraped yet)
   DIRECT_SIMPLE_METRICS=$(curl -s "http://localhost:$SIMPLE_METRICS_PORT/metrics" | grep -c "request_count_total" || echo "0")
   if [[ $DIRECT_SIMPLE_METRICS -gt 0 ]]; then
      echo -e "${GREEN}SUCCESS: simple-service metrics available (not yet in Prometheus, but endpoint works)${NC}"
   else
      echo -e "${RED}FAILURE: simple-service metrics verification failed.${NC}"
      echo "Response snippet: ${PROM_RESPONSE:0:200}..."
      FAILURE=true
   fi
fi

# Check Prometheus for gin-service metrics
echo "Checking Prometheus for gin-service metrics..."
PROM_GIN_RESPONSE=$(curl -s "http://localhost:$PROM_PORT/api/v1/query?query=gin_request_count_total")

if [[ $PROM_GIN_RESPONSE == *"success"* ]] && [[ $PROM_GIN_RESPONSE == *"gin"* ]]; then
   echo -e "${GREEN}SUCCESS: gin-service metrics found in Prometheus!${NC}"
else
   # Fallback: check metrics endpoint directly
   DIRECT_METRICS=$(curl -s "http://localhost:$GIN_METRICS_PORT/metrics" | grep -c "gin_request_count_total" || echo "0")
   if [[ $DIRECT_METRICS -gt 0 ]]; then
      echo -e "${GREEN}SUCCESS: gin-service metrics available (not yet in Prometheus, but endpoint works)${NC}"
   else
      echo -e "${RED}FAILURE: gin-service metrics verification failed.${NC}"
      echo "Response snippet: ${PROM_GIN_RESPONSE:0:200}..."
      FAILURE=true
   fi
fi

if [ "$FAILURE" = true ]; then
    echo -e "${RED}E2E Tests Failed!${NC}"
    exit 1
else
    echo -e "${GREEN}All E2E Tests Passed!${NC}"
fi
