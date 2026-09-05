package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Get 获取指定代币的市场数据
func Get(symbol string, provider DataProvider, le *LiquidityEngine) (*Data, error) {
	// 标准化symbol
	symbol = Normalize(symbol)

	var (
		klines3m, klines15m                                                               []Kline
		klines1h, klines4h                                                                []Kline
		oiData                                                                            *OIData
		oiHistory                                                                         []map[string]interface{}
		fundingRate                                                                       float64
		errKlines3m, errKlines15m, errKlines1h, errKlines4h, errOI, errOIHist, errFunding error
	)

	var wg sync.WaitGroup
	wg.Add(7) // 3m, 15m, 1h, 4h, OI, OIHist, Funding

	safeGo := func(fn func()) {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("❌ 获取市场数据 goroutine 异常 panic (%s): %v", symbol, r)
				}
				wg.Done()
			}()
			fn()
		}()
	}

	safeGo(func() { klines3m, errKlines3m = provider.GetCurrentKlines(symbol, "3m") })
	safeGo(func() { klines15m, errKlines15m = provider.GetCurrentKlines(symbol, "15m") })
	safeGo(func() { klines1h, errKlines1h = provider.GetCurrentKlines(symbol, "1h") })
	safeGo(func() { klines4h, errKlines4h = provider.GetCurrentKlines(symbol, "4h") })

	safeGo(func() {
		oiData, errOI = provider.GetOpenInterest(symbol)
		if errOI != nil {
			log.Printf("GetOpenInterest failed: %v", errOI)
			oiData = &OIData{Latest: 0, Average: 0}
		}
	})

	safeGo(func() {
		oiHistory, errOIHist = provider.GetOpenInterestHistory(symbol, "15m", 10) // Fetch last 10 candles for Delta
		if errOIHist != nil {
			log.Printf("GetOpenInterestHistory failed: %v", errOIHist)
		}
	})

	safeGo(func() {
		fundingRate, errFunding = provider.GetFundingRate(symbol)
		if errFunding != nil {
			log.Printf("GetFundingRate failed: %v", errFunding)
		}
	})

	wg.Wait()

	// Check for critical errors (Klines are critical)
	if errKlines3m != nil {
		return nil, fmt.Errorf("getting 3m klines failed: %v", errKlines3m)
	}
	if errKlines15m != nil {
		return nil, fmt.Errorf("getting 15m klines failed: %v", errKlines15m)
	}
	if errKlines1h != nil {
		return nil, fmt.Errorf("getting 1h klines failed: %v", errKlines1h)
	}
	if errKlines4h != nil {
		return nil, fmt.Errorf("getting 4h klines failed: %v", errKlines4h)
	}

	// 计算当前指标 (基于3分钟最新数据)
	currentPrice := klines3m[len(klines3m)-1].Close
	currentEMA20 := calculateEMA(klines3m, 20)
	currentMACD, currentSignal, currentHist := calculateMACDSeries(klines3m)
	currentRSI7 := calculateRSI(klines3m, 7)

	// 计算15分钟指标
	ema15m := calculateEMA(klines15m, 20)
	macd15m, signal15m, hist15m := calculateMACDSeries(klines15m)
	rsi15m := calculateRSI(klines15m, 14) // 通常使用RSI 14

	// 计算价格变化百分比
	// 1小时价格变化 = 20个3分钟K线前的价格
	priceChange1h := 0.0
	if len(klines3m) >= 21 { // 至少需要21根K线 (当前 + 20根前)
		price1hAgo := klines3m[len(klines3m)-21].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}

	// 4小时价格变化 = 16个15分钟K线前（或4个1小时K线前）的价格
	priceChange4h := 0.0
	if len(klines15m) >= 17 {
		price4hAgo := klines15m[len(klines15m)-17].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	} else if len(klines1h) >= 5 {
		price4hAgo := klines1h[len(klines1h)-5].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	} else if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// 计算BuySellRatio（买卖比例）
	// BuySellRatio = TakerBuyBaseVolume / Volume
	// >0.5表示买方力量强，<0.5表示卖方力量强
	buySellRatio := 0.0
	latestKline := klines3m[len(klines3m)-1]
	if latestKline.Volume > 0 {
		buySellRatio = latestKline.TakerBuyBaseVolume / latestKline.Volume
	}

	// CoinGecko 数据（免费API，可选）
	// 注意：这里暂时跳过，改为在fetchMarketDataForContext中批量获取
	// coinGeckoData, _ := getCoinGeckoData(symbol)
	var coinGeckoData *CoinGeckoData = nil

	// Fetch Fear & Greed Index (Global Sentiment)
	// sentiment, _ := getCryptoFearGreedIndex() // Removed: Fetched globally in engine.go

	// 计算日内系列数据
	intradayData := calculateIntradaySeries(klines3m)

	// 计算1小时数据（新增）
	hourlyData := calculateHourlyData(klines1h)

	// 计算K线完成度 (Candle Progress)
	latest1h := klines1h[len(klines1h)-1]
	now := time.Now().UnixMilli()
	// 使用 Kline 的 CloseTime (Binance CloseTime 是收盘时间戳)
	// OpenTime + 1h 也是理论上的收盘时间
	// 避免时间同步误差，直接计算 elapsed / duration
	duration := float64(latest1h.CloseTime - latest1h.OpenTime)
	elapsed := float64(now - latest1h.OpenTime)
	candleProgress := 0.0
	timeUntilClose := 0.0

	if duration > 0 {
		candleProgress = (elapsed / duration) * 100
		if candleProgress > 100 {
			candleProgress = 100
		} else if candleProgress < 0 {
			candleProgress = 0
		}
		timeUntilClose = (duration - elapsed) / 60000.0 // 转换为分钟
	}

	// 计算长期数据
	longerTermData := calculateLongerTermData(klines4h)

	// 计算价格行为数据 (🔧 修正IND-02：基于已收盘的1小时K线，避免新K线刚开盘实体极小时误判Doji/Pinbar导致重绘)
	var priceActionData *PriceActionData
	if len(klines1h) > 1 {
		priceActionData = calculatePriceAction(klines1h[:len(klines1h)-1], hourlyData.EMA20)
	} else {
		priceActionData = calculatePriceAction(klines1h, hourlyData.EMA20)
	}

	// 计算技术分析汇总 (逻辑固化)
	// 计算背离 (需要足够的 RSI 历史)
	// hourlyData.RSI14Values 应该有 30 个点 (修改 calculateHourlyData 后)
	divergence := "None"
	if len(hourlyData.RSI14Values) >= 20 {
		// 截取对应的 K 线历史 (最后 N 个)
		n := len(hourlyData.RSI14Values)
		if len(klines1h) >= n {
			relevantKlines := klines1h[len(klines1h)-n:]
			divergence = calculateRSIDivergence(relevantKlines, hourlyData.RSI14Values)
		}
	}

	var currentMACD1h, currentRSI141h float64
	if len(hourlyData.MACDValues) > 0 {
		currentMACD1h = hourlyData.MACDValues[len(hourlyData.MACDValues)-1]
	}
	if len(hourlyData.RSI14Values) > 0 {
		currentRSI141h = hourlyData.RSI14Values[len(hourlyData.RSI14Values)-1]
	}

	// Order Flow
	cvd := CalculateCVD(klines15m)

	// Phase 3: VPVR Calculation (Volume Profile)
	// Use last 200 candles (15m) for Intraday Profile? Or 100 1h candles?
	// Let's use 100 1h candles for a "Weekly/Session" Profile view.
	vpvr := calculateVPVR(klines1h, 100)

	// Phase 21: Liquidity Clusters
	var liquidityClusters []LiquidityCluster
	if le != nil {
		// Use Persistent Engine
		le.Init(symbol)
		// Update with latest confirmed closed candle (prevent lookahead repainting)
		if len(klines15m) > 1 {
			closed15m := klines15m[len(klines15m)-2]
			le.Update(symbol, closed15m, oiData)
		}
		// Use GetTopClusters (Sorted by Volume Desc, Top 5)
		liquidityClusters = le.GetTopClusters(symbol, currentPrice)
	} else {
		// Fallback to Lite Version
		liquidityClusters = calculateLiquidityClusters(klines1h)
		// Manually Sort and Truncate Fallback
		sort.Slice(liquidityClusters, func(i, j int) bool {
			return liquidityClusters[i].Volume > liquidityClusters[j].Volume
		})
		if len(liquidityClusters) > 5 {
			liquidityClusters = liquidityClusters[:5]
		}
	}

	// --- Upgrade: Calculate OI Delta (Phase 20) ---
	if oiData != nil && len(oiHistory) >= 2 {
		// oiHistory is map[string]interface{}, convert values
		// Assume sorted by time (Binance API sorts ascending)
		latest := oiHistory[len(oiHistory)-1]
		prev := oiHistory[len(oiHistory)-2]

		latestVal, _ := latest["sumOpenInterest"].(float64)
		prevVal, _ := prev["sumOpenInterest"].(float64)

		oiData.Delta15m = latestVal - prevVal // Simple Delta

		// 1H Delta (4 candles ago)
		if len(oiHistory) >= 5 {
			prev1h := oiHistory[len(oiHistory)-5]
			prev1hVal, _ := prev1h["sumOpenInterest"].(float64)
			oiData.Delta1H = latestVal - prev1hVal
		}
	}

	// Phase 19: VWAP
	vwap := calculateVWAP(klines15m)

	// Technical Analysis (Logic Hardened)
	techAnalysis := calculateTechnicalAnalysis(
		currentPrice,
		hourlyData.EMA20, // Use 1H EMA20 for Trend
		currentMACD1h,    // Use 1H MACD for Trend
		currentRSI141h,   // Use 1H RSI for Trend
		hourlyData.CurrentVolume,
		hourlyData.AverageVolume,
		priceActionData,
		divergence,
		hourlyData.MACDHistValues,
		vwap,
		cvd,
		oiData,            // Pass OI Data for Scoring
		liquidityClusters, // Phase 21
		&vpvr,
	)

	// 🧠 Upgrade: Calculate SMC Analysis (使用已收盤 K 線保證無未來函數，防止未收盤 K 棒造成虛假 BOS / Repainting)
	if len(klines15m) > 1 {
		techAnalysis.SMC = CalculateSMC(klines15m[:len(klines15m)-1])
		if techAnalysis.SMC != nil {
			// 拿當前實時最新市價檢驗是否打破已確認的結構
			if techAnalysis.SMC.Structure.Trend == "Bullish" {
				if currentPrice > techAnalysis.SMC.Structure.LastSwingHigh.Price {
					techAnalysis.SMC.Structure.BreakOfStructure = "Bullish"
				} else if currentPrice < techAnalysis.SMC.Structure.LastSwingLow.Price {
					techAnalysis.SMC.Structure.BreakOfStructure = "Bearish CHoCH"
				}
			} else if techAnalysis.SMC.Structure.Trend == "Bearish" {
				if currentPrice < techAnalysis.SMC.Structure.LastSwingLow.Price {
					techAnalysis.SMC.Structure.BreakOfStructure = "Bearish"
				} else if currentPrice > techAnalysis.SMC.Structure.LastSwingHigh.Price {
					techAnalysis.SMC.Structure.BreakOfStructure = "Bullish CHoCH"
				}
			}
		}
	} else {
		techAnalysis.SMC = CalculateSMC(klines15m)
	}

	// 🧠 Upgrade: Calculate Order Flow (CVD)
	techAnalysis.OrderFlow = CalculateCVD(klines15m)

	// 🧠 Upgrade: Macro Context (Phase 5)
	// We need to fetch 1D candles here.
	// Optimization: Do not fetch on every 3m cycle. Cache it?
	// For now, let's fetch it on every cycle to be safe, but use a separate routine if perf drags.
	// We need access to API client.
	// Simplification: We assume 'provider' has access or we create a fresh API client?
	// Creating Fresh API client is costly? No, strict http client.
	apiClient := NewAPIClient()
	klines1d, err := apiClient.GetKlines(symbol, "1d", 100) // Fetch 100 days
	var macro *MacroData
	if err == nil && len(klines1d) > 50 {
		macro = calculateMacroContext(klines1d)
	}

	return &Data{
		Symbol:            symbol,
		Timestamp:         time.UnixMilli(klines3m[len(klines3m)-1].CloseTime),
		CurrentPrice:      currentPrice,
		PriceChange1h:     priceChange1h,
		PriceChange4h:     priceChange4h,
		CurrentEMA20:      currentEMA20,
		CurrentMACD:       currentMACD,
		CurrentMACDSignal: currentSignal,
		CurrentMACDHist:   currentHist,
		CurrentRSI7:       currentRSI7,
		CandleProgress1H:  candleProgress,
		TimeUntilClose1H:  timeUntilClose,
		EMA15m:            ema15m,
		MACD15m:           macd15m,
		MACDSignal15m:     signal15m,
		MACDHist15m:       hist15m,
		RSI15m:            rsi15m,
		OpenInterest:      oiData,
		FundingRate:       fundingRate,
		BuySellRatio:      buySellRatio,
		IntradaySeries:    intradayData,
		HourlyContext:     hourlyData,
		LongerTermContext: longerTermData,
		CoinGeckoData:     coinGeckoData,
		PriceAction:       priceActionData,
		TechnicalAnalysis: techAnalysis,
		Sentiment:         nil,   // Sentiment is injected globally by engine
		Macro:             macro, // Phase 5
	}, nil
}

