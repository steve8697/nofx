# 免费API集成说明

## ✅ 已集成的免费API

### 1. **CoinGecko API** (已集成)

**状态**: ✅ 已实现并测试通过

**特点**:
- ✅ **完全免费**，不需要API key
- ✅ **公开可用**，无需注册
- ✅ 数据丰富：价格、市值、24h变化、交易量、排名

**获取的数据**:
- 📊 24小时价格变化百分比
- 🏆 市值排名
- 💰 市值（USD）
- 📈 24小时交易量（USD）
- 💵 24小时价格变化（USD）

**API限制**:
- 每分钟最多30次调用
- 每月最多10,000次调用（免费版）

**使用方式**:
- 自动集成，无需配置
- 在每次获取市场数据时自动调用
- 如果API失败，不影响主流程（优雅降级）

**代码位置**:
- `market/data.go`: `getCoinGeckoData()`
- `market/types.go`: `CoinGeckoData` 结构体

**示例输出**:
```
CoinGecko Market Data (免费API):

Market Cap Rank: #1
24h Price Change: +1.67% (+1728.08 USDT)
Market Cap: 2063.50B USDT
24h Volume: 52376.53M USDT
```

---

## 📋 数据流

```
AI决策流程
    ↓
market.Get(symbol)
    ↓
getCoinGeckoData(symbol)  ← 自动调用免费API
    ↓
格式化数据到Prompt
    ↓
AI分析并返回决策
```

---

## 🔧 技术实现

### 数据结构

```go
type CoinGeckoData struct {
    PriceChange24h    float64 // 24小时价格变化百分比
    MarketCapRank     int     // 市值排名
    MarketCap         float64 // 市值（USD）
    TotalVolume24h    float64 // 24小时交易量（USD）
    PriceChange24hUSD float64 // 24小时价格变化（USD）
}
```

### 币种映射

系统自动将交易对符号（如 `BTCUSDT`）转换为CoinGecko ID（如 `bitcoin`）。

支持的币种映射包括：
- BTC, ETH, SOL, BNB, XRP, DOGE, ADA
- LTC, ZEC, ZEN, AVAX, ARB, MINA, ZK
- DOT, TON, KAS, FIL, LINK, DASH, ICP
- AAVE, RENDER, POL, XPLUS, POLYX
- 以及更多...

### 错误处理

- ✅ API失败不影响主流程
- ✅ 自动跳过无法获取数据的币种
- ✅ 优雅降级（如果CoinGecko数据不可用，仍可使用Binance数据）

---

## 🚀 如何使用

### 自动使用

**无需任何配置！** 系统会自动：
1. 在获取市场数据时调用CoinGecko API
2. 将数据格式化到AI的Prompt中
3. AI可以基于这些数据做出更好的决策

### 查看数据

CoinGecko数据会自动包含在AI的决策Prompt中，你可以在：
- AI决策日志中查看
- Web界面的交易详情中查看

---

## 📊 API调用统计

### 调用频率

- **每次决策周期**: 每个候选币种调用1-2次
- **典型场景**: 30个候选币种 = 30-60次调用/周期
- **限制**: 每分钟最多30次（免费版）

### 优化建议

1. **缓存机制**: 考虑添加缓存，减少重复调用
2. **批量查询**: CoinGecko支持批量查询多个币种
3. **选择性调用**: 只对主要币种获取排名数据

---

## 🔮 未来可扩展的免费API

### 候选API列表

1. **DefiLlama API** (https://defillama.com)
   - TVL数据
   - 协议数据
   - 完全免费，无需API key

2. **CoinMarketCap API** (https://coinmarketcap.com/api)
   - 需要API key（免费层可用）
   - 每月10,000次调用

3. **CryptoCompare API** (https://www.cryptocompare.com)
   - 免费层可用
   - 需要API key

4. **Tokenview API** (https://tokenview.io)
   - 链上数据
   - 每天80,000次免费调用
   - 需要注册获取API key

---

## ⚠️ 注意事项

### API限制

1. **频率限制**: CoinGecko免费版每分钟30次
2. **月度限制**: 每月10,000次调用
3. **超限处理**: 如果超限，系统会自动跳过CoinGecko数据

### 最佳实践

1. **不要频繁调用**: 系统已优化，只在需要时调用
2. **监控使用量**: 注意API调用频率
3. **备用方案**: 如果CoinGecko不可用，Binance数据仍可用

---

## 🧪 测试

运行测试脚本验证API集成：

```bash
./test_coingecko_api.sh
```

如果看到JSON数据输出，说明API正常工作！

---

## 📝 更新日志

### 2024-11-10
- ✅ 集成CoinGecko免费API
- ✅ 添加24h价格变化、市值排名、交易量数据
- ✅ 实现自动币种映射
- ✅ 添加错误处理和优雅降级
- ✅ 测试通过

---

## 💡 总结

**已成功集成CoinGecko免费API！**

- ✅ 无需配置，自动使用
- ✅ 丰富的数据源（市值排名、24h变化等）
- ✅ 优雅的错误处理
- ✅ 不影响主流程

AI现在可以访问更多市场数据，做出更准确的交易决策！

