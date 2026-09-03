# 🏗️ AETHERIS 架构文档

**语言:** [English](README.md) | [中文](README.zh-CN.md)

为希望了解 AETHERIS 内部实现的开发者提供的技术文档。

---

## 📋 概述

AETHERIS 是一个全栈 AI 交易平台：
- **后端：** Go (Gin 框架, SQLite)
- **前端：** React/TypeScript (Vite, TailwindCSS)
- **架构：** 微服务启发的模块化设计

---

## 📁 项目结构

```
aetheris/
├── main.go                          # 程序入口（多交易员管理器）
├── config.json                      # ~~多交易员配置~~ (现通过Web界面)
├── trading.db                       # SQLite 数据库（交易员、模型、交易所）
│
├── api/                            # HTTP API 服务
│   └── server.go                   # Gin 框架，RESTful API
│
├── trader/                         # 交易核心
│   ├── auto_trader.go              # 自动交易主控制器
│   ├── interface.go                # 统一交易员接口
│   ├── binance_futures.go          # Binance API 包装器
│   ├── hyperliquid_trader.go       # Hyperliquid DEX 包装器
│   └── aster_trader.go             # Aster DEX 包装器
│
├── manager/                        # 多交易员管理
│   └── trader_manager.go           # 管理多个交易员实例
│
├── config/                         # 配置与数据库
│   └── database.go                 # SQLite 操作和模式
│
├── auth/                           # 认证
│   └── jwt.go                      # JWT token 管理 & 2FA
│
├── mcp/                            # Model Context Protocol - AI 通信
│   └── client.go                   # AI API 客户端（DeepSeek/Qwen/自定义）
│
├── decision/                       # AI 决策引擎
│   ├── engine.go                   # 带历史反馈的决策逻辑
│   └── prompt_manager.go           # 提示词模板系统
│
├── market/                         # 市场数据获取
│   └── data.go                     # 市场数据与技术指标（TA-Lib）
│   └── api_client.go               # 行情获取 Api接口
│   └── websocket_client.go         # 行情获取 Websocket接口
│   └── combined_streams.go         # 行情获取 组合流式(单链接订阅多个币种)
│   └── monitor.go                  # 行情数据缓存
│   └── types.go                    # market结构体
│
├── pool/                           # 币种池管理
│   └── coin_pool.go                # AI500 + OI Top 合并池
│
├── logger/                         # 日志系统
│   └── decision_logger.go          # 决策记录 + 性能分析
│
├── decision_logs/                  # 决策日志存储（JSON 文件）
│   ├── {trader_id}/                # 每个交易员的日志
│   └── {timestamp}.json            # 单个决策
│
└── web/                            # React 前端
    ├── src/
    │   ├── components/             # React 组件
    │   │   ├── EquityChart.tsx     # 权益曲线图表
    │   │   ├── ComparisonChart.tsx # 多 AI 对比图表
    │   │   └── CompetitionPage.tsx # 竞赛排行榜
    │   ├── lib/api.ts              # API 调用包装器
    │   ├── types/index.ts          # TypeScript 类型
    │   ├── stores/                 # Zustand 状态管理
    │   ├── index.css               # Binance 风格样式
    │   └── App.tsx                 # 主应用
    ├── package.json                # 前端依赖
    └── vite.config.ts              # Vite 配置
```

---

## 🔧 核心依赖

### 后端 (Go)

| 包 | 用途 | 版本 |
|---------|---------|---------|
| `github.com/gin-gonic/gin` | HTTP API 框架 | v1.9+ |
| `github.com/adshao/go-binance/v2` | Binance API 客户端 | v2.4+ |
| `github.com/markcheno/go-talib` | 技术指标（TA-Lib） | 最新 |
| `github.com/mattn/go-sqlite3` | SQLite 数据库驱动 | v1.14+ |
| `github.com/golang-jwt/jwt/v5` | JWT 认证 | v5.0+ |
| `github.com/pquerna/otp` | 2FA/TOTP 支持 | v1.4+ |
| `golang.org/x/crypto` | 密码哈希（bcrypt） | 最新 |

### 前端 (React + TypeScript)

| 包 | 用途 | 版本 |
|---------|---------|---------|
| `react` + `react-dom` | UI 框架 | 18.3+ |
| `typescript` | 类型安全 | 5.8+ |
| `vite` | 构建工具 | 6.0+ |
| `recharts` | 图表（权益、对比） | 2.15+ |
| `swr` | 数据获取与缓存 | 2.2+ |
| `zustand` | 状态管理 | 5.0+ |
| `tailwindcss` | CSS 框架 | 3.4+ |
| `lucide-react` | 图标库 | 最新 |

---

## 🗂️ 系统架构

### 高层架构概览

