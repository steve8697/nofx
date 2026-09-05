package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"aetheris/auth"
	"aetheris/backtest"
	"aetheris/config"
	"aetheris/decision"
	"aetheris/manager"
	"aetheris/mcp"
	"aetheris/trader"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Server HTTP API服务器
type Server struct {
	router          *gin.Engine
	traderManager   *manager.TraderManager
	backtestManager *backtest.BacktestManager
	database        *config.Database
	port            int
	httpServer      *http.Server
	mu              sync.Mutex
}

// NewServer 创建API服务器
func NewServer(traderManager *manager.TraderManager, database *config.Database, port int, backtestManager *backtest.BacktestManager) *Server {
	// 设置为Release模式（减少日志输出）
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// 启用CORS
	router.Use(corsMiddleware())

	s := &Server{
		router:          router,
		traderManager:   traderManager,
		backtestManager: backtestManager,
		database:        database,
		port:            port,
	}

	// 设置路由
	s.setupRoutes()

	return s
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	// 从环境变量读取允许的域名，多个域名用逗号分隔
	// 生产环境建议设置 AETHERIS_CORS_ORIGINS=https://yourdomain.com
	allowedOrigins := os.Getenv("AETHERIS_CORS_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = os.Getenv("NOFX_CORS_ORIGINS")
	}
	if allowedOrigins == "" {
		allowedOrigins = "*" // 开发环境默认允许所有
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 检查是否允许该来源
		allowOrigin := "*"
		if allowedOrigins != "*" {
			allowOrigin = ""
			for _, allowed := range strings.Split(allowedOrigins, ",") {
				if strings.TrimSpace(allowed) == origin {
					allowOrigin = origin
					break
				}
			}
		}

		if allowOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// API路由组
	api := s.router.Group("/api")
	{
		// 健康检查
		api.Any("/health", s.handleHealth)

		// 管理员登录（管理员模式下使用，公共）
		api.POST("/admin-login", s.handleAdminLogin)

		// 系统支持的模型和交易所（无需认证）
		api.GET("/supported-models", s.handleGetSupportedModels)
		api.GET("/supported-exchanges", s.handleGetSupportedExchanges)
		api.GET("/model-providers", s.handleGetModelProviders)

		// 非管理员模式下的公开认证路由
		if !auth.IsAdminMode() {
			// 认证相关路由（无需认证）
			api.POST("/register", s.handleRegister)
			api.POST("/login", s.handleLogin)
			api.POST("/verify-otp", s.handleVerifyOTP)
			api.POST("/complete-registration", s.handleCompleteRegistration)
			api.POST("/reset-password", s.handleResetPassword)

		}

		// 系统配置（无需认证，用于前端判断是否管理员模式/注册是否开启）
		api.GET("/config", s.handleGetSystemConfig)

		// ✅ 修正：系統提示詞模板和公開數據應該在管理員模式下也可用
		// 這些 API 用於前端顯示和配置，不應該受管理員模式限制
		api.GET("/prompt-templates", s.handleGetPromptTemplates)
		api.GET("/prompt-templates/:name", s.handleGetPromptTemplate)
		api.GET("/traders", s.handlePublicTraderList)
		api.GET("/competition", s.handlePublicCompetition)
		api.GET("/top-traders", s.handleTopTraders)
		api.GET("/equity-history", s.handleEquityHistory)
		api.POST("/equity-history-batch", s.handleEquityHistoryBatch)
		api.GET("/traders/:id/public-config", s.handleGetPublicTraderConfig)
		// ✅ 修正：performance API 應該公開可用（類似其他公開數據 API）
		api.GET("/performance", s.handlePerformance)

		// 需要认证的路由
		protected := api.Group("/", s.authMiddleware())
		{
			// 注销（加入黑名单）
			protected.POST("/logout", s.handleLogout)

			// 服务器IP查询（需要认证，用于白名单配置）
			protected.GET("/server-ip", s.handleGetServerIP)

			// AI交易员管理
			protected.GET("/my-traders", s.handleTraderList)
			protected.GET("/traders/:id/config", s.handleGetTraderConfig)
			protected.POST("/traders", s.handleCreateTrader)
			protected.PUT("/traders/:id", s.handleUpdateTrader)
			protected.DELETE("/traders/:id", s.handleDeleteTrader)
			protected.POST("/traders/:id/start", s.handleStartTrader)
			protected.POST("/traders/:id/stop", s.handleStopTrader)
			protected.PUT("/traders/:id/prompt", s.handleUpdateTraderPrompt)
			protected.POST("/traders/:id/sync-balance", s.handleSyncBalance)

			// AI模型配置
			protected.GET("/models", s.handleGetModelConfigs)
			protected.PUT("/models", s.handleUpdateModelConfigs)
			protected.POST("/models/probe", s.handleProbeModels)

			// 交易所配置
			protected.GET("/exchanges", s.handleGetExchangeConfigs)
			protected.PUT("/exchanges", s.handleUpdateExchangeConfigs)

			// 用户信号源配置
			protected.GET("/user/signal-sources", s.handleGetUserSignalSource)
			protected.POST("/user/signal-sources", s.handleSaveUserSignalSource)

			// 指定trader的数据（使用query参数 ?trader_id=xxx）
			protected.GET("/status", s.handleStatus)
			protected.GET("/account", s.handleAccount)
			protected.GET("/positions", s.handlePositions)
			protected.GET("/decisions", s.handleDecisions)
			protected.GET("/decisions/latest", s.handleLatestDecisions)
			protected.GET("/statistics", s.handleStatistics)

			// Prompt热重载
			protected.POST("/reload-prompts", s.handleReloadPrompts)

			protected.GET("/operator-directive", s.handleGetOperatorDirective)
			protected.GET("/operator-events", s.handleListOperatorEvents)
			protected.POST("/operator-events", s.handleCreateOperatorEvent)

			// Backtest API
			protected.POST("/backtest/run", s.handleStartBacktest)
			protected.GET("/backtest/runs", s.handleListBacktestRuns)
			protected.GET("/backtest/runs/:id", s.handleGetBacktestRun)
		}
	}
}

// handleHealth 健康检查
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// handleGetSystemConfig 获取系统配置（客户端需要知道的配置）
func (s *Server) handleGetSystemConfig(c *gin.Context) {
	// 获取默认币种
	defaultCoinsStr, _ := s.database.GetSystemConfig("default_coins")
	var defaultCoins []string
	if defaultCoinsStr != "" {
		json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins)
	}
	if len(defaultCoins) == 0 {
		// 使用硬编码的默认币种
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
	}

	// 获取杠杆配置
	btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage")
	altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage")

	btcEthLeverage := 5
	if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
		btcEthLeverage = val
	}

	altcoinLeverage := 5
	if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
		altcoinLeverage = val
	}

	// 获取内测模式配置
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	betaMode := betaModeStr == "true"

	c.JSON(http.StatusOK, gin.H{
		"admin_mode":       auth.IsAdminMode(),
		"beta_mode":        betaMode,
		"default_coins":    defaultCoins,
		"btc_eth_leverage": btcEthLeverage,
		"altcoin_leverage": altcoinLeverage,
	})
}

// handleGetServerIP 获取服务器IP地址（用于白名单配置）
func (s *Server) handleGetServerIP(c *gin.Context) {
	// 尝试通过第三方API获取公网IP
	publicIP := getPublicIPFromAPI()

	// 如果第三方API失败，从网络接口获取第一个公网IP
	if publicIP == "" {
		publicIP = getPublicIPFromInterface()
	}

	// 如果还是没有获取到，返回错误
	if publicIP == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取公网IP地址"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_ip": publicIP,
		"message":   "请将此IP地址添加到白名单中",
	})
}

// getPublicIPFromAPI 通过第三方API获取公网IP
func getPublicIPFromAPI() string {
	// 尝试多个公网IP查询服务
	services := []string{
		"https://api.ipify.org?format=text",
		"https://icanhazip.com",
		"https://ifconfig.me",
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, service := range services {
		if ip := func() string {
			resp, err := client.Get(service)
			if err != nil {
				return ""
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				body := make([]byte, 128)
				n, err := resp.Body.Read(body)
				if err != nil && err.Error() != "EOF" {
					return ""
				}

				ip := strings.TrimSpace(string(body[:n]))
				// 验证是否为有效的IP地址
				if net.ParseIP(ip) != nil {
					return ip
				}
			}
			return ""
		}(); ip != "" {
			return ip
		}
	}

	return ""
}

// getPublicIPFromInterface 从网络接口获取第一个公网IP
func getPublicIPFromInterface() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// 跳过未启用的接口和回环接口
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// 只考虑IPv4地址
			if ip.To4() != nil {
				ipStr := ip.String()
				// 排除私有IP地址范围
				if !isPrivateIP(ip) {
					return ipStr
				}
			}
		}
	}

	return ""
}

