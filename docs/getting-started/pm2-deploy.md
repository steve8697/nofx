# NoFX Trading Bot - PM2 部署指南

使用 PM2 进行本地开发和生产部署的完整指南。

## 🚀 快速开始

### 1. 安装 PM2

```bash
npm install -g pm2
```

### 2. 一键启动

```bash
./pm2.sh start
```

就这么简单！前后端将自动启动。

---

## 📋 所有命令

### 服务管理

```bash
# 启动服务
./pm2.sh start

# 停止服务
./pm2.sh stop

# 重启服务
./pm2.sh restart

# 查看状态
./pm2.sh status

# 删除服务
./pm2.sh delete
```

### 日志查看

```bash
# 查看所有日志（实时）
./pm2.sh logs

# 只看后端日志
./pm2.sh logs backend

# 只看前端日志
./pm2.sh logs frontend
```

### 构建与编译

```bash
# 编译后端
./pm2.sh build

# 重新编译后端并重启
./pm2.sh rebuild
```

### 监控

```bash
# 打开 PM2 监控面板（实时CPU/内存）
./pm2.sh monitor
```

---

## 📊 访问地址

启动成功后：

- **前端 Web 界面**: http://localhost:3000
- **后端 API**: http://localhost:3636
- **健康检查**: http://localhost:3636/api/health

---

## 🔧 配置文件

### pm2.config.js

PM2 配置文件，定义了前后端的启动参数：

```javascript
const path = require('path');

module.exports = {
  apps: [
    {
      name: 'aetheris-backend',
      script: './aetheris',           // Go 二进制文件
      cwd: __dirname,             // 动态获取当前目录
      autorestart: true,
      max_memory_restart: '500M'
    },
    {
      name: 'aetheris-frontend',
      script: 'npm',
      args: 'run dev',            // Vite 开发服务器
      cwd: path.join(__dirname, 'web'), // 动态拼接路径
      autorestart: true,
      max_memory_restart: '300M'
    }
  ]
};
```

**修改配置后需要重启：**
```bash
./pm2.sh restart
```

---

## 📝 日志文件位置

- **后端日志**: `./logs/backend-error.log` 和 `./logs/backend-out.log`
- **前端日志**: `./web/logs/frontend-error.log` 和 `./web/logs/frontend-out.log`

---

## 🔄 开机自启动

设置 PM2 开机自启动：

```bash
# 1. 启动服务
./pm2.sh start

# 2. 保存当前进程列表
pm2 save

# 3. 生成启动脚本
pm2 startup

# 4. 按照提示执行命令（需要 sudo）
```

**取消开机自启动：**
```bash
pm2 unstartup
```

---

## 🛠️ 常见操作

### 修改代码后重启

**后端修改：**
```bash
./pm2.sh rebuild  # 自动编译并重启
```

**前端修改：**
```bash
./pm2.sh restart  # Vite 会自动热重载，无需重启
```

### 查看实时资源占用

```bash
./pm2.sh monitor
```

### 查看详细信息

```bash
pm2 info aetheris-backend   # 后端详情
pm2 info aetheris-frontend  # 前端详情
```

### 清空日志

```bash
pm2 flush
```

---

## 🐛 故障排查

### 服务启动失败

```bash
# 1. 查看详细错误
./pm2.sh logs

# 2. 检查端口占用
lsof -i :3636  # 后端端口
lsof -i :3000  # 前端端口

# 3. 手动编译测试
go build -o aetheris
./aetheris
```

### 后端无法启动

```bash
# ~~检查 config.json 是否存在~~
# ~~ls -l config.json~~

# 检查数据库文件是否存在
ls -l trading.db

# 检查权限
chmod +x aetheris

# 手动运行看报错
./aetheris
```

### 前端无法访问

```bash
# 检查 node_modules
cd web && npm install

# 手动启动测试
npm run dev
```

---

## 🎯 生产环境建议

### 1. 使用生产模式

修改 `pm2.config.js`：

```javascript
{
  name: 'aetheris-frontend',
  script: 'npm',
  args: 'run preview',  // 改为 preview（需先 npm run build）
  env: {
    NODE_ENV: 'production'
  }
}
```

### 2. 增加实例数（负载均衡）

```javascript
{
  name: 'aetheris-backend',
  script: './aetheris',
  instances: 2,  // 启动 2 个实例
  exec_mode: 'cluster'
}
```

### 3. 自动重启策略

```javascript
{
  autorestart: true,
  max_restarts: 10,
  min_uptime: '10s',
  max_memory_restart: '500M'
}
```

---

## 📦 与 Docker 部署的对比

| 特性 | PM2 部署 | Docker 部署 |
|------|---------|------------|
| 启动速度 | ⚡ 快 | 🐌 较慢 |
| 资源占用 | 💚 低 | 🟡 中等 |
| 隔离性 | 🟡 中等 | 💚 高 |
| 适用场景 | 开发/单机 | 生产/集群 |
| 配置复杂度 | 💚 简单 | 🟡 中等 |

**建议：**
- **开发环境**: 使用 `./pm2.sh`
- **生产环境**: 使用 `./start.sh` (Docker)

---

## 🆘 获取帮助

```bash
./pm2.sh help
```

或查看 PM2 官方文档：https://pm2.keymetrics.io/

---

## 📄 License

MIT
