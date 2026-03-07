# Hanfledge 2.0 快速启动指南

## 🚀 一键启动 (推荐)

```bash
# 启动完整开发环境 (基础设施 + 后端 + 前端)
bash scripts/dev.sh --seed
```

这个脚本会自动:
1. 启动 Docker 容器 (PostgreSQL, Neo4j, Redis)
2. 复制 `.env.example` 到 `.env` (如果不存在)
3. 安装前端依赖
4. 启动 Go 后端 (端口 8080)
5. 启动 Next.js 前端 (端口 3000)
6. 创建测试账号 (使用 `--seed` 参数)

---

## 📊 验证 2.0 优化效果

### 1. 验证并行化优化

```bash
# 运行性能基准测试
go run scripts/benchmark-parallel.go
```

**预期输出:**
```
🚀 Hanfledge 2.0 性能基准测试
================================

⏱️  [测试 1] Strategist 单独耗时
   ✓ Strategist: 150ms

⏱️  [测试 2] Designer 预加载 (Neo4j GetKPContext)
   ✓ Designer 预加载: 100ms

⏱️  [测试 3] 并行执行模拟
   ✓ Strategist (并行): 152ms
   ✓ Designer (并行): 98ms
   ✓ 总耗时 (并行): 152ms

================================
📈 性能对比
================================
串行总耗时: 250ms
并行总耗时: 152ms
性能提升:   39.2%

✅ 并行化优化效果显著!
```

### 2. 验证 VAD 功能

```bash
# 启动前端
cd frontend && npm run dev
```

**测试步骤:**
1. 访问 http://localhost:3000
2. 使用测试账号登录:
   - 学生账号: `13800000100` / `student123`
3. 进入任意学习会话
4. 点击麦克风按钮 🎤
5. 观察状态变化:
   - 🔴 红色脉冲 = 等待语音
   - 🟢 绿色脉冲 = 检测到语音

**浏览器控制台输出:**
```
[VAD] 检测到语音开始
[VAD] 检测到语音结束, 样本数: 48000
```

---

## 🔧 手动启动 (分步)

### 步骤 1: 启动基础设施

```bash
docker compose -f deployments/docker-compose.yml up -d
```

**验证:**
```bash
docker ps
# 应该看到 3 个容器: postgres, neo4j, redis
```

### 步骤 2: 配置环境变量

```bash
cp .env.example .env
# 编辑 .env 设置 JWT_SECRET
```

### 步骤 3: 启动后端

```bash
go run cmd/server/main.go
```

**验证:**
```bash
curl http://localhost:8080/health
# 应该返回: {"status":"ok"}
```

### 步骤 4: 创建测试数据

```bash
go run scripts/seed.go
```

**测试账号:**
| 角色 | 手机号 | 密码 |
|------|--------|------|
| 系统管理员 | 13800000001 | admin123 |
| 教师 | 13800000010 | teacher123 |
| 学生 | 13800000100 | student123 |

### 步骤 5: 启动前端

```bash
cd frontend
npm install
npm run dev
```

**访问:** http://localhost:3000

---

## 📝 观察性能指标

### 后端日志

启动后端后,观察日志输出:

```bash
go run cmd/server/main.go 2>&1 | grep -E "strategist done|designer preload"
```

**预期输出:**
```
[DEBUG] strategist done session_id=1 skill=socratic-questioning elapsed=148ms
[DEBUG] designer preload done nodes=12 elapsed=95ms
```

**关键指标:**
- `strategist done elapsed`: Strategist 耗时 (应该 ~150ms)
- `designer preload done elapsed`: Designer 预加载耗时 (应该 ~100ms)
- 两者应该几乎同时完成 (并行执行)

### 前端 VAD 日志

打开浏览器开发者工具 (F12),切换到 Console 标签:

```javascript
// 点击麦克风后应该看到:
[VAD] 检测到语音开始
[VAD] 检测到语音结束, 样本数: 48000
```

---

## 🐛 故障排查

### 问题 1: 后端编译失败

```bash
# 检查 Go 版本
go version
# 应该 >= 1.25

# 清理缓存
go clean -modcache
go mod download
```

### 问题 2: 前端编译失败

```bash
cd frontend
rm -rf node_modules package-lock.json
npm install
```

### 问题 3: VAD 不工作

**可能原因:**
1. 浏览器不支持 WebAssembly
   - 解决: 使用 Chrome/Edge/Firefox 最新版
2. 麦克风权限被拒绝
   - 解决: 在浏览器设置中允许麦克风访问
3. HTTPS 要求 (生产环境)
   - 解决: 使用 `localhost` 或配置 HTTPS

**禁用 VAD (回退到传统模式):**
```tsx
// frontend/src/app/student/session/[id]/components/SessionInput.tsx
<VoiceInput enableVAD={false} ... />
```

### 问题 4: 并行化未生效

**检查方法:**
```bash
# 查看日志中的 elapsed 时间
go run cmd/server/main.go 2>&1 | grep elapsed
```

**预期:** `strategist done` 和 `designer preload done` 的 elapsed 时间应该接近。

**如果不接近:** 可能是 Neo4j 查询慢,检查索引:
```cypher
// 在 Neo4j Browser 中执行
CREATE INDEX kp_id_index FOR (n:KnowledgePoint) ON (n.id)
```

---

## 📚 更多资源

- [完整优化报告](./OPTIMIZATION_SUMMARY.md)
- [优化实施日志](./OPTIMIZATION_LOG.md)
- [项目 README](../README.md)
- [API 文档](../docs/swagger.yaml)

---

## 🎯 下一步

1. **运行性能基准测试** 验证优化效果
2. **测试 VAD 功能** 确保语音输入正常
3. **查看优化报告** 了解技术细节
4. **规划下一阶段优化** (SLM Critic, GraphRAG, FSRS)

---

**需要帮助?** 查看 [故障排查](#-故障排查) 或提交 Issue。
