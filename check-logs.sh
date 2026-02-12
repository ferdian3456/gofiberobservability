#!/bin/bash

# Script untuk mengecek logs di Loki
# Usage: ./check-logs.sh

echo "=== Checking Loki Status ==="
curl -s http://localhost:3102/ready
echo -e "\n"

echo "=== Checking Available Labels in Loki ==="
curl -s -G "http://localhost:3102/loki/api/v1/labels" | jq -r '.data[]' | head -20
echo -e "\n"

echo "=== Checking Logs Count for gofiberobservability ==="
LOG_COUNT=$(curl -s -G "http://localhost:3102/loki/api/v1/query" \
  --data-urlencode 'query={service_name="gofiberobservability"}' \
  --data-urlencode 'limit=100' | jq '.data.result | length')
echo "Total log streams found: $LOG_COUNT"
echo -e "\n"

if [ "$LOG_COUNT" -gt 0 ]; then
  echo "=== Sample Logs (Latest 5) ==="
  curl -s -G "http://localhost:3102/loki/api/v1/query" \
    --data-urlencode 'query={service_name="gofiberobservability"}' \
    --data-urlencode 'limit=5' | jq -r '.data.result[0].values[]? | .[1]' | head -5
  echo -e "\n"
  
  echo "✅ SUCCESS: Logs are flowing to Loki!"
else
  echo "❌ No logs found in Loki yet."
  echo "Please ensure:"
  echo "  1. Application is running (go run main.go)"
  echo "  2. You've made some HTTP requests to the application"
  echo "  3. Wait 10-15 seconds for batch export"
  echo "  4. OTEL Collector is running (docker compose ps)"
fi

echo -e "\n=== Checking OTEL Collector Status ==="
curl -s http://localhost:13133/ | jq