// isPrivateIP 判断是否为私有IP地址
func isPrivateIP(ip net.IP) bool {
	// 私有IP地址范围：
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

// getTraderFromQuery 从query参数获取trader
func (s *Server) getTraderFromQuery(c *gin.Context) (*manager.TraderManager, string, error) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// ✅ 修正：智能載入交易員，避免頻繁重載但確保必要時載入
	// 檢查特定交易員是否已經在記憶體中
	if traderID != "" {
		// 如果指定了 trader_id，檢查該交易員是否已載入
		_, err := s.traderManager.GetTrader(traderID)
		if err != nil {
			// 交易員不在記憶體中，需要載入
			log.Printf("📋 交易員 %s 不在記憶體中，載入用戶 %s 的交易員", traderID, userID)
			loadErr := s.traderManager.LoadUserTraders(s.database, userID)
			if loadErr != nil {
				log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, loadErr)
			}
		}
	} else {
		// 沒有指定 trader_id，檢查是否有任何交易員在記憶體中
		traderIDs := s.traderManager.GetTraderIDs()
		if len(traderIDs) == 0 {
			log.Printf("📋 交易員記憶體為空，載入用戶 %s 的交易員", userID)
			loadErr := s.traderManager.LoadUserTraders(s.database, userID)
			if loadErr != nil {
				log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, loadErr)
			}
		}
	}

	if traderID == "" {
		// 获取用户的交易员列表，优先返回用户自己的第一个交易员
		userTraders, err := s.database.GetTraders(userID)
		if err == nil && len(userTraders) > 0 {
			traderID = userTraders[0].ID
		} else {
			return nil, "", fmt.Errorf("当前用户没有可用的交易员")
		}
	} else {
		// 校验该 traderID 是否属于当前用户（管理员模式或本人）
		if !auth.IsAdminMode() {
			if _, _, _, err := s.database.GetTraderConfig(userID, traderID); err != nil {
				return nil, "", fmt.Errorf("交易员不存在或无访问权限")
			}
		}
	}

	return s.traderManager, traderID, nil
}

// AI交易员管理相关结构体
type CreateTraderRequest struct {
	Name                 string  `json:"name" binding:"required"`
	AIModelID            string  `json:"ai_model_id" binding:"required"`
	ExchangeID           string  `json:"exchange_id" binding:"required"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`
	AltcoinLeverage      int     `json:"altcoin_leverage"`
	TradingSymbols       string  `json:"trading_symbols"`
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        *bool   `json:"is_cross_margin"`        // 指针类型，nil表示使用默认值true
	UseCoinPool          bool    `json:"use_coin_pool"`
	UseOITop             bool    `json:"use_oi_top"`
}

type ModelConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	APIKey       string `json:"apiKey,omitempty"`
	CustomAPIURL string `json:"customApiUrl,omitempty"`
}

type ExchangeConfig struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Type                  string `json:"type"` // "cex" or "dex"
	Enabled               bool   `json:"enabled"`
	APIKey                string `json:"apiKey,omitempty"`
	SecretKey             string `json:"secretKey,omitempty"`
	Testnet               bool   `json:"testnet,omitempty"`
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr,omitempty"`
	AsterUser             string `json:"asterUser,omitempty"`
	AsterSigner           string `json:"asterSigner,omitempty"`
	AsterPrivateKey       string `json:"asterPrivateKey,omitempty"`
}

type UpdateModelConfigRequest struct {
	Models map[string]struct {
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
		EnvKey          string `json:"env_key"`
		Name            string `json:"name"`
		Provider        string `json:"provider"`
	} `json:"models"`
}

type ProbeModelsRequest struct {
	ModelID  string `json:"model_id"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	EnvKey   string `json:"env_key"`
}

type UpdateExchangeConfigRequest struct {
	Exchanges map[string]struct {
		Enabled               bool   `json:"enabled"`
		APIKey                string `json:"api_key"`
		SecretKey             string `json:"secret_key"`
		Testnet               bool   `json:"testnet"`
		HyperliquidWalletAddr string `json:"hyperliquid_wallet_addr"`
		AsterUser             string `json:"aster_user"`
		AsterSigner           string `json:"aster_signer"`
		AsterPrivateKey       string `json:"aster_private_key"`
	} `json:"exchanges"`
}

// handleCreateTrader 创建新的AI交易员
func (s *Server) handleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 校验杠杆值
	if req.BTCETHLeverage < 0 || req.BTCETHLeverage > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BTC/ETH杠杆必须在1-50倍之间"})
		return
	}
	if req.AltcoinLeverage < 0 || req.AltcoinLeverage > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "山寨币杠杆必须在1-20倍之间"})
		return
	}

	// 校验交易币种格式
	if req.TradingSymbols != "" {
		symbols := strings.Split(req.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.HasSuffix(strings.ToUpper(symbol), "USDT") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的币种格式: %s，必须以USDT结尾", symbol)})
				return
			}
		}
	}

	// 生成交易员ID
	traderID := fmt.Sprintf("%s_%s_%d", req.ExchangeID, req.AIModelID, time.Now().Unix())

	// 设置默认值
	isCrossMargin := true // 默认为全仓模式
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值（从系统配置获取）
	btcEthLeverage := 5
	altcoinLeverage := 5
	if req.BTCETHLeverage > 0 {
		btcEthLeverage = req.BTCETHLeverage
	} else {
		// 从系统配置获取默认值
		if btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage"); btcEthLeverageStr != "" {
			if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
				btcEthLeverage = val
			}
		}
	}
	if req.AltcoinLeverage > 0 {
		altcoinLeverage = req.AltcoinLeverage
	} else {
		// 从系统配置获取默认值
		if altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage"); altcoinLeverageStr != "" {
			if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
				altcoinLeverage = val
			}
		}
	}

	// 设置系统提示词模板默认值
	systemPromptTemplate := "default"
	if req.SystemPromptTemplate != "" {
		systemPromptTemplate = req.SystemPromptTemplate
	}

	// 设置扫描间隔默认值
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3 // 默认3分钟，且不允许小于3
	}

	// ⚠️ 重要修复：始终使用用户输入的初始余额作为初始本金
	// 不应该使用交易所实际余额，因为用户输入的初始余额是用户想要追踪的起始本金
	// 实际余额可能已经包含交易盈亏，不应该作为初始余额
	initialBalance := req.InitialBalance

	// 如果交易所已配置，查询实际余额用于验证和提示（但不用于设置初始余额）
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		log.Printf("⚠️ 获取交易所配置失败: %v", err)
	}

	// 查找匹配的交易所配置
	var exchangeCfg *config.ExchangeConfig
	for _, ex := range exchanges {
		if ex.ID == req.ExchangeID {
			exchangeCfg = ex
			break
		}
	}

	if exchangeCfg != nil && exchangeCfg.Enabled {
		// 根据交易所类型创建临时 trader 查询余额（仅用于验证和提示）
		var tempTrader trader.Trader
		var createErr error

		switch req.ExchangeID {
		case "binance":
			tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey)
		case "hyperliquid":
			tempTrader, createErr = trader.NewHyperliquidTrader(
				exchangeCfg.APIKey, // private key
				exchangeCfg.HyperliquidWalletAddr,
				exchangeCfg.Testnet,
			)
		case "aster":
			tempTrader, createErr = trader.NewAsterTrader(
				exchangeCfg.AsterUser,
				exchangeCfg.AsterSigner,
				exchangeCfg.AsterPrivateKey,
				exchangeCfg.Testnet,
			)
		default:
			log.Printf("⚠️ 不支持的交易所类型: %s", req.ExchangeID)
		}

		if createErr != nil {
			log.Printf("⚠️ 创建临时 trader 失败，无法验证余额: %v", createErr)
		} else if tempTrader != nil {
			// 查询实际余额（仅用于验证和提示）
			balanceInfo, balanceErr := tempTrader.GetBalance()
			if balanceErr != nil {
				log.Printf("⚠️ 查询交易所余额失败，无法验证: %v", balanceErr)
			} else {
				// 提取可用余额进行验证
				var actualBalance float64
				if availableBalance, ok := balanceInfo["available_balance"].(float64); ok && availableBalance > 0 {
					actualBalance = availableBalance
				} else if totalBalance, ok := balanceInfo["balance"].(float64); ok && totalBalance > 0 {
					actualBalance = totalBalance
				}

				if actualBalance > 0 {
					log.Printf("ℹ️ 交易所实际余额: %.2f USDT，用户设置的初始本金: %.2f USDT", actualBalance, initialBalance)
					if math.Abs(actualBalance-initialBalance) > 0.01 {
						log.Printf("⚠️ 注意：交易所实际余额与用户输入的初始本金不一致，将使用用户输入的初始本金 %.2f USDT 作为计算基准", initialBalance)
					}
				}
			}
		}
	}

	// 创建交易员配置（数据库实体）
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       initialBalance, // ⚠️ 使用用户输入的初始余额，而不是交易所实际余额
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		UseCoinPool:          req.UseCoinPool,
		UseOITop:             req.UseOITop,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            false,
	}

	// 保存到数据库
	err = s.database.CreateTrader(trader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建交易员失败: %v", err)})
		return
	}

	// 立即将新交易员加载到TraderManager中
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易员已经成功创建到数据库
	}

	log.Printf("✓ 创建交易员成功: %s (模型: %s, 交易所: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusCreated, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"is_running":  false,
	})
}

// UpdateTraderRequest 更新交易员请求
type UpdateTraderRequest struct {
	Name                 string  `json:"name" binding:"required"`
	AIModelID            string  `json:"ai_model_id" binding:"required"`
	ExchangeID           string  `json:"exchange_id" binding:"required"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`
	AltcoinLeverage      int     `json:"altcoin_leverage"`
	TradingSymbols       string  `json:"trading_symbols"`
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"` // ✅ 修正：添加遺漏的提示詞模板字段
	IsCrossMargin        *bool   `json:"is_cross_margin"`
	UseCoinPool          bool    `json:"use_coin_pool"`
	UseOITop             bool    `json:"use_oi_top"`
}

