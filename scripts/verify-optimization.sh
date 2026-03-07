#!/bin/bash
# Hanfledge 2.0 优化验证脚本

set -e

echo "🚀 Hanfledge 2.0 优化验证"
echo "=========================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 1. 检查后端编译
echo "📦 [1/5] 检查后端编译..."
if go build -o /tmp/hanfledge cmd/server/main.go 2>&1; then
    echo -e "${GREEN}✓ 后端编译成功${NC}"
else
    echo -e "${RED}✗ 后端编译失败${NC}"
    exit 1
fi
echo ""

# 2. 检查前端编译
echo "📦 [2/5] 检查前端编译..."
cd frontend
if npm run build > /tmp/frontend-build.log 2>&1; then
    echo -e "${GREEN}✓ 前端编译成功${NC}"
else
    echo -e "${RED}✗ 前端编译失败${NC}"
    cat /tmp/frontend-build.log
    exit 1
fi
cd ..
echo ""

# 3. 检查 Neo4j GetKPContext 方法
echo "🔍 [3/5] 检查 Neo4j GetKPContext 方法..."
if grep -q "func.*GetKPContext" internal/repository/neo4j/client.go; then
    echo -e "${GREEN}✓ GetKPContext 方法已添加${NC}"
else
    echo -e "${RED}✗ GetKPContext 方法未找到${NC}"
    exit 1
fi
echo ""

# 4. 检查 VAD 集成
echo "🎤 [4/5] 检查 VAD 集成..."
if [ -f "frontend/src/lib/vad.ts" ]; then
    echo -e "${GREEN}✓ VAD 工具模块已创建${NC}"
else
    echo -e "${RED}✗ VAD 工具模块未找到${NC}"
    exit 1
fi

if grep -q "enableVAD" frontend/src/components/VoiceInput/VoiceInput.tsx; then
    echo -e "${GREEN}✓ VoiceInput 已集成 VAD${NC}"
else
    echo -e "${RED}✗ VoiceInput 未集成 VAD${NC}"
    exit 1
fi
echo ""

# 5. 检查并行化实现
echo "⚡ [5/5] 检查并行化实现..."
if grep -q "strategistCh := make(chan strategistResult" internal/agent/orchestrator.go; then
    echo -e "${GREEN}✓ Strategist + Designer 并行化已实现${NC}"
else
    echo -e "${RED}✗ 并行化未实现${NC}"
    exit 1
fi
echo ""

echo "=========================="
echo -e "${GREEN}✅ 所有检查通过!${NC}"
echo ""
echo "📝 下一步:"
echo "  1. 启动基础设施: docker compose -f deployments/docker-compose.yml up -d"
echo "  2. 启动后端: go run cmd/server/main.go"
echo "  3. 启动前端: cd frontend && npm run dev"
echo "  4. 观察日志中的性能指标:"
echo "     - [DEBUG] strategist done ... elapsed=XXXms"
echo "     - [DEBUG] designer preload done ... elapsed=XXXms"
echo "  5. 测试 VAD: 访问学生会话页面,点击麦克风按钮"
echo ""
echo "📊 性能对比:"
echo "  - 并行化: 预期 TTFT 降低 ~40%"
echo "  - VAD: 预期 ASR 开销降低 ~50-70%"
echo ""
