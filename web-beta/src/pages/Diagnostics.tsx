import { useEffect, useState, useCallback } from 'react'
import { Brain, TrendingUp, TrendingDown, Clock, AlertTriangle, Activity } from 'lucide-react'
import { api } from '../lib/api'

interface TradeOutcome {
	symbol: string
	side: string
	quantity: number
	leverage: number
	open_price: number
	close_price: number
	position_value: number
	margin_used: number
	pn_l: number
	pn_l_pct: number
	duration: string
	open_time: string
	close_time: string
	was_stop_loss: boolean
}

interface SymbolPerformance {
	symbol: string
	total_trades: number
	winning_trades: number
	losing_trades: number
	win_rate: number
	total_pn_l: number
	avg_pn_l: number
}

interface PerformanceAnalysis {
	total_trades: number
	winning_trades: number
	losing_trades: number
	win_rate: number
	avg_win: number
	avg_loss: number
	profit_factor: number
	sharpe_ratio: number
	recent_trades: TradeOutcome[]
	symbol_stats: { [key: string]: SymbolPerformance }
	best_symbol: string
	worst_symbol: string
}

interface DiagnosticsPageProps {
	selectedTraderId?: string
}

export function DiagnosticsPage({ selectedTraderId }: DiagnosticsPageProps) {
	const [data, setData] = useState<PerformanceAnalysis | null>(null)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState<string | null>(null)

	const loadPerformance = useCallback(() => {
		if (!selectedTraderId) return
		setLoading(true)
		api.getPerformance(selectedTraderId)
			.then(res => {
				setData(res)
				setError(null)
			})
			.catch(err => {
				console.error('Failed to load performance data:', err)
				setError(err.message || '無法讀取該交易員的診斷分析數據。請確認交易員已正常啟動並產生首筆交易。')
			})
			.finally(() => {
				setLoading(false)
			})
	}, [selectedTraderId])

	useEffect(() => {
		loadPerformance()
		const timer = setInterval(loadPerformance, 30000)
		return () => clearInterval(timer)
	}, [loadPerformance])

	if (loading && !data) {
		return (
			<div className="p-8 max-w-7xl mx-auto font-mono text-xs text-[var(--text-muted)]">
				<div className="flex items-center gap-2">
					<Activity className="w-4 h-4 animate-spin text-[var(--accent)]" />
					<span>LOADING TELEMETRY ANALYTICS...</span>
				</div>
			</div>
		)
	}

	if (error && !data) {
		return (
			<div className="p-8 max-w-7xl mx-auto">
				<div className="border border-red-500/15 bg-red-500/5 p-6 rounded-sm text-sm font-mono space-y-3">
					<div className="flex items-center gap-2 text-[var(--red)] font-bold">
						<AlertTriangle className="w-5 h-5" />
						<span>DIAGNOSTICS OFFLINE</span>
					</div>
					<p className="text-xs text-[var(--text-muted)] leading-relaxed">{error}</p>
					<button 
						onClick={loadPerformance}
						className="px-3 py-1.5 bg-red-500/10 hover:bg-red-500/15 border border-red-500/20 text-xs text-red-400 rounded-sm font-bold tracking-widest cursor-pointer transition-colors"
					>
						RETRY TELEMETRY LINK
					</button>
				</div>
			</div>
		)
	}

	// 防禦零交易狀態
	if (!data || data.total_trades === 0) {
		return (
			<div className="p-8 max-w-7xl mx-auto">
				<div className="border border-[var(--border)] bg-[var(--bg-panel)] p-8 rounded-sm text-center space-y-4">
					<Brain className="w-12 h-12 mx-auto text-[var(--text-dim)] animate-pulse" />
					<h3 className="text-sm font-bold font-mono tracking-widest text-[var(--text)] uppercase">NO CLOSED TRADES TELEMETRY</h3>
					<p className="text-xs text-[var(--text-muted)] max-w-sm mx-auto leading-relaxed">
						此交易員尚未有已平倉交易記錄。量化模型可能正在持有頭寸或等待合適的開倉信號，請等待交易員完成平倉循環後再查看。
					</p>
					<button 
						onClick={loadPerformance}
						className="px-4 py-2 bg-blue-500/5 hover:bg-blue-500/10 border border-blue-500/15 text-xs text-[var(--accent)] rounded-sm font-mono tracking-wider cursor-pointer transition-colors"
					>
						REFRESH SENSORS
					</button>
				</div>
			</div>
		)
	}

	// 取得夏普比率的安全指標框
	const getSharpeStatus = (val: number) => {
		if (val >= 2.0) return { label: 'EXCELLENT', color: 'text-[var(--green)] border-[var(--green)]/20 bg-[var(--green)]/5' }
		if (val >= 1.0) return { label: 'HEALTHY', color: 'text-[var(--cyan)] border-[var(--cyan)]/20 bg-[var(--cyan)]/5' }
		return { label: 'VOLATILE', color: 'text-[var(--amber)] border-[var(--amber)]/20 bg-[var(--amber)]/5' }
	}

	const sharpeStatus = getSharpeStatus(data.sharpe_ratio)
	const symbolStatsList = Object.values(data.symbol_stats || {})
		.filter(Boolean)
		.sort((a, b) => b.total_pn_l - a.total_pn_l)

	return (
		<div className="p-8 max-w-7xl mx-auto space-y-6">
			{/* Page Header */}
			<div className="flex items-center justify-between">
				<div>
					<div className="font-mono text-[10px] tracking-[3px] text-[var(--text-dim)] mb-1">COGNITIVE METRICS</div>
					<h1 className="text-4xl font-medium tracking-tight">Intelligence</h1>
				</div>
				<div className="flex items-center gap-1.5 px-3 py-1 rounded-sm bg-blue-500/5 border border-blue-500/15 text-[var(--accent)] font-mono text-xs font-bold">
					<span className="pulse-dot-green" style={{ width: 6, height: 6 }}></span>
					SENSOR ONLINE • PF: {data.profit_factor.toFixed(2)}
				</div>
			</div>

			{/* Core Metric Cards */}
			<div className="grid grid-cols-1 md:grid-cols-4 gap-4">
				{/* Sharpe Ratio */}
				<div className="border border-[var(--border)] bg-[var(--bg-panel)] p-5 rounded-sm flex flex-col justify-between h-36">
					<div>
						<span className="text-[10px] font-mono text-[var(--text-dim)] uppercase tracking-wider">Sharpe Ratio</span>
						<div className="text-3xl font-mono tracking-tight mt-1">{data.sharpe_ratio.toFixed(2)}</div>
					</div>
					<div className={`mt-2 py-1 px-2.5 border rounded-sm font-mono text-[10px] tracking-widest font-bold inline-block text-center ${sharpeStatus.color}`}>
						{sharpeStatus.label} PROFILE
					</div>
				</div>

				{/* Profit Factor */}
				<div className="border border-[var(--border)] bg-[var(--bg-panel)] p-5 rounded-sm flex flex-col justify-between h-36">
					<div>
						<span className="text-[10px] font-mono text-[var(--text-dim)] uppercase tracking-wider">Profit Factor</span>
						<div className="text-3xl font-mono tracking-tight mt-1">{data.profit_factor.toFixed(2)}</div>
					</div>
					<div className="text-[10px] font-mono text-[var(--text-muted)] leading-relaxed">
						{data.profit_factor >= 1.5 ? '獲利能力極強，風險回報比優異' : data.profit_factor >= 1.0 ? '表現穩健，總收益大於總虧損' : '處於回撤區，需優化風控參數'}
					</div>
				</div>

				{/* Win Rate */}
				<div className="border border-[var(--border)] bg-[var(--bg-panel)] p-5 rounded-sm flex flex-col justify-between h-36">
					<div>
						<span className="text-[10px] font-mono text-[var(--text-dim)] uppercase tracking-wider">Win Rate</span>
						<div className="text-3xl font-mono tracking-tight mt-1">{(data.win_rate * 100).toFixed(1)}%</div>
					</div>
					<div className="text-[10px] font-mono text-[var(--text-muted)] flex items-center justify-between">
						<span>W: {data.winning_trades} / L: {data.losing_trades}</span>
						<span>TOTAL: {data.total_trades}</span>
					</div>
				</div>

				{/* Best/Worst Performers */}
				<div className="border border-[var(--border)] bg-[var(--bg-panel)] p-5 rounded-sm flex flex-col justify-between h-36 text-xs font-mono">
					<div>
						<span className="text-[10px] font-mono text-[var(--text-dim)] uppercase tracking-wider">Cognitive Assets</span>
						<div className="space-y-1.5 mt-2.5">
							<div className="flex items-center justify-between">
								<span className="text-[var(--text-muted)] flex items-center gap-1"><TrendingUp size={12} className="text-[var(--green)]" /> BEST:</span>
								<span className="font-bold text-gray-200">{data.best_symbol || 'N/A'}</span>
							</div>
							<div className="flex items-center justify-between">
								<span className="text-[var(--text-muted)] flex items-center gap-1"><TrendingDown size={12} className="text-[var(--red)]" /> WORST:</span>
								<span className="font-bold text-gray-200">{data.worst_symbol || 'N/A'}</span>
							</div>
						</div>
					</div>
					<div className="text-[9px] font-mono text-[var(--text-dim)] uppercase tracking-wide">
						Cognitive pool matching active
					</div>
				</div>
			</div>

			{/* Asset Statistics Table */}
			<div className="border border-[var(--border)] bg-[var(--bg-panel)] rounded-sm overflow-hidden">
				<div className="px-6 py-4 border-b border-[var(--border)]">
					<div className="font-mono text-sm tracking-widest text-[var(--text-dim)]">ASSET PERFORMANCE STATS</div>
				</div>
				<div className="overflow-x-auto">
					<table className="w-full text-xs font-mono">
						<thead>
							<tr className="border-b border-[var(--border)] text-[10px] text-[var(--text-dim)] text-left uppercase bg-[var(--bg-elev)]">
								<th className="px-6 py-3 font-semibold">Symbol</th>
								<th className="px-6 py-3 font-semibold text-center">Trades</th>
								<th className="px-6 py-3 font-semibold text-center">Wins</th>
								<th className="px-6 py-3 font-semibold text-center">Losses</th>
								<th className="px-6 py-3 font-semibold text-center">Win Rate</th>
								<th className="px-6 py-3 font-semibold text-right">Avg Return</th>
								<th className="px-6 py-3 font-semibold text-right">Net Return</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-[var(--border-subtle)]">
							{symbolStatsList.map((stat, i) => (
								<tr key={i} className="hover:bg-[var(--bg-subtle)]/50 transition-colors">
									<td className="px-6 py-3.5 font-bold text-gray-200">{stat.symbol}</td>
									<td className="px-6 py-3.5 text-center text-[var(--text-muted)]">{stat.total_trades}</td>
									<td className="px-6 py-3.5 text-center text-[var(--green)]">{stat.winning_trades}</td>
									<td className="px-6 py-3.5 text-center text-[var(--red)]">{stat.losing_trades}</td>
									<td className="px-6 py-3.5 text-center font-bold text-gray-300">{(stat.win_rate * 100).toFixed(1)}%</td>
									<td className={`px-6 py-3.5 text-right font-semibold ${stat.avg_pn_l >= 0 ? 'text-[var(--green)]' : 'text-[var(--red)]'}`}>
										{stat.avg_pn_l >= 0 ? '+' : ''}{stat.avg_pn_l.toFixed(2)} USDT
									</td>
									<td className={`px-6 py-3.5 text-right font-bold ${stat.total_pn_l >= 0 ? 'text-[var(--green)]' : 'text-[var(--red)]'}`}>
										{stat.total_pn_l >= 0 ? '+' : ''}{stat.total_pn_l.toFixed(2)} USDT
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			</div>

			{/* Recent Trades Table */}
			<div className="border border-[var(--border)] bg-[var(--bg-panel)] rounded-sm overflow-hidden">
				<div className="px-6 py-4 border-b border-[var(--border)]">
					<div className="font-mono text-sm tracking-widest text-[var(--text-dim)]">CLOSED POSITION TELEMETRY LOG</div>
				</div>
				{data.recent_trades && data.recent_trades.length > 0 ? (
					<div className="overflow-x-auto">
						<table className="w-full text-xs font-mono">
							<thead>
								<tr className="border-b border-[var(--border)] text-[10px] text-[var(--text-dim)] text-left uppercase bg-[var(--bg-elev)]">
									<th className="px-6 py-3 font-semibold">Symbol</th>
									<th className="px-6 py-3 font-semibold">Side</th>
									<th className="px-6 py-3 font-semibold text-center">Leverage</th>
									<th className="px-6 py-3 font-semibold text-right">Entry</th>
									<th className="px-6 py-3 font-semibold text-right">Exit</th>
									<th className="px-6 py-3 font-semibold text-center">Duration</th>
									<th className="px-6 py-3 font-semibold text-center">Close Time</th>
									<th className="px-6 py-3 font-semibold text-right">PnL</th>
								</tr>
							</thead>
							<tbody className="divide-y divide-[var(--border-subtle)]">
								{data.recent_trades.map((trade, i) => {
									const isLong = trade.side === 'long'
									const isWin = trade.pn_l >= 0
									return (
										<tr key={i} className="hover:bg-[var(--bg-subtle)]/50 transition-colors">
											<td className="px-6 py-3.5 font-bold text-gray-200">{trade.symbol}</td>
											<td className="px-6 py-3.5">
												<span className={`inline-block px-1.5 py-0.5 rounded-[2px] text-[10px] font-bold uppercase tracking-wide
													${isLong 
														? 'bg-[var(--green)]/10 text-[var(--green)] border border-[var(--green)]/15' 
														: 'bg-[var(--red)]/10 text-[var(--red)] border border-[var(--red)]/15'}`}>
													{isLong ? 'LONG' : 'SHORT'}
												</span>
											</td>
											<td className="px-6 py-3.5 text-center text-gray-300 font-bold">{trade.leverage}x</td>
											<td className="px-6 py-3.5 text-right text-[var(--text-muted)]">{trade.open_price.toFixed(4)}</td>
											<td className="px-6 py-3.5 text-right text-gray-300">{trade.close_price.toFixed(4)}</td>
											<td className="px-6 py-3.5 text-center text-[var(--text-muted)] flex items-center justify-center gap-1">
												<Clock size={11} /> {trade.duration || 'N/A'}
											</td>
											<td className="px-6 py-3.5 text-center text-[var(--text-muted)]">
												{new Date(trade.close_time).toLocaleDateString()} {new Date(trade.close_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
											</td>
											<td className="px-6 py-3.5 text-right font-bold">
												<div className={isWin ? 'text-[var(--green)]' : 'text-[var(--red)]'}>
													{isWin ? '+' : ''}{trade.pn_l.toFixed(2)} USDT
												</div>
												<div className={`text-[10px] ${isWin ? 'text-[var(--green)]/60' : 'text-[var(--red)]/60'}`}>
													{isWin ? '+' : ''}{trade.pn_l_pct.toFixed(2)}%
												</div>
											</td>
										</tr>
									)
								})}
							</tbody>
						</table>
					</div>
				) : (
					<div className="p-8 text-center text-[var(--text-muted)] text-xs font-mono">No closed trades recorded in the current telemetry window</div>
				)}
			</div>
		</div>
	)
}
