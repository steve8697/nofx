package trader

import (
	"fmt"
	"time"
)

// HaltKind 风控暂停类型。
type HaltKind int

const (
	HaltNone HaltKind = iota
	HaltDailyLoss
	HaltDrawdown
)

// RiskHaltInput 用账户快照判断是否该暂停新开仓。
type RiskHaltInput struct {
	DailyPnL        float64
	InitialBalance  float64
	TotalPnLPct     float64
	MaxDailyLossPct float64
	MaxDrawdownPct  float64
}

// EvaluateRiskHalt 只看数字，不碰交易所。未配置上限（<=0）视为关闭该项。
func EvaluateRiskHalt(in RiskHaltInput) (HaltKind, string, bool) {
	if in.MaxDailyLossPct > 0 && in.InitialBalance > 0 {
		dailyPct := (in.DailyPnL / in.InitialBalance) * 100
		if dailyPct <= -in.MaxDailyLossPct {
			reason := fmt.Sprintf("日亏损 %.2f%% 达到上限 -%.2f%%（当日盈亏 %.4f / 本金 %.2f）",
				dailyPct, in.MaxDailyLossPct, in.DailyPnL, in.InitialBalance)
			return HaltDailyLoss, reason, true
		}
	}
	if in.MaxDrawdownPct > 0 && in.TotalPnLPct <= -in.MaxDrawdownPct {
		reason := fmt.Sprintf("总回撤 %.2f%% 达到上限 -%.2f%%", in.TotalPnLPct, in.MaxDrawdownPct)
		return HaltDrawdown, reason, true
	}
	return HaltNone, "", false
}

// HaltUntil 日亏损暂停到下一个本地自然日 00:00；回撤按配置时长暂停。
func HaltUntil(now time.Time, kind HaltKind, pause time.Duration) time.Time {
	if pause <= 0 {
		pause = time.Hour
	}
	if kind == HaltDailyLoss {
		y, m, d := now.Date()
		return time.Date(y, m, d+1, 0, 0, 0, 0, now.Location())
	}
	return now.Add(pause)
}
