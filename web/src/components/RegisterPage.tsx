import React, { useState, useEffect } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { getSystemConfig } from '../lib/config'
import HeaderBar from './landing/HeaderBar'

export function RegisterPage() {
  const { language } = useLanguage()
  const { register, completeRegistration } = useAuth()
  const [step, setStep] = useState<'register' | 'setup-otp' | 'verify-otp'>(
    'register'
  )
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [betaCode, setBetaCode] = useState('')
  const [betaMode, setBetaMode] = useState(false)
  const [otpCode, setOtpCode] = useState('')
  const [userID, setUserID] = useState('')
  const [otpSecret, setOtpSecret] = useState('')
  const [qrCodeURL, setQrCodeURL] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    // 获取系统配置，检查是否开启内测模式
    getSystemConfig()
      .then((config) => {
        setBetaMode(config.beta_mode || false)
      })
      .catch((err) => {
        console.error('Failed to fetch system config:', err)
      })
  }, [])

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (password !== confirmPassword) {
      setError(t('passwordMismatch', language))
      return
    }

    if (password.length < 6) {
      setError(t('passwordTooShort', language))
      return
    }

    if (betaMode && !betaCode.trim()) {
      setError('内测期间，注册需要提供内测码')
      return
    }

    setLoading(true)

    const result = await register(email, password, betaCode.trim() || undefined)

    if (result.success && result.userID) {
      setUserID(result.userID)
      setOtpSecret(result.otpSecret || '')
      setQrCodeURL(result.qrCodeURL || '')
      setStep('setup-otp')
    } else {
      setError(result.message || t('registrationFailed', language))
    }

    setLoading(false)
  }

  const handleSetupComplete = () => {
    setStep('verify-otp')
  }

  const handleOTPVerify = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await completeRegistration(userID, otpCode)

    if (!result.success) {
      setError(result.message || t('registrationFailed', language))
    }
    // 成功的话AuthContext会自动处理登录状态

    setLoading(false)
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
  }

  return (
    <div className="min-h-screen relative overflow-hidden">
      {/* 🌌 Premium 消光鈦灰背景裝飾球 */}
      <div className="absolute top-1/4 left-1/4 w-[450px] h-[450px] rounded-full bg-[radial-gradient(circle,rgba(255,255,255,0.03),transparent_70%)] pointer-events-none"></div>
      <div className="absolute bottom-1/4 right-1/4 w-[450px] h-[450px] rounded-full bg-[radial-gradient(circle,rgba(255,255,255,0.02),transparent_70%)] pointer-events-none"></div>

      <HeaderBar
        isLoggedIn={false}
        isHomePage={false}
        currentPage="register"
        language={language}
        onLanguageChange={() => {}}
        onPageChange={(page) => {
          console.log('RegisterPage onPageChange called with:', page)
          if (page === 'competition') {
            window.location.href = '/competition'
          }
        }}
      />

      <div
        className="flex items-center justify-center pt-20"
        style={{ minHeight: 'calc(100vh - 80px)' }}
      >
        <div className="w-full max-w-md">
          {/* Logo */}
          <div className="text-center mb-8">
            <div className="w-24 h-24 mx-auto mb-4 flex items-center justify-center p-4 rounded-3xl bg-[#131418] border border-white/10 shadow-2xl">
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
            <p className="text-sm mt-2" style={{ color: '#848E9C' }}>
              {step === 'register' && t('registerTitle', language)}
              {step === 'setup-otp' && t('setupTwoFactor', language)}
              {step === 'verify-otp' && t('verifyOTP', language)}
            </p>
          </div>

          {/* Registration Form */}
          <div className="glass-card p-8 shadow-2xl relative overflow-hidden">
            {/* 卡片裝飾條 */}
            <div className="absolute top-0 left-0 right-0 h-[1.5px] bg-gradient-to-r from-transparent via-white/15 to-transparent"></div>
            {step === 'register' && (
              <form onSubmit={handleRegister} className="space-y-4">
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
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('confirmPassword', language)}
                  </label>
                  <input
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    className="w-full px-4 py-2.5 rounded-lg bg-black/40 border border-white/5 focus:border-white/40 focus:ring-1 focus:ring-white/40 focus:outline-none transition-all duration-200 text-[#eaecef] text-sm"
                    placeholder={t('confirmPasswordPlaceholder', language)}
                    required
                  />
                </div>

                {betaMode && (
                  <div>
                    <label
                      className="block text-sm font-semibold mb-2"
                      style={{ color: '#EAECEF' }}
                    >
                      内测码 *
                    </label>
                    <input
                      type="text"
                      value={betaCode}
                      onChange={(e) =>
                        setBetaCode(
                          e.target.value
                            .replace(/[^a-z0-9]/gi, '')
                            .toLowerCase()
                        )
                      }
                      className="w-full px-4 py-2.5 rounded-lg bg-black/40 border border-white/5 focus:border-white/40 focus:ring-1 focus:ring-white/40 focus:outline-none transition-all duration-200 text-[#eaecef] text-sm font-mono"
                      placeholder="请输入6位内测码"
                      maxLength={6}
                      required={betaMode}
                    />
                    <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
                      内测码由6位字母数字组成，区分大小写
                    </p>
                  </div>
                )}

                {error && (
                  <div
                    className="text-sm px-3 py-2 rounded"
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
                  disabled={loading || (betaMode && !betaCode.trim())}
                  className="w-full py-3 rounded-lg text-sm font-semibold transition-all glow-button-gold disabled:opacity-50"
                >
                  {loading
                    ? t('loading', language)
                    : t('registerButton', language)}
                </button>
              </form>
            )}

            {step === 'setup-otp' && (
              <div className="space-y-4">
                <div className="text-center">
                  <div className="text-4xl mb-2">📱</div>
                  <h3
                    className="text-lg font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    {t('setupTwoFactor', language)}
                  </h3>
                  <p className="text-sm" style={{ color: '#848E9C' }}>
                    {t('setupTwoFactorDesc', language)}
                  </p>
                </div>

                <div className="space-y-3">
                  <div className="p-3.5 rounded-lg bg-black/40 border border-white/5">
                    <p
                      className="text-sm font-semibold mb-2"
                      style={{ color: 'var(--brand-light-gray)' }}
                    >
                      {t('authStep1Title', language)}
                    </p>
                    <p
                      className="text-xs"
                      style={{ color: 'var(--text-secondary)' }}
                    >
                      {t('authStep1Desc', language)}
                    </p>
                  </div>

                  <div className="p-3.5 rounded-lg bg-black/40 border border-white/5">
                    <p
                      className="text-sm font-semibold mb-2"
                      style={{ color: 'var(--brand-light-gray)' }}
                    >
                      {t('authStep2Title', language)}
                    </p>
                    <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                      {t('authStep2Desc', language)}
                    </p>

                    {qrCodeURL && (
                      <div className="mt-2">
                        <p
                          className="text-xs mb-2"
                          style={{ color: '#848E9C' }}
                        >
                          {t('qrCodeHint', language)}
                        </p>
                        <div className="bg-white p-2 rounded text-center">
                          <img
                            src={`https://api.qrserver.com/v1/create-qr-code/?size=150x150&data=${encodeURIComponent(qrCodeURL)}`}
                            alt="QR Code"
                            className="mx-auto"
                          />
                        </div>
                      </div>
                    )}

                    <div className="mt-2">
                      <p className="text-xs mb-1" style={{ color: '#848E9C' }}>
                        {t('otpSecret', language)}
                      </p>
                      <div className="flex items-center gap-2">
                        <code
                          className="flex-1 px-2.5 py-1 text-xs rounded font-mono bg-black/50 text-white border border-white/5"
                        >
                          {otpSecret}
                        </code>
                        <button
                          onClick={() => copyToClipboard(otpSecret)}
                          className="px-2.5 py-1 text-xs rounded-md font-semibold transition-all duration-200 bg-white/5 hover:bg-white/10 text-gray-200 border border-white/10 hover:border-white/20"
                        >
                          {t('copy', language)}
                        </button>
                      </div>
                    </div>
                  </div>

                  <div className="p-3.5 rounded-lg bg-black/40 border border-white/5">
                    <p
                      className="text-sm font-semibold mb-2"
                      style={{ color: 'var(--brand-light-gray)' }}
                    >
                      {t('authStep3Title', language)}
                    </p>
                    <p
                      className="text-xs"
                      style={{ color: 'var(--text-secondary)' }}
                    >
                      {t('authStep3Desc', language)}
                    </p>
                  </div>
                </div>

                <button
                  onClick={handleSetupComplete}
                  className="w-full py-3 rounded-lg text-sm font-semibold transition-all glow-button-gold"
                >
                  {t('setupCompleteContinue', language)}
                </button>
              </div>
            )}

            {step === 'verify-otp' && (
              <form onSubmit={handleOTPVerify} className="space-y-4">
                <div className="text-center mb-4">
                  <div className="text-4xl mb-2">🔐</div>
                  <p className="text-sm" style={{ color: '#848E9C' }}>
                    {t('enterOTPCode', language)}
                    <br />
                    {t('completeRegistrationSubtitle', language)}
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
                    className="text-sm px-3 py-2 rounded"
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
                    onClick={() => setStep('setup-otp')}
                    className="flex-1 py-3 rounded-lg text-sm font-semibold bg-white/5 hover:bg-white/10 border border-white/5 transition-colors text-gray-400"
                  >
                    {t('back', language)}
                  </button>
                  <button
                    type="submit"
                    disabled={loading || otpCode.length !== 6}
                    className="flex-1 py-3 rounded-lg text-sm font-semibold transition-all glow-button-gold disabled:opacity-50"
                  >
                    {loading
                      ? t('loading', language)
                      : t('completeRegistration', language)}
                  </button>
                </div>
              </form>
            )}
          </div>

          {/* Login Link */}
          {step === 'register' && (
            <div className="text-center mt-6">
              <p className="text-sm text-gray-500">
                已有账户？{' '}
                <button
                  onClick={() => {
                    window.history.pushState({}, '', '/login')
                    window.dispatchEvent(new PopStateEvent('popstate'))
                  }}
                  className="font-semibold text-gray-300 hover:text-white hover:underline transition-colors"
                >
                  立即登录
                </button>
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
