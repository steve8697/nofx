import { useState } from 'react'
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
} from 'recharts'
import useSWR from 'swr'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import {
  AlertTriangle,
  DollarSign,
  Percent,
  TrendingUp as ArrowUp,
  TrendingDown as ArrowDown,
  Activity,
  Database,
  RefreshCw,
  Wallet
} from 'lucide-react'

interface EquityPoint {
  timestamp: string
  total_equity: number
  pnl: number
  pnl_pct: number
  cycle_number: number
}

interface EquityChartProps {
  traderId?: string
}

export function EquityChart({ traderId }: EquityChartProps) {
  const { language } = useLanguage()
  const [displayMode, setDisplayMode] = useState<'dollar' | 'percent'>('dollar')

  const { data: history, error, mutate: mutateHistory } = useSWR<EquityPoint[]>(
    traderId ? `equity-history-${traderId}` : 'equity-history',
    () => api.getEquityHistory(traderId),
    {
      refreshInterval: 15000, // 縮短至15秒刷新
      revalidateOnFocus: true,
      dedupingInterval: 5000,
    }
  )

  const { data: account, mutate: mutateAccount } = useSWR(
    traderId ? `account-${traderId}` : 'account',
    () => api.getAccount(traderId),
    {
      refreshInterval: 15000,
      revalidateOnFocus: true,
      dedupingInterval: 5000,
    }
  )

  const handleRefresh = async () => {
    await Promise.all([mutateHistory(), mutateAccount()])
  }

  // 錯誤狀態的 Premium 視覺展現
  if (error) {
    return (
      <div className="glass-card p-6 relative overflow-hidden">
        <div className="absolute top-0 left-0 w-full h-[1.5px] bg-gradient-to-r from-transparent via-[#f6465d]/40 to-transparent"></div>
        <div className="flex items-start gap-4 p-4 rounded-xl bg-[#f6465d]/5 border border-[#f6465d]/15">
          <AlertTriangle className="w-6 h-6 text-[#f6465d] shrink-0 mt-0.5" />
          <div className="flex-1">
            <div className="font-bold text-sm text-[#f6465d] font-mono tracking-wider">
              {t('loadingError', language).toUpperCase() || 'DATA CORRUPT / CONNECTION ERROR'}
            </div>
            <div className="text-xs text-gray-500 mt-1 font-mono">
              {error.message || 'Network endpoint returned unhealthy response.'}
            </div>
            <button
              onClick={handleRefresh}
              className="mt-3 px-3 py-1.5 rounded-lg text-xs font-bold bg-[#f6465d]/10 hover:bg-[#f6465d]/20 text-[#f6465d] border border-[#f6465d]/15 transition-all flex items-center gap-1.5 font-mono"
            >
              <RefreshCw className="w-3.5 h-3.5" /> RETRY CONNECTION
            </button>
          </div>
        </div>
      </div>
    )
  }

  // 過濾掉無效數據：total_equity 為 0 或小於 1 的點（防止API延遲返回空值損壞圖表）
  const validHistory = history?.filter((point) => point.total_equity > 1) || []

  // ✅ 核心防禦性 Bug 修正：
  // 初始餘額優先採用帳戶設定的 initial_balance。
  // 若其尚未加載完成、為空或 0，則 fallback 採用歷史紀錄中的第一個點的 total_equity。
  // 若歷史紀錄也為空，則 fallback 採用一個合理的預設本金 1000。
  const initialBalance = account?.initial_balance || validHistory[0]?.total_equity || 1000

  // 當無歷史數據時的高科技視覺預覽（Empty State）
  if (validHistory.length === 0) {
    return (
      <div className="glass-card p-6 relative overflow-hidden">
        <div className="absolute top-0 left-0 w-full h-[1.5px] bg-gradient-to-r from-transparent via-white/10 to-transparent"></div>
        <div className="flex items-center justify-between mb-6">
          <h3 className="text-sm font-bold font-mono tracking-wider text-gray-400">
            ⚡ {t('accountEquityCurve', language).toUpperCase()}
          </h3>
          <span className="text-[10px] font-bold font-mono text-white px-2 py-0.5 rounded bg-white/10 border border-white/15 animate-pulse">
            AWAITING FIRST CYCLE
          </span>
        </div>
        <div className="text-center py-16 rounded-xl bg-black/10 border border-white/[0.02] flex flex-col items-center justify-center relative overflow-hidden">
          {/* Cyber grid background */}
          <div className="absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.003)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.003)_1px,transparent_1px)] bg-[size:16px_16px] pointer-events-none opacity-40"></div>
          
          <div className="mb-4 p-4 rounded-full bg-white/[0.02] border border-white/5 relative z-10">
            <Activity className="w-10 h-10 text-gray-600 animate-pulse" />
          </div>
          <div className="text-sm font-bold text-gray-400 font-mono tracking-wider mb-1.5 relative z-10">
            {t('noHistoricalData', language).toUpperCase() || 'NO HISTORICAL DATA DETECTED'}
          </div>
          <div className="text-xs text-gray-600 font-mono max-w-xs mx-auto relative z-10">
            {t('dataWillAppear', language) || 'AI decision engine is generating telemetry. Chart will render once first transaction completes.'}
          </div>
          
          <button 
            onClick={handleRefresh}
            className="mt-5 px-3 py-1.5 rounded-lg text-xs font-bold bg-white/5 hover:bg-white/10 text-gray-400 border border-white/5 transition-all flex items-center gap-1.5 font-mono z-10"
          >
            <RefreshCw className="w-3 h-3" /> FORCE SYNC
          </button>
        </div>
      </div>
    )
  }

  // 限制顯示最近的數據點（防止長期高頻回測導致DOM卡頓）
  const MAX_DISPLAY_POINTS = 1000
  const displayHistory =
    validHistory.length > MAX_DISPLAY_POINTS
      ? validHistory.slice(-MAX_DISPLAY_POINTS)
      : validHistory

  // ✅ 核心防禦性 Bug 修正 2: 跨平台安全解析後端 timestamp "YYYY-MM-DD HH:MM:SS"
  // 直接以空格分割並提取 "HH:MM:SS" 部分，免除所有 Date 物件解析與瀏覽器/Safari 平台相容性問題！
  const parseSafeTime = (timestampStr: string) => {
    if (!timestampStr) return '00:00:00'
    const parts = timestampStr.split(' ')
    if (parts.length > 1) return parts[1]
    return timestampStr || '00:00:00'
  }

  // 轉換數據格式，計算準確損益與百分比
  const mappedData = displayHistory.map((point) => {
    const pnl = point.total_equity - initialBalance
    // 防範除以 0 導致 NaN
    const safeInitial = initialBalance > 0 ? initialBalance : 1
    const pnlPct = ((pnl / safeInitial) * 100).toFixed(2)
    const parsedPct = isNaN(parseFloat(pnlPct)) ? 0.0 : parseFloat(pnlPct)

    return {
      time: parseSafeTime(point.timestamp),
      value: displayMode === 'dollar' ? point.total_equity : parsedPct,
      cycle: point.cycle_number,
      raw_equity: point.total_equity,
      raw_pnl: pnl,
      raw_pnl_pct: parsedPct,
    }
  })

  // ✅ 核心防禦與視覺優化：為淨值曲線插值「初始配置起點 (START)」
  // 這樣即使目前只有 1 個 cycle 的數據，或所有點的數值都一樣，
  // 圖表也能正確描繪出從 initialBalance 躍遷到當前 total_equity 的盈虧曲線，徹底解決「看不到曲線」的 Bug！
  const chartData: any[] = []
  if (mappedData.length > 0) {
    chartData.push({
      time: 'START',
      value: displayMode === 'dollar' ? initialBalance : 0.0,
      cycle: 0,
      raw_equity: initialBalance,
      raw_pnl: 0.0,
      raw_pnl_pct: 0.0,
    })
    chartData.push(...mappedData)
  } else {
    chartData.push(...mappedData)
  }

  // ✅ 核心防禦性 Bug 修正 3: 防止 currentValue 為空時訪問屬性崩潰
  const currentValue = chartData[chartData.length - 1] || { raw_pnl: 0, raw_equity: initialBalance, raw_pnl_pct: 0 }
  const isProfit = currentValue.raw_pnl >= 0

  // 根據盈虧動態決定視覺主色調
  const primaryColor = isProfit ? '#0ecb81' : '#f6465d'

  // 計算Y軸動態範圍（留出上下氣墊區，使曲線更舒展）
  const calculateYDomain = () => {
    const values = chartData.map((d) => d.value)
    const minVal = Math.min(...values)
    const maxVal = Math.max(...values)
    
    if (displayMode === 'percent') {
      const range = Math.max(Math.abs(maxVal), Math.abs(minVal))
      const padding = Math.max(range * 0.25, 0.5) // 最少0.5%餘裕
      return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
    } else {
      const range = maxVal - minVal
      const padding = Math.max(range * 0.2, initialBalance * 0.005) // 最少千分之五餘裕
      return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
    }
  }

  // 自定義高科技浮動 Tooltip 樣式
  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      const isPointProfit = data.raw_pnl >= 0
      return (
        <div className="glass-card p-3.5 shadow-2xl relative overflow-hidden min-w-[160px] border border-white/10">
          {/* Subtle side indicator */}
          <div 
            className="absolute left-0 top-0 bottom-0 w-[3px]"
            style={{ backgroundColor: isPointProfit ? '#0ecb81' : '#f6465d' }}
          ></div>
          <div className="text-[10px] font-bold font-mono text-gray-500 mb-1.5 uppercase tracking-wider">
            CYCLE #{data.cycle}
          </div>
          <div className="font-mono text-xs font-semibold text-gray-400 flex items-center gap-1.5 mb-1">
            <Wallet className="w-3.5 h-3.5 text-gray-500" />
            <span>{data.raw_equity.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} USDT</span>
          </div>
          <div 
            className="text-xs font-bold font-mono flex items-center gap-1"
            style={{ color: isPointProfit ? '#0ecb81' : '#f6465d' }}
          >
            {isPointProfit ? '▲' : '▼'} {isPointProfit ? '+' : ''}{data.raw_pnl.toFixed(2)} USDT ({isPointProfit ? '+' : ''}{data.raw_pnl_pct.toFixed(2)}%)
          </div>
        </div>
      )
    }
    return null
  }

  return (
    <div className="glass-card p-5 animate-slide-in relative overflow-hidden">
      {/* Top decorative glow border - dynamic color! */}
      <div 
        className="absolute top-0 left-0 w-full h-[1.5px] bg-gradient-to-r from-transparent via-white/10 to-transparent transition-all duration-500"
        style={{ 
          background: `linear-gradient(90deg, transparent 0%, ${primaryColor}40 50%, transparent 100%)`
        }}
      ></div>

      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between mb-5 relative z-10">
        <div>
          <div className="flex items-center gap-2 mb-1.5">
            <h3 className="text-sm font-bold font-mono tracking-wider text-gray-400">
              ⚡ {t('accountEquityCurve', language).toUpperCase()}
            </h3>
            <span className="flex h-2 w-2 relative">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75" style={{ backgroundColor: primaryColor }}></span>
              <span className="relative inline-flex rounded-full h-2 w-2" style={{ backgroundColor: primaryColor }}></span>
            </span>
          </div>

          <div className="flex flex-col sm:flex-row sm:items-baseline gap-2 sm:gap-4">
            <span className="text-3xl font-bold font-mono tracking-tight text-white flex items-baseline">
              {(account?.total_equity || currentValue.raw_equity).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
              <span className="text-xs font-bold text-gray-500 ml-1.5 uppercase font-sans">
                USDT
              </span>
            </span>
            <div className="flex items-center gap-2">
              <span
                className="text-xs font-bold font-mono px-2 py-0.5 rounded flex items-center gap-1 transition-all duration-500"
                style={{
                  color: primaryColor,
                  background: `${primaryColor}08`,
                  border: `1px solid ${primaryColor}20`,
                }}
              >
                {isProfit ? (
                  <ArrowUp className="w-3.5 h-3.5" />
                ) : (
                  <ArrowDown className="w-3.5 h-3.5" />
                )}
                {isProfit ? '+' : ''}
                {currentValue.raw_pnl_pct.toFixed(2)}%
              </span>
              <span className="text-xs font-mono text-gray-500">
                ({isProfit ? '+' : ''}{currentValue.raw_pnl.toFixed(2)} USDT)
              </span>
            </div>
          </div>
        </div>

        {/* Display Mode Toggle */}
        <div className="flex rounded-lg p-0.5 bg-black/40 border border-white/[0.04] self-start sm:self-auto font-mono text-xs">
          <button
            onClick={() => setDisplayMode('dollar')}
            className="px-3.5 py-1.5 rounded-md font-bold transition-all duration-300 flex items-center gap-1"
            style={
              displayMode === 'dollar'
                ? {
                    background: `${primaryColor}15`,
                    color: primaryColor,
                    border: `1px solid ${primaryColor}25`,
                  }
                : { background: 'transparent', color: '#848E9C' }
            }
          >
            <DollarSign className="w-3.5 h-3.5" /> FIAT
          </button>
          <button
            onClick={() => setDisplayMode('percent')}
            className="px-3.5 py-1.5 rounded-md font-bold transition-all duration-300 flex items-center gap-1"
            style={
              displayMode === 'percent'
                ? {
                    background: `${primaryColor}15`,
                    color: primaryColor,
                    border: `1px solid ${primaryColor}25`,
                  }
                : { background: 'transparent', color: '#848E9C' }
            }
          >
            <Percent className="w-3 h-3" /> ROI
          </button>
        </div>
      </div>

      {/* ✅ 核心防禦性 Bug 修正 4: 直接給予 ResponsiveContainer 固定的 280px 高度 */}
      {/* 配合相對定位的外層容器，能徹底防護 Recharts ResponsiveContainer 寬高塌陷為 0 像素的經典 Bug，保證曲線絕對可見 */}
      <div 
        className="my-2 relative overflow-hidden rounded-xl bg-black/10 border border-white/[0.02]"
        style={{
          position: 'relative',
          overflow: 'hidden',
        }}
      >
        {/* Tech Grid Backdrop */}
        <div className="absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.002)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.002)_1px,transparent_1px)] bg-[size:24px_24px] pointer-events-none opacity-30"></div>
        
        {/* Subtle holographic logo */}
        <div className="absolute top-4 right-4 text-[13px] font-black font-mono text-white/[0.03] select-none tracking-widest pointer-events-none">
          KRONOS QUANTUM CORE
        </div>

        <ResponsiveContainer width="100%" height={280}>
          <AreaChart
            data={chartData}
            margin={{ top: 15, right: 15, left: -10, bottom: 20 }}
          >
            <defs>
              {/* Monochromatic Premium Obsidian Gradient */}
              <linearGradient id="obsidianGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#ffffff" stopOpacity={0.03} />
                <stop offset="100%" stopColor="#ffffff" stopOpacity={0.0} />
              </linearGradient>
            </defs>

            {/* TradingView style clean grids (no vertical lines) */}
            <CartesianGrid strokeDasharray="5 5" stroke="rgba(255, 255, 255, 0.02)" vertical={false} />

            <XAxis
              dataKey="time"
              stroke="rgba(255, 255, 255, 0.08)"
              tick={{ fill: 'rgba(255, 255, 255, 0.25)', fontSize: 10, fontFamily: 'monospace' }}
              tickLine={{ stroke: 'rgba(255, 255, 255, 0.03)' }}
              interval={Math.max(1, Math.floor(chartData.length / 5))}
              height={40}
              dy={10}
            />

            <YAxis
              stroke="rgba(255, 255, 255, 0.08)"
              tick={{ fill: 'rgba(255, 255, 255, 0.25)', fontSize: 10, fontFamily: 'monospace' }}
              tickLine={{ stroke: 'rgba(255, 255, 255, 0.03)' }}
              domain={calculateYDomain()}
              dx={-5}
              tickFormatter={(value) => {
                if (displayMode === 'dollar') {
                  const values = chartData.map((d) => d.raw_equity)
                  const minVal = Math.min(...values)
                  const maxVal = Math.max(...values)
                  const diff = maxVal - minVal
                  return diff < 10 ? `$${value.toFixed(2)}` : `$${value.toFixed(0)}`
                } else {
                  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
                }
              }}
            />

            <Tooltip content={<CustomTooltip />} />

            {/* Reference Line: Initial Principal */}
            <ReferenceLine
              y={displayMode === 'dollar' ? initialBalance : 0}
              stroke="rgba(255, 255, 255, 0.1)"
              strokeDasharray="4 4"
              label={{
                value: displayMode === 'dollar' ? `START: $${initialBalance.toFixed(0)}` : 'BASE: 0.00%',
                fill: 'rgba(255, 255, 255, 0.2)',
                fontSize: 9,
                fontFamily: 'monospace',
                position: 'insideBottomLeft',
                dy: -6
              }}
            />

            {/* Area Fill - White monochromatic gradient */}
            <Area
              type="monotone"
              dataKey="value"
              stroke="none"
              fillOpacity={1}
              fill="url(#obsidianGradient)"
              dot={false}
              activeDot={false}
              isAnimationActive={false}
            />

            {/* Area Glow Line - Hairline subtle background trace */}
            <Area
              type="monotone"
              dataKey="value"
              fill="none"
              stroke="#ffffff"
              strokeWidth={5}
              strokeOpacity={0.03}
              dot={false}
              activeDot={false}
              isAnimationActive={false}
            />

            {/* Area Core Line - 1.5px Razor Sharp Silver Hairline */}
            <Area
              type="monotone"
              dataKey="value"
              fill="none"
              stroke="#e5e5ea"
              strokeWidth={1.5}
              strokeOpacity={0.85}
              dot={false}
              isAnimationActive={false}
              activeDot={{
                r: 4,
                fill: '#ffffff',
                stroke: '#020203',
                strokeWidth: 2,
              }}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>

      {/* Cyber Stats Footer */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3.5 mt-5 pt-4 border-t border-white/[0.04] font-mono text-xs">
        <div className="p-3 rounded-lg bg-white/[0.01] border border-white/[0.03] relative overflow-hidden transition-all duration-300 hover:border-white/[0.06] hover:bg-white/[0.02]">
          <div className="absolute left-0 top-0 bottom-0 w-[2px] bg-white/20"></div>
          <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-1.5 flex items-center gap-1">
            <Database className="w-3.5 h-3.5 text-gray-600" /> {t('initialBalance', language)}
          </div>
          <div className="font-bold text-gray-200 text-sm">
            {initialBalance.toLocaleString('en-US', { minimumFractionDigits: 2 })} USDT
          </div>
        </div>

        <div className="p-3 rounded-lg bg-white/[0.01] border border-white/[0.03] relative overflow-hidden transition-all duration-300 hover:border-white/[0.06] hover:bg-white/[0.02]">
          <div className="absolute left-0 top-0 bottom-0 w-[2px]" style={{ backgroundColor: primaryColor }}></div>
          <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-1.5 flex items-center gap-1">
            <Activity className="w-3.5 h-3.5 text-gray-600" /> {t('currentEquity', language)}
          </div>
          <div className="font-bold text-gray-200 text-sm">
            {currentValue.raw_equity.toLocaleString('en-US', { minimumFractionDigits: 2 })} USDT
          </div>
        </div>

        <div className="p-3 rounded-lg bg-white/[0.01] border border-white/[0.03] relative overflow-hidden transition-all duration-300 hover:border-white/[0.06] hover:bg-white/[0.02]">
          <div className="absolute left-0 top-0 bottom-0 w-[2px] bg-white/10"></div>
          <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-1.5 flex items-center gap-1">
            <RefreshCw className="w-3.5 h-3.5 text-gray-600" /> {t('historicalCycles', language)}
          </div>
          <div className="font-bold text-gray-200 text-sm">
            {validHistory.length} {t('cycles', language).toUpperCase()}
          </div>
        </div>

        <div className="p-3 rounded-lg bg-white/[0.01] border border-white/[0.03] relative overflow-hidden transition-all duration-300 hover:border-white/[0.06] hover:bg-white/[0.02]">
          <div className="absolute left-0 top-0 bottom-0 w-[2px] bg-white/10"></div>
          <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-1.5 flex items-center gap-1">
            <span>⚙️</span> {t('displayRange', language)}
          </div>
          <div className="font-bold text-gray-200 text-sm">
            {validHistory.length > MAX_DISPLAY_POINTS
              ? `LAST ${MAX_DISPLAY_POINTS}`
              : 'ALL TELEMETRY'}
          </div>
        </div>
      </div>
    </div>
  )
}
