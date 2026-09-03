import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { AIModel, Exchange } from '../lib/api'
import { ChevronDown, ChevronUp, Cpu, Key, Radio, Save, Loader2 } from 'lucide-react'
import { toast } from 'sonner'

export function SettingsPage() {
	const [models, setModels] = useState<AIModel[]>([])
	const [exchanges, setExchanges] = useState<Exchange[]>([])
	const [signalSources, setSignalSources] = useState({ coin_pool_url: '', oi_top_url: '' })

	const [loading, setLoading] = useState(true)
	const [savingModelId, setSavingModelId] = useState<string | null>(null)
	const [savingExchangeId, setSavingExchangeId] = useState<string | null>(null)
	const [savingSignals, setSavingSignals] = useState(false)

	// Accordion state
	const [expandedModel, setExpandedModel] = useState<string | null>(null)
	const [expandedExchange, setExpandedExchange] = useState<string | null>(null)

	const loadConfigs = async () => {
		try {
			setLoading(true)
			const token = localStorage.getItem('auth_token')
			
			// Only fetch private configs if authenticated
			if (token) {
				const [mList, eList, signals] = await Promise.all([
					api.getModelConfigs().catch(() => []),
					api.getExchangeConfigs().catch(() => []),
					api.getUserSignalSource().catch(() => ({ coin_pool_url: '', oi_top_url: '' }))
				])
				setModels(mList)
				setExchanges(eList)
				setSignalSources(signals)
			} else {
				const [mList, eList] = await Promise.all([
					api.getSupportedModels().catch(() => []),
					api.getSupportedExchanges().catch(() => [])
				])
				setModels(mList)
				setExchanges(eList)
			}
		} catch (err) {
			toast.error('無法載入系統配置參數')
		} finally {
			setLoading(false)
		}
	}

	useEffect(() => {
		loadConfigs()
	}, [])

	const handleModelUpdate = async (modelId: string) => {
		setSavingModelId(modelId)
		try {
			const targetModel = models.find(m => m.id === modelId)
			if (!targetModel) return

			// Reconstruct payload using snake_case expected by API
			const modelPayload = {
				models: {
					[modelId]: {
						enabled: targetModel.enabled,
						api_key: targetModel.apiKey || '',
						custom_api_url: targetModel.customApiUrl || '',
						custom_model_name: targetModel.customModelName || '',
						env_key: targetModel.envKey || '',
						provider: targetModel.provider,
						name: targetModel.name,
					}
				}
			}

			await api.updateModelConfigs(modelPayload)
			toast.success(`模型引擎 ${targetModel.name} 配置已儲存`)
			loadConfigs()
		} catch (err: any) {
			toast.error(`模型配置儲存失敗: ${err.message || '未知錯誤'}`)
		} finally {
			setSavingModelId(null)
		}
	}

	const handleExchangeUpdate = async (exchangeId: string) => {
		setSavingExchangeId(exchangeId)
		try {
			const targetExch = exchanges.find(e => e.id === exchangeId)
			if (!targetExch) return

			// Reconstruct payload using snake_case expected by API
			const exchPayload = {
				exchanges: {
					[exchangeId]: {
						enabled: targetExch.enabled,
						api_key: targetExch.apiKey || '',
						secret_key: targetExch.secretKey || '',
						testnet: !!targetExch.testnet,
						hyperliquid_wallet_addr: targetExch.hyperliquidWalletAddr || '',
						aster_user: targetExch.asterUser || '',
						aster_signer: targetExch.asterSigner || '',
						aster_private_key: targetExch.asterPrivateKey || ''
					}
				}
			}

			await api.updateExchangeConfigs(exchPayload)
			toast.success(`交易所 ${targetExch.name} 配置已成功對接`)
			loadConfigs()
		} catch (err: any) {
			toast.error(`交易所配置儲存失敗: ${err.message || '未知錯誤'}`)
		} finally {
			setSavingExchangeId(null)
		}
	}

	const handleSignalsUpdate = async (e: React.FormEvent) => {
		e.preventDefault()
		setSavingSignals(true)
		try {
			await api.saveUserSignalSource(signalSources.coin_pool_url, signalSources.oi_top_url)
			toast.success('智能信號源端點設定已更新')
			loadConfigs()
		} catch (err: any) {
			toast.error(`信號源端點更新失敗: ${err.message || '未知錯誤'}`)
		} finally {
			setSavingSignals(false)
		}
	}

	const updateModelField = (id: string, field: keyof AIModel, value: any) => {
		setModels(prev => prev.map(m => m.id === id ? { ...m, [field]: value } : m))
	}

	const updateExchangeField = (id: string, field: keyof Exchange, value: any) => {
		setExchanges(prev => prev.map(e => e.id === id ? { ...e, [field]: value } : e))
	}

	if (loading) return <div className="p-8 text-[var(--text-muted)] font-mono text-xs">載入終端安全配置中...</div>

	return (
		<div className="p-8 max-w-4xl mx-auto space-y-8 min-h-screen pb-16">
			<div>
				<div className="font-mono text-[10px] tracking-[3px] text-[var(--text-dim)] mb-1">CONFIGURATION TERMINAL</div>
				<h1 className="text-4xl font-medium tracking-tight">系統安全設定</h1>
			</div>

			{/* Section 1: AI Engines */}
			<section className="space-y-4">
				<div className="flex items-center gap-2 font-mono text-sm tracking-widest text-[var(--text-dim)]">
					<Cpu size={16} />
					<span>AI 決策引擎設定</span>
				</div>
				
				<div className="border border-[var(--border)] rounded-sm divide-y divide-[var(--border)] bg-[var(--bg-panel)]/30 overflow-hidden">
					{models.map(m => {
						const isExpanded = expandedModel === m.id
						return (
							<div key={m.id} className="transition-colors">
								<div 
									onClick={() => setExpandedModel(isExpanded ? null : m.id)}
									className="flex items-center justify-between px-6 py-4 text-sm font-mono cursor-pointer hover:bg-[var(--bg-subtle)]/50 transition-colors"
								>
									<div className="flex items-center gap-3">
										<span className="font-medium text-white">{m.name}</span>
										<span className="text-[var(--text-dim)] text-xs">({m.provider})</span>
									</div>
									<div className="flex items-center gap-4 text-xs" onClick={e => e.stopPropagation()}>
										<span className={`px-2 py-0.5 rounded text-[10px] font-bold ${m.enabled ? 'bg-[var(--green)]/10 text-[var(--green)]' : 'bg-[var(--text-dim)]/10 text-[var(--text-dim)]'}`}>
											{m.enabled ? 'ENABLED' : 'DISABLED'}
										</span>
										<button 
											onClick={() => setExpandedModel(isExpanded ? null : m.id)}
											className="text-[var(--text-muted)] hover:text-white p-0.5 cursor-pointer"
										>
											{isExpanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
										</button>
									</div>
								</div>

								{isExpanded && (
									<div className="px-6 pb-6 pt-2 border-t border-[var(--border)] bg-[#111318]/50 space-y-4 font-mono text-xs">
										<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
											<div>
												<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">引擎啟動狀態</label>
												<select
													value={m.enabled ? 'true' : 'false'}
													onChange={e => updateModelField(m.id, 'enabled', e.target.value === 'true')}
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs cursor-pointer"
												>
													<option value="true">啟用此決策引擎</option>
													<option value="false">停用此決策引擎</option>
												</select>
											</div>
											<div>
												<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">API 金鑰 (API Key)</label>
												<input
													type="password"
													value={m.apiKey || ''}
													onChange={e => updateModelField(m.id, 'apiKey', e.target.value)}
													placeholder="輸入對接的 API 金鑰..."
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
												/>
											</div>
										</div>

										<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
											<div>
												<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">自訂 API 端點 URL (選填)</label>
												<input
													type="text"
													value={m.customApiUrl || ''}
													onChange={e => updateModelField(m.id, 'customApiUrl', e.target.value)}
													placeholder="例如: https://api.openai.com/v1"
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
												/>
											</div>
											<div>
												<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">自訂模型名稱 (選填)</label>
												<input
													type="text"
													value={m.customModelName || ''}
													onChange={e => updateModelField(m.id, 'customModelName', e.target.value)}
													placeholder="例如: gpt-4-turbo"
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
												/>
											</div>
											<div>
												<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">環境變數名 env_key (選填)</label>
												<input
													type="text"
													value={m.envKey || ''}
													onChange={e => updateModelField(m.id, 'envKey', e.target.value)}
													placeholder="例如: NVIDIA_API_KEY"
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
												/>
											</div>
										</div>

										<div className="flex justify-end pt-2">
											<button
												onClick={() => handleModelUpdate(m.id)}
												disabled={savingModelId === m.id}
												className="px-4 py-2 bg-blue-500/10 border border-blue-500/25 hover:bg-blue-500/15 text-blue-400 font-bold rounded-sm transition-all cursor-pointer flex items-center gap-2"
											>
												{savingModelId === m.id ? (
													<>
														<Loader2 className="w-3.5 h-3.5 animate-spin" />
														儲存中...
													</>
												) : (
													<>
														<Save className="w-3.5 h-3.5" />
														儲存此引擎設定
													</>
												)}
											</button>
										</div>
									</div>
								)}
							</div>
						)
					})}
				</div>
			</section>

			{/* Section 2: Exchanges */}
			<section className="space-y-4">
				<div className="flex items-center gap-2 font-mono text-sm tracking-widest text-[var(--text-dim)]">
					<Key size={16} />
					<span>交易所對接設定</span>
				</div>

				<div className="border border-[var(--border)] rounded-sm divide-y divide-[var(--border)] bg-[var(--bg-panel)]/30 overflow-hidden">
					{exchanges.map(e => {
						const isExpanded = expandedExchange === e.id
						return (
							<div key={e.id} className="transition-colors">
								<div 
									onClick={() => setExpandedExchange(isExpanded ? null : e.id)}
									className="flex items-center justify-between px-6 py-4 text-sm font-mono cursor-pointer hover:bg-[var(--bg-subtle)]/50 transition-colors"
								>
									<div className="flex items-center gap-3">
										<span className="font-medium text-white">{e.name}</span>
										<span className="text-[var(--text-dim)] text-xs">({e.type.toUpperCase()})</span>
									</div>
									<div className="flex items-center gap-4 text-xs" onClick={ev => ev.stopPropagation()}>
										<span className={`px-2 py-0.5 rounded text-[10px] font-bold ${e.enabled ? 'bg-[var(--green)]/10 text-[var(--green)]' : 'bg-[var(--text-dim)]/10 text-[var(--text-dim)]'}`}>
											{e.enabled ? 'CONNECTED' : 'DISCONNECTED'}
										</span>
										<button 
											onClick={() => setExpandedExchange(isExpanded ? null : e.id)}
											className="text-[var(--text-muted)] hover:text-white p-0.5 cursor-pointer"
										>
											{isExpanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
										</button>
									</div>
								</div>

								{isExpanded && (
									<div className="px-6 pb-6 pt-2 border-t border-[var(--border)] bg-[#111318]/50 space-y-4 font-mono text-xs">
										<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
											<div>
												<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">交易所啟用狀態</label>
												<select
													value={e.enabled ? 'true' : 'false'}
													onChange={ev => updateExchangeField(e.id, 'enabled', ev.target.value === 'true')}
													className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs cursor-pointer"
												>
													<option value="true">啟用並連接</option>
													<option value="false">關閉連接</option>
												</select>
											</div>
											<div className="flex items-end pb-2.5">
												<label className="flex items-center gap-2 cursor-pointer">
													<input
														type="checkbox"
														checked={!!e.testnet}
														onChange={ev => updateExchangeField(e.id, 'testnet', ev.target.checked)}
														className="rounded-sm bg-[var(--bg)] border-[var(--border)] text-[var(--accent)] cursor-pointer"
													/>
													<span className="text-xs text-[var(--text-muted)]">啟用 Testnet 模擬測試網模式</span>
												</label>
											</div>
										</div>

										{e.id === 'hyperliquid' ? (
											<div className="grid grid-cols-1 gap-4">
												<div>
													<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">Hyperliquid L1 錢包地址 (Wallet Address)</label>
													<input
														type="text"
														value={e.hyperliquidWalletAddr || ''}
														onChange={ev => updateExchangeField(e.id, 'hyperliquidWalletAddr', ev.target.value)}
														placeholder="0x..."
														className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
													/>
												</div>
												<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
													<div>
														<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">API 金鑰 (API Key)</label>
														<input
															type="text"
															value={e.apiKey || ''}
															onChange={ev => updateExchangeField(e.id, 'apiKey', ev.target.value)}
															placeholder="輸入交易所 API Key..."
															className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
														/>
													</div>
													<div>
														<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">API 密鑰 (Secret Key / Agent Private Key)</label>
														<input
															type="password"
															value={e.secretKey || ''}
															onChange={ev => updateExchangeField(e.id, 'secretKey', ev.target.value)}
															placeholder="輸入交易所私鑰或 API 密鑰..."
															className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
														/>
													</div>
												</div>
											</div>
										) : e.id === 'aster' ? (
											<div className="space-y-4">
												<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
													<div>
														<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">Aster 用戶標識 (User)</label>
														<input
															type="text"
															value={e.asterUser || ''}
															onChange={ev => updateExchangeField(e.id, 'asterUser', ev.target.value)}
															placeholder="Aster 用戶..."
															className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
														/>
													</div>
													<div>
														<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">Aster 簽名地址 (Signer)</label>
														<input
															type="text"
															value={e.asterSigner || ''}
															onChange={ev => updateExchangeField(e.id, 'asterSigner', ev.target.value)}
															placeholder="Aster 簽名地址..."
															className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
														/>
													</div>
												</div>
												<div>
													<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">Aster 私鑰 (Aster Private Key)</label>
													<input
														type="password"
														value={e.asterPrivateKey || ''}
														onChange={ev => updateExchangeField(e.id, 'asterPrivateKey', ev.target.value)}
														placeholder="輸入安全私鑰..."
														className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
													/>
												</div>
											</div>
										) : (
											<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
												<div>
													<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">API 金鑰 (API Key)</label>
													<input
														type="text"
														value={e.apiKey || ''}
														onChange={ev => updateExchangeField(e.id, 'apiKey', ev.target.value)}
														placeholder="輸入交易所 API Key..."
														className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
													/>
												</div>
												<div>
													<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-1.5">API 密鑰 (Secret Key)</label>
													<input
														type="password"
														value={e.secretKey || ''}
														onChange={ev => updateExchangeField(e.id, 'secretKey', ev.target.value)}
														placeholder="輸入交易所 API Secret..."
														className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
													/>
												</div>
											</div>
										)}

										<div className="flex justify-end pt-2">
											<button
												onClick={() => handleExchangeUpdate(e.id)}
												disabled={savingExchangeId === e.id}
												className="px-4 py-2 bg-blue-500/10 border border-blue-500/25 hover:bg-blue-500/15 text-blue-400 font-bold rounded-sm transition-all cursor-pointer flex items-center gap-2"
											>
												{savingExchangeId === e.id ? (
													<>
														<Loader2 className="w-3.5 h-3.5 animate-spin" />
														對接中...
													</>
												) : (
													<>
														<Save className="w-3.5 h-3.5" />
														儲存交易所設定
													</>
												)}
											</button>
										</div>
									</div>
								)}
							</div>
						)
					})}
				</div>
			</section>

			{/* Section 3: User Signal Sources */}
			<section className="space-y-4">
				<div className="flex items-center gap-2 font-mono text-sm tracking-widest text-[var(--text-dim)]">
					<Radio size={16} />
					<span>外部智能信號源設定</span>
				</div>

				<form onSubmit={handleSignalsUpdate} className="border border-[var(--border)] rounded-sm p-6 bg-[var(--bg-panel)]/30 space-y-4 font-mono text-xs">
					<div className="grid grid-cols-1 md:grid-cols-2 gap-6">
						<div>
							<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-2">智能候選幣池信號 URL (Coin Pool Endpoint)</label>
							<input
								type="url"
								value={signalSources.coin_pool_url}
								onChange={e => setSignalSources({ ...signalSources, coin_pool_url: e.target.value })}
								placeholder="https://example.com/api/coin-pool"
								className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
							/>
							<span className="text-[10px] text-[var(--text-dim)] mt-1.5 block leading-normal">
								提供 AI 量化掃描的多空智能篩選幣種池 API 端點。
							</span>
						</div>
						<div>
							<label className="block text-[10px] text-[var(--text-muted)] uppercase mb-2">合約持倉排行信號 URL (OI Top Endpoint)</label>
							<input
								type="url"
								value={signalSources.oi_top_url}
								onChange={e => setSignalSources({ ...signalSources, oi_top_url: e.target.value })}
								placeholder="https://example.com/api/oi-top"
								className="w-full px-3 py-2 bg-[#161a21] border border-[var(--border)] rounded-sm text-white focus:outline-none focus:border-blue-500/40 text-xs"
							/>
							<span className="text-[10px] text-[var(--text-dim)] mt-1.5 block leading-normal">
								提供全網持倉量（Open Interest）前列與資金費率異常監測 API 端點。
							</span>
						</div>
					</div>

					<div className="flex justify-end pt-2 border-t border-[var(--border)]">
						<button
							type="submit"
							disabled={savingSignals}
							className="px-4 py-2 bg-blue-500/10 border border-blue-500/25 hover:bg-blue-500/15 text-blue-400 font-bold rounded-sm transition-all cursor-pointer flex items-center gap-2"
						>
							{savingSignals ? (
								<>
									<Loader2 className="w-3.5 h-3.5 animate-spin" />
									儲存中...
								</>
							) : (
								<>
									<Save className="w-3.5 h-3.5" />
									儲存信號端點
								</>
							)}
						</button>
					</div>
				</form>
			</section>
		</div>
	)
}