// calculateEMA 计算EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 计算SMA作为初始EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// 计算EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateMACDSeries 计算完整 MACD 系列 (DIF, Signal, Histogram)
// 返回: lastDIF, lastSignal, lastHist
func calculateMACDSeries(klines []Kline) (float64, float64, float64) {
	if len(klines) < 26 {
		return 0, 0, 0
	}

	// 1. Calculate DIF Series (EMA12 - EMA26) for the entire applicable range
	// We need enough DIF points to calculate Signal Line (EMA9 of DIF).
	// So we need at least 9 DIF points.
	// DIF[i] corresponds to Kline[i].

	difs := make([]float64, len(klines))

	// Pre-calculate EMAs? No, inefficient to recalculate for every point.
	// Better: Calculate EMA series.

	ema12s := calculateEMASeries(klines, 12)
	ema26s := calculateEMASeries(klines, 26)

	for i := 0; i < len(klines); i++ {
		if ema12s[i] != 0 && ema26s[i] != 0 {
			difs[i] = ema12s[i] - ema26s[i]
		}
	}

	// 2. Calculate Signal Line (EMA9 of DIF)
	// We treat DIFs as the input "Close" prices for EMA calculation.
	// Note: calculateEMA function expects []Kline, so we need a helper or modification.

	// Helper for float64 slice EMA
	signals := calculateEMAForSeries(difs, 9)

	// 3. Get latest values
	lastIdx := len(klines) - 1
	lastDIF := difs[lastIdx]
	lastSignal := signals[lastIdx]
	lastHist := lastDIF - lastSignal

	return lastDIF, lastSignal, lastHist
}

// calculateEMASeries 计算 EMA 序列
func calculateEMASeries(klines []Kline, period int) []float64 {
	result := make([]float64, len(klines))
	if len(klines) < period {
		return result
	}

	// SMA for first point
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	result[period-1] = sum / float64(period)

	multiplier := 2.0 / float64(period+1)

	for i := period; i < len(klines); i++ {
		result[i] = (klines[i].Close-result[i-1])*multiplier + result[i-1]
	}

	return result
}

// calculateEMAForSeries 针对 float64 数组计算 EMA (用于 Signal Line)
func calculateEMAForSeries(values []float64, period int) []float64 {
	result := make([]float64, len(values))
	// Find first non-zero index to start (as DIFs have leading zeros)
	startIdx := -1
	for i := 0; i < len(values); i++ {
		if values[i] != 0 {
			startIdx = i
			break
		}
	}

	if startIdx == -1 || len(values)-startIdx < period {
		return result
	}

	// Shift index to start from valid DIFs
	// BUT Signal Line EMA9 needs 9 points of DIF.
	// So data available from startIdx + period - 1.

	realStart := startIdx + period - 1
	if realStart >= len(values) {
		return result
	}

	sum := 0.0
	for i := startIdx; i < startIdx+period; i++ {
		sum += values[i]
	}
	result[realStart] = sum / float64(period)

	multiplier := 2.0 / float64(period+1)
	for i := realStart + 1; i < len(values); i++ {
		result[i] = (values[i]-result[i-1])*multiplier + result[i-1]
	}

	return result
}

// calculateRSI 计算RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	gains := 0.0
	losses := 0.0

	// 计算初始平均涨跌幅
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// 使用Wilder平滑方法计算后续RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateATR 计算ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 计算初始ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder平滑
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateIntradaySeries 计算日内系列数据
func calculateIntradaySeries(klines []Kline) *IntradayData {
	data := &IntradayData{
		MidPrices:        make([]float64, 0, 10),
		EMA20Values:      make([]float64, 0, 10),
		MACDValues:       make([]float64, 0, 10),
		MACDSignalValues: make([]float64, 0, 10), // 新增
		MACDHistValues:   make([]float64, 0, 10), // 新增
		RSI7Values:       make([]float64, 0, 10),
		RSI14Values:      make([]float64, 0, 10),
	}

	// 获取最近10个数据点
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)

		// 计算每个点的EMA20
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}

		// 计算每个点的MACD
		if i >= 25 {
			// Calculate series up to i
			dif, sig, hist := calculateMACDSeries(klines[:i+1])
			data.MACDValues = append(data.MACDValues, dif)
			data.MACDSignalValues = append(data.MACDSignalValues, sig)
			data.MACDHistValues = append(data.MACDHistValues, hist)
		}

		// 计算每个点的RSI
		if i >= 7 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	return data
}

