// Speakerphone mode: protect a call that is on speaker next to this laptop.
// No phone number or telephony provider needed: just Prisma, Evon and Timbre.

import { useEffect, useRef, useState } from 'react'
import { api, getGnaniKey, type ProtectedUser } from '../lib/api'
import { useAccess } from '../lib/access'
import { useI18n } from '../i18n'
import { LANGUAGES } from '../lib/format'
import { GnaniKeyCard } from '../components/GnaniKeyCard'
import { microphoneProblem, startSpeakerphone, type SpeakerphoneSession } from '../lib/speakerphone'
import { Button, Card, ErrorNote } from '../components/ui'
import { CallView } from './CallDetailPage'

type Status = 'idle' | 'starting' | 'listening' | 'speaking'

export function SpeakerphonePage() {
  const { t, lang } = useI18n()
  const [users, setUsers] = useState<ProtectedUser[]>([])
  const [userPhone, setUserPhone] = useState('')
  const [mode, setMode] = useState<'guardian' | 'screen'>('guardian')
  const [status, setStatus] = useState<Status>('idle')
  const [level, setLevel] = useState(0)
  const [callId, setCallId] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const session = useRef<SpeakerphoneSession | null>(null)
  const { access } = useAccess()
  const demoVisitor = access.demo && !access.family
  const [language, setLanguage] = useState(lang)
  const [hasKey, setHasKey] = useState(!!getGnaniKey())
  const [micSilent, setMicSilent] = useState(false)
  const micProblem = microphoneProblem()

  useEffect(() => {
    if (!demoVisitor) {
      api.users().then((list) => {
        setUsers(list)
        if (list[0]) setUserPhone(list[0].phone)
      }).catch(() => {})
    }
    return () => session.current?.stop() // stop the mic when leaving the page
  }, [demoVisitor])

  // Keep the screen on while listening: it may be a spare phone lying near the call.
  useEffect(() => {
    if (status !== 'listening' && status !== 'speaking') return
    let lock: { release: () => Promise<void> } | null = null
    const nav = navigator as Navigator & { wakeLock?: { request: (t: 'screen') => Promise<{ release: () => Promise<void> }> } }
    const acquire = () => nav.wakeLock?.request('screen').then((l) => { lock = l }).catch(() => {})
    // The browser drops the lock whenever the page is hidden.
    const onVisible = () => { if (document.visibilityState === 'visible') void acquire() }
    void acquire()
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      document.removeEventListener('visibilitychange', onVisible)
      void lock?.release().catch(() => {})
    }
  }, [status === 'idle'])

  const start = async () => {
    setError(null)
    setCallId(null)
    setStatus('starting')
    try {
      session.current = await startSpeakerphone({
        userPhone,
        mode,
        language: demoVisitor ? language : undefined,
        gnaniKey: demoVisitor ? getGnaniKey() : undefined,
        onStarted: (id) => {
          setCallId(id)
          setStatus('listening')
        },
        onSpeaking: (speaking) => setStatus(speaking ? 'speaking' : 'listening'),
        onLevel: setLevel,
        onMicSilent: setMicSilent,
        onError: (message) => setError(t(message)),
        onClosed: () => {
          session.current = null
          setStatus('idle')
          setLevel(0)
          setMicSilent(false)
        },
      })
    } catch (e) {
      const err = e as DOMException
      setError(err.name === 'NotAllowedError'
        ? t('Microphone permission was denied. Allow it in the browser address bar and try again.')
        : err.name === 'NotReadableError'
          ? t('The microphone is in use by something else, often the phone call itself. Use a second device for the browser.')
          : t('Could not start the microphone: {error}', { error: err.message }))
      setStatus('idle')
    }
  }

  const stop = () => session.current?.stop()
  const running = status !== 'idle'

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl text-slate-100">{t('Speakerphone mode')}</h1>
        <p className="mt-1 text-sm text-slate-400">
          {t('Put the phone call on speaker next to this device. Chaukanna listens through the microphone and, if it is a scam, speaks the warning aloud so everyone on the call hears it.')}
        </p>
      </div>

      <ErrorNote message={error ?? (micProblem && t(micProblem))} />

      {micSilent && (
        <p role="alert" className="rounded-lg border border-amber-500/50 bg-amber-500/15 px-3 py-3 text-sm font-medium text-amber-100">
          {t("The microphone isn't hearing anything, so this call isn't being protected. If the call is on this same phone, the call has probably taken the microphone: open this page on a second phone or laptop and keep the call on speaker next to it.")}
        </p>
      )}

      {demoVisitor && <GnaniKeyCard onChange={setHasKey} need="The microphone needs your own Gnani key, so the demo costs nobody else anything." />}

      <Card>
        <div className="flex flex-wrap items-center gap-6">
          <button
            onClick={running ? stop : start}
            disabled={!running && (!!micProblem || (demoVisitor && !hasKey))}
            aria-label={running ? t('Stop listening') : t('Start listening')}
            className="mx-auto rounded-full disabled:opacity-50 sm:mx-0"
          >
            <MicOrb status={status} level={level} />
          </button>

          <div className="min-w-0 flex-1 basis-64 space-y-3">
            <p className="text-lg font-medium text-slate-100">{t(STATUS_TEXT[status])}</p>

            <div className="flex flex-wrap gap-4 text-sm">
              {demoVisitor ? (
                <label className="flex items-center gap-2 text-slate-300">
                  {t('Language')}
                  <select
                    value={language}
                    disabled={running}
                    onChange={(e) => setLanguage(e.target.value)}
                    className="rounded-lg border border-slate-700 bg-slate-900 px-2 py-1.5"
                  >
                    {Object.entries(LANGUAGES).map(([code, name]) => <option key={code} value={code}>{name}</option>)}
                  </select>
                </label>
              ) : (
              <label className="flex items-center gap-2 text-slate-300">
                {t('Protecting')}
                <select
                  value={userPhone}
                  disabled={running}
                  onChange={(e) => setUserPhone(e.target.value)}
                  className="rounded-lg border border-slate-700 bg-slate-900 px-2 py-1.5"
                >
                  {users.length === 0 && <option value="">{t('Demo user')}</option>}
                  {users.map((u) => (
                    <option key={u.id} value={u.phone}>{u.name}</option>
                  ))}
                </select>
              </label>
              )}
              <label className="flex min-w-0 max-w-full items-center gap-2 text-slate-300">
                {t('Mode')}
                <select
                  value={mode}
                  disabled={running}
                  onChange={(e) => setMode(e.target.value as 'guardian' | 'screen')}
                  className="min-w-0 max-w-full flex-1 rounded-lg border border-slate-700 bg-slate-900 px-2 py-1.5"
                >
                  <option value="guardian">{t('Live protection: listen silently, warn if scam')}</option>
                  <option value="screen">{t('Call screening: Chaukanna talks to the caller')}</option>
                </select>
              </label>
            </div>

            {running ? (
              <Button large variant="danger" onClick={stop}>■ {t('Stop listening')}</Button>
            ) : (
              <Button large onClick={start} disabled={!!micProblem || (demoVisitor && !hasKey)}>▶ {t('Start listening')}</Button>
            )}
          </div>
        </div>
      </Card>

      <a href="tel:1930" className="block rounded-2xl border border-red-500/30 bg-red-500/10 px-5 py-4 text-center text-lg font-medium text-red-300 sm:hidden">
        {t('Fraud helpline: 1930')}
      </a>

      {callId != null && <CallView key={callId} callId={callId} elderScreen />}
    </div>
  )
}

const STATUS_TEXT: Record<Status, string> = {
  idle: 'Not listening',
  starting: 'Starting microphone…',
  listening: 'Listening to the call…',
  speaking: 'Speaking warning…',
}

/** A round mic indicator that grows with the input volume. */
function MicOrb({ status, level }: { status: Status; level: number }) {
  const active = status === 'listening'
  const speaking = status === 'speaking'
  const scale = active ? 1 + Math.min(level * 3, 0.6) : 1
  return (
    <div className="relative flex h-28 w-28 items-center justify-center">
      <div
        className={`absolute inset-0 rounded-full transition-transform duration-100 ${speaking ? 'bg-red-500/30 animate-pulse' : active ? 'bg-cyan-500/25' : 'bg-slate-800'}`}
        style={{ transform: `scale(${scale})` }}
      />
      <span className={`relative h-5 w-5 rounded-full ${speaking ? 'bg-red-500' : active ? 'bg-cyan-500' : 'bg-slate-600'}`} />
    </div>
  )
}
