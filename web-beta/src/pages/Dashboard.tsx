import { useEffect, useState, useCallback } from 'react'
import { api } from '../lib/api'
import type { AccountInfo, Position, DecisionRecord, TraderInfo } from '../lib/api'
import { Brain, X, CheckCircle2, AlertTriangle, Code, Terminal, ListTodo } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'

interface DashboardPageProps {
	selectedTraderId?: string
}

export function DashboardPage({ selectedTraderId: propsSelectedTraderId }: DashboardPageProps) {
	const [traders, setTraders] = useState<TraderInfo[]>([])
	const [selectedTraderId, setSelectedTraderId] = useState<string>('')
	const [account, setAccount] = useState<AccountInfo | null>(null)
	const [positions, setPositions] = useState<Position[]>([])
	const [decisions, setDecisions] = useState<DecisionRecord[]>([])
	const [equityData, setEquityData] = useState<any[]>([])
	const [status, setStatus] = useState<any>(null)
	const [loading, setLoading] = useState(true)
	const [selectedDecision, setSelectedDecision] = useState<DecisionRecord | null>(null)

	// Sync local selected trader state with props from App / SideNav
	useEffect(() => {
		if (propsSelectedTraderId) {
			setSelectedTraderId(propsSelectedTraderId)
		}
	}, [propsSelectedTraderId])

	const fetchData = useCallback(async () => {
		if (!selectedTraderId) return
		try {
			setLoading(true)
			const [acct, pos, dec, eq, st] = await Promise.all([
				api.getAccount(selectedTraderId),
				api.getPositions(selectedTraderId),
				api.getLatestDecisions(selectedTraderId),
				api.getEquityHistory(selectedTraderId),
				api.getStatus(selectedTraderId).catch(() => null),
			])
			setAccount(acct)
			setPositions(pos)
			setDecisions(dec)
			setEquityData(eq)
			setStatus(st)
		} catch (e) {
			console.error('Failed to load dashboard data:', e)
		} finally {
			setLoading(false)
		}
	}, [selectedTraderId])

	useEffect(() => {
		let mounted = true
		api.getTraders().then(t => {
			if (mounted && t.length > 0) {
				setTraders(t)
				// If no prop is set, choose the first one
				if (!propsSelectedTraderId) {
					setSelectedTraderId(t[0].trader_id)
				}
			}
		}).catch(console.error)
		return () => { mounted = false }
	}, [propsSelectedTraderId])

	useEffect(() => {
		fetchData()
		const timer = setInterval(fetchData, 15000)
		return () => clearInterval(timer)
	}, [fetchData])

	return (
		<div className="p-8 max-w-7xl mx-auto relative">
			<div className="mb-8 flex items-start justify-between">
				<div>
					<div className="font-mono text-[10px] tracking-[3px] text-[var(--text-dim)] mb-1">OVERVIEW</div>
					<h1 className="text-4xl font-medium tracking-tight">Dashboard</h1>
					{status?.risk_halted && (
						<div className="mt-3 text-sm text-red-400">
							风控暂停新开仓至 {status.stop_until} · 连续 wait {status.consecutive_wait ?? 0}
						</div>
					)}
					{(status?.consecutive_wait || 0) >= 5 && !status?.risk_halted && (
						<div className="mt-3 text-sm text-blue-400">连续观望 {status.consecutive_wait} 周期</div>
					)}
				</div>
				{traders.length > 1 && (
					<select
						value={selectedTraderId}
						onChange={e => setSelectedTraderId(e.target.value)}
						className="px-3 py-2 text-sm font-mono border rounded-sm bg-[var(--bg-panel)] text-[var(--text)] border-[var(--border)] cursor-pointer"
					>
						{traders.map(t => (
							<option key={t.trader_id} value={t.trader_id}>{t.trader_name}</option>
						))}
					</select>
				)}
			</div>

			<div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
				<KPICard label="TOTAL EQUITY" value={`${account?.total_equity?.toFixed(2) || '0.00'} USDT`} sub={`Initial: ${account?.initial_balance?.toFixed(2) || '0.00'}`} />
				<KPICard label="AVAILABLE BALANCE" value={`${account?.available_balance?.toFixed(2) || '0.00'} USDT`} sub={`${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% FREE`} />
				<KPICard label="TOTAL PNL" value={`${account?.total_pnl !== undefined && account.total_pnl >= 0 ? '+' : ''}${account?.total_pnl?.toFixed(2) || '0.00'} USDT`} sub={account?.total_pnl !== undefined ? `${account.total_pnl >= 0 ? '+' : '-'}${Math.abs(account.total_pnl_pct).toFixed(2)}%` : ''} positive={account?.total_pnl ? account.total_pnl >= 0 : false} />
				<KPICard label="ACTIVE POSITIONS" value={`${account?.position_count || 0}`} sub={`Margin: ${account?.margin_used_pct?.toFixed(1) || '0.0'}%`} />
			</div>

			<div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
				<div className="lg:col-span-2 border border-[var(--border)] bg-[var(--bg-panel)] p-6 rounded-sm min-h-[320px]">
					<div className="font-mono text-sm mb-6 tracking-widest text-[var(--text-dim)]">EQUITY HISTORY</div>
					{loading && !equityData.length ? (
						<div className="flex items-center justify-center h-[220px]"><div className="text-[var(--text-muted)] text-xs font-mono">Loading...</div></div>
					) : equityData && equityData.length > 0 ? (
						<SwgEquityChart data={equityData} />
					) : (
						<div className="flex items-center justify-center h-[220px]"><div className="text-center text-[var(--text-muted)] text-xs font-mono">No equity history</div></div>
					)}
				</div>

				<div className="border border-[var(--border)] bg-[var(--bg-panel)] p-6 rounded-sm min-h-[320px]">
					<div className="font-mono text-sm mb-4 tracking-widest text-[var(--text-dim)]">RECENT DECISIONS</div>
					{loading && !decisions.length ? (
						<div className="flex items-center justify-center h-[220px]"><div className="text-[var(--text-muted)] text-xs font-mono">Loading...</div></div>
					) : decisions && decisions.length > 0 ? (
						<div className="space-y-3 text-xs font-mono max-h-[280px] overflow-y-auto pr-1">
							{decisions.map((d, i) => (
								<div 
									key={i} 
									onClick={() => setSelectedDecision(d)}
									className="cursor-pointer hover:bg-[var(--bg-subtle)] p-2 rounded-sm border border-transparent hover:border-[var(--border)] transition-all"
								>
									<DecisionRow decision={d} />
								</div>
							))}
						</div>
					) : (
						<div className="flex items-center justify-center h-[220px]"><div className="text-center text-[var(--text-muted)] text-xs font-mono">No decisions yet</div></div>
					)}
				</div>
			</div>

			{positions.length > 0 && (
				<div className="mt-6 border border-[var(--border)] bg-[var(--bg-panel)] p-6 rounded-sm">
					<div className="font-mono text-sm mb-4 tracking-widest text-[var(--text-dim)]">POSITIONS</div>
					<div className="overflow-x-auto">
						<table className="w-full text-xs font-mono">
							<thead>
								<tr className="border-b border-[var(--border)] text-left text-[var(--text-dim)]">
									<th className="pb-3 pr-4 font-normal">SYMBOL</th>
									<th className="pb-3 pr-4 font-normal">SIDE</th>
									<th className="pb-3 pr-4 font-normal">LEVERAGE</th>
									<th className="pb-3 pr-4 font-normal">ENTRY</th>
									<th className="pb-3 pr-4 font-normal">MARK</th>
									<th className="pb-3 pr-4 font-normal text-right">PNL</th>
									<th className="pb-3 font-normal text-right">LIQ PRICE</th>
								</tr>
							</thead>
							<tbody className="divide-y divide-[var(--border)]">
								{positions.map((p, i) => (
									<tr key={i} className="hover:bg-[var(--bg-subtle)]">
										<td className="py-3 pr-4 font-bold">{p.symbol}</td>
										<td className="py-3 pr-4">
											<span className={`px-2 py-0.5 rounded text-[10px] font-bold ${p.side === 'long' ? 'bg-[var(--green)]/10 text-[var(--green)]' : 'bg-[var(--red)]/10 text-[var(--red)]'}`}>
												{p.side.toUpperCase()}
											</span>
										</td>
										<td className="py-3 pr-4">{p.leverage}x</td>
										<td className="py-3 pr-4 text-[var(--text-muted)]">{p.entry_price.toFixed(4)}</td>
										<td className="py-3 pr-4 text-[var(--text-muted)]">{p.mark_price.toFixed(4)}</td>
										<td className={`py-3 pr-4 text-right tabular-nums ${(p.unrealized_pnl >= 0 ? 'text-[var(--green)]' : 'text-[var(--red)]')}`}>
											{p.unrealized_pnl >= 0 ? '+' : ''}{p.unrealized_pnl.toFixed(2)} ({p.unrealized_pnl_pct.toFixed(2)}%)
										</td>
										<td className="py-3 text-right text-[var(--text-dim)]">{p.liquidation_price.toFixed(4)}</td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				</div>
			)}

			{/* Sliding Flyout Decision Details Drawer */}
			<AnimatePresence>
				{selectedDecision && (
					<>
						{/* Semi-transparent Backdrop overlay */}
						<motion.div 
							initial={{ opacity: 0 }}
							animate={{ opacity: 1 }}
							exit={{ opacity: 0 }}
							onClick={() => setSelectedDecision(null)}
							className="fixed inset-0 z-40 bg-black/60 backdrop-blur-xs"
						/>

						{/* Right hand panel drawer */}
						<motion.div
							initial={{ x: '100%' }}
							animate={{ x: 0 }}
							exit={{ x: '100%' }}
							transition={{ type: 'spring', damping: 25, stiffness: 220 }}
							className="fixed top-0 right-0 z-50 w-full max-w-2xl h-screen bg-[#111318]/95 backdrop-blur-2xl border-l border-[var(--border)] shadow-2xl flex flex-col overflow-hidden"
						>
							{/* Drawer Header */}
							<div className="p-6 border-b border-[var(--border)] flex items-center justify-between">
								<div className="flex items-center gap-3">
									<div className="w-10 h-10 rounded-sm bg-blue-500/5 border border-blue-500/15 flex items-center justify-center text-blue-400">
										<Brain size={18} className="animate-pulse" />
									</div>
									<div>
										<div className="text-[10px] font-mono tracking-widest text-[var(--text-dim)] uppercase">DECISION CORE TELEMETRY</div>
										<h2 className="text-lg font-medium tracking-tight">CYCLE #{selectedDecision.cycle_number}</h2>
									</div>
								</div>
								<button 
									onClick={() => setSelectedDecision(null)}
									className="p-2 border border-[var(--border)] hover:bg-[var(--bg-subtle)] text-[var(--text-muted)] hover:text-white rounded-sm transition-colors cursor-pointer"
								>
									<X size={16} />
								</button>
							</div>

							{/* Drawer Content */}
							<div className="flex-1 overflow-y-auto p-6 space-y-6 font-mono text-xs">
								{/* Timestamp & Success */}
								<div className="grid grid-cols-2 gap-4 p-4 border border-[var(--border)] bg-[#161a21]/50 rounded-sm">
									<div>
										<div className="text-[10px] text-[var(--text-dim)] uppercase mb-1">TELEMETRY TIMESTAMP</div>
										<div className="text-gray-300">{new Date(selectedDecision.timestamp).toLocaleString()}</div>
									</div>
									<div>
										<div className="text-[10px] text-[var(--text-dim)] uppercase mb-1">EXECUTION STATUS</div>
										<div className="flex items-center gap-1.5 font-bold">
											{selectedDecision.success ? (
												<>
													<CheckCircle2 size={14} className="text-[var(--green)]" />
													<span className="text-[var(--green)]">SUCCESS // EXECUTION OK</span>
												</>
											) : (
												<>
													<AlertTriangle size={14} className="text-[var(--red)]" />
													<span className="text-[var(--red)]">FAILED // {selectedDecision.error_message || 'ERROR'}</span>
												</>
											)}
										</div>
									</div>
								</div>

								{/* CoT Thinking Chain */}
								<div className="space-y-2">
									<div className="flex items-center gap-2 text-[var(--text-dim)] uppercase tracking-wider font-bold">
										<Terminal size={14} className="text-blue-400" />
										<span>AI Thinking Trace (CoT)</span>
									</div>
									<div className="p-4 border border-[var(--border)] bg-[#0a0b0f] rounded-sm max-h-72 overflow-y-auto">
										<pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-blue-300/80">
											{selectedDecision.cot_trace || 'No cognitive trace recorded for this cycle.'}
										</pre>
									</div>
								</div>

								{/* Candidate assets */}
								{selectedDecision.candidate_coins && selectedDecision.candidate_coins.length > 0 && (
									<div className="space-y-2">
										<div className="flex items-center gap-2 text-[var(--text-dim)] uppercase tracking-wider font-bold">
											<ListTodo size={14} className="text-teal-400" />
											<span>Candidate Coin Ratings</span>
										</div>
										<div className="p-4 border border-[var(--border)] bg-[#161a21]/30 rounded-sm flex flex-wrap gap-2">
											{selectedDecision.candidate_coins.map((coin, index) => (
												<span 
													key={index}
													className="px-2.5 py-1 bg-[#1c212a] border border-[var(--border)] text-gray-300 font-bold rounded-sm"
												>
													{coin}
												</span>
											))}
										</div>
									</div>
								)}

								{/* Actions Executed */}
								<div className="space-y-2">
									<div className="flex items-center gap-2 text-[var(--text-dim)] uppercase tracking-wider font-bold">
										<CheckCircle2 size={14} className="text-green-400" />
										<span>Executed Trade Decisions</span>
									</div>
									<div className="border border-[var(--border)] rounded-sm overflow-hidden bg-[#161a21]/20">
										{selectedDecision.decisions && selectedDecision.decisions.length > 0 ? (
											<table className="w-full text-left">
												<thead>
													<tr className="border-b border-[var(--border)] text-[9px] text-[var(--text-dim)] bg-[var(--bg-elev)] uppercase">
														<th className="px-4 py-2 font-semibold">Action</th>
														<th className="px-4 py-2 font-semibold">Symbol</th>
														<th className="px-4 py-2 font-semibold text-center">Leverage</th>
														<th className="px-4 py-2 font-semibold text-right">Value</th>
														<th className="px-4 py-2 font-semibold text-right">Price</th>
														<th className="px-4 py-2 font-semibold text-center">Status</th>
													</tr>
												</thead>
												<tbody className="divide-y divide-[var(--border-subtle)] text-[11px]">
													{selectedDecision.decisions.map((act, idx) => (
														<tr key={idx} className="hover:bg-[var(--bg-subtle)]/50">
															<td className="px-4 py-2.5 font-bold">
																<span className={act.action.includes('open') ? 'text-[var(--green)]' : 'text-gray-400'}>
																	{act.action.toUpperCase()}
																</span>
															</td>
															<td className="px-4 py-2.5 text-gray-200">{act.symbol}</td>
															<td className="px-4 py-2.5 text-center">{act.leverage}x</td>
															<td className="px-4 py-2.5 text-right">{(act.quantity * act.price).toFixed(2)} USDT</td>
															<td className="px-4 py-2.5 text-right text-[var(--text-muted)]">{act.price.toFixed(4)}</td>
															<td className="px-4 py-2.5 text-center">
																<span className={act.success ? 'text-[var(--green)]' : 'text-[var(--red)] font-bold'}>
																	{act.success ? 'OK' : 'ERR'}
																</span>
															</td>
														</tr>
													))}
												</tbody>
											</table>
										) : (
											<div className="p-4 text-center text-[var(--text-dim)]">WAIT // NO TRADING SIGNAL DETECTED</div>
										)}
									</div>
								</div>

								{/* Technical analysis details */}
								{selectedDecision.execution_log && selectedDecision.execution_log.length > 0 && (
									<div className="space-y-2">
										<div className="flex items-center gap-2 text-[var(--text-dim)] uppercase tracking-wider font-bold">
											<Terminal size={14} className="text-gray-400" />
											<span>Execution Telemetry Logs</span>
										</div>
										<div className="p-4 border border-[var(--border)] bg-[#0a0b0f] rounded-sm max-h-48 overflow-y-auto">
											<ul className="space-y-1 text-[11px] leading-relaxed text-gray-400">
												{selectedDecision.execution_log.map((log, index) => (
													<li key={index} className="flex gap-2">
														<span className="text-[var(--text-dim)]">[{index+1}]</span>
														<span className="break-all">{log}</span>
													</li>
												))}
											</ul>
										</div>
									</div>
								)}

								{/* Raw JSON */}
								<div className="space-y-2">
									<div className="flex items-center gap-2 text-[var(--text-dim)] uppercase tracking-wider font-bold">
										<Code size={14} className="text-amber-400" />
										<span>Raw Decision JSON Payload</span>
									</div>
									<div className="p-4 border border-[var(--border)] bg-[#0a0b0f] rounded-sm max-h-48 overflow-y-auto">
										<pre className="text-amber-400/80 text-[10px] leading-normal font-mono">
											{(() => {
												try {
													return JSON.stringify(JSON.parse(selectedDecision.decision_json), null, 2)
												} catch {
													return selectedDecision.decision_json || '{}'
												}
											})()}
										</pre>
									</div>
								</div>
							</div>
						</motion.div>
					</>
				)}
			</AnimatePresence>
		</div>
	)
}

function KPICard({ label, value, sub, positive }: { label: string; value: string; sub: string; positive?: boolean }) {
	const textColor = positive === undefined ? '' : positive ? 'text-[var(--green)]' : 'text-[var(--red)]'
	return (
		<div className="border border-[var(--border)] bg-[var(--bg-panel)] p-5 rounded-sm">
			<div className="text-[10px] font-mono tracking-widest text-[var(--text-dim)] mb-2">{label}</div>
			<div className={`font-mono text-2xl tabular-nums tracking-[-1px] ${textColor}`}>{value}</div>
			{sub && <div className="text-xs text-[var(--text-muted)] mt-1">{sub}</div>}
		</div>
	)
}

function SwgEquityChart({ data }: { data: any[] }) {
	if (!data || data.length < 2) return <div className="text-[var(--text-dim)] text-xs font-mono">No chart data</div>
	const equities: number[] = data.map((d: any) => d.total_equity)
	const W = 600, H = 200, padX = 40, padY = 30
	const minE = Math.min(...equities), maxE = Math.max(...equities)
	const range = maxE - minE || 1
	const xStep = (W - padX * 2) / (data.length - 1)
	const toX = (i: number) => padX + i * xStep
	const toY = (e: number) => H - padY - ((e - minE) / range) * (H - padY * 2)
	const pathD = data.map((_, i) => `${i === 0 ? 'M' : 'L'} ${toX(i)} ${toY(equities[i])}`).join(' ')
	const areaD = pathD + ` L ${toX(data.length - 1)} ${H - padY} L ${toX(0)} ${H - padY} Z`
	const color = equities[equities.length - 1] >= equities[0] ? '#22c55e' : '#ef4444'
	return (
		<svg viewBox={`0 0 ${W} ${H}`} className="w-full h-64">
			{[0, 0.25, 0.5, 0.75, 1].map(frac => {
				const y = H - padY - frac * (H - padY * 2)
				const val = minE + frac * range
				return (
					<g key={frac}>
						<line x1={padX} y1={y} x2={W - padX} y2={y} stroke="var(--border-subtle)" strokeDasharray="4 4" />
						<text x={0} y={y + 4} fontSize="10" fill="var(--text-dim)" fontFamily="monospace">${val.toFixed(0)}</text>
					</g>
				)
			})}
			<path d={areaD} fill={color} opacity="0.1" />
			<path d={pathD} fill="none" stroke={color} strokeWidth="2" />
			<circle cx={toX(data.length - 1)} cy={toY(equities[equities.length - 1])} r="4" fill={color} />
		</svg>
	)
}

function DecisionRow({ decision }: { decision: DecisionRecord }) {
	const timeStr = new Date(decision.timestamp).toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
	const actions = decision.decisions || []
	return (
		<div className="border-l-2 border-[var(--border-subtle)] pl-3 py-1">
			<div className="flex items-center justify-between text-[var(--text-dim)] mb-1">
				<span>CYCLE {decision.cycle_number}</span>
				<span>{timeStr}</span>
			</div>
			{actions.length > 0 ? (
				<div className="space-y-1">
					{actions.slice(0, 2).map((a, j) => (
						<div key={j} className="flex items-center justify-between">
							<div className="flex items-center gap-2">
								<span className={`font-bold ${a.action.includes('open') ? 'text-[var(--green)]' : 'text-[var(--text-muted)]'}`}>{a.action.toUpperCase()}</span>
								<span className="text-[var(--text)]">{a.symbol}</span>
								{a.leverage > 0 && <span className="text-[var(--text-dim)]">{a.leverage}x</span>}
							</div>
							<span className={a.success ? 'text-[var(--green)]' : 'text-[var(--red)]'}>{a.success ? 'OK' : 'ERR'}</span>
						</div>
					))}
				</div>
			) : (
				<div className="text-[var(--text-muted)] font-bold tracking-wider">WAIT</div>
			)}
		</div>
	)
}