// calculateTrimmedMean 计算去除最高和最低指定比例后的成交量平均值
func calculateTrimmedMean(klines []Kline, trimPercent float64) float64 {
	n := len(klines)
	if n == 0 {
		return 0
	}
	if n <= 2 {
		sum := 0.0
		for _, k := range klines {
			sum += k.Volume
		}
		return sum / float64(n)
	}

	// 复制成交量数值并进行排序
	volumes := make([]float64, n)
	for i, k := range klines {
		volumes[i] = k.Volume
	}
	sort.Float64s(volumes)

	// 计算两边各自切除的数量 k
	k := int(math.Round(float64(n) * trimPercent))
	// 确保两边切除后至少保留中间的数据计算平均值，防范数组越界
	if 2*k >= n {
		k = (n - 1) / 2
	}

	sum := 0.0
	count := 0
	for i := k; i < n-k; i++ {
		sum += volumes[i]
		count++
	}

	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// calculateTrimmedStdDev 计算去除最高和最低指定比例后的成交量标准差
func calculateTrimmedStdDev(klines []Kline, trimPercent float64, mean float64) float64 {
	n := len(klines)
	if n <= 1 {
		return 0
	}
	if n <= 2 {
		sumSq := 0.0
		for _, k := range klines {
			diff := k.Volume - mean
			sumSq += diff * diff
		}
		return math.Sqrt(sumSq / float64(n))
	}

	// 复制成交量数值并进行排序
	volumes := make([]float64, n)
	for i, k := range klines {
		volumes[i] = k.Volume
	}
	sort.Float64s(volumes)

	// 计算两边各自切除的数量 k
	k := int(math.Round(float64(n) * trimPercent))
	if 2*k >= n {
		k = (n - 1) / 2
	}

	sumSq := 0.0
	count := 0
	for i := k; i < n-k; i++ {
		diff := volumes[i] - mean
		sumSq += diff * diff
		count++
	}

	if count <= 1 {
		return 0
	}
	return math.Sqrt(sumSq / float64(count))
}

// calculateHourlyData 计算1小时数据（新增）
func calculateHourlyData(klines []Kline) *HourlyData {
	data := &HourlyData{
		MACDValues:       make([]float64, 0, 50),
		MACDSignalValues: make([]float64, 0, 50),
		MACDHistValues:   make([]float64, 0, 50),
		RSI14Values:      make([]float64, 0, 50),
	}

	// 计算EMA
	data.EMA20 = calculateEMA(klines, 20)
	data.EMA50 = calculateEMA(klines, 50)

	// 计算ATR
	// 🔧 修正：使用已收盘 K 线计算 ATR3 / ATR14，避免未走完的 K 线导致 ATR 严重失真 (ATR3 会被拉低 30%~50%)
	atrKlines := klines
	if len(klines) > 1 {
		atrKlines = klines[:len(klines)-1]
	}
	data.ATR3 = calculateATR(atrKlines, 3)
	data.ATR14 = calculateATR(atrKlines, 14)

	// 计算成交量
	// 🔧 修正（v5.6.0）：使用倒数第2根已收盘的 K 线计算 Z-Score，而非当前未走完的 K 线。
	// 原因：当前 K 线可能只走了 10~30 分钟，成交量天然只有完整 K 线的 1/6 ~ 1/2，
	// 直接与已收盘均量比较会导致 Z-Score 系统性偏低（常态 < -2.0），
	// 触发 filters.md 的「强制禁止开仓」规则，导致全市场被错误锁定。
	if len(klines) > 1 {
		data.CurrentVolume = klines[len(klines)-2].Volume // 使用已收盘的完整 K 线
		// 改用 10% 去尾平均数，以剔除极端成交量尖峰与冷清噪音
		// 注意：均值计算排除最后一根未收盘 K 线
		closedKlines := klines[:len(klines)-1]
		data.AverageVolume = calculateTrimmedMean(closedKlines, 0.10)
		trimmedStdDev := calculateTrimmedStdDev(closedKlines, 0.10, data.AverageVolume)
		if trimmedStdDev > 0 {
			data.VolumeZScore = (data.CurrentVolume - data.AverageVolume) / trimmedStdDev
		} else {
			data.VolumeZScore = 0
		}
	} else if len(klines) > 0 {
		data.CurrentVolume = klines[len(klines)-1].Volume
		data.AverageVolume = data.CurrentVolume
		data.VolumeZScore = 0
	}

	// 计算MACD和RSI序列
	// 增加历史长度以支持背离计算 (30个点)
	start := len(klines) - 30
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		if i >= 25 {
			dif, sig, hist := calculateMACDSeries(klines[:i+1])
			data.MACDValues = append(data.MACDValues, dif)
			data.MACDSignalValues = append(data.MACDSignalValues, sig)
			data.MACDHistValues = append(data.MACDHistValues, hist)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	// 計算布林帶 (1H 體制識別)
	data.BB = calculateBollingerBands(klines, 20, 2.0)

	return data
}

// calculateLongerTermData 计算长期数据
func calculateLongerTermData(klines []Kline) *LongerTermData {
	data := &LongerTermData{
		MACDValues:       make([]float64, 0, 10),
		MACDSignalValues: make([]float64, 0, 10),
		MACDHistValues:   make([]float64, 0, 10),
		RSI14Values:      make([]float64, 0, 10),
	}

	// 计算EMA
	data.EMA20 = calculateEMA(klines, 20)
	data.EMA50 = calculateEMA(klines, 50)

	// 计算ATR
	// 🔧 修正：使用已收盘 K 线计算 ATR
	atrKlines := klines
	if len(klines) > 1 {
		atrKlines = klines[:len(klines)-1]
	}
	data.ATR3 = calculateATR(atrKlines, 3)
	data.ATR14 = calculateATR(atrKlines, 14)

	// 计算成交量（同第一处修正：使用已收盘 K 线）
	if len(klines) > 1 {
		data.CurrentVolume = klines[len(klines)-2].Volume
		closedKlines := klines[:len(klines)-1]
		data.AverageVolume = calculateTrimmedMean(closedKlines, 0.10)
		trimmedStdDev := calculateTrimmedStdDev(closedKlines, 0.10, data.AverageVolume)
		if trimmedStdDev > 0 {
			data.VolumeZScore = (data.CurrentVolume - data.AverageVolume) / trimmedStdDev
		} else {
			data.VolumeZScore = 0
		}
	} else if len(klines) > 0 {
		data.CurrentVolume = klines[len(klines)-1].Volume
		data.AverageVolume = data.CurrentVolume
		data.VolumeZScore = 0
	}

	// 计算MACD和RSI序列
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		if i >= 25 {
			dif, sig, hist := calculateMACDSeries(klines[:i+1])
			data.MACDValues = append(data.MACDValues, dif)
			data.MACDSignalValues = append(data.MACDSignalValues, sig)
			data.MACDHistValues = append(data.MACDHistValues, hist)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	// 計算布林帶 (4H 體制識別)
	data.BB = calculateBollingerBands(klines, 20, 2.0)

	return data
}

// calculateBollingerBands 計算布林帶數據（用於體制識別，非入場信號）
// period: SMA週期 (默認20), stdDev: 標準差倍數 (默認2.0)
func calculateBollingerBands(klines []Kline, period int, stdDev float64) *BollingerBands {
	if len(klines) < period {
		return nil
	}

	// 1. 計算收盤價序列
	closePrices := make([]float64, len(klines))
	for i, k := range klines {
		closePrices[i] = k.Close
	}

	// 2. 計算 SMA (中軌) - 使用最近 period 個價格
	recentPrices := closePrices[len(closePrices)-period:]
	sum := 0.0
	for _, p := range recentPrices {
		sum += p
	}
	sma := sum / float64(period)

	// 3. 計算標準差 (Population Standard Deviation)
	variance := 0.0
	for _, p := range recentPrices {
		diff := p - sma
		variance += diff * diff
	}
	variance /= float64(period)
	std := math.Sqrt(variance)

	// 4. 計算三軌
	upper := sma + (stdDev * std)
	lower := sma - (stdDev * std)

	// 5. 計算衍生指標
	bandwidth := 0.0
	if sma > 0 {
		bandwidth = ((upper - lower) / sma) * 100 // 百分比
	}

	currentPrice := closePrices[len(closePrices)-1]
	percentB := 0.5 // 默認中間值
	if upper != lower {
		percentB = (currentPrice - lower) / (upper - lower)
	}

	// 6. 帶寬收縮/排名檢測 (近 lookback 週期)
	squeeze := false
	bwRank := 50 // 默認中間值

	lookback := 20
	// 需要至少 period + lookback - 1 根 K 線才能計算 lookback 個帶寬
	minKlinesNeeded := period + lookback - 1
	if len(klines) >= minKlinesNeeded {
		// ⚡ 優化：只計算需要的最近 lookback 個帶寬值
		var bandwidthHistory []float64

		// 從倒數第 lookback 個週期開始計算到當前
		// 每個週期需要 period 根 K 線，所以起始索引是 len(klines) - lookback
		for offset := lookback - 1; offset >= 0; offset-- {
			endIdx := len(klines) - offset
			startIdx := endIdx - period
			if startIdx < 0 {
				continue // 不足 period 根 K 線，跳過
			}

			subPrices := closePrices[startIdx:endIdx]

			subSum := 0.0
			for _, p := range subPrices {
				subSum += p
			}
			subSma := subSum / float64(period)

			subVar := 0.0
			for _, p := range subPrices {
				diff := p - subSma
				subVar += diff * diff
			}
			subVar /= float64(period)
			subStd := math.Sqrt(subVar)

			subUpper := subSma + (stdDev * subStd)
			subLower := subSma - (stdDev * subStd)
			if subSma > 0 {
				bw := ((subUpper - subLower) / subSma) * 100
				bandwidthHistory = append(bandwidthHistory, bw)
			}
		}

		// 現在 bandwidthHistory[len-1] 應該是當前帶寬
		if len(bandwidthHistory) >= 2 {
			// 找最小帶寬
			minBW := bandwidthHistory[0]
			for _, bw := range bandwidthHistory {
				if bw < minBW {
					minBW = bw
				}
			}

			// 如果當前帶寬是近期最低 (5% 容許誤差)，則為 Squeeze
			if bandwidth <= minBW*1.05 {
				squeeze = true
			}

			// 計算 BWRank: 當前帶寬在近期中的百分位
			lowerCount := 0
			for _, bw := range bandwidthHistory {
				if bw < bandwidth {
					lowerCount++
				}
			}
			bwRank = (lowerCount * 100) / len(bandwidthHistory)
		}
	}

	// 7. 判斷 Regime
	regime := "MeanReversion"
	if squeeze {
		regime = "Squeeze"
	} else if percentB > 0.8 || percentB < 0.2 {
		regime = "Trend"
	}

	return &BollingerBands{
		Upper:     upper,
		Middle:    sma,
		Lower:     lower,
		Bandwidth: bandwidth,
		PercentB:  percentB,
		Squeeze:   squeeze,
		BWRank:    bwRank,
		Regime:    regime,
	}
}

// Helper for standalone RSI calculation (since we need it for Macro)
func calculateRSISeries(klines []Kline, period int) []float64 {
	if len(klines) < period+1 {
		return nil
	}
	rsis := make([]float64, len(klines))

	avgGain := 0.0
	avgLoss := 0.0

	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain += change
		} else {
			avgLoss -= change
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	if avgLoss == 0 {
		rsis[period] = 100.0
	} else {
		rsis[period] = 100.0 - (100.0 / (1.0 + (avgGain / avgLoss)))
	}

	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		gain := 0.0
		loss := 0.0
		if change > 0 {
			gain = change
		} else {
			loss = -change
		}

		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)

		if avgLoss == 0 {
			rsis[i] = 100.0
		} else {
			rs := avgGain / avgLoss
			rsis[i] = 100.0 - (100.0 / (1.0 + rs))
		}
	}
	return rsis
}

// calculatePriceAction 计算价格行为特征（增强版：支持双K线形态）
func calculatePriceAction(klines []Kline, currentEMA float64) *PriceActionData {
	if len(klines) == 0 {
		return nil
	}

	latest := klines[len(klines)-1]
	open := latest.Open
	close := latest.Close
	high := latest.High
	low := latest.Low
	isBullish := close > open

	// 基础数据
	bodySize := math.Abs(open - close)
	totalRange := high - low

	// 避免除以零
	if totalRange == 0 {
		return &PriceActionData{
			CandleType: "Doji", // 极小波动视为十字星
		}
	}

	upperWick := high - math.Max(open, close)
	lowerWick := math.Min(open, close) - low

	// 计算比率
	upperWickRatio := upperWick / totalRange
	lowerWickRatio := lowerWick / totalRange
	bodyRatio := bodySize / totalRange

	// 计算EMA乖离率
	distToEMA := 0.0
	if currentEMA > 0 {
		distToEMA = ((close - currentEMA) / currentEMA) * 100
	}

	// ========== 形态识别（优先级从高到低）==========
	candleType := identifyCandlePattern(klines, bodyRatio, upperWickRatio, lowerWickRatio, upperWick, lowerWick, bodySize, isBullish)

	return &PriceActionData{
		UpperWickRatio: upperWickRatio,
		LowerWickRatio: lowerWickRatio,
		BodyRatio:      bodyRatio,
		DistToEMA20:    distToEMA,
		CandleType:     candleType,
	}
}

// identifyCandlePattern 识别K线形态（支持单K线和双K线形态）
func identifyCandlePattern(klines []Kline, bodyRatio, upperWickRatio, lowerWickRatio, upperWick, lowerWick, bodySize float64, isBullish bool) string {
	// ========== 1. 双K线形态（优先级最高）==========
	if len(klines) >= 2 {
		prev := klines[len(klines)-2]
		curr := klines[len(klines)-1]

		prevBodySize := math.Abs(prev.Open - prev.Close)
		currBodySize := math.Abs(curr.Open - curr.Close)
		prevIsBullish := prev.Close > prev.Open
		currIsBullish := curr.Close > curr.Open

		// Bullish Engulfing（看涨吞噬）：前阴后阳，后K线实体完全覆盖前K线实体
		if !prevIsBullish && currIsBullish &&
			curr.Open <= prev.Close && curr.Close >= prev.Open &&
			currBodySize > prevBodySize*1.1 { // 当前实体至少比前一根大10%
			return "Bullish Engulfing"
		}

		// Bearish Engulfing（看跌吞噬）：前阳后阴，后K线实体完全覆盖前K线实体
		if prevIsBullish && !currIsBullish &&
			curr.Open >= prev.Close && curr.Close <= prev.Open &&
			currBodySize > prevBodySize*1.1 {
			return "Bearish Engulfing"
		}

		// Bullish Harami（看涨孕线）：前阴后阳，后K线实体完全在前K线实体内
		if !prevIsBullish && currIsBullish &&
			curr.Open > prev.Close && curr.Close < prev.Open &&
			currBodySize < prevBodySize*0.5 { // 当前实体小于前一根的50%
			return "Bullish Harami"
		}

		// Bearish Harami（看跌孕线）：前阳后阴，后K线实体完全在前K线实体内
		if prevIsBullish && !currIsBullish &&
			curr.Open < prev.Close && curr.Close > prev.Open &&
			currBodySize < prevBodySize*0.5 {
			return "Bearish Harami"
		}

		// Tweezer Bottom（镊子底）：连续两根K线有相近的最低点
		lowDiff := math.Abs(prev.Low-curr.Low) / curr.Low
		if lowDiff < 0.002 && !prevIsBullish && currIsBullish { // 最低点差异小于0.2%
			return "Bullish Tweezer Bottom"
		}

		// Tweezer Top（镊子顶）：连续两根K线有相近的最高点
		highDiff := math.Abs(prev.High-curr.High) / curr.High
		if highDiff < 0.002 && prevIsBullish && !currIsBullish { // 最高点差异小于0.2%
			return "Bearish Tweezer Top"
		}
	}

	// ========== 2. Pinbar（优先级次高，因为信号强）==========
	// 看跌Pinbar/Shooting Star（长上影线）
	if upperWickRatio > 0.6 && upperWick > bodySize*2 {
		if isBullish {
			return "Bearish Pinbar (Shooting Star)" // 上涨趋势中更有效看跌
		}
		// 🔧 修正（IND-01）：下跌趋势中长上影线是倒锤头（Inverted Hammer），属于看涨探底反转信号
		return "Bullish Inverted Hammer"
	}

	// 看涨Pinbar/Hammer（长下影线）
	if lowerWickRatio > 0.6 && lowerWick > bodySize*2 {
		if !isBullish {
			return "Bullish Pinbar (Hammer)" // 下跌趋势中看涨见底
		}
		// 🔧 修正（IND-01）：上涨趋势中出现长下影高位吊线（Hanging Man），属于高位筹码松动的看跌见顶警告
		return "Bearish Hanging Man"
	}

	// ========== 3. 十字星类形态 ==========
	if bodyRatio < 0.1 {
		// 蜻蜓十字星（Dragonfly Doji）：几乎无上影线，长下影线
		if upperWickRatio < 0.1 && lowerWickRatio > 0.6 {
			return "Bullish Dragonfly Doji"
		}
		// 墓碑十字星（Gravestone Doji）：几乎无下影线，长上影线
		if lowerWickRatio < 0.1 && upperWickRatio > 0.6 {
			return "Bearish Gravestone Doji"
		}
		// 长脚十字星（Long-legged Doji）：上下影线都长
		if upperWickRatio > 0.3 && lowerWickRatio > 0.3 {
			return "Long-legged Doji"
		}
		// 普通十字星
		return "Doji"
	}

	// ========== 4. 纺锤形态（Spinning Top）==========
	if bodyRatio >= 0.1 && bodyRatio < 0.3 && upperWickRatio > 0.25 && lowerWickRatio > 0.25 {
		if isBullish {
			return "Bullish Spinning Top"
		}
		return "Bearish Spinning Top"
	}

	// ========== 5. 光头光脚（Marubozu）==========
	if bodyRatio > 0.8 {
		if isBullish {
			return "Bullish Marubozu" // 大阳线
		}
		return "Bearish Marubozu" // 大阴线
	}

	// ========== 6. 普通K线 ==========
	if isBullish {
		if bodyRatio > 0.5 {
			return "Bullish" // 阳线
		}
		return "Small Bullish" // 小阳线
	} else {
		if bodyRatio > 0.5 {
			return "Bearish" // 阴线
		}
		return "Small Bearish" // 小阴线
	}
}

// calculateTechnicalAnalysis 计算技术分析汇总 (Logic Upgrade: Dual-Track Scoring + Phase 20 OI + Phase 21 Liquidity + Phase 3 VPVR)
func calculateTechnicalAnalysis(price, ema, macd, rsi, vol, avgVol float64, pa *PriceActionData, div string, macdHist []float64, vwap float64, cvd *CVDData, oi *OIData, liquidity []LiquidityCluster, vpvr *VolumeProfileResponse) *TechnicalAnalysis {
	// 1. 趋势状态 (Trend State)
	trend := "Neutral"
	if price > ema && macd > 0 {
		trend = "Bullish"
	} else if price < ema && macd < 0 {
		trend = "Bearish"
	}

	// 2. RSI状态 (RSI State)
	rsiState := "Neutral"
	if rsi > 70 {
		rsiState = "Overbought"
	} else if rsi < 30 {
		rsiState = "Oversold"
	}

	// 3. 成交量状态 (Volume State)
	volState := "Normal"
	if avgVol > 0 {
		ratio := vol / avgVol
		if ratio > 1.5 {
			volState = "High"
		} else if ratio < 0.5 {
			volState = "Low"
		}
	}

	// 4. 双轨评分计算 (Dual-Track Scoring)
	// 分别计算多头力度 (BullScore) 和 空头力度 (BearScore)
	// 最终分数为两者的最大值，代表"市场当下的信号强度/确信度"，无论方向。
	// 这解决了 "Short Paradox" (熊市导致低分) 和 "Cancellation Effect" (趋势与背离互斥)。
	// Update Phase 32: Lowered Base from 50 to 35 to prevent "Inflation".
	// Requirement: Trend (25) + VWAP (10) + Base (35) = 70 (Barely Executable).
	bullScore := 35
	bearScore := 35

	// --- VWAP Correction (Institutional Mean) ---
	if vwap > 0 {
		if price > vwap {
			bullScore += 10 // Above Intraday Value (Increased importance)
		} else {
			bearScore += 10 // Below Intraday Value
		}
	}

	// --- 趋势得分 (Trend Score) ---
	switch trend {
	case "Bullish":
		bullScore += 25 // 强多头趋势 -> 支持做多
	case "Bearish":
		bearScore += 25 // 强空头趋势 -> 支持做空
	}

	// --- RSI 修正 (Reversion Logic) ---
	// "Elite Sniper" 喜欢在极端位置反向操作 (Mean Reversion)
	switch rsiState {
	case "Oversold": // 超卖 (<30)
		bullScore += 15 // 强烈看涨反转信号
		bearScore -= 5  // 追空风险大
	case "Overbought": // 超买 (>70)
		bearScore += 15 // 强烈看跌反转信号
		bullScore -= 5  // 追多风险大
	}

	// --- RSI 背离 (Divergence: The Sniper Trigger) ---
	switch div {
	case "Bullish":
		bullScore += 25 // 强力底部背离 (Reversal)
	case "Hidden Bullish":
		bullScore += 15 // 强力趋势延续 (Continuation) - "True RSI" Strategy
	case "Bearish":
		bearScore += 25 // 强力顶部背离 (Reversal)
	case "Hidden Bearish":
		bearScore += 15 // 强力趋势延续 (Continuation) - "True RSI" Strategy
	}

	// --- 成交量确认 (Volume Confirmation) ---
	if volState == "High" {
		// 放量通常确认当下的主要逻辑
		// 如果有背离，放量确认反转；如果有趋势，放量确认延续。
		// 简化逻辑：加成给得分较高的一方 (Momentum Fuel)
		if bullScore > bearScore {
			bullScore += 10
		} else {
			bearScore += 10
		}
	}

	// --- Price Action 修正 ---
	if pa != nil {
		candleType := pa.CandleType
		if strings.Contains(candleType, "Bullish") {
			// 如果是强反转形态 (Pinbar, Engulfing, Inverted Hammer)
			if strings.Contains(candleType, "Engulfing") || strings.Contains(candleType, "Pinbar") || strings.Contains(candleType, "Hammer") {
				bullScore += 15
			} else {
				bullScore += 5
			}
		} else if strings.Contains(candleType, "Bearish") {
			// 如果是强反转形态 (Pinbar, Engulfing, Hanging Man)
			if strings.Contains(candleType, "Engulfing") || strings.Contains(candleType, "Pinbar") || strings.Contains(candleType, "Hanging Man") {
				bearScore += 15
			} else {
				bearScore += 5
			}
		}
	}

	// --- 动能反转 (Momentum Reversal) ---
	macdHistState := "Neutral"
	momentumReversal := "None"

	if len(macdHist) >= 3 {
		currHist := macdHist[len(macdHist)-1]
		prevHist := macdHist[len(macdHist)-2]

		if math.Abs(currHist) > math.Abs(prevHist) {
			macdHistState = "Expansion"
		} else {
			macdHistState = "Contraction"
		}

		// Bullish Reversal: Hist Negative but rising
		if currHist < 0 && currHist > prevHist {
			momentumReversal = "Bullish Reversal"
			bullScore += 20 // 动能转强
		}

		// Bearish Reversal: Hist Positive but falling
		if currHist > 0 && currHist < prevHist {
			momentumReversal = "Bearish Reversal"
			bearScore += 20 // 动能转弱
		}
	}

	// 5. 最终决策 (Final Decision)
	// 取两者最大值作为 Signal Score
	finalScore := 0
	if bullScore > bearScore {
		finalScore = bullScore
	} else {
		finalScore = bearScore
	}

	// 限制分数范围 0-100
	if finalScore > 100 {
		finalScore = 100
	} else if finalScore < 0 {
		finalScore = 0
	}

	// --- Phase 20: OI Scoring ---
	if oi != nil {
		// Identify Rising OI > 0.5% ??? No, just Direction for now.
		// If OI Rising (+Delta), it confirms the current Price Trend.
		// If OI Falling (-Delta), it suggests liqudations or lack of interest.

		if oi.Delta15m > 0 {
			// Confirmation: Price and OI moving together
			switch trend {
			case "Bullish":
				bullScore += 5 // Strong Bullish Interest
			case "Bearish":
				bearScore += 5 // Strong Bearish Interest (Shorting)
			}
		} else if oi.Delta15m < 0 {
			// Liquidation or Exhaustion?
			// If Price Up + OI Down = Short Covering (Not organic buying) -> Bearish Reversal sign?
			// For now, keep it simple: -5 points for trend if OI is dropping (Less conviction)
			switch trend {
			case "Bullish":
				bullScore = int(math.Max(0, float64(bullScore-5)))
			case "Bearish":
				bearScore = int(math.Max(0, float64(bearScore-5)))
			}
		}
	}

	// 最终双轨归一 (Max Strategy)
	finalScore = 0
	// ... (Rest of logic)

	// Note: We need to inject oiState into return struct.
	// But struct only has OIDelta field.

	// Recalculate max
	if bullScore > bearScore {
		finalScore = bullScore
	} else {
		finalScore = bearScore
	}
	if finalScore > 100 {
		finalScore = 100
	}

	return &TechnicalAnalysis{
		TrendState:         trend,
		RSIState:           rsiState,
		VolumeState:        volState,
		Divergence:         div,
		MACDHistogramState: macdHistState,
		MomentumReversal:   momentumReversal,
		SignalScore:        finalScore, // 输出最高分 (Intensity)
		VWAP:               vwap,
		OrderFlow:          cvd,
		OIDelta:            oi,        // Corrected: Use passed *OIData struct
		Liquidity:          liquidity, // Phase 21
		VPVR:               vpvr,
	}
}

// calculateRSIDivergence 计算 RSI 背离
// 逻辑：比较最近 20 根 K 线内的价格和 RSI 极值点
func calculateRSIDivergence(klines []Kline, rsiValues []float64) string {
	// 需要至少 20 个点的数据
	if len(klines) < 20 || len(rsiValues) < 20 {
		return "None"
	}

	// 获取最近 20 个数据点
	// startIdx := len(klines) - 20 (unused)
	// 确保 RSI 数据和 K 线数据对齐（rsiValues 通常是从 klines计算来的，长度可能不同，需对齐）
	// rsiValues 长度可能小于 klines，因为前 14 个点没有 RSI
	// 假设传入的 rsiValues 已经是对应 klines 后半部分的
	// 简单起见，我们假设传入的 rsiValues 和 klines 是一一对应的切片 (在调用处处理截断)

	// 查找最近的两个低点 (Pivot Lows)
	// Pivot Low 定义：low[i] < low[i-1] && low[i] < low[i+1]
	// 我们只在 [startIdx, current-2] 范围内查找（当前点还未确认 Pivot）

	var lowPoints []int
	var highPoints []int

	// Pivot 判断：左右各 1 根 K 线
	// 修正：从倒数第3根 K 线 (len-3) 开始向过去查找 (len-18)
	// 原因：len-1 是 Open Candle (变动中)，len-2 是最近 Closed Candle。
	// 为了确认 len-2 是 Pivot，需要 len-2 < len-1 和 len-2 < len-3。
	// 但 len-1 是变动的，如果 len-1 价格下跌，len-2 的 Pivot 属性可能会消失 (Repainting)。
	// 为了完全消除 Repainting 风险，我们只能确认 len-3 (它的左右 len-2 和 len-4 都是 Closed 的)。
	// 虽然这引入了约 1 小时的滞后，但保证了信号的绝对真实性 (No Repaint)。
	for i := len(klines) - 3; i >= len(klines)-18; i-- { // 优先找最近的 Confirmed Pivot
		if isPivotLow(klines, i) {
			lowPoints = append(lowPoints, i)
		}
		if isPivotHigh(klines, i) {
			highPoints = append(highPoints, i)
		}
	}

	// 查找更早的一个 Pivot
	// 如果由于数据长度限制找不到，则无法计算背离
	// 简化逻辑：仅比较最近的 Pivot 和当前价格状态（如果当前正在创新低）
	// 或者比较最近的两个 Pivot

	// ----------------------------------------------------------------
	// 简化版背离算法：比较最近两个 Pivot (如果都在 20 周期内)
	// ----------------------------------------------------------------

	// 1. 底背离检测 (Bullish Divergence)
	// 价格创新低 (RecentLow < PreviousLow)，但 RSI 抬高 (RecentRSI > PreviousRSI)
	if len(lowPoints) >= 2 {
		currIdx := lowPoints[0] // 最近的低点
		prevIdx := lowPoints[1] // 前一个低点

		// 🛡️ 新增：时效性过滤 (Recency Filter)
		// 之前是 8，现在放宽到 15，以捕捉更宏观的背离结构 (Addressing "Indicator Handicap")
		if len(klines)-1-currIdx > 15 {
			// 信号太老，不予采纳
		} else if prevIdx < len(rsiValues) && currIdx < len(rsiValues) {
			priceLL := klines[currIdx].Low < klines[prevIdx].Low // 价格更低
			rsiHL := rsiValues[currIdx] > rsiValues[prevIdx]     // RSI 更高
			rsiOversold := rsiValues[currIdx] < 35               // RSI 处于低位 (更严格：< 35)

			if priceLL && rsiHL && rsiOversold {
				return "Bullish" // Regular Bullish (Reversal)
			}

			// 1.1 隐藏底背离 (Hidden Bullish) - 趋势延续
			// 价格抬高 (Higher Low) + RSI 降低 (Lower Low)
			priceHL := klines[currIdx].Low > klines[prevIdx].Low
			rsiLL := rsiValues[currIdx] < rsiValues[prevIdx]

			if priceHL && rsiLL {
				return "Hidden Bullish" // Trend Continuation
			}

		}
	}

	// 2. 顶背离检测 (Bearish Divergence)
	// 价格创新高 (RecentHigh > PreviousHigh)，但 RSI 走低 (RecentRSI < PreviousRSI)
	if len(highPoints) >= 2 {
		currIdx := highPoints[0]
		prevIdx := highPoints[1]

		// 🛡️ 新增：时效性过滤 (Relaxed to 15)
		if len(klines)-1-currIdx > 15 {
			// 过期信号
		} else if prevIdx < len(rsiValues) && currIdx < len(rsiValues) {
			priceHH := klines[currIdx].High > klines[prevIdx].High // 价格更高
			rsiLH := rsiValues[currIdx] < rsiValues[prevIdx]       // RSI 更低
			rsiOverbought := rsiValues[currIdx] > 65               // RSI 处于高位 (更严格：> 65)

			if priceHH && rsiLH && rsiOverbought {
				return "Bearish" // Regular Bearish (Reversal)
			}

			// 2.1 隐藏顶背离 (Hidden Bearish) - 趋势延续
			// 价格降低 (Lower High) + RSI 抬高 (Higher High)
			priceLH := klines[currIdx].High < klines[prevIdx].High
			rsiHH := rsiValues[currIdx] > rsiValues[prevIdx]

			if priceLH && rsiHH {
				return "Hidden Bearish" // Trend Continuation
			}
		}
	}

	return "None"
}

// 辅助：判断是否为 Pivot Low (左右各1根更高)
func isPivotLow(klines []Kline, i int) bool {
	if i <= 0 || i >= len(klines)-1 {
		return false
	}
	return klines[i].Low < klines[i-1].Low && klines[i].Low < klines[i+1].Low
}

// 辅助：判断是否为 Pivot High
func isPivotHigh(klines []Kline, i int) bool {
	if i <= 0 || i >= len(klines)-1 {
		return false
	}
	return klines[i].High > klines[i-1].High && klines[i].High > klines[i+1].High
}

// symbolToCoinGeckoID 将交易对符号转换为CoinGecko ID
func symbolToCoinGeckoID(symbol string) string {
	// 移除USDT后缀
	baseAsset := strings.ToLower(strings.TrimSuffix(symbol, "USDT"))
	if baseAsset == "" {
		baseAsset = strings.ToLower(symbol)
	}

	// CoinGecko ID映射表（常见币种）
	coinGeckoIDMap := map[string]string{
		"btc":    "bitcoin",
		"eth":    "ethereum",
		"sol":    "solana",
		"bnb":    "binancecoin",
		"xrp":    "ripple",
		"doge":   "dogecoin",
		"ada":    "cardano",
		"ltc":    "litecoin",
		"zec":    "zcash",
		"zen":    "zencash",
		"avax":   "avalanche-2",
		"arb":    "arbitrum",
		"mina":   "mina-protocol",
		"zk":     "zksync",
		"dot":    "polkadot",
		"ton":    "the-open-network",
		"kas":    "kaspa",
		"fil":    "filecoin",
		"link":   "chainlink",
		"dash":   "dash",
		"icp":    "internet-computer",
		"aave":   "aave",
		"render": "render-token",
		"pol":    "polygon",
		"xplus":  "xplus",
		"polyx":  "polymath",
		"0g":     "0g",
		"stbl":   "stablecoin",
		"ai16z":  "ai16z",
		"prompt": "prompt",
		"pump":   "pump-fun",
		"hype":   "hyperliquid",
		"aster":  "aster",
		"avny":   "avny",
		"aia":    "aia",
		"trust":  "trust",
		"stable": "stablecoin",
	}

	if id, ok := coinGeckoIDMap[baseAsset]; ok {
		return id
	}

	// 如果找不到，尝试直接使用baseAsset（CoinGecko可能支持）
	return baseAsset
}

// GetCoinGeckoDataBatch 批量获取CoinGecko数据（优化：减少API调用次数）
// 使用 coins/markets 端点获取更丰富的数据（包括排名和市值）
func GetCoinGeckoDataBatch(symbols []string) map[string]*CoinGeckoData {
	result := make(map[string]*CoinGeckoData)
	if len(symbols) == 0 {
		return result
	}

	// 转换为CoinGecko ID并建立映射
	coinIDMap := make(map[string]string) // coinID -> symbol
	coinIDs := make([]string, 0, len(symbols))

	for _, symbol := range symbols {
		coinID := symbolToCoinGeckoID(symbol)
		if coinID != "" {
			coinIDMap[coinID] = symbol
			coinIDs = append(coinIDs, coinID)
		}
	}

	if len(coinIDs) == 0 {
		return result
	}

	// CoinGecko API: coins/markets
	// 这个接口一次可以查询多个币种，并且直接返回 rank, current_price, market_cap, total_volume
	// 注意：URL长度限制，如果币种太多可能需要分批，但在我们的候选数量(20-30)下应该是安全的
	idsParam := strings.Join(coinIDs, ",")
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=%s&order=market_cap_desc&per_page=250&page=1&sparkline=false", idsParam)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("⚠️ CoinGecko API调用失败: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ CoinGecko API返回非200状态: %d", resp.StatusCode)
		return result
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("⚠️ 读取CoinGecko响应失败: %v", err)
		return result
	}

	// 解析响应数组
	var markets []struct {
		ID            string  `json:"id"`
		Symbol        string  `json:"symbol"`
		CurrentPrice  float64 `json:"current_price"`
		MarketCap     float64 `json:"market_cap"` // ✅ 新增
		MarketCapRank int     `json:"market_cap_rank"`
		TotalVolume   float64 `json:"total_volume"`
		PriceChange24 float64 `json:"price_change_percentage_24h"`
	}

	if err := json.Unmarshal(body, &markets); err != nil {
		return result
	} // End of json.Unmarshal check

	// 映射回我们的结构体
	count := 0
	for _, m := range markets {
		originalSymbol, ok := coinIDMap[m.ID]
		if !ok {
			continue
		}

		data := &CoinGeckoData{
			// Price field does not exist in struct, skip it.
			TotalVolume24h: m.TotalVolume,
			PriceChange24h: m.PriceChange24,
			MarketCapRank:  m.MarketCapRank,
			MarketCap:      m.MarketCap, // ✅ 赋值
		}

		// 计算价格变化USD (估算)
		if m.PriceChange24 != 0 {
			data.PriceChange24hUSD = m.CurrentPrice * (m.PriceChange24 / 100.0)
		}

		result[originalSymbol] = data
		count++
	}

	if count > 0 {
		log.Printf("✓ 批量获取CoinGecko数据成功: %d/%d 个币种 (包含市值排名)", count, len(symbols))
	} else {
		log.Printf("⚠️ CoinGecko未返回匹配数据 (ID映射可能需要更新)")
	}

	return result
}

