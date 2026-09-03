package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"aetheris/config"
	"aetheris/market"
	"aetheris/trader"
	"aetheris/utils"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CompetitionCache 竞赛数据缓存
type CompetitionCache struct {
	data      map[string]interface{}
	timestamp time.Time
	mu        sync.RWMutex
}

// TraderManager 管理多个trader实例
type TraderManager struct {
	traders          map[string]*trader.AutoTrader // key: trader ID
	autoStart        map[string]bool               // DB is_running at load time
	competitionCache *CompetitionCache
	mu               sync.RWMutex
	wsMonitor        *market.WSMonitor
	promptRules      *config.PromptRules
}

// NewTraderManager 创建trader管理器
func NewTraderManager(wsMonitor *market.WSMonitor, promptRules *config.PromptRules) *TraderManager {
	return &TraderManager{
		traders:   make(map[string]*trader.AutoTrader),
		autoStart: make(map[string]bool),
		competitionCache: &CompetitionCache{
			data: make(map[string]interface{}),
		},
		wsMonitor:   wsMonitor,
		promptRules: promptRules,
	}
}

// LoadTradersFromDatabase 从数据库加载所有交易员到内存
func (tm *TraderManager) LoadTradersFromDatabase(database *config.Database) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 获取所有用户
	userIDs, err := database.GetAllUsers()
	if err != nil {
		return fmt.Errorf("获取用户列表失败: %w", err)
	}

	log.Printf("📋 发现 %d 个用户，开始加载所有交易员配置...", len(userIDs))

	var allTraders []*config.TraderRecord
	for _, userID := range userIDs {
		// 获取每个用户的交易员
		traders, err := database.GetTraders(userID)
		if err != nil {
			log.Printf("⚠️ 获取用户 %s 的交易员失败: %v", userID, err)
			continue
		}
		log.Printf("📋 用户 %s: %d 个交易员", userID, len(traders))
		allTraders = append(allTraders, traders...)
	}

	log.Printf("📋 总共加载 %d 个交易员配置", len(allTraders))

	// 获取系统配置（不包含信号源，信号源现在为用户级别）
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// 解析配置
	maxDailyLoss := 10.0 // 默认值
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // 默认值
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // 默认值
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// 解析默认币种列表
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ 解析默认币种配置失败: %v，使用空列表", err)
			defaultCoins = []string{}
		}
	}

	// 为每个交易员获取AI模型和交易所配置
	for _, traderCfg := range allTraders {
		// 获取AI模型配置（使用交易员所属的用户ID）
		aiModels, err := database.GetAIModels(traderCfg.UserID)
		if err != nil {
			log.Printf("⚠️  获取AI模型配置失败: %v", err)
			continue
		}

		var aiModelCfg *config.AIModelConfig
		// 优先精确匹配 model.ID（新版逻辑）
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		// 如果没有精确匹配，尝试匹配 provider（兼容旧数据）
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					log.Printf("⚠️  交易员 %s 使用旧版 provider 匹配: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
					break
				}
			}
		}

		if aiModelCfg == nil {
			log.Printf("⚠️  交易员 %s 的AI模型 %s 不存在，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			log.Printf("⚠️  交易员 %s 的AI模型 %s 未启用，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// 获取交易所配置（使用交易员所属的用户ID）
		exchanges, err := database.GetExchanges(traderCfg.UserID)
		if err != nil {
			log.Printf("⚠️  获取交易所配置失败: %v", err)
			continue
		}

		var exchangeCfg *config.ExchangeConfig
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			log.Printf("⚠️  交易员 %s 的交易所 %s 不存在，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			log.Printf("⚠️  交易员 %s 的交易所 %s 未启用，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// 获取用户信号源配置
		var coinPoolURL, oiTopURL string
		if userSignalSource, err := database.GetUserSignalSource(traderCfg.UserID); err == nil {
			coinPoolURL = userSignalSource.CoinPoolURL
			oiTopURL = userSignalSource.OITopURL

			// 🛡️ SSRF 防护: 验证 URL 安全性
			// 允许的白名单 Host (如果需要的话，目前传 nil 表示仅拦截私有 IP)
			var allowedHosts map[string]bool = nil

			// 验证 CoinPool URL
			if coinPoolURL != "" {
				if _, err := utils.ValidateURL(coinPoolURL, allowedHosts); err != nil {
					log.Printf("⚠️ [SSRF Block] 用户 %s 的 CoinPool URL 不安全，已拦截: %v", traderCfg.UserID, err)
					coinPoolURL = "" // 清空不安全 URL
				}
			}

			// 验证 OITop URL
			if oiTopURL != "" {
				if _, err := utils.ValidateURL(oiTopURL, allowedHosts); err != nil {
					log.Printf("⚠️ [SSRF Block] 用户 %s 的 OITop URL 不安全，已拦截: %v", traderCfg.UserID, err)
					oiTopURL = "" // 清空不安全 URL
				}
			}
		} else {
			// 如果用户没有配置信号源，使用空字符串
			log.Printf("🔍 用户 %s 暂未配置信号源", traderCfg.UserID)
		}

		// 添加到TraderManager
		err = tm.addTraderFromDB(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, oiTopURL, maxDailyLoss, maxDrawdown, stopTradingMinutes, defaultCoins, database, traderCfg.UserID)
		if err != nil {
			log.Printf("❌ 添加交易员 %s 失败: %v", traderCfg.Name, err)
			continue
		}
	}

	log.Printf("✓ 成功加载 %d 个交易员到内存", len(tm.traders))
	return nil
}

// addTraderFromDB 内部方法：从配置添加交易员（不加锁，因为调用方已加锁）
func (tm *TraderManager) addTraderFromDB(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, _ string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", traderCfg.ID)
	}

	// 处理交易币种列表
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// 解析逗号分隔的交易币种列表
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// 如果没有指定交易币种，使用默认币种
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// 根据交易员配置决定是否使用信号源
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ 交易员 %s 启用 COIN POOL 信号源: %s", traderCfg.Name, coinPoolURL)
	}

	// 构建AutoTraderConfig
	ai := aiModelCfg.ClientSettings()

	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               ai.Kind,
		Exchange:              exchangeCfg.ID, // 使用exchange ID
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        effectiveCoinPoolURL,
		UseQwen:               ai.Kind == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		CustomAPIURL:          ai.BaseURL,
		CustomModelName:       ai.ModelName,
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		BTCETHLeverage:        traderCfg.BTCETHLeverage,
		AltcoinLeverage:       traderCfg.AltcoinLeverage,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:         traderCfg.IsCrossMargin,
		DefaultCoins:          defaultCoins,
		TradingCoins:          tradingCoins,
		SystemPromptTemplate:  traderCfg.SystemPromptTemplate, // 系统提示词模板
		PromptRules:           tm.promptRules,                 // 提示词规则配置
	}

	// 根据交易所类型设置API密钥
	switch exchangeCfg.ID {
	case "binance":
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	case "hyperliquid":
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid用APIKey存储private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	case "aster":
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
		traderConfig.AsterTestnet = exchangeCfg.Testnet
	}

	applyResolvedAIKey(&traderConfig, ai)

	// 创建trader实例
	log.Printf("📊 加载交易员 %s: 数据库中的初始余额=%.2f USDT", traderCfg.Name, traderCfg.InitialBalance)
	// 创建AutoTrader实例
	at, err := trader.NewAutoTrader(traderConfig, database, userID, tm.wsMonitor, &utils.RealTimeProvider{}, nil)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}
	log.Printf("✓ 交易员 %s 已创建，内存中的初始余额=%.2f USDT", traderCfg.Name, at.GetInitialBalance())

	// 设置自定义prompt（如果有）
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ 已设置自定义交易策略prompt (覆盖基础prompt)")
		} else {
			log.Printf("✓ 已设置自定义交易策略prompt (补充基础prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	tm.autoStart[traderCfg.ID] = traderCfg.IsRunning
	log.Printf("✓ Trader '%s' (%s + %s) 已加载到内存，初始余额=%.2f USDT, 自动启动标记=%v", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID, at.GetInitialBalance(), traderCfg.IsRunning)
	return nil
}

// AddTrader 从数据库配置添加trader (移除旧版兼容性)

// AddTraderFromDB 从数据库配置添加trader
func (tm *TraderManager) AddTraderFromDB(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", traderCfg.ID)
	}

	// 处理交易币种列表
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// 解析逗号分隔的交易币种列表
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// 如果没有指定交易币种，使用默认币种
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// 根据交易员配置决定是否使用信号源
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ 交易员 %s 启用 COIN POOL 信号源: %s", traderCfg.Name, coinPoolURL)
	}

	ai := aiModelCfg.ClientSettings()
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               ai.Kind,
		Exchange:              exchangeCfg.ID, // 使用exchange ID
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        effectiveCoinPoolURL,
		UseQwen:               ai.Kind == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		CustomAPIURL:          ai.BaseURL,
		CustomModelName:       ai.ModelName,
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		BTCETHLeverage:        traderCfg.BTCETHLeverage,
		AltcoinLeverage:       traderCfg.AltcoinLeverage,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:         traderCfg.IsCrossMargin,
		DefaultCoins:          defaultCoins,
		TradingCoins:          tradingCoins,
		PromptRules:           tm.promptRules, // 提示词规则配置
	}

	// 根据交易所类型设置API密钥
	switch exchangeCfg.ID {
	case "binance":
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	case "hyperliquid":
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid用APIKey存储private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	case "aster":
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	}

	applyResolvedAIKey(&traderConfig, ai)

	// 创建trader实例
	at, err := trader.NewAutoTrader(traderConfig, database, userID, tm.wsMonitor, &utils.RealTimeProvider{}, nil)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}

	// 设置自定义prompt（如果有）
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ 已设置自定义交易策略prompt (覆盖基础prompt)")
		} else {
			log.Printf("✓ 已设置自定义交易策略prompt (补充基础prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	tm.autoStart[traderCfg.ID] = traderCfg.IsRunning
	log.Printf("✓ Trader '%s' (%s + %s) 已添加", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

// GetTrader 获取指定ID的trader
func (tm *TraderManager) GetTrader(id string) (*trader.AutoTrader, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, exists := tm.traders[id]
	if !exists {
		return nil, fmt.Errorf("trader ID '%s' 不存在", id)
	}
	return t, nil
}

// RemoveTrader 从内存中删除指定ID的trader（用于重新加载配置）
func (tm *TraderManager) RemoveTrader(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if existingTrader, exists := tm.traders[id]; exists {
		// 如果正在运行，先停止
		status := existingTrader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			existingTrader.Stop()
			time.Sleep(200 * time.Millisecond)
		}

		// 🔧 Resource Cleanup: Unregister symbols from WSMonitor
		if tm.wsMonitor != nil {
			tradingCoins := existingTrader.GetTradingCoins()
			for _, symbol := range tradingCoins {
				tm.wsMonitor.UnregisterSymbol(symbol)
			}
			log.Printf("🧹 已清理 %d 个币种的 WebSocket 资源", len(tradingCoins))
		}

		delete(tm.traders, id)
		log.Printf("🗑️ 已从内存中删除交易员: %s", id)
	}
}

// GetAllTraders 获取所有trader
func (tm *TraderManager) GetAllTraders() map[string]*trader.AutoTrader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*trader.AutoTrader)
	for id, t := range tm.traders {
		result[id] = t
	}
	return result
}

