package backtest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

// BacktestManager manages backtest runs
type BacktestManager struct {
	runsDir string
	active  map[string]*BacktestEngine
	mu      sync.RWMutex
}

// NewBacktestManager creates a new manager
func NewBacktestManager(dataDir string) *BacktestManager {
	runsDir := filepath.Join(filepath.Dir(dataDir), "decision_logs") // Store alongside logs for now
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		fmt.Printf("Warning: failed to create runs dir: %v\n", err)
	}
	return &BacktestManager{
		runsDir: runsDir,
		active:  make(map[string]*BacktestEngine),
	}
}

// StartRun initiates a new backtest
func (bm *BacktestManager) StartRun(config RunConfig, dataFilePath string) (*BacktestEngine, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	engine, err := NewBacktestEngine(config, dataFilePath)
	if err != nil {
		return nil, err
	}

	bm.active[config.ID] = engine

	// Run in background
	go func() {
		defer func() {
			// ✅ 物理隔離防禦性清理：清理回測產生的臨時狀態與熱力圖文件，保持磁碟潔淨
			stateFile := filepath.Join("data", fmt.Sprintf("position_state_%s.json", config.ID))
			heatmapFile := filepath.Join("data", fmt.Sprintf("liquidity_heatmap_%s.json", config.ID))
			_ = os.Remove(stateFile)
			_ = os.Remove(heatmapFile)
		}()

		engine.Config.Status = "running"
		result, err := engine.Run()
		if err != nil {
			fmt.Printf("Backtest %s failed: %v\n", config.ID, err)
			engine.Config.Status = "failed"
		} else {
			engine.Config.Status = "completed"
			bm.saveResult(config.ID, result)
		}

		// Remove from active list after completion? Or keep for querying?
		// Keep for now, maybe cleanup later.
	}()

	return engine, nil
}

func (bm *BacktestManager) saveResult(id string, result *BacktestResult) {
	filename := filepath.Join(bm.runsDir, fmt.Sprintf("backtest_%s.json", id))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal result for %s: %v\n", id, err)
		return
	}
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		fmt.Printf("Failed to write result file for %s: %v\n", id, err)
	}
}

// ListRuns returns all backtest runs (active and persisted)
func (bm *BacktestManager) ListRuns() ([]RunConfig, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var runs []RunConfig

	// 1. Add active runs
	for _, engine := range bm.active {
		runs = append(runs, engine.Config)
	}

	// 2. Read from disk (simplified: just list files)
	// Ideally we should cache this or database it.
	files, err := ioutil.ReadDir(bm.runsDir)
	if err != nil {
		return runs, nil // Return what we have
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" && len(f.Name()) > 9 && f.Name()[:9] == "backtest_" {
			// Check if already in active list
			id := f.Name()[9 : len(f.Name())-5]
			if _, active := bm.active[id]; active {
				continue
			}

			// Read file metadata
			content, err := ioutil.ReadFile(filepath.Join(bm.runsDir, f.Name()))
			if err == nil {
				var result BacktestResult
				if err := json.Unmarshal(content, &result); err == nil {
					runs = append(runs, result.Config)
				}
			}
		}
	}

	return runs, nil
}

// GetRun returns details for a specific run
func (bm *BacktestManager) GetRun(id string) (*BacktestResult, error) {
	bm.mu.RLock()
	// Check active first
	if engine, ok := bm.active[id]; ok {
		// Construct partial result
		res := &BacktestResult{
			Config:      engine.Config,
			EquityCurve: engine.Equity,
			// Trades?
		}
		bm.mu.RUnlock()
		return res, nil
	}
	bm.mu.RUnlock()

	// Check disk
	filename := filepath.Join(bm.runsDir, fmt.Sprintf("backtest_%s.json", id))
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("run not found: %s", id)
	}

	var result BacktestResult
	if err := json.Unmarshal(content, &result); err != nil {
		return nil, fmt.Errorf("failed to parse result: %v", err)
	}
	return &result, nil
}
