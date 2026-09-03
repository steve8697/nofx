package decision

import (
	"encoding/json"
	"fmt"
	"aetheris/logger"
)

// CycleReplay 把历史决策日志再跑一遍当前 sanitize 规则（不碰交易所）。
type CycleReplay struct {
	Timestamp string
	Cycle     int
	Equity    float64
	Proposed  []string
	Kept      []string
	Executed  []string
	Dropped   []string
	Success   bool
	Error     string
}

func formatAct(symbol, action string) string {
	if symbol == "" {
		return action
	}
	return action + " " + symbol
}

func proposedFromRecord(rec logger.DecisionRecord) []Decision {
	if rec.DecisionJSON != "" {
		var list []Decision
		if err := json.Unmarshal([]byte(rec.DecisionJSON), &list); err == nil && len(list) > 0 {
			return list
		}
	}
	out := make([]Decision, 0, len(rec.Decisions))
	for _, d := range rec.Decisions {
		out = append(out, Decision{
			Symbol:          d.Symbol,
			Action:          d.Action,
			Leverage:        d.Leverage,
			StopLoss:        d.StopLoss,
			TakeProfit:      d.TakeProfit,
			Reasoning:       d.Reasoning,
			ExecutionError:  d.Error,
			PositionSizeUSD: d.Price * d.Quantity,
		})
	}
	return out
}

func positionsFromRecord(rec logger.DecisionRecord) []PositionInfo {
	out := make([]PositionInfo, 0, len(rec.Positions))
	for _, p := range rec.Positions {
		out = append(out, PositionInfo{
			Symbol:           p.Symbol,
			Side:             p.Side,
			EntryPrice:       p.EntryPrice,
			MarkPrice:        p.MarkPrice,
			Quantity:         p.PositionAmt,
			Leverage:         int(p.Leverage),
			UnrealizedPnL:    p.UnrealizedProfit,
			LiquidationPrice: p.LiquidationPrice,
			StopLoss:         p.StopLoss,
			TakeProfit:       p.TakeProfit,
		})
	}
	return out
}

// ReplaySanitize 用当前验证器重放一条历史记录。没有当时的完整行情，
// 开仓可能因缺 market data 被丢掉——这是回放限制，不是当时一定非法。
func ReplaySanitize(rec logger.DecisionRecord, btcEthLeverage, altcoinLeverage, maxPositions int, minRR float64) CycleReplay {
	proposed := proposedFromRecord(rec)
	kept := SanitizeDecisions(proposed, rec.AccountState.TotalBalance, btcEthLeverage, altcoinLeverage, positionsFromRecord(rec), nil, minRR, maxPositions, nil)

	out := CycleReplay{
		Timestamp: rec.Timestamp.Format("2006-01-02 15:04:05"),
		Cycle:     rec.CycleNumber,
		Equity:    rec.AccountState.TotalBalance,
		Success:   rec.Success,
		Error:     rec.ErrorMessage,
	}
	keptSet := map[string]bool{}
	for _, d := range proposed {
		out.Proposed = append(out.Proposed, formatAct(d.Symbol, d.Action))
	}
	for _, d := range kept {
		label := formatAct(d.Symbol, d.Action)
		out.Kept = append(out.Kept, label)
		keptSet[label] = true
	}
	for _, label := range out.Proposed {
		if !keptSet[label] {
			out.Dropped = append(out.Dropped, label)
		}
	}
	for _, d := range rec.Decisions {
		exec := formatAct(d.Symbol, d.Action)
		if !d.Success {
			exec += " FAIL"
		}
		out.Executed = append(out.Executed, exec)
	}
	return out
}

func (r CycleReplay) Summary() string {
	return fmt.Sprintf("cycle %d %s equity=%.2f proposed=%v kept=%v dropped=%v executed=%v ok=%v %s",
		r.Cycle, r.Timestamp, r.Equity, r.Proposed, r.Kept, r.Dropped, r.Executed, r.Success, r.Error)
}
