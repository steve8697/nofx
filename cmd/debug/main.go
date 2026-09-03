package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"aetheris/config"
	"aetheris/decision"
	"aetheris/logger"
	"aetheris/trader"
)

func main() {
	cmd := "help"
	rest := os.Args[1:]
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		cmd = rest[0]
		rest = rest[1:]
	}
	fs := flag.NewFlagSet("debug", flag.ExitOnError)
	dir := fs.String("dir", "", "decision_logs 目录（默认最新的 trader 目录）")
	n := fs.Int("n", 50, "inspect/replay 最近 N 条")
	dbPath := fs.String("db", "config.db", "配置库")
	_ = fs.Parse(rest)

	switch cmd {
	case "inspect":
		runInspect(resolveDir(*dir), *n)
	case "replay":
		runReplay(resolveDir(*dir), *n)
	case "snapshot":
		runSnapshot(*dbPath)
	case "dryrun":
		runDryRun(*dbPath)
	default:
		fmt.Print(`NOFX debug — 默认不实盘、不下单

用法:
  go run ./cmd/debug inspect [-n 50] [-dir decision_logs/...]
  go run ./cmd/debug replay  [-n 50] [-dir decision_logs/...]
  go run ./cmd/debug snapshot
  go run ./cmd/debug dryrun

层:
  inspect  只读日志：动作分布、连续 wait、净值
  replay   用当前验证器重放日志（无交易所、无 AI）
  snapshot 只读打交易所：余额/持仓/行情 + 风控数字
  dryrun   在写操作包装上模拟 OpenLong（保证不发单）

实盘不在这里。preflight: go run ./cmd/preflight
`)
	}
}

func resolveDir(dir string) string {
	if dir != "" {
		return dir
	}
	root := "decision_logs"
	entries, err := os.ReadDir(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读 %s 失败: %v\n", root, err)
		os.Exit(1)
	}
	var best string
	var bestTime time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(root, e.Name())
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.ModTime().After(bestTime) {
			bestTime = info.ModTime()
			best = p
		}
	}
	if best == "" {
		fmt.Fprintln(os.Stderr, "没有 decision_logs 子目录")
		os.Exit(1)
	}
	return best
}

type slimRecord struct {
	Timestamp    time.Time                `json:"timestamp"`
	CycleNumber  int                      `json:"cycle_number"`
	DecisionJSON string                   `json:"decision_json"`
	AccountState logger.AccountSnapshot   `json:"account_state"`
	Decisions    []logger.DecisionAction  `json:"decisions"`
	Success      bool                     `json:"success"`
	ErrorMessage string                   `json:"error_message"`
}