// handleUpdateTrader 更新交易员配置
func (s *Server) handleUpdateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ⚠️ 调试：输出接收到的更新请求
	log.Printf("📝 收到更新交易员请求 [%s]", traderID)
	log.Printf("📝 请求中的 InitialBalance 字段: %.2f USDT", req.InitialBalance)
	log.Printf("📝 请求完整内容: %+v", req)

	// ⚠️ 重要：检查 InitialBalance 是否为有效值
	// 如果前端发送的是 0 或未定义，可能是前端没有正确传递值
	if req.InitialBalance == 0 {
		log.Printf("⚠️ 警告：前端发送的 InitialBalance 为 0，这可能表示前端没有正确传递初始余额值")
	}

	// 检查交易员是否存在且属于当前用户
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
		return
	}

	var existingTrader *config.TraderRecord
	for _, trader := range traders {
		if trader.ID == traderID {
			existingTrader = trader
			break
		}
	}

	if existingTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	log.Printf("📊 数据库中的现有交易员配置 [%s]: InitialBalance=%.2f USDT", existingTrader.Name, existingTrader.InitialBalance)

	// 设置默认值
	isCrossMargin := existingTrader.IsCrossMargin // 保持原值
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值
	btcEthLeverage := req.BTCETHLeverage
	altcoinLeverage := req.AltcoinLeverage
	if btcEthLeverage <= 0 {
		btcEthLeverage = existingTrader.BTCETHLeverage // 保持原值
	}
	if altcoinLeverage <= 0 {
		altcoinLeverage = existingTrader.AltcoinLeverage // 保持原值
	}

	// 设置扫描间隔，允许更新
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // 保持原值
	} else if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3
	}

	// ⚠️ 检查是否需要热重载（Hot Reload）
	// 如果关键配置改变且交易员正在运行，需要重启才能生效
	needsReload := false

	// 1. 检查扫描间隔是否改变
	if scanIntervalMinutes != existingTrader.ScanIntervalMinutes {
		needsReload = true
		log.Printf("🔄 检测到扫描间隔变更: %d -> %d", existingTrader.ScanIntervalMinutes, scanIntervalMinutes)
	}

	// 2. 检查交易币种是否改变
	if req.TradingSymbols != existingTrader.TradingSymbols {
		needsReload = true
		log.Printf("🔄 检测到交易币种变更: %s -> %s", existingTrader.TradingSymbols, req.TradingSymbols)
	}

	// 3. 检查AI模型是否改变
	if req.AIModelID != existingTrader.AIModelID {
		needsReload = true
		log.Printf("🔄 检测到AI模型变更: %s -> %s", existingTrader.AIModelID, req.AIModelID)
	}

	// 4. 检查交易所是否改变
	if req.ExchangeID != existingTrader.ExchangeID {
		needsReload = true
		log.Printf("🔄 检测到交易所变更: %s -> %s", existingTrader.ExchangeID, req.ExchangeID)
	}

	// 5. 检查杠杆倍数是否改变
	if btcEthLeverage != existingTrader.BTCETHLeverage || altcoinLeverage != existingTrader.AltcoinLeverage {
		needsReload = true
		log.Printf("🔄 检测到杠杆倍数变更")
	}

	// 6. 检查全仓模式是否改变
	if isCrossMargin != existingTrader.IsCrossMargin {
		needsReload = true
		log.Printf("🔄 检测到全仓模式变更: %v -> %v", existingTrader.IsCrossMargin, isCrossMargin)
	}

	// 7. 检查信号源配置是否改变
	if req.UseCoinPool != existingTrader.UseCoinPool || req.UseOITop != existingTrader.UseOITop {
		needsReload = true
		log.Printf("🔄 检测到信号源配置变更")
	}

	// 8. 检查Prompt配置是否改变
	if req.CustomPrompt != existingTrader.CustomPrompt ||
		req.OverrideBasePrompt != existingTrader.OverrideBasePrompt ||
		req.SystemPromptTemplate != existingTrader.SystemPromptTemplate {
		needsReload = true
		log.Printf("🔄 检测到Prompt配置变更")
	}

	var wasRunning bool
	var runningTrader *trader.AutoTrader

	// 如果需要重载，先检查当前运行状态
	if needsReload {
		if at, err := s.traderManager.GetTrader(traderID); err == nil {
			status := at.GetStatus()
			if isRunning, ok := status["is_running"].(bool); ok && isRunning {
				wasRunning = true
				runningTrader = at
				log.Printf("⚠️ 配置变更需热重载，交易员正在运行，准备重启...")
			}
		}
	}

	// ✅ 修正：初始餘額保護機制
	// 初始餘額是用戶的投入本金，應該謹慎處理更新
	// 只有在用戶明確修改且與原值不同時才更新
	initialBalance := existingTrader.InitialBalance // 默認保持原值

	log.Printf("📊 处理初始余额: 请求值=%.2f, 数据库原值=%.2f", req.InitialBalance, existingTrader.InitialBalance)

	// ✅ 修正：更嚴格的初始餘額檢查
	// 檢查用戶是否真的要更新初始餘額
	if req.InitialBalance > 0 && math.Abs(req.InitialBalance-existingTrader.InitialBalance) > 0.01 {
		// 額外保護：如果新值看起來像是當前淨值而不是本金，拒絕更新
		if req.InitialBalance < 50 && req.InitialBalance != math.Trunc(req.InitialBalance*100)/100 {
			log.Printf("🛡️ 初始餘額保護：新值 %.6f 看起來像是當前淨值而不是投入本金，拒絕更新", req.InitialBalance)
			log.Printf("🛡️ 保持原始初始餘額: %.2f USDT", existingTrader.InitialBalance)
			initialBalance = existingTrader.InitialBalance
		} else {
			// 用戶明確要更新初始餘額
			initialBalance = req.InitialBalance
			log.Printf("⚠️ 用户明确修改初始余额（本金）: %.2f → %.2f USDT", existingTrader.InitialBalance, initialBalance)
			log.Printf("⚠️ 注意：修改初始余额会影响盈亏计算，请确保这是用户的真实意图（如充值/提现）")
		}
	} else if req.InitialBalance <= 0 {
		log.Printf("ℹ️ 用户未提供有效的初始余额（收到: %.2f），保持数据库原值: %.2f USDT", req.InitialBalance, initialBalance)
	} else {
		log.Printf("ℹ️ 用户提供的初始余额与原值相同或差异很小，保持不变: %.2f USDT", initialBalance)
	}

	// 更新交易员配置
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       initialBalance, // 使用处理后的初始余额
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: req.SystemPromptTemplate, // ✅ 修正：使用請求中的新值
		UseCoinPool:          req.UseCoinPool,          // ✅ 修正：添加信號源配置
		UseOITop:             req.UseOITop,             // ✅ 修正：添加信號源配置
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            existingTrader.IsRunning, // 保持原值
	}

	// ⚠️ 重要：在更新数据库前，再次确认初始余额的变化
	if math.Abs(initialBalance-existingTrader.InitialBalance) > 0.01 {
		log.Printf("⚠️ 警告：初始余额将被更新: %.2f → %.2f USDT (变化: %.2f%%)",
			existingTrader.InitialBalance, initialBalance,
			((initialBalance-existingTrader.InitialBalance)/existingTrader.InitialBalance)*100)
	}

	// 更新数据库
	err = s.database.UpdateTrader(trader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易员失败: %v", err)})
		return
	}

	// ⚠️ 如果需要热重载且交易员正在运行，先停止它
	if needsReload && wasRunning && runningTrader != nil {
		log.Printf("⏹ 停止交易员以应用新配置")
		runningTrader.Stop()
		// 等待一下确保停止完成
		time.Sleep(500 * time.Millisecond)
		// 更新数据库中的运行状态
		s.database.UpdateTraderStatus(userID, traderID, false)
	}

	// ⚠️ 如果需要热重载，删除旧的交易员实例，以便重新创建
	if needsReload {
		s.traderManager.RemoveTrader(traderID)
		log.Printf("🗑️ 已删除旧的交易员实例，准备重新创建")
	}

	// 重新加载交易员到内存
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
	} else {
		// ⚠️ 验证：检查内存中的初始余额是否正确更新
		if updatedTrader, err := s.traderManager.GetTrader(traderID); err == nil {
			memoryInitialBalance := updatedTrader.GetInitialBalance()
			log.Printf("✅ 验证：数据库中的初始余额=%.2f USDT, 内存中的初始余额=%.2f USDT", initialBalance, memoryInitialBalance)
			if math.Abs(memoryInitialBalance-initialBalance) > 0.01 {
				log.Printf("⚠️ 警告：内存中的初始余额与数据库不一致！数据库=%.2f, 内存=%.2f", initialBalance, memoryInitialBalance)
			}

			// ⚠️ 验证：检查扫描间隔是否正确更新
			status := updatedTrader.GetStatus()
			if scanInterval, ok := status["scan_interval"].(string); ok {
				log.Printf("✅ 验证：扫描间隔已更新为 %s", scanInterval)
			}
		}

		// ⚠️ 如果之前正在运行，重新启动交易员
		if needsReload && wasRunning {
			if updatedTrader, err := s.traderManager.GetTrader(traderID); err == nil {
				log.Printf("▶️ 重新启动交易员以应用新配置")
				go func() {
					if err := updatedTrader.Run(); err != nil {
						log.Printf("❌ 交易员 %s 重新启动失败: %v", updatedTrader.GetName(), err)
					}
				}()
				// 更新数据库中的运行状态
				s.database.UpdateTraderStatus(userID, traderID, true)
			}
		}
	}

	log.Printf("✓ 更新交易员成功: %s (模型: %s, 交易所: %s, 初始余额: %.2f USDT, 扫描间隔: %d 分钟)",
		req.Name, req.AIModelID, req.ExchangeID, initialBalance, scanIntervalMinutes)

	c.JSON(http.StatusOK, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"message":     "交易员更新成功",
	})
}

