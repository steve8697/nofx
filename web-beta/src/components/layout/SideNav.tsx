import { useEffect, useState, useCallback } from 'react'
import { Users, BarChart3, Trophy, Settings, ChevronRight, Brain } from 'lucide-react'
import { api } from '../../lib/api'
import type { TraderInfo } from '../../lib/api'

interface SideNavProps {
	currentView: string
	onViewChange: (view: string) => void
	selectedTrader?: string
	onTraderChange?: (traderId: string) => void
}

export function SideNav({ currentView, onViewChange, selectedTrader, onTraderChange }: SideNavProps) {
	const [traders, setTraders] = useState<TraderInfo[]>([])

	const loadTraders = useCallback(() => {
		const token = localStorage.getItem('auth_token')
		const fetchMethod = token ? api.getTraders : api.getPublicTraders
		
		fetchMethod()
			?.then(t => {
				if (Array.isArray(t)) {
					setTraders(t)
				}
			})
			.catch(err => {
				console.error('Failed to load traders in SideNav:', err)
				// If authorized fetch fails (e.g. 401), fallback silently
				if (token) {
					api.getPublicTraders()?.then(t => {
						if (Array.isArray(t)) setTraders(t)
					}).catch(console.error)
				}
			})
	}, [])

	useEffect(() => {
		loadTraders()
		// Listen for storage events to reload traders when authenticated
		const handleStorage = (e: StorageEvent) => {
			if (e.key === 'auth_token') loadTraders()
		}
		window.addEventListener('storage', handleStorage)
		const timer = setInterval(loadTraders, 20000)
		return () => {
			clearInterval(timer)
			window.removeEventListener('storage', handleStorage)
		}
	}, [loadTraders])

	const navItems = [
		{ id: 'dashboard', label: 'Dashboard', icon: BarChart3 },
		{ id: 'traders', label: 'Traders', icon: Users },
		{ id: 'diagnostics', label: 'Intelligence', icon: Brain },
		{ id: 'competition', label: 'Competition', icon: Trophy },
		{ id: 'settings', label: 'Settings', icon: Settings },
	]

	return (
		<div className="w-56 border-r border-[var(--border)] bg-[var(--bg-elev)] flex flex-col h-full">
			{/* Quick Trader Switcher */}
			<div className="p-4 border-b border-[var(--border)]">
				<div className="text-[10px] font-mono tracking-widest text-[var(--text-dim)] mb-2 px-1">ACTIVE TRADERS</div>
				<div className="space-y-px max-h-40 overflow-y-auto">
					{traders.map(t => (
						<button
							key={t.trader_id}
							onClick={() => {
								onTraderChange?.(t.trader_id)
								if (currentView !== 'dashboard') onViewChange('dashboard')
							}}
							className={`w-full flex items-center justify-between px-3 py-2 text-left rounded-sm text-sm font-mono transition-colors
								${selectedTrader === t.trader_id 
									? 'bg-[var(--bg-panel)] text-[var(--text)] border-l-2 border-[var(--accent)]' 
									: 'text-[var(--text-muted)] hover:bg-[var(--bg-subtle)] hover:text-[var(--text)]'}`}
						>
							<span className="truncate">{t.trader_name}</span>
							<span className={`text-[10px] ${t.is_running ? 'text-[var(--green)]' : 'text-[var(--text-dim)]'}`}>
								{t.is_running ? '●' : '○'}
							</span>
						</button>
					))}
				</div>
			</div>

			{/* Navigation */}
			<div className="p-2 flex-1">
				<div className="text-[10px] font-mono tracking-widest text-[var(--text-dim)] px-3 py-2">NAVIGATION</div>
				{navItems.map(item => {
					const Icon = item.icon
					const active = currentView === item.id
					return (
						<button
							key={item.id}
							onClick={() => onViewChange(item.id)}
							className={`w-full flex items-center gap-3 px-3 py-2.5 text-sm font-mono rounded-sm mb-px transition-all
								${active 
									? 'bg-[var(--bg-panel)] text-[var(--text)]' 
									: 'text-[var(--text-muted)] hover:bg-[var(--bg-subtle)] hover:text-[var(--text)]'}`}
						>
							<Icon size={16} />
							<span>{item.label}</span>
							{active && <ChevronRight size={14} className="ml-auto text-[var(--text-dim)]" />}
						</button>
					)
				})}
			</div>

			{/* Footer */}
			<div className="p-4 border-t border-[var(--border)] text-[10px] font-mono text-[var(--text-dim)]">
				BETA v0.1
			</div>
		</div>
	)
}