// GetCryptoFearGreedIndex fetches the Fear & Greed Index from alternative.me
func GetCryptoFearGreedIndex() (*FearGreedData, error) {
	url := "https://api.alternative.me/fng/?limit=1"

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch F&G failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Name string `json:"name"`
		Data []struct {
			Value               string `json:"value"`
			ValueClassification string `json:"value_classification"`
			Timestamp           string `json:"timestamp"`
		} `json:"data"`
	}

	body, _ := ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse F&G failed: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no F&G data returned")
	}

	item := result.Data[0]
	val, _ := strconv.Atoi(item.Value)
	ts, _ := strconv.ParseInt(item.Timestamp, 10, 64)

	return &FearGreedData{
		Value:          val,
		Classification: item.ValueClassification,
		Timestamp:      ts,
	}, nil
}

// calculateVWAP computes the Intraday VWAP (Volume Weighted Average Price)
// Resets at 00:00 UTC.
func calculateVWAP(klines []Kline) float64 {
	if len(klines) == 0 {
		return 0
	}

	var cumPV float64  // Cumulative (Price * Volume)
	var cumVol float64 // Cumulative Volume

	// Iterate backwards to find the start of the current UTC day
	// Then iterate forward to calculate VWAP

	// simplified: find the index where the day changes to today (UTC)
	lastK := klines[len(klines)-1]
	lastTime := time.Unix(lastK.OpenTime/1000, 0).UTC()
	startOfDay := time.Date(lastTime.Year(), lastTime.Month(), lastTime.Day(), 0, 0, 0, 0, time.UTC)
	startTs := startOfDay.Unix() * 1000

	startIndex := 0
	for i := len(klines) - 1; i >= 0; i-- {
		if klines[i].OpenTime < startTs {
			startIndex = i + 1
			break
		}
	}

	// Calculate specific VWAP for the current day
	for i := startIndex; i < len(klines); i++ {
		k := klines[i]
		typicalPrice := (k.High + k.Low + k.Close) / 3.0
		cumPV += typicalPrice * k.Volume
		cumVol += k.Volume
	}

	if cumVol == 0 {
		return 0
	}

	return cumPV / cumVol
}

