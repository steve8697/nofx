import React, { useState } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { Header } from './Header'
import { ArrowLeft, KeyRound, Eye, EyeOff } from 'lucide-react'

export function ResetPasswordPage() {
  const { language } = useLanguage()
  const { resetPassword } = useAuth()
  const [email, setEmail] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [otpCode, setOtpCode] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess(false)

    // 验证两次密码是否一致
    if (newPassword !== confirmPassword) {
      setError(t('passwordMismatch', language))
      return
    }

    setLoading(true)

    const result = await resetPassword(email, newPassword, otpCode)

    if (result.success) {
      setSuccess(true)
      // 3秒后跳转到登录页面
      setTimeout(() => {
        window.history.pushState({}, '', '/login')
        window.dispatchEvent(new PopStateEvent('popstate'))
      }, 3000)
    } else {
      setError(result.message || t('resetPasswordFailed', language))
    }

    setLoading(false)
  }

  return (
    <div className="min-h-screen relative overflow-hidden bg-transparent">
      {/* 🌌 Premium 消光鈦灰微光球 */}
      <div className="absolute top-1/4 left-1/4 w-[450px] h-[450px] rounded-full bg-white/[0.02] blur-[120px] pointer-events-none"></div>
      <div className="absolute bottom-1/4 right-1/4 w-[450px] h-[450px] rounded-full bg-white/[0.02] blur-[120px] pointer-events-none"></div>

      <Header simple />

      <div
        className="flex items-center justify-center relative z-10"
        style={{ minHeight: 'calc(100vh - 80px)' }}
      >
        <div className="w-full max-w-md px-4">
          {/* Back to Login */}
          <button
            onClick={() => {
              window.history.pushState({}, '', '/login')
              window.dispatchEvent(new PopStateEvent('popstate'))
            }}
            className="flex items-center gap-2 mb-6 text-sm hover:text-white transition-colors"
            style={{ color: '#848E9C' }}
          >
            <ArrowLeft className="w-4 h-4" />
            {t('backToLogin', language)}
          </button>

          {/* Logo */}
          <div className="text-center mb-8">
            <div
              className="w-20 h-20 mx-auto mb-4 flex items-center justify-center p-4 rounded-2xl bg-black/45 border border-white/5 shadow-2xl backdrop-blur-md"
            >
              <KeyRound className="w-10 h-10 text-white/80" />
            </div>
            <h1 className="text-2xl font-extrabold tracking-wider text-white">
              {t('resetPasswordTitle', language)}
            </h1>
            <p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>
              使用邮箱和 Google Authenticator 重置密码
            </p>
          </div>

          {/* Reset Password Form */}
          <div className="glass-card p-8 shadow-2xl relative overflow-hidden">
            {/* 卡片裝飾條 */}
            <div className="absolute top-0 left-0 right-0 h-[2px] bg-gradient-to-r from-transparent via-white/10 to-transparent"></div>
            {success ? (
              <div className="text-center py-8">
                <div className="text-5xl mb-4">✅</div>
                <p
                  className="text-lg font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('resetPasswordSuccess', language)}
                </p>
                <p className="text-sm" style={{ color: '#848E9C' }}>
                  3秒后将自动跳转到登录页面...
                </p>
              </div>
            ) : (
              <form onSubmit={handleResetPassword} className="space-y-5">
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
                    className="w-full px-4 py-2.5 rounded-lg bg-black/40 border border-white/5 focus:border-white/20 focus:ring-1 focus:ring-white/20 focus:outline-none transition-all duration-200 text-[#eaecef] text-sm"
                    placeholder={t('emailPlaceholder', language)}
                    required
                  />
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('newPassword', language)}
                  </label>
                  <div className="relative">
                    <input
                      type={showPassword ? 'text' : 'password'}
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      className="w-full px-4 py-2.5 rounded-lg bg-black/40 border border-white/5 focus:border-white/20 focus:ring-1 focus:ring-white/20 focus:outline-none transition-all duration-200 text-[#eaecef] text-sm pr-10"
                      placeholder={t('newPasswordPlaceholder', language)}
                      required
                      minLength={6}
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-300"
                    >
                      {showPassword ? (
                        <EyeOff className="w-5 h-5" />
                      ) : (
                        <Eye className="w-5 h-5" />
                      )}
                    </button>
                  </div>
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('confirmPassword', language)}
                  </label>
                  <div className="relative">
                    <input
                      type={showConfirmPassword ? 'text' : 'password'}
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      className="w-full px-4 py-2.5 rounded-lg bg-black/40 border border-white/5 focus:border-white/20 focus:ring-1 focus:ring-white/20 focus:outline-none transition-all duration-200 text-[#eaecef] text-sm pr-10"
                      placeholder={t('confirmPasswordPlaceholder', language)}
                      required
                      minLength={6}
                    />
                    <button
                      type="button"
                      onClick={() =>
                        setShowConfirmPassword(!showConfirmPassword)
                      }
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-300"
                    >
                      {showConfirmPassword ? (
                        <EyeOff className="w-5 h-5" />
                      ) : (
                        <Eye className="w-5 h-5" />
                      )}
                    </button>
                  </div>
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('otpCode', language)}
                  </label>
                  <div className="text-center mb-3">
                    <div className="text-3xl mb-1">📱</div>
                    <p className="text-xs" style={{ color: '#848E9C' }}>
                      打开 Google Authenticator 获取6位验证码
                    </p>
                  </div>
                  <input
                    type="text"
                    value={otpCode}
                    onChange={(e) =>
                      setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))
                    }
                    className="w-full px-4 py-3 rounded-lg bg-black/40 border border-white/5 focus:border-white/20 focus:ring-1 focus:ring-white/20 focus:outline-none transition-all duration-200 text-[#eaecef] text-center text-2xl font-mono"
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

                <button
                  type="submit"
                  disabled={loading || otpCode.length !== 6}
                  className="w-full py-3 rounded-lg text-sm font-semibold transition-all glow-button-gold disabled:opacity-50"
                >
                  {loading
                    ? t('loading', language)
                    : t('resetPasswordButton', language)}
                </button>
              </form>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
