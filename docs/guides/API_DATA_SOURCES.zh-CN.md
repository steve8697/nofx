# AI 数据源说明文档

## 📋 目录
1. [AI 如何读取数据](#ai-如何读取数据)
2. [当前数据源](#当前数据源)
3. [数据流图](#数据流图)
4. [如何添加新的数据源](#如何添加新的数据源)
5. [配置外部 API](#配置外部-api)
6. [示例：添加链上数据](#示例添加链上数据)

---

## AI 如何读取数据

### 核心流程

AI **不直接调用 API**，而是通过以下流程获取数据：

```
1. 交易循环启动
   ↓
2. decision/engine.go → fetchMarketDataForContext()
   ↓
3. market.Get(symbol) → 获取市场数据
   ↓
4. 构建 User Prompt（包含所有市场数据）
   ↓
5. 发送给 AI（通过 MCP Client）
   ↓
6. AI 分析数据并返回决策
```

### 关键代码位置

```109:134:decision/engine.go
// GetFullDecisionWithCustomPrompt 获取AI的完整交易决策（支持自定义prompt和模板选择）
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient *mcp.Client, customPrompt string, overrideBase bool, templateName string) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt := buildSystemPromptWithCustom(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, customPrompt, overrideBase, templateName)
	userPrompt := buildUserPrompt(ctx)

	// 3. 调用AI API（使用 system + user prompt）
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	// 4. 解析AI响应
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage)
	if err != nil {
		return decision, fmt.Errorf("解析AI响应失败: %w", err)
	}

	decision.Timestamp = time.Now()
	decision.SystemPrompt = systemPrompt // 保存系统prompt
	decision.UserPrompt = userPrompt     // 保存输入prompt
	return decision, nil
}
```

---

## 当前数据源

### 1. **Binance API**（主要数据源）

**位置**: `market/api_client.go`, `market/data.go`

**获取的数据**:
- ✅ K线数据（3分钟、4小时）
- ✅ 当前价格
- ✅ Open Interest（持仓量）
- ✅ Funding Rate（资金费率）
- ✅ 技术指标（EMA、MACD、RSI、ATR）

**API 端点**:
```go
baseURL = "https://fapi.binance.com"
- /fapi/v1/exchangeInfo      // 交易所信息
- /fapi/v1/klines            // K线数据
- /fapi/v1/ticker/price       // 当前价格
- /fapi/v1/openInterest       // 持仓量
- /fapi/v1/premiumIndex       // 资金费率
```

**WebSocket 实时更新**: `market/websocket_client.go`
- 实时接收 K线更新
- 自动重连机制

### 2. **币种池 API**（可选）

**位置**: `pool/coin_pool.go`

**功能**: 获取 AI500 排名靠前的币种列表

**配置方式**:
```json
// config.json
{
  "coin_pool_api_url": "https://your-api.com/coin-pool"
}
```

**API 响应格式**:
```json
{
  "success": true,
  "data": {
    "coins": [
      {
        "pair": "BTCUSDT",
        "score": 95.5,
        "is_available": true
      }
    ]
  }
}
```

**使用场景**: 
- 当 `use_default_coins: false` 时，从 API 获取候选币种
- 如果 API 失败，会回退到 `default_coins` 列表

### 3. **OI Top API**（可选）

**位置**: `pool/coin_pool.go`

**功能**: 获取持仓量增长 Top20 的币种数据

**配置方式**:
```json
// config.json
{
  "oi_top_api_url": "https://your-api.com/oi-top"
}
```

**API 响应格式**:
```json
{
  "success": true,
  "data": {
    "positions": [
      {
        "symbol": "BTCUSDT",
        "rank": 1,
        "current_oi": 1000000,
        "oi_delta": 50000,
        "oi_delta_percent": 5.0,
        "oi_delta_value": 5000000,
        "price_delta_percent": 2.5,
        "net_long": 600000,
        "net_short": 400000
      }
    ],
    "count": 20,
    "exchange": "binance",
    "time_range": "1h"
  }
}
```

**使用场景**:
- 识别持仓量快速增长的币种（可能有大资金入场）
- 作为候选币种的补充信号

---

## 数据流图

```
┌─────────────────────────────────────────────────────────────┐
│                    AI 决策流程                                │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
        ┌───────────────────────────────────────┐
        │  decision/engine.go                   │
        │  GetFullDecisionWithCustomPrompt()    │
        └───────────────────────────────────────┘
                            │
                            ▼
        ┌───────────────────────────────────────┐
        │  fetchMarketDataForContext()          │
        │  - 收集需要数据的币种                  │
        │  - 调用 market.Get() 获取数据          │
        │  - 调用 pool.GetOITopPositions()       │
        └───────────────────────────────────────┘
                            │
        ┌───────────────────┴───────────────────┐
        │                                       │
        ▼                                       ▼
┌───────────────────┐              ┌──────────────────────┐
│  market.Get()     │              │  pool.GetOITopPositions()│
│                   │              │                        │
│  ┌─────────────┐ │              │  ┌──────────────────┐ │
│  │ Binance API │ │              │  │ OI Top API       │ │
│  │ - K线数据   │ │              │  │ (可选)           │ │
│  │ - OI数据    │ │              │  │                  │ │
│  │ - Funding   │ │              │  │                  │ │
│  └─────────────┘ │              │  └──────────────────┘ │
│                  │              │                        │
│  ┌─────────────┐ │              └──────────────────────┘
│  │ WebSocket  │ │
│  │ 实时更新    │ │
│  └─────────────┘ │
└───────────────────┘
        │
        ▼
┌───────────────────────────────────────┐
│  buildUserPrompt()                    │
│  - 格式化所有市场数据                  │
│  - 包含持仓、候选币种、技术指标        │
└───────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────┐
│  mcpClient.CallWithMessages()        │
│  - 发送 System Prompt + User Prompt  │
│  - 接收 AI 决策                       │
└───────────────────────────────────────┘
```

---

## 如何添加新的数据源

### 方法 1: 在 `market/data.go` 中添加新数据

**步骤**:

1. **在 `Data` 结构体中添加新字段**:

```go
// market/types.go
type Data struct {
    Symbol            string
    CurrentPrice      float64
    // ... 现有字段 ...
    
    // 新增字段
    OnChainData      *OnChainData  // 链上数据
    SocialSentiment  float64       // 社交情绪
}
```

2. **在 `market.Get()` 中获取新数据**:

```go
// market/data.go
func Get(symbol string) (*Data, error) {
    // ... 现有代码 ...
    
    // 获取链上数据（新增）
    onChainData, _ := getOnChainData(symbol)
    
    // 获取社交情绪（新增）
    sentiment, _ := getSocialSentiment(symbol)
    
    return &Data{
        // ... 现有字段 ...
        OnChainData:     onChainData,
        SocialSentiment: sentiment,
    }, nil
}
```

3. **实现数据获取函数**:

```go
// market/data.go
func getOnChainData(symbol string) (*OnChainData, error) {
    // 调用链上数据 API
    url := fmt.Sprintf("https://your-onchain-api.com/data/%s", symbol)
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // 解析响应
    var data OnChainData
    json.NewDecoder(resp.Body).Decode(&data)
    return &data, nil
}
```

4. **在 `Format()` 函数中格式化输出**:

```go
// market/data.go
func Format(data *Data) string {
    var sb strings.Builder
    // ... 现有格式化代码 ...
    
    // 添加新数据的格式化
    if data.OnChainData != nil {
        sb.WriteString(fmt.Sprintf("链上数据: 大户持仓变化 %.2f%%\n", 
            data.OnChainData.WhalePositionChange))
    }
    
    sb.WriteString(fmt.Sprintf("社交情绪: %.2f\n", data.SocialSentiment))
    
    return sb.String()
}
```

### 方法 2: 在 `buildUserPrompt()` 中添加新数据

**步骤**:

1. **在 `Context` 结构体中添加新字段**:

```go
// decision/engine.go
type Context struct {
    // ... 现有字段 ...
    
    // 新增字段
    OnChainDataMap map[string]*OnChainData
    SocialDataMap  map[string]*SocialData
}
```

2. **在 `fetchMarketDataForContext()` 中获取新数据**:

```go
// decision/engine.go
func fetchMarketDataForContext(ctx *Context) error {
    // ... 现有代码 ...
    
    // 获取链上数据
    ctx.OnChainDataMap = make(map[string]*OnChainData)
    for symbol := range symbolSet {
        onChainData, err := fetchOnChainData(symbol)
        if err == nil {
            ctx.OnChainDataMap[symbol] = onChainData
        }
    }
    
    return nil
}
```

3. **在 `buildUserPrompt()` 中添加到 Prompt**:

```go
// decision/engine.go
func buildUserPrompt(ctx *Context) string {
    var sb strings.Builder
    // ... 现有代码 ...
    
    // 添加链上数据
    if onChainData, ok := ctx.OnChainDataMap[coin.Symbol]; ok {
        sb.WriteString(fmt.Sprintf("### 链上数据\n"))
        sb.WriteString(fmt.Sprintf("大户持仓变化: %.2f%%\n", 
            onChainData.WhalePositionChange))
        sb.WriteString(fmt.Sprintf("交易所流入: %.2f USDT\n", 
            onChainData.ExchangeInflow))
    }
    
    return sb.String()
}
```

---

## 配置外部 API

### 1. 配置币种池 API

**在 `config.json` 中添加**:

```json
{
  "use_default_coins": false,
  "coin_pool_api_url": "https://your-api.com/coin-pool"
}
```

**或在数据库中设置**:

```sql
INSERT INTO system_config (key, value) 
VALUES ('coin_pool_api_url', 'https://your-api.com/coin-pool');
```

### 2. 配置 OI Top API

**在 `config.json` 中添加**:

```json
{
  "oi_top_api_url": "https://your-api.com/oi-top"
}
```

**或在数据库中设置**:

```sql
INSERT INTO system_config (key, value) 
VALUES ('oi_top_api_url', 'https://your-api.com/oi-top');
```

### 3. 重启服务

配置更改后需要重启服务：

```bash
docker compose restart aetheris
```

---

## 示例：添加链上数据

### 场景：添加大户持仓变化数据

### 步骤 1: 定义数据结构

```go
// market/types.go
type OnChainData struct {
    WhalePositionChange float64 // 大户持仓变化百分比
    ExchangeInflow      float64 // 交易所流入（USDT）
    ExchangeOutflow     float64 // 交易所流出（USDT）
    ActiveAddresses     int64   // 活跃地址数
}
```

### 步骤 2: 实现数据获取

```go
// market/onchain.go (新建文件)
package market

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

var onChainAPIClient = &http.Client{
    Timeout: 10 * time.Second,
}

func getOnChainData(symbol string) (*OnChainData, error) {
    // 示例：调用 Glassnode API
    url := fmt.Sprintf("https://api.glassnode.com/v1/metrics/addresses/active_count?a=%s&i=1h", 
        getBaseAsset(symbol))
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    // 添加 API Key（如果需要）
    req.Header.Set("X-API-KEY", "your-api-key")
    
    resp, err := onChainAPIClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var data OnChainData
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, err
    }
    
    return &data, nil
}

func getBaseAsset(symbol string) string {
    // BTCUSDT -> BTC
    if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
        return symbol[:len(symbol)-4]
    }
    return symbol
}
```

### 步骤 3: 集成到现有流程

```go
// market/data.go
func Get(symbol string) (*Data, error) {
    // ... 现有代码 ...
    
    // 获取链上数据
    onChainData, _ := getOnChainData(symbol)
    
    return &Data{
        // ... 现有字段 ...
        OnChainData: onChainData,
    }, nil
}
```

### 步骤 4: 格式化输出

```go
// market/data.go
func Format(data *Data) string {
    var sb strings.Builder
    // ... 现有格式化代码 ...
    
    if data.OnChainData != nil {
        sb.WriteString("\n### 链上数据\n\n")
        sb.WriteString(fmt.Sprintf("大户持仓变化: %.2f%%\n", 
            data.OnChainData.WhalePositionChange))
        sb.WriteString(fmt.Sprintf("交易所净流入: %.2f USDT\n", 
            data.OnChainData.ExchangeInflow - data.OnChainData.ExchangeOutflow))
        sb.WriteString(fmt.Sprintf("活跃地址数: %d\n\n", 
            data.OnChainData.ActiveAddresses))
    }
    
    return sb.String()
}
```

### 步骤 5: 测试

```bash
# 重启服务
docker compose restart aetheris

# 查看日志，确认数据获取成功
docker compose logs aetheris | grep "链上数据"
```

---

## 常见链上数据 API 推荐

### 1. **Glassnode** (https://glassnode.com)
- 活跃地址数
- 大户持仓分布
- 交易所流入/流出

### 2. **Santiment** (https://santiment.net)
- 社交情绪
- 开发活动
- 大户交易

### 3. **CryptoQuant** (https://cryptoquant.com)
- 交易所储备
- 矿工持仓
- 资金费率

### 4. **DefiLlama** (https://defillama.com)
- TVL 数据
- 协议数据
- 跨链数据

### 5. **CoinGecko API** (https://www.coingecko.com/api)
- 价格数据
- 市值数据
- 社区数据

---

## 注意事项

### 1. **API 限制**
- 注意 API 调用频率限制
- 实现缓存机制避免重复请求
- 使用重试机制处理临时失败

### 2. **错误处理**
- 新数据源失败不应影响主流程
- 使用默认值或跳过该数据源

### 3. **性能考虑**
- 避免在每次决策时都调用慢速 API
- 考虑使用后台任务定期更新数据
- 使用缓存减少 API 调用

### 4. **数据质量**
- 验证 API 响应格式
- 处理数据缺失情况
- 过滤异常值

---

## 总结

- ✅ **AI 不直接调用 API**，而是通过系统获取数据后构建到 Prompt 中
- ✅ **当前主要数据源**: Binance API（K线、OI、Funding Rate）
- ✅ **可选数据源**: 币种池 API、OI Top API
- ✅ **可以扩展**: 在 `market/data.go` 或 `decision/engine.go` 中添加新数据源
- ✅ **配置方式**: 通过 `config.json` 或数据库配置 API URL

如需添加新的数据源，请参考上述示例代码，或联系开发团队获取支持。

