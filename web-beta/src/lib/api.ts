const API_BASE = '/api'

type RequestInitWithRetry = RequestInit & { retryOnAuth?: boolean }

// 統一的管理員登入與自動重新認證函數
async function handleAuthError(password: string): Promise<string> {
	const response = await fetch(`${API_BASE}/admin-login`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ password }),
	})
	if (!response.ok) {
		throw new Error('AUTH_FAILED')
	}
	const data = await response.json()
	if (data.token) {
		const userInfo = {
			id: data.user_id || 'admin',
			email: data.email || 'admin@localhost',
		}
		localStorage.setItem('auth_token', data.token)
		localStorage.setItem('auth_user', JSON.stringify(userInfo))
		sessionStorage.setItem('admin_password', password)
		window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token', newValue: data.token }))
		return data.token
	}
	throw new Error('NO_TOKEN')
}

async function apiRequest<T>(url: string, options: RequestInitWithRetry = {}): Promise<T> {
	let token = localStorage.getItem('auth_token')
	const headers: Record<string, string> = {
		'Content-Type': 'application/json',
		...(options.headers as Record<string, string>),
	}
	if (token) headers['Authorization'] = `Bearer ${token}`
	
	let res = await fetch(`${API_BASE}${url}`, { ...options, headers })
	
	// 處理 401/403：嘗試使用 sessionStorage 中保存的密碼進行自動重新登入並重試
	if (res.status === 401 || res.status === 403) {
		const savedPassword = sessionStorage.getItem('admin_password')
		if (savedPassword) {
			try {
				token = await handleAuthError(savedPassword)
				headers['Authorization'] = `Bearer ${token}`
				res = await fetch(`${API_BASE}${url}`, { ...options, headers })
			} catch {
				// 自動登入失敗，清除 token 觸發解鎖畫面
				localStorage.removeItem('auth_token')
				localStorage.removeItem('auth_user')
				window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token' }))
				throw new Error('UNAUTHORIZED')
			}
		} else {
			// 無儲存密碼，清除 token 觸發解鎖畫面
			localStorage.removeItem('auth_token')
			localStorage.removeItem('auth_user')
			window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token' }))
			throw new Error('UNAUTHORIZED')
		}
	}
	
	if (res.ok) return res.json()
	throw new Error(`Request failed: ${res.status}`)
}

export async function apiFetchPublic<T>(url: string): Promise<T> {
	const res = await fetch(`${API_BASE}${url}`)
	if (!res.ok) throw new Error(`請求失敗: ${res.status}`)
	return res.json()
}

export interface SystemStatus {
	trader_id: string
	trader_name: string
	ai_model: string
	is_running: boolean
	start_time: string
	runtime_minutes: number
	call_count: number
	initial_balance: number
	scan_interval: string
	stop_until: string
	last_reset_time: string
	ai_provider: string
	risk_halted?: boolean
	consecutive_wait?: number
	daily_pnl?: number
	injected_skills?: string[]
}

export interface AccountInfo {
	total_equity: number
	wallet_balance: number
	unrealized_profit: number
	available_balance: number
	total_pnl: number
	total_pnl_pct: number
	total_unrealized_pnl: number
	initial_balance: number
	daily_pnl: number
	position_count: number
	margin_used: number
	margin_used_pct: number
}

export interface Position {
	symbol: string
	side: string
	entry_price: number
	mark_price: number
	quantity: number
	leverage: number
	unrealized_pnl: number
	unrealized_pnl_pct: number
	liquidation_price: number
	margin_used: number
}

export interface DecisionAction {
	action: string
	symbol: string
	quantity: number
	leverage: number
	price: number
	order_id: number
	timestamp: string
	success: boolean
	error?: string
}

export interface AccountSnapshot {
	total_balance: number
	available_balance: number
	total_unrealized_profit: number
	position_count: number
	margin_used_pct: number
}

export interface DecisionRecord {
	timestamp: string
	cycle_number: number
	input_prompt: string
	cot_trace: string
	decision_json: string
	account_state: AccountSnapshot
	positions: any[]
	candidate_coins: string[]
	decisions: DecisionAction[]
	technical_analysis?: Record<string, any>
	price_action?: Record<string, any>
	execution_log: string[]
	success: boolean
	error_message?: string
}