// GetTraderIDs 获取所有trader ID列表
func (tm *TraderManager) GetTraderIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := make([]string, 0, len(tm.traders))
	for id := range tm.traders {
		ids = append(ids, id)
	}
	return ids
}

// StartAll 只启动数据库标记为运行中的 trader，避免进程重启误开实盘。
func (tm *TraderManager) StartAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	started := 0
	for id, t := range tm.traders {
		if !tm.autoStart[id] {
			log.Printf("⏭  %s 未标记运行，跳过自动启动", t.GetName())
			continue
		}
		started++
		go func(traderID string, at *trader.AutoTrader) {
			log.Printf("▶️  启动 %s...", at.GetName())
			if err := at.Run(); err != nil {
				log.Printf("❌ %s 运行错误: %v", at.GetName(), err)
			}
		}(id, t)
	}
	if started == 0 {
		log.Println("ℹ️  启动时未自动拉起任何交易员。实盘请用 API POST /api/traders/:id/start")
	}
}

// StopAll 停止记忆体中的 trader，不改数据库 is_running。
// Docker / SIGTERM 走这里；用户点停止必须走 API UpdateTraderStatus(false)。
func (tm *TraderManager) StopAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("⏹  停止所有Trader（仅进程内，保留 is_running 以便容器重启后恢复）...")
	for _, t := range tm.traders {
		t.Stop()
	}
}

