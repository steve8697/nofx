import { useEffect, useState, useCallback } from 'react'
import { api } from '../lib/api'
import type { TraderInfo, AIModel, Exchange, CreateTraderRequest } from '../lib/api'
import { Play, Square, Trash2, Settings2, Loader2, X, AlertTriangle } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import { toast } from 'sonner'

function formatPnl(pnl?: number, pnlPct?: number) {
	if (pnl === undefined) return '-'
	const sign = pnl >= 0 ? '+' : ''
	return `${sign}${pnl.toFixed(2)} (${sign}${pnlPct?.toFixed(2) || 0}%)`
}

export function TradersPage() {
	const [traders, setTraders] = useState<TraderInfo[]>([])
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState<string | null>(null)
	const [actionLoading, setActionLoading] = useState<Record<string, boolean>>({})

	// Modal States
	const [isModalOpen, setIsModalOpen] = useState(false)
	const [modalMode, setModalMode] = useState<'create' | 'edit'>('create')
	const [editingTraderId, setEditingTraderId] = useState<string>('')
	const [supportedModels, setSupportedModels] = useState<AIModel[]>([])
	const [supportedExchanges, setSupportedExchanges] = useState<Exchange[]>([])
	const [modalLoading, setModalLoading] = useState(false)

	// Delete Confirmation States
	const [deletingTrader, setDeletingTrader] = useState<TraderInfo | null>(null)

	// Form States
	const [promptTemplates, setPromptTemplates] = useState<{ name: string }[]>([])
	const [formData, setFormData] = useState<CreateTraderRequest>({
		name: '',
		ai_model_id: '',
		exchange_id: '',
		initial_balance: 1000,
		scan_interval_minutes: 3,
		btc_eth_leverage: 3,
		altcoin_leverage: 2,
		trading_symbols: 'BTCUSDT,ETHUSDT,SOLUSDT',
		custom_prompt: '',
		override_base_prompt: false,
		system_prompt_template: 'adaptive',
		is_cross_margin: true,
		use_coin_pool: true,
		use_oi_top: true
	})

	const loadTraders = useCallback(() => {
		const token = localStorage.getItem('auth_token')
		const fetchMethod = token ? api.getTraders : api.getPublicTraders

		fetchMethod()
			.then(t => {
				if (Array.isArray(t)) {
					setTraders(t)
				}
				setLoading(false)
			})
			.catch(e => {
				setError(e.message)
				setLoading(false)
			})
	}, [])

	useEffect(() => {
		loadTraders()
		const timer = setInterval(loadTraders, 10000)
		return () => clearInterval(timer)
	}, [loadTraders])

	// Load supported models and exchanges when opening modal
	const openModal = async (mode: 'create' | 'edit', traderId?: string) => {
		setModalMode(mode)
		setIsModalOpen(true)
		setModalLoading(true)

		try {
			const [models, exchanges, templates] = await Promise.all([
				api.getSupportedModels().catch(() => []),
				api.getSupportedExchanges().catch(() => []),
				fetch('/api/prompt-templates').then(r => r.ok ? r.json() : { templates: [] }).catch(() => ({ templates: [] }))
			])
			setSupportedModels(models.filter(m => m.enabled))
			setSupportedExchanges(exchanges.filter(e => e.enabled))
			const list = Array.isArray(templates) ? templates : templates.templates || []
			setPromptTemplates(list.map((t: any) => typeof t === 'string' ? { name: t } : t))

			if (mode === 'edit' && traderId) {
				setEditingTraderId(traderId)
				const config = await api.getTraderConfig(traderId)
				setFormData({
					name: config.name || config.trader_name || '',
					ai_model_id: config.ai_model_id || config.ai_model || '',
					exchange_id: config.exchange_id || config.exchange || '',
					initial_balance: config.initial_balance || 1000,
					scan_interval_minutes: config.scan_interval_minutes || 3,
					btc_eth_leverage: config.btc_eth_leverage || 3,
					altcoin_leverage: config.altcoin_leverage || 2,
					trading_symbols: config.trading_symbols || '',
					custom_prompt: config.custom_prompt || '',
					override_base_prompt: config.override_base_prompt || false,
					system_prompt_template: config.system_prompt_template || '',
					is_cross_margin: config.is_cross_margin !== false,
					use_coin_pool: config.use_coin_pool !== false,
					use_oi_top: config.use_oi_top !== false
				})
			} else {
				setEditingTraderId('')
				setFormData({
					name: '',
					ai_model_id: models.find(m => m.enabled)?.id || '',
					exchange_id: exchanges.find(e => e.enabled)?.id || '',
					initial_balance: 1000,
					scan_interval_minutes: 3,
					btc_eth_leverage: 3,
					altcoin_leverage: 2,
					trading_symbols: 'BTCUSDT,ETHUSDT,SOLUSDT',
					custom_prompt: '',
					override_base_prompt: false,
					system_prompt_template: '',
					is_cross_margin: true,
					use_coin_pool: true,
					use_oi_top: true
				})
			}
		} catch (err) {
			toast.error('無法載入模組或交易所配置列表')
		} finally {
			setModalLoading(false)
		}
	}

	const handleToggleStatus = async (trader: TraderInfo) => {
		const tid = trader.trader_id
		setActionLoading(prev => ({ ...prev, [tid]: true }))
		try {
			if (trader.is_running) {
				await api.stopTrader(tid)
				toast.success(`交易員 ${trader.trader_name} 已成功停止`)
			} else {
				await api.startTrader(tid)
				toast.success(`交易員 ${trader.trader_name} 已成功啟動`)
			}
			loadTraders()
		} catch (err: any) {
			toast.error(`操作失敗：${err.message || '未知錯誤'}`)
		} finally {
			setActionLoading(prev => ({ ...prev, [tid]: false }))
		}
	}

	const handleDeleteTrader = async () => {
		if (!deletingTrader) return
		const tid = deletingTrader.trader_id
		try {
			await api.deleteTrader(tid)
			toast.success(`交易員 ${deletingTrader.trader_name} 已成功刪除`)
			setDeletingTrader(null)
			loadTraders()
		} catch (err: any) {
			toast.error(`刪除失敗：${err.message || '未知錯誤'}`)
		}
	}

	const handleFormSubmit = async (e: React.FormEvent) => {
		e.preventDefault()
		setModalLoading(true)
		try {
			if (modalMode === 'edit') {
				await api.updateTrader(editingTraderId, formData)
				toast.success('交易員配置已成功更新')
			} else {
				await api.createTrader(formData)
				toast.success('新交易員已成功建立')
			}
			setIsModalOpen(false)
			loadTraders()
		} catch (err: any) {
			toast.error(`儲存失敗：${err.message || '未知錯誤'}`)
		} finally {
			setModalLoading(false)
		}
	}

	if (loading && traders.length === 0) return <div className="p-8 text-[var(--text-muted)] font-mono text-xs">載入交易員模組中...</div>
	if (error) return <div className="p-8 text-[var(--red)] font-mono text-xs">Error: {error}</div>

	return (
		<div className="p-8 max-w-7xl mx-auto relative min-h-screen">
			<div className="mb-8 flex items-end justify-between">
				<div>
					<div className="font-mono text-[10px] tracking-[3px] text-[var(--text-dim)] mb-1">QUANT TRADERS</div>
					<h1 className="text-4xl font-medium tracking-tight">AI 交易員管理</h1>
				</div>
				<button 
					onClick={() => openModal('create')}
					className="px-4 py-2 text-xs font-mono tracking-widest border border-[var(--accent)] text-[var(--accent)] hover:bg-[var(--accent)] hover:text-white rounded-sm transition-colors cursor-pointer"
				>
					+ 建立新交易員
				</button>
			</div>

			<div className="border border-[var(--border)] rounded-sm overflow-hidden bg-[var(--bg-panel)]/40 backdrop-blur-xs">
				<table className="w-full text-sm font-mono">
					<thead className="bg-[var(--bg-elev)] border-b border-[var(--border)]">
						<tr className="text-left text-[var(--text-dim)] text-[11px] tracking-wider">
							<th className="py-3 px-6 font-normal">交易員名稱</th>
							<th className="py-3 px-6 font-normal">AI 模型</th>
							<th className="py-3 px-6 font-normal">交易所</th>
							<th className="py-3 px-6 font-normal">狀態</th>
							<th className="py-3 px-6 font-normal text-right">當前淨值</th>
							<th className="py-3 px-6 font-normal text-right">累計盈虧 (ROI)</th>
							<th className="py-3 px-6 font-normal text-center w-48">系統操作</th>
						</tr>
					</thead>
					<tbody className="divide-y divide-[var(--border)] text-[var(--text)]">
						{traders.length === 0 ? (
							<tr>
								<td colSpan={7} className="py-8 text-center text-xs text-[var(--text-dim)]">
									尚無運行中的交易員，點擊右上角建立。
								</td>
							</tr>
						) : (
							traders.map((t) => (
								<tr key={t.trader_id} className="hover:bg-[var(--bg-subtle)] transition-colors">
									<td className="py-4 px-6 font-medium text-[var(--text)]">{t.trader_name}</td>
									<td className="py-4 px-6 text-[var(--text-muted)]">{t.ai_model}</td>
									<td className="py-4 px-6 text-[var(--text-muted)]">{t.exchange_id || '-'}</td>
									<td className="py-4 px-6">
										<span className={`inline-flex items-center gap-1.5 px-2 py-0.5 text-[10px] font-bold rounded ${t.is_running ? 'bg-[var(--green)]/10 text-[var(--green)]' : 'bg-[var(--text-dim)]/10 text-[var(--text-dim)]'}`}>
											<span className={`w-1.5 h-1.5 rounded-full ${t.is_running ? 'bg-[var(--green)] animate-pulse' : 'bg-[var(--text-dim)]'}`}></span>
											{t.is_running ? 'RUNNING' : 'STOPPED'}
										</span>
									</td>
									<td className="py-4 px-6 text-right tabular-nums">{(t as any).total_equity !== undefined ? `${(t as any).total_equity.toFixed(2)} USDT` : '-'}</td>
									<td className={`py-4 px-6 text-right tabular-nums ${((t as any).total_pnl || 0) >= 0 ? 'text-[var(--green)]' : 'text-[var(--red)]'}`}>
										{formatPnl((t as any).total_pnl, (t as any).total_pnl_pct)}
									</td>
									<td className="py-4 px-6 text-center">
										<div className="flex items-center justify-center gap-2">
											<button
												onClick={() => handleToggleStatus(t)}
												disabled={actionLoading[t.trader_id]}
												className={`p-1.5 rounded-sm border transition-colors cursor-pointer disabled:opacity-50
													${t.is_running 
														? 'border-red-500/20 text-red-400 hover:bg-red-500/10' 
														: 'border-green-500/20 text-green-400 hover:bg-green-500/10'}`}
												title={t.is_running ? '停止交易員' : '啟動交易員'}
											>
												{actionLoading[t.trader_id] ? (
													<Loader2 className="w-3.5 h-3.5 animate-spin" />
												) : t.is_running ? (
													<Square className="w-3.5 h-3.5 fill-red-400/10" />
												) : (
													<Play className="w-3.5 h-3.5 fill-green-400/10" />
												)}
											</button>

											<button
												onClick={() => openModal('edit', t.trader_id)}
												className="p-1.5 rounded-sm border border-blue-500/20 text-blue-400 hover:bg-blue-500/10 transition-colors cursor-pointer"
												title="修改配置"
											>
												<Settings2 className="w-3.5 h-3.5" />
											</button>

											<button
												onClick={() => setDeletingTrader(t)}
												className="p-1.5 rounded-sm border border-red-500/20 text-red-500 hover:bg-red-500/10 transition-colors cursor-pointer"
												title="刪除交易員"
											>
												<Trash2 className="w-3.5 h-3.5" />
											</button>
										</div>
									</td>
								</tr>
							))
						)}
					</tbody>
				</table>
			</div>

			{/* Floating Glassmorphic Form Modal */}
			<AnimatePresence>
				{isModalOpen && (
					<>
						{/* Backdrop */}
						<motion.div
							initial={{ opacity: 0 }}
							animate={{ opacity: 1 }}
							exit={{ opacity: 0 }}
							onClick={() => setIsModalOpen(false)}
							className="fixed inset-0 z-40 bg-black/70 backdrop-blur-xs"
						/>

						{/* Modal Content */}
						<motion.div
							initial={{ scale: 0.95, opacity: 0 }}
							animate={{ scale: 1, opacity: 1 }}
							exit={{ scale: 0.95, opacity: 0 }}
							className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none overflow-y-auto"
						>
							<div className="bg-[#111318]/95 border border-[var(--border)] w-full max-w-2xl rounded-sm shadow-2xl flex flex-col max-h-[90vh] pointer-events-auto overflow-hidden">
								
								{/* Header */}
								<div className="p-6 border-b border-[var(--border)] flex items-center justify-between">
									<div className="flex items-center gap-2.5">
										<div className="w-8 h-8 rounded-sm bg-blue-500/5 border border-blue-500/15 flex items-center justify-center text-blue-400">
											<Settings2 size={16} />
										</div>
										<div>
											<h2 className="text-lg font-medium tracking-tight">
												{modalMode === 'create' ? '建立新 AI 交易員' : '編輯 AI 交易員配置'}
											</h2>
											<p className="text-[10px] font-mono text-[var(--text-dim)] tracking-wider">
												{modalMode === 'create' ? 'INITIALIZE NEW COGNITIVE AGENT' : `AGENT ID: ${editingTraderId}`}
											</p>
										</div>
									</div>
									<button 
										onClick={() => setIsModalOpen(false)}
										className="p-1 rounded-sm text-[var(--text-muted)] hover:text-white hover:bg-[var(--bg-subtle)] transition-colors cursor-pointer"
									>
										<X size={16} />
									</button>
								</div>

								{/* Scrollable Form */}
								{modalLoading ? (
									<div className="flex-1 flex flex-col items-center justify-center py-20">
										<Loader2 className="w-8 h-8 animate-spin text-[var(--accent)] mb-4" />
										<p className="text-xs font-mono text-[var(--text-muted)]">同步安全參數中...</p>
									</div>
								) : (
									<form onSubmit={handleFormSubmit} className="flex-1 overflow-y-auto p-6 space-y-6 text-sm font-mono">
										{/* Row 1: Name and AI Model */}
										<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
											<div>
												<label className="block text-[11px] text-[var(--text-muted)] uppercase mb-2">交易員代號 / 名稱</label>
												<input
													type="text"
													required
													value={formData.name}
													onChange={e => setFormData({ ...formData, name: e.target.value })}
													placeholder="例如: AlphaMax-v1"
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
												/>
											</div>
											<div>
												<label className="block text-[11px] text-[var(--text-muted)] uppercase mb-2">AI 決策引擎</label>
												<select
													required
													value={formData.ai_model_id}
													onChange={e => setFormData({ ...formData, ai_model_id: e.target.value })}
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs cursor-pointer"
												>
													<option value="">選擇決策模型...</option>
													{supportedModels.map(m => (
														<option key={m.id} value={m.id}>{m.name} ({m.provider})</option>
													))}
												</select>
											</div>
										</div>

										{/* Row 2: Exchange and Initial Balance */}
										<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
											<div>
												<label className="block text-[11px] text-[var(--text-muted)] uppercase mb-2">交易所對接帳戶</label>
												<select
													required
													value={formData.exchange_id}
													onChange={e => setFormData({ ...formData, exchange_id: e.target.value })}
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs cursor-pointer"
												>
													<option value="">選擇交易所...</option>
													{supportedExchanges.map(e => (
														<option key={e.id} value={e.id}>{e.name} ({e.type.toUpperCase()})</option>
													))}
												</select>
											</div>
											<div>
												<label className="block text-[11px] text-[var(--text-muted)] uppercase mb-2">初始分配金額 (USDT)</label>
												<input
													type="number"
													required
													min={10}
													value={formData.initial_balance}
													onChange={e => setFormData({ ...formData, initial_balance: Number(e.target.value) })}
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
												/>
											</div>
										</div>

										{/* Row 3: Scan Interval and Trading Symbols */}
										<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
											<div>
												<label className="block text-[11px] text-[var(--text-muted)] uppercase mb-2">市場掃描分析週期 (分鐘)</label>
												<input
													type="number"
													required
													min={1}
													value={formData.scan_interval_minutes}
													onChange={e => setFormData({ ...formData, scan_interval_minutes: Number(e.target.value) })}
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
												/>
											</div>
											<div>
												<label className="block text-[11px] text-[var(--text-muted)] uppercase mb-2">交易幣種限制 (逗號分隔)</label>
												<input
													type="text"
													required
													value={formData.trading_symbols}
													onChange={e => setFormData({ ...formData, trading_symbols: e.target.value })}
													placeholder="BTCUSDT,ETHUSDT"
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
												/>
											</div>
										</div>

										{/* Row 4: Leverages */}
										<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
											<div>
												<label className="block text-[11px] text-[var(--text-muted)] uppercase mb-2">BTC / ETH 最大槓桿</label>
												<input
													type="number"
													required
													min={1}
													max={100}
													value={formData.btc_eth_leverage}
													onChange={e => setFormData({ ...formData, btc_eth_leverage: Number(e.target.value) })}
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
												/>
											</div>
											<div>
												<label className="block text-[11px] text-[var(--text-muted)] uppercase mb-2">山寨幣 (Altcoin) 最大槓桿</label>
												<input
													type="number"
													required
													min={1}
													max={50}
													value={formData.altcoin_leverage}
													onChange={e => setFormData({ ...formData, altcoin_leverage: Number(e.target.value) })}
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
												/>
											</div>
										</div>

										{/* Switch Options */}
										<div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2">
											<label className="flex items-center gap-2.5 p-3 rounded-sm border border-[var(--border)] bg-[#161a21]/50 cursor-pointer hover:bg-[var(--bg-subtle)] transition-all">
												<input
													type="checkbox"
													checked={formData.is_cross_margin}
													onChange={e => setFormData({ ...formData, is_cross_margin: e.target.checked })}
													className="rounded-sm bg-[var(--bg)] border-[var(--border)] text-[var(--accent)] focus:ring-0 focus:ring-offset-0 cursor-pointer"
												/>
												<div>
													<div className="text-xs font-bold text-white">全倉保證金</div>
													<div className="text-[9px] text-[var(--text-muted)]">Cross Margin</div>
												</div>
											</label>

											<label className="flex items-center gap-2.5 p-3 rounded-sm border border-[var(--border)] bg-[#161a21]/50 cursor-pointer hover:bg-[var(--bg-subtle)] transition-all">
												<input
													type="checkbox"
													checked={formData.use_coin_pool}
													onChange={e => setFormData({ ...formData, use_coin_pool: e.target.checked })}
													className="rounded-sm bg-[var(--bg)] border-[var(--border)] text-[var(--accent)] focus:ring-0 focus:ring-offset-0 cursor-pointer"
												/>
												<div>
													<div className="text-xs font-bold text-white">智能信號池</div>
													<div className="text-[9px] text-[var(--text-muted)]">Coin Pool</div>
												</div>
											</label>

											<label className="flex items-center gap-2.5 p-3 rounded-sm border border-[var(--border)] bg-[#161a21]/50 cursor-pointer hover:bg-[var(--bg-subtle)] transition-all">
												<input
													type="checkbox"
													checked={formData.use_oi_top}
													onChange={e => setFormData({ ...formData, use_oi_top: e.target.checked })}
													className="rounded-sm bg-[var(--bg)] border-[var(--border)] text-[var(--accent)] focus:ring-0 focus:ring-offset-0 cursor-pointer"
												/>
												<div>
													<div className="text-xs font-bold text-white">持倉量排行篩選</div>
													<div className="text-[9px] text-[var(--text-muted)]">OI Top Filter</div>
												</div>
											</label>
										</div>

										{/* System Prompt Customization */}
										<div className="space-y-4 pt-2 border-t border-[var(--border)]">
											<div className="flex items-center justify-between">
												<span className="text-[11px] text-[var(--text-muted)] uppercase">覆蓋基礎系統 Prompt (進階選項)</span>
												<label className="relative inline-flex items-center cursor-pointer">
													<input
														type="checkbox"
														checked={formData.override_base_prompt}
														onChange={e => setFormData({ ...formData, override_base_prompt: e.target.checked })}
														className="sr-only peer"
													/>
													<div className="w-8 h-4 bg-gray-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-gray-400 after:border-gray-300 after:border after:rounded-full after:h-3 after:w-3.5 after:transition-all peer-checked:bg-[var(--accent)] peer-checked:after:bg-white"></div>
												</label>
											</div>

											<div>
												<label className="block text-[11px] text-[var(--text-muted)] uppercase mb-2">系統 Prompt 模板名（對應 prompts/*.md）</label>
												<select
													value={formData.system_prompt_template || 'adaptive'}
													onChange={e => setFormData({ ...formData, system_prompt_template: e.target.value })}
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs font-mono"
												>
													{(promptTemplates.length ? promptTemplates : [{ name: 'adaptive' }]).map(t => (
														<option key={t.name} value={t.name}>{t.name}</option>
													))}
												</select>
											</div>

											<div>
												<label className="block text-[11px] text-[var(--text-muted)] uppercase mb-2">微調補充指令 (Custom Prompt / Signal Guidelines)</label>
												<textarea
													rows={3}
													value={formData.custom_prompt}
													onChange={e => setFormData({ ...formData, custom_prompt: e.target.value })}
													placeholder="在此輸入給 AI 交易員的額外策略指令（會追加在系統提示詞後方）..."
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs font-mono resize-y"
												/>
											</div>
										</div>

										{/* Modal Actions */}
										<div className="p-6 border-t border-[var(--border)] flex items-center justify-end gap-3 bg-[var(--bg-elev)]">
											<button
												type="button"
												onClick={() => setIsModalOpen(false)}
												className="px-4 py-2 border border-[var(--border)] text-xs font-bold text-[var(--text-muted)] hover:text-white hover:bg-[var(--bg-subtle)] rounded-sm transition-all cursor-pointer"
											>
												取消
											</button>
											<button
												type="submit"
												className="px-4 py-2 bg-blue-500/15 border border-blue-500/35 hover:bg-blue-500/25 text-blue-400 text-xs font-bold rounded-sm transition-all cursor-pointer"
											>
												確認並儲存
											</button>
										</div>
									</form>
								)}
							</div>
						</motion.div>
					</>
				)}
			</AnimatePresence>

			{/* Danger Zone: Delete Confirmation Popup */}
			<AnimatePresence>
				{deletingTrader && (
					<>
						<motion.div
							initial={{ opacity: 0 }}
							animate={{ opacity: 1 }}
							exit={{ opacity: 0 }}
							onClick={() => setDeletingTrader(null)}
							className="fixed inset-0 z-50 bg-black/80 backdrop-blur-xs"
						/>
						<motion.div
							initial={{ scale: 0.95, opacity: 0 }}
							animate={{ scale: 1, opacity: 1 }}
							exit={{ scale: 0.95, opacity: 0 }}
							className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none"
						>
							<div className="bg-[#181111]/95 border border-red-500/20 w-full max-w-md rounded-sm shadow-2xl p-6 pointer-events-auto space-y-4 font-mono">
								<div className="flex items-center gap-3 text-red-500">
									<AlertTriangle className="w-5 h-5 shrink-0" />
									<h3 className="text-sm font-bold tracking-wider uppercase">WARNING: AGENT TERMINATION</h3>
								</div>
								
								<div className="text-xs text-[var(--text-muted)] leading-relaxed space-y-2">
									<p>
										您正準備永久停用並刪除 AI 交易員 <strong className="text-white">{deletingTrader.trader_name}</strong>。
									</p>
									<p className="border-l-2 border-red-500/30 pl-3 italic bg-red-500/5 py-1">
										此操作無法還原。所有相關的本地配置與參數將會被抹除，交易所的未平倉合約可能需要手動處理。
									</p>
								</div>

								<div className="flex items-center justify-end gap-3 pt-2">
									<button
										onClick={() => setDeletingTrader(null)}
										className="px-3.5 py-1.5 border border-[var(--border)] text-xs text-[var(--text-muted)] hover:text-white hover:bg-[var(--bg-subtle)] rounded-sm transition-all cursor-pointer"
									>
										安全取消
									</button>
									<button
										onClick={handleDeleteTrader}
										className="px-3.5 py-1.5 bg-red-500/15 border border-red-500/35 hover:bg-red-500/25 text-red-400 text-xs font-bold rounded-sm transition-all cursor-pointer"
									>
										永久刪除交易員
									</button>
								</div>
							</div>
						</motion.div>
					</>
				)}
			</AnimatePresence>
		</div>
	)
}