// Format 格式化输出市场数据
func Format(data *Data) string {
	var sb strings.Builder

	// Adaptive Precision Logic
	pFmt := "%.2f"
	if data.CurrentPrice < 1.0 {
		pFmt = "%.6f"
	}

	var atr1h, atr1hPct float64
	var atr4h, atr4hPct float64

	if data.HourlyContext != nil {
		atr1h = data.HourlyContext.ATR14
		if data.CurrentPrice > 0 {
			atr1hPct = (atr1h / data.CurrentPrice) * 100
		}
	}
	if data.LongerTermContext != nil {
		atr4h = data.LongerTermContext.ATR14
		if data.CurrentPrice > 0 {
			atr4hPct = (atr4h / data.CurrentPrice) * 100
		}
	}

	sb.WriteString(fmt.Sprintf("current_price = "+pFmt+", current_ema20 = "+pFmt+", current_macd = %.4f, current_macd_signal = %.4f, current_macd_hist = %.4f, current_rsi (7 period) = %.2f\n",
		data.CurrentPrice, data.CurrentEMA20, data.CurrentMACD, data.CurrentMACDSignal, data.CurrentMACDHist, data.CurrentRSI7))
	sb.WriteString(fmt.Sprintf("15m_ema20 = "+pFmt+", 15m_macd = %.4f, 15m_macd_signal = %.4f, 15m_macd_hist = %.4f, 15m_rsi (14 period) = %.2f\n",
		data.EMA15m, data.MACD15m, data.MACDSignal15m, data.MACDHist15m, data.RSI15m))

	// 🔥 Expose ATR (Volatility) for Dynamic Risk Management
	sb.WriteString(fmt.Sprintf("1H_ATR14 = %.4f (%.2f%%), 4H_ATR14 = %.4f (%.2f%%)\n",
		atr1h, atr1hPct, atr4h, atr4hPct))

	// VWAP
	if data.TechnicalAnalysis != nil {
		ta := data.TechnicalAnalysis
		sb.WriteString(fmt.Sprintf("\nIntraday VWAP: %.4f\n", ta.VWAP))

		// Phase 3: VPVR Display
		if ta.VPVR != nil {
			pocDist := ((ta.VPVR.POC - data.CurrentPrice) / data.CurrentPrice) * 100
			sb.WriteString(fmt.Sprintf("VPVR (Volume Profile): POC=%.2f (Dist: %.2f%%) | VA High=%.2f | VA Low=%.2f\n",
				ta.VPVR.POC, pocDist, ta.VPVR.VAHigh, ta.VPVR.VALow))
		}
	} else {
		sb.WriteString("\n")
	}

	// CoinGecko 数据（全球视角）
	if data.CoinGeckoData != nil {
		sb.WriteString(fmt.Sprintf("Global Market Data (CoinGecko): Rank #%d | Global Vol: %.2fM | 24h Change: %+.2f%%\n",
			data.CoinGeckoData.MarketCapRank, data.CoinGeckoData.TotalVolume24h/1000000, data.CoinGeckoData.PriceChange24h))
	} else {
		sb.WriteString("Global Market Data: N/A (Using Binance local data only)\n")
	}

	// Fear & Greed Index Visualization (New)
	if data.Sentiment != nil {
		sb.WriteString(fmt.Sprintf("Market Sentiment: %s (%d)\n", data.Sentiment.Classification, data.Sentiment.Value))
	} else {
		sb.WriteString("Market Sentiment: N/A\n")
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
		data.Symbol))

	// Open Interest (Existing + Delta Upgrade)
	if data.OpenInterest != nil && data.OpenInterest.Latest > 0 {
		sb.WriteString(fmt.Sprintf("Open Interest: %.2f (Avg: %.2f)\n",
			data.OpenInterest.Latest, data.OpenInterest.Average))
		// Phase 20: Delta
		if data.TechnicalAnalysis != nil && data.TechnicalAnalysis.OIDelta != nil {
			sb.WriteString(fmt.Sprintf("OI Delta (15m): %.2f (1H: %.2f)\n",
				data.OpenInterest.Delta15m, data.TechnicalAnalysis.OIDelta.Delta1H))
		}
		sb.WriteString("\n")
	}

	fundingRatePct := data.FundingRate * 100
	rateNote := ""
	if math.Abs(fundingRatePct) >= 0.1 {
		rateNote = " ⚠️ 资金费率偏高，注意结算成本与挤压风险"
	}
	sb.WriteString(fmt.Sprintf("Funding Rate: %+.4f%% (8h: %.2e)%s\n\n", fundingRatePct, data.FundingRate, rateNote))

	// BuySellRatio（买卖比例）- 明确标记，便于AI识别
	if data.BuySellRatio > 0 {
		sb.WriteString(fmt.Sprintf("BuySellRatio: %.3f (%.1f%% 主动买入量 / 总成交量",
			data.BuySellRatio, data.BuySellRatio*100))
		if data.BuySellRatio > 0.7 {
			sb.WriteString(", 强买信号")
		} else if data.BuySellRatio > 0.55 {
			sb.WriteString(", 买方较强")
		} else if data.BuySellRatio < 0.3 {
			sb.WriteString(", 强卖信号")
		} else if data.BuySellRatio < 0.45 {
			sb.WriteString(", 卖方较强")
		}
		sb.WriteString(")\n\n")
	}

	// 🧠 Order Flow (CVD) Visualization
	if data.TechnicalAnalysis != nil && data.TechnicalAnalysis.OrderFlow != nil {
		sb.WriteString("🌊 Order Flow Analysis (CVD):\n")
		cvd := data.TechnicalAnalysis.OrderFlow
		sb.WriteString(fmt.Sprintf("  Divergence: %s\n", cvd.Divergence))
		sb.WriteString(fmt.Sprintf("  Absorption State: %s\n", cvd.AbsorptionState))
		sb.WriteString(fmt.Sprintf("  Current CVD Delta Sum: %.2f\n\n", cvd.CurrentCVD))
	}

	// Helper for dynamic price formatting
	formatPrice := func(p float64) string {
		if p < 1.0 {
			return fmt.Sprintf("%.5f", p)
		} else if p < 10.0 {
			return fmt.Sprintf("%.4f", p)
		} else if p < 100.0 {
			return fmt.Sprintf("%.3f", p)
		}
		return fmt.Sprintf("%.2f", p)
	}

	// Phase 21: Liquidity Heatmap Display
	if data.TechnicalAnalysis != nil && len(data.TechnicalAnalysis.Liquidity) > 0 {
		sb.WriteString("🔥 Liquidity Heatmap (Est. Top Unbroken Levels):\n")

		clusters := data.TechnicalAnalysis.Liquidity
		count := 0
		lastStr := ""
		for i := len(clusters) - 1; i >= 0; i-- {
			c := clusters[i]
			dist := ((c.Price - data.CurrentPrice) / data.CurrentPrice) * 100
			if math.Abs(dist) < 20.0 {
				str := fmt.Sprintf("  - %s @ %s (Dist: %.2f%%)\n", c.Type, formatPrice(c.Price), dist)
				// Deduplication
				if str == lastStr {
					continue
				}
				sb.WriteString(str)
				lastStr = str
				count++
				if count >= 5 { // Increased from 3 to 5 for better visibility
					break
				}
			}
		}
		sb.WriteString("\n")
	}

	// CoinGecko数据（免费API）
	if data.CoinGeckoData != nil {
		sb.WriteString("CoinGecko Market Data (免费API):\n\n")
		if data.CoinGeckoData.MarketCapRank > 0 {
			sb.WriteString(fmt.Sprintf("Market Cap Rank: #%d\n", data.CoinGeckoData.MarketCapRank))
		}
		if data.CoinGeckoData.PriceChange24h != 0 {
			sb.WriteString(fmt.Sprintf("24h Price Change: %+.2f%% (%+.2f USDT)\n",
				data.CoinGeckoData.PriceChange24h, data.CoinGeckoData.PriceChange24hUSD))
		}
		if data.CoinGeckoData.MarketCap > 0 {
			marketCapBillions := data.CoinGeckoData.MarketCap / 1_000_000_000
			sb.WriteString(fmt.Sprintf("Market Cap: %.2fB USDT\n", marketCapBillions))
		}
		if data.CoinGeckoData.TotalVolume24h > 0 {
			volumeMillions := data.CoinGeckoData.TotalVolume24h / 1_000_000
			sb.WriteString(fmt.Sprintf("24h Volume: %.2fM USDT\n", volumeMillions))
		}
		sb.WriteString("\n")
	}

	// Technical Analysis 数据（新增 - 逻辑固化）
	if data.TechnicalAnalysis != nil {
		sb.WriteString("Technical Analysis Summary (Logic Hardened):\n\n")
		sb.WriteString(fmt.Sprintf("Signal Score: %d (Threshold: 70+ for Trade, 60+ for Analysis Paralysis)\n", data.TechnicalAnalysis.SignalScore))
		// P31: 增加多时间框架原始数据的可见性
		macd15m := data.MACD15m
		hist15m := data.MACDHist15m

		macd1h := 0.0
		hist1h := 0.0
		if data.HourlyContext != nil {
			if len(data.HourlyContext.MACDValues) > 0 {
				macd1h = data.HourlyContext.MACDValues[len(data.HourlyContext.MACDValues)-1]
			}
			if len(data.HourlyContext.MACDHistValues) > 0 {
				hist1h = data.HourlyContext.MACDHistValues[len(data.HourlyContext.MACDHistValues)-1]
			}
		}

		macd4h := 0.0
		hist4h := 0.0
		if data.LongerTermContext != nil {
			if len(data.LongerTermContext.MACDValues) > 0 {
				macd4h = data.LongerTermContext.MACDValues[len(data.LongerTermContext.MACDValues)-1]
			}
			if len(data.LongerTermContext.MACDHistValues) > 0 {
				hist4h = data.LongerTermContext.MACDHistValues[len(data.LongerTermContext.MACDHistValues)-1]
			}
		}

		sb.WriteString(fmt.Sprintf("Multi-Timeframe Data:\n  15m: MACD=%.4f, Hist=%.4f\n  1h:  MACD=%.4f, Hist=%.4f\n  4h:  MACD=%.4f, Hist=%.4f\n",
			macd15m, hist15m, macd1h, hist1h, macd4h, hist4h))
		sb.WriteString(fmt.Sprintf("Trend State: %s (MACD/EMA)\n", data.TechnicalAnalysis.TrendState))
		sb.WriteString(fmt.Sprintf("RSI State: %s\n", data.TechnicalAnalysis.RSIState))
		sb.WriteString(fmt.Sprintf("Volume State: %s\n", data.TechnicalAnalysis.VolumeState))
		sb.WriteString(fmt.Sprintf("Divergence: %s (Automated Detection)\n", data.TechnicalAnalysis.Divergence))
		sb.WriteString(fmt.Sprintf("MACD Histogram State: %s\n", data.TechnicalAnalysis.MACDHistogramState))
		sb.WriteString(fmt.Sprintf("Momentum Reversal: %s\n\n", data.TechnicalAnalysis.MomentumReversal))

		// 🧠 SMC Analysis Visualization
		if data.TechnicalAnalysis.SMC != nil {
			sb.WriteString("📊 Market Structure (SMC Analysis):\n")
			smc := data.TechnicalAnalysis.SMC
			sb.WriteString(fmt.Sprintf("  Trend: %s\n", smc.Structure.Trend))
			if smc.Structure.BreakOfStructure != "None" {
				sb.WriteString(fmt.Sprintf("  ⚠️ Break of Structure (BOS): %s\n", smc.Structure.BreakOfStructure))
			}

			// Display last 3 FVGs
			if len(smc.FVGs) > 0 {
				sb.WriteString("  Fair Value Gaps (FVG):\n")
				count := 0
				lastStr := ""
				for i := len(smc.FVGs) - 1; i >= 0; i-- {
					fvg := smc.FVGs[i]
					str := fmt.Sprintf("    - %s FVG @ %s - %s\n", fvg.Type, formatPrice(fvg.Bottom), formatPrice(fvg.Top))
					if str == lastStr {
						continue // Skip duplicates
					}
					sb.WriteString(str)
					lastStr = str
					count++
					if count >= 3 {
						break
					}
				}
			}

			// Display last 3 OBs
			if len(smc.OrderBlocks) > 0 {
				sb.WriteString("  Order Blocks (Support/Resistance):\n")
				count := 0
				lastStr := ""
				for i := len(smc.OrderBlocks) - 1; i >= 0; i-- {
					ob := smc.OrderBlocks[i]
					str := fmt.Sprintf("    - %s OB @ %s - %s (%s)\n", ob.Type, formatPrice(ob.Bottom), formatPrice(ob.Top), ob.Strength)
					if str == lastStr {
						continue // Skip duplicates
					}
					sb.WriteString(str)
					lastStr = str
					count++
					if count >= 3 {
						break
					}
				}
			}
			sb.WriteString("\n")
		}

		// 🔥 Liquidity Heatmap Display (Pivot Proxy)
		if data.TechnicalAnalysis != nil && len(data.TechnicalAnalysis.Liquidity) > 0 {
			sb.WriteString("🔥 Liquidity Heatmap (Estimated Liquidations):\n")
			for _, pool := range data.TechnicalAnalysis.Liquidity {
				sb.WriteString(fmt.Sprintf("  - %s Pool @ %.2f (Vol: $%.0f)\n", pool.Type, pool.Price, pool.Volume))
			}
			sb.WriteString("\n")
		}
	}

	// Price Action 数据（新增）
	if data.PriceAction != nil {
		sb.WriteString("Price Action Analysis (Based on 1h Candle):\n\n")
		sb.WriteString(fmt.Sprintf("Candle Type: %s\n", data.PriceAction.CandleType))
		sb.WriteString(fmt.Sprintf("Upper Wick Ratio: %.3f (Pinbar > 0.6)\n", data.PriceAction.UpperWickRatio))
		sb.WriteString(fmt.Sprintf("Lower Wick Ratio: %.3f (Pinbar > 0.6)\n", data.PriceAction.LowerWickRatio))
		sb.WriteString(fmt.Sprintf("Body Ratio: %.3f (Doji < 0.2)\n", data.PriceAction.BodyRatio))
		sb.WriteString(fmt.Sprintf("Distance to EMA20: %.2f%%\n\n", data.PriceAction.DistToEMA20))
	}

	if data.IntradaySeries != nil {
		sb.WriteString("Intraday series (3‑minute intervals, oldest → latest):\n\n")

		if len(data.IntradaySeries.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
		}

		if len(data.IntradaySeries.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
		}

		if len(data.IntradaySeries.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
		}

		if len(data.IntradaySeries.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
		}

		if len(data.IntradaySeries.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
		}
	}

	// 🧠 Macro Context Authorization (Phase 5)
	if data.Macro != nil {
		sb.WriteString("🌍 Macro Context (Daily Timeframe):\n")
		sb.WriteString(fmt.Sprintf("  Trend State: %s\n", data.Macro.TrendState1D))
		sb.WriteString(fmt.Sprintf("  Structure: %s\n", data.Macro.StructState1D))
		if len(data.Macro.KeyLevels) >= 2 {
			sb.WriteString(fmt.Sprintf("  Key Levels: High=%.2f, Low=%.2f\n", data.Macro.KeyLevels[0], data.Macro.KeyLevels[1]))
		}
		sb.WriteString(fmt.Sprintf("  Daily RSI: %.2f\n", data.Macro.RSI1D))
		sb.WriteString(fmt.Sprintf("  Daily EMA20: %.2f\n", data.Macro.EMA20_1D))
		sb.WriteString(fmt.Sprintf("  Daily EMA50: %.2f\n\n", data.Macro.EMA50_1D))
	} else {
		sb.WriteString("🌍 Macro Context: Loading...\n\n")
	}

	// 1小时时间框架数据（新增）
	if data.HourlyContext != nil {
		sb.WriteString("Hourly context (1‑hour timeframe):\n\n")

		sb.WriteString(fmt.Sprintf("20‑Period EMA: %.3f vs. 50‑Period EMA: %.3f\n\n",
			data.HourlyContext.EMA20, data.HourlyContext.EMA50))

		sb.WriteString(fmt.Sprintf("3‑Period ATR: %.3f vs. 14‑Period ATR: %.3f\n\n",
			data.HourlyContext.ATR3, data.HourlyContext.ATR14))

		sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f (Volume Z-Score: %.2f)\n\n",
			data.HourlyContext.CurrentVolume, data.HourlyContext.AverageVolume, data.HourlyContext.VolumeZScore))

		if len(data.HourlyContext.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.HourlyContext.MACDValues)))
		}

		if len(data.HourlyContext.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.HourlyContext.RSI14Values)))
		}

		// 📊 Bollinger Bands (1H 體制識別)
		if data.HourlyContext.BB != nil {
			bb := data.HourlyContext.BB
			sb.WriteString("📊 Bollinger Bands (1H):\n")
			sb.WriteString(fmt.Sprintf("  Upper: %.2f | Middle: %.2f | Lower: %.2f\n", bb.Upper, bb.Middle, bb.Lower))
			sb.WriteString(fmt.Sprintf("  %%B: %.2f | Bandwidth: %.2f%%\n", bb.PercentB, bb.Bandwidth))
			if bb.Squeeze {
				sb.WriteString("  ⚠️ Squeeze: YES (Volatility Contracting - WAIT for Direction)\n")
			}
			sb.WriteString(fmt.Sprintf("  Regime: %s | BW Rank: %d%%\n\n", bb.Regime, bb.BWRank))
		}
	}

	if data.LongerTermContext != nil {
		sb.WriteString("Longer‑term context (4‑hour timeframe):\n\n")

		sb.WriteString(fmt.Sprintf("20‑Period EMA: %.3f vs. 50‑Period EMA: %.3f\n\n",
			data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))

		sb.WriteString(fmt.Sprintf("3‑Period ATR: %.3f vs. 14‑Period ATR: %.3f\n\n",
			data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))

		sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f (Volume Z-Score: %.2f)\n\n",
			data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume, data.LongerTermContext.VolumeZScore))

		if len(data.LongerTermContext.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
		}

		if len(data.LongerTermContext.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
		}

		// 📊 Bollinger Bands (4H 體制識別)
		if data.LongerTermContext.BB != nil {
			bb := data.LongerTermContext.BB
			sb.WriteString("📊 Bollinger Bands (4H):\n")
			sb.WriteString(fmt.Sprintf("  Upper: %.2f | Middle: %.2f | Lower: %.2f\n", bb.Upper, bb.Middle, bb.Lower))
			sb.WriteString(fmt.Sprintf("  %%B: %.2f | Bandwidth: %.2f%%\n", bb.PercentB, bb.Bandwidth))
			if bb.Squeeze {
				sb.WriteString("  ⚠️ Squeeze: YES (Volatility Contracting - WAIT for Direction)\n")
			}
			sb.WriteString(fmt.Sprintf("  Regime: %s | BW Rank: %d%%\n\n", bb.Regime, bb.BWRank))
		}
	}

	return sb.String()
}

