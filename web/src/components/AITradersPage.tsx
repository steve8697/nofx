import React, { useState, useEffect } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import type {
  TraderInfo,
  CreateTraderRequest,
  AIModel,
  Exchange,
  ModelProvider,
} from '../types'
import { useLanguage } from '../contexts/LanguageContext'
import { t, type Language } from '../i18n/translations'
import { useAuth } from '../contexts/AuthContext'
import { getExchangeIcon } from './ExchangeIcons'
import { TraderConfigModal } from './TraderConfigModal'
import {
  Bot,
  BarChart3,
  Trash2,
  Plus,
  Users,
  AlertTriangle,
  BookOpen,
  HelpCircle,
  LayoutGrid,
  List,
} from 'lucide-react'

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    case 'custom':
      return 'Custom'
    default:
      return modelId.toUpperCase()
  }
}

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

interface AITraderCardProps {
  trader: TraderInfo;
  isRunning: boolean;
  language: Language;
  onTraderSelect?: (id: string) => void;
  handleToggleTrader: (id: string, isRunning: boolean) => void;
  handleSyncBalance: (id: string) => void;
  handleEditTrader: (id: string) => void;
  handleDeleteTrader: (id: string) => void;
  getModelDisplayName: (model: string) => string;
  getExchangeIcon: (exchange: string, size?: any) => any;
  t: any;
}

function AITraderCard({
  trader,
  isRunning,
  language,
  onTraderSelect,
  handleToggleTrader,
  handleSyncBalance,
  handleEditTrader,
  handleDeleteTrader,
  getModelDisplayName,
  getExchangeIcon,
  t
}: AITraderCardProps) {
  const [rotateX, setRotateX] = useState(0);
  const [rotateY, setRotateY] = useState(0);
  const [glowX, setGlowX] = useState(50);
  const [glowY, setGlowY] = useState(50);
  const [isHovered, setIsHovered] = useState(false);

  const handleMouseMove = (e: React.MouseEvent<HTMLDivElement>) => {
    const el = e.currentTarget;
    const rect = el.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    const xc = x / rect.width - 0.5;
    const yc = y / rect.height - 0.5;
    setRotateX(-yc * 12);
    setRotateY(xc * 12);
    setGlowX((x / rect.width) * 100);
    setGlowY((y / rect.height) * 100);
  };

  const handleMouseLeave = () => {
    setRotateX(0);
    setRotateY(0);
    setIsHovered(false);
  };

  return (
    <div
      onMouseMove={handleMouseMove}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={handleMouseLeave}
      style={{
        transform: `perspective(800px) rotateX(${rotateX}deg) rotateY(${rotateY}deg) scale3d(${isHovered ? 1.025 : 1}, ${isHovered ? 1.025 : 1}, 1)`,
        transition: isHovered ? 'transform 0.08s cubic-bezier(0.25, 1, 0.5, 1)' : 'transform 0.4s cubic-bezier(0.25, 1, 0.5, 1)',
        transformStyle: 'preserve-3d',
      }}
      className={`glass-card p-6 flex flex-col justify-between relative overflow-hidden ${
        isRunning ? 'border border-white/20' : 'border border-white/5'
      }`}
    >
      {/* 霓虹滑鼠追隨光暈 (Mouse Glow Overlay) */}
      {isHovered && (
        <div 
          className="absolute inset-0 pointer-events-none transition-opacity duration-300"
          style={{
            background: `radial-gradient(circle 120px at ${glowX}% ${glowY}%, rgba(255, 255, 255, 0.04), transparent)`,
            zIndex: 1,
          }}
        />
      )}

      {/* 雷射發光條 */}
      {isRunning && (
        <div className="absolute top-0 left-0 right-0 h-[2px] bg-gradient-to-r from-transparent via-white/40 to-transparent animate-pulse z-10"></div>
      )}

      <div style={{ transform: 'translateZ(20px)', transformStyle: 'preserve-3d' }}>
        {/* Top Row: Icon & Status */}
        <div className="flex items-start justify-between mb-5">
          <div className="flex items-center gap-3">
            <div
              className={`w-12 h-12 rounded-2xl flex items-center justify-center flex-shrink-0 transition-all duration-500 ${
                isRunning 
                  ? 'bg-white border border-white text-black' 
                  : 'bg-white/5 border border-white/10 text-gray-400'
              }`}
            >
              <Bot className="w-6 h-6" />
            </div>
            <div>
              <div
                className="font-bold text-base md:text-lg text-[#EAECEF] hover:text-white transition-colors cursor-pointer"
                onClick={() => onTraderSelect?.(trader.trader_id)}
              >
                {trader.trader_name}
              </div>
              <div
                className="text-xs font-mono uppercase tracking-wider mt-0.5"
                style={{
                  color: '#8E8E93',
                }}
              >
                {getModelDisplayName(trader.ai_model)}
              </div>
            </div>
          </div>

          {/* Status Pulse */}
          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-black/40 border border-white/5">
            <span className={isRunning ? 'pulse-dot-green' : 'pulse-dot-red'}></span>
            <span className="text-[10px] font-bold font-mono tracking-wide" style={{ color: isRunning ? '#0ECB81' : '#F6465D' }}>
              {isRunning ? 'RUNNING' : 'STOPPED'}
            </span>
          </div>
        </div>

        {/* Stats & Details Grid */}
        <div className="grid grid-cols-2 gap-3 py-4 border-t border-b border-white/5 mb-5 font-mono text-xs">
          <div>
            <div className="text-[10px] text-gray-500 tracking-wider mb-0.5 uppercase">Exchange</div>
            <div className="text-[#EAECEF] font-semibold flex items-center gap-1.5">
              {getExchangeIcon(trader.exchange_id || '', { width: 14, height: 14 })}
              {trader.exchange_id?.toUpperCase()}
            </div>
          </div>
          <div>
            <div className="text-[10px] text-gray-500 tracking-wider mb-0.5 uppercase">PnL Status</div>
            <div className="text-[#EAECEF] font-semibold flex items-center gap-1.5">
              <span className={isRunning ? 'text-[#0ECB81]' : 'text-gray-400'}>
                {isRunning ? '✦ ACTIVE' : '■ INACTIVE'}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Actions Grid */}
      <div className="grid grid-cols-2 gap-2.5 pt-2 relative z-10" style={{ transform: 'translateZ(10px)' }}>
        <button
          onClick={() => onTraderSelect?.(trader.trader_id)}
          className="px-3 py-2 rounded-xl text-xs font-bold transition-all duration-300 hover:scale-[1.02] flex items-center justify-center gap-1.5 bg-white/5 hover:bg-white/10 text-white border border-white/10 btn-cyber"
        >
          <BarChart3 className="w-3.5 h-3.5" />
          {t('view', language)}
        </button>

        <button
          onClick={() =>
            handleToggleTrader(
              trader.trader_id,
              trader.is_running || false
            )
          }
          className={`px-3 py-2 rounded-xl text-xs font-bold transition-all duration-300 hover:scale-[1.02] flex items-center justify-center gap-1.5 border btn-cyber bg-white/5 hover:bg-white/10 text-white border-white/10`}
        >
          <span>
            {isRunning
              ? t('stop', language)
              : t('start', language)}
          </span>
        </button>

        <button
          onClick={() => handleSyncBalance(trader.trader_id)}
          className="px-3 py-2 rounded-xl text-xs font-bold transition-all duration-300 hover:scale-[1.02] flex items-center justify-center gap-1.5 bg-white/5 hover:bg-white/10 text-white border border-white/10 btn-cyber col-span-2"
        >
          {t('syncBalance', language)}
        </button>

        <button
          onClick={() => handleEditTrader(trader.trader_id)}
          disabled={isRunning}
          className="px-3 py-2 rounded-xl text-xs font-bold transition-all duration-300 hover:scale-[1.02] disabled:opacity-40 disabled:cursor-not-allowed flex items-center justify-center gap-1.5 bg-white/5 hover:bg-white/10 text-white border border-white/10 btn-cyber"
        >
          <span>✏️ {t('edit', language)}</span>
        </button>

        <button
          onClick={() => handleDeleteTrader(trader.trader_id)}
          className="px-3 py-2 rounded-xl text-xs font-bold transition-all duration-300 hover:scale-[1.02] flex items-center justify-center gap-1.5 bg-white/5 hover:bg-white/10 text-white border border-white/10 btn-cyber"
        >
          <Trash2 className="w-3.5 h-3.5" />
          {t('delete', language) || '刪除'}
        </button>
      </div>
    </div>
  );
}