// handleDeleteTrader 删除交易员
func (s *Server) handleDeleteTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 从数据库删除
	err := s.database.DeleteTrader(userID, traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除交易员失败: %v", err)})
		return
	}

	// 如果交易员正在运行，先停止它
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		status := trader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			trader.Stop()
			log.Printf("⏹  已停止运行中的交易员: %s", traderID)
		}
	}

	log.Printf("✓ 交易员已删除: %s", traderID)
	c.JSON(http.StatusOK, gin.H{"message": "交易员已删除"})
}

// handleStartTrader 启动交易员
func (s *Server) handleStartTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 校验交易员是否属于当前用户
	_, _, _, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		// 嘗試從資料庫載入該用戶的交易員
		if loadErr := s.traderManager.LoadUserTraders(s.database, userID); loadErr == nil {
			trader, err = s.traderManager.GetTrader(traderID)
		}
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "交易员未初始化"})
			return
		}
	}

	// 检查交易员是否已经在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已在运行中"})
		return
	}

	// 启动交易员
	go func() {
		// 启动前自动重载Prompt模板（确保使用最新的Prompt）
		if err := decision.ReloadPromptTemplates(); err != nil {
			log.Printf("⚠️ Prompt模板重载失败: %v (将使用现有模板)", err)
		} else {
			log.Printf("✓ Prompt模板已重载")
		}

		log.Printf("▶️  启动交易员 %s (%s)", traderID, trader.GetName())
		if err := trader.Run(); err != nil {
			log.Printf("❌ 交易员 %s 运行错误: %v", trader.GetName(), err)
		}
	}()

	// 更新数据库中的运行状态
	err = s.database.UpdateTraderStatus(userID, traderID, true)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("✓ 交易员 %s 已启动", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "交易员已启动"})
}

// handleStopTrader 停止交易员
func (s *Server) handleStopTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 校验交易员是否属于当前用户
	_, _, _, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 检查交易员是否正在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已停止"})
		return
	}

	// 停止交易员
	trader.Stop()

	// 更新数据库中的运行状态
	err = s.database.UpdateTraderStatus(userID, traderID, false)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("⏹  交易员 %s 已停止", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "交易员已停止"})
}

// handleUpdateTraderPrompt 更新交易员自定义Prompt
func (s *Server) handleUpdateTraderPrompt(c *gin.Context) {
	traderID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		CustomPrompt       string `json:"custom_prompt"`
		OverrideBasePrompt bool   `json:"override_base_prompt"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新数据库
	err := s.database.UpdateTraderCustomPrompt(userID, traderID, req.CustomPrompt, req.OverrideBasePrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新自定义prompt失败: %v", err)})
		return
	}

	// 如果trader在内存中，更新其custom prompt和override设置
	trader, err := s.traderManager.GetTrader(traderID)
	if err == nil {
		trader.SetCustomPrompt(req.CustomPrompt)
		trader.SetOverrideBasePrompt(req.OverrideBasePrompt)
		log.Printf("✓ 已更新交易员 %s 的自定义prompt (覆盖基础=%v)", trader.GetName(), req.OverrideBasePrompt)
	}

	c.JSON(http.StatusOK, gin.H{"message": "自定义prompt已更新"})
}

// handleSyncBalance 同步交易所余额到initial_balance（选项B：手动同步 + 选项C：智能检测）
func (s *Server) handleSyncBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	log.Printf("🔄 用户 %s 请求同步交易员 %s 的余额", userID, traderID)

	// 从数据库获取交易员配置（包含交易所信息）
	traderConfig, _, exchangeCfg, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易所未配置或未启用"})
		return
	}

	// 创建临时 trader 查询余额
	var tempTrader trader.Trader
	var createErr error

	switch traderConfig.ExchangeID {
	case "binance":
		tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey)
	case "hyperliquid":
		tempTrader, createErr = trader.NewHyperliquidTrader(
			exchangeCfg.APIKey,
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
		)
	case "aster":
		tempTrader, createErr = trader.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			exchangeCfg.AsterPrivateKey,
			exchangeCfg.Testnet,
		)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的交易所类型"})
		return
	}

	if createErr != nil {
		log.Printf("⚠️ 创建临时 trader 失败: %v", createErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("连接交易所失败: %v", createErr)})
		return
	}

	// 查询实际余额
	balanceInfo, balanceErr := tempTrader.GetBalance()
	if balanceErr != nil {
		log.Printf("⚠️ 查询交易所余额失败: %v", balanceErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询余额失败: %v", balanceErr)})
		return
	}

	// 提取可用余额
	var actualBalance float64
	if availableBalance, ok := balanceInfo["available_balance"].(float64); ok && availableBalance > 0 {
		actualBalance = availableBalance
	} else if availableBalance, ok := balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
		actualBalance = availableBalance
	} else if totalBalance, ok := balanceInfo["balance"].(float64); ok && totalBalance > 0 {
		actualBalance = totalBalance
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取可用余额"})
		return
	}

	initialBalance := traderConfig.InitialBalance

	// ✅ 修正：同步餘額功能不應該更新初始餘額
	// 初始餘額是用戶投入的本金，應該保持不變
	// 只返回當前餘額資訊供用戶參考

	// 計算盈虧供參考
	profitLoss := actualBalance - initialBalance
	profitLossPercent := 0.0
	if initialBalance > 0 {
		profitLossPercent = (profitLoss / initialBalance) * 100
	}

	log.Printf("ℹ️ 查詢餘額: 初始本金=%.2f USDT, 交易所實際餘額=%.2f USDT, 盈虧=%.2f USDT (%.2f%%)",
		initialBalance, actualBalance, profitLoss, profitLossPercent)

	// ⚠️ 不再更新 initial_balance，保持原始本金不變
	// 這樣盈虧計算才會準確
	// 如果用戶真的需要更新初始餘額（例如充值後），應該在編輯交易員配置時手動修改

	log.Printf("✅ 餘額查詢成功: 初始本金=%.2f USDT (保持不變), 當前餘額=%.2f USDT", initialBalance, actualBalance)

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         "餘額查詢成功",
		"initial_balance": initialBalance,    // 保持原始本金不變
		"current_balance": actualBalance,     // 當前實際餘額
		"profit_loss":     profitLoss,        // 盈虧金額
		"profit_loss_pct": profitLossPercent, // 盈虧百分比
		"note":            "初始餘額（本金）保持不變，確保盈虧計算準確",
	})
}

// handleGetModelConfigs 获取AI模型配置
// ⚠️ 安全修复：API Key返回遮蔽值"***"，不返回实际密钥
func (s *Server) handleGetModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	log.Printf("🔍 查询用户 %s 的AI模型配置", userID)
	models, err := s.database.GetAIModels(userID)
	if err != nil {
		log.Printf("❌ 获取AI模型配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取AI模型配置失败: %v", err)})
		return
	}
	log.Printf("✅ 找到 %d 个AI模型配置", len(models))

	// 🔒 安全处理：遮蔽API Key
	type SafeModelConfig struct {
		ID              string `json:"id"`
		UserID          string `json:"user_id"`
		Name            string `json:"name"`
		Provider        string `json:"provider"`
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"apiKey,omitempty"`
		CustomAPIURL    string `json:"customApiUrl"`
		CustomModelName string `json:"customModelName"`
		EnvKey          string `json:"envKey"`
		HasAPIKey       bool   `json:"hasApiKey"`
	}

	safeModels := make([]SafeModelConfig, 0, len(models))
	for _, m := range models {
		apiKey := ""
		if m.APIKey != "" {
			apiKey = "***"
		}
		safeModels = append(safeModels, SafeModelConfig{
			ID:              m.ID,
			UserID:          m.UserID,
			Name:            m.Name,
			Provider:        m.Provider,
			Enabled:         m.Enabled,
			APIKey:          apiKey,
			CustomAPIURL:    m.CustomAPIURL,
			CustomModelName: m.CustomModelName,
			EnvKey:          m.EnvKey,
			HasAPIKey:       m.APIKey != "",
		})
	}

	c.JSON(http.StatusOK, safeModels)
}

