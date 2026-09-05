import React, { createContext, useContext, useState, useEffect } from 'react'
import { getSystemConfig } from '../lib/config'

interface User {
  id: string
  email: string
}

interface AuthContextType {
  user: User | null
  token: string | null
  login: (
    email: string,
    password: string
  ) => Promise<{
    success: boolean
    message?: string
    userID?: string
    requiresOTP?: boolean
  }>
  loginAdmin: (password: string) => Promise<{
    success: boolean
    message?: string
  }>
  register: (
    email: string,
    password: string,
    betaCode?: string
  ) => Promise<{
    success: boolean
    message?: string
    userID?: string
    otpSecret?: string
    qrCodeURL?: string
  }>
  verifyOTP: (
    userID: string,
    otpCode: string
  ) => Promise<{ success: boolean; message?: string }>
  completeRegistration: (
    userID: string,
    otpCode: string
  ) => Promise<{ success: boolean; message?: string }>
  resetPassword: (
    email: string,
    newPassword: string,
    otpCode: string
  ) => Promise<{ success: boolean; message?: string }>
  logout: () => void
  isLoading: boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    // 自動登錄函數（提前定义，避免作用域问题）
    const performAutoLogin = () => {
      const savedPassword = sessionStorage.getItem('admin_password')
      if (!savedPassword) {
        console.log('⚠️ 管理员模式：沒有儲存的密碼，跳過自動登錄，請手動登入')
        setIsLoading(false)
        return
      }
      console.log('🔐 管理员模式：使用儲存的密碼自動登錄中...')
      fetch('/api/admin-login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: savedPassword }),
      })
        .then((response) => response.json())
        .then((data) => {
          if (data.token) {
            const userInfo = {
              id: data.user_id || 'admin',
              email: data.email || 'admin@localhost',
            }
            setToken(data.token)
            setUser(userInfo)
            localStorage.setItem('auth_token', data.token)
            localStorage.setItem('auth_user', JSON.stringify(userInfo))
            console.log('✅ 自动登录成功')
          } else {
            console.error('❌ 自动登录失败:', data.error)
          }
          setIsLoading(false)
        })
        .catch((err) => {
          console.error('❌ 自动登录异常:', err)
          setIsLoading(false)
        })
    }

    // 先检查是否为管理员模式（使用带缓存的系统配置获取）
    getSystemConfig()
      .then((config) => {
        // 检查本地存储的 token
        const savedToken = localStorage.getItem('auth_token')
        const savedUser = localStorage.getItem('auth_user')

        // ✅ 修正：簡化管理員模式的自動登錄邏輯
        // 在管理員模式下，無論是否有 savedToken，都先嘗試自動登錄
        if (config.admin_mode) {
          // 如果有 savedToken，先驗證其有效性
          if (savedToken && savedUser) {
            const savedUserObj = JSON.parse(savedUser)
            // 檢查保存的用戶是否是管理員
            if (savedUserObj.email !== 'admin@localhost') {
              console.log('🔐 管理员模式：检测到旧的用户信息，清除中...')
              localStorage.removeItem('auth_token')
              localStorage.removeItem('auth_user')
              // 繼續自動登錄流程
              performAutoLogin()
            } else {
              // 驗證 token 是否有效
              console.log('🔐 管理员模式：验证现有 token...')
              fetch('/api/my-traders', {
                headers: { 'Authorization': `Bearer ${savedToken}` }
              })
                .then(response => {
                  if (response.ok) {
                    console.log('✅ 现有 token 有效')
                    setToken(savedToken)
                    setUser(savedUserObj)
                    setIsLoading(false)
                  } else if (response.status === 401 || response.status === 403) {
                    console.log('⚠️ 现有 token 已过期，清除並重新登录...')
                    localStorage.removeItem('auth_token')
                    localStorage.removeItem('auth_user')
                    // 繼續執行自動登錄
                    performAutoLogin()
                  } else {
                    console.log('⚠️ token 验证失败，清除並重新登录...')
                    localStorage.removeItem('auth_token')
                    localStorage.removeItem('auth_user')
                    // 繼續執行自動登錄
                    performAutoLogin()
                  }
                })
                .catch(() => {
                  console.log('⚠️ token 验证异常，清除並重新登录...')
                  localStorage.removeItem('auth_token')
                  localStorage.removeItem('auth_user')
                  // 繼續執行自動登錄
                  performAutoLogin()
                })
            }
          } else {
            // 沒有 savedToken，直接執行自動登錄
            performAutoLogin()
          }
        } else {
          // 非管理員模式
          if (savedToken && savedUser) {
            setToken(savedToken)
            setUser(JSON.parse(savedUser))
          }
          setIsLoading(false)
        }
      })
      .catch((err) => {
        console.error('Failed to fetch system config:', err)
        // 发生错误时，先尝试从本地存储恢复
        const savedToken = localStorage.getItem('auth_token')
        const savedUser = localStorage.getItem('auth_user')

        if (savedToken && savedUser) {
          setToken(savedToken)
          setUser(JSON.parse(savedUser))
        }

        // 然后尝试强制自动登录
        console.log('⚠️ 系统配置获取失败，尝试强制自动登录...')
        performAutoLogin()
      })

    // 监听 localStorage 变化（用于跨标签页同步）
    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === 'auth_token' && e.newValue) {
        const newUser = localStorage.getItem('auth_user')
        if (newUser) {
          setToken(e.newValue)
          setUser(JSON.parse(newUser))
        }
      } else if (e.key === 'auth_token' && !e.newValue) {
        setToken(null)
        setUser(null)
      }
    }
    window.addEventListener('storage', handleStorageChange)
    return () => window.removeEventListener('storage', handleStorageChange)
  }, [])

  const login = async (email: string, password: string) => {
    try {
      const response = await fetch('/api/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }),
      })

      const data = await response.json()

      if (response.ok) {
        if (data.requires_otp) {
          return {
            success: true,
            userID: data.user_id,
            requiresOTP: true,
            message: data.message,
          }
        }
      } else {
        return { success: false, message: data.error }
      }
    } catch (error) {
      return { success: false, message: '登录失败，请重试' }
    }

    return { success: false, message: '未知错误' }
  }

  const loginAdmin = async (password: string) => {
    try {
      const response = await fetch('/api/admin-login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      })
      const data = await response.json()
      if (response.ok) {
        const userInfo = {
          id: data.user_id || 'admin',
          email: data.email || 'admin@localhost',
        }
        setToken(data.token)
        setUser(userInfo)
        localStorage.setItem('auth_token', data.token)
        localStorage.setItem('auth_user', JSON.stringify(userInfo))
        // 🔒 儲存密碼用於自動重新登入（sessionStorage 關閉瀏覽器後自動清除）
        sessionStorage.setItem('admin_password', password)
        // 跳转到仪表盘
        window.history.pushState({}, '', '/dashboard')
        window.dispatchEvent(new PopStateEvent('popstate'))
        return { success: true }
      } else {
        return { success: false, message: data.error || '登录失败' }
      }
    } catch (e) {
      return { success: false, message: '登录失败，请重试' }
    }
  }

  const register = async (
    email: string,
    password: string,
    betaCode?: string
  ) => {
    try {
      const requestBody: {
        email: string
        password: string
        beta_code?: string
      } = { email, password }
      if (betaCode) {
        requestBody.beta_code = betaCode
      }

      const response = await fetch('/api/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody),
      })

      const data = await response.json()

      if (response.ok) {
        return {
          success: true,
          userID: data.user_id,
          otpSecret: data.otp_secret,
          qrCodeURL: data.qr_code_url,
          message: data.message,
        }
      } else {
        return { success: false, message: data.error }
      }
    } catch (error) {
      return { success: false, message: '注册失败，请重试' }
    }
  }

  const verifyOTP = async (userID: string, otpCode: string) => {
    try {
      const response = await fetch('/api/verify-otp', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ user_id: userID, otp_code: otpCode }),
      })

      const data = await response.json()

      if (response.ok) {
        // 登录成功，保存token和用户信息
        const userInfo = { id: data.user_id, email: data.email }
        setToken(data.token)
        setUser(userInfo)
        localStorage.setItem('auth_token', data.token)
        localStorage.setItem('auth_user', JSON.stringify(userInfo))

        // 跳转到配置页面
        window.history.pushState({}, '', '/traders')
        window.dispatchEvent(new PopStateEvent('popstate'))

        return { success: true, message: data.message }
      } else {
        return { success: false, message: data.error }
      }
    } catch (error) {
      return { success: false, message: 'OTP验证失败，请重试' }
    }
  }

  const completeRegistration = async (userID: string, otpCode: string) => {
    try {
      const response = await fetch('/api/complete-registration', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ user_id: userID, otp_code: otpCode }),
      })

      const data = await response.json()

      if (response.ok) {
        // 注册完成，自动登录
        const userInfo = { id: data.user_id, email: data.email }
        setToken(data.token)
        setUser(userInfo)
        localStorage.setItem('auth_token', data.token)
        localStorage.setItem('auth_user', JSON.stringify(userInfo))

        // 跳转到配置页面
        window.history.pushState({}, '', '/traders')
        window.dispatchEvent(new PopStateEvent('popstate'))

        return { success: true, message: data.message }
      } else {
        return { success: false, message: data.error }
      }
    } catch (error) {
      return { success: false, message: '注册完成失败，请重试' }
    }
  }

  const resetPassword = async (
    email: string,
    newPassword: string,
    otpCode: string
  ) => {
    try {
      const response = await fetch('/api/reset-password', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          email,
          new_password: newPassword,
          otp_code: otpCode,
        }),
      })

      const data = await response.json()

      if (response.ok) {
        return { success: true, message: data.message }
      } else {
        return { success: false, message: data.error }
      }
    } catch (error) {
      return { success: false, message: '密码重置失败，请重试' }
    }
  }

  const logout = () => {
    const savedToken = localStorage.getItem('auth_token')
    if (savedToken) {
      fetch('/api/logout', {
        method: 'POST',
        headers: { Authorization: `Bearer ${savedToken}` },
      }).catch(() => {
        /* ignore network errors on logout */
      })
    }
    setUser(null)
    setToken(null)
    localStorage.removeItem('auth_token')
    localStorage.removeItem('auth_user')
    sessionStorage.removeItem('admin_password')
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        login,
        loginAdmin,
        register,
        verifyOTP,
        completeRegistration,
        resetPassword,
        logout,
        isLoading,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
