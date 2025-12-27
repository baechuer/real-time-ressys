#!/bin/bash

# =================配置区域=================
AUTH_HOST="http://localhost:8080"
EVENT_HOST="http://localhost:8081"
EMAIL="user@example.com"
PASSWORD="UserPassword123!"
# =========================================

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[STEP] $1${NC}"
}

success() {
    echo -e "${GREEN}✔ $1${NC}"
}

error() {
    echo -e "${RED}✘ $1${NC}"
    exit 1
}

# 0. 检查依赖
if ! command -v jq &> /dev/null; then
    error "jq 未安装。请运行: sudo apt-get install jq (Ubuntu) 或 brew install jq (Mac) 或 choco install jq (Windows)"
fi

echo "==========================================="
echo "   Redis Caching Integration Test Script   "
echo "==========================================="

# 1. 登录 (Login)
log "1. Logging into Auth Service..."
LOGIN_RESP=$(curl -s -X POST "$AUTH_HOST/auth/v1/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\": \"$EMAIL\", \"password\": \"$PASSWORD\"}")

# 提取 Token
TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.tokens.access_token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
    echo "Login Response: $LOGIN_RESP"
    error "Login failed! Could not extract token."
fi
success "Login successful! Token acquired."

# 2. 创建活动 (Create Draft)
log "2. Creating a new Event..."
CREATE_RESP=$(curl -s -X POST "$EVENT_HOST/event/v1/events" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Auto Test Event",
    "description": "Created by bash script",
    "city": "Shanghai",
    "category": "Tech",
    "start_time": "2025-12-30T10:00:00Z",
    "end_time": "2025-12-30T12:00:00Z",
    "capacity": 50
  }')

EVENT_ID=$(echo "$CREATE_RESP" | jq -r '.data.id')

if [ "$EVENT_ID" == "null" ] || [ -z "$EVENT_ID" ]; then
    echo "Create Response: $CREATE_RESP"
    error "Create failed!"
fi
success "Event created. ID: $EVENT_ID"

# 3. 发布活动 (Publish)
log "3. Publishing Event..."
PUB_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$EVENT_HOST/event/v1/events/$EVENT_ID/publish" \
  -H "Authorization: Bearer $TOKEN")

if [ "$PUB_CODE" -ne 200 ]; then
    error "Publish failed with code $PUB_CODE"
fi
success "Event published."

# 4. 获取详情 - 第一次 (Cache Miss & Set)
log "4. Get Public Detail (Expect Cache MISS -> SET)"
# 稍微睡一下让日志打印清楚点
sleep 0.5
curl -s "$EVENT_HOST/event/v1/events/$EVENT_ID" | jq -r '.data.title' > /dev/null
success "Request sent. (Check server logs for 'cache miss')"

# 5. 获取详情 - 第二次 (Cache Hit)
log "5. Get Public Detail Again (Expect Cache HIT)"
sleep 0.5
START_TIME=$(date +%s%N)
curl -s "$EVENT_HOST/event/v1/events/$EVENT_ID" | jq -r '.data.title' > /dev/null
END_TIME=$(date +%s%N)
# 计算耗时 (纳秒 -> 毫秒)
DURATION=$(( ($END_TIME - $START_TIME) / 1000000 ))
success "Request sent in ${DURATION}ms. (Check server logs for 'cache hit')"

# 6. 更新活动 (Invalidate Cache)
log "6. Updating Event Title (Expect Cache INVALIDATE)"
NEW_TITLE="Auto Test Event - UPDATED"
UPDATE_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$EVENT_HOST/event/v1/events/$EVENT_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"title\": \"$NEW_TITLE\"}")

if [ "$UPDATE_CODE" -ne 200 ]; then
    error "Update failed code $UPDATE_CODE"
fi
success "Event updated."

# 7. 再次获取详情 (Verify New Data)
log "7. Get Detail After Update (Expect MISS & New Data)"
GET_RESP=$(curl -s "$EVENT_HOST/event/v1/events/$EVENT_ID")
ACTUAL_TITLE=$(echo "$GET_RESP" | jq -r '.data.title')

if [ "$ACTUAL_TITLE" == "$NEW_TITLE" ]; then
    success "Data verification passed: Got '$ACTUAL_TITLE'"
else
    error "Data verification failed! Expected '$NEW_TITLE', got '$ACTUAL_TITLE'. Cache invalidation might be broken."
fi

# 8. 列表页测试 (List & Cursor)
log "8. List Events (First Page)"
LIST_RESP=$(curl -s "$EVENT_HOST/event/v1/events?city=Shanghai&page_size=1")
NEXT_CURSOR=$(echo "$LIST_RESP" | jq -r '.data.next_cursor')
ITEM_COUNT=$(echo "$LIST_RESP" | jq '.data.items | length')

success "Got page 1 with $ITEM_COUNT items."
echo "Next Cursor: $NEXT_CURSOR"

if [ "$NEXT_CURSOR" != "null" ] && [ -n "$NEXT_CURSOR" ]; then
    log "9. List Events (Next Page using Cursor)"
    # 注意：curl GET 带参数时，特殊字符(|)需要被引用，bash变量加双引号即可
    PAGE2_CODE=$(curl -s -o /dev/null -w "%{http_code}" -G "$EVENT_HOST/event/v1/events" \
        --data-urlencode "city=Shanghai" \
        --data-urlencode "page_size=1" \
        --data-urlencode "cursor=$NEXT_CURSOR")
    
    if [ "$PAGE2_CODE" -eq 200 ]; then
        success "Page 2 request successful (Bypassed Cache)."
    else
        error "Page 2 failed with code $PAGE2_CODE"
    fi
else
    echo -e "${BLUE}(Skipping Step 9: No next_cursor returned, maybe only 1 event exists?)${NC}"
fi

echo "==========================================="
echo -e "${GREEN}ALL TESTS PASSED!${NC}"
echo "Check your Server Logs (Terminal A) to verify 'cache hit/miss' messages."
echo "==========================================="