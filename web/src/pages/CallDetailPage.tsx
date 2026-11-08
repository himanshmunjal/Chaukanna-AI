// One call, live: transcript, risk gauge, red-flag timeline, warnings, alerts.

import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, type Call, type RedFlag, type TranscriptLine } from '../lib/api'
import { LANGUAGES, SCAM_TYPES, formatSeconds } from '../lib/format'
import { useLiveEvents } from '../lib/live'
import { useAccess } from '../lib/access'
import { useT } from '../i18n'
import { Card, ErrorNote, LiveDot, StateBadge } from '../components/ui'
import { RiskGauge } from '../components/RiskGauge'
import { FlagTimeline } from '../components/FlagTimeline'
import { TranscriptView } from '../components/TranscriptView'
import { ElderAlert } from '../components/ElderAlert'
import { FamilyControls } from '../components/FamilyControls'
import { WhatNextCard } from '../components/WhatNextCard'
import { TestLabelCard } from '../components/TestLabelCard'

export function CallDetailPage() {
  const { id } = useParams()
  return <CallView callId={Number(id)} />
}

/** The live view of one call. Also embedded in the Speakerphone page. */
export function CallView({ callId, elderScreen = false }: { callId: number; elderScreen?: boolean }) {
  const t = useT()
  const [call, setCall] = useState<Call | null>(null)
  const [lines, setLines] = useState<TranscriptLine[]>([])
  const [flags, setFlags] = useState<RedFlag[]>([])
  const [partial, setPartial] = useState('')
  const [warning, setWarning] = useState<string | null>(null)
  const [caution, setCaution] = useState<string | null>(null)
  const [played, setPlayed] = useState(true)
  const [complying, setComplying] = useState<string | null>(null)
  const [safe, setSafe] = useState(false)
  const [live, setLive] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const audio = useAudioQueue()
  const { access } = useAccess()
  const demoVisitor = access.demo && !access.family

  const load = useCallback(async () => {
    try {
      const [c, active] = await Promise.all([api.call(callId), api.activeCalls()])
      setCall(c)
      setLines(c.transcript ?? [])
      setFlags(c.flags ?? [])
      setLive(active.ids.includes(callId))
      if (c.complying_at) setComplying((prev) => prev ?? '')
      setSafe(!!c.marked_safe_at)
      const lastBotLine = (c.transcript ?? []).filter((l) => l.speaker === 'chaukanna').at(-1)
      if (c.warned_at && lastBotLine) {
        setWarning(lastBotLine.text)
        setPlayed(c.warning_played)
      }
    } catch (e) {
      setError((e as Error).message)
    }
  }, [callId])

  useEffect(() => {
    load()
  }, [load])

  // Apply live events for this call as they arrive.
  useLiveEvents((event) => {
    if (event.call_id !== callId) return
    const d = event.data
    switch (event.type) {
      case 'transcript_partial':
        setPartial(d.text)
        break
      case 'transcript':
        setPartial('')
        setLines((prev) => [...prev, d])
        break
      case 'flags':
        setFlags((prev) => [...prev, ...d.flags.filter((f: RedFlag) => !prev.some((p) => p.type === f.type))])
        break
      case 'analysis':
        setCall((prev) => (prev ? { ...prev, state: d.state, risk: d.risk, reason: d.reason } : prev))
        break
      case 'warning':
        setWarning(d.text)
        setPlayed(d.audio_played)
        setCall((prev) => (prev ? { ...prev, scam_type: d.scam_type ?? prev.scam_type } : prev))
        break
      case 'caution':
        setCaution(d.text)
        break
      case 'complying':
        setComplying(d.heard)
        break
      case 'call_safe':
        setSafe(d.safe)
        break
      case 'call_updated':
        setCall((prev) => (prev ? { ...prev, ...d } : prev))
        break
      case 'call_audio':
        audio.play(d.audio)
        break
      case 'call_ended':
        setLive(false)
        setPartial('')
        window.setTimeout(load, 800) // pick up final summary and alerts
        break
    }
  })

  if (error) return <ErrorNote message={error} />
  if (!call) return <p className="text-slate-500">{t('Loading…')}</p>

  const isScreen = call.mode === 'screen'

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex flex-wrap items-center gap-3">
        <Link to="/" className="text-sm text-slate-400 hover:text-slate-200">← {t('All calls')}</Link>
        <h1 className="text-3xl text-slate-100">
          {isScreen ? t('Screened call') : t('Protected call')} #{call.id}
        </h1>
        {live && (
          <span className="flex items-center gap-2 text-sm font-medium text-red-300">
            <LiveDot /> {t('LIVE')}
          </span>
        )}
        <StateBadge state={call.state} />
        <span className="text-sm text-slate-500">
          {call.protected_user?.name} · {LANGUAGES[call.language] ?? call.language} · {call.provider}
        </span>
        <button
          onClick={audio.toggle}
          className="ml-auto rounded-full border border-slate-700 bg-slate-900 px-3.5 py-1.5 text-xs text-slate-300 hover:bg-slate-800"
        >
          {audio.enabled ? t('Sound on') : t('Sound off')}
        </button>
      </div>

      {/* Big warning banner */}
      {warning && (
        <div className="rounded-2xl border border-red-500/30 bg-red-500/10 p-5">
          <p className="text-sm font-medium text-red-300">
            {t('Scam detected')}{call.detected_after_sec != null && ` ${t('at {time}', { time: formatSeconds(call.detected_after_sec) })}`}
            {call.scam_type && SCAM_TYPES[call.scam_type] && ` · ${t(SCAM_TYPES[call.scam_type])}`}
            {played ? `. ${t('Warning spoken on the call')}` : demoVisitor ? '. Shown as text: this clip is not prepared in the demo' : `. ${t('The warning could NOT be played: please call them now')}`}
          </p>
          <p className="mt-2 text-lg leading-relaxed text-red-100">{warning}</p>
        </div>
      )}

      {safe && (
        <p className="rounded-2xl border border-emerald-500/30 bg-emerald-500/10 p-4 text-sm font-medium text-emerald-200">
          {t("The family marked this call safe. Chaukanna is still listening, but won't warn or send alerts.")}
        </p>
      )}

      {complying !== null && (
        <div role="alert" className="rounded-2xl border-2 border-red-400 bg-red-500/10 p-5">
          <p className="text-lg font-medium text-red-100">{t('They may be giving a code or money. Call them now')}{call.protected_user?.phone ? `: ${call.protected_user.phone}` : ''}</p>
          {complying && <p className="mt-1 text-red-100">{t('Heard')}: “{complying}”</p>}
        </div>
      )}

      {caution && !warning && (
        <div className="rounded-2xl border border-amber-500/30 bg-amber-500/10 p-5">
          <p className="text-sm font-medium text-amber-300">{t('Doubtful call: caution spoken')}</p>
          <p className="mt-2 text-lg leading-relaxed text-amber-100">{caution}</p>
        </div>
      )}

      {elderScreen && warning && !safe && <ElderAlert key={warning} text={warning} language={call.language} />}

      {live && !isScreen && <FamilyControls callId={callId} language={call.language} safe={safe} />}

      <div className="grid gap-5 lg:grid-cols-3">
        <Card title={t('Live transcript')} className="lg:col-span-2">
          <TranscriptView lines={lines} partial={partial} flags={flags} />
        </Card>

        <div className="space-y-5">
          <Card>
            <RiskGauge risk={call.risk} state={call.state} />
            {call.reason && <p className="mt-4 text-sm leading-relaxed text-slate-300">{call.reason}</p>}
          </Card>

          {isScreen && (call.caller_name || call.purpose || call.summary) && (
            <Card title={t('Screening result')}>
              <dl className="space-y-2 text-sm">
                {call.caller_number && <Row label={t('Number')} value={call.caller_number} />}
                {call.caller_name && <Row label={t('Caller')} value={call.caller_name} />}
                {call.purpose && <Row label={t('Purpose')} value={call.purpose} />}
              </dl>
              {call.summary && <p className="mt-3 rounded-lg bg-slate-800/60 p-3 text-sm text-slate-200">{call.summary}</p>}
            </Card>
          )}

          {!live && !isScreen && (call.state === 'scam' || call.state === 'warned') && (
            <WhatNextCard call={call} onChange={(outcome) => setCall({ ...call, outcome })} />
          )}

          {!live && access.family && (
            <TestLabelCard call={call} onChange={(test_label, test_category) => setCall({ ...call, test_label, test_category })} />
          )}

          <Card title={t('Red flags ({n})', { n: flags.length })}>
            <FlagTimeline flags={flags} />
          </Card>

          {(call.alerts?.length ?? 0) > 0 && (
            <Card title={t('Alerts sent')}>
              <ul className="space-y-2 text-sm">
                {call.alerts!.map((a) => (
                  <li key={a.id} className="flex items-center justify-between gap-2">
                    <span className="text-slate-300">
                      {a.channel} · {a.recipient}
                    </span>
                    <span className={a.status === 'failed' ? 'text-red-300' : 'text-emerald-300'} title={a.error}>
                      {t(a.status)}
                    </span>
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </div>
      </div>
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex gap-3">
      <dt className="w-20 shrink-0 text-slate-500">{label}</dt>
      <dd className="text-slate-200">{value}</dd>
    </div>
  )
}

/**
 * Plays base64 WAV clips one after another (simulator calls send the bot's
 * voice to the dashboard). Browsers block autoplay until the user clicks once,
 * so sound starts disabled and is turned on with the "Sound" button.
 */
function useAudioQueue() {
  const [enabled, setEnabled] = useState(false)
  const queue = useRef<string[]>([])
  const playing = useRef(false)
  const enabledRef = useRef(enabled)
  enabledRef.current = enabled

  const next = useCallback(() => {
    const clip = queue.current.shift()
    if (!clip) {
      playing.current = false
      return
    }
    playing.current = true
    const el = new Audio(`data:audio/wav;base64,${clip}`)
    el.onended = next
    el.onerror = next
    el.play().catch(next)
  }, [])

  return {
    enabled,
    toggle: () => setEnabled((v) => !v),
    play: (base64Wav: string) => {
      if (!enabledRef.current) return
      queue.current.push(base64Wav)
      if (!playing.current) next()
    },
  }
}