```
┌──────────────────────────────────────────────────────────────────┐
│                         表现层                                   │
│    React SPA (Vite + TypeScript + TailwindCSS)                  │
│    - 竞赛仪表板、交易员管理 UI                                   │
│    - 实时图表 (Recharts)、认证页面                               │
└──────────────────────────────────────────────────────────────────┘
                             ↓ HTTP/JSON API
┌──────────────────────────────────────────────────────────────────┐
│                      API 层 (Gin Router)                         │
│    /api/traders, /api/status, /api/positions, /api/decisions    │
│    认证中间件 (JWT)、CORS 处理                                   │
└──────────────────────────────────────────────────────────────────┘
                             ↓
┌──────────────────────────────────────────────────────────────────┐
│                         业务逻辑层                               │
│  ┌──────────────────┐  ┌──────────────────┐  ┌────────────────┐ │
│  │ TraderManager    │  │ DecisionEngine   │  │ MarketData     │ │
│  │ - 多交易员       │  │ - AI 推理        │  │ - K线数据      │ │
│  │   编排           │  │ - 风险控制       │  │ - 技术指标     │ │
│  └──────────────────┘  └──────────────────┘  └────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
                             ↓
┌──────────────────────────────────────────────────────────────────┐
│                         数据访问层                               │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐     │
│  │ SQLite DB    │  │ 文件日志     │  │ 外部 APIs          │     │
│  │ - Traders    │  │ - Decisions  │  │ - Binance          │     │
│  │ - Models     │  │ - Performance│  │ - Hyperliquid      │     │
│  │ - Exchanges  │  │   analysis   │  │ - Aster            │     │
│  └──────────────┘  └──────────────┘  └────────────────────┘     │
└──────────────────────────────────────────────────────────────────┘
```

### 组件图

*（即将推出：详细的组件交互图）*

---

## 📚 核心模块

### 1. 交易系统 (`trader/`)

**用途：** 支持多交易所的交易执行层

**关键文件：**
- `auto_trader.go` - 主交易编排器（100+ 行）
- `interface.go` - 统一的交易员接口
- `binance_futures.go` - Binance API 包装器
- `hyperliquid_trader.go` - Hyperliquid DEX 包装器
- `aster_trader.go` - Aster DEX 包装器

**设计模式：** 基于接口抽象的策略模式

**示例：**
```go
type ExchangeClient interface {
    GetAccount() (*AccountInfo, error)
    GetPositions() ([]*Position, error)
    CreateOrder(*OrderParams) (*Order, error)
    // ... 更多方法
}
```

---

### 2. 决策引擎 (`decision/`)

**用途：** AI 驱动的交易决策制定

**关键文件：**
- `engine.go` - 带历史反馈的决策逻辑
- `prompt_manager.go` - AI 提示词模板系统

**特性：**
- 思维链推理
- 历史表现分析
- 风险感知决策
- 多模型支持（DeepSeek、Qwen、自定义）

**流程：**
```
历史数据 → 提示词生成 → AI API 调用 →
决策解析 → 风险验证 → 执行
```

---

### 3. 市场数据系统 (`market/`)

**用途：** 获取和分析市场数据

**关键文件：**
- `data.go` - 市场数据获取和技术指标

**特性：**
- 多时间周期 K线数据（3分钟、4小时）
- 通过 TA-Lib 计算技术指标：
  - EMA (20, 50)
  - MACD
  - RSI (7, 14)
  - ATR（波动率）
- 持仓量跟踪

---

### 4. 管理器 (`manager/`)

**用途：** 多交易员编排

**关键文件：**
- `trader_manager.go` - 管理多个交易员实例

**职责：**
- 交易员生命周期（启动、停止、重启）
- 资源分配
- 并发执行协调

---

### 5. API 服务器 (`api/`)

**用途：** 前端通信的 HTTP API

**关键文件：**
- `server.go` - Gin 框架 RESTful API

**端点：**
```
GET  /api/traders           # 列出所有交易员
POST /api/traders           # 创建交易员
POST /api/traders/:id/start # 启动交易员
GET  /api/status            # 系统状态
GET  /api/positions         # 当前持仓
GET  /api/decisions/latest  # 最近决策
```

---

### 6. 数据库层 (`config/`)

**用途：** SQLite 数据持久化

**关键文件：**
- `database.go` - 数据库操作和模式

**表：**
- `users` - 用户账户（支持 2FA）
- `ai_models` - AI 模型配置
- `exchanges` - 交易所凭证
- `traders` - 交易员实例
- `equity_history` - 绩效跟踪
- `system_config` - 应用程序设置

---

### 7. 认证 (`auth/`)

**用途：** 用户认证和授权

**特性：**
- 基于 JWT token 的认证
- 使用 TOTP 的 2FA（Google Authenticator）
- Bcrypt 密码哈希
- 管理员模式（简化的单用户模式）

---

## 🔄 请求流程示例

### 示例 1：创建新交易员

```
用户操作（前端）
    ↓
POST /api/traders
    ↓
API 服务器（认证中间件）
    ↓
Database.CreateTrader()
    ↓
TraderManager.StartTrader()
    ↓
AutoTrader.Run() → goroutine
    ↓
响应: {trader_id, status}
```

### 示例 2：交易决策周期