// handleUpdateModelConfigs 更新AI模型配置
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	var req UpdateModelConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated := make([]string, 0, len(req.Models))
	for modelID, modelData := range req.Models {
		err := s.database.UpdateAIModel(
			userID, modelID, modelData.Enabled, modelData.APIKey,
			modelData.CustomAPIURL, modelData.CustomModelName,
			modelData.EnvKey, modelData.Name, modelData.Provider,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新模型 %s 失败: %v", modelID, err)})
			return
		}
		updated = append(updated, modelID)
	}

	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
	}

	log.Printf("✓ AI模型配置已更新: ids=%v", updated)
	c.JSON(http.StatusOK, gin.H{"message": "模型配置已更新", "ids": updated})
}

func (s *Server) handleGetModelProviders(c *gin.Context) {
	c.JSON(http.StatusOK, config.Catalog())
}

func (s *Server) handleProbeModels(c *gin.Context) {
	userID := c.GetString("user_id")
	var req ProbeModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	baseURL := strings.TrimSpace(req.BaseURL)
	envKey := strings.TrimSpace(req.EnvKey)
	provider := strings.TrimSpace(req.Provider)
	apiKey := strings.TrimSpace(req.APIKey)

	if req.ModelID != "" {
		models, err := s.database.GetAIModels(userID)
		if err == nil {
			for _, m := range models {
				if m.ID != req.ModelID {
					continue
				}
				if baseURL == "" {
					baseURL = m.CustomAPIURL
				}
				if envKey == "" {
					envKey = m.EnvKey
				}
				if provider == "" {
					provider = m.Provider
				}
				if apiKey == "" || apiKey == "***" {
					apiKey = m.APIKey
				}
				break
			}
		}
	}

	if baseURL == "" {
		if p := config.LookupPreset(provider); p != nil {
			baseURL = p.BaseURL
		}
	}
	baseURL = config.RewriteLoopbackURL(baseURL)
	resolved := config.ResolveAPIKey(apiKey, envKey, provider)
	if baseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 Base URL"})
		return
	}

	listed, err := mcp.ListModels(baseURL, resolved)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"ok":       false,
			"base_url": baseURL,
			"error":    err.Error(),
		})
		return
	}
	ids := make([]string, 0, len(listed))
	for _, m := range listed {
		ids = append(ids, m.ID)
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"base_url": baseURL,
		"models":   ids,
		"count":    len(ids),
	})
}

// handleGetExchangeConfigs 获取交易所配置
// ⚠️ 安全修复：敏感字段返回遮蔽值"***"，不返回实际密钥
func (s *Server) handleGetExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	log.Printf("🔍 查询用户 %s 的交易所配置", userID)
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		log.Printf("❌ 获取交易所配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易所配置失败: %v", err)})
		return
	}
	log.Printf("✅ 找到 %d 个交易所配置", len(exchanges))

	// 🔒 安全处理：遮蔽敏感字段，返回占位符"***"而不是实际值
	type SafeExchangeConfig struct {
		ID                    string `json:"id"`
		UserID                string `json:"user_id"`
		Name                  string `json:"name"`
		Type                  string `json:"type"`
		Enabled               bool   `json:"enabled"`
		Testnet               bool   `json:"testnet"`
		APIKey                string `json:"apiKey,omitempty"`
		SecretKey             string `json:"secretKey,omitempty"`
		HyperliquidWalletAddr string `json:"hyperliquidWalletAddr,omitempty"`
		AsterUser             string `json:"asterUser,omitempty"`
		AsterSigner           string `json:"asterSigner,omitempty"`
		AsterPrivateKey       string `json:"asterPrivateKey,omitempty"`
	}

	maskValue := func(val string) string {
		if val != "" {
			return "***"
		}
		return ""
	}

	safeExchanges := make([]SafeExchangeConfig, 0, len(exchanges))
	for _, ex := range exchanges {
		safeExchanges = append(safeExchanges, SafeExchangeConfig{
			ID:                    ex.ID,
			UserID:                ex.UserID,
			Name:                  ex.Name,
			Type:                  ex.Type,
			Enabled:               ex.Enabled,
			Testnet:               ex.Testnet,
			APIKey:                maskValue(ex.APIKey),
			SecretKey:             maskValue(ex.SecretKey),
			HyperliquidWalletAddr: maskValue(ex.HyperliquidWalletAddr),
			AsterUser:             maskValue(ex.AsterUser),
			AsterSigner:           maskValue(ex.AsterSigner),
			AsterPrivateKey:       maskValue(ex.AsterPrivateKey),
		})
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// handleUpdateExchangeConfigs 更新交易所配置
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	var req UpdateExchangeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新每个交易所的配置
	for exchangeID, exchangeData := range req.Exchanges {
		err := s.database.UpdateExchange(userID, exchangeID, exchangeData.Enabled, exchangeData.APIKey, exchangeData.SecretKey, exchangeData.Testnet, exchangeData.HyperliquidWalletAddr, exchangeData.AsterUser, exchangeData.AsterSigner, exchangeData.AsterPrivateKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易所 %s 失败: %v", exchangeID, err)})
			return
		}
	}

	// 重新加载该用户的所有交易员，使新配置立即生效
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易所配置已经成功更新到数据库
	}

	log.Printf("✓ 交易所配置已更新: %+v", req.Exchanges)
	c.JSON(http.StatusOK, gin.H{"message": "交易所配置已更新"})
}

// handleGetUserSignalSource 获取用户信号源配置
func (s *Server) handleGetUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	source, err := s.database.GetUserSignalSource(userID)
	if err != nil {
		// 如果配置不存在，返回空配置而不是404错误
		c.JSON(http.StatusOK, gin.H{
			"coin_pool_url": "",
			"oi_top_url":    "",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"coin_pool_url": source.CoinPoolURL,
		"oi_top_url":    source.OITopURL,
	})
}

