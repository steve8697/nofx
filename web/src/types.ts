export interface SystemStatus {
  trader_id: string
  trader_name: string
  ai_model: string
  is_running: boolean
  start_time: string
  runtime_minutes: number
  call_count: number
  initial_balance: number
  scan_interval: string
  stop_until: string
  last_reset_time: string
  ai_provider: string
  risk_halted?: boolean
  consecutive_wait?: number
  daily_pnl?: number
  injected_skills?: string[]
  operator_pause_opens?: boolean
  operator_pause_actor?: string
  operator_pause_until?: string
  operator_note_count?: number
}

export interface OperatorEvent {
  id: number
  ts: string
  actor: string
  action: string
  note: string
  expires_at?: string | null
}

export interface OperatorDirective {
  pause_opens: boolean
  pause_until?: string | null
  pause_actor?: string
  notes: OperatorEvent[]
}

export interface AccountInfo {
  total_equity: number
  wallet_balance: number
  unrealized_profit: number
  available_balance: number
  total_pnl: number
  total_pnl_pct: number
  total_unrealized_pnl: number
  initial_balance: number
  daily_pnl: number
  position_count: number
  margin_used: number
  margin_used_pct: number
}

export interface Position {
  symbol: string
  side: string
  entry_price: number
  mark_price: number
  quantity: number
  leverage: number
  unrealized_pnl: number
  unrealized_pnl_pct: number
  liquidation_price: number
  margin_used: number
}

export interface DecisionAction {
  action: string
  symbol: string
  quantity: number
  leverage: number
  price: number
  reasoning?: string
  order_id: number
  timestamp: string
  success: boolean
  error?: string
}

export interface AccountSnapshot {
  total_balance: number
  available_balance: number
  total_unrealized_profit: number
  position_count: number
  margin_used_pct: number
}

export interface DecisionRecord {
  timestamp: string
  cycle_number: number
  input_prompt: string
  cot_trace: string
  decision_json: string
  account_state: AccountSnapshot
  positions: any[]
  candidate_coins: string[]
  decisions: DecisionAction[]
  technical_analysis?: Record<string, TechnicalAnalysis>
  price_action?: Record<string, PriceAction>
  execution_log: string[]
  success: boolean
  error_message?: string
}

export interface Statistics {
  total_trades: number
  win_rate: number
  profit_factor: number
  total_pnl: number
  max_drawdown: number
  avg_profit: number
  avg_loss: number
  trades_by_symbol: Record<string, { count: number; pnl: number }>
}

export interface TechnicalAnalysis {
  TrendState: string
  RSIState: string
  VolumeState: string
  SignalScore: number
  volume_zscore?: number
}

export interface PriceAction {
  UpperWickRatio: number
  LowerWickRatio: number
  BodyRatio: number
  DistToEMA20: number
  CandleType: string
}

// AI Trading相关类型
export interface TraderInfo {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange_id?: string
  is_running?: boolean
  custom_prompt?: string
  use_coin_pool?: boolean
  use_oi_top?: boolean
}

export interface AIModel {
  id: string
  name: string
  provider: string
  enabled: boolean
  apiKey?: string  // 安全修复：返回遮蔽值 '***' 而不是实际密钥
  customApiUrl?: string
  customModelName?: string
  envKey?: string
  hasApiKey?: boolean
}

export interface ModelProvider {
  id: string
  name: string
  base_url: string
  env_key: string
  default_model: string
  suggested_models?: string[]
  local?: boolean
  timeout_seconds?: number
  notes?: string
}

export interface Exchange {
  id: string
  name: string
  type: 'cex' | 'dex'
  enabled: boolean
  testnet?: boolean
  // 安全修复：返回遮蔽值 '***' 而不是实际密钥
  apiKey?: string
  secretKey?: string
  hyperliquidWalletAddr?: string
  asterUser?: string
  asterSigner?: string
  asterPrivateKey?: string
}

export interface CreateTraderRequest {
  name: string
  ai_model_id: string
  exchange_id: string
  initial_balance: number
  scan_interval_minutes?: number
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  system_prompt_template?: string
  is_cross_margin?: boolean
  use_coin_pool?: boolean
  use_oi_top?: boolean
}

export interface UpdateModelConfigRequest {
  models: {
    [key: string]: {
      enabled: boolean
      api_key: string
      custom_api_url?: string
      custom_model_name?: string
      env_key?: string
      name?: string
      provider?: string
    }
  }
}

export interface UpdateExchangeConfigRequest {
  exchanges: {
    [key: string]: {
      enabled: boolean
      api_key: string
      secret_key: string
      testnet?: boolean
      // Hyperliquid 特定字段
      hyperliquid_wallet_addr?: string
      // Aster 特定字段
      aster_user?: string
      aster_signer?: string
      aster_private_key?: string
    }
  }
}

// Competition related types
export interface CompetitionTraderData {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange: string
  total_equity: number
  total_pnl: number
  total_pnl_pct: number
  position_count: number
  margin_used_pct: number
  is_running: boolean
}

export interface CompetitionData {
  traders: CompetitionTraderData[]
  count: number
}

// Trader Configuration Data for View Modal
export interface TraderConfigData {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  btc_eth_leverage: number
  altcoin_leverage: number
  trading_symbols: string
  custom_prompt: string
  override_base_prompt: boolean
  system_prompt_template?: string
  is_cross_margin: boolean
  use_coin_pool: boolean
  use_oi_top: boolean
  initial_balance: number
  scan_interval_minutes: number
  is_running: boolean
}