```
AutoTrader（每 3-5 分钟）
    ↓
1. FetchAccountStatus()
    ↓
2. GetOpenPositions()
    ↓
3. FetchMarketData() → TA-Lib 指标
    ↓
4. AnalyzeHistory() → 最近 20 笔交易
    ↓
5. GeneratePrompt() → 完整上下文
    ↓
6. CallAI() → DeepSeek/Qwen
    ↓
7. ParseDecision() → 结构化输出
    ↓
8. ValidateRisk() → 仓位限制、保证金
    ↓
9. ExecuteOrders() → 交易所 API
    ↓
10. LogDecision() → JSON 文件 + 数据库
```

---

## 📊 数据流

### 市场数据流

```
交易所 API
    ↓
market.FetchKlines()
    ↓
TA-Lib.Calculate(EMA, MACD, RSI)
    ↓
DecisionEngine（作为上下文）
    ↓
AI 模型（推理）
```

### 决策日志流

```
AI 响应
    ↓
decision_logger.go
    ↓
JSON 文件: decision_logs/{trader_id}/{timestamp}.json
    ↓
数据库: 绩效跟踪
    ↓
前端: /api/decisions/latest
```

---

## 🗄️ 数据库架构

### 核心表

**users**
```sql
- id (INTEGER PRIMARY KEY)
- username (TEXT UNIQUE)
- password_hash (TEXT)
- totp_secret (TEXT)
- is_admin (BOOLEAN)
- created_at (DATETIME)
```

**ai_models**
```sql
- id (INTEGER PRIMARY KEY)
- name (TEXT)
- model_type (TEXT) -- deepseek, qwen, custom
- api_key (TEXT)
- api_url (TEXT)
- enabled (BOOLEAN)
```

**traders**
```sql
- id (TEXT PRIMARY KEY)
- name (TEXT)
- ai_model_id (INTEGER FK)
- exchange_id (INTEGER FK)
- initial_balance (REAL)
- current_equity (REAL)
- status (TEXT) -- running, stopped
- created_at (DATETIME)
```

*（更多详情：database-schema.md - 即将推出）*

---

## 🔌 API 参考

### 认证

**POST /api/auth/login**
```json
请求: {
  "username": "string",
  "password": "string",
  "totp_code": "string" // 可选
}

响应: {
  "token": "jwt_token",
  "user": {...}
}
```

### 交易员管理

**GET /api/traders**
```json
响应: {
  "traders": [
    {
      "id": "string",
      "name": "string",
      "status": "running|stopped",
      "balance": 1000.0,
      "roi": 5.2
    }
  ]
}
```

*（完整 API 参考：api-reference.md - 即将推出）*

---

## 🧪 测试架构

### 当前状态
- ⚠️ 尚无单元测试
- ⚠️ 仅手动测试
- ⚠️ 测试网验证

### 计划的测试策略

**单元测试（优先级 1）**
```
trader/binance_futures_test.go
- 模拟 API 响应
- 测试精度处理
- 验证订单构造
```

**集成测试（优先级 2）**
```
- 端到端交易流程（测试网）
- 多交易员场景
- 数据库操作
```

**前端测试（优先级 3）**
```
- 组件测试（Vitest + React Testing Library）
- API 集成测试
- E2E 测试（Playwright）
```

*（测试指南：testing-guide.md - 即将推出）*

---

## 🔧 开发工具

### 构建与运行

```bash
# 后端
go build -o aetheris
./aetheris

# 前端
cd web
npm run dev

# Docker
docker compose up --build
```

### 代码质量

```bash
# 格式化 Go 代码
go fmt ./...

# Lint（如果配置）
golangci-lint run

# TypeScript 类型检查
cd web && npm run build
```

---

## 📈 性能考虑

### 后端
- **并发：** 每个交易员在独立的 goroutine 中运行
- **数据库：** SQLite（适用于 <100 个交易员）
- **API 速率限制：** 按交易所处理
- **内存：** 每个交易员 ~50-100MB

### 前端
- **数据获取：** SWR，5-10 秒轮询
- **状态：** Zustand（轻量级）
- **包大小：** ~500KB（gzipped）

---

## 🔮 未来架构计划

### 计划改进

1. **微服务拆分**（如需扩展）
   - 独立的决策引擎服务
   - 市场数据服务
   - 执行服务

2. **数据库迁移**
   - 生产环境使用 Mysql (>100 个交易员）
   - Redis 缓存

3. **事件驱动架构**
   - WebSocket 实时更新
   - 消息队列（RabbitMQ/NATS）

4. **Kubernetes 部署**
   - Helm charts
   - 自动扩展
   - 高可用性

---

## 🆘 开发者资源

**想要贡献？**
- 阅读[贡献指南](../../CONTRIBUTING.md)
- 查看[开放问题](https://github.com/tinkle-community/aetheris/issues)
- 加入 [Telegram 社区](https://t.me/aetheris_dev_community)

**需要澄清？**
- 开启 [GitHub 讨论](https://github.com/tinkle-community/aetheris/discussions)
- 在 Telegram 提问

---

## 📚 相关文档

- [快速开始](../getting-started/README.zh-CN.md) - 设置和部署
- [贡献指南](../../CONTRIBUTING.md) - 如何贡献
- [社区](../community/README.md) - 悬赏和认可

---

[← 返回文档首页](../README.md)
