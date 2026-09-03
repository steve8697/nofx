import { useState, useEffect } from 'react'
import { TopBar } from './components/layout/TopBar'
import { SideNav } from './components/layout/SideNav'
import { StatusBar } from './components/layout/StatusBar'
import { DashboardPage } from './pages/Dashboard'
import { TradersPage } from './pages/Traders'
import { CompetitionPage } from './pages/Competition'
import { SettingsPage } from './pages/Settings'
import { DiagnosticsPage } from './pages/Diagnostics'
import { UnlockScreen } from './components/auth/UnlockScreen'
import { Toaster } from 'sonner'

function App() {
  const [currentView, setCurrentView] = useState<'dashboard' | 'traders' | 'competition' | 'settings' | 'diagnostics'>('dashboard')
  const [selectedTrader, setSelectedTrader] = useState<string>('')
  const [isUnlocked, setIsUnlocked] = useState(!!localStorage.getItem('auth_token'))

  // Listen to global authorization token storage changes
  useEffect(() => {
    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === 'auth_token') {
        setIsUnlocked(!!e.newValue)
      }
    }

    // Custom window listener for same-window storage events dispatched by api.ts
    window.addEventListener('storage', handleStorageChange)
    return () => {
      window.removeEventListener('storage', handleStorageChange)
    }
  }, [])

  const renderPage = () => {
    switch (currentView) {
      case 'dashboard':
        return <DashboardPage selectedTraderId={selectedTrader} />
      case 'traders':
        return <TradersPage />
      case 'diagnostics':
        return <DiagnosticsPage selectedTraderId={selectedTrader} />
      case 'competition':
        return <CompetitionPage />
      case 'settings':
        return <SettingsPage />
      default:
        return <DashboardPage selectedTraderId={selectedTrader} />
    }
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-[var(--bg)] text-[var(--text)]">
      <Toaster theme="dark" position="top-right" closeButton richColors />
      
      {!isUnlocked && (
        <UnlockScreen onUnlock={() => setIsUnlocked(true)} />
      )}

      <TopBar currentView={currentView} onViewChange={(v) => setCurrentView(v as any)} />

      <div className="flex flex-1 overflow-hidden">
        <SideNav
          currentView={currentView}
          onViewChange={(v) => setCurrentView(v as any)}
          selectedTrader={selectedTrader}
          onTraderChange={setSelectedTrader}
        />

        <div className="flex-1 overflow-auto">
          {renderPage()}
        </div>
      </div>

      <StatusBar />
    </div>
  )
}

export default App
