import { useMemo } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
  Legend,
} from 'recharts'
import useSWR from 'swr'
import { api } from '../lib/api'
import type { CompetitionTraderData } from '../types'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { BarChart3 } from 'lucide-react'

interface ComparisonChartProps {
  traders: CompetitionTraderData[]
}

export function ComparisonChart({ traders }: ComparisonChartProps) {
  const { language } = useLanguage()
  // 获取所有trader的历史数据 - 使用单个useSWR并发请求所有trader数据
  // 生成唯一的key，当traders变化时会触发重新请求
  const tradersKey = traders
    .map((t) => t.trader_id)
    .sort()
    .join(',')

  const { data: allTraderHistories, isLoading } = useSWR(
    traders.length > 0 ? `all-equity-histories-${tradersKey}` : null,
    async () => {
      // 使用批量API一次性获取所有trader的历史数据
      const traderIds = traders.map((trader) => trader.trader_id)
      const batchData = await api.getEquityHistoryBatch(traderIds)

      // ✅ 修正：適配新的資料結構（包含 initial_balance）
      return traders.map((trader) => {
        const traderData = batchData.histories[trader.trader_id]
        if (!traderData) return []
        // 如果是新格式（包含 initial_balance 和 data），返回 data
        if (traderData.data && Array.isArray(traderData.data)) {
          return traderData.data
        }
        // 兼容舊格式
        return traderData
      })
    },
    {
      refreshInterval: 30000, // 30秒刷新（对比图表数据更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  // 将数据转换为与原格式兼容的结构
  const traderHistories = useMemo(() => {
    if (!allTraderHistories) {
      return traders.map(() => ({ data: undefined }))
    }
    return allTraderHistories.map((data) => ({ data }))
  }, [allTraderHistories, traders.length])

  // 使用useMemo自动处理数据合并，直接使用data对象作为依赖
  const combinedData = useMemo(() => {
    // 等待所有数据加载完成
    const allLoaded = traderHistories.every((h) => h.data)
    if (!allLoaded) return []

    console.log(`[${new Date().toISOString()}] Recalculating chart data...`)

    // 新方案：按时间戳分组，不再依赖 cycle_number（因为后端会重置）
    // 收集所有时间戳
    const timestampMap = new Map<
      string,
      {
        timestamp: string
        time: string
        traders: Map<string, { pnl_pct: number; equity: number }>
      }
    >()

    traderHistories.forEach((history, index) => {
      const trader = traders[index]
      if (!history.data) return

      console.log(
        `Trader ${trader.trader_id}: ${history.data.length} data points`
      )

      history.data.forEach((point: any) => {
        const ts = point.timestamp

        if (!timestampMap.has(ts)) {
          const time = new Date(ts).toLocaleTimeString('zh-CN', {
            hour: '2-digit',
            minute: '2-digit',
          })
          timestampMap.set(ts, {
            timestamp: ts,
            time,
            traders: new Map(),
          })
        }

        // 计算盈亏百分比：从total_pnl和balance计算
        // 假设初始余额 = balance - total_pnl
        const initialBalance = point.balance - point.total_pnl
        const pnlPct =
          initialBalance > 0 ? (point.total_pnl / initialBalance) * 100 : 0

        timestampMap.get(ts)!.traders.set(trader.trader_id, {
          pnl_pct: pnlPct,
          equity: point.total_equity,
        })
      })
    })

    // 按时间戳排序，转换为数组
    const combined = Array.from(timestampMap.entries())
      .sort(([tsA], [tsB]) => new Date(tsA).getTime() - new Date(tsB).getTime())
      .map(([ts, data], index) => {
        const entry: any = {
          index: index + 1, // 使用序号代替cycle
          time: data.time,
          timestamp: ts,
        }

        traders.forEach((trader) => {
          const traderData = data.traders.get(trader.trader_id)
          if (traderData) {
            entry[`${trader.trader_id}_pnl_pct`] = traderData.pnl_pct
            entry[`${trader.trader_id}_equity`] = traderData.equity
          }
        })

        return entry
      })

    // ✅ 核心防禦與視覺優化：為多引擎對比圖插值一個初始起跑點 (START 基準)
    // 所有對比引擎在起跑點的 ROI 均為 0.00%，並且以其 initial_balance 作為起點，
    // 這使得對比折線圖能完美畫出從 Break Even 0% 往外波動發散的完整過程，解決「無聊死水平線」或單點無法繪圖的 Bug！
    if (combined.length > 0) {
      const startEntry: any = {
        index: 0,
        time: 'START',
        timestamp: 'START_TIME',
      }
      
      traders.forEach((trader) => {
        // 從第一個 combined 元素中推斷 initial_balance
        // initial_balance = equity / (1 + pnl_pct/100)
        const firstPointEquity = combined[0][`${trader.trader_id}_equity`]
        const firstPointPnlPct = combined[0][`${trader.trader_id}_pnl_pct`]
        
        let initialBal = 1000 // 預設防禦
        if (firstPointEquity !== undefined && firstPointPnlPct !== undefined) {
          const safePct = 1 + (firstPointPnlPct / 100)
          initialBal = safePct > 0 ? (firstPointEquity / safePct) : firstPointEquity
        }
        
        startEntry[`${trader.trader_id}_pnl_pct`] = 0.0
        startEntry[`${trader.trader_id}_equity`] = initialBal
      })
      
      combined.unshift(startEntry)
    }

    if (combined.length > 0) {
      const lastPoint = combined[combined.length - 1]
      console.log(
        `Chart: ${combined.length} data points, last time: ${lastPoint.time}, timestamp: ${lastPoint.timestamp}`
      )
    }

    return combined;
  }, [allTraderHistories, traders])

  if (isLoading) {
    return (
      <div className="text-center py-16" style={{ color: '#848E9C' }}>
        <div className="spinner mx-auto mb-4"></div>
        <div className="text-sm font-semibold">Loading comparison data...</div>
      </div>
    )
  }

  if (combinedData.length === 0) {
    return (
      <div className="text-center py-16" style={{ color: '#848E9C' }}>
        <BarChart3 className="w-12 h-12 mx-auto mb-4 opacity-60" />
        <div className="text-lg font-semibold mb-2">
          {t('noHistoricalData', language)}
        </div>
        <div className="text-sm">{t('dataWillAppear', language)}</div>
      </div>
    )
  }

  // 限制显示数据点
  const MAX_DISPLAY_POINTS = 2000
  const displayData =
    combinedData.length > MAX_DISPLAY_POINTS
      ? combinedData.slice(-MAX_DISPLAY_POINTS)
      : combinedData

  // 计算Y轴范围
  const calculateYDomain = () => {
    const allValues: number[] = []
    displayData.forEach((point) => {
      traders.forEach((trader) => {
        const value = point[`${trader.trader_id}_pnl_pct`]
        if (value !== undefined) {
          allValues.push(value)
        }
      })
    })

    if (allValues.length === 0) return [-5, 5]

    const minVal = Math.min(...allValues)
    const maxVal = Math.max(...allValues)
    const range = Math.max(Math.abs(maxVal), Math.abs(minVal))
    const padding = Math.max(range * 0.2, 1) // 至少留1%余量

    return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
  }

  // 使用 AETHERIS QUANTUM v5.0 消光鈦金灰度色板（Stealth Monochromatic Palette）
  const traderColor = (traderId: string) => {
    const index = traders.findIndex((t) => t.trader_id === traderId)
    // 高雅灰度序列：純白、鈦銀、月灰、消光灰、炭黑
    const grayPalette = ['#ffffff', '#cccccc', '#9e9e9e', '#757575', '#494949']
    return grayPalette[index % grayPalette.length] || '#ffffff'
  }

  // 自定義Tooltip - Binance Style
  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      return (
        <div
          className="glass-card p-3.5 shadow-2xl relative overflow-hidden min-w-[170px] border border-white/10"
          style={{ background: 'rgba(10, 10, 12, 0.95)', backdropFilter: 'blur(12px)' }}
        >
          <div className="text-[10px] font-bold font-mono text-gray-500 mb-2 uppercase tracking-wider">
            {data.time} - SYNC #{data.index}
          </div>
          {traders.map((trader) => {
            const pnlPct = data[`${trader.trader_id}_pnl_pct`]
            const equity = data[`${trader.trader_id}_equity`]
            if (pnlPct === undefined) return null

            return (
              <div key={trader.trader_id} className="mb-2 last:mb-0 font-mono text-xs">
                <div
                  className="font-bold mb-0.5"
                  style={{ color: traderColor(trader.trader_id) }}
                >
                  ● {trader.trader_name}
                </div>
                <div
                  className="font-bold flex items-center gap-1.5"
                  style={{ color: pnlPct >= 0 ? '#ffffff' : '#8e8e93' }}
                >
                  <span>{pnlPct >= 0 ? '▲' : '▼'} {pnlPct >= 0 ? '+' : ''}{pnlPct.toFixed(2)}%</span>
                  <span className="text-[10px] font-normal text-gray-400">
                    ({equity?.toFixed(0)} USDT)
                  </span>
                </div>
              </div>
            )
          })}
        </div>
      )
    }
    return null
  }

  // 计算当前差距
  const currentGap =
    displayData.length > 0
      ? (() => {
          const lastPoint = displayData[displayData.length - 1]
          const values = traders.map(
            (t) => lastPoint[`${t.trader_id}_pnl_pct`] || 0
          )
          return Math.abs(values[0] - values[1])
        })()
      : 0

  return (
    <div>
      <div
        className="my-2 relative overflow-hidden rounded-xl bg-black/10 border border-white/[0.02]"
        style={{
          borderRadius: '8px',
          overflow: 'hidden',
          position: 'relative',
        }}
      >
        {/* Tech Grid Backdrop */}
        <div className="absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.002)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.002)_1px,transparent_1px)] bg-[size:24px_24px] pointer-events-none opacity-30"></div>

        {/* AETHERIS QUANTUM Watermark */}
        <div
          style={{
            position: 'absolute',
            top: '20px',
            right: '20px',
            fontSize: '12px',
            fontWeight: 'bold',
            color: 'rgba(255, 255, 255, 0.03)',
            zIndex: 10,
            pointerEvents: 'none',
            fontFamily: 'monospace',
            letterSpacing: '0.15em'
          }}
        >
          AETHERIS QUANTUM TELEMETRY
        </div>
        <ResponsiveContainer width="100%" height={520}>
          <LineChart
            data={displayData}
            margin={{ top: 20, right: 30, left: 20, bottom: 40 }}
          >
            <defs>
              {/* Monochromatic Subtle Gradients - No neon colors */}
            </defs>

            {/* TradingView Style Clean Grims */}
            <CartesianGrid strokeDasharray="5 5" stroke="rgba(255, 255, 255, 0.02)" vertical={false} />

            <XAxis
              dataKey="time"
              stroke="rgba(255, 255, 255, 0.08)"
              tick={{ fill: 'rgba(255, 255, 255, 0.25)', fontSize: 10, fontFamily: 'monospace' }}
              tickLine={{ stroke: 'rgba(255, 255, 255, 0.03)' }}
              interval={Math.floor(displayData.length / 12)}
              angle={-15}
              textAnchor="end"
              height={60}
            />

            <YAxis
              stroke="rgba(255, 255, 255, 0.08)"
              tick={{ fill: 'rgba(255, 255, 255, 0.25)', fontSize: 10, fontFamily: 'monospace' }}
              tickLine={{ stroke: 'rgba(255, 255, 255, 0.03)' }}
              domain={calculateYDomain()}
              tickFormatter={(value) => `${value.toFixed(1)}%`}
              width={60}
            />

            <Tooltip content={<CustomTooltip />} />

            <ReferenceLine
              y={0}
              stroke="rgba(255, 255, 255, 0.1)"
              strokeDasharray="5 5"
              strokeWidth={1.2}
              label={{
                value: 'Break Even',
                fill: 'rgba(255, 255, 255, 0.2)',
                fontSize: 10,
                fontFamily: 'monospace',
                position: 'right',
              }}
            />

            {traders.map((trader) => (
              <Line
                key={trader.trader_id}
                type="monotone"
                dataKey={`${trader.trader_id}_pnl_pct`}
                stroke={traderColor(trader.trader_id)}
                strokeWidth={1.5}
                strokeOpacity={0.85}
                dot={
                  displayData.length < 50
                    ? { fill: traderColor(trader.trader_id), r: 2 }
                    : false
                }
                activeDot={{
                  r: 4,
                  fill: '#ffffff',
                  stroke: '#020203',
                  strokeWidth: 1.5,
                }}
                name={trader.trader_name}
                connectNulls
                isAnimationActive={false}
              />
            ))}

            <Legend
              wrapperStyle={{ paddingTop: '20px' }}
              iconType="line"
              formatter={(value, entry: any) => {
                // If line has no name or empty, do not render a phantom legend item
                if (!value && !entry?.dataKey) return null;

                const matchedTrader = traders.find(
                  (t) => value === t.trader_name || entry?.dataKey === `${t.trader_id}_pnl_pct`
                );

                if (!matchedTrader) {
                  return value ? <span className="font-mono text-xs font-semibold">{value}</span> : null;
                }

                const modelTag = matchedTrader.ai_model ? matchedTrader.ai_model.split('_').pop()?.toUpperCase() : '';
                return (
                  <span
                    className="font-mono"
                    style={{
                      color: entry.color || '#ECEBE6',
                      fontWeight: 600,
                      fontSize: '12px',
                    }}
                  >
                    {matchedTrader.trader_name} {modelTag ? `(${modelTag})` : ''}
                  </span>
                );
              }}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* Stats */}
      <div
        className="mt-6 grid grid-cols-2 md:grid-cols-4 gap-3.5 pt-5 text-xs font-mono"
        style={{ borderTop: '1px solid rgba(255,255,255,0.05)' }}
      >
        <div
          className="p-3 rounded-lg bg-white/[0.01] border border-white/[0.03] transition-all hover:bg-white/[0.02]"
          style={{ borderLeft: '2px solid rgba(255, 255, 255, 0.2)' }}
        >
          <div
            className="text-[10px] mb-1.5 uppercase tracking-wider text-gray-500"
          >
            {t('comparisonMode', language)}
          </div>
          <div
            className="font-bold text-gray-200 text-sm"
          >
            PnL %
          </div>
        </div>
        <div
          className="p-3 rounded-lg bg-white/[0.01] border border-white/[0.03] transition-all hover:bg-white/[0.02]"
          style={{ borderLeft: '2px solid rgba(255, 255, 255, 0.1)' }}
        >
          <div
            className="text-[10px] mb-1.5 uppercase tracking-wider text-gray-500"
          >
            {t('dataPoints', language)}
          </div>
          <div
            className="font-bold text-gray-200 text-sm"
          >
            {t('count', language, { count: combinedData.length })}
          </div>
        </div>
        <div
          className="p-3 rounded-lg bg-white/[0.01] border border-white/[0.03] transition-all hover:bg-white/[0.02]"
          style={{ borderLeft: '2px solid rgba(255, 255, 255, 0.15)' }}
        >
          <div
            className="text-[10px] mb-1.5 uppercase tracking-wider text-gray-500"
          >
            {t('currentGap', language)}
          </div>
          <div
            className="font-bold text-sm text-white"
          >
            {currentGap.toFixed(2)}%
          </div>
        </div>
        <div
          className="p-3 rounded-lg bg-white/[0.01] border border-white/[0.03] transition-all hover:bg-white/[0.02]"
          style={{ borderLeft: '2px solid rgba(255, 255, 255, 0.1)' }}
        >
          <div
            className="text-[10px] mb-1.5 uppercase tracking-wider text-gray-500"
          >
            {t('displayRange', language)}
          </div>
          <div
            className="font-bold text-gray-200 text-sm"
          >
            {combinedData.length > MAX_DISPLAY_POINTS
              ? `${t('recent', language)} ${MAX_DISPLAY_POINTS}`
              : t('allData', language)}
          </div>
        </div>
      </div>
    </div>
  )
}
