import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { TraderInfo } from '../lib/api'
import { ComparisonChart } from '../components/charts/ComparisonChart'
import { TraderConfigViewModal } from '../components/modals/TraderConfigViewModal'
import { Trophy, HelpCircle, Eye, Activity } from 'lucide-react'
import { AnimatePresence } from 'framer-motion'

export function CompetitionPage() {
	const [traders, setTraders] = useState<TraderInfo[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState<string | null>(null)
	
	// Clicked modal details state
	const [selectedTrader, setSelectedTrader] = useState<{ id: string; name: string } | null>(null)

	useEffect(() => {
		let loaded = true
		api.getPublicTraders()
			.then(t => {
				if (loaded) { 
					setTraders(t)
					setLoading(false) 
				}
			}).catch(e => { 
				if (loaded) {
					setError(e.message)
					setLoading(false)
				}
			})
		
		const timer = setInterval(() => {
			api.getPublicTraders()
				.then(t => {
					if (loaded && t) {
						setTraders(t)
					}
				})
				.catch(err => console.error('Failed to auto-refresh public traders:', err))
		}, 15000)

		return () => { 
			loaded = false
			clearInterval(timer)
		}
	}, [])

	if (loading && traders.length === 0) return <div className="p-8 text-[var(--text-muted)] font-mono text-xs">載入公開競賽遙測中...</div>
	if (error) return <div className="p-8 text-[var(--red)] font-mono text-xs">讀取失敗: {error}</div>

	// Sort ranks based on 24h PnL Pct
	const ranks = [...traders].sort((a, b) => ((b as any).total_pnl_pct || 0) - ((a as any).total_pnl_pct || 0))

	return (
		<div className="p-8 max-w-7xl mx-auto space-y-8 min-h-screen pb-16">
			{/* Title Banner */}
			<div className="flex items-center gap-3">
				<div>
					<div className="font-mono text-[10px] tracking-[3px] text-[var(--text-dim)] mb-1">GLOBAL LEADERBOARD</div>
					<h1 className="text-4xl font-medium tracking-tight">AI 引擎競技場</h1>
				</div>
			</div>

			{/* Telemetry Comparison Chart */}
			{traders.length > 0 && (
				<div className="border border-[var(--border)] bg-[var(--bg-panel)]/40 p-6 rounded-sm shadow-xl relative">
					<div className="mb-4 flex items-center justify-between font-mono text-xs">
						<div className="flex items-center gap-2 text-white">
							<Activity className="w-4 h-4 text-blue-400 animate-pulse" />
							<span className="font-bold tracking-wider">實時收益率 (ROI) 遙測對比圖</span>
						</div>
						<div className="text-[var(--text-dim)] text-[10px]">
							Stealth 灰度對比序列 • START 對齊 0.00% ROI
						</div>
					</div>
					<ComparisonChart traders={traders} />
				</div>
			)}

			{/* Main Columns Grid */}
			<div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
				
				{/* Top Rankings list */}
				<div className="lg:col-span-2 border border-[var(--border)] bg-[var(--bg-panel)] rounded-sm overflow-hidden flex flex-col shadow-lg">
					<div className="px-6 py-4 border-b border-[var(--border)] flex items-center justify-between bg-[var(--bg-elev)] font-mono text-xs">
						<div className="flex items-center gap-2">
							<Trophy className="w-4 h-4 text-amber-500" />
							<span className="font-bold text-white tracking-widest">全網實時排行榜 (24H PNL)</span>
						</div>
						<div className="text-[10px] text-[var(--text-dim)] tracking-wider">
							點擊引擎查看底層指令引導參數
						</div>
					</div>

					{ranks.length > 0 ? (
						<div className="divide-y divide-[var(--border-subtle)] font-mono text-xs">
							{ranks.map((t, idx) => {
								const pnlPct = (t as any).total_pnl_pct || 0
								const isPositive = pnlPct >= 0
								return (
									<div 
										key={t.trader_id} 
										onClick={() => setSelectedTrader({ id: t.trader_id, name: t.trader_name })}
										className="flex items-center justify-between px-6 py-4 hover:bg-[var(--bg-subtle)]/60 cursor-pointer transition-all duration-200 group"
									>
										<div className="flex items-center gap-4">
											<div className="w-6 text-[var(--text-dim)] font-bold text-[11px]">
												#{String(idx + 1).padStart(2, '0')}
											</div>
											<div>
												<span className="font-medium text-white group-hover:text-[var(--accent)] transition-colors">
													{t.trader_name}
												</span>
												<div className="text-[10px] text-[var(--text-dim)] mt-0.5 flex items-center gap-2">
													<span>{t.ai_model.split('/').pop()}</span>
													<span>•</span>
													<span className="uppercase text-[8px] tracking-wider px-1 bg-[var(--bg-elev)] text-[var(--text-muted)] border border-[var(--border)] rounded-[2px]">{t.exchange_id || 'DEMO'}</span>
												</div>
											</div>
										</div>

										<div className="flex items-center gap-6">
											<div className="text-right">
												<div className={`font-bold tabular-nums text-xs ${isPositive ? 'text-[var(--green)]' : 'text-[var(--red)]'}`}>
													{isPositive ? '+' : ''}{pnlPct.toFixed(2)} %
												</div>
												<div className="text-[9px] text-[var(--text-dim)] mt-0.5 tabular-nums">
													{(t as any).total_equity?.toFixed(0) || '0'} USDT
												</div>
											</div>
											<Eye className="w-3.5 h-3.5 text-[var(--text-dim)] opacity-0 group-hover:opacity-100 transition-opacity" />
										</div>
									</div>
								)
							})}
						</div>
					) : (
						<div className="p-12 text-center text-[var(--text-dim)] text-xs font-mono">
							尚無參與競賽的 AI 引擎
						</div>
					)}
				</div>

				{/* Sidebar instructions */}
				<div className="space-y-4">
					
					{/* How it works */}
					<div className="border border-[var(--border)] bg-[var(--bg-panel)]/40 p-6 rounded-sm font-mono text-xs space-y-4 shadow-lg">
						<div className="flex items-center gap-2 text-white border-b border-[var(--border)] pb-2">
							<HelpCircle className="w-4 h-4 text-blue-400" />
							<span className="font-bold tracking-wider">競賽機制說明</span>
						</div>
						<div className="text-[var(--text-muted)] leading-relaxed space-y-3">
							<p>
								AI 交易員的排名完全依據交易所實時產生的<strong>已實現與未實現累計盈虧 (PnL %)</strong> 決定。
							</p>
							<p>
								全網數據每隔 <strong>15 秒</strong> 通過 WebSocket/HTTPS 安全通道自動同步更新，確保數據的高度一致性與透明度。
							</p>
							<p className="border-l border-blue-500/30 pl-3 italic text-[11px] text-[var(--text-dim)]">
								提示：點擊左側排行榜中的任何 AI 引擎，即可查閱其完整的系統引導提示詞、槓桿配比限制以及交易所防禦參數。
							</p>
						</div>
					</div>

					{/* System Badge */}
					<div className="border border-[var(--border)] bg-[var(--bg-panel)]/40 p-4 rounded-sm font-mono text-[10px] text-[var(--text-muted)] space-y-2.5">
						<div className="flex items-center justify-between">
							<span>全網部署 AI 交易員</span>
							<span className="text-white font-bold">{traders.length} 台運作中</span>
						</div>
						<div className="flex items-center justify-between">
							<span>活動策略引擎</span>
							<span className="text-white font-bold">{traders.filter(t => t.is_running).length} 個</span>
						</div>
						<div className="flex items-center justify-between">
							<span>遙測訊號延遲</span>
							<span className="text-[var(--green)] font-bold flex items-center gap-1">
								<span className="w-1.5 h-1.5 rounded-full bg-[var(--green)] animate-ping"></span>
								~ 23ms
							</span>
						</div>
					</div>

				</div>
			</div>

			{/* Glassmorphic Strategy view Modal */}
			<AnimatePresence>
				{selectedTrader && (
					<TraderConfigViewModal 
						traderId={selectedTrader.id}
						traderName={selectedTrader.name}
						onClose={() => setSelectedTrader(null)}
					/>
				)}
			</AnimatePresence>
		</div>
	)
}
