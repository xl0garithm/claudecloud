#!/bin/bash
set -euo pipefail

API="http://localhost:8080"
KEY="dev-api-key"

echo "=== CloudCode E2E Test ==="

# 1. Health check
echo -n "Health check... "
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$API/healthz")
if [ "$STATUS" != "200" ]; then
  echo "FAIL (got $STATUS)"
  exit 1
fi
echo "OK"

# 2. Create instance
echo -n "Create instance... "
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API/instances/" \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"user_id": 1}')
HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')
if [ "$HTTP_CODE" != "201" ]; then
  echo "FAIL (got $HTTP_CODE): $BODY"
  exit 1
fi
INSTANCE_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "$BODY" | grep -o '"id":[0-9]*' | grep -o '[0-9]*')
echo "OK (id=$INSTANCE_ID)"

# 3. Get instance
echo -n "Get instance... "
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$API/instances/$INSTANCE_ID" \
  -H "X-API-Key: $KEY")
if [ "$STATUS" != "200" ]; then
  echo "FAIL (got $STATUS)"
  exit 1
fi
echo "OK"

# 4. Pause instance
echo -n "Pause instance... "
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/instances/$INSTANCE_ID/pause" \
  -H "X-API-Key: $KEY")
if [ "$STATUS" != "200" ]; then
  echo "FAIL (got $STATUS)"
  exit 1
fi
echo "OK"

# 5. Wake instance
echo -n "Wake instance... "
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/instances/$INSTANCE_ID/wake" \
  -H "X-API-Key: $KEY")
if [ "$STATUS" != "200" ]; then
  echo "FAIL (got $STATUS)"
  exit 1
fi
echo "OK"

# 6. Delete instance
echo -n "Delete instance... "
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$API/instances/$INSTANCE_ID" \
  -H "X-API-Key: $KEY")
if [ "$STATUS" != "200" ]; then
  echo "FAIL (got $STATUS)"
  exit 1
fi
echo "OK"

# 7. Verify deleted (should return destroyed status)
echo -n "Verify deleted... "
RESPONSE=$(curl -s "$API/instances/$INSTANCE_ID" -H "X-API-Key: $KEY")
INST_STATUS=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null || echo "unknown")
if [ "$INST_STATUS" != "destroyed" ]; then
  echo "FAIL (status=$INST_STATUS)"
  exit 1
fi
echo "OK"

echo ""
echo "=== All E2E tests passed ==="
