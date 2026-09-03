import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  AIModel,
  ModelProvider,
  Exchange,
  OperatorDirective,
  OperatorEvent,
  CreateTraderRequest,
  UpdateModelConfigRequest,
  UpdateExchangeConfigRequest,
  CompetitionData,
} from '../types'

const API_BASE = '/api'

// 全局错误处理：检测401/403并自动重新登录（管理员模式）
let isReauthenticating = false
let reauthPromise: Promise<void> | null = null

async function handleAuthError(): Promise<void> {
  // 如果已经在重新认证，等待完成
  if (isReauthenticating && reauthPromise) {
    return reauthPromise
  }

  // 检查是否为管理员模式
  try {
    const configRes = await fetch(`${API_BASE}/config`)
    if (!configRes.ok) {
      return
    }
    const config = await configRes.json()
    if (!config.admin_mode) {
      return // 非管理员模式，不自动重新登录
    }
  } catch {
    console.warn('⚠️ 无法获取系统配置，假设为管理员模式并尝试自动登录...')
    // 继续执行自动登录
  }

  isReauthenticating = true
  reauthPromise = (async () => {
    try {
      console.log('🔄 检测到认证失败，尝试自动重新登录（管理员模式）...')
      const savedPassword = sessionStorage.getItem('admin_password')
      if (!savedPassword) {
        console.warn('⚠️ 没有储存的管理员密码，无法自动重新登录')
        // 觸發全局登出
        localStorage.removeItem('auth_token')
        localStorage.removeItem('auth_user')
        window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token' }))
        window.location.href = '/login'
        return
      }
      const response = await fetch('/api/admin-login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: savedPassword }),
      })

      if (response.ok) {
        const data = await response.json()
        if (data.token) {
          const userInfo = {
            id: data.user_id || 'admin',
            email: data.email || 'admin@localhost',
          }
          localStorage.setItem('auth_token', data.token)
          localStorage.setItem('auth_user', JSON.stringify(userInfo))
          console.log('✅ 自动重新登录成功，token已更新')

          // 触发 storage 事件，通知其他标签页和组件
          window.dispatchEvent(new StorageEvent('storage', {
            key: 'auth_token',
            newValue: data.token,
          }))
        } else {
          console.error('❌ 自动重新登录失败：未返回token')
        }
      } else {
        const errorData = await response.json().catch(() => ({ error: '未知错误' }))
        console.error('❌ 自动重新登录失败:', errorData.error || response.status)
        localStorage.removeItem('auth_token')
        localStorage.removeItem('auth_user')
        window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token' }))
        window.location.href = '/login'
      }
    } catch (error) {
      console.error('❌ 自动重新登录异常:', error)
      localStorage.removeItem('auth_token')
      localStorage.removeItem('auth_user')
      window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token' }))
      window.location.href = '/login'
    } finally {
      isReauthenticating = false
      reauthPromise = null
    }
  })()

  return reauthPromise
}

// 统一的API请求包装器，自动处理认证错误
async function apiRequest<T>(
  url: string,
  options: RequestInit = {},
  retryOnAuth = true
): Promise<T> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(url, {
    ...options,
    headers,
  })

  // 处理401/403错误：尝试自动重新登录
  if ((response.status === 401 || response.status === 403) && retryOnAuth) {
    // 先保存错误信息（在读取response之前）
    let errorMessage = '认证失败，请重新登录'
    try {
      const errorData = await response.clone().json()
      errorMessage = errorData.error || errorMessage
    } catch {
      // 忽略JSON解析错误
    }

    await handleAuthError()

    // 重新获取token并重试一次
    const newToken = localStorage.getItem('auth_token')
    if (newToken) {
      headers['Authorization'] = `Bearer ${newToken}`
      const retryResponse = await fetch(url, {
        ...options,
        headers,
      })
      if (retryResponse.ok) {
        return retryResponse.json()
      }

      // 重试仍然失败，尝试读取错误信息
      try {
        const retryErrorData = await retryResponse.json()
        throw new Error(retryErrorData.error || errorMessage)
      } catch {
        throw new Error(errorMessage)
      }
    }

    // 如果重试仍然失败，抛出错误
    throw new Error(errorMessage)
  }

  if (!response.ok) {
    const errorData = await response.json().catch(() => ({ error: '请求失败' }))
    throw new Error(errorData.error || `请求失败: ${response.status}`)
  }

  return response.json()
}

