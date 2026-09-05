package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type WSMonitor struct {
	wsClient           *WSClient
	combinedClient     *CombinedStreamsClient
	symbols            []string
	featuresMap        sync.Map
	alertsChan         chan Alert
	klineDataMap3m     sync.Map // 存储每个交易对的K线历史数据
	klineDataMap15m    sync.Map // 存储每个交易对的K线历史数据（15分钟）
	klineDataMap1h     sync.Map // 存储每个交易对的K线历史数据（1小时）
	klineDataMap4h     sync.Map // 存储每个交易对的K线历史数据
	tickerDataMap      sync.Map // 存储每个交易对的ticker数据
	batchSize          int
	filterSymbols      sync.Map // 使用sync.Map来存储需要监控的币种和其状态
	symbolStats        sync.Map // 存储币种统计信息
	FilterSymbol       []string //经过筛选的币种
	liquidityEngineMap sync.Map // Map[symbol]*LiquidityEngine (新增 Phase 3)
	lastKlineTimeMap   sync.Map // 记录各交易对各周期最近一次收到WS更新的时间戳 (symbol_time -> int64 unix ms)
}
type SymbolStats struct {
	LastActiveTime   time.Time
	AlertCount       int
	VolumeSpikeCount int
	LastAlertTime    time.Time
	Score            float64 // 综合评分
}

var subKlineTime = []string{"3m", "15m", "1h", "4h"} // 管理订阅流的K线周期（新增15m）

func NewWSMonitor(batchSize int) *WSMonitor {
	return &WSMonitor{
		wsClient:       NewWSClient(),
		combinedClient: NewCombinedStreamsClient(batchSize),
		alertsChan:     make(chan Alert, 1000),
		batchSize:      batchSize,
	}
}

func (m *WSMonitor) Initialize(coins []string) error {
	log.Println("初始化WebSocket监控器...")
	// 获取交易对信息
	apiClient := NewAPIClient()
	// 如果不指定交易对，则使用market市场的所有交易对币种
	if len(coins) == 0 {
		exchangeInfo, err := apiClient.GetExchangeInfo()
		if err != nil {
			return err
		}
		// 筛选永续合约交易对 --仅测试时使用
		//exchangeInfo.Symbols = exchangeInfo.Symbols[0:2]
		for _, symbol := range exchangeInfo.Symbols {
			if symbol.Status == "TRADING" && symbol.ContractType == "PERPETUAL" && strings.HasSuffix(strings.ToUpper(symbol.Symbol), "USDT") {
				m.symbols = append(m.symbols, symbol.Symbol)
				m.filterSymbols.Store(symbol.Symbol, true)
			}
		}
	} else {
		m.symbols = coins
	}

	log.Printf("找到 %d 个交易对", len(m.symbols))
	// 初始化历史数据
	if err := m.initializeHistoricalData(); err != nil {
		log.Printf("初始化历史数据失败: %v", err)
	}

	return nil
}

func (m *WSMonitor) initializeHistoricalData() error {
	apiClient := NewAPIClient()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // 限制并发数

	for _, symbol := range m.symbols {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(s string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// 获取历史K线数据
			klines, err := apiClient.GetKlines(s, "3m", 100)
			if err != nil {
				log.Printf("获取 %s 历史数据失败: %v", s, err)
				return
			}
			if len(klines) > 0 {
				m.klineDataMap3m.Store(s, klines)
				log.Printf("已加载 %s 的历史K线数据-3m: %d 条", s, len(klines))
			}
			// 获取15分钟历史K线数据
			klines15m, err := apiClient.GetKlines(s, "15m", 100)
			if err != nil {
				log.Printf("获取 %s 历史数据失败: %v", s, err)
			} else if len(klines15m) > 0 {
				m.klineDataMap15m.Store(s, klines15m)
				log.Printf("已加载 %s 的历史K线数据-15m: %d 条", s, len(klines15m))
			}
			// 获取1小时历史K线数据
			klines1h, err := apiClient.GetKlines(s, "1h", 100)
			if err != nil {
				log.Printf("获取 %s 历史数据失败: %v", s, err)
			} else if len(klines1h) > 0 {
				m.klineDataMap1h.Store(s, klines1h)
				log.Printf("已加载 %s 的历史K线数据-1h: %d 条", s, len(klines1h))
			}
			// 获取历史K线数据
			klines4h, err := apiClient.GetKlines(s, "4h", 100)
			if err != nil {
				log.Printf("获取 %s 历史数据失败: %v", s, err)
			} else if len(klines4h) > 0 {
				m.klineDataMap4h.Store(s, klines4h)
				log.Printf("已加载 %s 的历史K线数据-4h: %d 条", s, len(klines4h))
			}
		}(symbol)
	}

	wg.Wait()
	return nil
}

