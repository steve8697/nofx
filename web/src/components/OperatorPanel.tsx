import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { OperatorDirective, OperatorEvent } from '../types'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'

export function OperatorPanel() {
  const { language } = useLanguage()
  const [directive, setDirective] = useState<OperatorDirective | null>(null)
  const [events, setEvents] = useState<OperatorEvent[]>([])
  const [note, setNote] = useState('')
  const [hours, setHours] = useState(4)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const refresh = async () => {
    try {
      const [d, ev] = await Promise.all([
        api.getOperatorDirective(),
        api.listOperatorEvents(12),
      ])
      setDirective(d.directive)
      setEvents(ev)
      setError('')
    } catch (e: any) {
      setError(e?.message || 'load failed')
    }
  }

  useEffect(() => {
    refresh()
    const id = setInterval(refresh, 15000)
    return () => clearInterval(id)
  }, [])

  const submit = async (action: string) => {
    setBusy(true)
    try {
      await api.createOperatorEvent({
        actor: 'web-ui',
        action,
        note: action === 'note' ? note : note,
        expires_in_minutes:
          action === 'resume_opens' ? -1 : Math.max(1, hours) * 60,
      })
      if (action === 'note') setNote('')
      await refresh()
    } catch (e: any) {
      setError(e?.message || 'save failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div
      className="glass-card p-5 md:p-6 border border-white/5"
      style={{ background: 'rgba(0,0,0,0.25)' }}
    >
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-base font-bold text-[#EAECEF]">
          {t('operatorTitle', language)}
        </h3>
        <span
          className="text-[10px] px-2 py-0.5 rounded"
          style={{
            background: directive?.pause_opens
              ? 'rgba(246,70,93,0.15)'
              : 'rgba(14,203,129,0.12)',
            color: directive?.pause_opens ? '#F6465D' : '#0ECB81',
          }}
        >
          {directive?.pause_opens
            ? t('operatorPaused', language)
            : t('operatorLive', language)}
        </span>
      </div>
      <p className="text-xs text-gray-500 mb-4">{t('operatorHelp', language)}</p>
      {directive?.pause_opens && (
        <div className="text-xs mb-3" style={{ color: '#F6465D' }}>
          {t('operatorPausedBy', language)} {directive.pause_actor || '—'}
          {directive.pause_until
            ? ` → ${new Date(directive.pause_until).toLocaleString()}`
            : ''}
        </div>
      )}
      <textarea
        value={note}
        onChange={(e) => setNote(e.target.value)}
        placeholder={t('operatorNotePlaceholder', language)}
        className="w-full px-3 py-2 rounded text-sm mb-3"
        style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
        rows={2}
      />
      <div className="flex flex-wrap items-center gap-2 mb-3">
        <label className="text-xs text-gray-500">
          {t('operatorHours', language)}
          <input
            type="number"
            min={1}
            max={72}
            value={hours}
            onChange={(e) => setHours(Number(e.target.value) || 4)}
            className="ml-2 w-16 px-2 py-1 rounded text-xs"
            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
          />
        </label>
        <button
          disabled={busy}
          onClick={() => submit('pause_opens')}
          className="px-3 py-1.5 rounded text-xs font-semibold"
          style={{ background: 'rgba(246,70,93,0.15)', color: '#F6465D' }}
        >
          {t('operatorPauseOpens', language)}
        </button>
        <button
          disabled={busy}
          onClick={() => submit('resume_opens')}
          className="px-3 py-1.5 rounded text-xs font-semibold"
          style={{ background: 'rgba(14,203,129,0.12)', color: '#0ECB81' }}
        >
          {t('operatorResumeOpens', language)}
        </button>
        <button
          disabled={busy || !note.trim()}
          onClick={() => submit('note')}
          className="px-3 py-1.5 rounded text-xs font-semibold"
          style={{ background: '#2B3139', color: '#EAECEF' }}
        >
          {t('operatorLeaveNote', language)}
        </button>
      </div>
      {error && <div className="text-xs mb-2" style={{ color: '#F6465D' }}>{error}</div>}
      <div className="space-y-1 max-h-40 overflow-y-auto">
        {events.map((e) => (
          <div key={e.id} className="text-[11px] text-gray-400 font-mono">
            {e.ts} · {e.actor} · {e.action}
            {e.note ? ` · ${e.note}` : ''}
          </div>
        ))}
        {events.length === 0 && (
          <div className="text-xs text-gray-600">{t('operatorNoEvents', language)}</div>
        )}
      </div>
    </div>
  )
}