// formatFloatSlice 格式化float64切片为字符串（限制长度以节省 token）
func formatFloatSlice(values []float64) string {
	const maxDisplay = 30
	displayValues := values
	prefix := ""
	if len(values) > maxDisplay {
		displayValues = values[len(values)-maxDisplay:]
		prefix = "...(truncated)... "
	}

	strValues := make([]string, len(displayValues))
	for i, v := range displayValues {
		strValues[i] = fmt.Sprintf("%.3f", v)
	}
	return prefix + "[" + strings.Join(strValues, ", ") + "]"
}

// Phase 5: Calculate Macro Context (1D Analysis)
func calculateMacroContext(klines []Kline) *MacroData {
	if len(klines) < 50 {
		return nil
	}

	closePrices := make([]float64, len(klines))
	for i, k := range klines {
		closePrices[i] = k.Close
	}

	// EMA
	// Use calculateEMA helper (assuming it accepts float64 slice? No, it takes []Kline)
	// We need to adapt or use existing helpers.
	// calculateEMA takes []Kline.

	ema20 := calculateEMA(klines, 20)
	ema50 := calculateEMA(klines, 50)

	// RSI ?? calculateRSI takes []float64? Let's check.
	// We need to check CalculateRSI definition.
	// Assuming there is a calculateRSI helper or similar.
	// If not, we might need to implement it or use what's available.
	// Looking at file view, we have calculateIntradaySeries.
	// Let's assume we need to implement or find the RSI function.
	// Line 258: calculateRSISeries(klines).

	rsiSeries := calculateRSISeries(klines, 14)
	currentRSI := 0.0
	if len(rsiSeries) > 0 {
		currentRSI = rsiSeries[len(rsiSeries)-1]
	}

	currentEMA20 := ema20
	currentEMA50 := ema50
	currentPrice := closePrices[len(closePrices)-1] // Keep this line, it was there before.

	// Trend State
	trend := "Neutral"
	if currentPrice > currentEMA20 && currentEMA20 > currentEMA50 {
		trend = "Strong Bullish"
	} else if currentPrice > currentEMA20 {
		trend = "Bullish"
	} else if currentPrice < currentEMA20 && currentEMA20 < currentEMA50 {
		trend = "Strong Bearish"
	} else if currentPrice < currentEMA20 {
		trend = "Bearish"
	}

	// Structure (Simple Swing Points)
	// Last 20 days high/low
	highest20 := 0.0
	lowest20 := 100000000.0
	for i := len(klines) - 21; i < len(klines)-1; i++ { // Excluding current incomplete candle? No, include it.
		if i < 0 {
			continue
		}
		if klines[i].High > highest20 {
			highest20 = klines[i].High
		}
		if klines[i].Low < lowest20 {
			lowest20 = klines[i].Low
		}
	}

	structState := "Neutral"
	if currentPrice > highest20 {
		structState = "Breakout (New 20D High)"
	} else if currentPrice < lowest20 {
		structState = "Breakdown (New 20D Low)"
	}

	return &MacroData{
		TrendState1D:  trend,
		StructState1D: structState,
		EMA20_1D:      currentEMA20,
		EMA50_1D:      currentEMA50,
		RSI1D:         currentRSI,
		KeyLevels:     []float64{highest20, lowest20},
	}
}

