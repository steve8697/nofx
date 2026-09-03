package market

import "time"

// Data 市场数据结构
type Data struct {
	Symbol            string
	Timestamp         time.Time // 数据时间戳 (基于3mK线收盘时间)
	CurrentPrice      float64
	PriceChange1h     float64 // 1小时价格变化百分比
	PriceChange4h     float64 // 4小时价格变化百分比
	CurrentEMA20      float64
	CurrentMACD       float64
	CurrentMACDSignal float64 // 新增：MACD信号线
	CurrentMACDHist   float64 // 新增：MACD直方图
	CurrentRSI7       float64
	// Candle Completion Metrics (Dynamic Lag Understanding)
	CandleProgress1H float64 // 1小时K线完成度百分比 (0-100)
	TimeUntilClose1H float64 // 距离1小时K线收盘剩余分钟数
	// 15m Indicators (Explicitly requested by prompt)
	EMA15m            float64
	MACD15m           float64
	MACDSignal15m     float64 // 新增
	MACDHist15m       float64 // 新增
	RSI15m            float64
	OpenInterest      *OIData
	FundingRate       float64
	BuySellRatio      float64         // 买卖比例（TakerBuyBaseVolume / Volume，>0.5表示买方强，<0.5表示卖方强）
	IntradaySeries    *IntradayData   // 3分钟数据
	HourlyContext     *HourlyData     // 1小时数据（新增）
	LongerTermContext *LongerTermData // 4小时数据
	// CoinGecko 数据（免费API）
	CoinGeckoData *CoinGeckoData
	// Price Action 数据（新增）
	PriceAction *PriceActionData
	// Technical Analysis 数据（新增 - 逻辑固化）
	// Technical Analysis 数据（新增 - 逻辑固化）
	TechnicalAnalysis *TechnicalAnalysis
	// Sentiment Data (Fear & Greed)
	Sentiment *FearGreedData
	// Macro Context (Daily/Weekly) - Phase 5
	Macro *MacroData
}

// TechnicalAnalysis 技术分析汇总
type TechnicalAnalysis struct {
	TrendState         string                 // "Bullish", "Bearish", "Neutral"
	RSIState           string                 // "Overbought", "Oversold", "Neutral"
	VolumeState        string                 // "High", "Normal", "Low"
	VolumeZScore       float64                `json:"volume_zscore"`        // 成交量 Z-Score
	Divergence         string                 `json:"divergence"`           // "Bullish", "Bearish", "None" (RSI Divergence)
	MACDHistogramState string                 // "Expansion", "Contraction" (新增)
	MomentumReversal   string                 `json:"momentum_reversal"`    // "Bullish Reversal", "Bearish Reversal", "None" (新增)
	SignalScore        int                    `json:"signal_score"`         // 0-100 技术评分
	SMC                *SMCData               `json:"smc,omitempty"`        // Smart Money Concepts Data
	OrderFlow          *CVDData               `json:"order_flow,omitempty"` // Order Flow (CVD) Data
	VWAP               float64                `json:"vwap"`                 // Phase 19: VWAP
	OIDelta            *OIData                `json:"oi_delta"`             // Phase 20: OI Delta
	Liquidity          []LiquidityCluster     `json:"liquidity"`            // Phase 21: Liquidity Logic
	VPVR               *VolumeProfileResponse `json:"vpvr"`                 // Phase 3: Volume Profile
}

// PriceActionData 价格行为数据
type PriceActionData struct {
	UpperWickRatio float64 // 上影线比率
	LowerWickRatio float64 // 下影线比率
	BodyRatio      float64 // 实体比率
	DistToEMA20    float64 // 距离EMA20百分比
	CandleType     string  // K线形态描述 (e.g., "Bullish Pinbar", "Doji")
}

// OIData Open Interest数据
type OIData struct {
	Latest   float64
	Average  float64
	Delta15m float64 // OI Change in last 15m (Phase 20)
	Delta1H  float64 // OI Change in last 1H (Phase 20)
}

// CoinGeckoData CoinGecko免费API数据
type CoinGeckoData struct {
	PriceChange24h    float64 // 24小时价格变化百分比
	MarketCapRank     int     // 市值排名
	MarketCap         float64 // 市值（USD）
	TotalVolume24h    float64 // 24小时交易量（USD）
	PriceChange24hUSD float64 // 24小时价格变化（USD）
}

// FearGreedData 恐慌与贪婪指数数据
type FearGreedData struct {
	Value          int    // 0-100
	Classification string // "Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"
	Timestamp      int64
}

// MacroData 宏观数据 (日线/周线)
type MacroData struct {
	TrendState1D  string    // "Bullish" (Price > EMA20), "Bearish", "Neutral"
	StructState1D string    // "Bullish Structure" (HH/HL), "Bearish Structure"
	KeyLevels     []float64 // Major Support/Resistance
	RSI1D         float64
	EMA20_1D      float64
	EMA50_1D      float64
}

