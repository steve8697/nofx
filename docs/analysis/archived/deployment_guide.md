# 重新部署指南

## ✅ 需要重新部署

由于我们进行了代码重构（将 `engine.go` 拆分为多个文件），**需要重新编译和部署**才能生效。

---

## 🔍 部署方式检查

根据你的部署方式，选择对应的重新部署方法：

### 方式1：Docker Compose 部署（推荐）

**检查是否使用 Docker**：
```bash
docker ps | grep aetheris
```

**重新部署步骤**：
```bash
# 1. 停止现有服务
docker compose down

# 2. 重新构建镜像（包含新的代码）
docker compose build --no-cache

# 3. 启动服务
docker compose up -d

# 4. 查看日志确认启动成功
docker compose logs -f aetheris
```

**或者使用一键脚本**：
```bash
./start.sh stop
./start.sh start --build
```

---

### 方式2：PM2 部署

**检查是否使用 PM2**：
```bash
pm2 list | grep aetheris
```

**重新部署步骤**：
```bash
# 1. 重新编译 Go 程序
go build -o aetheris

# 2. 重启 PM2 服务
pm2 restart aetheris-backend

# 或者
pm2 reload pm2.config.js
```

---

### 方式3：直接运行（开发环境）

**重新部署步骤**：
```bash
# 1. 停止当前运行的程序（Ctrl+C）

# 2. 重新编译
go build -o aetheris

# 3. 运行
./aetheris
```

---

## ⚠️ 重要提示

### 1. 代码变更说明
- ✅ **功能完全一致**：只是代码结构优化，功能逻辑不变
- ✅ **无破坏性变更**：所有API和接口保持不变
- ✅ **向后兼容**：可以安全升级，不会影响现有数据

### 2. 部署前检查
- [x] 代码已保存
- [x] 无编译错误
- [x] 无 linter 错误
- [x] 配置文件正确（config.json）

### 3. 部署后验证
部署完成后，检查：
```bash
# 查看日志确认启动成功
docker compose logs aetheris | tail -20

# 或 PM2
pm2 logs aetheris-backend --lines 20

# 检查服务状态
curl http://localhost:3636/api/health
```

---

## 🎯 推荐操作

### 如果使用 Docker（最常见）

```bash
# 一键重新部署
cd /Users/huangjunyou/aetheris
docker compose down
docker compose build --no-cache
docker compose up -d

# 查看日志
docker compose logs -f aetheris
```

### 如果使用 PM2

```bash
# 重新编译并重启
cd /Users/huangjunyou/aetheris
go build -o aetheris
pm2 restart aetheris-backend
pm2 logs aetheris-backend
```

---

## ✅ 验证部署成功

部署后，检查以下内容：

1. **服务启动成功**
   - 日志中无错误信息
   - API 可以访问

2. **功能正常**
   - AI 可以正常生成决策
   - 决策可以被正确解析和验证
   - 交易可以正常执行

3. **日志正常**
   - 无编译错误
   - 无运行时错误

---

## 📝 总结

**✅ 需要重新部署**

- Docker: `docker compose down && docker compose build --no-cache && docker compose up -d`
- PM2: `go build && pm2 restart aetheris-backend`
- 直接运行: `go build && ./aetheris`

**代码重构已完成，功能保持不变，可以安全部署！**

