// Per-person setup: consent, the family voice for warnings, and Telegram alerts.

import { useEffect, useRef, useState } from 'react'
import { api, type ProtectedUser } from '../lib/api'
import { playWav, recordWav, type WavRecording } from '../lib/wavRecorder'
import { Button, ErrorNote } from './ui'

// A sample that covers many sounds. Reading aloud gives cleaner audio than free speech.
const SCRIPT = 'नमस्ते, मैं अपने परिवार की सुरक्षा के लिए अपनी आवाज़ रिकॉर्ड कर रहा हूँ। अगर कोई फ़ोन पर पैसे, ओटीपी या कोई जानकारी माँगे, तो कुछ मत बताइए। फ़ोन काट दीजिए और मुझे कॉल कीजिए। Please stay calm, and never share your password with anyone.'
const MIN_SECONDS = 10
const MAX_SECONDS = 30

export function FamilySetup({ user, onChange }: { user: ProtectedUser; onChange: () => void }) {
  const [open, setOpen] = useState(false)
  const [owner, setOwner] = useState(user.family_name)
  const [agreed, setAgreed] = useState(false)
  const [seconds, setSeconds] = useState<number | null>(null)
  const [busy, setBusy] = useState(false)
  const [telegram, setTelegram] = useState<{ enabled: boolean; linked?: boolean; url?: string } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const recording = useRef<WavRecording | null>(null)
  const timer = useRef<number>()

  useEffect(() => {
    if (open) api.telegramLink(user.id).then(setTelegram).catch(() => {})
    return () => {
      window.clearInterval(timer.current)
      recording.current?.cancel()
    }
  }, [open, user.id])

  const start = async () => {
    setError(null)
    try {
      recording.current = await recordWav()
      setSeconds(0)
      const begun = Date.now()
      timer.current = window.setInterval(() => {
        const s = Math.floor((Date.now() - begun) / 1000)
        setSeconds(s)
        if (s >= MAX_SECONDS) void finish()
      }, 250)
    } catch {
      setError('Could not use the microphone. Allow it in the browser and try again.')
    }
  }

  const finish = async () => {
    window.clearInterval(timer.current)
    const take = recording.current
    recording.current = null
    if (!take) return
    const wav = take.stop()
    setSeconds(null)
    setBusy(true)
    try {
      const res = await api.enrollVoice(user.id, wav, owner.trim())
      onChange()
      if (res.sample_audio) await playWav(res.sample_audio)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const removeVoice = async () => {
    await api.deleteVoice(user.id).catch((e) => setError(e.message))
    onChange()
  }

  const badge = (ok: boolean | undefined, yes: string, no: string) => (
    <span className={`rounded px-2 py-0.5 ${ok ? 'bg-emerald-500/15 text-emerald-300' : 'bg-slate-800 text-slate-400'}`}>{ok ? '✓ ' + yes : no}</span>
  )

  return (
    <div className="mt-2">
      <div className="flex flex-wrap items-center gap-2 text-xs">
        {badge(!!user.consent_at, 'Consent recorded', 'No consent recorded')}
        {badge(user.has_voice, `Voice: ${user.voice_owner}`, 'Stock voice')}
        {badge(user.telegram_linked, 'Telegram linked', 'Telegram not linked')}
        <button onClick={() => setOpen(!open)} aria-expanded={open} className="text-cyan-400 underline">{open ? 'Close' : 'Set up'}</button>
      </div>

      {open && (
        <div className="mt-3 space-y-4 rounded-lg border border-slate-800 bg-slate-950/50 p-4 text-sm">
          <ErrorNote message={error} />

          {!user.consent_at && (
            <div>
              <p className="text-slate-300">Has {user.name} agreed to have calls they choose protected?</p>
              <div className="mt-2">
                <Button variant="ghost" onClick={() => api.recordConsent(user.id).then(onChange).catch((e) => setError(e.message))}>
                  Yes, record their consent
                </Button>
              </div>
            </div>
          )}

          <div>
            <h3 className="font-medium text-slate-100">Warn in a family member's voice</h3>
            <p className="mt-1 text-slate-400">A warning in a daughter's voice is harder to ignore than a stranger's. The warning still says it comes from Chaukanna AI.</p>
            {user.has_voice ? (
              <div className="mt-2 flex items-center gap-3">
                <span className="text-slate-300">Using {user.voice_owner}'s voice.</span>
                <Button variant="danger" onClick={removeVoice}>Delete voice</Button>
              </div>
            ) : (
              <div className="mt-2 space-y-3">
                <label className="block">
                  <span className="text-slate-400">Whose voice is this?</span>
                  <input value={owner} onChange={(e) => setOwner(e.target.value)} className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100" />
                </label>
                <label className="flex items-start gap-2 text-slate-300">
                  <input type="checkbox" checked={agreed} onChange={(e) => setAgreed(e.target.checked)} className="mt-1" />
                  <span>I am recording my own voice, and I agree to it being cloned by Gnani's service to speak Chaukanna's warnings. Only the voice model is kept, and I can delete it here.</span>
                </label>
                <p className="rounded-lg bg-slate-900 p-3 leading-relaxed text-slate-200">{SCRIPT}</p>
                {seconds === null ? (
                  <Button onClick={start} disabled={busy || !agreed || !owner.trim()}>{busy ? 'Creating the voice…' : '⏺ Record'}</Button>
                ) : (
                  <Button variant="danger" onClick={finish} disabled={seconds < MIN_SECONDS}>
                    ■ Stop ({seconds}s{seconds < MIN_SECONDS ? `, keep reading to ${MIN_SECONDS}s` : ''})
                  </Button>
                )}
              </div>
            )}
          </div>

          <div>
            <h3 className="font-medium text-slate-100">Telegram alerts (free)</h3>
            {telegram === null && <p className="mt-1 text-slate-500">Checking…</p>}
            {telegram && !telegram.enabled && <p className="mt-1 text-slate-400">Set TELEGRAM_BOT_TOKEN in backend/.env to turn this on.</p>}
            {telegram?.enabled && (telegram.linked
              ? <p className="mt-1 text-emerald-300">Linked.</p>
              : <p className="mt-1 text-slate-400">Send this private link to the family member, who taps Start: <a className="break-all text-cyan-400 underline" href={telegram.url} target="_blank" rel="noreferrer">{telegram.url}</a></p>)}
          </div>
        </div>
      )}
    </div>
  )
}