// handleSaveUserSignalSource 保存用户信号源配置
func (s *Server) handleSaveUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		CoinPoolURL string `json:"coin_pool_url"`
		OITopURL    string `json:"oi_top_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.database.CreateUserSignalSource(userID, req.CoinPoolURL, req.OITopURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存用户信号源配置失败: %v", err)})
		return
	}

	log.Printf("✓ 用户信号源配置已保存: user=%s, coin_pool=%s, oi_top=%s", userID, req.CoinPoolURL, req.OITopURL)
	c.JSON(http.StatusOK, gin.H{"message": "用户信号源配置已保存"})
}

// handleTraderList trader列表
func (s *Server) handleTraderList(c *gin.Context) {
	userID := c.GetString("user_id")
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易员列表失败: %v", err)})
		return
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// 获取实时运行状态
		isRunning := trader.IsRunning
		if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		// 返回完整的 AIModelID（如 "admin_deepseek"），不要截断
		// 前端需要完整 ID 来验证模型是否存在（与 handleGetTraderConfig 保持一致）
		result = append(result, map[string]interface{}{
			"trader_id":       trader.ID,
			"trader_name":     trader.Name,
			"ai_model":        trader.AIModelID, // 使用完整 ID
			"exchange_id":     trader.ExchangeID,
			"is_running":      isRunning,
			"initial_balance": trader.InitialBalance,
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleGetTraderConfig 获取交易员详细配置
func (s *Server) handleGetTraderConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	traderConfig, _, _, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("获取交易员配置失败: %v", err)})
		return
	}

	// 获取实时运行状态
	isRunning := traderConfig.IsRunning
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	// 返回完整的模型ID，不做转换，保持与前端模型列表一致
	aiModelID := traderConfig.AIModelID

	log.Printf("📊 获取交易员配置 [%s]: 数据库中的初始余额=%.2f USDT", traderConfig.Name, traderConfig.InitialBalance)

	result := map[string]interface{}{
		"trader_id":              traderConfig.ID,
		"trader_name":            traderConfig.Name,
		"ai_model":               aiModelID,
		"exchange_id":            traderConfig.ExchangeID,
		"initial_balance":        traderConfig.InitialBalance, // ⚠️ 确保字段名与前端一致
		"scan_interval_minutes":  traderConfig.ScanIntervalMinutes,
		"btc_eth_leverage":       traderConfig.BTCETHLeverage,
		"altcoin_leverage":       traderConfig.AltcoinLeverage,
		"trading_symbols":        traderConfig.TradingSymbols,
		"custom_prompt":          traderConfig.CustomPrompt,
		"override_base_prompt":   traderConfig.OverrideBasePrompt,
		"system_prompt_template": traderConfig.SystemPromptTemplate, // ✅ 修正：添加遺漏的提示詞模板字段
		"is_cross_margin":        traderConfig.IsCrossMargin,
		"use_coin_pool":          traderConfig.UseCoinPool,
		"use_oi_top":             traderConfig.UseOITop,
		"is_running":             isRunning,
	}

	log.Printf("📊 返回交易员配置 [%s]: initial_balance=%.2f USDT", traderConfig.Name, result["initial_balance"])

	c.JSON(http.StatusOK, result)
}

// handleStatus 系统状态
func (s *Server) handleStatus(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	status := trader.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleAccount 账户信息
func (s *Server) handleAccount(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("📊 收到账户信息请求 [%s]", trader.GetName())
	account, err := trader.GetAccountInfo()
	if err != nil {
		log.Printf("❌ 获取账户信息失败 [%s]: %v", trader.GetName(), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取账户信息失败: %v", err),
		})
		return
	}

	log.Printf("✓ 返回账户信息 [%s]: 净值=%.2f, 可用=%.2f, 盈亏=%.2f (%.2f%%), 初始余额=%.2f",
		trader.GetName(),
		account["total_equity"],
		account["available_balance"],
		account["total_pnl"],
		account["total_pnl_pct"],
		account["initial_balance"])
	c.JSON(http.StatusOK, account)
}

// handlePositions 持仓列表
func (s *Server) handlePositions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	positions, err := trader.GetPositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取持仓列表失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, positions)
}

// handleDecisions 决策日志列表
func (s *Server) handleDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 获取所有历史决策记录（无限制）
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, records)
}

// handleLatestDecisions 最新决策日志（最近5条，最新的在前）
func (s *Server) handleLatestDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	records, err := trader.GetDecisionLogger().GetLatestRecords(5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	// 反转数组，让最新的在前面（用于列表显示）
	// GetLatestRecords返回的是从旧到新（用于图表），这里需要从新到旧
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}

// handleStatistics 统计信息
func (s *Server) handleStatistics(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	stats, err := trader.GetDecisionLogger().GetStatistics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取统计信息失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleEquityHistory 收益率历史数据
func (s *Server) handleEquityHistory(c *gin.Context) {
	// ✅ 修正：簡化邏輯，直接使用已載入的交易員
	traderID := c.Query("trader_id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 trader_id 参数"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 获取尽可能多的历史数据（几天的数据）
	// 每3分钟一个周期：10000条 = 约20天的数据
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取历史数据失败: %v", err),
		})
		return
	}

	// 构建收益率历史数据点
	type EquityPoint struct {
		Timestamp        string  `json:"timestamp"`
		TotalEquity      float64 `json:"total_equity"`      // 账户净值（wallet + unrealized）
		AvailableBalance float64 `json:"available_balance"` // 可用余额
		TotalPnL         float64 `json:"total_pnl"`         // 总盈亏（相对初始余额）
		TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
		PositionCount    int     `json:"position_count"`    // 持仓数量
		MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
		CycleNumber      int     `json:"cycle_number"`
	}

	// ✅ 修正：始終從 AutoTrader 獲取真實的初始餘額
	// 初始餘額是用戶設定的本金，不應該從歷史數據推算
	initialBalance := 0.0
	if status := trader.GetStatus(); status != nil {
		if ib, ok := status["initial_balance"].(float64); ok && ib > 0 {
			initialBalance = ib
			log.Printf("📊 [%s] 从status获取初始余额: %.2f USDT", trader.GetName(), initialBalance)
		}
	}

	// 如果無法從 status 獲取，嘗試從 account info 獲取
	if initialBalance == 0 {
		if accountInfo, err := trader.GetAccountInfo(); err == nil {
			if ib, ok := accountInfo["initial_balance"].(float64); ok && ib > 0 {
				initialBalance = ib
				log.Printf("📊 [%s] 从accountInfo获取初始余额: %.2f USDT", trader.GetName(), initialBalance)
			}
		}
	}

	// ⚠️ 不要從歷史數據推算初始餘額，這是錯誤的
	// 如果還是無法獲取，返回錯誤
	if initialBalance == 0 {
		log.Printf("❌ [%s] 无法获取初始余额", trader.GetName())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "无法获取初始余额",
		})
		return
	}

	var history []EquityPoint
	for _, record := range records {
		// TotalBalance字段实际存储的是TotalEquity
		totalEquity := record.AccountState.TotalBalance
		// TotalUnrealizedProfit字段实际存储的是TotalPnL（相对初始余额）
		totalPnL := record.AccountState.TotalUnrealizedProfit

		// 计算盈亏百分比
		totalPnLPct := 0.0
		if initialBalance > 0 {
			totalPnLPct = (totalPnL / initialBalance) * 100
		}

		history = append(history, EquityPoint{
			Timestamp:        record.Timestamp.Format("2006-01-02 15:04:05"),
			TotalEquity:      totalEquity,
			AvailableBalance: record.AccountState.AvailableBalance,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			PositionCount:    record.AccountState.PositionCount,
			MarginUsedPct:    record.AccountState.MarginUsedPct,
			CycleNumber:      record.CycleNumber,
		})
	}

	c.JSON(http.StatusOK, history)
}

// handlePerformance AI历史表现分析（用于展示AI学习和反思）
func (s *Server) handlePerformance(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 分析最近2880个周期的交易表现（避免长期持仓的交易记录丢失）
	// 假设每3分钟一个周期，2880个周期 = 6天，足够覆盖大部分交易
	// ⚠️ 修正：增加回溯窗口，确保能包含完整的交易周期
	performance, err := trader.GetDecisionLogger().AnalyzePerformance(2880)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("分析历史表现失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, performance)
}

// authMiddleware JWT认证中间件
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少Authorization头"})
			c.Abort()
			return
		}

		// 检查Bearer token格式
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的Authorization格式"})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// 黑名单检查
		if auth.IsTokenBlacklisted(tokenString) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token已失效，请重新登录"})
			c.Abort()
			return
		}

		// 验证JWT token
		claims, err := auth.ValidateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token: " + err.Error()})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}

// handleAdminLogin 管理员登录（密码仅来自环境变量）
func (s *Server) handleAdminLogin(c *gin.Context) {
	if !auth.IsAdminMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员模式可用"})
		return
	}

	// 简单的IP速率限制（5次/分钟 + 递增退避）
	// 为简化，此处省略复杂实现，可在后续使用中间件或Redis增强

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少密码"})
		return
	}
	if !auth.CheckAdminPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	token, err := auth.GenerateJWT("admin", "admin@localhost")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user_id": "admin", "email": "admin@localhost"})
}

// handleLogout 将当前token加入黑名单
func (s *Server) handleLogout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少Authorization头"})
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的Authorization格式"})
		return
	}
	tokenString := parts[1]
	claims, err := auth.ValidateJWT(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token"})
		return
	}
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	} else {
		exp = time.Now().Add(24 * time.Hour)
	}
	auth.BlacklistToken(tokenString, exp)
	c.JSON(http.StatusOK, gin.H{"message": "已登出"})
}

// handleRegister 处理用户注册请求
func (s *Server) handleRegister(c *gin.Context) {
	// 管理员模式下禁用注册
	if auth.IsAdminMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "管理员模式下禁用注册"})
		return
	}

	// 若未开启注册，返回403
	allowRegStr, _ := s.database.GetSystemConfig("allow_registration")
	if allowRegStr == "false" {
		c.JSON(http.StatusForbidden, gin.H{"error": "注册已关闭"})
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
		BetaCode string `json:"beta_code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查是否开启了内测模式
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr == "true" {
		// 内测模式下必须提供有效的内测码
		if req.BetaCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "内测期间，注册需要提供内测码"})
			return
		}

		// 验证内测码
		isValid, err := s.database.ValidateBetaCode(req.BetaCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证内测码失败"})
			return
		}
		if !isValid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "内测码无效或已被使用"})
			return
		}
	}

	// 检查邮箱是否已存在
	_, err := s.database.GetUserByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "邮箱已被注册"})
		return
	}

	// 生成密码哈希
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	// 生成OTP密钥
	otpSecret, err := auth.GenerateOTPSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OTP密钥生成失败"})
		return
	}

	// 创建用户（未验证OTP状态）
	userID := uuid.New().String()
	user := &config.User{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: passwordHash,
		OTPSecret:    otpSecret,
		OTPVerified:  false,
	}

	err = s.database.CreateUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败: " + err.Error()})
		return
	}

	// 如果是内测模式，标记内测码为已使用
	betaModeStr2, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr2 == "true" && req.BetaCode != "" {
		err := s.database.UseBetaCode(req.BetaCode, req.Email)
		if err != nil {
			log.Printf("⚠️ 标记内测码为已使用失败: %v", err)
			// 这里不返回错误，因为用户已经创建成功
		} else {
			log.Printf("✓ 内测码 %s 已被用户 %s 使用", req.BetaCode, req.Email)
		}
	}

	// 返回OTP设置信息
	qrCodeURL := auth.GetOTPQRCodeURL(otpSecret, req.Email)
	c.JSON(http.StatusOK, gin.H{
		"user_id":     userID,
		"email":       req.Email,
		"otp_secret":  otpSecret,
		"qr_code_url": qrCodeURL,
		"message":     "请使用Google Authenticator扫描二维码并验证OTP",
	})
}