// Normalize 标准化symbol,确保是USDT交易对
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat 解析float值
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// calculateLiquidityClusters identifies potential liquidity pools based on unbroken pivots
// Logic: A Swing High that has NOT been swept by subsequent price action is a "Short Liquidity Pool".
// A Swing Low that has not been swept is a "Long Liquidation Pool".
func calculateLiquidityClusters(klines []Kline) []LiquidityCluster {
	var clusters []LiquidityCluster
	if len(klines) < 5 {
		return clusters
	}

	// Lookback window for pivot definition (fractal: High[i] > neighbors)
	for i := 2; i < len(klines)-2; i++ {
		// 1. Identify Swing High
		isSwingHigh := klines[i].High > klines[i-1].High &&
			klines[i].High > klines[i-2].High &&
			klines[i].High > klines[i+1].High &&
			klines[i].High > klines[i+2].High

		if isSwingHigh {
			swept := false
			for j := i + 1; j < len(klines); j++ {
				if klines[j].High > klines[i].High {
					swept = true
					break
				}
			}
			if !swept {
				clusters = append(clusters, LiquidityCluster{
					Price:     klines[i].High,
					Volume:    klines[i].Volume,
					Type:      "Short Liquidation (Buy Stops)",
					Timestamp: klines[i].CloseTime,
				})
			}
		}

		// 2. Identify Swing Low
		isSwingLow := klines[i].Low < klines[i-1].Low &&
			klines[i].Low < klines[i-2].Low &&
			klines[i].Low < klines[i+1].Low &&
			klines[i].Low < klines[i+2].Low

		if isSwingLow {
			swept := false
			for j := i + 1; j < len(klines); j++ {
				if klines[j].Low < klines[i].Low {
					swept = true
					break
				}
			}
			if !swept {
				clusters = append(clusters, LiquidityCluster{
					Price:     klines[i].Low,
					Volume:    klines[i].Volume,
					Type:      "Long Liquidation (Sell Stops)",
					Timestamp: klines[i].CloseTime,
				})
			}
		}
	}
	return clusters
}
