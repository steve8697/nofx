import { useEffect, useState } from 'react'
import { api } from '../../lib/api'

export function StatusBar() {
  const [time, setTime] = useState(new Date())
  const [serverIp, setServerIp] = useState<string>('RETRIEVING...')

  useEffect(() => {
    const timer = setInterval(() => setTime(new Date()), 1000)
    return () => clearInterval(timer)
  }, [])

  useEffect(() => {
    let mounted = true
    const fetchIp = () => {
      const token = localStorage.getItem('auth_token')
      if (!token) {
        if (mounted) setServerIp('OFFLINE')
        return
      }
      api.getServerIP()
        .then(res => {
          if (mounted && res.public_ip) {
            setServerIp(res.public_ip)
          }
        })
        .catch(() => {
          if (mounted) setServerIp('OFFLINE')
        })
    }

    fetchIp()
    const ipTimer = setInterval(fetchIp, 60000)
    
    const handleStorage = (e: StorageEvent) => {
      if (e.key === 'auth_token') {
        fetchIp()
      }
    }
    window.addEventListener('storage', handleStorage)

    return () => {
      mounted = false
      clearInterval(ipTimer)
      window.removeEventListener('storage', handleStorage)
    }
  }, [])

  const timeStr = time.toLocaleTimeString('en-US', { hour12: false })

  return (
    <div className="h-8 border-t border-[var(--border)] bg-[var(--bg-elev)] flex items-center px-4 text-[10px] font-mono text-[var(--text-dim)] justify-between select-none">
      <div className="flex items-center gap-4">
        <span>AETHERIS web-beta (experimental, not in Docker)</span>
        <span className="text-[var(--border-subtle)]">|</span>
        <span>API /api → :3636</span>
        <span className="text-[var(--border-subtle)]">|</span>
        <span>NODE IP: {serverIp}</span>
      </div>

      <div className="flex items-center gap-4">
        <span>LAST SYNC: JUST NOW</span>
        <span className="text-[var(--border-subtle)]">|</span>
        <span>{timeStr}</span>
      </div>
    </div>
  )
}