func loadSlim(dir string, n int) []slimRecord {
	names, err := filepath.Glob(filepath.Join(dir, "decision_*.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "glob: %v\n", err)
		os.Exit(1)
	}
	sort.Strings(names)
	if n > 0 && len(names) > n {
		names = names[len(names)-n:]
	}
	out := make([]slimRecord, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var rec slimRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func toLogger(s slimRecord) logger.DecisionRecord {
	return logger.DecisionRecord{
		Timestamp:    s.Timestamp,
		CycleNumber:  s.CycleNumber,
		DecisionJSON: s.DecisionJSON,
		AccountState: s.AccountState,
		Decisions:    s.Decisions,
		Success:      s.Success,
		ErrorMessage: s.ErrorMessage,
	}
}

func runInspect(dir string, n int) {
	recs := loadSlim(dir, n)
	fmt.Printf("inspect %s  (%d records)\n", dir, len(recs))
	if len(recs) == 0 {
		return
	}
	actions := map[string]int{}
	waitStreak, maxWait := 0, 0
	fails := 0
	for _, rec := range recs {
		if !rec.Success || rec.ErrorMessage != "" {
			fails++
		}
		isWaitOnly := true
		for _, d := range rec.Decisions {
			act := strings.ToLower(d.Action)
			actions[act]++
			if act != "wait" && act != "hold" {
				isWaitOnly = false
			}
		}
		if len(rec.Decisions) == 0 {
			isWaitOnly = true
			actions["(empty)"]++
		}
		if isWaitOnly {
			waitStreak++
			if waitStreak > maxWait {
				maxWait = waitStreak
			}
		} else {
			waitStreak = 0
		}
	}
	first, last := recs[0], recs[len(recs)-1]
	fmt.Printf("range: %s → %s\n", first.Timestamp.Format("2006-01-02 15:04"), last.Timestamp.Format("2006-01-02 15:04"))
	fmt.Printf("equity: %.4f → %.4f\n", first.AccountState.TotalBalance, last.AccountState.TotalBalance)
	fmt.Printf("failed/error cycles: %d\n", fails)
	fmt.Printf("max consecutive wait/hold/empty: %d\n", maxWait)
	fmt.Printf("actions: %v\n", actions)
	fmt.Println("last 5:")
	start := len(recs) - 5
	if start < 0 {
		start = 0
	}
	for _, rec := range recs[start:] {
		acts := []string{}
		for _, d := range rec.Decisions {
			acts = append(acts, d.Action+" "+d.Symbol)
		}
		errMsg := rec.ErrorMessage
		if len(errMsg) > 140 {
			errMsg = strings.Split(errMsg, "\n")[0]
			if len(errMsg) > 140 {
				errMsg = errMsg[:140] + "..."
			}
		}
		fmt.Printf("  #%d %s eq=%.2f %v ok=%v %s\n",
			rec.CycleNumber, rec.Timestamp.Format("15:04:05"), rec.AccountState.TotalBalance, acts, rec.Success, errMsg)
	}
}

func runReplay(dir string, n int) {
	recs := loadSlim(dir, n)
	fmt.Printf("replay %s  (%d records)  — 当前验证器，无交易所\n", dir, len(recs))
	droppedOpens := 0
	for _, rec := range recs {
		got := decision.ReplaySanitize(toLogger(rec), 5, 5, 3, 2.0)
		for _, d := range got.Dropped {
			if strings.HasPrefix(d, "open_") {
				droppedOpens++
			}
		}
		if len(got.Dropped) > 0 {
			fmt.Println(" ", got.Summary())
		}
	}
	fmt.Printf("cycles with dropped opens under current sanitizer: %d / %d\n", droppedOpens, len(recs))
	fmt.Println("注: 回放没有当时完整行情，open 缺 market data 会被丢掉，不代表当时非法。")
}

func runSnapshot(dbPath string) {
	fmt.Println("snapshot — 只读交易所 API，不下单、不调 AI")
	db, err := config.NewDatabase(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	maxDaily, _ := strconv.ParseFloat(mustConfig(db, "max_daily_loss", "10"), 64)
	maxDD, _ := strconv.ParseFloat(mustConfig(db, "max_drawdown", "20"), 64)

	traders, err := db.GetAllTraders()
	if err != nil || len(traders) == 0 {
		fmt.Fprintf(os.Stderr, "无交易员: %v\n", err)
		os.Exit(1)
	}
	tr := traders[0]
	exchanges, err := db.GetExchanges(tr.UserID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exchanges: %v\n", err)
		os.Exit(1)
	}
	var ex *config.ExchangeConfig
	for _, item := range exchanges {
		if item.ID == tr.ExchangeID {
			ex = item
			break
		}
	}
	if ex == nil {
		fmt.Fprintln(os.Stderr, "找不到交易所配置")
		os.Exit(1)
	}
	client, err := newReadOnlyClient(ex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: %v\n", err)
		os.Exit(1)
	}
	bal, err := client.GetBalance()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetBalance: %v\n", err)
		os.Exit(1)
	}
	wallet, _ := bal["totalWalletBalance"].(float64)
	unreal, _ := bal["totalUnrealizedProfit"].(float64)
	equity := wallet + unreal
	pos, err := client.GetPositions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetPositions: %v\n", err)
		os.Exit(1)
	}
	px, err := client.GetMarketPrice("BTCUSDT")
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetMarketPrice: %v\n", err)
		os.Exit(1)
	}
	pnlPct := 0.0
	if tr.InitialBalance > 0 {
		pnlPct = (equity - tr.InitialBalance) / tr.InitialBalance * 100
	}
	kind, reason, halt := trader.EvaluateRiskHalt(trader.RiskHaltInput{
		DailyPnL:        0,
		InitialBalance:  tr.InitialBalance,
		TotalPnLPct:     pnlPct,
		MaxDailyLossPct: maxDaily,
		MaxDrawdownPct:  maxDD,
	})
	fmt.Printf("trader %s is_running=%v initial=%.4f\n", tr.Name, tr.IsRunning, tr.InitialBalance)
	fmt.Printf("equity=%.4f wallet=%.4f uPnL=%.4f pnl=%.2f%% positions=%d\n", equity, wallet, unreal, pnlPct, len(pos))
	fmt.Printf("BTCUSDT=%.4f\n", px)
	fmt.Printf("halt limits daily=%.1f%% drawdown=%.1f%% → halt=%v kind=%d %s\n", maxDaily, maxDD, halt, kind, reason)
}

func runDryRun(dbPath string) {
	fmt.Println("dryrun — 真实只读 + 写操作走 DryRunTrader（不发单）")
	db, err := config.NewDatabase(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	traders, err := db.GetAllTraders()
	if err != nil || len(traders) == 0 {
		fmt.Fprintf(os.Stderr, "无交易员: %v\n", err)
		os.Exit(1)
	}
	tr := traders[0]
	exchanges, _ := db.GetExchanges(tr.UserID)
	var ex *config.ExchangeConfig
	for _, item := range exchanges {
		if item.ID == tr.ExchangeID {
			ex = item
			break
		}
	}
	if ex == nil {
		fmt.Fprintln(os.Stderr, "找不到交易所配置")
		os.Exit(1)
	}
	live, err := newReadOnlyClient(ex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: %v\n", err)
		os.Exit(1)
	}
	dry := trader.NewDryRunTrader(live)
	bal, err := dry.GetBalance()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetBalance: %v\n", err)
		os.Exit(1)
	}
	wallet, _ := bal["totalWalletBalance"].(float64)
	fmt.Printf("read ok wallet=%.4f\n", wallet)
	if _, err := dry.OpenLong("BTCUSDT", 0, 1); err != nil {
		fmt.Fprintf(os.Stderr, "dry OpenLong: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("write calls captured (not sent): %v\n", dry.Calls)
	fmt.Println("dryrun 结束：没有向交易所发送开仓。")
}

func mustConfig(db *config.Database, key, fallback string) string {
	v, err := db.GetSystemConfig(key)
	if err != nil || v == "" {
		return fallback
	}
	return v
}

func newReadOnlyClient(ex *config.ExchangeConfig) (trader.Trader, error) {
	kind := strings.ToLower(ex.ID + " " + ex.Name + " " + ex.Type)
	switch {
	case strings.Contains(kind, "aster"):
		return trader.NewAsterTrader(ex.AsterUser, strings.TrimSpace(ex.AsterSigner), ex.AsterPrivateKey, ex.Testnet)
	case strings.Contains(kind, "hyperliquid"):
		return trader.NewHyperliquidTrader(ex.APIKey, ex.HyperliquidWalletAddr, ex.Testnet)
	case strings.Contains(kind, "binance"):
		return trader.NewFuturesTrader(ex.APIKey, ex.SecretKey), nil
	default:
		return nil, fmt.Errorf("不支持的交易所 id=%s type=%s", ex.ID, ex.Type)
	}
}
