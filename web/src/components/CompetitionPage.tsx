import { useState } from 'react'
import { BarChart3, Zap } from 'lucide-react'
import useSWR from 'swr'
import { api } from '../lib/api'
import type { CompetitionData } from '../types'
import { ComparisonChart } from './ComparisonChart'
import { TraderConfigViewModal } from './TraderConfigViewModal'
import { getTraderColor } from '../utils/traderColors'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'

export function CompetitionPage() {
  const { language } = useLanguage()
  const [selectedTrader, setSelectedTrader] = useState<any>(null)
  const [isModalOpen, setIsModalOpen] = useState(false)

  const { data: competition } = useSWR<CompetitionData>(
    'competition',
    api.getCompetition,
    {
      refreshInterval: 15000,
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  )

  const handleTraderClick = async (traderId: string) => {
    try {
      let traderConfig
      try {
        traderConfig = await api.getTraderConfig(traderId)
      } catch {
        traderConfig = await api.getPublicTraderConfig(traderId)
      }
      setSelectedTrader(traderConfig)
      setIsModalOpen(true)
    } catch (error) {
      console.error('Failed to fetch trader config:', error)
    }
  }

  const closeModal = () => {
    setIsModalOpen(false)
    setSelectedTrader(null)
  }

  if (!competition) {
    return (
      <div className="space-y-6">
        <div className="binance-card p-8 animate-pulse">
          <div className="flex items-center justify-between mb-6">
            <div className="space-y-3 flex-1">
              <div className="skeleton h-8 w-64"></div>
              <div className="skeleton h-4 w-48"></div>
            </div>
            <div className="skeleton h-12 w-32"></div>
          </div>
        </div>
        <div className="binance-card p-6">
          <div className="skeleton h-6 w-40 mb-4"></div>
          <div className="space-y-3">
            <div className="skeleton h-20 w-full rounded"></div>
            <div className="skeleton h-20 w-full rounded"></div>
          </div>
        </div>
      </div>
    )
  }

  // 如果有数据返回但没有交易员，显示空状态
  if (!competition.traders || competition.traders.length === 0) {
    return (
      <div className="space-y-5 animate-fade-in">
        {/* Header */}
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-3 md:gap-0">
          <div className="flex items-center gap-3 md:gap-4">
            <div
              className="w-10 h-10 md:w-12 md:h-12 rounded-xl flex items-center justify-center animate-pulse"
              style={{
                background: 'linear-gradient(135deg, #FFFFFF 0%, #8E8E93 100%)',
                boxShadow: 'none',
              }}
            >
              <BarChart3
                className="w-6 h-6 md:w-7 md:h-7"
                style={{ color: '#000' }}
              />
            </div>
            <div>
              <h1
                className="text-xl md:text-2xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                {t('aiCompetition', language)}
                <span
                  className="text-xs font-normal px-2 py-1 rounded"
                  style={{
                    background: 'rgba(255, 255, 255, 0.05)',
                    color: '#FFFFFF',
                  }}
                >
                  0 {t('traders', language)}
                </span>
              </h1>
              <p className="text-xs" style={{ color: '#848E9C' }}>
                {t('liveBattle', language)}
              </p>
            </div>
          </div>
        </div>

        {/* Empty State */}
        <div className="binance-card p-8 text-center">
          <BarChart3
            className="w-16 h-16 mx-auto mb-4 opacity-40"
            style={{ color: '#848E9C' }}
          />
          <h3 className="text-lg font-bold mb-2" style={{ color: '#EAECEF' }}>
            {t('noTraders', language)}
          </h3>
          <p className="text-sm" style={{ color: '#848E9C' }}>
            {t('createFirstTrader', language)}
          </p>
        </div>
      </div>
    )
  }

  // 按收益率排序
  const sortedTraders = [...competition.traders].sort(
    (a, b) => b.total_pnl_pct - a.total_pnl_pct
  )

  // 找出领先者
  const leader = sortedTraders[0]

  return (
    <div className="space-y-5 animate-fade-in">
      {/* Header - Luxury Gallery Style */}
      <div className="flex flex-col md:flex-row items-start md:items-end justify-between gap-6 pb-8 border-b border-white/[0.08] mb-12">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <span className="text-[10px] uppercase font-inter font-medium tracking-luxury text-[#7C7A75]">
              ALGORITHMIC BENCHMARK ARENA
            </span>
            <span className="text-[#383940]">•</span>
            <span className="text-[10px] font-mono text-[#6E987E] tracking-widest uppercase">
              {competition.count} ACTIVE ENGINES
            </span>
          </div>
          <h1 className="text-3xl md:text-5xl font-playfair font-normal text-[#ECEBE6] tracking-tight">
            The <span className="luxury-gold-italic">Arena</span> & Rankings
          </h1>
          <p className="text-xs font-inter font-light text-[#7C7A75] mt-2 max-w-lg leading-relaxed">
            Multi-model autonomous performance benchmark across market regimes and execution venues.
          </p>
        </div>

        {/* Leading Engine Showcase */}
        {leader && (
          <div className="text-left md:text-right border-l md:border-l-0 md:border-r border-white/10 pl-5 md:pl-0 md:pr-6 py-1 font-mono">
            <div className="text-[10px] uppercase font-inter tracking-[0.25em] text-[#7C7A75] mb-1">
              REGIME LEADER
            </div>
            <div className="text-2xl font-playfair font-normal text-[#ECEBE6]">
              {leader.trader_name}
            </div>
            <div
              className="text-sm font-mono mt-0.5"
              style={{
                color: (leader.total_pnl ?? 0) >= 0 ? '#6E987E' : '#B86B65',
              }}
            >
              {(leader.total_pnl ?? 0) >= 0 ? '+' : ''}
              {leader.total_pnl_pct?.toFixed(2) || '0.00'}% PnL
            </div>
          </div>
        )}
      </div>

      {/* Main Arena Layout: Asymmetric 7:5 Split with Airy Space */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-10 mb-12 items-start">
        {/* Left: Performance Comparison Chart (7 cols) */}
        <div className="lg:col-span-7 space-y-4">
          <div className="flex items-baseline justify-between border-b border-white/[0.06] pb-3 mb-6">
            <h2 className="text-xl font-playfair font-normal text-[#ECEBE6]">
              Equity <span className="luxury-gold-italic">Trajectory</span>
            </h2>
            <span className="text-[9px] font-inter uppercase tracking-luxury text-[#7C7A75]">
              MULTI-AGENT CUMULATIVE RETURN
            </span>
          </div>
          <div className="p-2">
            <ComparisonChart traders={sortedTraders.slice(0, 5)} />
          </div>
        </div>

        {/* Right: Performance Rankings (5 cols - Clean Editorial Table) */}
        <div className="lg:col-span-5 space-y-4">
          <div className="flex items-baseline justify-between border-b border-white/[0.06] pb-3 mb-6">
            <h2 className="text-xl font-playfair font-normal text-[#ECEBE6]">
              The <span className="luxury-gold-italic">Leaderboard</span>
            </h2>
            <div className="flex items-center gap-1.5 font-mono text-[9px] tracking-widest text-[#6E987E] uppercase">
              <span className="w-1.5 h-1.5 rounded-full bg-[#6E987E]"></span>
              LIVE ARBITRATION
            </div>
          </div>

          <div className="divide-y divide-white/[0.06]">
            {sortedTraders.map((trader, index) => {
              const isLeader = index === 0
              const traderColor = getTraderColor(
                sortedTraders,
                trader.trader_id
              )

              return (
                <div
                  key={trader.trader_id}
                  onClick={() => handleTraderClick(trader.trader_id)}
                  className={`stat-card-pane relative overflow-hidden py-4 px-3 transition-all duration-300 cursor-pointer hover:bg-white/[0.02] flex items-center justify-between group ${
                    isLeader ? 'bg-white/[0.015]' : ''
                  }`}
                >
                  {/* Rank & Name */}
                  <div className="flex items-center gap-4">
                    <span className="text-xs font-mono text-[#5E5D58] w-4">
                      {String(index + 1).padStart(2, '0')}
                    </span>
                    <span
                      className="w-1.5 h-1.5 rounded-full"
                      style={{ backgroundColor: traderColor }}
                    />
                    <div>
                      <div className="text-base font-playfair font-normal text-[#ECEBE6] group-hover:text-white transition-colors">
                        {trader.trader_name}
                      </div>
                      <div className="text-[10px] font-mono tracking-wider text-[#7C7A75] mt-0.5">
                        {trader.ai_model.toUpperCase()} • {trader.exchange.toUpperCase()}
                      </div>
                    </div>
                  </div>

                  {/* Equity & PnL */}
                  <div className="text-right font-mono">
                    <div className="text-sm font-medium text-[#ECEBE6]">
                      {trader.total_equity?.toFixed(2) || '0.00'} <span className="text-[10px] text-[#5E5D58]">USDT</span>
                    </div>
                    <div
                      className="text-xs mt-0.5"
                      style={{
                        color: (trader.total_pnl ?? 0) >= 0 ? '#6E987E' : '#B86B65',
                      }}
                    >
                      {(trader.total_pnl ?? 0) >= 0 ? '+' : ''}
                      {trader.total_pnl_pct?.toFixed(2) || '0.00'}%
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>



      {/* Head-to-Head Performance Comparison */}
      {competition.traders.length === 2 && (
        <div
          className="binance-card p-5 animate-slide-in"
          style={{ animationDelay: '0.3s' }}
        >
          <h2
            className="text-lg font-bold mb-4 flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <Zap className="w-5 h-5" style={{ color: '#EAECEF' }} />
            {t('headToHead', language)}
          </h2>
          <div className="grid grid-cols-2 gap-4">
            {sortedTraders.map((trader, index) => {
              const isWinning = index === 0
              const opponent = sortedTraders[1 - index]
              const gap = trader.total_pnl_pct - opponent.total_pnl_pct

              return (
                <div
                  key={trader.trader_id}
                  className="stat-card-pane relative overflow-hidden p-4 rounded transition-all duration-300 hover:scale-[1.02]"
                  style={
                    isWinning
                      ? {
                          background:
                            'linear-gradient(135deg, rgba(14, 203, 129, 0.08) 0%, rgba(14, 203, 129, 0.02) 100%)',
                          border: '2px solid rgba(14, 203, 129, 0.3)',
                          boxShadow: '0 3px 15px rgba(14, 203, 129, 0.12)',
                        }
                      : {
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          boxShadow: '0 1px 4px rgba(0, 0, 0, 0.3)',
                        }
                  }
                >
                  <div className="text-center">
                    <div
                      className="text-sm md:text-base font-bold mb-2"
                      style={{
                        color: getTraderColor(sortedTraders, trader.trader_id),
                      }}
                    >
                      {trader.trader_name}
                    </div>
                    <div
                      className="text-lg md:text-2xl font-bold mono mb-1"
                      style={{
                        color:
                          (trader.total_pnl ?? 0) >= 0 ? '#0ECB81' : '#F6465D',
                      }}
                    >
                      {(trader.total_pnl ?? 0) >= 0 ? '+' : ''}
                      {trader.total_pnl_pct?.toFixed(2) || '0.00'}%
                    </div>
                    {isWinning && gap > 0 && (
                      <div
                        className="text-xs font-semibold"
                        style={{ color: '#0ECB81' }}
                      >
                        {t('leadingBy', language, { gap: gap.toFixed(2) })}
                      </div>
                    )}
                    {!isWinning && gap < 0 && (
                      <div
                        className="text-xs font-semibold"
                        style={{ color: '#F6465D' }}
                      >
                        {t('behindBy', language, {
                          gap: Math.abs(gap).toFixed(2),
                        })}
                      </div>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Trader Config View Modal */}
      <TraderConfigViewModal
        isOpen={isModalOpen}
        onClose={closeModal}
        traderData={selectedTrader}
      />
    </div>
  )
}
