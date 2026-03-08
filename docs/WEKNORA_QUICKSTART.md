# WeKnora 快速启动指南

## 5 分钟快速启动

### 1. 启动所有服务 (1 分钟)

```bash
# 启动基础设施 + WeKnora
docker compose -f deployments/docker-compose.yml --profile weknora up -d

# 等待服务就绪
sleep 10
```

### 2. 同步用户 (30 秒)

```bash
# 同步 Hanfledge 用户到 WeKnora
go run scripts/sync_weknora_users.go
```

输出示例：
```
✓ Updated WeKnora user: 13800000001 (admin=true)
✓ Updated WeKnora user: 13800000010 (admin=true)
...
✅ Synced 13 users to WeKnora
```

### 3. 配置 Hanfledge (1 分钟)

```bash
# 获取 WeKnora token
WK_TOKEN=$(curl -s http://localhost:9380/api/v1/auth/login -X POST \
  -H "Content-Type: application/json" \
  -d '{"email":"13800000010@hanfledge.local","password":"teacher123"}' | jq -r '.token')

# 添加到 .env
echo "WEKNORA_ENABLED=true" >> .env
echo "WEKNORA_BASE_URL=http://localhost:9380/api/v1" >> .env
echo "WEKNORA_API_KEY=$WK_TOKEN" >> .env
```

### 4. 启动 Hanfledge 后端 (30 秒)

```bash
# 启动后端
go run cmd/server/main.go

# 验证 WeKnora 集成
# 查看日志应该显示: "WeKnora integration enabled"
```

### 5. 测试集成 (1 分钟)

```bash
# 运行自动化测试
bash scripts/test_weknora_integration.sh
```

或手动测试：

```bash
# 登录
TOKEN=$(curl -s http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000010","password":"teacher123"}' | jq -r '.token')

# 列出知识库
curl -s http://localhost:8080/api/v1/weknora/knowledge-bases \
  -H "Authorization: Bearer $TOKEN" | jq .
```

## 完成！🎉

现在你可以：
- ✅ 在 Hanfledge 中浏览 WeKnora 知识库
- ✅ 将知识库绑定到课程
- ✅ 搜索知识库内容
- ✅ 管理知识库绑定关系

## 常用命令

### 查看服务状态
```bash
docker ps | grep hanfledge
```

### 重启 WeKnora
```bash
docker compose -f deployments/docker-compose.yml --profile weknora restart weknora
```

### 查看 WeKnora 日志
```bash
docker logs hanfledge-weknora -f
```

### 重新同步用户
```bash
go run scripts/sync_weknora_users.go
```

### 停止所有服务
```bash
docker compose -f deployments/docker-compose.yml --profile weknora down
```

## 故障排查

### WeKnora 连接失败
```bash
# 重新生成 token
WK_TOKEN=$(curl -s http://localhost:9380/api/v1/auth/login -X POST \
  -H "Content-Type: application/json" \
  -d '{"email":"13800000010@hanfledge.local","password":"teacher123"}' | jq -r '.token')

# 更新 .env
sed -i "s/^WEKNORA_API_KEY=.*/WEKNORA_API_KEY=$WK_TOKEN/" .env

# 重启后端
pkill -f "go run.*server"
go run cmd/server/main.go
```

### 用户无法登录 WeKnora
```bash
# 重新同步用户
go run scripts/sync_weknora_users.go

# 检查用户
docker exec hanfledge-postgres psql -U hanfledge -d weknora \
  -c "SELECT username, tenant_id, can_access_all_tenants FROM users LIMIT 5;"
```

## 更多信息

- 📖 [完整集成文档](WEKNORA_INTEGRATION.md)
- 📊 [完成总结](WEKNORA_COMPLETION_SUMMARY.md)
- 🧪 [测试脚本](../scripts/test_weknora_integration.sh)
- 🔄 [用户同步脚本](../scripts/sync_weknora_users.go)
