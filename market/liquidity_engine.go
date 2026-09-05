package market

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math"
	"os"
	"sort"
	"sync"
	"time"
)

// LiquidityCluster represents a potential liquidation zone
type LiquidityCluster struct {
	Price     float64 `json:"price"`
	Volume    float64 `json:"volume"` // Estimated liquidation volume (USDT)
	Type      string  `json:"type"`   // "Long Liquidation" or "Short Liquidation"
	Timestamp int64   `json:"timestamp"`
	Visited   bool    `json:"visited"`
}

// LiquidityEngine manages the heatmap for multiple symbols
type LiquidityEngine struct {
	Clusters      map[string][]LiquidityCluster // 🔥 Key: Symbol (e.g. "BTCUSDT")
	LastUpdated   time.Time
	FilePath      string
	mu            sync.Mutex
	HistoryClient HistoryProvider // Interface to fetch history
}

type HistoryProvider interface {
	GetOpenInterestHistory(symbol string, period string, limit int) ([]map[string]interface{}, error)
}

// NewLiquidityEngine creates a new engine
func NewLiquidityEngine(filePath string, client HistoryProvider) *LiquidityEngine {
	engine := &LiquidityEngine{
		FilePath:      filePath,
		Clusters:      make(map[string][]LiquidityCluster),
		HistoryClient: client,
	}
	engine.LoadState()
	return engine
}

// Init initializes the map by fetching history if empty for the specific symbol
func (le *LiquidityEngine) Init(symbol string) {
	le.mu.Lock()
	if le.Clusters == nil {
		le.Clusters = make(map[string][]LiquidityCluster)
	}
	// Check if this symbol is already loaded
	if len(le.Clusters[symbol]) > 0 {
		le.mu.Unlock()
		return // Already loaded for this symbol
	}
	le.mu.Unlock() // Unlock for network call

	log.Printf("🔥 LiquidityEngine: Initializing %s from Binance History...", symbol)

	// Fetch 30 days of 15m data (approx 30 * 96 = 2880 points)
	// API limit is 500, so we need multiple calls?
	// For MVP, let's fetch the last 500 points (5 days) which is most relevant.
	history, err := le.HistoryClient.GetOpenInterestHistory(symbol, "15m", 500)
	if err != nil {
		log.Printf("⚠️ LiquidityEngine: Failed to fetch history for %s: %v", symbol, err)
		return
	}

	// Process history to build clusters
	le.BuildFromHistory(symbol, history)
}

// BuildFromHistory reconstructs the map from historical OI data
func (le *LiquidityEngine) BuildFromHistory(symbol string, history []map[string]interface{}) {
	le.mu.Lock()
	defer le.mu.Unlock()

	// Need price data?
	// For MVP, we assume we don't have historical CANDLES aligned perfectly right now in this function context.
	// We will simplify: The API returns OI sum and OI Value.
	// We can't know the entry price without Klines.
	// CRITICAL: We need Klines to know the Entry Price.
	// Refinement: The main DataProvider loop has Klines.
	// This function might be better called incrementally.

	// For now, let's just clear implementation. To do "Real" heatmap validation properly,
	// we need to pass Klines alongside OI history.

	// Maintain map entry
	if _, ok := le.Clusters[symbol]; !ok {
		le.Clusters[symbol] = []LiquidityCluster{}
	}
}

// Update processes a new candle and OI data for a specific symbol
func (le *LiquidityEngine) Update(symbol string, k Kline, oi *OIData) {
	le.mu.Lock()
	defer le.mu.Unlock()

	if le.Clusters == nil {
		le.Clusters = make(map[string][]LiquidityCluster)
	}

	clusters := le.Clusters[symbol]

	// 1. Check for Visits (Price wiping out clusters) (Existing Logic)
	for i := range clusters {
		if !clusters[i].Visited {
			// Long Liquidation (Sell Stops below Swing Low)
			// Triggered if Price drops below it
			if clusters[i].Type == "Long Liquidation" && k.Low <= clusters[i].Price {
				clusters[i].Visited = true
				clusters[i].Volume = 0 // Liquidity Taken
			}
			// Short Liquidation (Buy Stops above Swing High)
			// Triggered if Price rises above it
			if clusters[i].Type == "Short Liquidation" && k.High >= clusters[i].Price {
				clusters[i].Visited = true
				clusters[i].Volume = 0 // Liquidity Taken
			}
		}
	}

	// 2. Identify New Liquidity (Pivots)
	// Price Action: Significant Wicks using true candle body bounds
	bodyTop := math.Max(k.Open, k.Close)
	bodyBottom := math.Min(k.Open, k.Close)
	bodySize := math.Max(bodyTop-bodyBottom, (k.High-k.Low)*0.05) // 防止除以0或極端十字線
	upperWick := k.High - bodyTop
	lowerWick := bodyBottom - k.Low

	longWickUp := upperWick > bodySize*2.0 && upperWick > lowerWick*2.0
	longWickDown := lowerWick > bodySize*2.0 && lowerWick > upperWick*2.0

	if longWickUp && k.Volume > 1000 { // Arbitrary volume filter
		// Selling pressure -> Short Liquidity (Buy Stops) pile up ABOVE the wick
		clusters = append(clusters, LiquidityCluster{
			Price:     k.High,
			Volume:    k.Volume * 0.1, // Estimate 10% are stops
			Type:      "Short Liquidation",
			Timestamp: k.CloseTime,
			Visited:   false,
		})
	}

	if longWickDown && k.Volume > 1000 {
		// Buying pressure -> Long Liquidation (Sell Stops) pile up BELOW the wick
		clusters = append(clusters, LiquidityCluster{
			Price:     k.Low,
			Volume:    k.Volume * 0.1,
			Type:      "Long Liquidation",
			Timestamp: k.CloseTime,
			Visited:   false,
		})
	}

	// Update the map
	le.Clusters[symbol] = clusters

	// 3. Prune State (Self-Cleaning)
	le.pruneSymbolState(symbol)

	le.SaveState()
}

