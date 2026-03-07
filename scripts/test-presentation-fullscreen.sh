#!/bin/bash
# 演示文稿全屏功能测试脚本

set -e

echo "🎬 演示文稿全屏功能测试"
echo "=========================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 1. 检查前端编译
echo "📦 [1/3] 检查前端编译..."
cd frontend
if npm run build > /tmp/presentation-build.log 2>&1; then
    echo -e "${GREEN}✓ 前端编译成功${NC}"
else
    echo -e "${RED}✗ 前端编译失败${NC}"
    cat /tmp/presentation-build.log
    exit 1
fi
cd ..
echo ""

# 2. 检查 RevealDeck 组件
echo "🔍 [2/3] 检查 RevealDeck 组件..."
if grep -q "fullscreen?: boolean" frontend/src/components/RevealDeck.tsx; then
    echo -e "${GREEN}✓ RevealDeck 支持全屏参数${NC}"
else
    echo -e "${RED}✗ RevealDeck 缺少全屏参数${NC}"
    exit 1
fi

if grep -q "requestFullscreen" frontend/src/components/RevealDeck.tsx; then
    echo -e "${GREEN}✓ RevealDeck 实现全屏 API${NC}"
else
    echo -e "${RED}✗ RevealDeck 未实现全屏 API${NC}"
    exit 1
fi
echo ""

# 3. 检查 PresentationRenderer
echo "🎨 [3/3] 检查 PresentationRenderer..."
if grep -q "fullscreen={isFullscreen}" frontend/src/lib/plugin/renderers/PresentationRenderer.tsx; then
    echo -e "${GREEN}✓ PresentationRenderer 传递全屏参数${NC}"
else
    echo -e "${RED}✗ PresentationRenderer 未传递全屏参数${NC}"
    exit 1
fi

if grep -q "fullscreenHint" frontend/src/lib/plugin/renderers/PresentationRenderer.tsx; then
    echo -e "${GREEN}✓ PresentationRenderer 包含全屏提示${NC}"
else
    echo -e "${RED}✗ PresentationRenderer 缺少全屏提示${NC}"
    exit 1
fi
echo ""

echo "=========================="
echo -e "${GREEN}✅ 所有检查通过!${NC}"
echo ""
echo "📝 测试步骤:"
echo "  1. 启动开发环境: bash scripts/dev.sh"
echo "  2. 登录学生账号: 13800000100 / student123"
echo "  3. 进入任意学习会话"
echo "  4. 等待 AI 生成演示文稿"
echo "  5. 点击 '📊 立即查看演示文稿'"
echo "  6. 点击工具栏 '⛶ 全屏' 按钮"
echo ""
echo "🎯 预期效果:"
echo "  - 演示文稿进入全屏模式"
echo "  - 底部显示退出提示 (3秒)"
echo "  - 可以使用键盘导航 (←/→/↑/↓)"
echo "  - 按 ESC 或 F 退出全屏"
echo ""
echo "📚 详细文档: docs/PRESENTATION_FULLSCREEN.md"
echo ""