// GetComparisonData 获取对比数据
func (tm *TraderManager) GetComparisonData() (map[string]interface{}, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	comparison := make(map[string]interface{})
	traders := make([]map[string]interface{}, 0, len(tm.traders))

	for _, t := range tm.traders {
		account, err := t.GetAccountInfo()
		if err != nil {
			continue
		}

		status := t.GetStatus()

		traders = append(traders, map[string]interface{}{
			"trader_id":       t.GetID(),
			"trader_name":     t.GetName(),
			"ai_model":        t.GetAIModel(),
			"exchange":        t.GetExchange(),
			"total_equity":    account["total_equity"],
			"total_pnl":       account["total_pnl"],
			"total_pnl_pct":   account["total_pnl_pct"],
			"position_count":  account["position_count"],
			"margin_used_pct": account["margin_used_pct"],
			"call_count":      status["call_count"],
			"is_running":      status["is_running"],
		})
	}

	comparison["traders"] = traders
	comparison["count"] = len(traders)

	return comparison, nil
}

// GetCompetitionData 获取竞赛数据（全平台所有交易员）
func (tm *TraderManager) GetCompetitionData() (map[string]interface{}, error) {
	// 检查缓存是否有效（30秒内）
	tm.competitionCache.mu.RLock()
	if time.Since(tm.competitionCache.timestamp) < 30*time.Second && len(tm.competitionCache.data) > 0 {
		// 返回缓存数据
		cachedData := make(map[string]interface{})
		for k, v := range tm.competitionCache.data {
			cachedData[k] = v
		}
		tm.competitionCache.mu.RUnlock()
		log.Printf("📋 返回竞赛数据缓存 (缓存时间: %.1fs)", time.Since(tm.competitionCache.timestamp).Seconds())
		return cachedData, nil
	}
	tm.competitionCache.mu.RUnlock()

	tm.mu.RLock()

	// 获取所有交易员列表
	allTraders := make([]*trader.AutoTrader, 0, len(tm.traders))
	for _, t := range tm.traders {
		allTraders = append(allTraders, t)
	}
	tm.mu.RUnlock()

	log.Printf("🔄 重新获取竞赛数据，交易员数量: %d", len(allTraders))

	// 并发获取交易员数据
	traders := tm.getConcurrentTraderData(allTraders)

	// 按收益率排序（降序）
	sort.Slice(traders, func(i, j int) bool {
		pnlPctI, okI := traders[i]["total_pnl_pct"].(float64)
		pnlPctJ, okJ := traders[j]["total_pnl_pct"].(float64)
		if !okI {
			pnlPctI = 0
		}
		if !okJ {
			pnlPctJ = 0
		}
		return pnlPctI > pnlPctJ
	})

	// 限制返回前50名
	totalCount := len(traders)
	limit := 50
	if len(traders) > limit {
		traders = traders[:limit]
	}

	comparison := make(map[string]interface{})
	comparison["traders"] = traders
	comparison["count"] = len(traders)
	comparison["total_count"] = totalCount // 总交易员数量

	// 更新缓存
	tm.competitionCache.mu.Lock()
	tm.competitionCache.data = comparison
	tm.competitionCache.timestamp = time.Now()
	tm.competitionCache.mu.Unlock()

	return comparison, nil
}

