import { useState } from 'react'
import type { TraderConfigData } from '../types'

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

interface TraderConfigViewModalProps {
  isOpen: boolean
  onClose: () => void
  traderData?: TraderConfigData | null
}

export function TraderConfigViewModal({
  isOpen,
  onClose,
  traderData,
}: TraderConfigViewModalProps) {
  const [copiedField, setCopiedField] = useState<string | null>(null)

  if (!isOpen || !traderData) return null

  const copyToClipboard = async (text: string, fieldName: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedField(fieldName)
      setTimeout(() => setCopiedField(null), 2000)
    } catch (error) {
      console.error('Failed to copy:', error)
    }
  }

  const CopyButton = ({
    text,
    fieldName,
  }: {
    text: string
    fieldName: string
  }) => (
    <button
      onClick={() => copyToClipboard(text, fieldName)}
      className="ml-2 px-2 py-1 text-xs rounded transition-all duration-200 hover:scale-105"
      style={{
        background:
          copiedField === fieldName
            ? 'rgba(255, 255, 255, 0.15)'
            : 'rgba(255, 255, 255, 0.05)',
        color: copiedField === fieldName ? '#ffffff' : '#EAECEF',
        border: `1px solid ${copiedField === fieldName ? 'rgba(255, 255, 255, 0.3)' : 'rgba(255, 255, 255, 0.1)'}`,
      }}
    >
      {copiedField === fieldName ? '✓ 已复制' : '📋 复制'}
    </button>
  )

  const InfoRow = ({
    label,
    value,
    copyable = false,
    fieldName = '',
  }: {
    label: string
    value: string | number | boolean
    copyable?: boolean
    fieldName?: string
  }) => (
    <div className="flex justify-between items-start py-2 border-b border-white/5 last:border-b-0">
      <span className="text-sm text-gray-500 font-medium">{label}</span>
      <div className="flex items-center text-right">
        <span className="text-sm text-[#EAECEF] font-mono">
          {typeof value === 'boolean' ? (value ? '是' : '否') : value}
        </span>
        {copyable && typeof value === 'string' && value && (
          <CopyButton text={value} fieldName={fieldName} />
        )}
      </div>
    </div>
  )

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80">
      <div
        className="bg-[#0B0E11] border border-white/5 rounded-xl shadow-2xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-white/5 bg-black/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-white/10 border border-white/15 flex items-center justify-center">
              <span className="text-lg">👁️</span>
            </div>
            <div>
              <h2 className="text-xl font-bold text-[#EAECEF]">交易员配置</h2>
              <p className="text-sm text-gray-500 mt-1">
                {traderData.trader_name} 的配置信息
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {/* Running Status */}
            <div
              className="px-3 py-1 rounded-full text-xs font-bold flex items-center gap-1"
              style={
                traderData.is_running
                  ? { background: 'rgba(255, 255, 255, 0.08)', color: '#EAECEF' }
                  : { background: 'rgba(255, 255, 255, 0.02)', color: '#8E8E93' }
              }
            >
              <span>{traderData.is_running ? '●' : '○'}</span>
              {traderData.is_running ? '运行中' : '已停止'}
            </div>
            <button
              onClick={onClose}
              className="w-8 h-8 rounded-lg text-gray-400 hover:text-white hover:bg-white/10 transition-colors flex items-center justify-center"
            >
              ✕
            </button>
          </div>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6">
          {/* Basic Info */}
          <div className="bg-[#0A0A0C] border border-white/5 rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-4 flex items-center gap-2">
              🤖 基础信息
            </h3>
            <div className="space-y-3">
              <InfoRow
                label="交易员ID"
                value={traderData.trader_id || ''}
                copyable
                fieldName="trader_id"
              />
              <InfoRow
                label="交易员名称"
                value={traderData.trader_name}
                copyable
                fieldName="trader_name"
              />
              <InfoRow
                label="AI模型"
                value={getShortName(traderData.ai_model).toUpperCase()}
              />
              <InfoRow
                label="交易所"
                value={getShortName(traderData.exchange_id).toUpperCase()}
              />
              <InfoRow
                label="初始余额"
                value={`$${traderData.initial_balance.toLocaleString()}`}
              />
            </div>
          </div>

          {/* Trading Configuration */}
          <div className="bg-[#0A0A0C] border border-white/5 rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-4 flex items-center gap-2">
              ⚖️ 交易配置
            </h3>
            <div className="space-y-3">
              <InfoRow
                label="保证金模式"
                value={traderData.is_cross_margin ? '全仓' : '逐仓'}
              />
              <InfoRow
                label="BTC/ETH 杠杆"
                value={`${traderData.btc_eth_leverage}x`}
              />
              <InfoRow
                label="山寨币杠杆"
                value={`${traderData.altcoin_leverage}x`}
              />
              <InfoRow
                label="系统提示词模板"
                value={traderData.system_prompt_template || 'adaptive'}
                copyable
                fieldName="system_prompt_template"
              />
              <InfoRow
                label="扫描间隔"
                value={`${traderData.scan_interval_minutes || 3} 分钟`}
              />
              <InfoRow
                label="交易币种"
                value={traderData.trading_symbols || '使用默认币种'}
                copyable
                fieldName="trading_symbols"
              />
            </div>
          </div>

          {/* Signal Sources */}
          <div className="bg-[#0A0A0C] border border-white/5 rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-4 flex items-center gap-2">
              📡 信号源配置
            </h3>
            <div className="space-y-3">
              <InfoRow
                label="Coin Pool 信号"
                value={traderData.use_coin_pool}
              />
              <InfoRow label="OI Top 信号" value={traderData.use_oi_top} />
            </div>
          </div>

          {/* Custom Prompt */}
          <div className="bg-[#0A0A0C] border border-white/5 rounded-lg p-5">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-[#EAECEF] flex items-center gap-2">
                💬 交易策略提示词
              </h3>
              {traderData.custom_prompt && (
                <CopyButton
                  text={traderData.custom_prompt}
                  fieldName="custom_prompt"
                />
              )}
            </div>
            <div className="space-y-3">
              <InfoRow
                label="覆盖默认提示词"
                value={traderData.override_base_prompt}
              />
              {traderData.custom_prompt ? (
                <div>
                  <div className="text-sm text-gray-500 mb-2">
                    {traderData.override_base_prompt
                      ? '自定义提示词'
                      : '附加提示词'}
                    ：
                  </div>
                  <div
                    className="p-3 rounded border text-sm text-[#EAECEF] font-mono leading-relaxed max-h-48 overflow-y-auto border-white/5"
                    style={{
                      background: 'rgba(0, 0, 0, 0.35)',
                      whiteSpace: 'pre-wrap',
                    }}
                  >
                    {traderData.custom_prompt}
                  </div>
                </div>
              ) : (
                <div
                  className="text-sm text-gray-500 italic p-3 rounded border border-white/5 bg-black/35"
                >
                  未设置自定义提示词，使用系统默认策略
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-6 border-t border-white/5 bg-black/40">
          <button
            onClick={onClose}
            className="px-6 py-3 bg-black/35 text-gray-400 border border-white/5 rounded-lg hover:bg-white/5 hover:text-white transition-all"
          >
            关闭
          </button>
          <button
            onClick={() =>
              copyToClipboard(
                JSON.stringify(traderData, null, 2),
                'full_config'
              )
            }
            className="px-6 py-3 bg-white text-black font-bold rounded-lg hover:bg-gray-200 transition-all shadow-lg"
          >
            {copiedField === 'full_config' ? '✓ 已复制配置' : '📋 复制完整配置'}
          </button>
        </div>
      </div>
    </div>
  )
}
