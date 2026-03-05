#!/usr/bin/env bash
# ============================================================
# Hanfledge 开发环境一键启动脚本
# Usage: bash scripts/dev.sh [--seed] [--backend-only] [--frontend-only] [--weknora]
# ============================================================
set -euo pipefail

# ── Colors ───────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[  OK]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*"; exit 1; }

# ── Parse Arguments ──────────────────────────────────────────
SEED=false
BACKEND_ONLY=false
FRONTEND_ONLY=false
WEKNORA=false

for arg in "$@"; do
    case $arg in
        --seed)           SEED=true ;;
        --backend-only)   BACKEND_ONLY=true ;;
        --frontend-only)  FRONTEND_ONLY=true ;;
        --weknora)        WEKNORA=true ;;
        -h|--help)
            echo "Usage: bash scripts/dev.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --seed            运行种子数据脚本 (创建测试账号)"
            echo "  --backend-only    仅启动后端 (跳过前端)"
            echo "  --frontend-only   仅启动前端 (跳过后端和基础设施)"
            echo "  --weknora         同时启动 WeKnora 知识库服务"
            echo "  -h, --help        显示帮助信息"
            exit 0
            ;;
        *) warn "Unknown argument: $arg" ;;
    esac
done

# ── Project Root ─────────────────────────────────────────────
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

echo ""
echo -e "${BOLD}🚀 Hanfledge 开发环境启动${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ── .env Check ───────────────────────────────────────────────
if [ ! -f .env ]; then
    warn ".env 文件不存在, 从 .env.example 复制..."
    cp .env.example .env
    ok ".env 已创建, 请根据需要修改配置"
fi

# ── Go Proxy ─────────────────────────────────────────────────
export GOPROXY=https://goproxy.cn,https://goproxy.io,direct

# ── Prerequisite Check ───────────────────────────────────────
check_cmd() {
    if ! command -v "$1" &>/dev/null; then
        fail "未找到 $1, 请先安装: $2"
    fi
}

check_cmd docker      "https://docs.docker.com/get-docker/"
check_cmd go          "https://go.dev/dl/"

# Check for npm or bun (prefer bun if available)
PKG_MANAGER="npm"
if command -v bun &>/dev/null; then
    PKG_MANAGER="bun"
fi

# ── 1. Start Infrastructure ─────────────────────────────────
if [ "$FRONTEND_ONLY" = false ]; then
    info "启动基础设施 (PostgreSQL + Neo4j + Redis)..."
    COMPOSE_PROFILES=""
    if [ "$WEKNORA" = true ]; then
        COMPOSE_PROFILES="--profile weknora"
        info "WeKnora 知识库服务已启用"
    fi
    docker compose -f deployments/docker-compose.yml $COMPOSE_PROFILES up -d

    # Wait for PostgreSQL to be ready
    info "等待 PostgreSQL 就绪..."
    RETRIES=30
    until docker exec hanfledge-postgres pg_isready -U hanfledge &>/dev/null || [ $RETRIES -eq 0 ]; do
        RETRIES=$((RETRIES - 1))
        sleep 1
    done

    if [ $RETRIES -eq 0 ]; then
        fail "PostgreSQL 启动超时"
    fi
    ok "PostgreSQL 就绪 (端口 5433)"

    # Check Neo4j (non-blocking)
    if docker ps --format '{{.Names}}' | grep -q hanfledge-neo4j; then
        ok "Neo4j 启动中 (Web UI: http://localhost:7475)"
    fi

    # Check Redis
    if docker ps --format '{{.Names}}' | grep -q hanfledge-redis; then
        ok "Redis 就绪 (端口 6381)"
    fi

    # Check WeKnora
    if [ "$WEKNORA" = true ]; then
        if docker ps --format '{{.Names}}' | grep -q hanfledge-weknora; then
            ok "WeKnora 启动中 (API: http://localhost:9380)"
        else
            warn "WeKnora 容器未启动, 请检查镜像是否可用"
        fi
    fi
fi

# ── 2. Seed Data (optional) ─────────────────────────────────
if [ "$SEED" = true ] && [ "$FRONTEND_ONLY" = false ]; then
    echo ""
    info "填充测试数据..."
    go run scripts/seed.go
    ok "种子数据已填充"
fi

# ── 3. Install Frontend Dependencies ────────────────────────
if [ "$BACKEND_ONLY" = false ]; then
    if [ ! -d frontend/node_modules ]; then
        info "安装前端依赖 ($PKG_MANAGER install)..."
        (cd frontend && $PKG_MANAGER install)
        ok "前端依赖安装完成"
    else
        ok "前端依赖已存在, 跳过安装"
    fi
fi

# ── 4. Start Services ───────────────────────────────────────
echo ""
echo -e "${BOLD}🎬 启动应用服务${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Trap to clean up background processes on exit
PIDS=()
cleanup() {
    echo ""
    info "正在停止服务..."
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null
    ok "所有服务已停止"
}
trap cleanup EXIT INT TERM

# Start Backend
if [ "$FRONTEND_ONLY" = false ]; then
    info "启动后端 (Go, 端口 8080)..."
    go run cmd/server/main.go &
    PIDS+=($!)
fi

# Start Frontend
if [ "$BACKEND_ONLY" = false ]; then
    info "启动前端 (Next.js, 端口 3000)..."
    (cd frontend && $PKG_MANAGER run dev) &
    PIDS+=($!)
fi

# ── 5. Print Summary ────────────────────────────────────────
sleep 2
echo ""
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}${BOLD}✅ 开发环境已启动!${NC}"
echo ""
if [ "$FRONTEND_ONLY" = false ]; then
    echo -e "  📡 Backend API:    ${CYAN}http://localhost:8080${NC}"
    echo -e "  💚 Health Check:   ${CYAN}http://localhost:8080/health${NC}"
fi
if [ "$BACKEND_ONLY" = false ]; then
    echo -e "  🌐 Frontend:       ${CYAN}http://localhost:3000${NC}"
fi
if [ "$FRONTEND_ONLY" = false ]; then
    echo -e "  🐘 PostgreSQL:     ${CYAN}localhost:5433${NC}"
    echo -e "  🔵 Neo4j Web UI:   ${CYAN}http://localhost:7475${NC}"
    echo -e "  🔴 Redis:          ${CYAN}localhost:6381${NC}"
    if [ "$WEKNORA" = true ]; then
        echo -e "  📚 WeKnora API:    ${CYAN}http://localhost:9380${NC}"
    fi
fi
echo ""
echo -e "  📋 测试账号:  ${YELLOW}13800000001 / admin123${NC} (管理员)"
echo -e "                ${YELLOW}13800000010 / teacher123${NC} (教师)"
echo -e "                ${YELLOW}13800000100 / student123${NC} (学生)"
echo ""
echo -e "  按 ${BOLD}Ctrl+C${NC} 停止所有服务"
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Wait for all background processes
wait