export const api = {
  // AI交易员管理接口
  async getTraders(): Promise<TraderInfo[]> {
    return apiRequest<TraderInfo[]>(`${API_BASE}/my-traders`)
  },

  // 获取公开的交易员列表（无需认证）
  async getPublicTraders(): Promise<any[]> {
    const res = await fetch(`${API_BASE}/traders`)
    if (!res.ok) throw new Error('获取公开trader列表失败')
    return res.json()
  },

  async createTrader(request: CreateTraderRequest): Promise<TraderInfo> {
    return apiRequest<TraderInfo>(`${API_BASE}/traders`, {
      method: 'POST',
      body: JSON.stringify(request),
    })
  },

  async deleteTrader(traderId: string): Promise<void> {
    await apiRequest<void>(`${API_BASE}/traders/${traderId}`, {
      method: 'DELETE',
    })
  },

  async startTrader(traderId: string): Promise<void> {
    await apiRequest<void>(`${API_BASE}/traders/${traderId}/start`, {
      method: 'POST',
    })
  },

  async stopTrader(traderId: string): Promise<void> {
    await apiRequest<void>(`${API_BASE}/traders/${traderId}/stop`, {
      method: 'POST',
    })
  },

  async syncBalance(traderId: string): Promise<void> {
    await apiRequest<void>(`${API_BASE}/traders/${traderId}/sync-balance`, {
      method: 'POST',
    })
  },

  async reloadPrompts(): Promise<{ success: boolean; templates?: string[] }> {
    return apiRequest<{ success: boolean; templates?: string[] }>(
      `${API_BASE}/reload-prompts`,
      { method: 'POST' }
    )
  },

  async getOperatorDirective(): Promise<{
    directive: OperatorDirective
    digest: string
  }> {
    return apiRequest(`${API_BASE}/operator-directive`)
  },

  async listOperatorEvents(limit = 20): Promise<OperatorEvent[]> {
    return apiRequest(`${API_BASE}/operator-events?limit=${limit}`)
  },

  async createOperatorEvent(payload: {
    actor?: string
    action: string
    note?: string
    expires_in_minutes?: number
  }): Promise<{ event: OperatorEvent; directive: OperatorDirective; digest: string }> {
    return apiRequest(`${API_BASE}/operator-events`, {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },

  async updateTraderPrompt(
    traderId: string,
    customPrompt: string
  ): Promise<void> {
    await apiRequest<void>(`${API_BASE}/traders/${traderId}/prompt`, {
      method: 'PUT',
      body: JSON.stringify({ custom_prompt: customPrompt }),
    })
  },

  async getTraderConfig(traderId: string): Promise<any> {
    return apiRequest<any>(`${API_BASE}/traders/${traderId}/config`)
  },

  async updateTrader(
    traderId: string,
    request: CreateTraderRequest
  ): Promise<TraderInfo> {
    return apiRequest<TraderInfo>(`${API_BASE}/traders/${traderId}`, {
      method: 'PUT',
      body: JSON.stringify(request),
    })
  },

  // AI模型配置接口
  async getModelConfigs(): Promise<AIModel[]> {
    return apiRequest<AIModel[]>(`${API_BASE}/models`)
  },

  // 获取系统支持的AI模型列表（无需认证）
  async getSupportedModels(): Promise<AIModel[]> {
    const res = await fetch(`${API_BASE}/supported-models`)
    if (!res.ok) throw new Error('获取支持的模型失败')
    return res.json()
  },

  async getModelProviders(): Promise<ModelProvider[]> {
    const res = await fetch(`${API_BASE}/model-providers`)
    if (!res.ok) throw new Error('获取模型 provider catalog 失败')
    return res.json()
  },

  async probeModels(payload: {
    model_id?: string
    provider?: string
    base_url?: string
    api_key?: string
    env_key?: string
  }): Promise<{ ok: boolean; base_url: string; models?: string[]; error?: string; count?: number }> {
    return apiRequest(`${API_BASE}/models/probe`, {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },

  async updateModelConfigs(request: UpdateModelConfigRequest): Promise<void> {
    await apiRequest<void>(`${API_BASE}/models`, {
      method: 'PUT',
      body: JSON.stringify(request),
    })
  },

  // 交易所配置接口
  async getExchangeConfigs(): Promise<Exchange[]> {
    return apiRequest<Exchange[]>(`${API_BASE}/exchanges`)
  },

  // 获取系统支持的交易所列表（无需认证）
  async getSupportedExchanges(): Promise<Exchange[]> {
    const res = await fetch(`${API_BASE}/supported-exchanges`)
    if (!res.ok) throw new Error('获取支持的交易所失败')
    return res.json()
  },

  async updateExchangeConfigs(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    await apiRequest<void>(`${API_BASE}/exchanges`, {
      method: 'PUT',
      body: JSON.stringify(request),
    })
  },

  // 获取系统状态（支持trader_id）
  async getStatus(traderId?: string): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    return apiRequest<SystemStatus>(url)
  },

  // 获取账户信息（支持trader_id）
  async getAccount(traderId?: string): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const data = await apiRequest<AccountInfo>(url, {
      cache: 'no-store',
      headers: {
        'Cache-Control': 'no-cache',
      },
    })
    console.log('📊 API返回的账户信息:', data)
    console.log('📊 initial_balance:', data.initial_balance)
    console.log('📊 total_equity:', data.total_equity)
    console.log('📊 total_pnl:', data.total_pnl)
    return data
  },

  // 获取持仓列表（支持trader_id）
  async getPositions(traderId?: string): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    return apiRequest<Position[]>(url)
  },

  // 获取决策日志（支持trader_id）
  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    return apiRequest<DecisionRecord[]>(url)
  },

  // 获取最新决策（支持trader_id）
  async getLatestDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions/latest?trader_id=${traderId}`
      : `${API_BASE}/decisions/latest`
    return apiRequest<DecisionRecord[]>(url)
  },

  // 获取统计信息（支持trader_id）
  async getStatistics(traderId?: string): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    return apiRequest<Statistics>(url)
  },

  // 获取收益率历史数据（支持trader_id）
  async getEquityHistory(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    return apiRequest<any[]>(url)
  },

  // 批量获取多个交易员的历史数据（无需认证）
  async getEquityHistoryBatch(traderIds: string[]): Promise<any> {
    const res = await fetch(`${API_BASE}/equity-history-batch`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ trader_ids: traderIds }),
    })
    if (!res.ok) throw new Error('获取批量历史数据失败')
    return res.json()
  },

  // 获取前5名交易员数据（无需认证）
  async getTopTraders(): Promise<any[]> {
    const res = await fetch(`${API_BASE}/top-traders`)
    if (!res.ok) throw new Error('获取前5名交易员失败')
    return res.json()
  },

  // 获取公开交易员配置（无需认证）
  async getPublicTraderConfig(traderId: string): Promise<any> {
    const res = await fetch(`${API_BASE}/traders/${traderId}/public-config`)
    if (!res.ok) throw new Error('获取公开交易员配置失败')
    return res.json()
  },

  // 获取AI学习表现分析（支持trader_id）
  async getPerformance(traderId?: string): Promise<any> {
    const url = traderId
      ? `${API_BASE}/performance?trader_id=${traderId}`
      : `${API_BASE}/performance`
    return apiRequest<any>(url, {}, false) // 公开API，不需要重试认证
  },

  // 获取竞赛数据（无需认证）
  async getCompetition(): Promise<CompetitionData> {
    const res = await fetch(`${API_BASE}/competition`)
    if (!res.ok) throw new Error('获取竞赛数据失败')
    return res.json()
  },

  // 用户信号源配置接口
  async getUserSignalSource(): Promise<{
    coin_pool_url: string
    oi_top_url: string
  }> {
    return apiRequest<{
      coin_pool_url: string
      oi_top_url: string
    }>(`${API_BASE}/user/signal-sources`)
  },

  async saveUserSignalSource(
    coinPoolUrl: string,
    oiTopUrl: string
  ): Promise<void> {
    await apiRequest<void>(`${API_BASE}/user/signal-sources`, {
      method: 'POST',
      body: JSON.stringify({
        coin_pool_url: coinPoolUrl,
        oi_top_url: oiTopUrl,
      }),
    })
  },

  // 获取服务器IP（需要认证，用于白名单配置）
  async getServerIP(): Promise<{
    public_ip: string
    message: string
  }> {
    return apiRequest<{
      public_ip: string
      message: string
    }>(`${API_BASE}/server-ip`)
  },

  // 回测接口 (Backtest API)
  async startBacktest(payload: {
    strategy: string
    symbol: string
    timeframe: string
    initial_balance: number
    leverage: number
    description?: string
  }): Promise<any> {
    return apiRequest(`${API_BASE}/backtest/run`, {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },

  async listBacktestRuns(): Promise<any[]> {
    return apiRequest<any[]>(`${API_BASE}/backtest/runs`)
  },

  async getBacktestRun(runId: string): Promise<any> {
    return apiRequest<any>(`${API_BASE}/backtest/runs/${runId}`)
  },
}
