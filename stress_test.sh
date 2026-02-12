#!/bin/bash

API_URL="http://localhost:3002"
ITERATIONS=20
CONCURRENCY=5

echo "🚀 Starting Production-Grade Stress Test..."

# Function to hit a variety of endpoints
hit_endpoints() {
    local i=$1
    echo "Iteration $i: Sending requests..."
    
    # Root & Health
    curl -s "$API_URL/" > /dev/null
    curl -s "$API_URL/health" > /dev/null
    
    # User CRUD
    # 1. Create User
    USER_JSON="{\"name\":\"User_$i\",\"email\":\"user_$i@stress.test\"}"
    NEW_USER=$(curl -s -X POST "$API_URL/api/users" -H 'Content-Type: application/json' -d "$USER_JSON")
    USER_ID=$(echo $NEW_USER | grep -oP '"id":\K\d+')
    
    # 2. List Users
    curl -s "$API_URL/api/users?limit=20" > /dev/null
    
    # 3. Get User (Redis Cache Test)
    if [ ! -z "$USER_ID" ]; then
        # Hit 1: Miss
        curl -s "$API_URL/api/users/$USER_ID" > /dev/null
        # Hit 2 & 3: Hits
        curl -s "$API_URL/api/users/$USER_ID" > /dev/null
        curl -s "$API_URL/api/users/$USER_ID" > /dev/null
    fi
    
    # 4. Error Simulation
    curl -s "$API_URL/api/error" > /dev/null
    curl -s "$API_URL/debug/error" > /dev/null
    
    # 5. Panic Simulation (caught by middleware)
    curl -s "$API_URL/api/panic" > /dev/null
}

export -f hit_endpoints
export API_URL

# Simulating concurrent traffic using xargs
seq 1 $ITERATIONS | xargs -n 1 -P $CONCURRENCY bash -c 'hit_endpoints "$@"'

echo "✅ Stress Test Completed. Check your Grafana Dashboard!"