// getConcurrentTraderData 并发获取多个交易员的数据
func (tm *TraderManager) getConcurrentTraderData(traders []*trader.AutoTrader) []map[string]interface{} {
	type traderResult struct {
		index int
		data  map[string]interface{}
	}

	// 创建结果通道
	resultChan := make(chan traderResult, len(traders))

	// 并发获取每个交易员的数据
	for i, t := range traders {
		go func(index int, trader *trader.AutoTrader) {
			// 设置单个交易员的超时时间为3秒
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			// 使用通道来实现超时控制
			accountChan := make(chan map[string]interface{}, 1)
			errorChan := make(chan error, 1)

			go func() {
				account, err := trader.GetAccountInfo()
				if err != nil {
					errorChan <- err
				} else {
					accountChan <- account
				}
			}()

			status := trader.GetStatus()
			var traderData map[string]interface{}

			select {
			case account := <-accountChan:
				// 成功获取账户信息
				traderData = map[string]interface{}{
					"trader_id":       trader.GetID(),
					"trader_name":     trader.GetName(),
					"ai_model":        trader.GetAIModel(),
					"exchange":        trader.GetExchange(),
					"total_equity":    account["total_equity"],
					"total_pnl":       account["total_pnl"],
					"total_pnl_pct":   account["total_pnl_pct"],
					"position_count":  account["position_count"],
					"margin_used_pct": account["margin_used_pct"],
					"is_running":      status["is_running"],
				}
			case err := <-errorChan:
				// 获取账户信息失败
				log.Printf("⚠️ 获取交易员 %s 账户信息失败: %v", trader.GetID(), err)
				traderData = map[string]interface{}{
					"trader_id":       trader.GetID(),
					"trader_name":     trader.GetName(),
					"ai_model":        trader.GetAIModel(),
					"exchange":        trader.GetExchange(),
					"total_equity":    0.0,
					"total_pnl":       0.0,
					"total_pnl_pct":   0.0,
					"position_count":  0,
					"margin_used_pct": 0.0,
					"is_running":      status["is_running"],
					"error":           "账户数据获取失败",
				}
			case <-ctx.Done():
				// 超时
				log.Printf("⏰ 获取交易员 %s 账户信息超时", trader.GetID())
				traderData = map[string]interface{}{
					"trader_id":       trader.GetID(),
					"trader_name":     trader.GetName(),
					"ai_model":        trader.GetAIModel(),
					"exchange":        trader.GetExchange(),
					"total_equity":    0.0,
					"total_pnl":       0.0,
					"total_pnl_pct":   0.0,
					"position_count":  0,
					"margin_used_pct": 0.0,
					"is_running":      status["is_running"],
					"error":           "获取超时",
				}
			}

			resultChan <- traderResult{index: index, data: traderData}
		}(i, t)
	}

	// 收集所有结果
	results := make([]map[string]interface{}, len(traders))
	for i := 0; i < len(traders); i++ {
		result := <-resultChan
		results[result.index] = result.data
	}

	return results
}