// BollingerBands 布林帶數據（用於體制識別，非入場信號）
type BollingerBands struct {
	Upper     float64 // 上軌 = SMA20 + 2σ
	Middle    float64 // 中軌 = SMA20 (趨勢參考)
	Lower     float64 // 下軌 = SMA20 - 2σ
	Bandwidth float64 // 帶寬 = (Upper - Lower) / Middle (百分比)
	PercentB  float64 // %B = (Price - Lower) / (Upper - Lower)
	Squeeze   bool    // 帶寬收縮 (Bandwidth < 近20週期最低)
	BWRank    int     // 帶寬排名 (0-100, 0=最窄)
	Regime    string  // "Squeeze", "Trend", "MeanReversion"
}

// IntradayData 日内数据(3分钟间隔)
type IntradayData struct {
	MidPrices        []float64
	EMA20Values      []float64
	MACDValues       []float64
	MACDSignalValues []float64 // 新增
	MACDHistValues   []float64 // 新增
	RSI7Values       []float64
	RSI14Values      []float64
}

// HourlyData 1小时时间框架数据（新增）
type HourlyData struct {
	EMA20            float64
	EMA50            float64
	ATR3             float64
	ATR14            float64
	CurrentVolume    float64
	AverageVolume    float64
	VolumeZScore     float64 // 新增：去尾成交量 Z-Score
	MACDValues       []float64
	MACDSignalValues []float64 // 新增
	MACDHistValues   []float64 // 新增
	RSI14Values      []float64
	BB               *BollingerBands // 1H 布林帶（體制識別）
}

// LongerTermData 长期数据(4小时时间框架)
type LongerTermData struct {
	EMA20            float64
	EMA50            float64
	ATR3             float64
	ATR14            float64
	CurrentVolume    float64
	AverageVolume    float64
	VolumeZScore     float64 // 新增：去尾成交量 Z-Score
	MACDValues       []float64
	MACDSignalValues []float64 // 新增
	MACDHistValues   []float64 // 新增
	RSI14Values      []float64
	BB               *BollingerBands // 4H 布林帶（體制識別）
}

// Binance API 响应结构
type ExchangeInfo struct {
	Symbols []SymbolInfo `json:"symbols"`
}

type SymbolInfo struct {
	Symbol            string `json:"symbol"`
	Status            string `json:"status"`
	BaseAsset         string `json:"baseAsset"`
	QuoteAsset        string `json:"quoteAsset"`
	ContractType      string `json:"contractType"`
	PricePrecision    int    `json:"pricePrecision"`
	QuantityPrecision int    `json:"quantityPrecision"`
}

type Kline struct {
	OpenTime            int64   `json:"openTime"`
	Open                float64 `json:"open"`
	High                float64 `json:"high"`
	Low                 float64 `json:"low"`
	Close               float64 `json:"close"`
	Volume              float64 `json:"volume"`
	CloseTime           int64   `json:"closeTime"`
	QuoteVolume         float64 `json:"quoteVolume"`
	Trades              int     `json:"trades"`
	TakerBuyBaseVolume  float64 `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume float64 `json:"takerBuyQuoteVolume"`
}

type KlineResponse []interface{}

type PriceTicker struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type Ticker24hr struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
}

// 特征数据结构
type SymbolFeatures struct {
	Symbol           string    `json:"symbol"`
	Timestamp        time.Time `json:"timestamp"`
	Price            float64   `json:"price"`
	PriceChange15Min float64   `json:"price_change_15min"`
	PriceChange1H    float64   `json:"price_change_1h"`
	PriceChange4H    float64   `json:"price_change_4h"`
	Volume           float64   `json:"volume"`
	VolumeRatio5     float64   `json:"volume_ratio_5"`
	VolumeRatio20    float64   `json:"volume_ratio_20"`
	VolumeTrend      float64   `json:"volume_trend"`
	RSI14            float64   `json:"rsi_14"`
	SMA5             float64   `json:"sma_5"`
	SMA10            float64   `json:"sma_10"`
	SMA20            float64   `json:"sma_20"`
	HighLowRatio     float64   `json:"high_low_ratio"`
	Volatility20     float64   `json:"volatility_20"`
	PositionInRange  float64   `json:"position_in_range"`
}

// 警报数据结构
type Alert struct {
	Type      string    `json:"type"`
	Symbol    string    `json:"symbol"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type Config struct {
	AlertThresholds AlertThresholds `json:"alert_thresholds"`
	UpdateInterval  int             `json:"update_interval"` // seconds
	CleanupConfig   CleanupConfig   `json:"cleanup_config"`
}

type AlertThresholds struct {
	VolumeSpike      float64 `json:"volume_spike"`
	PriceChange15Min float64 `json:"price_change_15min"`
	VolumeTrend      float64 `json:"volume_trend"`
	RSIOverbought    float64 `json:"rsi_overbought"`
	RSIOversold      float64 `json:"rsi_oversold"`
}
type CleanupConfig struct {
	InactiveTimeout   time.Duration `json:"inactive_timeout"`    // 不活跃超时时间
	MinScoreThreshold float64       `json:"min_score_threshold"` // 最低评分阈值
	NoAlertTimeout    time.Duration `json:"no_alert_timeout"`    // 无警报超时时间
	CheckInterval     time.Duration `json:"check_interval"`      // 检查间隔
}
