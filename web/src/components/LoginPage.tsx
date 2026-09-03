import React, { useEffect, useState } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import HeaderBar from './landing/HeaderBar'
import { getSystemConfig } from '../lib/config'

export function LoginPage() {
  const { language } = useLanguage()
  const { login, loginAdmin, verifyOTP } = useAuth()
  const [step, setStep] = useState<'login' | 'otp'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [otpCode, setOtpCode] = useState('')
  const [userID, setUserID] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [adminPassword, setAdminPassword] = useState('')
  const [adminMode, setAdminMode] = useState<boolean | null>(null)
  const [rememberAdmin, setRememberAdmin] = useState(true)

  useEffect(() => {
    getSystemConfig()
      .then((cfg) => {
        const isAdm = !!cfg.admin_mode
        setAdminMode(isAdm)
        
        if (isAdm) {
          const savedPw = localStorage.getItem('saved_admin_pw')
          if (savedPw) {
            console.log('🔑 自部署助手：檢測到已儲存的管理員密碼，正在嘗試自動登入...')
            setAdminPassword(savedPw)
            setLoading(true)
            loginAdmin(savedPw).then((res) => {
              if (!res.success) {
                console.warn('⚠️ 自動登入失敗，可能密碼已更改，請手動登入')
                localStorage.removeItem('saved_admin_pw')
                setAdminPassword('')
              } else {
                console.log('✅ 自動登入成功！')
              }
              setLoading(false)
            }).catch(() => {
              setLoading(false)
            })
          }
        }
      })
      .catch(() => {
        setAdminMode(false)
      })
  }, [])

  const handleAdminLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    const result = await loginAdmin(adminPassword)
    if (result.success) {
      if (rememberAdmin) {
        localStorage.setItem('saved_admin_pw', adminPassword)
      } else {
        localStorage.removeItem('saved_admin_pw')
      }
    } else {
      setError(result.message || t('loginFailed', language))
    }
    setLoading(false)
  }

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await login(email, password)

    if (result.success) {
      if (result.requiresOTP && result.userID) {
        setUserID(result.userID)
        setStep('otp')
      }
    } else {
      setError(result.message || t('loginFailed', language))
    }

    setLoading(false)
  }

  const handleOTPVerify = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await verifyOTP(userID, otpCode)

    if (!result.success) {
      setError(result.message || t('verificationFailed', language))
    }
    // 成功的话AuthContext会自动处理登录状态

    setLoading(false)
  }

  return (
    <div className="min-h-screen relative overflow-hidden">
      {/* 🌌 Premium 消光鈦灰背景裝飾球 */}
      <div className="absolute top-1/4 left-1/4 w-[450px] h-[450px] rounded-full bg-gradient-to-tr from-white/4 to-transparent blur-[120px] pointer-events-none"></div>
      <div className="absolute bottom-1/4 right-1/4 w-[450px] h-[450px] rounded-full bg-gradient-to-br from-white/2 to-transparent blur-[120px] pointer-events-none"></div>

      <HeaderBar
        onLoginClick={() => {}}
        isLoggedIn={false}
        isHomePage={false}
        currentPage="login"
        language={language}
        onLanguageChange={() => {}}
        onPageChange={(page) => {
          console.log('LoginPage onPageChange called with:', page)
          if (page === 'competition') {
            window.location.href = '/competition'
          }
        }}
      />

      <div
        className="flex items-center justify-center pt-24 relative z-10"
        style={{ minHeight: 'calc(100vh - 80px)' }}
      >
        <div className="w-full max-w-md px-4">
          {/* Logo */}
          <div className="text-center mb-8">
            <div className="w-24 h-24 mx-auto mb-4 flex items-center justify-center p-4 rounded-3xl bg-black/45 border border-white/10 shadow-2xl backdrop-blur-md">
              <svg
                className="w-full h-full text-white/70"
                viewBox="0 0 100 100"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                {/* Dashed outer orbit ring */}
                <circle 
                  cx="50" 
                  cy="50" 
                  r="42" 
                  stroke="currentColor" 
                  strokeWidth="1.5" 
                  strokeDasharray="10 15" 
                  className="animate-spin" 
                  style={{ animationDuration: '15s', transformOrigin: 'center' }}
                />
                {/* Inner glowing ring */}
                <circle 
                  cx="50" 
                  cy="50" 
                  r="28" 
                  stroke="rgba(255, 255, 255, 0.25)" 
                  strokeWidth="2.5" 
                />
                <circle cx="50" cy="50" r="6" fill="#ffffff" className="animate-pulse" />
              </svg>
            </div>
            <h1
              className="text-2xl font-extrabold tracking-wider text-white"
            >
              AETHERIS QUANTUM
            </h1>
            <p
              className="text-sm mt-2"
              style={{ color: 'var(--text-secondary)' }}
            >
              {step === 'login' ? '請輸入您的管理密碼' : '請輸入二步驟驗證碼'}
            </p>
          </div>

          {/* Login Form */}
          <div className="glass-card p-8 shadow-2xl relative overflow-hidden">
            {/* 卡片裝飾條 */}
            <div className="absolute top-0 left-0 right-0 h-[1.5px] bg-gradient-to-r from-transparent via-white/15 to-transparent"></div>

            {adminMode ? (
              <form onSubmit={handleAdminLogin} className="space-y-5">
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    管理員密碼
                  </label>
                  <input
                    type="password"
                    value={adminPassword}
                    onChange={(e) => setAdminPassword(e.target.value)}
                    className="w-full px-4 py-2.5 rounded-lg bg-black/40 border border-white/5 focus:border-white/40 focus:ring-1 focus:ring-white/40 focus:outline-none transition-all duration-200 text-[#eaecef] text-sm"
                    placeholder="請輸入管理密碼"
                    required
                  />
                </div>

                <div className="flex items-center gap-2.5 py-1">
                  <input
                    type="checkbox"
                    id="rememberAdmin"
                    checked={rememberAdmin}
                    onChange={(e) => setRememberAdmin(e.target.checked)}
                    className="w-4 h-4 rounded cursor-pointer accent-white bg-[#000]"
                    style={{ border: '1px solid var(--panel-border)' }}
                  />
                  <label htmlFor="rememberAdmin" className="text-xs cursor-pointer select-none" style={{ color: 'var(--text-secondary)' }}>
                    在本機記住密碼，下次打開自動登入
                  </label>
                </div>

                {error && (
                  <div
                    className="text-sm px-3 py-2 rounded-lg border border-red-500/20"
                    style={{
                      background: 'var(--binance-red-bg)',
                      color: 'var(--binance-red)',
                    }}
                  >
                    {error}
                  </div>
                )}

                <button
                  type="submit"
                  disabled={loading}
                  className="w-full py-3 rounded-lg text-sm font-semibold transition-all glow-button-gold"
                >
                  {loading ? t('loading', language) : '登入'}
                </button>
              </form>
            ) : step === 'login' ? (
              <form onSubmit={handleLogin} className="space-y-5">
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('email', language)}
                  </label>
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="w-full px-4 py-2.5 rounded-lg bg-black/40 border border-white/5 focus:border-white/40 focus:ring-1 focus:ring-white/40 focus:outline-none transition-all duration-200 text-[#eaecef] text-sm"
                    placeholder={t('emailPlaceholder', language)}
                    required
                  />
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('password', language)}
                  </label>
                  <input
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full px-4 py-2.5 rounded-lg bg-black/40 border border-white/5 focus:border-white/40 focus:ring-1 focus:ring-white/40 focus:outline-none transition-all duration-200 text-[#eaecef] text-sm"
                    placeholder={t('passwordPlaceholder', language)}
                    required
                  />
                  <div className="text-right mt-2">
                    <button
                      type="button"
                      onClick={() => {
                        window.history.pushState({}, '', '/reset-password')
                        window.dispatchEvent(new PopStateEvent('popstate'))
                      }}
                      className="text-xs hover:text-white text-gray-400 hover:underline transition-colors font-semibold"
                    >
                      {t('forgotPassword', language)}
                    </button>
                  </div>
                </div>

                {error && (
                  <div
                    className="text-sm px-3 py-2 rounded-lg border border-red-500/20"
                    style={{
                      background: 'var(--binance-red-bg)',
                      color: 'var(--binance-red)',
                    }}
                  >
                    {error}
                  </div>
                )}

                <button
                  type="submit"
                  disabled={loading}
                  className="w-full py-3 rounded-lg text-sm font-semibold transition-all glow-button-gold"
                >
                  {loading
                    ? t('loading', language)
                    : t('loginButton', language)}
                </button>
              </form>
            ) : (
              <form onSubmit={handleOTPVerify} className="space-y-5">
                <div className="text-center mb-4">
                  <div className="text-4xl mb-2">📱</div>
                  <p className="text-sm" style={{ color: '#848E9C' }}>
                    {t('scanQRCodeInstructions', language)}
                    <br />
                    {t('enterOTPCode', language)}
                  </p>
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('otpCode', language)}
                  </label>
                  <input
                    type="text"
                    value={otpCode}
                    onChange={(e) =>
                      setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))
                    }
                    className="w-full px-4 py-3 rounded-lg bg-black/40 border border-white/5 focus:border-white/40 focus:ring-1 focus:ring-white/40 focus:outline-none transition-all duration-200 text-[#eaecef] text-center text-2xl font-mono"
                    placeholder={t('otpPlaceholder', language)}
                    maxLength={6}
                    required
                  />
                </div>

                {error && (
                  <div
                    className="text-sm px-3 py-2 rounded-lg border border-red-500/20"
                    style={{
                      background: 'var(--binance-red-bg)',
                      color: 'var(--binance-red)',
                    }}
                  >
                    {error}
                  </div>
                )}

                <div className="flex gap-3">
                  <button
                    type="button"
                    onClick={() => setStep('login')}
                    className="flex-1 py-3 rounded-lg text-sm font-semibold bg-white/5 hover:bg-white/10 border border-white/5 transition-colors text-gray-400"
                  >
                    {t('back', language)}
                  </button>
                  <button
                    type="submit"
                    disabled={loading || otpCode.length !== 6}
                    className="flex-1 py-3 rounded-lg text-sm font-semibold transition-all glow-button-gold"
                  >
                    {loading
                      ? t('loading', language)
                      : t('verifyOTP', language)}
                  </button>
                </div>
              </form>
            )}
          </div>

          {/* Register Link */}
          {!adminMode && (
            <div className="text-center mt-6">
              <p className="text-sm text-gray-500">
                还没有账户？{' '}
                <button
                  onClick={() => {
                    window.history.pushState({}, '', '/register')
                    window.dispatchEvent(new PopStateEvent('popstate'))
                  }}
                  className="font-semibold text-gray-300 hover:text-white hover:underline transition-colors"
                >
                  立即注册
                </button>
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