export interface Statistics {
	total_trades: number
	win_rate: number
	profit_factor: number
	total_pnl: number
	max_drawdown: number
	avg_profit: number
	avg_loss: number
	trades_by_symbol: Record<string, { count: number; pnl: number }>
}

export interface TraderInfo {
	trader_id: string
	trader_name: string
	ai_model: string
	exchange_id?: string
	is_running?: boolean
	custom_prompt?: string
	use_coin_pool?: boolean
	use_oi_top?: boolean
}

export interface AIModel {
	id: string
	name: string
	provider: string
	enabled: boolean
	apiKey?: string
	customApiUrl?: string
	customModelName?: string
	envKey?: string
	hasApiKey?: boolean
}

export interface Exchange {
	id: string
	name: string
	type: 'cex' | 'dex'
	enabled: boolean
	testnet?: boolean
	apiKey?: string
	secretKey?: string
	hyperliquidWalletAddr?: string
	asterUser?: string
	asterSigner?: string
	asterPrivateKey?: string
}

export interface CreateTraderRequest {
	name: string
	ai_model_id: string
	exchange_id: string
	initial_balance: number
	scan_interval_minutes?: number
	btc_eth_leverage?: number
	altcoin_leverage?: number
	trading_symbols?: string
	custom_prompt?: string
	override_base_prompt?: boolean
	system_prompt_template?: string
	is_cross_margin?: boolean
	use_coin_pool?: boolean
	use_oi_top?: boolean
}

export interface CompetitionTraderData {
	trader_id: string
	trader_name: string
	ai_model: string
	exchange: string
	total_equity: number
	total_pnl: number
	total_pnl_pct: number
	position_count: number
	margin_used_pct: number
	is_running: boolean
}

export interface CompetitionData {
	traders: CompetitionTraderData[]
	count: number
}

