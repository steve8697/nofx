import React, { useMemo, useEffect, useState, useCallback } from 'react'
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
import { api } from '../../lib/api'
import type { TraderInfo } from '../../lib/api'
import { BarChart3, Loader2 } from 'lucide-react'

interface ComparisonChartProps {
  traders: TraderInfo[]
}

export function ComparisonChart({ traders }: ComparisonChartProps) {
  const [histories, setHistories] = useState<Record<string, any[]>>({})
  const [loading, setLoading] = useState(false)

  const fetchHistories = useCallback(async () => {
    if (traders.length === 0) return
    try {
      const traderIds = traders.map(t => t.trader_id)
      const batchData = await api.getEquityHistoryBatch(traderIds)
      
      const newHistories: Record<string, any[]> = {}
      traders.forEach(trader => {
        const traderData = batchData.histories?.[trader.trader_id] || batchData[trader.trader_id]
        if (traderData) {
          if (traderData.data && Array.isArray(traderData.data)) {
            newHistories[trader.trader_id] = traderData.data
          } else if (Array.isArray(traderData)) {
            newHistories[trader.trader_id] = traderData
          }
        }
      })
      setHistories(newHistories)
    } catch (err) {
      console.error('Failed to fetch equity histories in ComparisonChart:', err)
    }
  }, [traders])

  useEffect(() => {
    let mounted = true
    if (mounted) setLoading(true)
    fetchHistories().finally(() => {
      if (mounted) setLoading(false)
    })

    const timer = setInterval(fetchHistories, 30000)
    return () => {
      mounted = false
      clearInterval(timer)
    }
  }, [fetchHistories])

  // Process and combine data for Recharts
  const combinedData = useMemo(() => {
    const allLoaded = traders.every(t => histories[t.trader_id])
    if (!allLoaded) return []

    const timestampMap = new Map<
      string,
      {
        timestamp: string
        time: string
        traders: Map<string, { pnl_pct: number; equity: number }>
      }
    >()

    traders.forEach(trader => {
      const data = histories[trader.trader_id]
      if (!data) return

      data.forEach((point: any) => {
        const ts = point.timestamp
        if (!ts) return

        if (!timestampMap.has(ts)) {
          const time = new Date(ts).toLocaleTimeString('zh-TW', {
            hour: '2-digit',
            minute: '2-digit',
          })
          timestampMap.set(ts, {
            timestamp: ts,
            time,
            traders: new Map(),
          })
        }

        // Calculate PnL Pct
        const initialBalance = point.balance - point.total_pnl
        const pnlPct = initialBalance > 0 ? (point.total_pnl / initialBalance) * 100 : 0

        timestampMap.get(ts)!.traders.set(trader.trader_id, {
          pnl_pct: pnlPct,
          equity: point.total_equity || point.balance,
        })
      })
    })

    // Sort by timestamp and construct array
    const combined = Array.from(timestampMap.entries())
      .sort(([tsA], [tsB]) => new Date(tsA).getTime() - new Date(tsB).getTime())
      .map(([ts, data], index) => {
        const entry: any = {
          index: index + 1,
          time: data.time,
          timestamp: ts,
        }

        traders.forEach(trader => {
          const traderData = data.traders.get(trader.trader_id)
          if (traderData) {
            entry[`${trader.trader_id}_pnl_pct`] = traderData.pnl_pct
            entry[`${trader.trader_id}_equity`] = traderData.equity
          }
        })

        return entry
      })

    // Add Break Even Starting Base
    if (combined.length > 0) {
      const startEntry: any = {
        index: 0,
        time: 'START',
        timestamp: 'START_TIME',
      }
      
      traders.forEach(trader => {
        const firstPointEquity = combined[0][`${trader.trader_id}_equity`]
        const firstPointPnlPct = combined[0][`${trader.trader_id}_pnl_pct`]
        
        let initialBal = 1000
        if (firstPointEquity !== undefined && firstPointPnlPct !== undefined) {
          const safePct = 1 + (firstPointPnlPct / 100)
          initialBal = safePct > 0 ? (firstPointEquity / safePct) : firstPointEquity
        }
        
        startEntry[`${trader.trader_id}_pnl_pct`] = 0.0
        startEntry[`${trader.trader_id}_equity`] = initialBal
      })
      
      combined.unshift(startEntry)
    }

    return combined
  }, [histories, traders])

  if (loading && combinedData.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-[var(--text-muted)] font-mono text-xs">
        <Loader2 className="w-6 h-6 animate-spin text-[var(--accent)] mb-3" />
        <div>同步多引擎量化對比走勢中...</div>
      </div>
    )
  }

  if (combinedData.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20 border border-[var(--border)] rounded-sm bg-[var(--bg-panel)]/30 font-mono text-xs">
        <BarChart3 className="w-10 h-10 text-[var(--text-dim)] mb-3 opacity-60" />
        <div className="text-[var(--text-muted)] font-bold mb-1">暫無引擎對比遙測數據</div>
        <div className="text-[var(--text-dim)] px-8 text-center leading-normal">
          當多個 AI 交易員在市場中產生交易決策時，實時 ROI 對比折線圖將會在此處智能繪製。
        </div>
      </div>
    )
  }

  const MAX_DISPLAY_POINTS = 1000
  const displayData = combinedData.length > MAX_DISPLAY_POINTS
    ? combinedData.slice(-MAX_DISPLAY_POINTS)
    : combinedData

  const calculateYDomain = () => {
    const allValues: number[] = []
    displayData.forEach(point => {
      traders.forEach(trader => {
        const value = point[`${trader.trader_id}_pnl_pct`]
        if (value !== undefined) allValues.push(value)
      })
    })

    if (allValues.length === 0) return [-5, 5]
    const minVal = Math.min(...allValues)
    const maxVal = Math.max(...allValues)
    const range = Math.max(Math.abs(maxVal), Math.abs(minVal))
    const padding = Math.max(range * 0.25, 2)
    return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
  }

  // Titanium Monochromatic Gray Palette
  const getTraderColor = (traderId: string) => {
    const index = traders.findIndex(t => t.trader_id === traderId)
    const grayPalette = ['#ffffff', '#a8a29e', '#78716c', '#57534e', '#44403c']
    return grayPalette[index % grayPalette.length] || '#ffffff'
  }

  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      return (
        <div className="p-4 border border-[var(--border)] bg-[#0f1115]/95 backdrop-blur-md shadow-2xl rounded-sm font-mono text-xs">
          <div className="text-[10px] text-[var(--text-dim)] mb-2.5 font-bold uppercase tracking-widest border-b border-[var(--border)] pb-1.5">
            {data.time === 'START' ? 'INITIAL STATUS' : `DECISION SYNC #${data.index}`}
          </div>
          <div className="space-y-2.5 max-h-48 overflow-y-auto pr-1">
            {traders.map(trader => {
              const pnlPct = data[`${trader.trader_id}_pnl_pct`]
              const equity = data[`${trader.trader_id}_equity`]
              if (pnlPct === undefined) return null

              return (
                <div key={trader.trader_id}>
                  <div className="flex items-center gap-1.5 font-bold" style={{ color: getTraderColor(trader.trader_id) }}>
                    <span className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: getTraderColor(trader.trader_id) }}></span>
                    {trader.trader_name}
                  </div>
                  <div className="flex items-center justify-between gap-6 pl-3 mt-0.5 text-[10px]">
                    <span className={pnlPct >= 0 ? 'text-[var(--green)]' : 'text-[var(--red)]'}>
                      {pnlPct >= 0 ? '▲ +' : '▼ '}{pnlPct.toFixed(2)}%
                    </span>
                    <span className="text-[var(--text-muted)]">({equity?.toFixed(2)} USDT)</span>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )
    }
    return null
  }

  const currentGap = displayData.length > 0
    ? (() => {
        const lastPoint = displayData[displayData.length - 1]
        const values = traders.map(t => lastPoint[`${t.trader_id}_pnl_pct`] || 0)
        return Math.max(...values) - Math.min(...values)
      })()
    : 0

  return (
    <div className="space-y-4">
      <div className="relative border border-[var(--border)] bg-[var(--bg-panel)]/30 rounded-sm overflow-hidden p-4">
        {/* Hologram lines background */}
        <div className="absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.0015)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.0015)_1px,transparent_1px)] bg-[size:32px_32px] pointer-events-none opacity-20"></div>

        <div className="absolute top-4 right-4 text-[9px] font-mono font-bold text-white/5 tracking-[0.2em] pointer-events-none select-none">
          KRONOS TELEMETRY v5.0.2
        </div>

        <ResponsiveContainer width="100%" height={360}>
          <LineChart data={displayData} margin={{ top: 20, right: 20, left: 0, bottom: 10 }}>
            <CartesianGrid strokeDasharray="5 5" stroke="rgba(255,255,255,0.015)" vertical={false} />
            
            <XAxis
              dataKey="time"
              stroke="var(--border-subtle)"
              tick={{ fill: 'var(--text-muted)', fontSize: 9, fontFamily: 'monospace' }}
              tickLine={{ stroke: 'rgba(255,255,255,0.03)' }}
              interval={Math.floor(displayData.length / 10)}
            />

            <YAxis
              stroke="var(--border-subtle)"
              tick={{ fill: 'var(--text-muted)', fontSize: 9, fontFamily: 'monospace' }}
              tickLine={{ stroke: 'rgba(255,255,255,0.03)' }}
              domain={calculateYDomain()}
              tickFormatter={v => `${v.toFixed(1)}%`}
              width={45}
            />

            <Tooltip content={<CustomTooltip />} />

            <ReferenceLine
              y={0}
              stroke="rgba(255,255,255,0.1)"
              strokeDasharray="4 4"
              label={{
                value: 'BREAK EVEN',
                fill: 'rgba(255,255,255,0.15)',
                fontSize: 8,
                fontFamily: 'monospace',
                position: 'right'
              }}
            />

            {traders.map(t => (
              <React.Fragment key={t.trader_id}>
                {/* Underglow line */}
                <Line
                  type="monotone"
                  dataKey={`${t.trader_id}_pnl_pct`}
                  stroke={getTraderColor(t.trader_id)}
                  strokeWidth={4}
                  strokeOpacity={0.02}
                  dot={false}
                  activeDot={false}
                  connectNulls
                />
                {/* Technical hairline */}
                <Line
                  type="monotone"
                  dataKey={`${t.trader_id}_pnl_pct`}
                  stroke={getTraderColor(t.trader_id)}
                  strokeWidth={1.5}
                  strokeOpacity={0.8}
                  dot={displayData.length < 30 ? { fill: getTraderColor(t.trader_id), r: 2 } : false}
                  activeDot={{ r: 4, fill: '#ffffff', stroke: getTraderColor(t.trader_id), strokeWidth: 1 }}
                  name={t.trader_name}
                  connectNulls
                />
              </React.Fragment>
            ))}

            <Legend
              wrapperStyle={{ paddingTop: '15px' }}
              iconType="circle"
              iconSize={8}
              formatter={(value, entry: any) => {
                const trader = traders.find(t => t.trader_name === value)
                return (
                  <span className="font-mono text-[10px] font-bold tracking-wider" style={{ color: entry.color }}>
                    {value} ({trader?.ai_model.split('/').pop()?.split(':').pop() || 'AGENT'})
                  </span>
                )
              }}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* Grid Diagnostics stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 pt-4 border-t border-[var(--border)] font-mono text-[10px] text-[var(--text-muted)]">
        <div className="p-3 border border-[var(--border)] rounded-sm bg-[#161a21]/15">
          <div className="text-[var(--text-dim)] mb-1">對比分析模式</div>
          <div className="text-white text-xs font-bold">收益率 ROI (%)</div>
        </div>
        <div className="p-3 border border-[var(--border)] rounded-sm bg-[#161a21]/15">
          <div className="text-[var(--text-dim)] mb-1">採樣數據點</div>
          <div className="text-white text-xs font-bold">{combinedData.length} 筆決策同步</div>
        </div>
        <div className="p-3 border border-[var(--border)] rounded-sm bg-[#161a21]/15">
          <div className="text-[var(--text-dim)] mb-1">最優/最差引擎差距</div>
          <div className="text-white text-xs font-bold">{currentGap.toFixed(2)} %</div>
        </div>
        <div className="p-3 border border-[var(--border)] rounded-sm bg-[#161a21]/15">
          <div className="text-[var(--text-dim)] mb-1">遙測分析狀態</div>
          <div className="text-[var(--green)] text-xs font-bold flex items-center gap-1">
            <span className="w-1.5 h-1.5 rounded-full bg-[var(--green)] animate-ping"></span>
            ACTIVE MONITORING
          </div>
        </div>
      </div>
    </div>
  )
}
