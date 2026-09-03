import { useState, useEffect, type FormEvent } from 'react'
import { 
	Lock, ShieldAlert, Loader2, Cpu, Mail, KeyRound, 
	QrCode, ArrowLeft, Copy, Check, UserPlus, Eye, EyeOff 
} from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import { api } from '../../lib/api'
import { toast } from 'sonner'

interface UnlockScreenProps {
	onUnlock: () => void
}

type AuthStep = 'loading' | 'admin-unlock' | 'login' | 'register' | 'setup-otp' | 'verify-otp' | 'reset-password'

export function UnlockScreen({ onUnlock }: UnlockScreenProps) {
	const [step, setStep] = useState<AuthStep>('loading')
	const [adminMode, setAdminMode] = useState(true)
	const [betaMode, setBetaMode] = useState(false)
	
	// Form fields
	const [password, setPassword] = useState('')
	const [email, setEmail] = useState('')
	const [confirmPassword, setConfirmPassword] = useState('')
	const [betaCode, setBetaCode] = useState('')
	const [otpCode, setOtpCode] = useState('')
	
	// OTP setup state
	const [userID, setUserID] = useState('')
	const [otpSecret, setOtpSecret] = useState('')
	const [qrCodeURL, setQrCodeURL] = useState('')
	const [otpActionType, setOtpActionType] = useState<'login' | 'register'>('login')
	
	// UI states
	const [loading, setLoading] = useState(false)
	const [error, setError] = useState<string | null>(null)
	const [shake, setShake] = useState(false)
	const [copied, setCopied] = useState(false)
	const [showPassword, setShowPassword] = useState(false)
	const [showConfirmPassword, setShowConfirmPassword] = useState(false)

	// Fetch system config on mount
	useEffect(() => {
		api.getSystemConfig()
			.then((cfg) => {
				setAdminMode(cfg.admin_mode)
				setBetaMode(cfg.beta_mode)
				
				if (cfg.admin_mode) {
					setStep('admin-unlock')
					// Auto login with saved credentials if in admin mode
					const savedPw = sessionStorage.getItem('admin_password')
					if (savedPw) {
						setLoading(true)
						api.loginAdmin(savedPw)
							.then(() => {
								toast.success('安全通道自動認證成功')
								onUnlock()
							})
							.catch(() => {
								sessionStorage.removeItem('admin_password')
								setLoading(false)
							})
					}
				} else {
					setStep('login')
				}
			})
			.catch((err) => {
				console.error('Failed to get system config:', err)
				setStep('admin-unlock') // Fallback to admin mode
			})
	}, [onUnlock])

	const triggerShake = () => {
		setShake(true)
		setTimeout(() => setShake(false), 500)
	}

	// 1. Admin Unlock Submit
	const handleAdminUnlock = async (e: FormEvent) => {
		e.preventDefault()
		if (!password) return

		setLoading(true)
		setError(null)

		try {
			await api.loginAdmin(password)
			toast.success('系統解鎖成功，已建立加密安全連線')
			onUnlock()
		} catch (err: any) {
			setError('認證失敗：密碼錯誤或安全通道拒絕連線')
			triggerShake()
			toast.error('管理密碼錯誤')
		} finally {
			setLoading(false)
		}
	}

	// 2. Multi-user Login Submit
	const handleLogin = async (e: FormEvent) => {
		e.preventDefault()
		if (!email || !password) return

		setLoading(true)
		setError(null)

		try {
			const result = await api.login(email, password)
			if (result.success) {
				if (result.requiresOTP && result.userID) {
					setUserID(result.userID)
					setOtpActionType('login')
					setStep('verify-otp')
					toast.success('密碼驗證成功，請輸入 2FA 驗證碼')
				} else if (result.requiresOTPSetup && result.userID) {
					// User registered but OTP setup was not completed
					setUserID(result.userID)
					// We need to register again to regenerate keys or prompt setup
					setError('您的帳戶尚未完成 OTP 設定，請聯繫管理員或重新註冊。')
					triggerShake()
				}
			} else {
				setError(result.message || '登入失敗')
				triggerShake()
			}
		} catch (err: any) {
			setError('登入失敗，請檢查網路連線')
			triggerShake()
		} finally {
			setLoading(false)
		}
	}

	// 3. Multi-user Register Submit
	const handleRegister = async (e: FormEvent) => {
		e.preventDefault()
		setError(null)

		if (password !== confirmPassword) {
			setError('兩次輸入的密碼不一致')
			triggerShake()
			return
		}

		if (password.length < 6) {
			setError('密碼長度至少需要 6 個字元')
			triggerShake()
			return
		}

		if (betaMode && !betaCode.trim()) {
			setError('內測期間，註冊需要提供有效的內測碼')
			triggerShake()
			return
		}

		setLoading(true)

		try {
			const result = await api.register(email, password, betaCode.trim() || undefined)
			if (result.success && result.userID) {
				setUserID(result.userID)
				setOtpSecret(result.otpSecret || '')
				setQrCodeURL(result.qrCodeURL || '')
				setStep('setup-otp')
				toast.success('帳戶創建成功，請綁定 Google Authenticator')
			} else {
				setError(result.message || '註冊失敗')
				triggerShake()
			}
		} catch (err) {
			setError('註冊請求失敗，請重試')
			triggerShake()
		} finally {
			setLoading(false)
		}
	}

	// 4. OTP Verification Submit (Both login and registration completion)
	const handleOTPVerify = async (e: FormEvent) => {
		e.preventDefault()
		if (otpCode.length !== 6) return

		setLoading(true)
		setError(null)

		try {
			let result
			if (otpActionType === 'login') {
				result = await api.verifyOTP(userID, otpCode)
			} else {
				result = await api.completeRegistration(userID, otpCode)
			}

			if (result.success) {
				toast.success(otpActionType === 'login' ? '登入成功，交易艙已就緒' : '雙因子綁定成功，歡迎登入')
				onUnlock()
			} else {
				setError(result.message || '驗證碼錯誤，請重新確認')
				triggerShake()
			}
		} catch (err) {
			setError('驗證失敗，請重試')
			triggerShake()
		} finally {
			setLoading(false)
		}
	}

	// 5. Password Reset Submit
	const handleResetPassword = async (e: FormEvent) => {
		e.preventDefault()
		setError(null)

		if (password !== confirmPassword) {
			setError('兩次輸入的密碼不一致')
			triggerShake()
			return
		}

		setLoading(true)

		try {
			const result = await api.resetPassword(email, password, otpCode)
			if (result.success) {
				toast.success('密碼重設成功，請使用新密碼登入')
				// Clear states
				setPassword('')
				setConfirmPassword('')
				setOtpCode('')
				setStep('login')
			} else {
				setError(result.message || '重設密碼失敗，請確認郵件與驗證碼')
				triggerShake()
			}
		} catch (err) {
			setError('重設請求失敗，請重試')
			triggerShake()
		} finally {
			setLoading(false)
		}
	}

	const handleCopySecret = () => {
		navigator.clipboard.writeText(otpSecret)
		setCopied(true)
		toast.success('密鑰已複製到剪貼簿')
		setTimeout(() => setCopied(false), 2000)
	}

	const handleBackToLogin = () => {
		setError(null)
		setPassword('')
		setConfirmPassword('')
		setOtpCode('')
		setStep('login')
	}

	// Motion animation variants
	const cardVariants = {
		hidden: { opacity: 0, y: 15, scale: 0.98 },
		visible: { 
			opacity: 1, 
			y: 0, 
			scale: 1,
			transition: { duration: 0.4, ease: [0.16, 1, 0.3, 1] as any }
		},
		exit: { 
			opacity: 0, 
			y: -15, 
			scale: 0.98,
			transition: { duration: 0.25, ease: 'easeIn' as any }
		}
	}

	return (
		<div className="fixed inset-0 z-50 flex items-center justify-center bg-[#0a0b0f]/90 backdrop-blur-md overflow-y-auto py-8">
			{/* Sci-fi backdrop grid effect */}
			<div className="absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.001)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.001)_1px,transparent_1px)] bg-[size:32px_32px] pointer-events-none opacity-20"></div>
			
			{/* Subtle glow orb */}
			<div className="absolute w-[500px] h-[500px] rounded-full bg-blue-500/5 blur-[120px] pointer-events-none"></div>

			<AnimatePresence mode="wait">
				{step === 'loading' ? (
					<motion.div 
						key="loading"
						initial={{ opacity: 0 }}
						animate={{ opacity: 1 }}
						exit={{ opacity: 0 }}
						className="flex flex-col items-center gap-4 text-blue-400/80 font-mono text-xs tracking-widest"
					>
						<Loader2 className="w-8 h-8 animate-spin text-blue-500" />
						CONNECTING SECURE GATEWAY...
					</motion.div>
				) : (
					<motion.div
						key={step}
						variants={cardVariants}
						initial="hidden"
						animate="visible"
						exit="exit"
						className={`relative w-full ${step === 'setup-otp' ? 'max-w-lg' : 'max-w-md'} m-auto p-8 border border-[var(--border)] bg-[#111318]/80 backdrop-blur-xl rounded-sm shadow-2xl transition-all duration-300 ${
							shake ? 'animate-shake' : ''
						}`}
						style={{
							boxShadow: '0 0 40px rgba(0, 0, 0, 0.8), inset 0 0 20px rgba(59, 130, 246, 0.03)'
						}}
					>
						{/* Top scanline pulse bar */}
						<div className="absolute top-0 left-0 w-full h-[2px] bg-gradient-to-r from-transparent via-blue-500 to-transparent animate-pulse"></div>

						<div className="flex flex-col items-center">
							{/* Terminal style badge */}
							<div className="flex items-center gap-1.5 px-2.5 py-1 mb-6 rounded-sm bg-blue-500/5 border border-blue-500/10 text-blue-400 font-mono text-[10px] tracking-wider uppercase">
								<span className="w-1.5 h-1.5 rounded-full bg-blue-500 animate-ping"></span>
								{adminMode ? 'STEALTH SECURE GATEWAY • ADMIN' : 'STEALTH SECURE GATEWAY • MULTI-USER'}
							</div>

							{/* --- STEP 1: ADMIN MODE UNLOCK --- */}
							{step === 'admin-unlock' && (
								<div className="w-full flex flex-col items-center text-center">
									<div className="relative w-16 h-16 mb-6 flex items-center justify-center rounded-full bg-blue-500/5 border border-blue-500/10">
										<Lock className="w-6 h-6 text-blue-400 animate-pulse" />
										<div className="absolute inset-0 rounded-full border border-blue-500/20 animate-ping" style={{ animationDuration: '3s' }}></div>
									</div>

									<h2 className="text-sm font-mono text-[var(--text-muted)] tracking-[0.2em] uppercase mb-1">
										SYSTEM LOCKED
									</h2>
									<h1 className="text-xl font-medium tracking-tight mb-2 text-white">
										量化交易控制台安全解鎖
									</h1>
									<p className="text-xs text-[var(--text-muted)] font-mono leading-relaxed mb-8 max-w-xs">
										AETHERIS AI TRADING CORE • COGNITIVE RUNTIME
										<br />
										請輸入您的管理密碼以存取內部量化模組
									</p>

									<form onSubmit={handleAdminUnlock} className="w-full space-y-4 font-mono">
										<div className="relative">
											<input
												type="password"
												placeholder="ENTER PASSWORD"
												value={password}
												onChange={(e) => setPassword(e.target.value)}
												disabled={loading}
												autoFocus
												className="w-full px-4 py-3 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-center text-sm tracking-[0.3em] font-bold text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 placeholder:tracking-normal placeholder:font-normal placeholder:text-gray-600 transition-colors"
											/>
											<Cpu className="absolute left-3.5 top-3.5 w-4 h-4 text-gray-600 pointer-events-none" />
										</div>

										{error && (
											<div className="flex items-center gap-2 p-3 text-xs border border-red-500/15 bg-red-500/5 text-red-400 rounded-sm text-left">
												<ShieldAlert className="w-4 h-4 shrink-0" />
												<span>{error}</span>
											</div>
										)}

										<button
											type="submit"
											disabled={loading || !password}
											className="w-full py-3 bg-blue-500/10 hover:bg-blue-500/15 border border-blue-500/25 active:bg-blue-500/20 rounded-sm font-bold text-sm tracking-[0.25em] text-blue-400 disabled:opacity-30 disabled:hover:bg-blue-500/10 transition-all cursor-pointer flex items-center justify-center gap-2"
										>
											{loading ? (
												<>
													<Loader2 className="w-4 h-4 animate-spin" />
													DECRYPTING KEYS...
												</>
											) : (
												'ACCESS CONTROL'
											)}
										</button>
									</form>
								</div>
							)}

							{/* --- STEP 2: USER LOGIN --- */}
							{step === 'login' && (
								<div className="w-full flex flex-col items-center">
									<div className="text-center mb-6">
										<h1 className="text-xl font-medium tracking-tight mb-2 text-white">
											量化交易控制台登入
										</h1>
										<p className="text-xs text-[var(--text-muted)] font-mono">
											請登入您的個人加密交易帳戶
										</p>
									</div>

									<form onSubmit={handleLogin} className="w-full space-y-4 font-mono text-left">
										<div className="space-y-1.5">
											<label className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider">EMAIL ADDRESS</label>
											<div className="relative">
												<input
													type="email"
													placeholder="example@domain.com"
													value={email}
													onChange={(e) => setEmail(e.target.value)}
													disabled={loading}
													required
													className="w-full pl-10 pr-4 py-2.5 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-sm text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 transition-colors"
												/>
												<Mail className="absolute left-3 top-3 w-4 h-4 text-gray-500 pointer-events-none" />
											</div>
										</div>

										<div className="space-y-1.5">
											<div className="flex justify-between items-center">
												<label className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider">ACCESS SECRET</label>
												<button
													type="button"
													onClick={() => setStep('reset-password')}
													className="text-[10px] text-blue-400 hover:underline hover:text-blue-300 transition-colors"
												>
													忘記金鑰？
												</button>
											</div>
											<div className="relative">
												<input
													type={showPassword ? 'text' : 'password'}
													placeholder="••••••••"
													value={password}
													onChange={(e) => setPassword(e.target.value)}
													disabled={loading}
													required
													className="w-full pl-10 pr-10 py-2.5 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-sm text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 transition-colors"
												/>
												<Lock className="absolute left-3 top-3 w-4 h-4 text-gray-500 pointer-events-none" />
												<button
													type="button"
													onClick={() => setShowPassword(!showPassword)}
													className="absolute right-3 top-2.5 text-gray-500 hover:text-gray-300"
												>
													{showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
												</button>
											</div>
										</div>

										{error && (
											<div className="flex items-center gap-2 p-3 text-xs border border-red-500/15 bg-red-500/5 text-red-400 rounded-sm">
												<ShieldAlert className="w-4 h-4 shrink-0" />
												<span>{error}</span>
											</div>
										)}

										<button
											type="submit"
											disabled={loading}
											className="w-full py-3 bg-blue-500/10 hover:bg-blue-500/15 border border-blue-500/25 active:bg-blue-500/20 rounded-sm font-bold text-sm tracking-[0.2em] text-blue-400 disabled:opacity-30 transition-all cursor-pointer flex items-center justify-center gap-2"
										>
											{loading ? (
												<>
													<Loader2 className="w-4 h-4 animate-spin" />
													VERIFYING CREDS...
												</>
											) : (
												'SIGN IN'
											)}
										</button>
									</form>

									<div className="mt-6 font-mono text-center">
										<p className="text-[11px] text-[var(--text-muted)]">
											還沒有專屬帳戶？{' '}
											<button
												onClick={() => {
													setError(null);
													setPassword('');
													setStep('register');
												}}
												className="text-blue-400 hover:underline hover:text-blue-300 font-bold transition-colors"
											>
												建立新帳密
											</button>
										</p>
									</div>
								</div>
							)}

							{/* --- STEP 3: USER REGISTER --- */}
							{step === 'register' && (
								<div className="w-full flex flex-col items-center">
									<div className="text-center mb-6">
										<h1 className="text-xl font-medium tracking-tight mb-2 text-white">
											創建量化交易帳戶
										</h1>
										<p className="text-xs text-[var(--text-muted)] font-mono">
											請建立您的專屬郵箱與安全存取金鑰
										</p>
									</div>

									<form onSubmit={handleRegister} className="w-full space-y-3.5 font-mono text-left">
										<div className="space-y-1">
											<label className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider">EMAIL ADDRESS</label>
											<div className="relative">
												<input
													type="email"
													placeholder="example@domain.com"
													value={email}
													onChange={(e) => setEmail(e.target.value)}
													disabled={loading}
													required
													className="w-full pl-10 pr-4 py-2 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-sm text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 transition-colors"
												/>
												<Mail className="absolute left-3 top-2.5 w-4 h-4 text-gray-500 pointer-events-none" />
											</div>
										</div>

										<div className="space-y-1">
											<label className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider">ACCESS SECRET (MIN 6 CHARS)</label>
											<div className="relative">
												<input
													type={showPassword ? 'text' : 'password'}
													placeholder="••••••••"
													value={password}
													onChange={(e) => setPassword(e.target.value)}
													disabled={loading}
													required
													className="w-full pl-10 pr-10 py-2 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-sm text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 transition-colors"
												/>
												<Lock className="absolute left-3 top-2.5 w-4 h-4 text-gray-500 pointer-events-none" />
												<button
													type="button"
													onClick={() => setShowPassword(!showPassword)}
													className="absolute right-3 top-2 text-gray-500 hover:text-gray-300"
												>
													{showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
												</button>
											</div>
										</div>

										<div className="space-y-1">
											<label className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider">CONFIRM ACCESS SECRET</label>
											<div className="relative">
												<input
													type={showConfirmPassword ? 'text' : 'password'}
													placeholder="••••••••"
													value={confirmPassword}
													onChange={(e) => setConfirmPassword(e.target.value)}
													disabled={loading}
													required
													className="w-full pl-10 pr-10 py-2 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-sm text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 transition-colors"
												/>
												<Lock className="absolute left-3 top-2.5 w-4 h-4 text-gray-500 pointer-events-none" />
												<button
													type="button"
													onClick={() => setShowConfirmPassword(!showConfirmPassword)}
													className="absolute right-3 top-2 text-gray-500 hover:text-gray-300"
												>
													{showConfirmPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
												</button>
											</div>
										</div>

										{betaMode && (
											<div className="space-y-1">
												<label className="text-[10px] text-blue-400 uppercase tracking-wider font-bold">BETA ACCESS KEY *</label>
												<div className="relative">
													<input
														type="text"
														placeholder="請輸入6位數邀請碼"
														value={betaCode}
														onChange={(e) => setBetaCode(e.target.value.replace(/[^a-z0-9]/gi, '').toLowerCase())}
														maxLength={6}
														disabled={loading}
														required
														className="w-full pl-10 pr-4 py-2 bg-[#161a21]/90 border border-blue-500/20 rounded-sm text-sm font-bold font-mono text-white focus:outline-none focus:border-blue-500/50 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 transition-colors"
													/>
													<UserPlus className="absolute left-3 top-2.5 w-4 h-4 text-blue-500 pointer-events-none" />
												</div>
											</div>
										)}

										{error && (
											<div className="flex items-center gap-2 p-3 text-xs border border-red-500/15 bg-red-500/5 text-red-400 rounded-sm">
												<ShieldAlert className="w-4 h-4 shrink-0" />
												<span>{error}</span>
											</div>
										)}

										<button
											type="submit"
											disabled={loading}
											className="w-full py-3 mt-2 bg-blue-500/10 hover:bg-blue-500/15 border border-blue-500/25 active:bg-blue-500/20 rounded-sm font-bold text-sm tracking-[0.2em] text-blue-400 disabled:opacity-30 transition-all cursor-pointer flex items-center justify-center gap-2"
										>
											{loading ? (
												<>
													<Loader2 className="w-4 h-4 animate-spin" />
													GENERATING KEYS...
												</>
											) : (
												'CREATE IDENTITY'
											)}
										</button>
									</form>

									<button
										onClick={handleBackToLogin}
										className="mt-6 flex items-center gap-2 text-[10px] text-gray-500 hover:text-white font-mono uppercase tracking-wider transition-colors"
									>
										<ArrowLeft className="w-3.5 h-3.5" />
										返回登入介面
									</button>
								</div>
							)}

							{/* --- STEP 4: OTP SETUP --- */}
							{step === 'setup-otp' && (
								<div className="w-full flex flex-col items-center font-mono">
									<div className="text-center mb-6">
										<h1 className="text-lg font-medium text-white tracking-tight mb-2">
											初始化雙因子密鑰閘道
										</h1>
										<p className="text-xs text-[var(--text-muted)] max-w-sm leading-relaxed">
											為了您的實時資產與量化模型安全，系統強制使用 Google Authenticator 雙因子驗證器。
										</p>
									</div>

									<div className="w-full space-y-5 text-left">
										{/* QR Code Container */}
										{qrCodeURL && (
											<div className="flex flex-col items-center gap-3 p-4 bg-white/5 border border-[var(--border)] rounded-sm">
												<span className="text-[10px] text-blue-400 font-bold uppercase tracking-wider">
													STEP 1: 掃描二維碼 (SCAN QR CODE)
												</span>
												<div className="p-2.5 bg-white rounded-sm">
													<img 
														src={`https://api.qrserver.com/v1/create-qr-code/?size=160x160&data=${encodeURIComponent(qrCodeURL)}`} 
														alt="TOTP QR Code"
														className="w-40 h-40"
													/>
												</div>
											</div>
										)}

										{/* Key Copy Container */}
										<div className="p-4 bg-white/5 border border-[var(--border)] rounded-sm space-y-2">
											<span className="text-[10px] text-blue-400 font-bold uppercase tracking-wider">
												STEP 2: 或手動鍵入密鑰 (OR ENTER SECRET KEY)
											</span>
											<div className="flex gap-2">
												<code className="flex-1 px-3 py-2 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-xs text-white font-bold select-all flex items-center overflow-x-auto whitespace-nowrap">
													{otpSecret}
												</code>
												<button
													type="button"
													onClick={handleCopySecret}
													className="px-3.5 py-2 bg-blue-500/10 hover:bg-blue-500/15 border border-blue-500/25 active:bg-blue-500/20 text-blue-400 rounded-sm font-bold text-xs transition-all flex items-center justify-center gap-1.5"
												>
													{copied ? (
														<>
															<Check className="w-3.5 h-3.5" />
															COPIED
														</>
													) : (
														<>
															<Copy className="w-3.5 h-3.5" />
															COPY
														</>
													)}
												</button>
											</div>
										</div>

										{/* Instructions Box */}
										<div className="p-3 text-[10px] border border-blue-500/10 bg-blue-500/5 text-blue-300 rounded-sm leading-relaxed">
											💡 提示：請在手機上下載 Google Authenticator，掃描上方 QR Code，或手動新增密鑰，認證器會自動生成 6 位數安全驗證碼。
										</div>

										<button
											onClick={() => {
												setOtpActionType('register');
												setStep('verify-otp');
											}}
											className="w-full py-3 bg-blue-500/10 hover:bg-blue-500/15 border border-blue-500/25 active:bg-blue-500/20 rounded-sm font-bold text-sm tracking-[0.2em] text-blue-400 transition-all flex items-center justify-center gap-2 cursor-pointer"
										>
											我已設定完畢，進行首步驗證
										</button>
									</div>
								</div>
							)}

							{/* --- STEP 5: OTP VERIFICATION --- */}
							{step === 'verify-otp' && (
								<div className="w-full flex flex-col items-center">
									<div className="text-center mb-6">
										<div className="w-12 h-12 mb-4 mx-auto flex items-center justify-center rounded-full bg-blue-500/5 border border-blue-500/10">
											<QrCode className="w-5 h-5 text-blue-400 animate-pulse" />
										</div>
										<h1 className="text-xl font-medium tracking-tight mb-2 text-white">
											輸入 2FA 驗證代碼
										</h1>
										<p className="text-xs text-[var(--text-muted)] font-mono leading-relaxed max-w-xs">
											{otpActionType === 'login' 
												? '開啟 Google Authenticator，輸入本帳密對應的 6 位數代碼' 
												: '輸入二維碼綁定成功的 6 位數認證代碼以啟用您的交易帳戶'}
										</p>
									</div>

									<form onSubmit={handleOTPVerify} className="w-full space-y-4 font-mono">
										<div>
											<input
												type="text"
												value={otpCode}
												onChange={(e) => setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
												placeholder="0 0 0 0 0 0"
												maxLength={6}
												disabled={loading}
												autoFocus
												required
												className="w-full px-4 py-3 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-center text-2xl font-bold tracking-[0.4em] font-mono text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 placeholder:tracking-normal placeholder:font-normal placeholder:text-gray-700 transition-colors"
											/>
										</div>

										{error && (
											<div className="flex items-center gap-2 p-3 text-xs border border-red-500/15 bg-red-500/5 text-red-400 rounded-sm text-left">
												<ShieldAlert className="w-4 h-4 shrink-0" />
												<span>{error}</span>
											</div>
										)}

										<div className="flex gap-3">
											<button
												type="button"
												onClick={() => setStep(otpActionType === 'login' ? 'login' : 'setup-otp')}
												className="flex-1 py-3 bg-[#161a21] hover:bg-[#1f242d] border border-[var(--border)] active:bg-[#161a21] rounded-sm font-bold text-xs tracking-wider text-gray-400 transition-all cursor-pointer"
											>
												返回上步
											</button>
											
											<button
												type="submit"
												disabled={loading || otpCode.length !== 6}
												className="flex-1 py-3 bg-blue-500/10 hover:bg-blue-500/15 border border-blue-500/25 active:bg-blue-500/20 rounded-sm font-bold text-xs tracking-wider text-blue-400 disabled:opacity-30 transition-all cursor-pointer flex items-center justify-center gap-2"
											>
												{loading ? (
													<>
														<Loader2 className="w-3.5 h-3.5 animate-spin" />
														VERIFYING...
													</>
												) : (
													'COMPLETE'
												)}
											</button>
										</div>
									</form>
								</div>
							)}

							{/* --- STEP 6: PASSWORD RESET --- */}
							{step === 'reset-password' && (
								<div className="w-full flex flex-col items-center">
									<div className="text-center mb-6">
										<div className="w-12 h-12 mb-4 mx-auto flex items-center justify-center rounded-full bg-blue-500/5 border border-blue-500/10">
											<KeyRound className="w-5 h-5 text-blue-400 animate-pulse" />
										</div>
										<h1 className="text-xl font-medium tracking-tight mb-2 text-white">
											密碼安全重設
										</h1>
										<p className="text-xs text-[var(--text-muted)] font-mono">
											請提供關聯郵箱、新金鑰以及 6 位數 2FA 代碼
										</p>
									</div>

									<form onSubmit={handleResetPassword} className="w-full space-y-3.5 font-mono text-left">
										<div className="space-y-1">
											<label className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider">EMAIL ADDRESS</label>
											<div className="relative">
												<input
													type="email"
													placeholder="example@domain.com"
													value={email}
													onChange={(e) => setEmail(e.target.value)}
													disabled={loading}
													required
													className="w-full pl-10 pr-4 py-2 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-sm text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 transition-colors"
												/>
												<Mail className="absolute left-3 top-2.5 w-4 h-4 text-gray-500 pointer-events-none" />
											</div>
										</div>

										<div className="space-y-1">
											<label className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider">NEW ACCESS SECRET (MIN 6 CHARS)</label>
											<div className="relative">
												<input
													type={showPassword ? 'text' : 'password'}
													placeholder="••••••••"
													value={password}
													onChange={(e) => setPassword(e.target.value)}
													disabled={loading}
													required
													className="w-full pl-10 pr-10 py-2 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-sm text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 transition-colors"
												/>
												<Lock className="absolute left-3 top-2.5 w-4 h-4 text-gray-500 pointer-events-none" />
												<button
													type="button"
													onClick={() => setShowPassword(!showPassword)}
													className="absolute right-3 top-2 text-gray-500 hover:text-gray-300"
												>
													{showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
												</button>
											</div>
										</div>

										<div className="space-y-1">
											<label className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider">CONFIRM NEW SECRET</label>
											<div className="relative">
												<input
													type={showConfirmPassword ? 'text' : 'password'}
													placeholder="••••••••"
													value={confirmPassword}
													onChange={(e) => setConfirmPassword(e.target.value)}
													disabled={loading}
													required
													className="w-full pl-10 pr-10 py-2 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-sm text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 transition-colors"
												/>
												<Lock className="absolute left-3 top-2.5 w-4 h-4 text-gray-500 pointer-events-none" />
												<button
													type="button"
													onClick={() => setShowConfirmPassword(!showConfirmPassword)}
													className="absolute right-3 top-2 text-gray-500 hover:text-gray-300"
												>
													{showConfirmPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
												</button>
											</div>
										</div>

										<div className="space-y-1">
											<label className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider">OTP CODE FROM AUTHENTICATOR</label>
											<div className="relative">
												<input
													type="text"
													placeholder="0 0 0 0 0 0"
													value={otpCode}
													onChange={(e) => setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
													maxLength={6}
													disabled={loading}
													required
													className="w-full px-4 py-2 bg-[#161a21]/90 border border-[var(--border)] rounded-sm text-center text-base tracking-[0.25em] font-bold font-mono text-white focus:outline-none focus:border-blue-500/40 focus:ring-1 focus:ring-blue-500/20 disabled:opacity-50 transition-colors"
												/>
											</div>
										</div>

										{error && (
											<div className="flex items-center gap-2 p-3 text-xs border border-red-500/15 bg-red-500/5 text-red-400 rounded-sm">
												<ShieldAlert className="w-4 h-4 shrink-0" />
												<span>{error}</span>
											</div>
										)}

										<button
											type="submit"
											disabled={loading || otpCode.length !== 6}
											className="w-full py-3 mt-2 bg-blue-500/10 hover:bg-blue-500/15 border border-blue-500/25 active:bg-blue-500/20 rounded-sm font-bold text-sm tracking-[0.2em] text-blue-400 disabled:opacity-30 transition-all cursor-pointer flex items-center justify-center gap-2"
										>
											{loading ? (
												<>
													<Loader2 className="w-4 h-4 animate-spin" />
													RESETTING SECRET...
												</>
											) : (
												'RESET ACCESS SECRET'
											)}
										</button>
									</form>

									<button
										onClick={handleBackToLogin}
										className="mt-6 flex items-center gap-2 text-[10px] text-gray-500 hover:text-white font-mono uppercase tracking-wider transition-colors"
									>
										<ArrowLeft className="w-3.5 h-3.5" />
										返回登入介面
									</button>
								</div>
							)}
						</div>
					</motion.div>
				)}
			</AnimatePresence>
		</div>
	)
}