export const api = {
	// 獲取系統配置
	async getSystemConfig(): Promise<{ admin_mode: boolean; beta_mode: boolean }> {
		return apiFetchPublic<{ admin_mode: boolean; beta_mode: boolean }>('/config')
	},

	// 管理員登入認證
	async loginAdmin(password: string): Promise<string> {
		return handleAuthError(password)
	},

	// 多用戶登入認證
	async login(email: string, password: string): Promise<{ success: boolean; message?: string; userID?: string; requiresOTP?: boolean; requiresOTPSetup?: boolean }> {
		try {
			const response = await fetch(`${API_BASE}/login`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email, password }),
			})
			const data = await response.json()
			
			if (response.ok) {
				if (data.requires_otp) {
					return {
						success: true,
						userID: data.user_id,
						requiresOTP: true,
						message: data.message,
					}
				}
			} else {
				if (response.status === 401 && data.requires_otp_setup) {
					return {
						success: true,
						userID: data.user_id,
						requiresOTPSetup: true,
						message: data.error || '帳戶未完成OTP設置',
					}
				}
				return { success: false, message: data.error || '登入失敗' }
			}
		} catch (error) {
			return { success: false, message: '登入失敗，請重試' }
		}
		return { success: false, message: '未知錯誤' }
	},

	// 用戶註冊
	async register(email: string, password: string, betaCode?: string): Promise<{ success: boolean; userID?: string; otpSecret?: string; qrCodeURL?: string; message?: string }> {
		try {
			const body: Record<string, string> = { email, password }
			if (betaCode) body.beta_code = betaCode

			const response = await fetch(`${API_BASE}/register`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body),
			})
			const data = await response.json()
			if (response.ok) {
				return {
					success: true,
					userID: data.user_id,
					otpSecret: data.otp_secret,
					qrCodeURL: data.qr_code_url,
					message: data.message,
				}
			} else {
				return { success: false, message: data.error || '註冊失敗' }
			}
		} catch (error) {
			return { success: false, message: '註冊失敗，請重試' }
		}
	},

	// 驗證登入 OTP 並寫入憑證
	async verifyOTP(userID: string, otpCode: string): Promise<{ success: boolean; message?: string }> {
		try {
			const response = await fetch(`${API_BASE}/verify-otp`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ user_id: userID, otp_code: otpCode }),
			})
			const data = await response.json()
			if (response.ok) {
				const userInfo = { id: data.user_id, email: data.email }
				localStorage.setItem('auth_token', data.token)
				localStorage.setItem('auth_user', JSON.stringify(userInfo))
				window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token', newValue: data.token }))
				return { success: true, message: data.message }
			} else {
				return { success: false, message: data.error || '驗證碼錯誤' }
			}
		} catch (error) {
			return { success: false, message: '驗證失敗，請重試' }
		}
	},

	// 完成註冊並驗證 OTP 寫入憑證
	async completeRegistration(userID: string, otpCode: string): Promise<{ success: boolean; message?: string }> {
		try {
			const response = await fetch(`${API_BASE}/complete-registration`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ user_id: userID, otp_code: otpCode }),
			})
			const data = await response.json()
			if (response.ok) {
				const userInfo = { id: data.user_id, email: data.email }
				localStorage.setItem('auth_token', data.token)
				localStorage.setItem('auth_user', JSON.stringify(userInfo))
				window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token', newValue: data.token }))
				return { success: true, message: data.message }
			} else {
				return { success: false, message: data.error || '註冊完成失敗' }
			}
		} catch (error) {
			return { success: false, message: '驗證失敗，請重試' }
		}
	},

	// 重設密碼
	async resetPassword(email: string, newPassword: string, otpCode: string): Promise<{ success: boolean; message?: string }> {
		try {
			const response = await fetch(`${API_BASE}/reset-password`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					email,
					new_password: newPassword,
					otp_code: otpCode,
				}),
			})
			const data = await response.json()
			if (response.ok) {
				return { success: true, message: data.message }
			} else {
				return { success: false, message: data.error || '重設密碼失敗' }
			}
		} catch (error) {
			return { success: false, message: '請求失敗，請重試' }
		}
	},

	// 登出
	async logout(): Promise<void> {
		const token = localStorage.getItem('auth_token')
		if (token) {
			try {
				await fetch(`${API_BASE}/logout`, {
					method: 'POST',
					headers: { Authorization: `Bearer ${token}` },
				})
			} catch (e) {
				// 忽略登出網路錯誤
			}
		}
		localStorage.removeItem('auth_token')
		localStorage.removeItem('auth_user')
		sessionStorage.removeItem('admin_password')
		window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token', newValue: null }))
	},

	// AI交易员管理接口
	async getTraders(): Promise<TraderInfo[]> {
		return apiRequest<TraderInfo[]>('/my-traders')
	},
	async getPublicTraders(): Promise<any[]> {
		return apiFetchPublic<any[]>('/traders')
	},
	async createTrader(request: CreateTraderRequest): Promise<TraderInfo> {
		return apiRequest<TraderInfo>('/traders', { method: 'POST', body: JSON.stringify(request) })
	},
	async deleteTrader(traderId: string): Promise<void> {
		await apiRequest<void>(`/traders/${traderId}`, { method: 'DELETE' })
	},
	async startTrader(traderId: string): Promise<void> {
		await apiRequest<void>(`/traders/${traderId}/start`, { method: 'POST' })
	},
	async stopTrader(traderId: string): Promise<void> {
		await apiRequest<void>(`/traders/${traderId}/stop`, { method: 'POST' })
	},
	async updateTraderPrompt(traderId: string, customPrompt: string): Promise<void> {
		await apiRequest<void>(`/traders/${traderId}/prompt`, {
			method: 'PUT',
			body: JSON.stringify({ custom_prompt: customPrompt }),
		})
	},
	async getTraderConfig(traderId: string): Promise<any> {
		return apiRequest<any>(`/traders/${traderId}/config`)
	},
	async updateTrader(traderId: string, request: CreateTraderRequest): Promise<TraderInfo> {
		return apiRequest<TraderInfo>(`/traders/${traderId}`, { method: 'PUT', body: JSON.stringify(request) })
	},

	// AI模型配置接口
	async getModelConfigs(): Promise<AIModel[]> {
		return apiRequest<AIModel[]>('/models')
	},
	async getSupportedModels(): Promise<AIModel[]> {
		return apiFetchPublic<AIModel[]>('/supported-models')
	},
	async updateModelConfigs(request: { models: { [key: string]: { enabled: boolean; api_key: string; custom_api_url?: string; custom_model_name?: string; env_key?: string; name?: string; provider?: string } } }): Promise<void> {
		await apiRequest<void>('/models', { method: 'PUT', body: JSON.stringify(request) })
	},

	// 交易所配置接口
	async getExchangeConfigs(): Promise<Exchange[]> {
		return apiRequest<Exchange[]>('/exchanges')
	},
	async getSupportedExchanges(): Promise<Exchange[]> {
		return apiFetchPublic<Exchange[]>('/supported-exchanges')
	},
	async updateExchangeConfigs(request: { exchanges: { [key: string]: { enabled: boolean; api_key: string; secret_key: string; testnet?: boolean; hyperliquid_wallet_addr?: string; aster_user?: string; aster_signer?: string; aster_private_key?: string } } }): Promise<void> {
		await apiRequest<void>('/exchanges', { method: 'PUT', body: JSON.stringify(request) })
	},

	// 获取系统状态（支持trader_id）
	async getStatus(traderId?: string): Promise<SystemStatus> {
		const url = traderId ? `/status?trader_id=${traderId}` : '/status'
		return apiRequest<SystemStatus>(url)
	},

	// 获取账户信息（支持trader_id）
	async getAccount(traderId?: string): Promise<AccountInfo> {
		const url = traderId ? `/account?trader_id=${traderId}` : '/account'
		return apiRequest<AccountInfo>(url, { headers: { 'Cache-Control': 'no-cache' } })
	},

	// 获取持仓列表（支持trader_id）
	async getPositions(traderId?: string): Promise<Position[]> {
		const url = traderId ? `/positions?trader_id=${traderId}` : '/positions'
		return apiRequest<Position[]>(url)
	},

	// 获取决策日志（支持trader_id）
	async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
		const url = traderId ? `/decisions?trader_id=${traderId}` : '/decisions'
		return apiRequest<DecisionRecord[]>(url)
	},

	// 获取最新决策（支持trader_id）
	async getLatestDecisions(traderId?: string): Promise<DecisionRecord[]> {
		const url = traderId ? `/decisions/latest?trader_id=${traderId}` : '/decisions/latest'
		return apiRequest<DecisionRecord[]>(url)
	},

	// 获取统计信息（支持trader_id）
	async getStatistics(traderId?: string): Promise<Statistics> {
		const url = traderId ? `/statistics?trader_id=${traderId}` : '/statistics'
		return apiRequest<Statistics>(url)
	},

	// 获取收益率历史数据（支持trader_id）
	async getEquityHistory(traderId?: string): Promise<any[]> {
		const url = traderId ? `/equity-history?trader_id=${traderId}` : '/equity-history'
		return apiRequest<any[]>(url)
	},

	// 批量获取多个交易员的历史数据
	async getEquityHistoryBatch(traderIds: string[]): Promise<any> {
		const res = await fetch(`${API_BASE}/equity-history-batch`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ trader_ids: traderIds }),
		})
		if (!res.ok) throw new Error('获取批量历史数据失败')
		return res.json()
	},

	// 获取前5名交易员数据（无需认证）
	async getTopTraders(): Promise<any[]> {
		return apiFetchPublic<any[]>('/top-traders')
	},

	// 获取公开交易员配置（无需认证）
	async getPublicTraderConfig(traderId: string): Promise<any> {
		return apiFetchPublic<any>(`/traders/${traderId}/public-config`)
	},

	// 获取AI学习表现分析
	async getPerformance(traderId?: string): Promise<any> {
		const url = traderId ? `/performance?trader_id=${traderId}` : '/performance'
		return apiRequest<any>(url, { retryOnAuth: false })
	},

	// 获取竞赛数据
	async getCompetition(): Promise<CompetitionData> {
		return apiFetchPublic<CompetitionData>('/competition')
	},

	// 用户信号源配置接口
	async getUserSignalSource(): Promise<{ coin_pool_url: string; oi_top_url: string }> {
		return apiRequest<{ coin_pool_url: string; oi_top_url: string }>('/user/signal-sources')
	},
	async saveUserSignalSource(coinPoolUrl: string, oiTopUrl: string): Promise<void> {
		await apiRequest<void>('/user/signal-sources', {
			method: 'POST',
			body: JSON.stringify({ coin_pool_url: coinPoolUrl, oi_top_url: oiTopUrl }),
		})
	},

	// 获取服务器IP
	async getServerIP(): Promise<{ public_ip: string; message: string }> {
		return apiRequest<{ public_ip: string; message: string }>('/server-ip')
	},
}