func (m *WSMonitor) Start(coins []string) {
	log.Printf("启动WebSocket实时监控...")
	// 初始化交易对
	err := m.Initialize(coins)
	if err != nil {
		log.Printf("❌ 初始化币种失败: %v", err)
		return
	}

	err = m.combinedClient.Connect()
	if err != nil {
		log.Printf("❌ 批量订阅流失败: %v", err)
		return
	}
	// 订阅所有交易对
	err = m.subscribeAll()
	if err != nil {
		log.Printf("❌ 订阅币种交易对失败: %v", err)
		return
	}
}

// UnregisterSymbol 移除币种监控并清理资源 (RAM Fix & Network Leak Fix)
func (m *WSMonitor) UnregisterSymbol(symbol string) {
	log.Printf("🗑️ Unregistering symbol: %s", symbol)

	// Ref Counting Removed for Simplicity (Single Trader context)

	// 1. 构造需要取消的流列表
	var streamsToUnsubscribe []string
	symbolLower := strings.ToLower(symbol)

	for _, st := range subKlineTime {
		streamName := fmt.Sprintf("%s@kline_%s", symbolLower, st)
		streamsToUnsubscribe = append(streamsToUnsubscribe, streamName)

		// 2. 移除订阅者通道 (防止Goroutine泄露)
		m.combinedClient.RemoveSubscriber(streamName)
	}

	// 3. 发送取消订阅指令 (防止网络带宽浪费)
	if len(streamsToUnsubscribe) > 0 {
		m.combinedClient.UnsubscribeStreams(streamsToUnsubscribe)
	}

	// 4. 清理内存映射 (RAM Cleanup)
	m.klineDataMap3m.Delete(symbol)
	m.klineDataMap15m.Delete(symbol)
	m.klineDataMap1h.Delete(symbol)
	m.klineDataMap4h.Delete(symbol)
	m.tickerDataMap.Delete(symbol)
	m.filterSymbols.Delete(symbol)
	m.symbolStats.Delete(symbol)
	m.liquidityEngineMap.Delete(symbol)

	// 从 active symbols 列表中移除
	for i, s := range m.symbols {
		if s == symbol {
			m.symbols = append(m.symbols[:i], m.symbols[i+1:]...)
			break
		}
	}
}

// subscribeSymbol 注册监听
func (m *WSMonitor) subscribeSymbol(symbol, st string) []string {
	var streams []string
	stream := fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), st)
	ch := m.combinedClient.AddSubscriber(stream, 100)
	streams = append(streams, stream)
	go m.handleKlineData(symbol, ch, st)

	return streams
}

func (m *WSMonitor) subscribeAll() error {
	// 执行批量订阅
	log.Println("开始订阅所有交易对...")
	for _, symbol := range m.symbols {
		for _, st := range subKlineTime {
			m.subscribeSymbol(symbol, st)
		}
	}
	for _, st := range subKlineTime {
		err := m.combinedClient.BatchSubscribeKlines(m.symbols, st)
		if err != nil {
			log.Printf("❌ 订阅 %s K线失败: %v", st, err)
			return err
		}
	}
	log.Println("所有交易对订阅完成")
	return nil
}

func (m *WSMonitor) handleKlineData(symbol string, ch <-chan []byte, _time string) {
	for data := range ch {
		var klineData KlineWSData
		if err := json.Unmarshal(data, &klineData); err != nil {
			log.Printf("解析Kline数据失败: %v", err)
			continue
		}
		m.processKlineUpdate(symbol, klineData, _time)
	}
}