// GetTopTradersData 获取前5名交易员数据（用于表现对比）
func (tm *TraderManager) GetTopTradersData() (map[string]interface{}, error) {
	// 复用竞赛数据缓存，因为前5名是从全部数据中筛选出来的
	competitionData, err := tm.GetCompetitionData()
	if err != nil {
		return nil, err
	}

	// 从竞赛数据中提取前5名
	allTraders, ok := competitionData["traders"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("竞赛数据格式错误")
	}

	// 限制返回前5名
	limit := 5
	topTraders := allTraders
	if len(allTraders) > limit {
		topTraders = allTraders[:limit]
	}

	result := map[string]interface{}{
		"traders": topTraders,
		"count":   len(topTraders),
	}

	return result, nil
}

// LoadUserTraders 为特定用户加载交易员到内存
func (tm *TraderManager) LoadUserTraders(database *config.Database, userID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 获取指定用户的所有交易员
	traders, err := database.GetTraders(userID)
	if err != nil {
		return fmt.Errorf("获取用户 %s 的交易员列表失败: %w", userID, err)
	}

	log.Printf("📋 为用户 %s 加载交易员配置: %d 个", userID, len(traders))

	// 获取系统配置（不包含信号源，信号源现在为用户级别）
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// 获取用户信号源配置
	var coinPoolURL, oiTopURL string
	if userSignalSource, err := database.GetUserSignalSource(userID); err == nil {
		coinPoolURL = userSignalSource.CoinPoolURL
		oiTopURL = userSignalSource.OITopURL

		// 🛡️ SSRF 防护: 验证 URL 安全性
		var allowedHosts map[string]bool = nil

		// 验证 CoinPool URL
		if coinPoolURL != "" {
			if _, err := utils.ValidateURL(coinPoolURL, allowedHosts); err != nil {
				log.Printf("⚠️ [SSRF Block] 用户 %s 的 CoinPool URL 不安全，已拦截: %v", userID, err)
				coinPoolURL = "" // 清空不安全 URL
			}
		}

		// 验证 OITop URL
		if oiTopURL != "" {
			if _, err := utils.ValidateURL(oiTopURL, allowedHosts); err != nil {
				log.Printf("⚠️ [SSRF Block] 用户 %s 的 OITop URL 不安全，已拦截: %v", userID, err)
				oiTopURL = "" // 清空不安全 URL
			}
		}

		log.Printf("📡 加载用户 %s 的信号源配置: COIN POOL=%s, OI TOP=%s", userID, coinPoolURL, oiTopURL)
	} else {
		log.Printf("🔍 用户 %s 暂未配置信号源", userID)
	}

	// 解析配置
	maxDailyLoss := 10.0 // 默认值
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // 默认值
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // 默认值
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// 解析默认币种列表
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ 解析默认币种配置失败: %v，使用空列表", err)
			defaultCoins = []string{}
		}
	}

	// 获取AI模型配置（使用该用户的配置）
	aiModels, err := database.GetAIModels(userID)
	if err != nil {
		return fmt.Errorf("获取用户 %s 的AI模型配置失败: %w", userID, err)
	}

	// 获取交易所配置（使用该用户的配置）
	exchanges, err := database.GetExchanges(userID)
	if err != nil {
		return fmt.Errorf("获取用户 %s 的交易所配置失败: %w", userID, err)
	}

	// 为每个交易员获取AI模型和交易所配置
	for _, traderCfg := range traders {
		// 1. 解析AI模型配置
		var aiModelCfg *config.AIModelConfig
		// 优先精确匹配 model.ID（新版逻辑）
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		// 如果没有精确匹配，尝试匹配 provider（兼容旧数据）
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					log.Printf("⚠️  交易员 %s 使用旧版 provider 匹配: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
					break
				}
			}
		}

		if aiModelCfg == nil {
			log.Printf("⚠️ 交易员 %s 的AI模型 %s 不存在，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			log.Printf("⚠️ 交易员 %s 的AI模型 %s 未启用，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// 2. 解析交易所配置
		var exchangeCfg *config.ExchangeConfig
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			log.Printf("⚠️ 交易员 %s 的交易所 %s 不存在，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			log.Printf("⚠️ 交易员 %s 的交易所 %s 未启用，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// 3. 检查是否已经加载过这个交易员
		wasRunning := false
		if existingTrader, exists := tm.traders[traderCfg.ID]; exists {
			needRecreate := false
			reason := ""

			// 获取现有配置
			currentConfig := existingTrader.GetConfig()
			status := existingTrader.GetStatus()

			// A. 检查扫描间隔是否改变
			currentInterval := time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute
			if scanIntervalStr, ok := status["scan_interval"].(string); ok {
				if parsedInterval, err := time.ParseDuration(scanIntervalStr); err == nil {
					if parsedInterval != currentInterval {
						needRecreate = true
						reason = fmt.Sprintf("扫描间隔改变: %v → %v", parsedInterval, currentInterval)
					}
				}
			}

			// B. 检查AI模型配置是否改变（比较解析后的 URL/模型/密钥，含 env_key）
			if !needRecreate {
				ai := aiModelCfg.ClientSettings()
				if currentConfig.CustomModelName != ai.ModelName {
					needRecreate = true
					reason = fmt.Sprintf("AI模型名称改变: %s → %s", currentConfig.CustomModelName, ai.ModelName)
				}
				if !needRecreate && currentConfig.CustomAPIURL != ai.BaseURL {
					needRecreate = true
					reason = "AI API URL改变"
				}
				if !needRecreate {
					oldKey := currentConfig.CustomAPIKey
					switch ai.Kind {
					case "qwen":
						oldKey = currentConfig.QwenKey
					case "deepseek":
						oldKey = currentConfig.DeepSeekKey
					}
					if ai.APIKey != oldKey {
						needRecreate = true
						reason = "AI API Key改变"
					}
				}
			}

			// C. 检查交易所配置是否改变
			if !needRecreate {
				// 简单检查API Key是否改变
				if exchangeCfg.ID == "binance" && currentConfig.BinanceAPIKey != exchangeCfg.APIKey {
					needRecreate = true
					reason = "交易所API Key改变"
				} else if exchangeCfg.ID == "hyperliquid" && (currentConfig.HyperliquidPrivateKey != exchangeCfg.APIKey || currentConfig.HyperliquidWalletAddr != exchangeCfg.HyperliquidWalletAddr || currentConfig.HyperliquidTestnet != exchangeCfg.Testnet) {
					needRecreate = true
					reason = "Hyperliquid配置改變"
				} else if exchangeCfg.ID == "aster" && (currentConfig.AsterUser != exchangeCfg.AsterUser || currentConfig.AsterSigner != exchangeCfg.AsterSigner || currentConfig.AsterPrivateKey != exchangeCfg.AsterPrivateKey || currentConfig.AsterTestnet != exchangeCfg.Testnet) {
					needRecreate = true
					reason = "Aster配置改變"
				}
			}

			// D. 执行重创建或更新
			if needRecreate {
				log.Printf("⚠️ 交易员 %s 配置变更 (%s)，需要重新创建实例", traderCfg.Name, reason)
				if isRunning, ok := status["is_running"].(bool); ok && isRunning {
					wasRunning = true
					log.Printf("⏹ 停止交易员 %s 以应用新配置", traderCfg.Name)
					existingTrader.Stop()
					time.Sleep(200 * time.Millisecond)
				}
				delete(tm.traders, traderCfg.ID)
				log.Printf("🗑️ 已删除旧的交易员实例，准备重新创建")
			} else {
				// 配置未改变，只更新初始余额
				currentInitialBalance := existingTrader.GetInitialBalance()

				// 只有在差異很大時（可能是用戶手動修改了配置）才更新
				if math.Abs(currentInitialBalance-traderCfg.InitialBalance) > 1.0 {
					log.Printf("⚠️ 交易员 %s 初始余额差异较大（内存: %.2f, 数据库: %.2f）",
						traderCfg.Name, currentInitialBalance, traderCfg.InitialBalance)
					log.Printf("⚠️ 可能是用户手动修改了配置，更新内存中的初始余额")
					existingTrader.UpdateInitialBalance(traderCfg.InitialBalance)
				} else {
					log.Printf("ℹ️ 交易员 %s 已加载，配置未变更", traderCfg.Name)
				}
				continue
			}
		}

		err = tm.loadSingleTrader(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, oiTopURL, maxDailyLoss, maxDrawdown, stopTradingMinutes, defaultCoins, database, userID)
		if err != nil {
			log.Printf("⚠️ 加载交易员 %s 失败: %v", traderCfg.Name, err)
			continue
		}
		// 仅恢复「刚才正在跑」的实例。is_running=0 的 trader 不会因为改模型被拉起。
		if wasRunning {
			if at, ok := tm.traders[traderCfg.ID]; ok {
				go func(auto *trader.AutoTrader) {
					log.Printf("▶️  模型配置已更新，恢复运行 %s", auto.GetName())
					if runErr := auto.Run(); runErr != nil {
						log.Printf("❌ %s 运行错误: %v", auto.GetName(), runErr)
					}
				}(at)
			}
		}
	}

	return nil
}

// loadSingleTrader 加载单个交易员（从现有代码提取的公共逻辑）
func (tm *TraderManager) loadSingleTrader(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, _ string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	// 处理交易币种列表
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// 解析逗号分隔的交易币种列表
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// 如果没有指定交易币种，使用默认币种
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// 根据交易员配置决定是否使用信号源
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ 交易员 %s 启用 COIN POOL 信号源: %s", traderCfg.Name, coinPoolURL)
	}

	ai := aiModelCfg.ClientSettings()
	traderConfig := trader.AutoTraderConfig{
		ID:                   traderCfg.ID,
		Name:                 traderCfg.Name,
		AIModel:              ai.Kind,
		Exchange:             exchangeCfg.ID, // 使用exchange ID
		InitialBalance:       traderCfg.InitialBalance,
		BTCETHLeverage:       traderCfg.BTCETHLeverage,
		AltcoinLeverage:      traderCfg.AltcoinLeverage,
		ScanInterval:         time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		CoinPoolAPIURL:       effectiveCoinPoolURL,
		CustomAPIURL:         ai.BaseURL,
		CustomModelName:      ai.ModelName,
		UseQwen:              ai.Kind == "qwen",
		MaxDailyLoss:         maxDailyLoss,
		MaxDrawdown:          maxDrawdown,
		StopTradingTime:      time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:        traderCfg.IsCrossMargin,
		DefaultCoins:         defaultCoins,
		TradingCoins:         tradingCoins,
		SystemPromptTemplate: traderCfg.SystemPromptTemplate, // 系统提示词模板
		HyperliquidTestnet:   exchangeCfg.Testnet,            // Hyperliquid测试网
		PromptRules:          tm.promptRules,                 // 提示词规则配置
	}

	// 根据交易所类型设置API密钥
	switch exchangeCfg.ID {
	case "binance":
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	case "hyperliquid":
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid用APIKey存储private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	case "aster":
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	}

	applyResolvedAIKey(&traderConfig, ai)

	// 创建trader实例
	at, err := trader.NewAutoTrader(traderConfig, database, userID, tm.wsMonitor, &utils.RealTimeProvider{}, nil)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}

	// 设置自定义prompt（如果有）
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ 已设置自定义交易策略prompt (覆盖基础prompt)")
		} else {
			log.Printf("✓ 已设置自定义交易策略prompt (补充基础prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	tm.autoStart[traderCfg.ID] = traderCfg.IsRunning
	log.Printf("✓ Trader '%s' (%s + %s) 已为用户加载到内存", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

func applyResolvedAIKey(cfg *trader.AutoTraderConfig, ai config.AIClientSettings) {
	switch ai.Kind {
	case "qwen":
		cfg.QwenKey = ai.APIKey
		cfg.UseQwen = true
	case "deepseek":
		cfg.DeepSeekKey = ai.APIKey
	default:
		cfg.CustomAPIKey = ai.APIKey
	}
	cfg.CustomAPIURL = ai.BaseURL
	cfg.CustomModelName = ai.ModelName
	cfg.AIModel = ai.Kind
}
