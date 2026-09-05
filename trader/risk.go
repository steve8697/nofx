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
	HaltConsecutiveLoss
)

// RiskHaltInput 用账户快照判断是否该暂停新开仓。
type RiskHaltInput struct {
	DailyPnL          float64
	InitialBalance    float64
	TotalPnLPct       float64
	PeakDrawdownPct   float64 // 峰值高水位回撤百分比: (TotalEquity - PeakEquity) / PeakEquity * 100
	MaxDailyLossPct   float64
	MaxDrawdownPct    float64
	ConsecutiveLosses int     // 连续亏损次数（防止 Tilt 和报复性开仓）
}

// EvaluateRiskHalt 只看数字，不碰交易所。未配置上限（<=0）视为关闭该项。
func EvaluateRiskHalt(in RiskHaltInput) (HaltKind, string, bool) {
	// 1. 连续亏损硬熔断：2连亏暂停45分钟，3+连亏暂停24小时
	if in.ConsecutiveLosses >= 3 {
		reason := fmt.Sprintf("连续亏损达到 %d 次，触发 24 小时硬熔断冷静期", in.ConsecutiveLosses)
		return HaltConsecutiveLoss, reason, true
	}
	if in.ConsecutiveLosses >= 2 {
		reason := fmt.Sprintf("连续亏损达到 %d 次，触发 45 分钟冷静期冷却", in.ConsecutiveLosses)
		return HaltConsecutiveLoss, reason, true
	}

	if in.MaxDailyLossPct > 0 && in.InitialBalance > 0 {
		dailyPct := (in.DailyPnL / in.InitialBalance) * 100
		if dailyPct <= -in.MaxDailyLossPct {
			reason := fmt.Sprintf("日亏损 %.2f%% 达到上限 -%.2f%%（当日盈亏 %.4f / 本金 %.2f）",
				dailyPct, in.MaxDailyLossPct, in.DailyPnL, in.InitialBalance)
			return HaltDailyLoss, reason, true
		}
	}
	// 峰值高水位回撤判定（优先使用真实的 PeakDrawdownPct；若未提供则兼容回退到 TotalPnLPct）
	effectiveDrawdown := in.PeakDrawdownPct
	if effectiveDrawdown == 0 && in.TotalPnLPct < 0 {
		effectiveDrawdown = in.TotalPnLPct
	}
	if in.MaxDrawdownPct > 0 && effectiveDrawdown <= -in.MaxDrawdownPct {
		reason := fmt.Sprintf("峰值回撤 %.2f%% 达到上限 -%.2f%%", effectiveDrawdown, in.MaxDrawdownPct)
		return HaltDrawdown, reason, true
	}
	return HaltNone, "", false
}

// HaltUntil 日亏损暂停到下一个本地自然日 00:00；连亏按梯度冷却；回撤按配置时长暂停。
func HaltUntil(now time.Time, kind HaltKind, pause time.Duration) time.Time {
	if kind == HaltDailyLoss {
		y, m, d := now.Date()
		return time.Date(y, m, d+1, 0, 0, 0, 0, now.Location())
	}
	if kind == HaltConsecutiveLoss {
		if pause <= 0 {
			pause = 45 * time.Minute
		}
		return now.Add(pause)
	}
	if pause <= 0 {
		pause = time.Hour
	}
	return now.Add(pause)
}