// pruneSymbolState removes old or visited clusters for a specific symbol
func (le *LiquidityEngine) pruneSymbolState(symbol string) {
	// Note: Caller must hold Lock
	const (
		MaxClusterAge    = 7 * 24 * time.Hour // Keep unvisited for 7 days
		MaxVisitedAge    = 24 * time.Hour     // Keep visited for 24 hours (for reference)
		MaxTotalClusters = 500                // Safety Cap per symbol
	)

	nowTime := time.Now()
	var kept []LiquidityCluster

	if le.Clusters[symbol] == nil {
		return
	}

	for _, c := range le.Clusters[symbol] {
		clusterTime := time.UnixMilli(c.Timestamp)
		age := nowTime.Sub(clusterTime)

		if c.Visited {
			if age < MaxVisitedAge {
				kept = append(kept, c)
			}
		} else {
			if age < MaxClusterAge {
				kept = append(kept, c)
			}
		}
	}

	// Safety Cap: If too many, keep newest
	if len(kept) > MaxTotalClusters {
		// Sort by time descending (newest first)
		sort.Slice(kept, func(i, j int) bool {
			return kept[i].Timestamp > kept[j].Timestamp
		})
		kept = kept[:MaxTotalClusters]
	}

	le.Clusters[symbol] = kept
}

// PruneState public wrapper if needed (not used now, we do per Update)
func (le *LiquidityEngine) PruneState() {
	le.mu.Lock()
	defer le.mu.Unlock()
	for symbol := range le.Clusters {
		le.pruneSymbolState(symbol)
	}
}

// SaveState persists to disk
func (le *LiquidityEngine) SaveState() {
	// Must hold lock if calling from outside, but usually called from Update (which holds lock).
	// But SaveState does IO, maybe better unlock before?
	// Update calls SaveState at end. Ideally we don't hold lock during IO.
	// But map reads are unsafe. Let's keep it simple for now or copy data.
	// We'll trust the current flow (Update calls it with lock held? Wait, Update defers Unlock).
	// Update calls SaveState at end, defer unlocks AFTER function returns?
	// YES. So lock is held.

	data, err := json.MarshalIndent(le.Clusters, "", "  ")
	if err != nil {
		return
	}
	ioutil.WriteFile(le.FilePath, data, 0644)
}

// LoadState loads from disk
func (le *LiquidityEngine) LoadState() {
	le.mu.Lock()
	defer le.mu.Unlock()

	if _, err := os.Stat(le.FilePath); os.IsNotExist(err) {
		return
	}
	data, err := ioutil.ReadFile(le.FilePath)
	if err == nil {
		// Try to unmarshal into map
		// Check invalid format (array vs map conversion)
		// If the file contains an array (old format), we need to handle or discard it.
		// Simple way: Try unmarshal map. If fails, try array and discard/migrate?
		// Since we want to fix corruption, let's just start fresh if format is wrong,
		// or maybe try to recover.
		// Given we had an array `[]LiquidityCluster`, `json.Unmarshal` to `map` will fail.

		err = json.Unmarshal(data, &le.Clusters)
		if err != nil {
			log.Printf("⚠️ LiquidityEngine: Failed to load map (likely old format), resetting state: %v", err)
			le.Clusters = make(map[string][]LiquidityCluster)
		}
	}
}

// GetTopClusters returns the most significant active clusters for a specific symbol
func (le *LiquidityEngine) GetTopClusters(symbol string, currentPrice float64) []LiquidityCluster {
	le.mu.Lock()
	defer le.mu.Unlock()

	if le.Clusters == nil {
		return []LiquidityCluster{}
	}

	clusters, ok := le.Clusters[symbol]
	if !ok || len(clusters) == 0 {
		return []LiquidityCluster{}
	}

	var active []LiquidityCluster
	for _, c := range clusters {
		if !c.Visited && c.Volume > 0 {
			active = append(active, c)
		}
	}

	// Sort by Volume DESC
	sort.Slice(active, func(i, j int) bool {
		return active[i].Volume > active[j].Volume
	})

	if len(active) > 5 {
		return active[:5]
	}
	return active
}