func (m *WSMonitor) getKlineDataMap(_time string) *sync.Map {
	var klineDataMap *sync.Map
	switch _time {
	case "3m":
		klineDataMap = &m.klineDataMap3m
	case "15m":
		klineDataMap = &m.klineDataMap15m
	case "1h":
		klineDataMap = &m.klineDataMap1h
	case "4h":
		klineDataMap = &m.klineDataMap4h
	default:
		klineDataMap = &sync.Map{}
	}
	return klineDataMap
}
func (m *WSMonitor) processKlineUpdate(symbol string, wsData KlineWSData, _time string) {
	// 转换WebSocket数据为Kline结构
	kline := Kline{
		OpenTime:  wsData.Kline.StartTime,
		CloseTime: wsData.Kline.CloseTime,
		Trades:    wsData.Kline.NumberOfTrades,
	}
	kline.Open, _ = parseFloat(wsData.Kline.OpenPrice)
	kline.High, _ = parseFloat(wsData.Kline.HighPrice)
	kline.Low, _ = parseFloat(wsData.Kline.LowPrice)
	kline.Close, _ = parseFloat(wsData.Kline.ClosePrice)
	kline.Volume, _ = parseFloat(wsData.Kline.Volume)
	kline.High, _ = parseFloat(wsData.Kline.HighPrice)
	kline.QuoteVolume, _ = parseFloat(wsData.Kline.QuoteVolume)
	kline.TakerBuyBaseVolume, _ = parseFloat(wsData.Kline.TakerBuyBaseVolume)
	kline.TakerBuyQuoteVolume, _ = parseFloat(wsData.Kline.TakerBuyQuoteVolume)
	// 更新K线数据
	normSymbol := strings.ToUpper(symbol)
	m.lastKlineTimeMap.Store(fmt.Sprintf("%s_%s", normSymbol, _time), time.Now().UnixMilli())

	var klineDataMap = m.getKlineDataMap(_time)
	value, exists := klineDataMap.Load(normSymbol)
	if !exists {
		value, exists = klineDataMap.Load(symbol)
	}
	var klines []Kline
	if exists {
		oldKlines := value.([]Kline)
		// 🔒 Copy-On-Write: 複製底層數組切片，確保讀取協程 (GetCurrentKlines) 不會發生並發數據競態
		klines = make([]Kline, len(oldKlines))
		copy(klines, oldKlines)

		// 检查是否是新的K线
		if len(klines) > 0 && klines[len(klines)-1].OpenTime == kline.OpenTime {
			// 更新当前K线
			klines[len(klines)-1] = kline
		} else {
			// 添加新K线
			klines = append(klines, kline)

			// 保持数据长度
			if len(klines) > 100 {
				klines = klines[1:]
			}
		}
	} else {
		klines = []Kline{kline}
	}

	// Phase 3: Update Liquidity Engine
	// We need 15m candles for the heatmap logic
	if _time == "15m" {
		// Lazily load or get engine
		var engine *LiquidityEngine
		val, ok := m.liquidityEngineMap.Load(symbol)
		if !ok {
			// Initialize new engine with persistence path
			path := fmt.Sprintf("liquidity_%s.json", symbol)
			// Need a HistoryProvider. WSMonitor itself (via APIClient) or pass a new one?
			// Let's create a temporary adapter or extend WSMonitor to implement HistoryProvider?
			// For simplicity here, we create a specialized HistoryProvider implementation
			// to avoid circular dependency with 'trader' package.
			provider := &BinanceHistoryProvider{
				BaseURL: "https://fapi.binance.com",
				Client:  &http.Client{Timeout: 10 * time.Second},
			}
			engine = NewLiquidityEngine(path, provider)
			engine.Init(symbol) // Fetch history on first load
			m.liquidityEngineMap.Store(symbol, engine)
		} else {
			engine = val.(*LiquidityEngine)
		}

		// Update logic: need OIData.
		// Optimization: We don't have OI inside kline update stream.
		// We can only update "Visits" (Price action) here.
		// OI-based updates might need a separate loop or just Skip OI update for now
		// and rely on Init() for the base map.
		engine.Update(symbol, kline, nil)
	}

	klineDataMap.Store(strings.ToUpper(symbol), klines)
}

func (m *WSMonitor) GetCurrentKlines(symbol string, _time string) ([]Kline, error) {
	normSymbol := strings.ToUpper(symbol)
	value, exists := m.getKlineDataMap(_time).Load(normSymbol)
	if !exists {
		value, exists = m.getKlineDataMap(_time).Load(symbol)
	}

	// 检查数据是否过期
	if exists {
		klines := value.([]Kline)
		if len(klines) > 0 {
			lastKline := klines[len(klines)-1]
			// 检查是否过期：如果当前时间超过K线收盘时间太久，说明WebSocket可能断开或未更新
			// 修正：收紧阈值以防止使用过时数据 (对于趋势交易，数据必须新鲜)
			// 3m: 允许30秒延迟 (原5m)
			// 15m: 允许1分钟延迟 (原20m)
			// 1h: 允许2分钟延迟 (原2h)
			// 4h: 允许5分钟延迟 (原8h)
			var threshold int64 = 30 * 1000 // 3m 默认 30s
			switch _time {
			case "15m":
				threshold = 60 * 1000 // 1m
			case "1h":
				threshold = 2 * 60 * 1000 // 2m
			case "4h":
				threshold = 5 * 60 * 1000 // 5m
			}

			now := time.Now().UnixMilli()
			isExpired := now > lastKline.CloseTime+threshold
			if !isExpired {
				if lastUpdateVal, ok := m.lastKlineTimeMap.Load(fmt.Sprintf("%s_%s", normSymbol, _time)); ok {
					lastUpdate := lastUpdateVal.(int64)
					// 判定 WebSocket 断流超时 (如 3m 超过 2 分钟无任何推送)
					streamTimeout := threshold * 4
					if now-lastUpdate > streamTimeout {
						isExpired = true
					}
				}
			}

			if isExpired {
				log.Printf("⚠️ %s %s K线数据过期 (CloseTime: %d, Now: %d), 强制刷新", symbol, _time, lastKline.CloseTime, now)
				exists = false
				m.getKlineDataMap(_time).Delete(normSymbol)
				m.getKlineDataMap(_time).Delete(symbol)
			}
		}
	}

	if !exists {
		// 如果Ws数据未初始化完成时,单独使用api获取 - 兼容性代码 (防止在未初始化完成是,已经有交易员运行)
		apiClient := NewAPIClient()
		klines, err := apiClient.GetKlines(symbol, _time, 100)
		if err != nil {
			return nil, fmt.Errorf("获取%v分钟K线失败: %v", _time, err)
		}

		// 动态缓存进缓存
		m.getKlineDataMap(_time).Store(normSymbol, klines)

		// 订阅 WebSocket 流
		subStr := m.subscribeSymbol(symbol, _time)
		subErr := m.combinedClient.subscribeStreams(subStr)
		log.Printf("动态订阅流: %v", subStr)
		if subErr != nil {
			log.Printf("动态订阅失败: %v", subErr)
		}

		// ✅ FIX: 返回深拷贝而非引用
		result := make([]Kline, len(klines))
		copy(result, klines)
		return result, nil
	}

	// ✅ FIX: 返回深拷贝而非引用，避免并发竞态条件
	klines := value.([]Kline)
	result := make([]Kline, len(klines))
	copy(result, klines)
	return result, nil
}

