#!/bin/bash
set -e

echo "=== OmniRAG Agent Test ==="
echo

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

API_URL="http://localhost:8082/api/agent/query"

echo -e "${BLUE}Testing Agent Endpoint${NC}"
echo "POST $API_URL"
echo

# Test query
QUERY="How does the PDF explain pump maintenance?"

echo -e "${BLUE}Request:${NC}"
echo "{\"query\": \"$QUERY\"}"
echo

echo -e "${BLUE}Response:${NC}"
curl -s -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -d "{\"query\": \"$QUERY\"}" | jq .

echo
echo -e "${GREEN}✓ Test complete${NC}"
