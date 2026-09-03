import { Terminal } from 'lucide-react'

interface TopBarProps {
  currentView: string
  onViewChange: (view: string) => void
}

export function TopBar({ currentView, onViewChange }: TopBarProps) {
  const navItems = [
    { id: 'dashboard', label: 'DASHBOARD' },
    { id: 'traders', label: 'TRADERS' },
    { id: 'competition', label: 'COMPETITION' },
    { id: 'settings', label: 'SETTINGS' },
  ]

  return (
    <div className="h-14 border-b border-[var(--border)] bg-[var(--bg-elev)] flex items-center px-6 justify-between">
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2">
          <Terminal size={18} className="text-[var(--accent)]" />
          <span className="font-mono text-sm tracking-[2px] font-semibold text-[var(--text)]">AETHERIS</span>
          <span className="text-[10px] px-1.5 py-px rounded bg-[var(--bg-subtle)] text-[var(--text-dim)] font-mono">BETA</span>
        </div>
      </div>

      <nav className="flex items-center gap-1">
        {navItems.map(item => (
          <button
            key={item.id}
            onClick={() => onViewChange(item.id)}
            className={`px-4 py-1.5 text-xs font-mono tracking-widest transition-colors rounded-sm
              ${currentView === item.id 
                ? 'bg-[var(--bg-panel)] text-[var(--text)] border border-[var(--border-subtle)]' 
                : 'text-[var(--text-muted)] hover:text-[var(--text)] hover:bg-[var(--bg-subtle)]'}`}
          >
            {item.label}
          </button>
        ))}
      </nav>

      <div className="flex items-center gap-3 text-xs font-mono text-[var(--text-dim)]">
        <div className="flex items-center gap-1.5">
          <div className="w-1.5 h-1.5 rounded-full bg-[var(--green)] animate-pulse" />
          <span>CONNECTED</span>
        </div>
        <div className="w-px h-3 bg-[var(--border)]" />
        <span>ADMIN</span>
      </div>
    </div>
  )
}
