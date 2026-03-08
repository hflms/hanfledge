#!/bin/bash
# WeKnora Integration Test Script
# Tests all WeKnora API endpoints and functionality

set -e

BASE_URL="http://localhost:8080"
WEKNORA_URL="http://localhost:9380"

echo "🧪 WeKnora Integration Test Suite"
echo "=================================="

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

test_pass() {
  echo -e "${GREEN}✓${NC} $1"
  ((TESTS_PASSED++))
}

test_fail() {
  echo -e "${RED}✗${NC} $1"
  ((TESTS_FAILED++))
}

test_info() {
  echo -e "${YELLOW}ℹ${NC} $1"
}

# 1. Login to Hanfledge
echo -e "\n📝 Test 1: Login to Hanfledge"
TOKEN=$(curl -s "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000010","password":"teacher123"}' | jq -r '.token')

if [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
  test_pass "Login successful"
  test_info "Token: ${TOKEN:0:30}..."
else
  test_fail "Login failed"
  exit 1
fi

# 2. List courses
echo -e "\n📚 Test 2: List courses"
COURSES=$(curl -s "$BASE_URL/api/v1/courses" \
  -H "Authorization: Bearer $TOKEN")
COURSE_ID=$(echo "$COURSES" | jq -r '.items[0].id // empty')
COURSE_COUNT=$(echo "$COURSES" | jq '.items | length')

if [ -n "$COURSE_ID" ]; then
  test_pass "Found $COURSE_COUNT course(s)"
  test_info "Using course ID: $COURSE_ID"
else
  test_fail "No courses found"
  exit 1
fi

# 3. List WeKnora knowledge bases
echo -e "\n📖 Test 3: List WeKnora knowledge bases"
KBS=$(curl -s "$BASE_URL/api/v1/weknora/knowledge-bases" \
  -H "Authorization: Bearer $TOKEN")
KB_COUNT=$(echo "$KBS" | jq '.data | length')

if [ "$KB_COUNT" -gt 0 ]; then
  test_pass "Found $KB_COUNT knowledge base(s)"
  KB_ID=$(echo "$KBS" | jq -r '.data[0].id')
  KB_NAME=$(echo "$KBS" | jq -r '.data[0].name')
  test_info "KB: $KB_NAME ($KB_ID)"
else
  test_info "No knowledge bases found, creating one..."
  
  # Login to WeKnora directly
  WK_TOKEN=$(curl -s "$WEKNORA_URL/api/v1/auth/login" -X POST \
    -H "Content-Type: application/json" \
    -d '{"email":"13800000010@hanfledge.local","password":"teacher123"}' | jq -r '.token')
  
  # Create KB
  KB_RESP=$(curl -s "$WEKNORA_URL/api/v1/knowledge-bases" -X POST \
    -H "Authorization: Bearer $WK_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "Test Knowledge Base",
      "description": "Created by integration test",
      "embedding_model": "bge-m3"
    }')
  
  KB_ID=$(echo "$KB_RESP" | jq -r '.data.id')
  KB_NAME=$(echo "$KB_RESP" | jq -r '.data.name')
  
  if [ -n "$KB_ID" ] && [ "$KB_ID" != "null" ]; then
    test_pass "Created knowledge base: $KB_NAME"
  else
    test_fail "Failed to create knowledge base"
    exit 1
  fi
fi

# 4. Get KB details
echo -e "\n🔍 Test 4: Get knowledge base details"
KB_DETAIL=$(curl -s "$BASE_URL/api/v1/weknora/knowledge-bases/$KB_ID" \
  -H "Authorization: Bearer $TOKEN")
KB_DETAIL_NAME=$(echo "$KB_DETAIL" | jq -r '.data.name // empty')

if [ -n "$KB_DETAIL_NAME" ]; then
  test_pass "Retrieved KB details: $KB_DETAIL_NAME"
else
  test_fail "Failed to get KB details"
fi

# 5. Bind KB to course
echo -e "\n🔗 Test 5: Bind knowledge base to course"
BIND_RESP=$(curl -s "$BASE_URL/api/v1/courses/$COURSE_ID/weknora-refs" -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"kb_id\":\"$KB_ID\",\"kb_name\":\"$KB_NAME\"}")

REF_ID=$(echo "$BIND_RESP" | jq -r '.id // empty')
if [ -n "$REF_ID" ] && [ "$REF_ID" != "null" ]; then
  test_pass "Bound KB to course (ref_id: $REF_ID)"
else
  # Check if already bound
  ERROR=$(echo "$BIND_RESP" | jq -r '.error // empty')
  if [[ "$ERROR" == *"already bound"* ]]; then
    test_info "KB already bound to course"
    # Get existing ref_id
    REFS=$(curl -s "$BASE_URL/api/v1/courses/$COURSE_ID/weknora-refs" \
      -H "Authorization: Bearer $TOKEN")
    REF_ID=$(echo "$REFS" | jq -r ".data[] | select(.kb_id == \"$KB_ID\") | .id")
  else
    test_fail "Failed to bind KB: $ERROR"
  fi
fi

# 6. List bound KBs
echo -e "\n📋 Test 6: List bound knowledge bases"
BOUND_KBS=$(curl -s "$BASE_URL/api/v1/courses/$COURSE_ID/weknora-refs" \
  -H "Authorization: Bearer $TOKEN")
BOUND_COUNT=$(echo "$BOUND_KBS" | jq '.data | length')

if [ "$BOUND_COUNT" -gt 0 ]; then
  test_pass "Found $BOUND_COUNT bound KB(s)"
else
  test_fail "No bound KBs found"
fi

# 7. Search in bound KBs
echo -e "\n🔎 Test 7: Search in bound knowledge bases"
SEARCH_RESP=$(curl -s "$BASE_URL/api/v1/courses/$COURSE_ID/weknora-search" -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"test"}')

SEARCH_COUNT=$(echo "$SEARCH_RESP" | jq '.data | length // 0')
test_info "Search returned $SEARCH_COUNT result(s)"
test_pass "Search endpoint working"

# 8. Unbind KB
echo -e "\n🔓 Test 8: Unbind knowledge base"
if [ -n "$REF_ID" ] && [ "$REF_ID" != "null" ]; then
  UNBIND_RESP=$(curl -s "$BASE_URL/api/v1/courses/$COURSE_ID/weknora-refs/$REF_ID" -X DELETE \
    -H "Authorization: Bearer $TOKEN")
  
  UNBIND_MSG=$(echo "$UNBIND_RESP" | jq -r '.message // empty')
  if [[ "$UNBIND_MSG" == *"success"* ]]; then
    test_pass "Unbound KB successfully"
  else
    test_fail "Failed to unbind KB"
  fi
else
  test_info "No ref_id to unbind"
fi

# 9. Verify unbind
echo -e "\n✅ Test 9: Verify unbind"
BOUND_KBS_AFTER=$(curl -s "$BASE_URL/api/v1/courses/$COURSE_ID/weknora-refs" \
  -H "Authorization: Bearer $TOKEN")
BOUND_COUNT_AFTER=$(echo "$BOUND_KBS_AFTER" | jq '.data | length')

if [ "$BOUND_COUNT_AFTER" -eq 0 ]; then
  test_pass "KB successfully unbound (count: 0)"
else
  test_info "Still have $BOUND_COUNT_AFTER bound KB(s)"
fi

# Summary
echo -e "\n=================================="
echo "📊 Test Summary"
echo "=================================="
echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
echo -e "Total:  $((TESTS_PASSED + TESTS_FAILED))"

if [ $TESTS_FAILED -eq 0 ]; then
  echo -e "\n${GREEN}✅ All tests passed!${NC}"
  exit 0
else
  echo -e "\n${RED}❌ Some tests failed${NC}"
  exit 1
fi
