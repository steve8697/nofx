package trader

import (
	"encoding/json"
	"fmt"
	"log"
	"aetheris/decision"
	"os"
	"path/filepath"
	"sync"
)

// PersistenceManager 管理持久化状态
type PersistenceManager struct {
	filePath string
	mu       sync.RWMutex
}

// PositionState 持久化的持仓状态和风控状态
type PositionState struct {
	FirstSeenTimes     map[string]int64        `json:"first_seen_times"`
	ActiveTradeReasons map[string]string       `json:"active_trade_reasons"` // 活跃持仓的开仓理由 (Symbol_Side -> Reason)
	DailyLoss          float64                 `json:"daily_loss"`           // 当日累计亏损
	ConsecutiveLosses  int                     `json:"consecutive_losses"`   // 连续亏损次数
	LastTradeTime      int64                   `json:"last_trade_time"`      // 上次交易时间戳
	LastResetTime      int64                   `json:"last_reset_time"`      // 上次重置日盈亏时间戳
	CallCount          int                     `json:"call_count"`           // AI调用次数 / AI决策周期
	PeakPnLCache       map[string]float64      `json:"peak_pnl_cache"`       // 各币种盈亏峰值缓存
	DecisionHistory    []decision.FullDecision `json:"decision_history"`     // 决策历史记录（最近5次）
	StopUntil          int64                   `json:"stop_until,omitempty"` // 风控暂停截止（unix 秒）
}

// NewPersistenceManager 创建持久化管理器
func NewPersistenceManager(dataDir string, traderID string) *PersistenceManager {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("⚠️ 创建持久化目录失败: %v", err)
	}
	fileName := "position_state.json"
	if traderID != "" && traderID != "default_trader" {
		fileName = fmt.Sprintf("position_state_%s.json", traderID)
	}
	return &PersistenceManager{
		filePath: filepath.Join(dataDir, fileName),
	}
}

// SavePositionState 保存持仓状态
func (pm *PersistenceManager) SavePositionState(state *PositionState) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化持仓状态失败: %w", err)
	}

	if err := os.WriteFile(pm.filePath, data, 0644); err != nil {
		return fmt.Errorf("写入持仓状态文件失败: %w", err)
	}

	return nil
}

// LoadPositionState 加载持仓状态
func (pm *PersistenceManager) LoadPositionState() (*PositionState, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 如果文件不存在，返回空状态
	if _, err := os.Stat(pm.filePath); os.IsNotExist(err) {
		return &PositionState{
			FirstSeenTimes:  make(map[string]int64),
			PeakPnLCache:    make(map[string]float64),
			DecisionHistory: make([]decision.FullDecision, 0),
		}, nil
	}

	data, err := os.ReadFile(pm.filePath)
	if err != nil {
		return nil, fmt.Errorf("读取持仓状态文件失败: %w", err)
	}

	var state PositionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("解析持仓状态失败: %w", err)
	}

	if state.FirstSeenTimes == nil {
		state.FirstSeenTimes = make(map[string]int64)
	}

	if state.ActiveTradeReasons == nil {
		state.ActiveTradeReasons = make(map[string]string)
	}

	if state.PeakPnLCache == nil {
		state.PeakPnLCache = make(map[string]float64)
	}

	if state.DecisionHistory == nil {
		state.DecisionHistory = make([]decision.FullDecision, 0)
	}

	return &state, nil
}
