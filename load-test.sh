#!/bin/bash

BASE_URL="http://localhost:3002"

echo "🚀 Starting Load Generation..."

# Create 20 users
for i in {1..20}; do
  curl -s -X POST "$BASE_URL/api/users" \
    -H 'Content-Type: application/json' \
    -d "{\"name\":\"User_$i\", \"email\":\"user_$i@example.com\"}" > /dev/null
  echo -n "."
  if [ $((i % 5)) -eq 0 ]; then echo " $i"; fi
done

echo -e "\n✅ Created 20 users."

# Generate traffic: 50 requests
echo "📈 Generating GET traffic..."
for i in {1..50}; do
  curl -s "$BASE_URL/api/users" > /dev/null
  curl -s "$BASE_URL/health" > /dev/null
  if [ $((i % 10)) -eq 0 ]; then echo -n "$i "; fi
  sleep 0.1
done

echo -e "\n✅ Traffic generation complete."
echo "📊 Check your Grafana Dashboards now!"
