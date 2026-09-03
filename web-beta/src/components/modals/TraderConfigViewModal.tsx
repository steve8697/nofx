import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { X, Cpu, PlayCircle, StopCircle, Terminal } from 'lucide-react'
import { motion } from 'framer-motion'

interface TraderConfigViewModalProps {
  traderId: string
  traderName: string
  onClose: () => void
}

export function TraderConfigViewModal({ traderId, traderName, onClose }: TraderConfigViewModalProps) {
  const [config, setConfig] = useState<any>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let mounted = true
    setLoading(true)
    api.getPublicTraderConfig(traderId)
      .then(res => {
        if (mounted) {
          setConfig(res)
        }
      })
      .catch(err => {
        console.error('Failed to load public trader config:', err)
      })
      .finally(() => {
        if (mounted) setLoading(false)
      })

    return () => {
      mounted = false
    }
  }, [traderId])

  return (
    <>
      {/* Backdrop */}
      <div 
        onClick={onClose}
        className="fixed inset-0 z-50 bg-black/75 backdrop-blur-xs cursor-pointer"
      />

      {/* Modal Container */}
      <motion.div 
        initial={{ scale: 0.96, opacity: 0 }}
        animate={{ scale: 1, opacity: 1 }}
        exit={{ scale: 0.96, opacity: 0 }}
        className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none"
      >
        <div className="bg-[#111318]/95 border border-[var(--border)] w-full max-w-xl rounded-sm shadow-2xl flex flex-col max-h-[85vh] pointer-events-auto overflow-hidden">
          
          {/* Header */}
          <div className="p-5 border-b border-[var(--border)] flex items-center justify-between bg-[var(--bg-elev)]">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 rounded-sm bg-blue-500/5 border border-blue-500/15 flex items-center justify-center text-blue-400">
                <Cpu size={16} className="animate-pulse" />
              </div>
              <div>
                <h3 className="text-sm font-bold tracking-tight font-mono text-white">AI 引擎策略與參數細節</h3>
                <p className="text-[10px] font-mono text-[var(--text-muted)] tracking-wider">
                  TELEMETRY REPORT // AGENT: {traderName}
                </p>
              </div>
            </div>
            <button 
              onClick={onClose}
              className="p-1 rounded-sm text-[var(--text-muted)] hover:text-white hover:bg-[var(--bg-subtle)] transition-colors cursor-pointer"
            >
              <X size={16} />
            </button>
          </div>

          {/* Content */}
          <div className="flex-1 overflow-y-auto p-6 space-y-6 font-mono text-xs text-[var(--text-muted)]">
            {loading ? (
              <div className="flex flex-col items-center justify-center py-20">
                <div className="w-5 h-5 rounded-full border-2 border-blue-500 border-t-transparent animate-spin mb-3"></div>
                <p className="text-[10px] text-[var(--text-dim)]">正在讀取遠端 AI 組態...</p>
              </div>
            ) : !config ? (
              <div className="p-8 text-center text-[var(--text-dim)]">
                無法獲取該交易員的公開配置報告。
              </div>
            ) : (
              <div className="space-y-5">
                {/* Status bar */}
                <div className="flex items-center justify-between p-3 border border-[var(--border)] rounded-sm bg-[#161a21]/20">
                  <span className="text-[10px] text-[var(--text-dim)]">交易引擎狀態</span>
                  <span className={`flex items-center gap-1 text-[10px] font-bold ${config.is_running ? 'text-[var(--green)]' : 'text-[var(--text-dim)]'}`}>
                    {config.is_running ? (
                      <>
                        <PlayCircle size={12} />
                        RUNNING
                      </>
                    ) : (
                      <>
                        <StopCircle size={12} />
                        STOPPED
                      </>
                    )}
                  </span>
                </div>

                {/* Grid Params */}
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-1">
                    <span className="text-[10px] text-[var(--text-dim)] uppercase block">決策模型 (Decision Engine)</span>
                    <span className="text-white font-bold block">{config.ai_model || config.ai_model_id || '-'}</span>
                  </div>
                  <div className="space-y-1">
                    <span className="text-[10px] text-[var(--text-dim)] uppercase block">所屬交易所 (Exchange)</span>
                    <span className="text-white font-bold block">{config.exchange || config.exchange_id || '-'}</span>
                  </div>
                  <div className="space-y-1">
                    <span className="text-[10px] text-[var(--text-dim)] uppercase block">市場分析週期 (Interval)</span>
                    <span className="text-white font-bold block">{config.scan_interval_minutes || config.scan_interval || 15} 分鐘</span>
                  </div>
                  <div className="space-y-1">
                    <span className="text-[10px] text-[var(--text-dim)] uppercase block">初始分配額 (Initial Fund)</span>
                    <span className="text-white font-bold block">{config.initial_balance || 1000} USDT</span>
                  </div>
                </div>

                {/* Leverages */}
                <div className="grid grid-cols-2 gap-4 pt-4 border-t border-[var(--border)]">
                  <div className="space-y-1">
                    <span className="text-[10px] text-[var(--text-dim)] uppercase block">BTC / ETH 槓桿限制</span>
                    <span className="text-white font-bold block">{config.btc_eth_leverage || 3} x</span>
                  </div>
                  <div className="space-y-1">
                    <span className="text-[10px] text-[var(--text-dim)] uppercase block">山寨幣槓桿限制</span>
                    <span className="text-white font-bold block">{config.altcoin_leverage || 2} x</span>
                  </div>
                </div>

                {/* Coins */}
                <div className="space-y-2 pt-4 border-t border-[var(--border)]">
                  <span className="text-[10px] text-[var(--text-dim)] uppercase block">授權交易目標幣種 (Symbols)</span>
                  <div className="flex flex-wrap gap-1">
                    {(config.trading_symbols || '').split(',').map((s: string) => s.trim()).filter(Boolean).map((sym: string) => (
                      <span key={sym} className="px-2 py-0.5 border border-[var(--border)] bg-[#161a21]/50 text-white rounded-[2px] text-[10px]">
                        {sym}
                      </span>
                    ))}
                    {!(config.trading_symbols) && <span className="text-[var(--text-dim)]">無特別限制</span>}
                  </div>
                </div>

                {/* Features toggles */}
                <div className="grid grid-cols-3 gap-2 pt-4 border-t border-[var(--border)] text-[10px]">
                  <div className={`p-2 border rounded-sm ${config.is_cross_margin !== false ? 'border-blue-500/10 bg-blue-500/5 text-blue-400' : 'border-[var(--border)] text-[var(--text-dim)]'}`}>
                    <div className="font-bold">全倉保證金</div>
                    <div className="text-[8px] mt-0.5 opacity-60">Cross Margin</div>
                  </div>
                  <div className={`p-2 border rounded-sm ${config.use_coin_pool ? 'border-blue-500/10 bg-blue-500/5 text-blue-400' : 'border-[var(--border)] text-[var(--text-dim)]'}`}>
                    <div className="font-bold">智能信號池</div>
                    <div className="text-[8px] mt-0.5 opacity-60">Coin Pool</div>
                  </div>
                  <div className={`p-2 border rounded-sm ${config.use_oi_top ? 'border-blue-500/10 bg-blue-500/5 text-blue-400' : 'border-[var(--border)] text-[var(--text-dim)]'}`}>
                    <div className="font-bold">持倉量排行</div>
                    <div className="text-[8px] mt-0.5 opacity-60">OI Top</div>
                  </div>
                </div>

                {/* Custom supplemental guidelines */}
                {config.custom_prompt && (
                  <div className="space-y-2 pt-4 border-t border-[var(--border)]">
                    <span className="text-[10px] text-[var(--text-dim)] uppercase block">補充引導指令 (Custom Guidelines)</span>
                    <pre className="p-3 bg-[#161a21] border border-[var(--border)] rounded-sm text-[10px] text-white overflow-x-auto max-h-24 whitespace-pre-wrap leading-normal font-sans">
                      {config.custom_prompt}
                    </pre>
                  </div>
                )}

                {/* System prompt template (truncated for safety) */}
                {config.override_base_prompt && config.system_prompt_template && (
                  <div className="space-y-2 pt-4 border-t border-[var(--border)]">
                    <div className="flex items-center gap-1 text-[10px] text-[var(--text-dim)] uppercase">
                      <Terminal size={10} />
                      <span>覆蓋用系統提示詞範本 (System Template)</span>
                    </div>
                    <pre className="p-3 bg-[#181111]/30 border border-red-500/10 text-red-300 rounded-sm text-[9px] overflow-x-auto max-h-28 whitespace-pre-wrap leading-normal font-mono">
                      {config.system_prompt_template}
                    </pre>
                  </div>
                )}
              </div>
            )}
          </div>
          
          {/* Footer */}
          <div className="p-4 border-t border-[var(--border)] bg-[var(--bg-elev)] text-right">
            <button 
              onClick={onClose}
              className="px-4 py-1.5 border border-[var(--border)] text-xs font-bold text-[var(--text-muted)] hover:text-white hover:bg-[var(--bg-subtle)] rounded-sm transition-all cursor-pointer"
            >
              關閉遙測報告
            </button>
          </div>
        </div>
      </motion.div>
    </>
  )
}