interface AITradersPageProps {
  onTraderSelect?: (traderId: string) => void
}

export function AITradersPage({ onTraderSelect }: AITradersPageProps) {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid')
  const [showEditModal, setShowEditModal] = useState(false)
  const [showModelModal, setShowModelModal] = useState(false)
  const [showExchangeModal, setShowExchangeModal] = useState(false)
  const [showSignalSourceModal, setShowSignalSourceModal] = useState(false)
  const [editingModel, setEditingModel] = useState<string | null>(null)
  const [editingExchange, setEditingExchange] = useState<string | null>(null)
  const [editingTrader, setEditingTrader] = useState<any>(null)
  const [allModels, setAllModels] = useState<AIModel[]>([])
  const [allExchanges, setAllExchanges] = useState<Exchange[]>([])
  const [supportedModels, setSupportedModels] = useState<AIModel[]>([])
  const [modelProviders, setModelProviders] = useState<ModelProvider[]>([])
  const [supportedExchanges, setSupportedExchanges] = useState<Exchange[]>([])
  const [userSignalSource, setUserSignalSource] = useState<{
    coinPoolUrl: string
    oiTopUrl: string
  }>({
    coinPoolUrl: '',
    oiTopUrl: '',
  })

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

  const { data: traders, mutate: mutateTraders } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    { refreshInterval: 5000 }
  )

  // 加载AI模型和交易所配置
  useEffect(() => {
    const loadConfigs = async () => {
      if (!user || !token) {
        // 未登录时只加载公开的支持模型和交易所
        try {
          const [supportedModels, supportedExchanges, providers] = await Promise.all([
            api.getSupportedModels(),
            api.getSupportedExchanges(),
            api.getModelProviders().catch(() => []),
          ])
          setSupportedModels(supportedModels)
          setSupportedExchanges(supportedExchanges)
          setModelProviders(providers)
        } catch (err) {
          console.error('Failed to load supported configs:', err)
        }
        return
      }

      try {
        const [
          modelConfigs,
          exchangeConfigs,
          supportedModels,
          supportedExchanges,
          providers,
        ] = await Promise.all([
          api.getModelConfigs(),
          api.getExchangeConfigs(),
          api.getSupportedModels(),
          api.getSupportedExchanges(),
          api.getModelProviders().catch(() => []),
        ])
        setAllModels(modelConfigs)
        setAllExchanges(exchangeConfigs)
        setSupportedModels(supportedModels)
        setSupportedExchanges(supportedExchanges)
        setModelProviders(providers)

        // 加载用户信号源配置
        try {
          const signalSource = await api.getUserSignalSource()
          setUserSignalSource({
            coinPoolUrl: signalSource.coin_pool_url || '',
            oiTopUrl: signalSource.oi_top_url || '',
          })
        } catch (error) {
          console.log('📡 用户信号源配置暂未设置')
        }
      } catch (error) {
        console.error('Failed to load configs:', error)
      }
    }
    loadConfigs()
  }, [user, token])

  // 显示所有用户的模型和交易所配置（用于调试）
  const configuredModels = allModels || []
  const configuredExchanges = allExchanges || []

  // 只在创建交易员时使用已启用且配置完整的
  const enabledModels =
    allModels?.filter(
      (m) => m.enabled && (m.hasApiKey || !!m.apiKey || !!m.envKey)
    ) || []
  const enabledExchanges =
    allExchanges?.filter((e) => {
      if (!e.enabled) return false

      // Aster 交易所需要特殊字段
      if (e.id === 'aster') {
        return (
          e.asterUser &&
          e.asterUser.trim() !== '' &&
          e.asterSigner &&
          e.asterSigner.trim() !== '' &&
          e.asterPrivateKey &&
          e.asterPrivateKey.trim() !== ''
        )
      }

      // Hyperliquid 只需要私钥（作为apiKey），钱包地址会自动从私钥生成
      if (e.id === 'hyperliquid') {
        return e.apiKey && e.apiKey.trim() !== ''
      }

      // Binance 等其他交易所需要 apiKey 和 secretKey
      return (
        e.apiKey &&
        e.apiKey.trim() !== '' &&
        e.secretKey &&
        e.secretKey.trim() !== ''
      )
    }) || []

  // 检查模型是否正在被运行中的交易员使用
  const isModelInUse = (modelId: string) => {
    return traders?.some((t) => t.ai_model === modelId && t.is_running) || false
  }

  // 检查交易所是否正在被运行中的交易员使用
  const isExchangeInUse = (exchangeId: string) => {
    return (
      traders?.some((t) => t.exchange_id === exchangeId && t.is_running) ||
      false
    )
  }

  const handleCreateTrader = async (data: CreateTraderRequest) => {
    try {
      const model = allModels?.find((m) => m.id === data.ai_model_id)
      const exchange = allExchanges?.find((e) => e.id === data.exchange_id)

      if (!model?.enabled) {
        alert(t('modelNotConfigured', language))
        return
      }

      if (!exchange?.enabled) {
        alert(t('exchangeNotConfigured', language))
        return
      }

      await api.createTrader(data)
      setShowCreateModal(false)
      mutateTraders()
    } catch (error) {
      console.error('Failed to create trader:', error)
      alert(t('createTraderFailed', language))
    }
  }

  const handleEditTrader = async (traderId: string) => {
    try {
      const traderConfig = await api.getTraderConfig(traderId)
      setEditingTrader(traderConfig)
      setShowEditModal(true)
    } catch (error) {
      console.error('Failed to fetch trader config:', error)
      alert(t('getTraderConfigFailed', language))
    }
  }

  const handleSaveEditTrader = async (data: CreateTraderRequest) => {
    if (!editingTrader) return

    try {
      const model = enabledModels?.find((m) => m.id === data.ai_model_id)
      const exchange = enabledExchanges?.find((e) => e.id === data.exchange_id)

      if (!model) {
        alert(t('modelConfigNotExist', language))
        return
      }

      if (!exchange) {
        alert(t('exchangeConfigNotExist', language))
        return
      }

      const request = {
        name: data.name,
        ai_model_id: data.ai_model_id,
        exchange_id: data.exchange_id,
        initial_balance: data.initial_balance,
        scan_interval_minutes: data.scan_interval_minutes,
        btc_eth_leverage: data.btc_eth_leverage,
        altcoin_leverage: data.altcoin_leverage,
        trading_symbols: data.trading_symbols,
        custom_prompt: data.custom_prompt,
        override_base_prompt: data.override_base_prompt,
        system_prompt_template: data.system_prompt_template,
        is_cross_margin: data.is_cross_margin,
        use_coin_pool: data.use_coin_pool,
        use_oi_top: data.use_oi_top,
      }

      await api.updateTrader(editingTrader.trader_id, request)
      setShowEditModal(false)
      setEditingTrader(null)
      mutateTraders()
    } catch (error) {
      console.error('Failed to update trader:', error)
      alert(t('updateTraderFailed', language))
    }
  }

  const handleDeleteTrader = async (traderId: string) => {
    if (!confirm(t('confirmDeleteTrader', language))) return

    try {
      await api.deleteTrader(traderId)
      mutateTraders()
    } catch (error) {
      console.error('Failed to delete trader:', error)
      alert(t('deleteTraderFailed', language))
    }
  }

  const handleReloadPrompts = async () => {
    try {
      const res = await api.reloadPrompts()
      alert(
        `${t('reloadPrompts', language)}: ${(res.templates || []).join(', ') || 'ok'}`
      )
    } catch (error) {
      console.error('Failed to reload prompts:', error)
      alert(t('operationFailed', language))
    }
  }

  const handleSyncBalance = async (traderId: string) => {
    try {
      await api.syncBalance(traderId)
      mutateTraders()
    } catch (error) {
      console.error('Failed to sync balance:', error)
      alert(t('operationFailed', language))
    }
  }

  const handleToggleTrader = async (traderId: string, running: boolean) => {
    try {
      if (running) {
        await api.stopTrader(traderId)
      } else {
        await api.startTrader(traderId)
      }
      mutateTraders()
    } catch (error) {
      console.error('Failed to toggle trader:', error)
      alert(t('operationFailed', language))
    }
  }

  const handleModelClick = (modelId: string) => {
    if (!isModelInUse(modelId)) {
      setEditingModel(modelId)
      setShowModelModal(true)
    }
  }

  const handleExchangeClick = (exchangeId: string) => {
    if (!isExchangeInUse(exchangeId)) {
      setEditingExchange(exchangeId)
      setShowExchangeModal(true)
    }
  }

  const handleDeleteModelConfig = async (modelId: string) => {
    if (!confirm(t('confirmDeleteModel', language))) return

    try {
      const existing = allModels?.find((m) => m.id === modelId)
      await api.updateModelConfigs({
        models: {
          [modelId]: {
            enabled: false,
            api_key: '',
            custom_api_url: '',
            custom_model_name: '',
            env_key: '',
            provider: existing?.provider,
            name: existing?.name,
          },
        },
      })
      const refreshedModels = await api.getModelConfigs()
      setAllModels(refreshedModels)
      setShowModelModal(false)
      setEditingModel(null)
    } catch (error) {
      console.error('Failed to delete model config:', error)
      alert(t('deleteConfigFailed', language))
    }
  }

  const handleSaveModelConfig = async (
    modelId: string,
    apiKey: string,
    customApiUrl?: string,
    customModelName?: string,
    extra?: { envKey?: string; provider?: string; name?: string }
  ) => {
    try {
      await api.updateModelConfigs({
        models: {
          [modelId]: {
            enabled: true,
            api_key: apiKey,
            custom_api_url: customApiUrl || '',
            custom_model_name: customModelName || '',
            env_key: extra?.envKey || '',
            provider: extra?.provider,
            name: extra?.name,
          },
        },
      })

      // 重新获取用户配置以确保数据同步
      const refreshedModels = await api.getModelConfigs()
      setAllModels(refreshedModels)

      setShowModelModal(false)
      setEditingModel(null)
    } catch (error) {
      console.error('Failed to save model config:', error)
      alert(t('saveConfigFailed', language))
    }
  }

  const handleDeleteExchangeConfig = async (exchangeId: string) => {
    if (!confirm(t('confirmDeleteExchange', language))) return

    try {
      const updatedExchanges =
        allExchanges?.map((e) =>
          e.id === exchangeId
            ? { ...e, apiKey: '', secretKey: '', enabled: false }
            : e
        ) || []

      const request = {
        exchanges: Object.fromEntries(
          updatedExchanges.map((exchange) => [
            exchange.id,
            {
              enabled: exchange.enabled,
              api_key: exchange.apiKey || '',
              secret_key: exchange.secretKey || '',
              testnet: exchange.testnet || false,
            },
          ])
        ),
      }

      await api.updateExchangeConfigs(request)
      setAllExchanges(updatedExchanges)
      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch (error) {
      console.error('Failed to delete exchange config:', error)
      alert(t('deleteExchangeConfigFailed', language))
    }
  }

  const handleSaveExchangeConfig = async (
    exchangeId: string,
    apiKey: string,
    secretKey?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string
  ) => {
    try {
      // 找到要配置的交易所（从supportedExchanges中）
      const exchangeToUpdate = supportedExchanges?.find(
        (e) => e.id === exchangeId
      )
      if (!exchangeToUpdate) {
        alert(t('exchangeNotExist', language))
        return
      }

      // 创建或更新用户的交易所配置
      const existingExchange = allExchanges?.find((e) => e.id === exchangeId)
      let updatedExchanges

      if (existingExchange) {
        // 更新现有配置
        updatedExchanges =
          allExchanges?.map((e) =>
            e.id === exchangeId
              ? {
                ...e,
                apiKey,
                secretKey,
                testnet,
                hyperliquidWalletAddr,
                asterUser,
                asterSigner,
                asterPrivateKey,
                enabled: true,
              }
              : e
          ) || []
      } else {
        // 添加新配置
        const newExchange = {
          ...exchangeToUpdate,
          apiKey,
          secretKey,
          testnet,
          hyperliquidWalletAddr,
          asterUser,
          asterSigner,
          asterPrivateKey,
          enabled: true,
        }
        updatedExchanges = [...(allExchanges || []), newExchange]
      }

      const request = {
        exchanges: Object.fromEntries(
          updatedExchanges.map((exchange) => [
            exchange.id,
            {
              enabled: exchange.enabled,
              api_key: exchange.apiKey || '',
              secret_key: exchange.secretKey || '',
              testnet: exchange.testnet || false,
              hyperliquid_wallet_addr: exchange.hyperliquidWalletAddr || '',
              aster_user: exchange.asterUser || '',
              aster_signer: exchange.asterSigner || '',
              aster_private_key: exchange.asterPrivateKey || '',
            },
          ])
        ),
      }

      await api.updateExchangeConfigs(request)

      // 重新获取用户配置以确保数据同步
      const refreshedExchanges = await api.getExchangeConfigs()
      setAllExchanges(refreshedExchanges)

      setShowExchangeModal(false)
    } catch (error) {
      console.error('Failed to save exchange config:', error)
      alert(t('saveConfigFailed', language))
    }
  }

  const handleAddModel = () => {
    setEditingModel(null)
    setShowModelModal(true)
  }

  const handleAddExchange = () => {
    setEditingExchange(null)
    setShowExchangeModal(true)
  }

  const handleSaveSignalSource = async (
    coinPoolUrl: string,
    oiTopUrl: string
  ) => {
    try {
      await api.saveUserSignalSource(coinPoolUrl, oiTopUrl)
      setUserSignalSource({ coinPoolUrl, oiTopUrl })
      setShowSignalSourceModal(false)
    } catch (error) {
      console.error('Failed to save signal source:', error)
      alert(t('saveSignalSourceFailed', language))
    }
  }

  return (
    <div className="space-y-6 md:space-y-8 animate-fade-in relative z-10">
      {/* 🌠 控制中心 Header - Atelier Slate 沉静社论風格 */}
      <div className="sharp-card p-6 flex flex-col lg:flex-row items-start lg:items-center justify-between gap-4 border border-white/5 relative ">
        <div className="flex items-center gap-3 md:gap-4 z-10">
          <div
            className="w-10 h-10 md:w-12 md:h-12 flex items-center justify-center bg-[#17181D] border border-white/10"
          >
            <Bot className="w-5 h-5 md:w-6 md:h-6 text-[#ECEBE6]" />
          </div>
          <div>
            <h1
              className="text-xl md:text-2xl font-serif font-bold flex items-center gap-2.5 text-[#ECEBE6]"
            >
              {t('aiTraders', language)}
              <span
                className="text-[9px] font-mono px-2 py-0.5 border border-white/10 text-[#9C9B96] bg-white/[0.02]"
              >
                {traders?.filter((tr) => tr.is_running).length || 0} {t('active', language)}
              </span>
            </h1>
            <p className="text-xs font-mono text-[#5E5D58] mt-0.5">
              {t('manageAITraders', language)} • AUTONOMOUS AGENTS POOL
            </p>
          </div>
        </div>

        {/* 控制面板按鈕群 (Sharp Minimal Slate Buttons) */}
        <div className="flex gap-2 w-full lg:w-auto overflow-x-auto flex-wrap lg:flex-nowrap pb-1 lg:pb-0 z-10 font-mono text-xs">
          <button
            onClick={handleAddModel}
            className="px-3 py-2 text-xs transition-all flex items-center gap-1.5 whitespace-nowrap bg-[#17181D] hover:bg-white/5 text-[#ECEBE6] border border-white/10"
          >
            <Plus className="w-3.5 h-3.5 text-[#9C9B96]" />
            {t('aiModels', language)}
          </button>

          <button
            onClick={handleAddExchange}
            className="px-3 py-2 text-xs transition-all flex items-center gap-1.5 whitespace-nowrap bg-[#17181D] hover:bg-white/5 text-[#ECEBE6] border border-white/10"
          >
            <Plus className="w-3.5 h-3.5 text-[#9C9B96]" />
            {t('exchanges', language)}
          </button>

          <button
            onClick={handleReloadPrompts}
            className="px-3 py-2 text-xs transition-all whitespace-nowrap bg-[#17181D] hover:bg-white/5 text-[#ECEBE6] border border-white/10"
          >
            {t('reloadPrompts', language)}
          </button>

          <button
            onClick={() => setShowSignalSourceModal(true)}
            className="px-3 py-2 text-xs transition-all whitespace-nowrap bg-[#17181D] hover:bg-white/5 text-[#ECEBE6] border border-white/10"
          >
            📡 {t('signalSource', language)}
          </button>

          <button
            onClick={() => setShowCreateModal(true)}
            disabled={
              configuredModels.length === 0 || configuredExchanges.length === 0
            }
            className={`px-4 py-2 text-xs font-medium transition-all disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-1.5 whitespace-nowrap border ${
              configuredModels.length > 0 && configuredExchanges.length > 0
                ? 'bg-[#ECEBE6] text-[#141416] border-[#ECEBE6] font-bold'
                : 'bg-white/5 text-[#5E5D58] border-white/5'
            }`}
          >
            <Plus className="w-4 h-4" />
            {t('createTrader', language)}
          </button>
        </div>
      </div>

      {/* 信号源配置警告 */}
      {traders &&
        ((traders.some((t) => t.use_coin_pool) && !userSignalSource.coinPoolUrl) ||
         (traders.some((t) => t.use_oi_top) && !userSignalSource.oiTopUrl)) && (
          <div
            className="rounded-xl px-4 py-3.5 flex items-start gap-3 animate-slide-in glass-card relative overflow-hidden"
            style={{
              background: 'rgba(246, 70, 93, 0.05)',
              border: '1px solid rgba(246, 70, 93, 0.25)',
            }}
          >
            <div className="absolute top-0 bottom-0 left-0 w-[3px] bg-rose-500"></div>
            <AlertTriangle
              size={20}
              className="flex-shrink-0 mt-0.5 text-rose-500 animate-pulse"
            />
            <div className="flex-1">
              <div className="font-semibold mb-1 text-rose-400">
                ⚠️ {t('signalSourceNotConfigured', language)}
              </div>
              <div className="text-xs text-gray-400 leading-relaxed">
                <p className="mb-2">
                  {t('signalSourceWarningMessage', language)}
                </p>
                <p className="font-semibold text-gray-300">
                  {t('solutions', language)}
                </p>
                <ul className="list-disc list-inside space-y-1 ml-2 mt-1">
                  <li>点击"📡 {t('signalSource', language)}"按钮配置API地址</li>
                  <li>或在交易员配置中禁用"使用币种池"和"使用OI Top"</li>
                  <li>或在交易员配置中设置自定义币种列表</li>
                </ul>
              </div>
              <button
                onClick={() => setShowSignalSourceModal(true)}
                className="mt-3 px-3 py-1.5 rounded-lg text-xs font-bold transition-all hover:scale-105 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/15"
              >
                {t('configureSignalSourceNow', language)}
              </button>
            </div>
          </div>
        )}

      {/* Configuration Status - Luxury Minimal Studio Rows */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 pt-4">
        {/* AI Models */}
        <div className="space-y-4">
          <div className="flex items-baseline justify-between border-b border-white/[0.08] pb-3">
            <h3 className="text-xl font-playfair font-normal text-[#ECEBE6]">
              AI <span className="luxury-gold-italic">Architectures</span>
            </h3>
            <span className="text-[9px] font-inter uppercase tracking-luxury text-[#7C7A75]">
              {configuredModels.length} CONFIGURED
            </span>
          </div>
          <div className="divide-y divide-white/[0.04]">
            {configuredModels.map((model) => {
              const inUse = isModelInUse(model.id)
              return (
                <div
                  key={model.id}
                  className={`flex items-center justify-between py-3.5 px-1 transition-all duration-300 ${inUse
                    ? 'opacity-40 cursor-not-allowed'
                    : 'cursor-pointer hover:bg-white/[0.015]'
                    }`}
                  onClick={() => handleModelClick(model.id)}
                >
                  <div className="flex items-center gap-3">
                    <div className="min-w-0">
                      <div className="font-playfair font-normal text-base text-[#ECEBE6]">
                        {model.customModelName && model.customModelName.trim() !== '' ? model.customModelName : getShortName(model.name)}
                      </div>
                      <div className="text-[10px] font-mono tracking-wider text-[#7C7A75] mt-0.5">
                        {model.provider?.toUpperCase() || 'PROV'} • {inUse ? 'ACTIVE IN AGENT' : model.enabled ? 'ONLINE' : 'STANDBY'}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className={`w-1.5 h-1.5 rounded-full ${model.enabled && (model.hasApiKey || model.apiKey || model.envKey) ? 'bg-[#6E987E]' : 'bg-[#5E5D58]'}`} />
                  </div>
                </div>
              )
            })}
          </div>
        </div>

        {/* Exchanges */}
        <div className="space-y-4">
          <div className="flex items-baseline justify-between border-b border-white/[0.08] pb-3">
            <h3 className="text-xl font-playfair font-normal text-[#ECEBE6]">
              Execution <span className="luxury-gold-italic">Venues</span>
            </h3>
            <span className="text-[9px] font-inter uppercase tracking-luxury text-[#7C7A75]">
              {configuredExchanges.length} CONFIGURED
            </span>
          </div>
          <div className="divide-y divide-white/[0.04]">
            {configuredExchanges.map((exchange) => {
              const inUse = isExchangeInUse(exchange.id)
              return (
                <div
                  key={exchange.id}
                  className={`flex items-center justify-between py-3.5 px-1 transition-all duration-300 ${inUse
                    ? 'opacity-40 cursor-not-allowed'
                    : 'cursor-pointer hover:bg-white/[0.015]'
                    }`}
                  onClick={() => handleExchangeClick(exchange.id)}
                >
                  <div className="flex items-center gap-3">
                    <div className="min-w-0">
                      <div className="font-playfair font-normal text-base text-[#ECEBE6]">
                        {exchange.id.toUpperCase()}
                      </div>
                      <div className="text-[10px] font-mono tracking-wider text-[#7C7A75] mt-0.5">
                        {exchange.type.toUpperCase()} • {inUse ? 'ACTIVE IN AGENT' : exchange.enabled ? 'ONLINE' : 'STANDBY'}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className={`w-1.5 h-1.5 rounded-full ${exchange.enabled && exchange.apiKey ? 'bg-[#6E987E]' : 'bg-[#5E5D58]'}`} />
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      </div>


      {/* 🚀 Traders Grid List */}
      <div className="glass-card p-5 md:p-8 border border-white/5 relative overflow-hidden">
        {/* 背景裝飾 */}
        <div className="absolute top-0 right-0 w-96 h-96 bg-[radial-gradient(circle_at_top_right,rgba(255,255,255,0.03),transparent_70%)] pointer-events-none"></div>
        <div className="absolute bottom-0 left-0 w-96 h-96 bg-[radial-gradient(circle_at_bottom_left,rgba(255,255,255,0.03),transparent_70%)] pointer-events-none"></div>

        <div className="flex items-center justify-between mb-6 md:mb-8 relative z-10">
          <h2
            className="text-xl md:text-2xl font-bold flex items-center gap-3 text-[#EAECEF]"
          >
            <Users
              className="w-6 h-6 text-white shadow-sm"
            />
            {t('currentTraders', language)}
          </h2>

          {/* Grid/List View Mode Toggle */}
          <div className="flex rounded-lg p-0.5 bg-black/40 border border-white/[0.04] text-[10px] font-mono">
            <button
              onClick={() => setViewMode('grid')}
              className={`px-3 py-1.5 rounded-md transition-all flex items-center gap-1.5 font-bold ${
                viewMode === 'grid'
                  ? 'bg-white/10 text-white border border-white/20'
                  : 'text-gray-500 hover:text-gray-300'
              }`}
            >
              <LayoutGrid className="w-3 h-3" />
              <span className="hidden sm:inline">3D GRID</span>
            </button>
            <button
              onClick={() => setViewMode('list')}
              className={`px-3 py-1.5 rounded-md transition-all flex items-center gap-1.5 font-bold ${
                viewMode === 'list'
                  ? 'bg-white/10 text-white border border-white/20'
                  : 'text-gray-500 hover:text-gray-300'
              }`}
            >
              <List className="w-3 h-3" />
              <span className="hidden sm:inline">RADAR LIST</span>
            </button>
          </div>
        </div>

        {traders && traders.length > 0 ? (
          viewMode === 'grid' ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 relative z-10">
              {traders.map((trader) => (
                <AITraderCard
                  key={trader.trader_id}
                  trader={trader}
                  isRunning={trader.is_running || false}
                  language={language}
                  onTraderSelect={onTraderSelect}
                  handleToggleTrader={handleToggleTrader}
                  handleSyncBalance={handleSyncBalance}
                  handleEditTrader={handleEditTrader}
                  handleDeleteTrader={handleDeleteTrader}
                  getModelDisplayName={getModelShowName}
                  getExchangeIcon={getExchangeIcon}
                  t={t}
                />
              ))}
            </div>
          ) : (
            /* 📟 Tactical Telemetry Radar List Mode */
            <div className="relative overflow-x-auto rounded-xl border border-white/5 bg-[#131418] z-10 font-mono text-xs">
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="border-b border-white/5 bg-white/[0.02] text-gray-500 font-bold uppercase tracking-wider">
                    <th className="py-3.5 px-4 text-center w-16">Status</th>
                    <th className="py-3.5 px-4">Engine Name</th>
                    <th className="py-3.5 px-4">AI Model</th>
                    <th className="py-3.5 px-4">Exchange Config</th>
                    <th className="py-3.5 px-4">Asset Matrix</th>
                    <th className="py-3.5 px-4 text-right pr-6 w-44">Operations</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-white/5">
                  {traders.map((trader) => {
                    const isRunning = trader.is_running || false;
                    const exchangeName = trader.exchange_id || 'Unknown';
                    const strategyMode = trader.use_coin_pool ? 'COIN POOL' : (trader.use_oi_top ? 'OI TOP' : 'STRATEGY');

                    return (
                      <tr 
                        key={trader.trader_id}
                        className="hover:bg-white/[0.02] transition-colors duration-200 align-middle group"
                      >
                        {/* Status Light */}
                        <td className="py-4 px-4 text-center">
                          <div className="inline-flex items-center justify-center">
                            <span className={isRunning ? 'pulse-dot-green' : 'pulse-dot-red'}></span>
                          </div>
                        </td>

                        {/* Engine Name */}
                        <td className="py-4 px-4 font-bold text-[#EAECEF] hover:text-white transition-colors cursor-pointer" onClick={() => onTraderSelect?.(trader.trader_id)}>
                          <div className="flex items-center gap-2">
                            <Bot className={`w-4 h-4 ${isRunning ? 'text-white animate-pulse' : 'text-gray-500'}`} />
                            <span className="tracking-wide">{trader.trader_name}</span>
                          </div>
                        </td>

                        {/* Model */}
                        <td className="py-4 px-4">
                          <span 
                            className="px-2 py-0.5 rounded text-[10px] font-bold border"
                            style={{
                              color: '#FFFFFF',
                              borderColor: 'rgba(255,255,255,0.15)',
                              background: 'rgba(255,255,255,0.03)',
                            }}
                          >
                            {getModelShowName(trader.ai_model).toUpperCase()}
                          </span>
                        </td>

                        {/* Exchange */}
                        <td className="py-4 px-4 text-gray-300">
                          <span className="flex items-center gap-1.5">
                            {getExchangeIcon(exchangeName.toLowerCase(), { width: 14, height: 14 }) || <span>🔌</span>}
                            <span className="uppercase font-semibold text-[11px]">{exchangeName}</span>
                          </span>
                        </td>

                        {/* Active Symbols */}
                        <td className="py-4 px-4 text-gray-400">
                          <span className="text-white font-bold">{strategyMode}</span>
                        </td>

                        {/* Operations */}
                        <td className="py-4 px-4 text-right pr-6">
                          <div className="flex items-center justify-end gap-2.5">
                            <button
                              onClick={() => handleToggleTrader(trader.trader_id, isRunning)}
                              className="px-2.5 py-1 rounded text-[10px] font-bold uppercase transition-all hover:scale-105"
                              style={
                                isRunning
                                  ? { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D', border: '1px solid rgba(246,70,93,0.2)' }
                                  : { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81', border: '1px solid rgba(14,203,129,0.2)' }
                              }
                            >
                              {isRunning ? t('stop', language) : t('start', language)}
                            </button>
                            <button
                              onClick={() => handleEditTrader(trader.trader_id)}
                              className="px-2 py-1 rounded text-gray-400 hover:text-white border border-white/5 hover:border-white/20 transition-all"
                            >
                              ⚙️
                            </button>
                            <button
                              onClick={() => handleDeleteTrader(trader.trader_id)}
                              className="px-2 py-1 rounded text-xs transition-all hover:scale-105 bg-[#f6465d]/5 text-[#f6465d] border border-[#f6465d]/10 hover:bg-[#f6465d]/10"
                            >
                              <Trash2 className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )
        ) : (
          <div
            className="text-center py-16 text-gray-500"
          >
            <Bot className="w-24 h-24 mx-auto mb-4 opacity-30" />
            <div className="text-lg font-bold mb-2">
              {t('noTraders', language)}
            </div>
            <div className="text-xs mb-4">
              {t('createFirstTrader', language)}
            </div>
            {(configuredModels.length === 0 ||
              configuredExchanges.length === 0) && (
                <div className="text-xs text-white font-semibold">
                  {configuredModels.length === 0 &&
                    configuredExchanges.length === 0
                    ? t('configureModelsAndExchangesFirst', language)
                    : configuredModels.length === 0
                      ? t('configureModelsFirst', language)
                      : t('configureExchangesFirst', language)}
                </div>
              )}
          </div>
        )}
      </div>

      {/* Create Trader Modal */}
      {showCreateModal && (
        <TraderConfigModal
          isOpen={showCreateModal}
          isEditMode={false}
          availableModels={enabledModels}
          availableExchanges={enabledExchanges}
          onSave={handleCreateTrader}
          onClose={() => setShowCreateModal(false)}
        />
      )}

      {/* Edit Trader Modal */}
      {showEditModal && editingTrader && (
        <TraderConfigModal
          isOpen={showEditModal}
          isEditMode={true}
          traderData={editingTrader}
          availableModels={enabledModels}
          availableExchanges={enabledExchanges}
          onSave={handleSaveEditTrader}
          onClose={() => {
            setShowEditModal(false)
            setEditingTrader(null)
          }}
        />
      )}

      {/* Model Configuration Modal */}
      {showModelModal && (
        <ModelConfigModal
          allModels={supportedModels}
          configuredModels={allModels}
          providers={modelProviders}
          editingModelId={editingModel}
          onSave={handleSaveModelConfig}
          onDelete={handleDeleteModelConfig}
          onClose={() => {
            setShowModelModal(false)
            setEditingModel(null)
          }}
          language={language}
        />
      )}

      {/* Exchange Configuration Modal */}
      {showExchangeModal && (
        <ExchangeConfigModal
          allExchanges={supportedExchanges}
          editingExchangeId={editingExchange}
          onSave={handleSaveExchangeConfig}
          onDelete={handleDeleteExchangeConfig}
          onClose={() => {
            setShowExchangeModal(false)
            setEditingExchange(null)
          }}
          language={language}
        />
      )}

      {/* Signal Source Configuration Modal */}
      {showSignalSourceModal && (
        <SignalSourceModal
          coinPoolUrl={userSignalSource.coinPoolUrl}
          oiTopUrl={userSignalSource.oiTopUrl}
          onSave={handleSaveSignalSource}
          onClose={() => setShowSignalSourceModal(false)}
          language={language}
        />
      )}
    </div>
  )
}

// Tooltip Helper Component
function Tooltip({
  content,
  children,
}: {
  content: string
  children: React.ReactNode
}) {
  const [show, setShow] = useState(false)

  return (
    <div className="relative inline-block">
      <div
        onMouseEnter={() => setShow(true)}
        onMouseLeave={() => setShow(false)}
        onClick={() => setShow(!show)}
      >
        {children}
      </div>
      {show && (
        <div
          className="absolute z-10 px-3 py-2 text-sm rounded-lg shadow-lg w-64 left-1/2 transform -translate-x-1/2 bottom-full mb-2"
          style={{
            background: '#1E2329',
            color: '#EAECEF',
            border: '1px solid #474D57',
          }}
        >
          {content}
          <div
            className="absolute left-1/2 transform -translate-x-1/2 top-full"
            style={{
              width: 0,
              height: 0,
              borderLeft: '6px solid transparent',
              borderRight: '6px solid transparent',
              borderTop: '6px solid #1E2329',
            }}
          />
        </div>
      )}
    </div>
  )
}

// Signal Source Configuration Modal Component
function SignalSourceModal({
  coinPoolUrl,
  oiTopUrl,
  onSave,
  onClose,
  language,
}: {
  coinPoolUrl: string
  oiTopUrl: string
  onSave: (coinPoolUrl: string, oiTopUrl: string) => void
  onClose: () => void
  language: Language
}) {
  const [coinPool, setCoinPool] = useState(coinPoolUrl || '')
  const [oiTop, setOiTop] = useState(oiTopUrl || '')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSave(coinPool.trim(), oiTop.trim())
  }

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50 p-4">
      <div
        className="rounded-xl p-6 w-full max-w-lg relative border border-white/10"
        style={{ background: '#161A1E' }}
      >
        <h3 className="text-xl font-bold mb-4" style={{ color: '#EAECEF' }}>
          📡 {t('signalSourceConfig', language)}
        </h3>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: '#EAECEF' }}
            >
              COIN POOL URL
            </label>
            <input
              type="url"
              value={coinPool}
              onChange={(e) => setCoinPool(e.target.value)}
              placeholder="https://api.example.com/coinpool"
              className="w-full px-3 py-2 rounded border"
              style={{
                background: '#0B0E11',
                borderColor: '#2B3139',
                color: '#EAECEF',
              }}
            />
            <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
              {t('coinPoolDescription', language)}
            </div>
          </div>

          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: '#EAECEF' }}
            >
              OI TOP URL
            </label>
            <input
              type="url"
              value={oiTop}
              onChange={(e) => setOiTop(e.target.value)}
              placeholder="https://api.example.com/oitop"
              className="w-full px-3 py-2 rounded"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
            <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
              {t('oiTopDescription', language)}
            </div>
          </div>

          <div
            className="p-4 rounded"
            style={{
              background: 'rgba(255, 255, 255, 0.03)',
              border: '1px solid rgba(255, 255, 255, 0.08)',
            }}
          >
            <div
              className="text-sm font-semibold mb-2"
              style={{ color: '#FFFFFF' }}
            >
              ℹ️ {t('information', language)}
            </div>
            <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
              <div>{t('signalSourceInfo1', language)}</div>
              <div>{t('signalSourceInfo2', language)}</div>
              <div>{t('signalSourceInfo3', language)}</div>
            </div>
          </div>

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              className="flex-1 px-4 py-2 rounded text-sm font-semibold hover:bg-gray-200 transition-colors"
              style={{ background: '#FFFFFF', color: '#000' }}
            >
              {t('save', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// Model Configuration Modal Component
function ModelConfigModal({
  configuredModels,
  providers,
  editingModelId,
  onSave,
  onDelete,
  onClose,
  language,
}: {
  allModels: AIModel[]
  configuredModels: AIModel[]
  providers: ModelProvider[]
  editingModelId: string | null
  onSave: (
    modelId: string,
    apiKey: string,
    baseUrl?: string,
    modelName?: string,
    extra?: { envKey?: string; provider?: string; name?: string }
  ) => void
  onDelete: (modelId: string) => void
  onClose: () => void
  language: Language
}) {
  const editing = editingModelId
    ? configuredModels?.find((m) => m.id === editingModelId)
    : undefined
  const [presetId, setPresetId] = useState(
    editing?.provider || providers[0]?.id || 'custom'
  )
  const [apiKey, setApiKey] = useState(editing?.apiKey || '')
  const [envKey, setEnvKey] = useState(editing?.envKey || '')
  const [baseUrl, setBaseUrl] = useState(editing?.customApiUrl || '')
  const [modelName, setModelName] = useState(editing?.customModelName || '')
  const [probed, setProbed] = useState<string[]>([])
  const [probeError, setProbeError] = useState('')
  const [probing, setProbing] = useState(false)

  const preset = providers.find((p) => p.id === presetId)

  useEffect(() => {
    if (editing) return
    if (!preset) return
    setBaseUrl(preset.base_url || '')
    setEnvKey(preset.env_key || '')
    if (preset.default_model) setModelName(preset.default_model)
    setProbed(preset.suggested_models || [])
    setProbeError('')
  }, [presetId, editing, preset])

  const canSave =
    !!presetId &&
    (!!apiKey.trim() || !!envKey.trim() || editing?.hasApiKey || editing?.apiKey === '***')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!canSave) return
    const id =
      editingModelId || `${Date.now()}_${presetId}`
    onSave(id, apiKey.trim(), baseUrl.trim() || undefined, modelName.trim() || undefined, {
      envKey: envKey.trim(),
      provider: presetId,
      name: preset?.name || presetId,
    })
  }

  const handleProbe = async () => {
    setProbing(true)
    setProbeError('')
    try {
      const result = await api.probeModels({
        model_id: editingModelId || undefined,
        provider: presetId,
        base_url: baseUrl.trim(),
        api_key: apiKey.trim(),
        env_key: envKey.trim(),
      })
      if (!result.ok) {
        setProbeError(result.error || 'probe failed')
        setProbed([])
        return
      }
      setProbed(result.models || [])
      if (!modelName && result.models && result.models.length > 0) {
        setModelName(result.models[0])
      }
    } catch (err: any) {
      setProbeError(err?.message || 'probe failed')
    } finally {
      setProbing(false)
    }
  }

  const inputStyle = {
    background: '#0B0E11',
    border: '1px solid #2B3139',
    color: '#EAECEF',
  } as const

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div
        className="bg-gray-800 rounded-lg p-6 w-full max-w-lg relative max-h-[90vh] overflow-y-auto"
        style={{ background: '#1E2329' }}
      >
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingModelId
              ? t('editAIModel', language)
              : t('addAIModel', language)}
          </h3>
          {editingModelId && (
            <button
              type="button"
              onClick={() => {
                if (confirm(t('confirmDeleteModel', language))) {
                  onDelete(editingModelId)
                }
              }}
              className="p-2 rounded hover:bg-red-100 transition-colors"
              style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
              title={t('deleteConfigFailed', language)}
            >
              <Trash2 className="w-4 h-4" />
            </button>
          )}
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              {t('providerPreset', language)}
            </label>
            <select
              value={presetId}
              onChange={(e) => setPresetId(e.target.value)}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
              required
            >
              {(providers.length ? providers : [{ id: 'custom', name: 'Custom' } as ModelProvider]).map(
                (p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                    {p.local ? ' (local)' : ''}
                  </option>
                )
              )}
            </select>
            {editingModelId && (
              <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                id: {editingModelId}
              </div>
            )}
          </div>

          {preset?.notes && (
            <div className="text-xs p-3 rounded" style={{ background: '#0B0E11', color: '#F0B90B' }}>
              {preset.notes}
            </div>
          )}

          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              API Key
            </label>
            <input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder={t('enterAPIKey', language)}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
            <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
              {t('apiKeyOrEnvHint', language)}
            </div>
          </div>

          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              {t('envKey', language)}
            </label>
            <input
              type="text"
              value={envKey}
              onChange={(e) => setEnvKey(e.target.value)}
              placeholder="NVIDIA_API_KEY"
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
            <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
              {t('envKeyHint', language)}
            </div>
          </div>

          <div>
            <label className="block text-sm font-semibold mb-2" style={{ color: '#EAECEF' }}>
              {t('customBaseURL', language)}
            </label>
            <input
              type="text"
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder={t('customBaseURLPlaceholder', language)}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
            <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
              {t('leaveBlankForDefault', language)}
            </div>
          </div>

          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-sm font-semibold" style={{ color: '#EAECEF' }}>
                Model
              </label>
              <button
                type="button"
                onClick={handleProbe}
                disabled={probing}
                className="text-xs px-2 py-1 rounded"
                style={{ background: '#2B3139', color: '#EAECEF' }}
              >
                {probing ? '...' : t('fetchModels', language)}
              </button>
            </div>
            {probed.length > 0 ? (
              <select
                value={modelName}
                onChange={(e) => setModelName(e.target.value)}
                className="w-full px-3 py-2 rounded mb-2"
                style={inputStyle}
              >
                <option value="">{t('pleaseSelectModel', language)}</option>
                {probed.map((id) => (
                  <option key={id} value={id}>
                    {id}
                  </option>
                ))}
              </select>
            ) : null}
            <input
              type="text"
              value={modelName}
              onChange={(e) => setModelName(e.target.value)}
              placeholder="deepseek-chat / qwen3.8-27b / google/gemma-4-12b"
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
            {probeError && (
              <div className="text-xs mt-1" style={{ color: '#F6465D' }}>
                {probeError}
              </div>
            )}
          </div>

          <div
            className="p-4 rounded"
            style={{
              background: 'rgba(255, 255, 255, 0.03)',
              border: '1px solid rgba(255, 255, 255, 0.08)',
            }}
          >
            <div className="text-sm font-semibold mb-2" style={{ color: '#FFFFFF' }}>
              ℹ️ {t('information', language)}
            </div>
            <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
              <div>{t('modelConfigInfo1', language)}</div>
              <div>{t('modelConfigInfo2', language)}</div>
              <div>{t('modelConfigInfo4', language)}</div>
              <div>{t('modelConfigInfo3', language)}</div>
            </div>
          </div>

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={!canSave}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50 hover:bg-gray-200 transition-colors"
              style={{ background: '#FFFFFF', color: '#000' }}
            >
              {t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// Exchange Configuration Modal Component
function ExchangeConfigModal({
  allExchanges,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language,
}: {
  allExchanges: Exchange[]
  editingExchangeId: string | null
  onSave: (
    exchangeId: string,
    apiKey: string,
    secretKey?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string
  ) => Promise<void>
  onDelete: (exchangeId: string) => void
  onClose: () => void
  language: Language
}) {
  const [selectedExchangeId, setSelectedExchangeId] = useState(
    editingExchangeId || ''
  )
  const [apiKey, setApiKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [testnet, setTestnet] = useState(false)
  const [showGuide, setShowGuide] = useState(false)
  const [serverIP, setServerIP] = useState<{
    public_ip: string
    message: string
  } | null>(null)
  const [loadingIP, setLoadingIP] = useState(false)
  const [copiedIP, setCopiedIP] = useState(false)

  // 币安配置指南展开状态
  const [showBinanceGuide, setShowBinanceGuide] = useState(false)

  // Aster 特定字段
  const [asterUser, setAsterUser] = useState('')
  const [asterSigner, setAsterSigner] = useState('')
  const [asterPrivateKey, setAsterPrivateKey] = useState('')

  // 获取当前编辑的交易所信息
  const selectedExchange = allExchanges?.find(
    (e) => e.id === selectedExchangeId
  )

  // 如果是编辑现有交易所，初始化表单数据
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      setApiKey(selectedExchange.apiKey || '')
      setSecretKey(selectedExchange.secretKey || '')
      setPassphrase('') // Don't load existing passphrase for security
      setTestnet(selectedExchange.testnet || false)

      // Aster 字段
      setAsterUser(selectedExchange.asterUser || '')
      setAsterSigner(selectedExchange.asterSigner || '')
      setAsterPrivateKey('') // Don't load existing private key for security
    }
  }, [editingExchangeId, selectedExchange])

  // 加载服务器IP（当选择binance时）
  useEffect(() => {
    if (selectedExchangeId === 'binance' && !serverIP) {
      setLoadingIP(true)
      api
        .getServerIP()
        .then((data) => {
          setServerIP(data)
        })
        .catch((err) => {
          console.error('Failed to load server IP:', err)
        })
        .finally(() => {
          setLoadingIP(false)
        })
    }
  }, [selectedExchangeId])

  const handleCopyIP = (ip: string) => {
    navigator.clipboard.writeText(ip).then(() => {
      setCopiedIP(true)
      setTimeout(() => setCopiedIP(false), 2000)
    })
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedExchangeId) return

    // 根据交易所类型验证不同字段
    if (selectedExchange?.id === 'binance') {
      if (!apiKey.trim() || !secretKey.trim()) return
      await onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet)
    } else if (selectedExchange?.id === 'hyperliquid') {
      if (!apiKey.trim()) return // 只验证私钥，钱包地址自动从私钥生成
      await onSave(selectedExchangeId, apiKey.trim(), '', testnet, '') // 传空字符串，后端自动生成地址
    } else if (selectedExchange?.id === 'aster') {
      if (!asterUser.trim() || !asterSigner.trim() || !asterPrivateKey.trim())
        return
      await onSave(
        selectedExchangeId,
        '',
        '',
        testnet,
        undefined,
        asterUser.trim(),
        asterSigner.trim(),
        asterPrivateKey.trim()
      )
    } else if (selectedExchange?.id === 'okx') {
      if (!apiKey.trim() || !secretKey.trim() || !passphrase.trim()) return
      await onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet)
    } else {
      // 默认情况（其他CEX交易所）
      if (!apiKey.trim() || !secretKey.trim()) return
      await onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet)
    }
  }

  // 可选择的交易所列表（所有支持的交易所）
  const availableExchanges = allExchanges || []

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div
        className="bg-gray-800 rounded-lg p-6 w-full max-w-lg relative"
        style={{ background: '#1E2329' }}
      >
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingExchangeId
              ? t('editExchange', language)
              : t('addExchange', language)}
          </h3>
          <div className="flex items-center gap-2">
            {selectedExchange?.id === 'binance' && (
              <button
                type="button"
                onClick={() => setShowGuide(true)}
                className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105 flex items-center gap-2"
                style={{
                  background: 'rgba(255, 255, 255, 0.08)',
                  color: '#FFFFFF',
                  border: '1px solid rgba(255, 255, 255, 0.15)',
                }}
              >
                <BookOpen className="w-4 h-4" />
                {t('viewGuide', language)}
              </button>
            )}
            {editingExchangeId && (
              <button
                type="button"
                onClick={() => {
                  if (confirm(t('confirmDeleteExchange', language))) {
                    onDelete(editingExchangeId)
                  }
                }}
                className="p-2 rounded hover:bg-red-100 transition-colors"
                style={{
                  background: 'rgba(246, 70, 93, 0.1)',
                  color: '#F6465D',
                }}
                title={t('deleteConfigFailed', language)}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {!editingExchangeId && (
            <div>
              <label
                className="block text-sm font-semibold mb-2"
                style={{ color: '#EAECEF' }}
              >
                {t('selectExchange', language)}
              </label>
              <select
                value={selectedExchangeId}
                onChange={(e) => setSelectedExchangeId(e.target.value)}
                className="w-full px-3 py-2 rounded"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
                required
              >
                <option value="">{t('pleaseSelectExchange', language)}</option>
                {availableExchanges.map((exchange) => (
                  <option key={exchange.id} value={exchange.id}>
                    {getShortName(exchange.name)} ({exchange.type.toUpperCase()}
                    )
                  </option>
                ))}
              </select>
            </div>
          )}

          {selectedExchange && (
            <div
              className="p-4 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
            >
              <div className="flex items-center gap-3 mb-3">
                <div className="w-8 h-8 flex items-center justify-center">
                  {getExchangeIcon(selectedExchange.id, {
                    width: 32,
                    height: 32,
                  })}
                </div>
                <div>
                  <div className="font-semibold" style={{ color: '#EAECEF' }}>
                    {getShortName(selectedExchange.name)}
                  </div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {selectedExchange.type.toUpperCase()} •{' '}
                    {selectedExchange.id}
                  </div>
                </div>
              </div>
            </div>
          )}

          {selectedExchange && (
            <>
              {/* Binance 和其他 CEX 交易所的字段 */}
              {(selectedExchange.id === 'binance' ||
                selectedExchange.type === 'cex') &&
                selectedExchange.id !== 'hyperliquid' &&
                selectedExchange.id !== 'aster' && (
                  <>
                    {/* 币安用户配置提示 (D1 方案) */}
                    {selectedExchange.id === 'binance' && (
                      <div
                        className="mb-4 p-3 rounded cursor-pointer transition-colors"
                        style={{
                          background: '#1a3a52',
                          border: '1px solid #2b5278',
                        }}
                        onClick={() => setShowBinanceGuide(!showBinanceGuide)}
                      >
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            <span style={{ color: '#58a6ff' }}>ℹ️</span>
                            <span
                              className="text-sm font-medium"
                              style={{ color: '#EAECEF' }}
                            >
                              <strong>币安用户必读：</strong>
                              使用「现货与合约交易」API，不要用「统一账户 API」
                            </span>
                          </div>
                          <span style={{ color: '#8b949e' }}>
                            {showBinanceGuide ? '▲' : '▼'}
                          </span>
                        </div>

                        {/* 展开的详细说明 */}
                        {showBinanceGuide && (
                          <div
                            className="mt-3 pt-3"
                            style={{
                              borderTop: '1px solid #2b5278',
                              fontSize: '0.875rem',
                              color: '#c9d1d9',
                            }}
                            onClick={(e) => e.stopPropagation()}
                          >
                            <p className="mb-2" style={{ color: '#8b949e' }}>
                              <strong>原因：</strong>统一账户 API
                              权限结构不同，会导致订单提交失败
                            </p>

                            <p
                              className="font-semibold mb-1"
                              style={{ color: '#EAECEF' }}
                            >
                              正确配置步骤：
                            </p>
                            <ol
                              className="list-decimal list-inside space-y-1 mb-3"
                              style={{ paddingLeft: '0.5rem' }}
                            >
                              <li>
                                登录币安 → 个人中心 → <strong>API 管理</strong>
                              </li>
                              <li>
                                创建 API → 选择「
                                <strong>系统生成的 API 密钥</strong>」
                              </li>
                              <li>
                                勾选「<strong>现货与合约交易</strong>」（
                                <span style={{ color: '#f85149' }}>
                                  不选统一账户
                                </span>
                                ）
                              </li>
                              <li>
                                IP 限制选「<strong>无限制</strong>」或添加服务器
                                IP
                              </li>
                            </ol>

                            <p
                              className="mb-2 p-2 rounded"
                              style={{
                                background: '#3d2a00',
                                border: '1px solid #9e6a03',
                              }}
                            >
                              💡 <strong>多资产模式用户注意：</strong>
                              如果您开启了多资产模式，将强制使用全仓模式。建议关闭多资产模式以支持逐仓交易。
                            </p>

                            <a
                              href="https://www.binance.com/zh-CN/support/faq/how-to-create-api-keys-on-binance-360002502072"
                              target="_blank"
                              rel="noopener noreferrer"
                              className="inline-block text-sm hover:underline"
                              style={{ color: '#58a6ff' }}
                            >
                              📖 查看币安官方教程 ↗
                            </a>
                          </div>
                        )}
                      </div>
                    )}

                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('apiKey', language)}
                      </label>
                      <input
                        type="password"
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        placeholder={t('enterAPIKey', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('secretKey', language)}
                      </label>
                      <input
                        type="password"
                        value={secretKey}
                        onChange={(e) => setSecretKey(e.target.value)}
                        placeholder={t('enterSecretKey', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    {selectedExchange.id === 'okx' && (
                      <div>
                        <label
                          className="block text-sm font-semibold mb-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {t('passphrase', language)}
                        </label>
                        <input
                          type="password"
                          value={passphrase}
                          onChange={(e) => setPassphrase(e.target.value)}
                          placeholder={t('enterPassphrase', language)}
                          className="w-full px-3 py-2 rounded"
                          style={{
                            background: '#0B0E11',
                            border: '1px solid #2B3139',
                            color: '#EAECEF',
                          }}
                          required
                        />
                      </div>
                    )}

                    {/* Binance 白名单IP提示 */}
                    {selectedExchange.id === 'binance' && (
                      <div
                        className="p-4 rounded"
                        style={{
                          background: 'rgba(255, 255, 255, 0.04)',
                          border: '1px solid rgba(255, 255, 255, 0.08)',
                        }}
                      >
                        <div
                          className="text-sm font-semibold mb-2"
                          style={{ color: '#FFFFFF' }}
                        >
                          {t('whitelistIP', language)}
                        </div>
                        <div
                          className="text-xs mb-3"
                          style={{ color: '#848E9C' }}
                        >
                          {t('whitelistIPDesc', language)}
                        </div>

                        {loadingIP ? (
                          <div className="text-xs" style={{ color: '#848E9C' }}>
                            {t('loadingServerIP', language)}
                          </div>
                        ) : serverIP && serverIP.public_ip ? (
                          <div
                            className="flex items-center gap-2 p-2 rounded"
                            style={{ background: '#0B0E11' }}
                          >
                            <code
                              className="flex-1 text-sm font-mono"
                              style={{ color: '#FFFFFF' }}
                            >
                              {serverIP.public_ip}
                            </code>
                            <button
                              type="button"
                              onClick={() => handleCopyIP(serverIP.public_ip)}
                              className="px-3 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
                              style={{
                                background: 'rgba(255, 255, 255, 0.08)',
                                color: '#FFFFFF',
                              }}
                            >
                              {copiedIP
                                ? t('ipCopied', language)
                                : t('copyIP', language)}
                            </button>
                          </div>
                        ) : null}
                      </div>
                    )}
                  </>
                )}

              {/* Hyperliquid 交易所的字段 */}
              {selectedExchange.id === 'hyperliquid' && (
                <>
                  <div>
                    <label
                      className="block text-sm font-semibold mb-2"
                      style={{ color: '#EAECEF' }}
                    >
                      {t('privateKey', language)}
                    </label>
                    <input
                      type="password"
                      value={apiKey}
                      onChange={(e) => setApiKey(e.target.value)}
                      placeholder={t('enterPrivateKey', language)}
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                      required
                    />
                    <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                      {t('hyperliquidPrivateKeyDesc', language)}
                    </div>
                  </div>
                </>
              )}

              {/* Aster 交易所的字段 */}
              {selectedExchange.id === 'aster' && (
                <>
                  <div>
                    <label
                      className="text-sm font-semibold mb-2 flex items-center gap-2"
                      style={{ color: '#EAECEF' }}
                    >
                      {t('user', language)}
                      <Tooltip content={t('asterUserDesc', language)}>
                        <HelpCircle
                          className="w-4 h-4 cursor-help"
                          style={{ color: '#848E9C' }}
                        />
                      </Tooltip>
                    </label>
                    <input
                      type="text"
                      value={asterUser}
                      onChange={(e) => setAsterUser(e.target.value)}
                      placeholder={t('enterUser', language)}
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                      required
                    />
                  </div>

                  <div>
                    <label
                      className="text-sm font-semibold mb-2 flex items-center gap-2"
                      style={{ color: '#EAECEF' }}
                    >
                      {t('signer', language)}
                      <Tooltip content={t('asterSignerDesc', language)}>
                        <HelpCircle
                          className="w-4 h-4 cursor-help"
                          style={{ color: '#848E9C' }}
                        />
                      </Tooltip>
                    </label>
                    <input
                      type="text"
                      value={asterSigner}
                      onChange={(e) => setAsterSigner(e.target.value)}
                      placeholder={t('enterSigner', language)}
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                    />
                  </div>

                  <div>
                    <label
                      className="text-sm font-semibold mb-2 flex items-center gap-2"
                      style={{ color: '#EAECEF' }}
                    >
                      {t('privateKey', language)}
                      <Tooltip content={t('asterPrivateKeyDesc', language)}>
                        <HelpCircle
                          className="w-4 h-4 cursor-help"
                          style={{ color: '#848E9C' }}
                        />
                      </Tooltip>
                    </label>
                    <input
                      type="password"
                      value={asterPrivateKey}
                      onChange={(e) => setAsterPrivateKey(e.target.value)}
                      placeholder={t('enterPrivateKey', language)}
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                    />
                  </div>
                </>
              )}

              <div>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={testnet}
                    onChange={(e) => setTestnet(e.target.checked)}
                    className="form-checkbox rounded"
                    style={{ accentColor: '#FFFFFF' }}
                  />
                  <span style={{ color: '#EAECEF' }}>
                    {t('useTestnet', language)}
                  </span>
                </label>
                <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                  {t('testnetDescription', language)}
                </div>
              </div>

              <div
                className="p-4 rounded"
                style={{
                  background: 'rgba(255, 255, 255, 0.03)',
                  border: '1px solid rgba(255, 255, 255, 0.08)',
                }}
              >
                <div
                  className="text-sm font-semibold mb-2"
                  style={{ color: '#FFFFFF' }}
                >
                  <span className="inline-flex items-center gap-1">
                    <AlertTriangle className="w-4 h-4" />{' '}
                    {t('securityWarning', language)}
                  </span>
                </div>
                <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                  {selectedExchange.id === 'aster' && (
                    <div>{t('asterUsdtWarning', language)}</div>
                  )}
                  <div>{t('exchangeConfigWarning1', language)}</div>
                  <div>{t('exchangeConfigWarning2', language)}</div>
                  <div>{t('exchangeConfigWarning3', language)}</div>
                </div>
              </div>
            </>
          )}

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={false}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50 hover:bg-gray-200 transition-colors"
              style={{ background: '#FFFFFF', color: '#000' }}
            >
              {t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>

      {/* Binance Setup Guide Modal */}
      {showGuide && (
        <div
          className="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4"
          onClick={() => setShowGuide(false)}
        >
          <div
            className="bg-gray-800 rounded-lg p-6 w-full max-w-4xl relative"
            style={{ background: '#1E2329' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                <BookOpen className="w-6 h-6" style={{ color: '#FFFFFF' }} />
                {t('binanceSetupGuide', language)}
              </h3>
              <button
                onClick={() => setShowGuide(false)}
                className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{ background: '#2B3139', color: '#848E9C' }}
              >
                {t('closeGuide', language)}
              </button>
            </div>
            <div className="overflow-y-auto max-h-[80vh]">
              <img
                src="/images/guide.png"
                alt={t('binanceSetupGuide', language)}
                className="w-full h-auto rounded"
              />
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
