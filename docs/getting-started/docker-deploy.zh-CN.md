# 🐳 Docker 一键部署教程

本教程将指导你使用 Docker 快速部署 AETHERIS AI 交易竞赛系统。

## 📋 前置要求

在开始之前，请确保你的系统已安装：

- **Docker**: 版本 20.10 或更高
- **Docker Compose**: 版本 2.0 或更高

### 安装 Docker

> #### 提示：Docker Compose 版本说明
> 
> **新用户建议**：
> - **推荐使用 Docker Desktop**：自动包含最新 Docker Compose，无需单独安装
> - 安装简单，一键搞定，提供图形界面管理
> - 支持 macOS、Windows、部分 Linux 发行版
> 
> **旧用户提醒**：
> - **弃用独立 docker-compose**：不再推荐下载独立的 Docker Compose 二进制文件
> - **使用内置版**：Docker 20.10+ 自带 `docker compose` 命令（注意是空格）
> - 如果还在使用旧的 `docker-compose`，请升级到新语法

#### macOS / Windows
下载并安装 [Docker Desktop](https://www.docker.com/products/docker-desktop/)

**安装后验证：**
```bash
docker --version
docker compose --version  # 注意：使用空格，不再是连字符
```

#### Linux (Ubuntu/Debian)
**推荐方式：使用 Docker Desktop（如果可用）或 Docker CE**

```bash
# 安装 Docker (自动包含 compose)
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# 将当前用户加入 docker 组
sudo usermod -aG docker $USER
newgrp docker

# 验证安装（新命令）
docker --version
docker compose --version  # Docker 24+ 自带，无需单独安装
```

## 🚀 快速开始（3步完成部署）

### 第 1 步：准备配置文件

```bash
# 复制配置文件模板
cp config.json.example config.json

# 编辑配置文件，填入你的 API 密钥
nano config.json  # 或使用其他编辑器
```

**必须配置的字段：**
```json
{
  "use_default_coins": true,
  "api_server_port": 8081,
  "jwt_secret": "YOUR_JWT_SECRET_CHANGE_IN_PRODUCTION"  // ← 填入一个长随机字符串作为JWT密钥
}
```

> **⚠️ 重要安全提醒**：
> - `jwt_secret` 字段是用户认证系统的关键安全配置
> - **必须设置一个长度至少32位的随机字符串**
> - 在生产环境中，建议使用64位以上的随机字符串
> - 可以使用命令生成：`openssl rand -base64 64`

**配置说明：**
- 🔐 **用户认证**：系统现在支持用户注册登录，每个用户都有独立的AI模型和交易所配置
- 🚫 **移除traders配置**：不再需要在config.json中预配置交易员，用户可以通过Web界面创建
- 🔑 **JWT密钥**：用于保护用户会话安全，强烈建议在生产环境中设置复杂密钥

### 第 2 步：一键启动

```bash
# 构建并启动所有服务（首次运行）
docker compose up -d --build

# 后续启动（不重新构建）
docker compose up -d
```

**启动过程说明：**
- `--build`: 构建 Docker 镜像（首次运行或代码更新后使用）
- `-d`: 后台运行（detached mode）

### 第 3 步：访问系统

部署成功后，打开浏览器访问：

- **Web 界面**: http://localhost:3000
- **API 文档**: http://localhost:3636/api/health

## 📊 服务管理

### 查看运行状态
```bash
# 查看所有容器状态
docker compose ps

# 查看服务健康状态
docker compose ps --format json | jq
```

### 查看日志
```bash
# 查看所有服务日志
docker compose logs -f

# 只查看后端日志
docker compose logs -f backend

# 只查看前端日志
docker compose logs -f frontend

# 查看最近 100 行日志
docker compose logs --tail=100
```

### 停止服务
```bash
# 停止所有服务（保留数据）
docker compose stop

# 停止并删除容器（保留数据）
docker compose down

# 停止并删除容器和卷（清除所有数据）
docker compose down -v
```

### 重启服务
```bash
# 重启所有服务
docker compose restart

# 只重启后端
docker compose restart backend

# 只重启前端
docker compose restart frontend
```

### 更新服务
```bash
# 拉取最新代码
git pull

# 重新构建并重启
docker compose up -d --build
```

## 🔧 高级配置

### 修改端口

编辑 `docker-compose.yml`，修改端口映射：

```yaml
services:
  backend:
    ports:
      - "3636:3636"  # 改为 "你的端口:3636"

  frontend:
    ports:
      - "3000:80"    # 改为 "你的端口:80"
```

### 资源限制

在 `docker-compose.yml` 中添加资源限制：

```yaml
services:
  backend:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

### 环境变量

创建 `.env` 文件来管理环境变量：

```bash
# .env
TZ=Asia/Shanghai
BACKEND_PORT=3636
FRONTEND_PORT=3000
```

然后在 `docker-compose.yml` 中使用：

```yaml
services:
  backend:
    ports:
      - "${BACKEND_PORT}:3636"
```

## 📁 数据持久化

系统会自动持久化以下数据到本地目录：

- `./decision_logs/`: AI 决策日志
- `./coin_pool_cache/`: 币种池缓存
- `./config.json`: 配置文件（挂载）

**数据位置：**
```bash
# 查看数据目录
ls -la decision_logs/
ls -la coin_pool_cache/

# 备份数据
tar -czf backup_$(date +%Y%m%d).tar.gz decision_logs/ coin_pool_cache/ config.json

# 恢复数据
tar -xzf backup_20241029.tar.gz
```

## 🐛 故障排查

### 容器无法启动

```bash
# 查看详细错误信息
docker compose logs backend
docker compose logs frontend

# 检查容器状态
docker compose ps -a

# 重新构建（清除缓存）
docker compose build --no-cache
```

### 端口被占用

```bash
# 查找占用端口的进程
lsof -i :3636  # 后端端口
lsof -i :3000  # 前端端口

# 杀死占用端口的进程
kill -9 <PID>
```

### 配置文件未找到

```bash
# 确保 config.json 存在
ls -la config.json

# 如果不存在，复制模板
cp config.json.example config.json
```

### 健康检查失败

```bash
# 检查健康状态
docker inspect aetheris-backend | jq '.[0].State.Health'
docker inspect aetheris-frontend | jq '.[0].State.Health'

# 手动测试健康端点
curl http://localhost:3636/api/health
curl http://localhost:3000/health
```

### 前端无法连接后端

```bash
# 检查网络连接
docker compose exec frontend ping backend

# 检查后端服务是否正常
docker compose exec frontend wget -O- http://backend:3636/health
```

### 清理 Docker 资源

```bash
# 清理未使用的镜像
docker image prune -a

# 清理未使用的卷
docker volume prune

# 清理所有未使用的资源（慎用）
docker system prune -a --volumes
```

## 🔐 安全建议

1. **JWT密钥安全配置**
   ```bash
   # 生成强随机JWT密钥
   openssl rand -base64 64
   
   # 或者使用其他工具生成
   head -c 64 /dev/urandom | base64
   ```
   
   **JWT密钥要求：**
   - 长度至少32位，推荐64位以上
   - 使用随机生成的字符串
   - 在生产环境中绝不使用默认值
   - 定期更换（会使现有用户需要重新登录）

2. ~~**不要将 config.json 提交到 Git**~~
   ```bash
   # ~~确保 config.json 在 .gitignore 中~~
   # ~~echo "config.json" >> .gitignore~~
   ```
   
   *注意：现在使用trading.db数据库，请确保不提交敏感数据*

3. **使用环境变量存储敏感信息**
   ```yaml
   # docker-compose.yml
   services:
     backend:
       environment:
         - JWT_SECRET=${JWT_SECRET}
       # 用户的API密钥现在通过Web界面配置，不再需要环境变量
   ```

4. **限制 API 访问**
   ```yaml
   # 只允许本地访问
   services:
     backend:
       ports:
         - "127.0.0.1:3636:3636"
   ```

4. **定期更新镜像**
   ```bash
   docker compose pull
   docker compose up -d
   ```

## 🌐 生产环境部署

### 使用 Nginx 反向代理

```nginx
# /etc/nginx/sites-available/aetheris
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /api/ {
        proxy_pass http://localhost:3636/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### 配置 HTTPS (Let's Encrypt)

```bash
# 安装 Certbot
sudo apt-get install certbot python3-certbot-nginx

# 获取 SSL 证书
sudo certbot --nginx -d your-domain.com

# 自动续期
sudo certbot renew --dry-run
```

### 使用 Docker Swarm (集群部署)

```bash
# 初始化 Swarm
docker swarm init

# 部署堆栈
docker stack deploy -c docker-compose.yml aetheris

# 查看服务状态
docker stack services aetheris

# 扩展服务
docker service scale aetheris_backend=3
```

## 📈 监控与日志

### 日志管理

```bash
# 配置日志轮转（已在 docker-compose.yml 中配置）
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"

# 查看日志统计
docker compose logs --timestamps | wc -l
```

### 监控工具集成

可以集成 Prometheus + Grafana 进行监控：

```yaml
# docker-compose.yml (添加监控服务)
services:
  prometheus:
    image: prom/prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana
    ports:
      - "3001:3000"
```

## 🆘 获取帮助

- **GitHub Issues**: [提交问题](https://github.com/yourusername/open-aetheris/issues)
- **文档**: 查看 [README.md](README.md)
- **社区**: 加入我们的 Discord/Telegram 群组

## 📝 常用命令速查表

```bash
# 启动
docker compose up -d --build       # 构建并启动
docker compose up -d               # 启动（不重新构建）

# 停止
docker compose stop                # 停止服务
docker compose down                # 停止并删除容器
docker compose down -v             # 停止并删除容器和数据

# 查看
docker compose ps                  # 查看状态
docker compose logs -f             # 查看日志
docker compose top                 # 查看进程

# 重启
docker compose restart             # 重启所有服务
docker compose restart backend     # 重启后端

# 更新
git pull && docker compose up -d --build

# 清理
docker compose down -v             # 清除所有数据
docker system prune -a             # 清理 Docker 资源
```

---

🎉 恭喜！你已经成功部署了 AETHERIS AI 交易竞赛系统！

如有问题，请查看[故障排查](#-故障排查)部分或提交 Issue。
