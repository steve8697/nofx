import { useEffect, useMemo, useState } from 'react'
import useSWR from 'swr'
import { api } from './lib/api'
import { EquityChart } from './components/EquityChart'
import { AITradersPage } from './components/AITradersPage'
import { LoginPage } from './components/LoginPage'
import { RegisterPage } from './components/RegisterPage'
import { ResetPasswordPage } from './components/ResetPasswordPage'
import { CompetitionPage } from './components/CompetitionPage'
import { LandingPage } from './pages/LandingPage'
import { FAQPage } from './pages/FAQPage'
import HeaderBar from './components/landing/HeaderBar'
import AILearning from './components/AILearning'
import { OperatorPanel } from './components/OperatorPanel'
import { Interactive3DBackground } from './components/Interactive3DBackground'
import { LanguageProvider, useLanguage } from './contexts/LanguageContext'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { t, type Language } from './i18n/translations'
import { useSystemConfig } from './hooks/useSystemConfig'
import { AlertTriangle } from 'lucide-react'
import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  AIModel,
} from './types'

type Page = 'competition' | 'traders' | 'trader'

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

function App() {
  const { language, setLanguage } = useLanguage()
  const { user, token, logout, isLoading } = useAuth()
  const { config: systemConfig, loading: configLoading } = useSystemConfig()
  const [route, setRoute] = useState(window.location.pathname)

  // 从URL路径读取初始页面状态（支持刷新保持页面）
  const getInitialPage = (): Page => {
    const path = window.location.pathname
    const hash = window.location.hash.slice(1) // 去掉 #

    if (path === '/traders' || hash === 'traders') return 'traders'
    if (path === '/dashboard' || hash === 'trader' || hash === 'details')
      return 'trader'
    return 'competition' // 默认为竞赛页面
  }

  const [currentPage, setCurrentPage] = useState<Page>(getInitialPage())
  const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>()
  const [lastUpdate, setLastUpdate] = useState<string>('--:--:--')

  // 统一的路由处理逻辑 - 监听URL变化并同步状态
  useEffect(() => {
    const handleRouteChange = () => {
      const path = window.location.pathname
      const hash = window.location.hash.slice(1)

      // 更新 route 状态
      setRoute(path)

      // 根据路径和hash更新 currentPage
      if (path === '/traders' || hash === 'traders') {
        setCurrentPage('traders')
      } else if (
        path === '/dashboard' ||
        hash === 'trader' ||
        hash === 'details'
      ) {
        setCurrentPage('trader')
      } else if (
        path === '/competition' ||
        hash === 'competition' ||
        hash === '' ||
        path === '/'
      ) {
        setCurrentPage('competition')
      }
    }

    // 初始化时执行一次
    handleRouteChange()

    // 监听路由变化
    window.addEventListener('hashchange', handleRouteChange)
    window.addEventListener('popstate', handleRouteChange)

    return () => {
      window.removeEventListener('hashchange', handleRouteChange)
      window.removeEventListener('popstate', handleRouteChange)
    }
  }, [])

  // 當偵測到用戶已登入且處於登入/註冊頁面時，主動重定向至 dashboard
  useEffect(() => {
    if (user && token && (route === '/login' || route === '/register' || route === '/reset-password' || route === '/')) {
      window.history.pushState({}, '', '/dashboard')
      setRoute('/dashboard')
      setCurrentPage('trader')
    }
  }, [user, token, route])


  // 获取trader列表（仅在用户登录时）
  const { data: traders } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    {
      refreshInterval: 10000,
    }
  )

  // 当获取到traders后，设置默认选中第一个
  useEffect(() => {
    if (traders && traders.length > 0 && !selectedTraderId) {
      setSelectedTraderId(traders[0].trader_id)
    }
  }, [traders, selectedTraderId])

  // 如果在trader页面，获取该trader的数据
  const { data: status } = useSWR<SystemStatus>(
    currentPage === 'trader' && selectedTraderId
      ? `status-${selectedTraderId}`
      : null,
    () => api.getStatus(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: account } = useSWR<AccountInfo>(
    currentPage === 'trader' && selectedTraderId
      ? `account-${selectedTraderId}`
      : null,
    () => api.getAccount(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: positions } = useSWR<Position[]>(
    currentPage === 'trader' && selectedTraderId
      ? `positions-${selectedTraderId}`
      : null,
    () => api.getPositions(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: decisions } = useSWR<DecisionRecord[]>(
    currentPage === 'trader' && selectedTraderId
      ? `decisions/latest-${selectedTraderId}`
      : null,
    () => api.getLatestDecisions(selectedTraderId),
    {
      refreshInterval: 30000, // 30秒刷新（决策更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  const { data: stats } = useSWR<Statistics>(
    currentPage === 'trader' && selectedTraderId
      ? `statistics-${selectedTraderId}`
      : null,
    () => api.getStatistics(selectedTraderId),
    {
      refreshInterval: 30000, // 30秒刷新（统计数据更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  useEffect(() => {
    if (account) {
      const now = new Date().toLocaleTimeString()
      setLastUpdate(now)
    }
  }, [account])

  const selectedTrader = traders?.find((t: TraderInfo) => t.trader_id === selectedTraderId)

  // 统一的页面导航函数
  const navigateToPage = (page: string) => {
    let path = '/competition'
    let targetPage: Page = 'competition'

    if (page === 'competition') {
      path = '/competition'
      targetPage = 'competition'
    } else if (page === 'traders') {
      path = '/traders'
      targetPage = 'traders'
    } else if (page === 'trader') {
      path = '/dashboard'
      targetPage = 'trader'
    }

    window.history.pushState({}, '', path)
    setRoute(path)
    setCurrentPage(targetPage)
  }

  // Show loading spinner while checking auth or config
  if (isLoading || configLoading) {
    return (
      <div
        className="min-h-screen flex items-center justify-center bg-transparent"
      >
        <div className="text-center">
          <img
            src="/icons/aetheris.svg"
            alt="Aetheris Logo"
            className="w-16 h-16 mx-auto mb-4 animate-pulse"
          />
          <p style={{ color: '#EAECEF' }}>{t('loading', language)}</p>
        </div>
      </div>
    )
  }

  // Handle specific routes regardless of authentication
  if (route === '/login') {
    if (user && token) {
      window.history.pushState({}, '', '/dashboard')
      // Let useEffect handle state update
      return null
    }
    return <LoginPage />
  }
  if (route === '/register') {
    if (systemConfig?.admin_mode) {
      window.history.pushState({}, '', '/login')
      return <LoginPage />
    }
    return <RegisterPage />
  }
  if (route === '/faq') {
    if (systemConfig?.admin_mode) {
      window.history.pushState({}, '', user && token ? '/dashboard' : '/login')
      return null
    }
    return <FAQPage />
  }
  if (route === '/reset-password') {
    if (systemConfig?.admin_mode) {
      window.history.pushState({}, '', '/login')
      return <LoginPage />
    }
    return <ResetPasswordPage />
  }
  if (route === '/competition') {
    if (systemConfig?.admin_mode && (!user || !token)) {
      window.history.pushState({}, '', '/login')
      return <LoginPage />
    }
    return (
      <div
        className="min-h-screen bg-transparent text-[#EAECEF] relative"
      >
        <Interactive3DBackground currentPage="competition" />
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="competition"
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          isAdminMode={systemConfig?.admin_mode}
          onPageChange={navigateToPage}
        />
        <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24 relative z-10">
          <CompetitionPage />
        </main>
      </div>
    )
  }

  // Show main app for authenticated users or redirect them to dashboard
  if (route === '/' || route === '') {
    if (user && token) {
      window.history.pushState({}, '', '/dashboard')
      return (
        <div
          className="min-h-screen bg-transparent text-[#EAECEF] relative"
        >
          {/* 3D Atmospheric Interactive WebGL Background */}
          <Interactive3DBackground currentPage="trader" />

          <HeaderBar
            isLoggedIn={!!user}
            currentPage="trader"
            language={language}
            onLanguageChange={setLanguage}
            user={user}
            onLogout={logout}
            isAdminMode={systemConfig?.admin_mode}
            onPageChange={navigateToPage}
          />
          <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24 relative z-10">
            <TraderDetailsPage
              selectedTrader={selectedTrader}
              status={status}
              account={account}
              positions={positions}
              decisions={decisions}
              stats={stats}
              lastUpdate={lastUpdate}
              language={language}
              traders={traders}
              selectedTraderId={selectedTraderId}
              onTraderSelect={setSelectedTraderId}
            />
          </main>
        </div>
      )
    } else {
      // 未登录时，若是自部署模式（admin_mode）直接引導至登入頁
      if (systemConfig?.admin_mode) {
        window.history.pushState({}, '', '/login')
        return <LoginPage />
      }
      return <LandingPage isAdminMode={systemConfig?.admin_mode} />
    }
  }

  // In admin mode, require authentication for any protected routes
  if (systemConfig?.admin_mode && (!user || !token)) {
    window.history.pushState({}, '', '/login')
    return <LoginPage />
  }

  // Show main app for authenticated users on other routes (non-admin mode)
  if (!systemConfig?.admin_mode && (!user || !token)) {
    // Default to landing page when not authenticated and no specific route
    return <LandingPage />
  }

  return (
    <div
      className="min-h-screen bg-transparent text-[#EAECEF] relative"
    >
      {/* 3D Atmospheric Interactive WebGL Background */}
      <Interactive3DBackground currentPage={currentPage} />

      <HeaderBar
        isLoggedIn={!!user}
        currentPage={currentPage}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        isAdminMode={systemConfig?.admin_mode}
        onPageChange={navigateToPage}
      />

      {/* Main Content */}
      <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24 relative z-10">
        {currentPage === 'competition' ? (
          <CompetitionPage />
        ) : currentPage === 'traders' ? (
          <AITradersPage
            onTraderSelect={(traderId) => {
              setSelectedTraderId(traderId)
              navigateToPage('trader')
            }}
          />
        ) : (
          <TraderDetailsPage
            selectedTrader={selectedTrader}
            status={status}
            account={account}
            positions={positions}
            decisions={decisions}
            stats={stats}
            lastUpdate={lastUpdate}
            language={language}
            traders={traders}
            selectedTraderId={selectedTraderId}
            onTraderSelect={setSelectedTraderId}
          />
        )}
      </main>

      {/* Footer */}
      <footer
        className="mt-20 border-t"
        style={{ borderColor: 'rgba(255, 255, 255, 0.035)', background: 'rgba(10, 10, 12, 0.85)' }}
      >
        <div
          className="max-w-[1920px] mx-auto px-6 py-8 text-center text-xs font-mono"
          style={{ color: '#5E6673' }}
        >
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4 max-w-6xl mx-auto border-b border-white/[0.03] pb-6 mb-6">
            <div className="flex items-center gap-2">
              <span className="inline-block w-2 h-2 rounded-full bg-white animate-pulse"></span>
              <span className="text-white font-bold tracking-wider">AETHERIS QUANTUM v5.0.0-STEALTH</span>
            </div>
            <div className="text-gray-500">
              COGNITIVE TRADING CORE: <span className="text-gray-400 font-bold">ONLINE</span> // STEALTH MATRIX ACTIVE
            </div>
            <div className="text-gray-500">
              ENVIRONMENT: <span className="text-gray-400 font-bold">LOCALHOST DOCKER CELL</span>
            </div>
          </div>
          <p className="text-gray-600 uppercase tracking-widest text-[10px] mb-2">
            {language === 'zh' 
              ? '⚡ 私有量化控制台 • 認知核心數據已完全隔離保護' 
              : '⚡ PRIVATE ALGORITHMIC CONSOLE • COGNITIVE DATA STRICTLY ISOLATED'}
          </p>
          <p className="text-gray-600 text-[10px]">
            {language === 'zh'
              ? '⚠️ 警示：量化算法具有不可預知風險。系統已加載自適應風控 (Micro-Equity Mode / Anti-Paralysis Detection)，請實時監測遙測日誌。'
              : '⚠️ TELEMETRY NOTICE: QUANTUM ALGORITHMS CONTAIN INHERENT RISK. ADAPTIVE RISK MITIGATION ACTIVE (ANTI-PARALYSIS).'}
          </p>
        </div>
      </footer>
    </div>
  )
}

// Trader Details Page Component
function TraderDetailsPage({
  selectedTrader,
  status,
  account,
  positions,
  decisions,
  lastUpdate,
  language,
  traders,
  selectedTraderId,
  onTraderSelect,
}: {
  selectedTrader?: TraderInfo
  traders?: TraderInfo[]
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
  status?: SystemStatus
  account?: AccountInfo
  positions?: Position[]
  decisions?: DecisionRecord[]
  stats?: Statistics
  lastUpdate: string
  language: Language
}) {
  const [allModels, setAllModels] = useState<AIModel[]>([])
  const { user, token } = useAuth()

  useEffect(() => {
    const loadModels = async () => {
      if (user && token) {
        try {
          const models = await api.getModelConfigs()
          setAllModels(models)
        } catch (error) {
          console.error('Failed to load models in Details:', error)
        }
      }
    }
    loadModels()
  }, [user, token])

  const getModelShowName = (modelId: string) => {
    // 优先匹配完整 ID
    let model = allModels?.find((m) => m.id === modelId)
    
    // 找不到时，剥离用户前缀进行匹配
    if (!model) {
      const provider = modelId.includes('_') ? modelId.split('_').pop() : modelId
      model = allModels?.find((m) => m.id === provider || m.provider === provider || m.id.split('_').pop() === provider)
    }

    if (model) {
      if (model.customModelName && model.customModelName.trim() !== '') {
        return model.customModelName
      }
      return model.name
    }
    return getModelDisplayName(modelId)
  }

  if (!selectedTrader) {
    return (
      <div className="space-y-6">
        {/* Loading Skeleton - Binance Style */}
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-8 w-48 mb-3"></div>
          <div className="flex gap-4">
            <div className="skeleton h-4 w-32"></div>
            <div className="skeleton h-4 w-24"></div>
            <div className="skeleton h-4 w-28"></div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="binance-card p-5 animate-pulse">
              <div className="skeleton h-4 w-24 mb-3"></div>
              <div className="skeleton h-8 w-32"></div>
            </div>
          ))}
        </div>
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-6 w-40 mb-4"></div>
          <div className="skeleton h-64 w-full"></div>
        </div>
      </div>
    )
  }

  return (
    <div>
      {/* Trader Masthead - Luxury Editorial Style */}
      <div className="mb-8 pb-6 border-b border-white/[0.08] flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <span className="text-[10px] uppercase font-inter font-medium tracking-luxury text-[#7C7A75]">
              ACTIVE AGENT DOSSIER
            </span>
            <span className="text-[#383940]">•</span>
            <span className="text-[10px] font-mono text-[#6E987E] tracking-widest uppercase">
              {status?.is_running ? 'AUTONOMOUS EXECUTING' : 'STANDBY'}
            </span>
          </div>
          
          <div className="flex items-baseline gap-4">
            <h1 className="text-3xl md:text-5xl font-playfair font-normal text-[#ECEBE6] tracking-tight">
              {selectedTrader.trader_name}
            </h1>
            <span className="text-xs font-mono uppercase tracking-widest text-[#7C7A75]">
              [{getModelShowName(selectedTrader.ai_model)}]
            </span>
            {(status?.consecutive_wait || 0) >= 5 && (
              <span className="text-[10px] font-mono uppercase px-2 py-0.5 border border-white/10 text-[#7C7A75] bg-white/[0.02]">
                WAIT {status?.consecutive_wait} CYCLES
              </span>
            )}
          </div>
        </div>

        {/* Right side: Switcher & Telemetry stats */}
        <div className="flex items-center gap-4 text-xs font-mono text-[#7C7A75]">
          {traders && traders.length > 1 && (
            <div className="flex items-center gap-2">
              <span className="text-[10px] font-inter uppercase tracking-wider text-[#5E5D58]">SELECT:</span>
              <select
                value={selectedTraderId}
                onChange={(e: React.ChangeEvent<HTMLSelectElement>) => onTraderSelect(e.target.value)}
                className="bg-[#17181D] border border-white/10 px-3 py-1 text-xs font-mono text-[#ECEBE6] cursor-pointer"
              >
                {traders.map((trader) => (
                  <option key={trader.trader_id} value={trader.trader_id}>
                    {trader.trader_name}
                  </option>
                ))}
              </select>
            </div>
          )}
          {status && (
            <div className="flex items-center gap-3 border-l border-white/10 pl-4 text-[11px]">
              <span>CYCLES: <strong className="text-[#ECEBE6] font-normal">{status.call_count}</strong></span>
              <span>•</span>
              <span>RUNTIME: <strong className="text-[#ECEBE6] font-normal">{status.runtime_minutes}m</strong></span>
            </div>
          )}
        </div>
      </div>
      {/* Risk / Pause Status Alert if active */}
      {status?.risk_halted && (
        <div
          className="mb-6 p-4 border border-[#B86B65]/30 bg-[#B86B65]/5 text-[#B86B65] text-xs font-mono tracking-wider flex items-center justify-between"
        >
          <div>
            ⚠️ {t('riskHalted', language)}
            {status.stop_until ? ` — ${t('haltUntil', language)} ${new Date(status.stop_until).toLocaleString()}` : ''}
            . {t('consecutiveWait', language)}: {status.consecutive_wait ?? 0}
          </div>
        </div>
      )}
      {status?.operator_pause_opens && (
        <div
          className="mb-6 p-4 border border-[#B86B65]/30 bg-[#B86B65]/5 text-[#B86B65] text-xs font-mono tracking-wider flex items-center justify-between"
        >
          <div>
            🛑 {t('operatorBanner', language)}: {status.operator_pause_actor || 'operator'}
            {status.operator_pause_until ? ` — ${t('haltUntil', language)} ${new Date(status.operator_pause_until).toLocaleString()}` : ''}
          </div>
        </div>
      )}

      {/* Live Sync Bar */}
      {account && (
        <div className="mb-6 px-4 py-2 bg-white/[0.02] border border-white/[0.06] text-[11px] font-mono text-[#7C7A75] flex items-center justify-between">
          <div className="flex items-center gap-3">
            <span className="inline-block w-1.5 h-1.5 rounded-full bg-[#6E987E]"></span>
            <span>LAST SYNC: <strong className="text-[#ECEBE6] font-normal">{lastUpdate}</strong></span>
            <span>•</span>
            <span>AUTO-POLL: <strong className="text-[#ECEBE6] font-normal">ACTIVE</strong></span>
          </div>
          <div className="hidden sm:flex items-center gap-4 text-[#5E5D58]">
            <span>EQUITY: {account?.total_equity?.toFixed(2) || '0.00'} USDT</span>
            <span>•</span>
            <span>AVAIL: {account?.available_balance?.toFixed(2) || '0.00'} USDT</span>
          </div>
        </div>
      )}
      {/* Luxury Editorial Minimal Telemetry Strip (Hairline Separators, Zero Thick Boxes) */}
      <div className="border-y border-white/[0.08] bg-[#17181C]/90 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 mb-10 divide-y sm:divide-y-0 sm:divide-x divide-white/[0.08]">
        <StatCard
          title={t('totalEquity', language)}
          value={`${account?.total_equity?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) > 0}
          subtitle={account?.initial_balance ? `${t('initialBalance', language) || '初始本金'}: ${account.initial_balance.toFixed(2)} USDT` : undefined}
        />
        <StatCard
          title={t('availableBalance', language)}
          value={`${account?.available_balance?.toFixed(2) || '0.00'} USDT`}
          subtitle={`${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% ${t('free', language)}`}
        />
        <StatCard
          title={t('totalPnL', language)}
          value={`${account?.total_pnl !== undefined && account.total_pnl >= 0 ? '+' : ''}${account?.total_pnl?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) >= 0}
          subtitle={account?.initial_balance && account?.total_equity ? `${account.total_equity.toFixed(2)} - ${account.initial_balance.toFixed(2)} = ${account.total_pnl?.toFixed(2) || '0.00'}` : undefined}
        />
        <StatCard
          title={t('positions', language)}
          value={`${account?.position_count || 0}`}
          unit={account?.position_count ? 'POSITIONS' : 'ACTIVE'}
          subtitle={`${t('margin', language)}: ${account?.margin_used_pct?.toFixed(1) || '0.0'}% · ${t('dailyPnL', language)}: ${account?.daily_pnl !== undefined ? account.daily_pnl.toFixed(2) : (status?.daily_pnl ?? 0).toFixed(2)}`}
        />
      </div>

      {/* 主要内容区：7:5 黄金非对称比例分屏 */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 mb-6 items-start">
        {/* 左侧：Recent Decisions 核心决策与雷达 (7 列 - 60% 宽度) */}
        <div className="lg:col-span-7 space-y-6">
          {/* Equity Chart */}
          <div className="animate-slide-in" style={{ animationDelay: '0.1s' }}>
            <EquityChart traderId={selectedTrader.trader_id} />
          </div>

          {/* Current Positions */}
          <div
            className="glass-card p-6 animate-slide-in relative overflow-hidden"
            style={{ animationDelay: '0.15s' }}
          >
            {/* Top decorative glow border */}
            <div className="absolute top-0 left-0 w-full h-[1.5px] bg-gradient-to-r from-transparent via-white/10 to-transparent"></div>

            <div className="flex items-center justify-between mb-6">
              <h2
                className="text-lg font-bold flex items-center gap-2.5 font-mono tracking-wide"
                style={{ color: '#EAECEF' }}
              >
                <span className="text-white">⚡</span> {t('currentPositions', language).toUpperCase()}
              </h2>
              {positions && positions.length > 0 && (
                <div className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-white/5 border border-white/10 text-white text-xs font-bold font-mono">
                  <span className="pulse-dot-green"></span>
                  {positions.length} ACTIVE
                </div>
              )}
            </div>
            {positions && positions.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-xs text-left">
                  <thead>
                    <tr className="border-b border-white/5 text-[10px] text-gray-500 font-mono tracking-wider uppercase">
                      <th className="pb-3 font-semibold">{t('symbol', language)}</th>
                      <th className="pb-3 font-semibold">{t('side', language)}</th>
                      <th className="pb-3 font-semibold">{t('entryPrice', language)}</th>
                      <th className="pb-3 font-semibold">{t('markPrice', language)}</th>
                      <th className="pb-3 font-semibold">{t('quantity', language)}</th>
                      <th className="pb-3 font-semibold">{t('positionValue', language)}</th>
                      <th className="pb-3 font-semibold">{t('leverage', language)}</th>
                      <th className="pb-3 font-semibold text-right">{t('unrealizedPnL', language)}</th>
                      <th className="pb-3 font-semibold text-right">{t('liqPrice', language)}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-white/5 font-mono">
                    {positions.map((pos, i) => {
                      const isLong = pos.side === 'long';
                      return (
                        <tr
                          key={i}
                          className="hover:bg-white/[0.02] transition-colors duration-150"
                        >
                          <td className="py-4 font-bold text-gray-200">{pos.symbol}</td>
                          <td className="py-4">
                            <span
                               className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider ${
                                isLong
                                  ? 'bg-[#0ecb81]/8 text-[#0ecb81] border border-[#0ecb81]/15'
                                  : 'bg-[#f6465d]/8 text-[#f6465d] border border-[#f6465d]/15'
                              }`}
                            >
                              <span className={isLong ? 'pulse-dot-green' : 'pulse-dot-red'} style={{ width: 6, height: 6 }}></span>
                              {t(pos.side === 'long' ? 'long' : 'short', language)}
                            </span>
                          </td>
                          <td className="py-4 text-gray-300">{pos.entry_price.toFixed(4)}</td>
                          <td className="py-4 text-gray-300">{pos.mark_price.toFixed(4)}</td>
                          <td className="py-4 text-gray-300">{pos.quantity.toFixed(4)}</td>
                          <td className="py-4 text-gray-300 font-bold">{(pos.quantity * pos.mark_price).toFixed(2)} USDT</td>
                          <td className="py-4 text-white font-bold">{pos.leverage}x</td>
                          <td className="py-4 text-right">
                            <span
                              className={`font-bold text-white`}
                            >
                              {pos.unrealized_pnl >= 0 ? '+' : ''}
                              {pos.unrealized_pnl.toFixed(2)} ({pos.unrealized_pnl_pct.toFixed(2)}%)
                            </span>
                          </td>
                          <td className="py-4 text-right text-gray-500">{pos.liquidation_price.toFixed(4)}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="text-center py-14 text-gray-500 rounded-lg bg-black/10 border border-white/[0.02]">
                <div className="text-4xl mb-3 opacity-25">📁</div>
                <div className="text-sm font-bold font-mono tracking-wider text-gray-400 mb-1">
                  {t('noPositions', language).toUpperCase()}
                </div>
                <div className="text-xs text-gray-600">
                  {t('noActivePositions', language)}
                </div>
              </div>
            )}
          </div>

          {/* External Operator Panel */}
          <div className="animate-slide-in" style={{ animationDelay: '0.2s' }}>
            <OperatorPanel />
          </div>
        </div>
        {/* 左侧结束 */}

        {/* 辅助栏：Recent Decisions - 5 列宽度 */}
        <div
          className="sharp-card bracket-corners p-6 animate-slide-in h-fit lg:col-span-5 lg:sticky lg:top-24 lg:max-h-[calc(100vh-120px)]"
          style={{ animationDelay: '0.2s' }}
        >
          {/* 标题 - Luxury Editorial Header */}
          <div className="flex items-center justify-between mb-6 pb-4 border-b border-white/[0.08]">
            <div className="flex items-baseline gap-3">
              <h2 className="text-2xl font-playfair font-normal tracking-tight text-[#ECEBE6]">
                The <span className="luxury-gold-italic">Decisions</span>
              </h2>
              <span className="text-[9px] uppercase font-inter font-medium tracking-luxury text-[#7C7A75]">
                {decisions && decisions.length > 0 ? `LAST ${decisions.length} CYCLES` : 'ACTIVE POOL'}
              </span>
            </div>
            <div className="flex items-center gap-1.5 font-mono text-[9px] tracking-widest text-[#6E987E] uppercase px-2 py-0.5 border border-[#6E987E]/20 bg-[#6E987E]/5">
              <span className="w-1.5 h-1.5 rounded-full bg-[#6E987E]"></span>
              SYNCED
            </div>
          </div>

          {/* 决策列表 - 可滚动 */}
          <div
            className="space-y-4 overflow-y-auto pr-2"
            style={{ maxHeight: 'calc(100vh - 280px)' }}
          >
            {decisions && decisions.length > 0 ? (
              decisions.map((decision, i) => (
                <DecisionCard key={i} decision={decision} language={language} />
              ))
            ) : (
              <div className="py-16 text-center">
                <div className="text-6xl mb-4 opacity-30">🧠</div>
                <div
                  className="text-lg font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('noDecisionsYet', language)}
                </div>
                <div className="text-sm" style={{ color: '#848E9C' }}>
                  {t('aiDecisionsWillAppear', language)}
                </div>
              </div>
            )}
          </div>
        </div>
        {/* 右侧结束 */}
      </div>

      {/* AI Learning & Performance Analysis */}
      <div className="mb-6 animate-slide-in" style={{ animationDelay: '0.3s' }}>
        <AILearning traderId={selectedTrader.trader_id} />
      </div>
    </div>
  )
}

// Stat Card Component - Luxury Editorial Minimal Gallery Strip
function StatCard({
  title,
  value,
  change,
  positive,
  subtitle,
  unit = 'USDT',
}: {
  title: string
  value: string
  change?: number
  positive?: boolean
  subtitle?: string
  unit?: string | null
}) {
  const pureValue = value.replace(' USDT', '')
  return (
    <div className="p-6 relative bg-transparent flex flex-col justify-between h-full group transition-all duration-300 hover:bg-white/[0.015]">
      <div>
        <div className="text-[10px] uppercase font-inter font-medium tracking-[0.28em] text-[#7C7A75] mb-2.5">
          {title}
        </div>
        <div className="text-3xl lg:text-4xl font-playfair font-normal tracking-tight text-[#ECEBE6]">
          {pureValue}
          {unit && (
            <span className="text-[11px] font-mono uppercase tracking-[0.2em] text-[#7C7A75] ml-1.5">
              {unit}
            </span>
          )}
        </div>
      </div>

      <div className="mt-5 flex items-baseline justify-between pt-3 border-t border-white/[0.04]">
        {change !== undefined ? (
          <div className="flex items-center gap-1.5 font-mono text-xs">
            <span className={positive ? 'text-[#6E987E]' : 'text-[#B86B65]'}>
              {positive ? '↑' : '↓'} {positive ? '+' : ''}{change.toFixed(2)}%
            </span>
          </div>
        ) : <div />}
        {subtitle && (
          <div className="text-[10px] font-mono tracking-wider text-[#5E5D58]">
            {subtitle}
          </div>
        )}
      </div>
    </div>
  )
}

// Decision Card Component with CoT Trace - Hackers Bloomberg Terminal Style
function DecisionCard({
  decision,
  language,
}: {
  decision: DecisionRecord
  language: Language
}) {
  const [showInputPrompt, setShowInputPrompt] = useState(false)
  const [showCoT, setShowCoT] = useState(false)
  const [showAnalysis, setShowAnalysis] = useState(false)
  const [selectedCandidate, setSelectedCandidate] = useState<string | null>(null)

  // Parse decision_json to get full un-truncated reasoning for each symbol
  const parsedDecisionList = useMemo(() => {
    if (!decision.decision_json) return []
    try {
      const parsed = JSON.parse(decision.decision_json)
      return Array.isArray(parsed) ? parsed : [parsed]
    } catch {
      return []
    }
  }, [decision.decision_json])

  const getFullReasoning = (symbol?: string, fallbackReasoning?: string) => {
    if (symbol && parsedDecisionList.length > 0) {
      const found = parsedDecisionList.find((p: any) => p?.symbol === symbol)
      if (found?.reasoning) return found.reasoning
    }
    return fallbackReasoning || ''
  }

  const isAllWait = !decision.decisions || decision.decisions.length === 0 || decision.decisions.every(d => d.action === 'wait' || d.action === 'hold');
  const primaryReasoning = getFullReasoning(decision.decisions?.[0]?.symbol, decision.decisions?.find(d => d.reasoning)?.reasoning) || decision.error_message || '';

  return (
    <div
      className="sharp-card bracket-corners p-6 transition-all duration-300 border border-white/5 relative"
    >
      {/* 右上角極小直角亮點指示 */}
      <div className="absolute top-4 right-4 flex items-center gap-1.5 px-2.5 py-1 border border-white/5 bg-black/50 font-mono text-[9px] tracking-[0.2em] uppercase">
        <span className={`w-1.5 h-1.5 ${decision.success ? 'bg-[#00DC82]' : 'bg-[#F43F5E]'}`}></span>
        <span style={{ color: decision.success ? '#F4F3EE' : '#9E9EA8' }}>
          {decision.success ? 'CYCLE_SUCCESS' : 'CYCLE_FAILURE'}
        </span>
      </div>

      {/* Header */}
      <div className="mb-6 flex items-baseline justify-between border-b border-white/[0.06] pb-4">
        <div>
          <div className="text-xl text-[#ECEBE6] font-playfair flex items-baseline gap-2.5">
            <span className="tracking-tight">Cycle Record <span className="luxury-gold-italic">#{decision.cycle_number}</span></span>
            <span className="text-[9px] font-inter font-medium text-[#7C7A75] tracking-luxury uppercase">[{t('cycle', language)}]</span>
          </div>
          <div className="text-[10px] text-[#5E5D58] font-mono tracking-widest mt-1">
            TIMESTAMP: {new Date(decision.timestamp).toLocaleString()}
          </div>
        </div>
      </div>

      {/* Account State Summary */}
      {decision.account_state && (
        <div className="border border-white/[0.06] bg-transparent p-4 grid grid-cols-2 sm:grid-cols-4 gap-4 text-xs font-mono mb-6 text-[#9C9B96] divide-x divide-white/[0.04]">
          <div className="pl-1">
            <div className="text-[9px] text-[#7C7A75] font-inter uppercase tracking-[0.25em]">NET_WORTH</div>
            <div className="font-playfair font-normal text-[#ECEBE6] mt-1 text-lg">{decision.account_state.total_balance.toFixed(2)} <span className="text-[#5E5D58] text-[10px] font-mono">USDT</span></div>
          </div>
          <div className="pl-3">
            <div className="text-[9px] text-[#7C7A75] font-inter uppercase tracking-[0.25em]">AVAILABLE</div>
            <div className="font-playfair font-normal text-[#ECEBE6] mt-1 text-lg">{decision.account_state.available_balance.toFixed(2)} <span className="text-[#5E5D58] text-[10px] font-mono">USDT</span></div>
          </div>
          <div>
            <div className="text-[9px] text-[#5E5D58] uppercase tracking-[0.2em]">MARGIN_PCT</div>
            <div className="font-medium text-[#ECEBE6] mt-0.5 font-mono text-sm">{decision.account_state.margin_used_pct.toFixed(1)}%</div>
          </div>
          <div>
            <div className="text-[9px] text-[#5E5D58] uppercase tracking-[0.2em]">ACTIVE_POS</div>
            <div className="font-medium text-[#B4B0A5] mt-0.5 font-mono text-sm">{decision.account_state.position_count}</div>
          </div>
        </div>
      )}

      {/* AI Core Thesis / Observation Reason Banner */}
      {isAllWait && (primaryReasoning || parsedDecisionList.length > 0) && (
        <div className="mb-5 p-4 bg-[#131418] border-l-2 border-white/20 border-t border-r border-b border-white/[0.06] space-y-2.5">
          <div className="flex items-center justify-between">
            <span className="text-[10px] font-medium text-[#B4B0A5] tracking-[0.2em] uppercase font-mono">CURATED OBSERVATION DISCIPLINE</span>
            <span className="px-2 py-0.5 bg-white/[0.03] border border-white/10 text-[#9C9B96] text-[9px] font-mono tracking-wider">CAPITAL DISCIPLINE</span>
          </div>
          <div className="space-y-2 text-xs font-mono text-[#C5C4BE] leading-relaxed">
            {parsedDecisionList.length > 0 ? (
              parsedDecisionList.map((p: any, idx: number) => (
                <div key={idx} className="flex items-start gap-2 bg-white/[0.02] p-2.5 border border-white/[0.06]">
                  <span className="font-medium text-[#ECEBE6] shrink-0">[{p.symbol}]:</span>
                  <span className="text-[#C5C4BE]">{p.reasoning || '觀望等待確認信號'}</span>
                </div>
              ))
            ) : (
              <div className="bg-white/[0.02] p-2.5 border border-white/[0.06]">{primaryReasoning}</div>
            )}
          </div>
        </div>
      )}

      {/* Scanned Candidates Radar */}
      {decision.candidate_coins && decision.candidate_coins.length > 0 && (
        <div className="mb-5 bg-[#131418] border border-white/[0.06] p-4 font-mono">
          <div className="flex items-center justify-between mb-3">
            <div className="text-[9px] text-[#5E5D58] uppercase tracking-[0.2em] flex items-center gap-2">
              <span className="text-[#ECEBE6] font-medium">SCANNED ASSETS SPECTRUM</span>
              <span className="px-1.5 py-0.5 bg-white/5 text-[#9C9B96] text-[9px]">
                {decision.candidate_coins.length} ASSETS
              </span>
            </div>
            <span className="text-[9px] text-[#5E5D58] tracking-wider">SELECT ASSET FOR PROFILE</span>
          </div>
          
          {/* Horizontal Chip Bar */}
          <div className="flex flex-wrap gap-2">
            {decision.candidate_coins.map((coin) => {
              const ta = decision.technical_analysis?.[coin];
              const isSelected = selectedCandidate === coin;
              const isBullish = ta?.TrendState === 'Bullish';
              const isBearish = ta?.TrendState === 'Bearish';
              const trendColor = isBullish ? 'bg-[#00E599]' : isBearish ? 'bg-[#FF3B69]' : 'bg-gray-400';
              const hasExplicitDecision = decision.decisions?.some(d => d.symbol === coin);
              
              return (
                <button
                  key={coin}
                  type="button"
                  onClick={() => setSelectedCandidate(isSelected ? null : coin)}
                  className={`sharp-chip px-3 py-2 text-xs font-mono flex items-center gap-2.5 transition-all cursor-pointer ${
                    isSelected ? 'active' : ''
                  }`}
                >
                  <span className={`w-1.5 h-1.5 ${trendColor}`}></span>
                  <span className="font-bold tracking-wider">{coin.replace('USDT', '')}</span>
                  {hasExplicitDecision && (
                    <span className="w-1.5 h-1.5 bg-[#D4AF37]" title="Explicit AI Action Output"></span>
                  )}
                  {ta?.SignalScore !== undefined && (
                    <span className={`text-[10px] px-1 py-0.2 font-semibold ${
                      ta.SignalScore >= 70 ? 'text-[#00DC82] bg-[#00DC82]/10 border border-[#00DC82]/20' : 'text-[#9E9EA8] bg-white/5'
                    }`}>
                      {ta.SignalScore}
                    </span>
                  )}
                </button>
              );
            })}
          </div>

          {/* Candidate Detail Inspector */}
          {selectedCandidate && (
            <div className="mt-4 pt-4 border-t border-white/10">
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-2.5">
                  <span className="font-bold text-sm text-[#D4AF37] font-mono tracking-wider">{selectedCandidate}</span>
                  <span className="text-[9px] text-[#60606C] font-mono uppercase tracking-[0.25em]">ARCHITECTURAL METRICS MATRIX</span>
                </div>
                <button
                  type="button"
                  onClick={() => setSelectedCandidate(null)}
                  className="text-[10px] text-[#9E9EA8] hover:text-[#F4F3EE] px-2.5 py-1 bg-white/5 border border-white/10 hover:border-[#D4AF37]/30 transition-all cursor-pointer font-mono"
                >
                  ✕ CLOSE
                </button>
              </div>

              {(() => {
                const ta = decision.technical_analysis?.[selectedCandidate];
                const pa = decision.price_action?.[selectedCandidate];

                if (!ta && !pa) {
                  return (
                    <div className="text-xs text-amber-400/90 bg-amber-500/10 p-2.5 rounded-lg border border-amber-500/20">
                      ℹ️ {selectedCandidate} 持倉價值或流動性未達系統門檻，已自動過濾（未列入本週期微觀指標計算）。
                    </div>
                  );
                }

                return (
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs">
                    <div className="bg-black/30 p-2.5 rounded-lg border border-white/5">
                      <span className="text-[10px] text-gray-500 block uppercase">Trend</span>
                      <span className={`font-bold ${
                        ta?.TrendState === 'Bullish' ? 'text-[#0ECB81]' : ta?.TrendState === 'Bearish' ? 'text-[#F6465D]' : 'text-gray-300'
                      }`}>
                        {ta?.TrendState || 'Neutral'}
                      </span>
                    </div>
                    <div className="bg-black/30 p-2.5 rounded-lg border border-white/5">
                      <span className="text-[10px] text-gray-500 block uppercase">RSI State</span>
                      <span className={`font-bold ${
                        ta?.RSIState === 'Overbought' ? 'text-[#F6465D]' : ta?.RSIState === 'Oversold' ? 'text-[#0ECB81]' : 'text-gray-300'
                      }`}>
                        {ta?.RSIState || 'Neutral'}
                      </span>
                    </div>
                    <div className="bg-black/30 p-2.5 rounded-lg border border-white/5">
                      <span className="text-[10px] text-gray-500 block uppercase">Volume</span>
                      <span className="font-bold text-gray-300">
                        {ta?.VolumeState || 'Normal'}
                      </span>
                    </div>
                    <div className="bg-black/30 p-2.5 rounded-lg border border-white/5">
                      <span className="text-[10px] text-gray-500 block uppercase">Score</span>
                      <span className={`font-bold ${
                        (ta?.SignalScore || 0) >= 70 ? 'text-[#0ECB81]' : 'text-gray-300'
                      }`}>
                        {ta?.SignalScore !== undefined ? `${ta.SignalScore}/100` : 'N/A'}
                      </span>
                    </div>
                    {ta?.volume_zscore !== undefined && ta.volume_zscore !== 0 && (
                      <div className="bg-black/30 p-2.5 rounded-lg border border-white/5">
                        <span className="text-[10px] text-gray-500 block uppercase">Volume Z-Score</span>
                        <span className={`font-bold ${
                          ta.volume_zscore < -1.5 ? 'text-[#F6465D]' : ta.volume_zscore < 0 ? 'text-amber-400' : 'text-[#0ECB81]'
                        }`}>
                          {ta.volume_zscore.toFixed(2)}
                          <span className="text-[9px] ml-1 font-normal text-gray-400">
                            {ta.volume_zscore < -1.5 ? '(枯竭)' : ta.volume_zscore < 0 ? '(偏冷)' : '(放量)'}
                          </span>
                        </span>
                      </div>
                    )}
                    {pa && (
                      <>
                        <div className="bg-black/30 p-2.5 rounded-lg border border-white/5">
                          <span className="text-[10px] text-gray-500 block uppercase">Candle Type</span>
                          <span className="font-bold text-gray-300 truncate block">{pa.CandleType || 'Doji'}</span>
                        </div>
                        <div className="bg-black/30 p-2.5 rounded-lg border border-white/5">
                          <span className="text-[10px] text-gray-500 block uppercase">Dist to EMA20</span>
                          <span className="font-bold text-gray-300">{pa.DistToEMA20 !== undefined ? `${pa.DistToEMA20.toFixed(2)}%` : '0.00%'}</span>
                        </div>
                        <div className="bg-black/30 p-2.5 rounded-lg border border-white/5">
                          <span className="text-[10px] text-gray-500 block uppercase">Body Ratio</span>
                          <span className="font-bold text-gray-300">{(pa.BodyRatio * 100).toFixed(0)}%</span>
                        </div>
                        <div className="bg-black/30 p-2.5 rounded-lg border border-white/5">
                          <span className="text-[10px] text-gray-500 block uppercase">Wick Ratio</span>
                          <span className="font-bold text-gray-300">{(pa.UpperWickRatio * 100).toFixed(0)}% / {(pa.LowerWickRatio * 100).toFixed(0)}%</span>
                        </div>
                      </>
                    )}
                  </div>
                );
              })()}
            </div>
          )}
        </div>
      )}

      {/* Actions */}
      <div className="space-y-3 mb-5">
        {/* Decisions Actions */}
        {decision.decisions && decision.decisions.length > 0 && (
          <div className="space-y-2">
            <div className="text-[10px] text-gray-500 font-mono uppercase tracking-wider">ACTIONS_DISPATCHED</div>
            {decision.decisions.map((action, j) => {
              const fullReason = getFullReasoning(action.symbol, action.reasoning);
              return (
                <div
                  key={j}
                  className="bg-black/40 border border-white/5 rounded-xl px-3.5 py-2.5 font-mono text-xs space-y-1.5"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2.5">
                      <span className="font-bold text-[#EAECEF] tracking-wider">{action.symbol}</span>
                      {(() => {
                        const isLong = action.action.includes('long') || action.action === 'open_long';
                        const isShort = action.action.includes('short') || action.action === 'open_short';
                        const isWait = action.action === 'wait';
                        const isHold = action.action === 'hold';
                        const badgeColor = isLong
                          ? 'bg-[#00DC82]/10 text-[#00DC82] border-[#00DC82]/30'
                          : isShort
                          ? 'bg-[#FF3B69]/10 text-[#FF3B69] border-[#FF3B69]/30'
                          : isHold
                          ? 'bg-amber-400/10 text-amber-300 border-amber-400/25'
                          : 'bg-white/5 text-[#9C9B96] border-white/10';
                        const icon = isLong ? '▲' : isShort ? '▼' : isWait ? '⏸' : '●';
                        return (
                          <span
                            className={`px-2.5 py-0.5 rounded-none text-[10px] font-bold uppercase tracking-wider font-mono border flex items-center gap-1.5 ${badgeColor}`}
                          >
                            <span>{icon}</span>
                            <span>{action.action}</span>
                          </span>
                        );
                      })()}
                      {action.leverage > 0 && (
                        <span className="text-[#ECEBE6] text-[11px] font-mono px-1.5 py-0.5 bg-white/5 border border-white/10">
                          {action.leverage}x
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-3">
                      {action.price > 0 && (
                        <span className="text-[#7C7A75] font-mono text-[11px]">@{action.price.toFixed(4)}</span>
                      )}
                      <span className={`text-[10px] font-mono font-bold tracking-wider ${action.success ? 'text-[#00DC82]' : 'text-[#FF3B69]'}`}>
                        {action.success ? '✓ DISPATCHED' : '✗ REJECTED'}
                      </span>
                    </div>
                  </div>
                  {fullReason && (
                    <div className="text-[11px] text-[#A6A49E] border-t border-white/[0.06] pt-2 flex items-start gap-2">
                      <span className="text-[#D4AF37] font-semibold shrink-0 uppercase tracking-widest text-[9px] mt-0.5">THESIS:</span>
                      <span className="text-[#C5C4BE] leading-relaxed font-mono">{fullReason}</span>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Input Prompt - Collapsible */}
      {decision.input_prompt && (
        <div className="mb-4">
          <button
            onClick={() => setShowInputPrompt(!showInputPrompt)}
            className="flex items-center gap-2 text-xs font-mono text-gray-400 hover:text-white transition-colors uppercase tracking-wider"
          >
            <span>📥 {t('inputPrompt', language)}</span>
            <span className="text-[10px] text-gray-500">[{showInputPrompt ? t('collapse', language) : t('expand', language)}]</span>
          </button>
          {showInputPrompt && (
            <div className="mt-2 rounded-lg border border-white/10 overflow-hidden shadow-2xl">
              {/* Terminal Title Bar */}
              <div className="flex items-center justify-between px-4 py-2 bg-[#0d0e12] border-b border-white/5 font-mono text-xs text-gray-400">
                <div className="flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-white/10 border border-white/20"></span>
                  <span className="w-2 h-2 rounded-full bg-white/10 border border-white/20"></span>
                  <span className="w-2 h-2 rounded-full bg-white/10 border border-white/20"></span>
                </div>
                <span className="text-[10px] tracking-wider text-[#EAECEF]">SYSTEM_INPUT_PROMPT.md</span>
              </div>
              <div
                className="p-4 text-xs font-mono whitespace-pre-wrap max-h-96 overflow-y-auto bg-[#020203] text-[#EAECEF] border border-white/5 leading-relaxed selection:bg-white/10"
                style={{
                  textShadow: '0 0 4px rgba(255, 255, 255, 0.1)',
                }}
              >
                {decision.input_prompt}
              </div>
            </div>
          )}
        </div>
      )}

      {/* AI Chain of Thought - Collapsible */}
      {decision.cot_trace && (
        <div className="mb-4">
          <button
            onClick={() => setShowCoT(!showCoT)}
            className="flex items-center gap-2 text-xs font-mono text-gray-400 hover:text-white transition-colors uppercase tracking-wider"
          >
            <span>📤 {t('aiThinking', language)}</span>
            <span className="text-[10px] text-gray-500">[{showCoT ? t('collapse', language) : t('expand', language)}]</span>
          </button>
          {showCoT && (
            <div className="mt-2 rounded-lg border border-white/10 overflow-hidden shadow-2xl">
              {/* Terminal Title Bar */}
              <div className="flex items-center justify-between px-4 py-2 bg-[#0d0e12] border-b border-white/5 font-mono text-xs text-gray-400">
                <div className="flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-white/10 border border-white/20"></span>
                  <span className="w-2 h-2 rounded-full bg-white/10 border border-white/20"></span>
                  <span className="w-2 h-2 rounded-full bg-white/10 border border-white/20"></span>
                </div>
                <span className="text-[10px] tracking-wider text-[#EAECEF]">COT_CHAIN_OF_THOUGHT.log</span>
              </div>
              <div
                className="p-4 text-xs font-mono whitespace-pre-wrap max-h-96 overflow-y-auto bg-[#020203] text-[#EAECEF] border border-white/5 leading-relaxed selection:bg-white/10"
                style={{
                  textShadow: '0 0 4px rgba(255, 255, 255, 0.1)',
                }}
              >
                {decision.cot_trace}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Market Analysis - Collapsible */}
      {(decision.technical_analysis || decision.price_action) && (
        <div className="mb-4">
          <button
            onClick={() => setShowAnalysis(!showAnalysis)}
            className="flex items-center gap-2 text-xs font-mono text-gray-400 hover:text-white transition-colors uppercase tracking-wider"
          >
            <span>📊 {t('marketAnalysis', language) || 'Market Analysis'}</span>
            <span className="text-[10px] text-gray-500">[{showAnalysis ? t('collapse', language) : t('expand', language)}]</span>
          </button>
          {showAnalysis && (
            <div className="mt-2 rounded-lg border border-white/10 overflow-hidden shadow-2xl">
              <div className="flex items-center justify-between px-4 py-2 bg-[#0d0e12] border-b border-white/5 font-mono text-xs text-gray-400">
                <div className="flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-white/10 border border-white/20"></span>
                  <span className="w-2 h-2 rounded-full bg-white/10 border border-white/20"></span>
                  <span className="w-2 h-2 rounded-full bg-white/10 border border-white/20"></span>
                </div>
                <span className="text-[10px] tracking-wider text-[#EAECEF]">MARKET_STRUCTURE_METRICS.json</span>
              </div>
              <div
                className="p-4 text-xs space-y-3 bg-[#030712] font-mono text-gray-300"
              >
                {/* Technical Analysis */}
                {decision.technical_analysis &&
                  Object.entries(decision.technical_analysis).map(([symbol, ta]) => (
                    <div key={`ta-${symbol}`} className="border-b border-white/5 last:border-0 pb-3 last:pb-0">
                      <div className="font-bold mb-2 flex items-center gap-2" style={{ color: '#FFFFFF' }}>
                        <span className="pulse-dot-green"></span>
                        {symbol}
                      </div>
                      <div className="grid grid-cols-2 gap-3 text-xs bg-black/30 p-2.5 rounded-lg border border-white/5">
                        <div className="flex justify-between">
                          <span className="text-gray-500">Trend:</span>
                          <span style={{ color: ta.TrendState === 'Bullish' ? '#0ECB81' : ta.TrendState === 'Bearish' ? '#F6465D' : '#EAECEF' }}>
                            {ta.TrendState}
                          </span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">RSI:</span>
                          <span style={{ color: ta.RSIState === 'Overbought' ? '#F6465D' : ta.RSIState === 'Oversold' ? '#0ECB81' : '#EAECEF' }}>
                            {ta.RSIState}
                          </span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">Volume:</span>
                          <span style={{ color: ta.VolumeState === 'High' ? '#FFFFFF' : '#EAECEF' }}>
                            {ta.VolumeState}
                          </span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">Score:</span>
                          <span className="font-bold" style={{ color: ta.SignalScore >= 80 ? '#0ECB81' : ta.SignalScore <= 20 ? '#F6465D' : '#FFFFFF' }}>
                            {ta.SignalScore}/100
                          </span>
                        </div>
                      </div>
                    </div>
                  ))}

                {/* Price Action */}
                {decision.price_action &&
                  Object.entries(decision.price_action).map(([symbol, pa]) => (
                    <div key={`pa-${symbol}`} className="pt-2 flex justify-between items-center text-xs bg-black/30 p-2.5 rounded-lg border border-white/5">
                      <span className="text-gray-500">Candle Pattern:</span>
                      <span className="font-mono font-bold text-[#EAECEF]">{pa.CandleType}</span>
                    </div>
                  ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Candidate Coins Warning */}
      {decision.candidate_coins && decision.candidate_coins.length === 0 && (
        <div
          className="rounded-xl px-4 py-3 mb-4 flex items-start gap-3 glass-card relative overflow-hidden"
          style={{
            background: 'rgba(246, 70, 93, 0.05)',
            border: '1px solid rgba(246, 70, 93, 0.25)',
          }}
        >
          <div className="absolute top-0 bottom-0 left-0 w-[3px] bg-rose-500"></div>
          <AlertTriangle size={18} className="flex-shrink-0 mt-0.5 text-rose-500 animate-pulse" />
          <div className="flex-1">
            <div className="font-semibold text-xs text-rose-400 mb-1">
              ⚠️ {t('candidateCoinsZeroWarning', language)}
            </div>
            <div className="text-[11px] space-y-1 text-gray-400">
              <div>{t('possibleReasons', language)}</div>
              <ul className="list-disc list-inside space-y-0.5 ml-2">
                <li>{t('coinPoolApiNotConfigured', language)}</li>
                <li>{t('apiConnectionTimeout', language)}</li>
                <li>{t('noCustomCoinsAndApiFailed', language)}</li>
              </ul>
              <div className="mt-2 text-gray-300">
                <strong>{t('solutions', language)}</strong>
              </div>
              <ul className="list-disc list-inside space-y-0.5 ml-2">
                <li>{t('setCustomCoinsInConfig', language)}</li>
                <li>{t('orConfigureCorrectApiUrl', language)}</li>
                <li>{t('orDisableCoinPoolOptions', language)}</li>
              </ul>
            </div>
          </div>
        </div>
      )}

      {/* Execution Logs */}
      {decision.execution_log && decision.execution_log.length > 0 && (
        <div className="mt-4 rounded-lg border border-white/10 overflow-hidden shadow-2xl">
          <div className="flex items-center justify-between px-4 py-2 bg-[#0d0e12] border-b border-white/5 font-mono text-xs text-gray-400">
            <div className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-[#ef4444]"></span>
              <span className="w-2 h-2 rounded-full bg-[#f59e0b]"></span>
              <span className="w-2 h-2 rounded-full bg-[#10b981]"></span>
            </div>
            <span className="text-[10px] tracking-wider text-gray-400">CYCLE_EXECUTION_JOURNAL.sh</span>
          </div>
          <div className="p-4 bg-[#030712] font-mono text-xs space-y-1.5 max-h-48 overflow-y-auto">
            {decision.execution_log.map((log, k) => {
              const isSuccess = log.includes('✓') || log.includes('成功') || log.includes('SUCCESS');
              return (
                <div
                  key={k}
                  className="flex items-start gap-1 leading-relaxed"
                  style={{
                    color: isSuccess ? '#0ECB81' : log.includes('❌') || log.includes('失敗') ? '#F6465D' : '#9ca3af',
                  }}
                >
                  <span className="text-gray-600 select-none">$</span>
                  <span>{log}</span>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Error Message */}
      {decision.error_message && (
        <div
          className="rounded-xl px-4 py-3 mt-4 glass-card relative overflow-hidden"
          style={{
            background: 'rgba(246, 70, 93, 0.05)',
            border: '1px solid rgba(246, 70, 93, 0.25)',
          }}
        >
          <div className="absolute top-0 bottom-0 left-0 w-[3px] bg-rose-500"></div>
          <div className="text-xs font-mono text-rose-400">
            ❌ SYSTEM_FATAL_ERROR: {decision.error_message}
          </div>
        </div>
      )}
    </div>
  )
}

// Wrap App with providers
export default function AppWithProviders() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <App />
      </AuthProvider>
    </LanguageProvider>
  )
}
