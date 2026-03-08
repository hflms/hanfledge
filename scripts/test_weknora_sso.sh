#!/bin/bash
# WeKnora SSO 单点登录测试脚本

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║          WeKnora SSO 单点登录测试                            ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# 1. 检查服务状态
echo "📡 检查服务状态..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

check_service() {
    local name=$1
    local url=$2
    if curl -s -f "$url" > /dev/null 2>&1; then
        echo "✅ $name: $url"
    else
        echo "❌ $name: $url (不可用)"
        return 1
    fi
}

check_service "Hanfledge Backend" "http://localhost:8080/health"
check_service "WeKnora Backend" "http://localhost:9380/health"
check_service "WeKnora Frontend" "http://localhost:9381/"

echo ""

# 2. 登录 Hanfledge
echo "🔐 登录 Hanfledge..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

TOKEN=$(curl -s http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000010","password":"teacher123"}' | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "❌ Hanfledge 登录失败"
    exit 1
fi

echo "✅ Hanfledge 登录成功"
echo "   Token: ${TOKEN:0:50}..."
echo ""

# 3. 获取 WeKnora 登录 token
echo "🎫 获取 WeKnora SSO token..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

RESP=$(curl -s http://localhost:8080/api/v1/weknora/login-token \
  -H "Authorization: Bearer $TOKEN")

WK_TOKEN=$(echo "$RESP" | jq -r '.token')
WK_URL=$(echo "$RESP" | jq -r '.weknora_url')

if [ -z "$WK_TOKEN" ] || [ "$WK_TOKEN" = "null" ]; then
    echo "❌ 获取 WeKnora token 失败"
    echo "$RESP" | jq .
    exit 1
fi

echo "✅ 获取 WeKnora token 成功"
echo "   Token: ${WK_TOKEN:0:50}..."
echo "   URL: $WK_URL"
echo ""

# 4. 验证 token 有效性
echo "🔍 验证 token 有效性..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

KB_RESP=$(curl -s http://localhost:9380/api/v1/knowledge-bases \
  -H "Authorization: Bearer $WK_TOKEN")

KB_SUCCESS=$(echo "$KB_RESP" | jq -r '.success')
KB_COUNT=$(echo "$KB_RESP" | jq -r '.data | length')

if [ "$KB_SUCCESS" = "true" ]; then
    echo "✅ Token 有效，可以访问 WeKnora API"
    echo "   知识库数量: $KB_COUNT"
else
    echo "❌ Token 无效"
    echo "$KB_RESP" | jq .
    exit 1
fi

echo ""

# 5. 构造 SSO URL
echo "🔗 构造 SSO URL..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

SSO_URL="${WK_URL}?token=${WK_TOKEN}"
echo "✅ SSO URL 已生成"
echo ""
echo "   $SSO_URL"
echo ""

# 6. 测试总结
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                    测试总结 ✅                                ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "✅ 所有测试通过"
echo ""
echo "📝 手动验证步骤:"
echo "   1. 访问 http://localhost:3000"
echo "   2. 登录: 13800000010 / teacher123"
echo "   3. 点击导航栏「WeKnora 知识库」"
echo "   4. 点击「打开 WeKnora 管理界面」按钮"
echo "   5. 验证是否自动登录到 WeKnora"
echo ""
echo "⚠️  如果 WeKnora 前端未自动登录:"
echo "   需要修改 WeKnora 前端代码支持 URL token 参数"
echo "   参考: docs/WEKNORA_SSO_SUMMARY.md"
echo ""
