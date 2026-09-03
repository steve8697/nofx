import { useState, useEffect } from 'react'
import type { AIModel, Exchange, CreateTraderRequest } from '../types'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { api } from '../lib/api'

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

interface TraderConfigData {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  btc_eth_leverage: number
  altcoin_leverage: number
  trading_symbols: string
  custom_prompt: string
  override_base_prompt: boolean
  system_prompt_template: string
  is_cross_margin: boolean
  use_coin_pool: boolean
  use_oi_top: boolean
  initial_balance: number
  scan_interval_minutes: number
}

interface TraderConfigModalProps {
  isOpen: boolean
  onClose: () => void
  traderData?: TraderConfigData | null
  isEditMode?: boolean
  availableModels?: AIModel[]
  availableExchanges?: Exchange[]
  onSave?: (data: CreateTraderRequest) => Promise<void>
}

export function TraderConfigModal({
  isOpen,
  onClose,
  traderData,
  isEditMode = false,
  availableModels = [],
  availableExchanges = [],
  onSave,
}: TraderConfigModalProps) {
  const { language } = useLanguage()
  const [formData, setFormData] = useState<TraderConfigData>({
    trader_name: '',
    ai_model: '',
    exchange_id: '',
    btc_eth_leverage: 5,
    altcoin_leverage: 3,
    trading_symbols: '',
    custom_prompt: '',
    override_base_prompt: false,
    system_prompt_template: 'adaptive',
    is_cross_margin: true,
    use_coin_pool: false,
    use_oi_top: false,
    initial_balance: 1000,
    scan_interval_minutes: 3,
  })
  const [isSaving, setIsSaving] = useState(false)
  const [availableCoins, setAvailableCoins] = useState<string[]>([])
  const [selectedCoins, setSelectedCoins] = useState<string[]>([])
  const [showCoinSelector, setShowCoinSelector] = useState(false)
  const [promptTemplates, setPromptTemplates] = useState<{ name: string }[]>([])
  const [isFetchingBalance, setIsFetchingBalance] = useState(false)
  const [balanceFetchError, setBalanceFetchError] = useState<string>('')

  useEffect(() => {
    if (traderData) {
      // 确保 system_prompt_template 有默认值
      const systemPromptTemplate = traderData.system_prompt_template || 'default'
      
      // ⚠️ 调试：输出接收到的交易员数据
      console.log('📊 接收到的交易员数据:', traderData)
      console.log('📊 system_prompt_template 原始值:', traderData.system_prompt_template)
      console.log('📊 system_prompt_template 處理後值:', systemPromptTemplate)
      console.log('📊 初始余额 (initial_balance):', traderData.initial_balance)
      console.log('📊 初始余额类型:', typeof traderData.initial_balance)
      
      // ✅ 修正：嚴格保護初始餘額，不允許意外修改
      // 初始餘額是用戶的投入本金，必須明確且合理
      let initialBalance = traderData.initial_balance
      
      // 如果初始餘額無效，使用預設值但發出警告
      if (initialBalance === undefined || initialBalance === null || initialBalance <= 0) {
        console.warn('⚠️ 初始餘額無效:', initialBalance, '使用預設值 1000')
        initialBalance = 1000
      }
      
      // 如果初始餘額看起來像是當前淨值（通常小於 50 且有很多小數位），發出警告
      if (initialBalance < 50 && initialBalance.toString().includes('.') && initialBalance.toString().split('.')[1].length > 2) {
        console.warn('⚠️ 警告：初始餘額看起來像是當前淨值而不是投入本金:', initialBalance)
        console.warn('⚠️ 如果這不是您的真實投入本金，請手動修改')
      }
      
      console.log('📊 处理后的初始余额:', initialBalance)
      
      setFormData({
        ...traderData,
        system_prompt_template: systemPromptTemplate, // 确保有值
        initial_balance: initialBalance, // 使用处理后的初始余额
      })
      
      // 调试：输出设置后的表单数据
      console.log('📊 设置后的表单数据 initial_balance:', initialBalance)
      
      // 设置已选择的币种
      if (traderData.trading_symbols) {
        const coins = traderData.trading_symbols
          .split(',')
          .map((s) => s.trim())
          .filter((s) => s)
        setSelectedCoins(coins)
      }
    } else if (!isEditMode) {
      setFormData({
        trader_name: '',
        ai_model: availableModels[0]?.id || '',
        exchange_id: availableExchanges[0]?.id || '',
        btc_eth_leverage: 5,
        altcoin_leverage: 3,
        trading_symbols: '',
        custom_prompt: '',
        override_base_prompt: false,
        system_prompt_template: 'adaptive',
        is_cross_margin: true,
        use_coin_pool: false,
        use_oi_top: false,
        initial_balance: 1000,
        scan_interval_minutes: 3,
      })
    }
  }, [traderData, isEditMode, availableModels, availableExchanges])

  // 获取系统配置中的币种列表
  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const response = await fetch('/api/config')
        const config = await response.json()
        if (config.default_coins) {
          setAvailableCoins(config.default_coins)
        }
      } catch (error) {
        console.error('Failed to fetch config:', error)
        // 使用默认币种列表
        setAvailableCoins([
          'BTCUSDT',
          'ETHUSDT',
          'SOLUSDT',
          'BNBUSDT',
          'XRPUSDT',
          'DOGEUSDT',
          'ADAUSDT',
        ])
      }
    }
    fetchConfig()
  }, [])

  // 获取系统提示词模板列表
  useEffect(() => {
    const fetchPromptTemplates = async () => {
      try {
        const response = await fetch('/api/prompt-templates')
        const data = await response.json()
        if (data.templates) {
          setPromptTemplates(data.templates)
        }
      } catch (error) {
        console.error('Failed to fetch prompt templates:', error)
        // 使用默认模板列表
        setPromptTemplates([{ name: 'default' }, { name: 'aggressive' }])
      }
    }
    fetchPromptTemplates()
  }, [])

  // 当选择的币种改变时，更新输入框
  useEffect(() => {
    const symbolsString = selectedCoins.join(',')
    setFormData((prev) => ({ ...prev, trading_symbols: symbolsString }))
  }, [selectedCoins])

  if (!isOpen) return null

  const handleInputChange = (field: keyof TraderConfigData, value: any) => {
    setFormData((prev) => ({ ...prev, [field]: value }))

    // 如果是直接编辑trading_symbols，同步更新selectedCoins
    if (field === 'trading_symbols') {
      const coins = value
        .split(',')
        .map((s: string) => s.trim())
        .filter((s: string) => s)
      setSelectedCoins(coins)
    }
  }

  const handleCoinToggle = (coin: string) => {
    setSelectedCoins((prev) => {
      if (prev.includes(coin)) {
        return prev.filter((c) => c !== coin)
      } else {
        return [...prev, coin]
      }
    })
  }

  const handleFetchCurrentBalance = async () => {
    if (!isEditMode || !traderData?.trader_id) {
      setBalanceFetchError('只有在编辑模式下才能查詢餘額')
      return
    }

    setIsFetchingBalance(true)
    setBalanceFetchError('')

    try {
      // 使用统一的API调用，自动处理认证错误
      const data = await api.getAccount(traderData.trader_id)

      // total_equity = 当前账户净值（包含未实现盈亏）
      const currentBalance = data.total_equity || data.wallet_balance || 0
      const currentInitialBalance = formData.initial_balance
      const profitLoss = currentBalance - currentInitialBalance
      const profitLossPercent = currentInitialBalance > 0 ? (profitLoss / currentInitialBalance) * 100 : 0

      // ✅ 修正：不再覆蓋初始餘額，只顯示當前餘額資訊
      // 初始餘額（本金）應該保持不變
      const infoMessage = `💰 餘額查詢成功\n\n` +
        `初始本金: ${currentInitialBalance.toFixed(2)} USDT（保持不變）\n` +
        `當前淨值: ${currentBalance.toFixed(2)} USDT\n` +
        `盈虧金額: ${profitLoss >= 0 ? '+' : ''}${profitLoss.toFixed(2)} USDT\n` +
        `盈虧比例: ${profitLossPercent >= 0 ? '+' : ''}${profitLossPercent.toFixed(2)}%\n\n` +
        `💡 提示：初始餘額是您的投入本金，不會因交易盈虧而改變。\n` +
        `如需調整本金（如充值/提現），請手動修改初始餘額欄位。`
      
      alert(infoMessage)
      
      // 不再更新 initial_balance，保持原值
      console.log('✅ 餘額查詢成功')
      console.log('初始本金:', currentInitialBalance, 'USDT (保持不變)')
      console.log('當前淨值:', currentBalance, 'USDT')
      console.log('盈虧:', profitLoss.toFixed(2), 'USDT (', profitLossPercent.toFixed(2), '%)')
    } catch (error) {
      console.error('获取余额失败:', error)
      setBalanceFetchError('获取余额失败，请检查网络连接')
    } finally {
      setIsFetchingBalance(false)
    }
  }

  const handleSave = async () => {
    if (!onSave) return

    setIsSaving(true)
    try {
      // ⚠️ 调试：输出保存前的表单数据
      console.log('💾 保存前的表单数据:', formData)
      console.log('💾 保存的初始余额 (initial_balance):', formData.initial_balance)
      
      const saveData: CreateTraderRequest = {
        name: formData.trader_name,
        ai_model_id: formData.ai_model,
        exchange_id: formData.exchange_id,
        btc_eth_leverage: formData.btc_eth_leverage,
        altcoin_leverage: formData.altcoin_leverage,
        trading_symbols: formData.trading_symbols,
        custom_prompt: formData.custom_prompt,
        override_base_prompt: formData.override_base_prompt,
        system_prompt_template: formData.system_prompt_template,
        is_cross_margin: formData.is_cross_margin,
        use_coin_pool: formData.use_coin_pool,
        use_oi_top: formData.use_oi_top,
        initial_balance: formData.initial_balance, // ⚠️ 确保发送初始余额
        scan_interval_minutes: formData.scan_interval_minutes,
      }
      
      // ⚠️ 调试：输出要发送的数据
      console.log('💾 准备发送的保存数据:', JSON.stringify(saveData, null, 2))
      console.log('💾 initial_balance 值:', saveData.initial_balance)
      console.log('💾 initial_balance 类型:', typeof saveData.initial_balance)
      
      await onSave(saveData)
      onClose()
    } catch (error) {
      console.error('保存失败:', error)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-md">
      <div
        className="bg-[#0B0E11] border border-white/5 rounded-xl shadow-2xl max-w-3xl w-full mx-4 max-h-[90vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-white/5 bg-black/40">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-white/10 border border-white/15 flex items-center justify-center">
              <span className="text-lg">{isEditMode ? '✏️' : '➕'}</span>
            </div>
            <div>
              <h2 className="text-xl font-bold text-[#EAECEF]">
                {isEditMode ? '修改交易员' : '创建交易员'}
              </h2>
              <p className="text-sm text-gray-500 mt-1">
                {isEditMode ? '修改交易员配置参数' : '配置新的AI交易员'}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg text-gray-400 hover:text-white hover:bg-white/10 transition-colors flex items-center justify-center"
          >
            ✕
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-8">
          {/* Basic Info */}
          <div className="bg-[#0A0A0C] border border-white/5 rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              🤖 基础配置
            </h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-gray-400 block mb-2">
                  交易员名称
                </label>
                <input
                  type="text"
                  value={formData.trader_name}
                  onChange={(e) =>
                    handleInputChange('trader_name', e.target.value)
                  }
                  className="w-full px-3 py-2 bg-black/50 border border-white/10 rounded text-[#EAECEF] focus:border-white focus:outline-none focus:ring-1 focus:ring-white transition-all"
                  placeholder="请输入交易员名称"
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-gray-400 block mb-2">
                    AI模型
                  </label>
                  <select
                    value={formData.ai_model}
                    onChange={(e) =>
                      handleInputChange('ai_model', e.target.value)
                    }
                    className="w-full px-3 py-2 bg-black/50 border border-white/10 rounded text-[#EAECEF] focus:border-white focus:outline-none focus:ring-1 focus:ring-white transition-all"
                  >
                    {availableModels.map((model) => (
                      <option key={model.id} value={model.id}>
                        {model.customModelName && model.customModelName.trim() !== ''
                          ? model.customModelName.toUpperCase()
                          : getShortName(model.name || model.id).toUpperCase()}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="text-sm text-gray-400 block mb-2">
                    交易所
                  </label>
                  <select
                    value={formData.exchange_id}
                    onChange={(e) =>
                      handleInputChange('exchange_id', e.target.value)
                    }
                    className="w-full px-3 py-2 bg-black/50 border border-white/10 rounded text-[#EAECEF] focus:border-white focus:outline-none focus:ring-1 focus:ring-white transition-all"
                  >
                    {availableExchanges.map((exchange) => (
                      <option key={exchange.id} value={exchange.id}>
                        {getShortName(
                          exchange.name || exchange.id
                        ).toUpperCase()}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </div>
          </div>

          {/* Trading Configuration */}
          <div className="bg-[#0A0A0C] border border-white/5 rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              ⚖️ 交易配置
            </h3>
            <div className="space-y-4">
              {/* 第一行：保证金模式和初始余额 */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-gray-400 block mb-2">
                    保证金模式
                  </label>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => handleInputChange('is_cross_margin', true)}
                      className={`flex-1 px-3 py-2 rounded text-sm transition-all ${
                        formData.is_cross_margin
                          ? 'bg-white text-black font-bold'
                          : 'bg-black/35 text-gray-400 border border-white/5'
                      }`}
                    >
                      全仓
                    </button>
                    <button
                      type="button"
                      onClick={() =>
                        handleInputChange('is_cross_margin', false)
                      }
                      className={`flex-1 px-3 py-2 rounded text-sm transition-all ${
                        !formData.is_cross_margin
                          ? 'bg-white text-black font-bold'
                          : 'bg-black/35 text-gray-400 border border-white/5'
                      }`}
                    >
                      逐仓
                    </button>
                  </div>
                </div>
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <label className="text-sm text-gray-400">
                      初始余额 ($)
                      {!isEditMode && (
                        <span className="text-white font-bold ml-1">*</span>
                      )}
                    </label>
                    {isEditMode && (
                      <button
                        type="button"
                        onClick={handleFetchCurrentBalance}
                        disabled={isFetchingBalance}
                        className="px-3 py-1 text-xs bg-white/10 text-[#EAECEF] border border-white/15 rounded hover:bg-white/20 transition-all disabled:bg-white/5 disabled:text-gray-500 disabled:cursor-not-allowed font-bold"
                        title="查詢交易所當前餘額和盈虧狀況（不會改變初始餘額）"
                      >
                        {isFetchingBalance ? '查詢中...' : '查詢餘額'}
                      </button>
                    )}
                  </div>
                  <input
                    type="number"
                    value={formData.initial_balance}
                    onChange={(e) =>
                      handleInputChange(
                        'initial_balance',
                        Number(e.target.value)
                      )
                    }
                    className="w-full px-3 py-2 bg-black/50 border border-white/10 rounded text-[#EAECEF] focus:border-white focus:outline-none focus:ring-1 focus:ring-white transition-all"
                    min="100"
                    step="0.01"
                  />
                  {!isEditMode && (
                    <p className="text-xs text-gray-400 mt-1 flex items-center gap-1">
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        className="w-3.5 h-3.5"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      >
                        <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
                        <line x1="12" x2="12" y1="9" y2="13" />
                        <line x1="12" x2="12.01" y1="17" y2="17" />
                      </svg>
                      请输入您交易所账户的当前实际余额。如果输入不准确，P&L统计将会错误。
                    </p>
                  )}
                  {isEditMode && (
                    <p className="text-xs text-gray-500 mt-1">
                      點擊"查詢餘額"按鈕可查看交易所當前淨值和盈虧狀況（初始餘額保持不變）
                    </p>
                  )}
                  {balanceFetchError && (
                    <p className="text-xs text-red-500 mt-1">
                      {balanceFetchError}
                    </p>
                  )}
                </div>
              </div>

              {/* 第二行：AI 扫描决策间隔 */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-gray-400 block mb-2">
                    {t('aiScanInterval', language)}
                  </label>
                  <input
                    type="number"
                    value={formData.scan_interval_minutes}
                    onChange={(e) => {
                      const parsedValue = Number(e.target.value)
                      const safeValue = Number.isFinite(parsedValue)
                        ? Math.max(3, parsedValue)
                        : 3
                      handleInputChange('scan_interval_minutes', safeValue)
                    }}
                    className="w-full px-3 py-2 bg-black/50 border border-white/10 rounded text-[#EAECEF] focus:border-white focus:outline-none focus:ring-1 focus:ring-white transition-all"
                    min="3"
                    max="60"
                    step="1"
                  />
                  <p className="text-xs text-gray-500 mt-1">
                    {t('scanIntervalRecommend', language)}
                  </p>
                </div>
                <div></div>
              </div>

              {/* 第三行：杠杆设置 */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-gray-400 block mb-2">
                    BTC/ETH 杠杆
                  </label>
                  <input
                    type="number"
                    value={formData.btc_eth_leverage}
                    onChange={(e) =>
                      handleInputChange(
                        'btc_eth_leverage',
                        Number(e.target.value)
                      )
                    }
                    className="w-full px-3 py-2 bg-black/50 border border-white/10 rounded text-[#EAECEF] focus:border-white focus:outline-none focus:ring-1 focus:ring-white transition-all"
                    min="1"
                    max="125"
                  />
                </div>
                <div>
                  <label className="text-sm text-gray-400 block mb-2">
                    山寨币杠杆
                  </label>
                  <input
                    type="number"
                    value={formData.altcoin_leverage}
                    onChange={(e) =>
                      handleInputChange(
                        'altcoin_leverage',
                        Number(e.target.value)
                      )
                    }
                    className="w-full px-3 py-2 bg-black/50 border border-white/10 rounded text-[#EAECEF] focus:border-white focus:outline-none focus:ring-1 focus:ring-white transition-all"
                    min="1"
                    max="75"
                  />
                </div>
              </div>

              {/* 第三行：交易币种 */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm text-gray-400">
                    交易币种 (用逗号分隔，留空使用默认)
                  </label>
                  <button
                    type="button"
                    onClick={() => setShowCoinSelector(!showCoinSelector)}
                    className="px-3 py-1 text-xs bg-white/10 text-[#EAECEF] border border-white/15 rounded hover:bg-white/20 transition-all font-bold"
                  >
                    {showCoinSelector ? '收起选择' : '快速选择'}
                  </button>
                </div>
                <input
                  type="text"
                  value={formData.trading_symbols}
                  onChange={(e) =>
                    handleInputChange('trading_symbols', e.target.value)
                  }
                  className="w-full px-3 py-2 bg-black/50 border border-white/10 rounded text-[#EAECEF] focus:border-white focus:outline-none focus:ring-1 focus:ring-white transition-all"
                  placeholder="例如: BTCUSDT,ETHUSDT,ADAUSDT"
                />

                {/* 币种选择器 */}
                {showCoinSelector && (
                  <div className="mt-3 p-3 bg-black/35 border border-white/5 rounded">
                    <div className="text-xs text-gray-500 mb-2">
                      点击选择币种：
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {availableCoins.map((coin) => (
                        <button
                          key={coin}
                          type="button"
                          onClick={() => handleCoinToggle(coin)}
                          className={`px-2 py-1 text-xs rounded transition-all ${
                            selectedCoins.includes(coin)
                              ? 'bg-white text-black font-bold'
                              : 'bg-black/35 text-gray-400 border border-white/5 hover:border-white/20 hover:text-white'
                          }`}
                        >
                          {coin.replace('USDT', '')}
                        </button>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Signal Sources */}
          <div className="bg-[#0A0A0C] border border-white/5 rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              📡 信号源配置
            </h3>
            <div className="grid grid-cols-2 gap-4">
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.use_coin_pool}
                  onChange={(e) =>
                    handleInputChange('use_coin_pool', e.target.checked)
                  }
                  className="w-4 h-4 rounded border-white/10 bg-black/50 text-white focus:ring-0 focus:ring-offset-0"
                />
                <label className="text-sm text-gray-400">
                  使用 Coin Pool 信号
                </label>
              </div>
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.use_oi_top}
                  onChange={(e) =>
                    handleInputChange('use_oi_top', e.target.checked)
                  }
                  className="w-4 h-4 rounded border-white/10 bg-black/50 text-white focus:ring-0 focus:ring-offset-0"
                />
                <label className="text-sm text-gray-400">
                  使用 OI Top 信号
                </label>
              </div>
            </div>
          </div>

          {/* Trading Prompt */}
          <div className="bg-[#0A0A0C] border border-white/5 rounded-lg p-5">
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              💬 交易策略提示词
            </h3>
            <div className="space-y-4">
              {/* 系统提示词模板选择 */}
              <div>
                <label className="text-sm text-gray-400 block mb-2">
                  系统提示词模板
                </label>
                <select
                  value={formData.system_prompt_template}
                  onChange={(e) =>
                    handleInputChange('system_prompt_template', e.target.value)
                  }
                  className="w-full px-3 py-2 bg-black/50 border border-white/10 rounded text-[#EAECEF] focus:border-white focus:outline-none focus:ring-1 focus:ring-white transition-all"
                >
                  {promptTemplates.map((template) => (
                    <option key={template.name} value={template.name}>
                      {template.name === 'default'
                        ? 'Default (默认稳健)'
                        : template.name === 'aggressive'
                          ? 'Aggressive (激进)'
                          : template.name.charAt(0).toUpperCase() +
                            template.name.slice(1)}
                    </option>
                  ))}
                </select>
                <p className="text-xs text-gray-500 mt-1">
                  选择预设的交易策略模板（包含交易哲学、风控原则等）
                </p>
              </div>

              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.override_base_prompt}
                  onChange={(e) =>
                    handleInputChange('override_base_prompt', e.target.checked)
                  }
                  className="w-4 h-4 rounded border-white/10 bg-black/50 text-white focus:ring-0 focus:ring-offset-0"
                />
                <label className="text-sm text-gray-400">覆盖默认提示词</label>
                <span className="text-xs text-gray-500 inline-flex items-center gap-1">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="w-3.5 h-3.5"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
                    <line x1="12" x2="12" y1="9" y2="13" />
                    <line x1="12" x2="12.01" y1="17" y2="17" />
                  </svg>{' '}
                  启用后将完全替换默认策略
                </span>
              </div>
              <div>
                <label className="text-sm text-gray-400 block mb-2">
                  {formData.override_base_prompt
                    ? '自定义提示词'
                    : '附加提示词'}
                </label>
                <textarea
                  value={formData.custom_prompt}
                  onChange={(e) =>
                    handleInputChange('custom_prompt', e.target.value)
                  }
                  className="w-full px-3 py-2 bg-black/50 border border-white/10 rounded text-[#EAECEF] focus:border-white focus:outline-none focus:ring-1 focus:ring-white transition-all h-24 resize-none"
                  placeholder={
                    formData.override_base_prompt
                      ? '输入完整的交易策略提示词...'
                      : '输入额外的交易策略提示...'
                  }
                />
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-6 border-t border-white/5 bg-black/40">
          <button
            onClick={onClose}
            className="px-6 py-3 bg-black/35 text-gray-400 border border-white/5 rounded-lg hover:bg-white/5 hover:text-white transition-all"
          >
            取消
          </button>
          {onSave && (
            <button
              onClick={handleSave}
              disabled={
                isSaving ||
                !formData.trader_name ||
                !formData.ai_model ||
                !formData.exchange_id
              }
              className="px-8 py-3 bg-white text-black font-bold rounded-lg hover:bg-gray-200 transition-all disabled:bg-white/5 disabled:text-gray-500 disabled:cursor-not-allowed shadow-lg"
            >
              {isSaving ? '保存中...' : isEditMode ? '保存修改' : '创建交易员'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
