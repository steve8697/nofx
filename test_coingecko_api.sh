#!/bin/bash

# 测试CoinGecko API集成

echo "🧪 测试CoinGecko免费API集成"
echo "================================"
echo ""

# 测试1: 简单价格查询
echo "1. 测试简单价格查询 (BTC, ETH)"
curl -s "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum&vs_currencies=usd&include_24hr_change=true&include_market_cap=true&include_24hr_vol=true" | jq '.' 2>/dev/null || echo "需要安装jq: brew install jq"
echo ""

# 测试2: 市值排名查询
echo "2. 测试市值排名查询 (BTC)"
curl -s "https://api.coingecko.com/api/v3/coins/bitcoin?localization=false&tickers=false&market_data=true&community_data=false&developer_data=false" | jq '.market_data.market_cap_rank' 2>/dev/null || echo "需要安装jq"
echo ""

# 测试3: 测试多个币种
echo "3. 测试多个币种 (SOL, BNB, XRP)"
curl -s "https://api.coingecko.com/api/v3/simple/price?ids=solana,binancecoin,ripple&vs_currencies=usd&include_24hr_change=true&include_market_cap=true&include_24hr_vol=true" | jq '.' 2>/dev/null || echo "需要安装jq"
echo ""

echo "✅ 如果看到JSON数据，说明API正常工作！"
echo "📝 注意：CoinGecko免费API限制为每分钟30次调用"

