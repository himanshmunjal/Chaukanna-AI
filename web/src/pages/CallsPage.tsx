// Home page: headline numbers + every call, updating live.

import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type Call, type Stats } from '../lib/api'
import { SCAM_TYPES, formatSeconds, timeAgo } from '../lib/format'
import { useLiveEvents } from '../lib/live'
import { useT } from '../i18n'
import { Card, ErrorNote, LiveDot, RiskBar, StateBadge } from '../components/ui'

export function CallsPage() {
  const t = useT()
  const [calls, setCalls] = useState<Call[]>([])
  const [stats, setStats] = useState<Stats | null>(null)
  const [activeIds, setActiveIds] = useState<number[]>([])
  const [error, setError] = useState<string | null>(null)
  const reloadTimer = useRef<number>()

  const load = useCallback(async () => {
    try {
      const [c, s, a] = await Promise.all([api.calls(), api.stats(), api.activeCalls()])
      setCalls(c)
      setStats(s)
      setActiveIds(a.ids)
      setError(null)
    } catch (e) {
      setError((e as Error).message)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  // Reload (at most twice a second) whenever something happens on any call.
  useLiveEvents((event) => {
    if (['call_started', 'call_ended', 'analysis', 'warning'].includes(event.type)) {
      window.clearTimeout(reloadTimer.current)
      reloadTimer.current = window.setTimeout(load, 500)
    }
  })

  return (
    <div className="space-y-10">
      <div>
        <h1 className="text-4xl text-slate-100 sm:text-5xl">{t('Your family, protected.')}</h1>
        <p className="mt-3 max-w-xl text-slate-400">{t('Every call, listened to quietly and flagged the moment it sounds like a scam.')}</p>
      </div>
      <ErrorNote message={error && t('Cannot reach the backend: {error}', { error })} />

      <div className="grid grid-cols-2 gap-px overflow-hidden rounded-2xl border border-slate-800 bg-slate-800 lg:grid-cols-4">
        <StatTile label={t('Calls protected')} value={stats?.total_calls ?? '–'} />
        <StatTile label={t('Scams detected')} value={stats?.scams_detected ?? '–'}  />
        <StatTile label={t('Warnings played')} value={stats?.warnings_played ?? '–'}  />
        <StatTile
          label={t('Avg. time to detect')}
          value={stats?.avg_detection_sec != null ? `${stats.avg_detection_sec.toFixed(1)} s` : '–'}
          
        />
      </div>

      {((stats?.labelled_scam ?? 0) + (stats?.labelled_legit ?? 0) > 0) && (
        <section className="rounded-2xl border border-slate-800 bg-slate-900 p-5 text-sm text-slate-300">
          <p className="font-medium text-slate-100">
            Test calls labelled: {(stats?.labelled_scam ?? 0) + (stats?.labelled_legit ?? 0)} ({stats?.labelled_scam ?? 0} scam, {stats?.labelled_legit ?? 0} normal)
          </p>
          <p className="mt-1 text-slate-400">Export them with:</p>
          <code className="mt-1 block overflow-x-auto rounded-lg bg-slate-800 p-3 text-xs text-slate-300">go run ./cmd/exportcalls -labelled -out ../benchmark/fresh</code>
        </section>
      )}

      <Card title={t('Calls')}>
        {calls.length === 0 ? (
          <p className="text-sm text-slate-500">
            {t('No calls yet.')} <Link to="/simulator" className="text-cyan-400 underline underline-offset-4">{t('Try a scam call in the Simulator')}</Link>
          </p>
        ) : (
          <div className="divide-y divide-slate-800">
            {calls.map((call) => (
              <CallRow key={call.id} call={call} live={activeIds.includes(call.id)} />
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}

function StatTile({ label, value, accent = 'text-slate-100' }: { label: string; value: string | number; accent?: string }) {
  return (
    <div className="bg-slate-900 p-5">
      <p className="text-sm text-slate-400">{label}</p>
      <p className={`mt-2 font-display text-4xl font-normal tabular-nums ${accent}`}>{value}</p>
    </div>
  )
}

function CallRow({ call, live }: { call: Call; live: boolean }) {
  const t = useT()
  const who = call.mode === 'screen'
    ? call.caller_name || call.caller_number || t('Unknown caller')
    : t("{name}'s call", { name: call.protected_user?.name ?? t('User') })

  return (
    <Link to={`/calls/${call.id}`} className="flex flex-wrap items-center gap-x-4 gap-y-2 -mx-3 rounded-xl px-3 py-4 transition-colors hover:bg-slate-950 sm:flex-nowrap">
      <div className="flex w-6 justify-center">{live ? <LiveDot /> : <span className="text-xs text-slate-600">#{call.id}</span>}</div>
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium text-slate-100">{who}</p>
        <p className="text-xs text-slate-500">
          {call.mode === 'screen' ? t('Call screening') : t('Live protection')} · {call.provider} · {timeAgo(call.started_at, t)}
          {call.detected_after_sec != null && ` · ${t('caught at {time}', { time: formatSeconds(call.detected_after_sec) })}`}
        </p>
      </div>
      {call.test_label && (
        <span className="rounded-full bg-slate-800 px-2.5 py-0.5 text-xs text-slate-300" title="Your label for scoring">
          {call.test_label === 'legit' ? 'Normal' : SCAM_TYPES[call.test_category] ?? 'Scam'}
        </span>
      )}
      <RiskBar risk={call.risk} state={call.state} />
      <StateBadge state={call.state} />
    </Link>
  )
}