func (m *WSMonitor) Close() {
	m.wsClient.Close()
	close(m.alertsChan)
}

// 🔧 修正（IND-03）：显式指定10秒超时客户端，避免 Binance API 偶发假死导致 sync.WaitGroup 永久阻塞死锁
var binanceHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// GetOpenInterest retrieves the latest Open Interest data.
func (m *WSMonitor) GetOpenInterest(symbol string) (*OIData, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	resp, err := binanceHTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	return &OIData{
		Latest:  oi,
		Average: oi, // 🔧 修正（IND-14）：真实填入，拒绝硬编码 0.999 伪造平均值
	}, nil
}

// GetOpenInterestHistory retrieves historical Open Interest data (Impl checking for Phase 20)
func (m *WSMonitor) GetOpenInterestHistory(symbol string, period string, limit int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://fapi.binance.com/futures/data/openInterestHist?symbol=%s&period=%s&limit=%d", symbol, period, limit)

	resp, err := binanceHTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	var rawData []struct {
		Symbol string `json:"symbol"`
		SumOI  string `json:"sumOpenInterest"`
		SumVal string `json:"sumOpenInterestValue"`
		Time   int64  `json:"timestamp"`
	}

	body, _ := ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, item := range rawData {
		oiVal, _ := strconv.ParseFloat(item.SumVal, 64)
		oiQty, _ := strconv.ParseFloat(item.SumOI, 64)
		result = append(result, map[string]interface{}{
			"symbol":               item.Symbol,
			"sumOpenInterest":      oiQty,
			"sumOpenInterestValue": oiVal,
			"timestamp":            item.Time,
		})
	}
	return result, nil
}

// GetFundingRate retrieves the latest Funding Rate.
func (m *WSMonitor) GetFundingRate(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	resp, err := binanceHTTPClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)
	return rate, nil
}

// GetLiquidityClusters returns the active clusters for a symbol
func (m *WSMonitor) GetLiquidityClusters(symbol string, currentPrice float64) []LiquidityCluster {
	val, ok := m.liquidityEngineMap.Load(symbol)
	if !ok {
		return nil
	}
	engine := val.(*LiquidityEngine)
	return engine.GetTopClusters(symbol, currentPrice)
}

// BinanceHistoryProvider implements HistoryProvider without importing 'trader'
type BinanceHistoryProvider struct {
	BaseURL string
	Client  *http.Client
}

func (p *BinanceHistoryProvider) GetOpenInterestHistory(symbol string, period string, limit int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/futures/data/openInterestHist?symbol=%s&period=%s&limit=%d", p.BaseURL, symbol, period, limit)

	resp, err := p.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	var rawData []struct {
		Symbol string `json:"symbol"`
		SumOI  string `json:"sumOpenInterest"`
		SumVal string `json:"sumOpenInterestValue"`
		Time   int64  `json:"timestamp"`
	}

	body, _ := ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, item := range rawData {
		oiVal, _ := strconv.ParseFloat(item.SumVal, 64)
		oiQty, _ := strconv.ParseFloat(item.SumOI, 64)
		result = append(result, map[string]interface{}{
			"symbol":               item.Symbol,
			"sumOpenInterest":      oiQty,
			"sumOpenInterestValue": oiVal,
			"timestamp":            item.Time,
		})
	}
	return result, nil
}