// handleCompleteRegistration 完成注册（验证OTP）
func (s *Server) handleCompleteRegistration(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OTP验证码错误"})
		return
	}

	// 更新用户OTP验证状态
	err = s.database.UpdateUserOTPVerified(req.UserID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户状态失败"})
		return
	}

	// 生成JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	// 初始化用户的默认模型和交易所配置
	err = s.initUserDefaultConfigs(user.ID)
	if err != nil {
		log.Printf("初始化用户默认配置失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "注册完成",
	})
}

// handleLogin 处理用户登录请求
func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	// 验证密码
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	// 检查OTP是否已验证
	if !user.OTPVerified {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":              "账户未完成OTP设置",
			"user_id":            user.ID,
			"requires_otp_setup": true,
		})
		return
	}

	// 返回需要OTP验证的状态
	c.JSON(http.StatusOK, gin.H{
		"user_id":      user.ID,
		"email":        user.Email,
		"message":      "请输入Google Authenticator验证码",
		"requires_otp": true,
	})
}

// handleVerifyOTP 验证OTP并完成登录
func (s *Server) handleVerifyOTP(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
		return
	}

	// 生成JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "登录成功",
	})
}

// handleResetPassword 重置密码（验证OTP）
func (s *Server) handleResetPassword(c *gin.Context) {
	// 管理员模式下禁用密码重置
	if auth.IsAdminMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "管理员模式下禁用密码重置"})
		return
	}

	var req struct {
		Email       string `json:"email" binding:"required,email"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
		OTPCode     string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OTP验证码错误"})
		return
	}

	// 生成新密码哈希
	passwordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	// 更新密码
	err = s.database.UpdateUserPassword(user.ID, passwordHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "密码重置成功",
	})
}

// initUserDefaultConfigs 为新用户初始化默认的模型和交易所配置
func (s *Server) initUserDefaultConfigs(userID string) error {
	// 注释掉自动创建默认配置，让用户手动添加
	// 这样新用户注册后不会自动有配置项
	log.Printf("用户 %s 注册完成，等待手动配置AI模型和交易所", userID)
	return nil
}

// handleGetSupportedModels 获取系统支持的AI模型列表
func (s *Server) handleGetSupportedModels(c *gin.Context) {
	// 返回系统支持的AI模型（从default用户获取）
	models, err := s.database.GetAIModels("default")
	if err != nil {
		log.Printf("❌ 获取支持的AI模型失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取支持的AI模型失败"})
		return
	}

	c.JSON(http.StatusOK, models)
}

// handleGetSupportedExchanges 获取系统支持的交易所列表
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// 返回系统支持的交易所（从default用户获取）
	exchanges, err := s.database.GetExchanges("default")
	if err != nil {
		log.Printf("❌ 获取支持的交易所失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取支持的交易所失败"})
		return
	}

	c.JSON(http.StatusOK, exchanges)
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🌐 API服务器启动在 http://localhost%s", addr)
	log.Printf("📊 API文档:")
	log.Printf("  • GET  /api/health           - 健康检查")
	log.Printf("  • GET  /api/traders          - 公开的AI交易员排行榜前50名（无需认证）")
	log.Printf("  • GET  /api/competition      - 公开的竞赛数据（无需认证）")
	log.Printf("  • GET  /api/top-traders      - 前5名交易员数据（无需认证，表现对比用）")
	log.Printf("  • GET  /api/equity-history?trader_id=xxx - 公开的收益率历史数据（无需认证，竞赛用）")
	log.Printf("  • GET  /api/equity-history-batch?trader_ids=a,b,c - 批量获取历史数据（无需认证，表现对比优化）")
	log.Printf("  • GET  /api/traders/:id/public-config - 公开的交易员配置（无需认证，不含敏感信息）")
	log.Printf("  • POST /api/traders          - 创建新的AI交易员")
	log.Printf("  • DELETE /api/traders/:id    - 删除AI交易员")
	log.Printf("  • POST /api/traders/:id/start - 启动AI交易员")
	log.Printf("  • POST /api/traders/:id/stop  - 停止AI交易员")
	log.Printf("  • GET  /api/models           - 获取AI模型配置")
	log.Printf("  • PUT  /api/models           - 更新AI模型配置")
	log.Printf("  • GET  /api/model-providers  - 内置 provider catalog")
	log.Printf("  • POST /api/models/probe     - 探测 GET /v1/models")
	log.Printf("  • GET  /api/exchanges        - 获取交易所配置")
	log.Printf("  • PUT  /api/exchanges        - 更新交易所配置")
	log.Printf("  • GET  /api/status?trader_id=xxx     - 指定trader的系统状态")
	log.Printf("  • GET  /api/account?trader_id=xxx    - 指定trader的账户信息")
	log.Printf("  • GET  /api/positions?trader_id=xxx  - 指定trader的持仓列表")
	log.Printf("  • GET  /api/decisions?trader_id=xxx  - 指定trader的决策日志")
	log.Printf("  • GET  /api/decisions/latest?trader_id=xxx - 指定trader的最新决策")
	log.Printf("  • GET  /api/statistics?trader_id=xxx - 指定trader的统计信息")
	log.Printf("  • GET  /api/performance?trader_id=xxx - 指定trader的AI学习表现分析")
	s.mu.Lock()
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	s.mu.Unlock()

	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpServer != nil {
		log.Println("⏹ 正在优雅关闭 API 服务器...")
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// handleGetPromptTemplates 获取所有系统提示词模板列表
func (s *Server) handleGetPromptTemplates(c *gin.Context) {
	// 导入 decision 包
	templates := decision.GetAllPromptTemplates()

	// 转换为响应格式
	response := make([]map[string]interface{}, 0, len(templates))
	for _, tmpl := range templates {
		response = append(response, map[string]interface{}{
			"name": tmpl.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": response,
	})
}

// handleGetPromptTemplate 获取指定名称的提示词模板内容
func (s *Server) handleGetPromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	template, err := decision.GetPromptTemplate(templateName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("模板不存在: %s", templateName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    template.Name,
		"content": template.Content,
	})
}

// handlePublicTraderList 获取公开的交易员列表（无需认证）
func (s *Server) handlePublicTraderList(c *gin.Context) {
	// 从所有用户获取交易员信息
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取交易员列表失败: %v", err),
		})
		return
	}

	// 获取traders数组
	tradersData, exists := competition["traders"]
	if !exists {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	traders, ok := tradersData.([]map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "交易员数据格式错误",
		})
		return
	}

	// 返回交易员基本信息，过滤敏感信息
	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		result = append(result, map[string]interface{}{
			"trader_id":       trader["trader_id"],
			"trader_name":     trader["trader_name"],
			"ai_model":        trader["ai_model"],
			"exchange":        trader["exchange"],
			"is_running":      trader["is_running"],
			"total_equity":    trader["total_equity"],
			"total_pnl":       trader["total_pnl"],
			"total_pnl_pct":   trader["total_pnl_pct"],
			"position_count":  trader["position_count"],
			"margin_used_pct": trader["margin_used_pct"],
		})
	}

	c.JSON(http.StatusOK, result)
}

// handlePublicCompetition 获取公开的竞赛数据（无需认证）
func (s *Server) handlePublicCompetition(c *gin.Context) {
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取竞赛数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleTopTraders 获取前5名交易员数据（无需认证，用于表现对比）
func (s *Server) handleTopTraders(c *gin.Context) {
	topTraders, err := s.traderManager.GetTopTradersData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取前10名交易员数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, topTraders)
}

// handleEquityHistoryBatch 批量获取多个交易员的收益率历史数据（无需认证，用于表现对比）
func (s *Server) handleEquityHistoryBatch(c *gin.Context) {
	var requestBody struct {
		TraderIDs []string `json:"trader_ids"`
	}

	// 尝试解析POST请求的JSON body
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// 如果JSON解析失败，尝试从query参数获取（兼容GET请求）
		traderIDsParam := c.Query("trader_ids")
		if traderIDsParam == "" {
			// 如果没有指定trader_ids，则返回前5名的历史数据
			topTraders, err := s.traderManager.GetTopTradersData()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("获取前5名交易员失败: %v", err),
				})
				return
			}

			traders, ok := topTraders["traders"].([]map[string]interface{})
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "交易员数据格式错误"})
				return
			}

			// 提取trader IDs
			traderIDs := make([]string, 0, len(traders))
			for _, trader := range traders {
				if traderID, ok := trader["trader_id"].(string); ok {
					traderIDs = append(traderIDs, traderID)
				}
			}

			result := s.getEquityHistoryForTraders(traderIDs)
			c.JSON(http.StatusOK, result)
			return
		}

		// 解析逗号分隔的trader IDs
		requestBody.TraderIDs = strings.Split(traderIDsParam, ",")
		for i := range requestBody.TraderIDs {
			requestBody.TraderIDs[i] = strings.TrimSpace(requestBody.TraderIDs[i])
		}
	}

	// 限制最多20个交易员，防止请求过大
	if len(requestBody.TraderIDs) > 20 {
		requestBody.TraderIDs = requestBody.TraderIDs[:20]
	}

	result := s.getEquityHistoryForTraders(requestBody.TraderIDs)
	c.JSON(http.StatusOK, result)
}

// getEquityHistoryForTraders 获取多个交易员的历史数据
func (s *Server) getEquityHistoryForTraders(traderIDs []string) map[string]interface{} {
	result := make(map[string]interface{})
	histories := make(map[string]interface{})
	errors := make(map[string]string)

	for _, traderID := range traderIDs {
		if traderID == "" {
			continue
		}

		trader, err := s.traderManager.GetTrader(traderID)
		if err != nil {
			errors[traderID] = "交易员不存在"
			continue
		}

		// 获取历史数据（用于对比展示，限制数据量）
		records, err := trader.GetDecisionLogger().GetLatestRecords(500)
		if err != nil {
			errors[traderID] = fmt.Sprintf("获取历史数据失败: %v", err)
			continue
		}

		// ✅ 修正：獲取交易員的初始餘額
		initialBalance := 0.0
		if status := trader.GetStatus(); status != nil {
			if ib, ok := status["initial_balance"].(float64); ok && ib > 0 {
				initialBalance = ib
			}
		}

		// 构建收益率历史数据
		history := make([]map[string]interface{}, 0, len(records))
		for _, record := range records {
			// 计算总权益（余额+未实现盈亏）
			totalEquity := record.AccountState.TotalBalance + record.AccountState.TotalUnrealizedProfit

			history = append(history, map[string]interface{}{
				"timestamp":       record.Timestamp,
				"total_equity":    totalEquity,
				"total_pnl":       record.AccountState.TotalUnrealizedProfit,
				"balance":         record.AccountState.TotalBalance,
				"initial_balance": initialBalance, // 添加初始餘額
			})
		}

		// 將初始餘額也放在頂層，方便前端使用
		histories[traderID] = map[string]interface{}{
			"initial_balance": initialBalance,
			"data":            history,
		}
	}

	result["histories"] = histories
	result["count"] = len(histories)
	if len(errors) > 0 {
		result["errors"] = errors
	}

	return result
}

// handleGetPublicTraderConfig 获取公开的交易员配置信息（无需认证，不包含敏感信息）
func (s *Server) handleGetPublicTraderConfig(c *gin.Context) {
	traderID := c.Param("id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 获取交易员的状态信息
	status := trader.GetStatus()

	// 只返回公开的配置信息，不包含API密钥等敏感数据
	result := map[string]interface{}{
		"trader_id":   trader.GetID(),
		"trader_name": trader.GetName(),
		"ai_model":    trader.GetAIModel(),
		"exchange":    trader.GetExchange(),
		"is_running":  status["is_running"],
		"ai_provider": status["ai_provider"],
		"start_time":  status["start_time"],
	}

	c.JSON(http.StatusOK, result)
}

// handleReloadPrompts 热重载Prompt模板（无需重启服务）
func (s *Server) handleReloadPrompts(c *gin.Context) {
	log.Printf("🔄 收到Prompt热重载请求...")

	// 调用 decision 包的重载函数
	err := decision.ReloadPromptTemplates()
	if err != nil {
		log.Printf("❌ Prompt模板重载失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "重载Prompt模板失败",
			"details": err.Error(),
		})
		return
	}

	// 获取重载后的模板列表
	templateNames := decision.GetAllPromptTemplateNames()

	log.Printf("✓ Prompt模板重载成功，共加载 %d 个模板", len(templateNames))

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Prompt模板重载成功",
		"templates": templateNames,
		"count":     len(templateNames),
	})
}

func (s *Server) handleGetOperatorDirective(c *gin.Context) {
	now := time.Now()
	dir, err := s.database.CurrentOperatorDirective(now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"directive": dir,
		"digest":    config.OperatorDigest(dir, now),
	})
}

func (s *Server) handleListOperatorEvents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	events, err := s.database.ListOperatorEvents(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

type createOperatorEventRequest struct {
	Actor             string `json:"actor"`
	Action            string `json:"action" binding:"required"`
	Note              string `json:"note"`
	ExpiresInMinutes  *int   `json:"expires_in_minutes"`
}

func (s *Server) handleCreateOperatorEvent(c *gin.Context) {
	var req createOperatorEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	action, err := config.NormalizeOperatorAction(req.Action)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var exp *time.Time
	switch {
	case req.ExpiresInMinutes != nil && *req.ExpiresInMinutes < 0:
		exp = nil
	case req.ExpiresInMinutes != nil && *req.ExpiresInMinutes > 0:
		t := time.Now().UTC().Add(time.Duration(*req.ExpiresInMinutes) * time.Minute)
		exp = &t
	case action == config.OperatorPauseOpens:
		t := time.Now().UTC().Add(4 * time.Hour)
		exp = &t
	case action == config.OperatorNote:
		t := time.Now().UTC().Add(12 * time.Hour)
		exp = &t
	}

	ev, err := s.database.InsertOperatorEvent(req.Actor, action, req.Note, exp)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	now := time.Now()
	dir, _ := s.database.CurrentOperatorDirective(now)
	log.Printf("✓ operator event id=%d actor=%s action=%s", ev.ID, ev.Actor, ev.Action)
	c.JSON(http.StatusOK, gin.H{
		"event":     ev,
		"directive": dir,
		"digest":    config.OperatorDigest(dir, now),
	})
}
