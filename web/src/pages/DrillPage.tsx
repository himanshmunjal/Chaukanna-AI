// Scam Drill: a practice call. Timbre plays the scammer, the person answers out loud,
// and each reply gets feedback. Getting it wrong here is the point.

import { useEffect, useRef, useState, type FormEvent } from 'react'
import { api, type DrillScenario, type DrillTurn } from '../lib/api'
import { useAccess } from '../lib/access'
import { useT } from '../i18n'
import { GnaniKeyCard } from '../components/GnaniKeyCard'
import { playWav, recordWav, type WavRecording } from '../lib/wavRecorder'
import { Button, Card, ErrorNote } from '../components/ui'

interface Exchange {
  line: string
  heard?: string
  verdict?: DrillTurn['verdict']
  lesson?: string
}

type Phase = 'choose' | 'speaking' | 'your-turn' | 'recording' | 'judging' | 'done'

const VERDICT = {
  safe: { icon: '', label: 'Safe answer', color: 'text-emerald-300' },
  risky: { icon: '', label: 'Risky: this is what the scammer wants', color: 'text-red-300' },
  unclear: { icon: '', label: 'Not sure what you meant', color: 'text-amber-300' },
}

export function DrillPage() {
  const t = useT()
  const [scenarios, setScenarios] = useState<DrillScenario[]>([])
  const [canListen, setCanListen] = useState(false)
  const [active, setActive] = useState<DrillScenario | null>(null)
  const [phase, setPhase] = useState<Phase>('choose')
  const [step, setStep] = useState(0)
  const [log, setLog] = useState<Exchange[]>([])
  const [hungUp, setHungUp] = useState(false)
  const [typed, setTyped] = useState('')
  const [error, setError] = useState<string | null>(null)
  const recording = useRef<WavRecording | null>(null)
  const { access } = useAccess()
  const demoVisitor = access.demo && !access.family

  const loadScenarios = () => api.drillScenarios().then((r) => {
    setScenarios(r.scenarios)
    setCanListen(r.listen)
  }).catch((e) => setError(e.message))

  useEffect(() => {
    loadScenarios()
    return () => recording.current?.cancel()
  }, [])

  /** Shows what came back and plays the scammer's next line. */
  const apply = async (turn: DrillTurn, base: Exchange[]) => {
    const log2 = [...base]
    if (turn.verdict && log2.length) {
      log2[log2.length - 1] = { ...log2[log2.length - 1], heard: turn.heard, verdict: turn.verdict, lesson: turn.lesson }
    }
    if (turn.done) {
      setLog(log2)
      setPhase('done')
      return
    }
    log2.push({ line: turn.line ?? '' })
    setLog(log2)
    setStep(turn.step ?? 0)
    setPhase('speaking')
    if (turn.audio) await playWav(turn.audio)
    setPhase('your-turn')
  }

  const begin = async (s: DrillScenario) => {
    setActive(s)
    setLog([])
    setHungUp(false)
    setError(null)
    setPhase('judging')
    try {
      await apply(await api.drillSay(s.id, -1, ''), [])
    } catch (e) {
      setError((e as Error).message)
      setPhase('choose')
    }
  }

  const startRecording = async () => {
    setError(null)
    try {
      recording.current = await recordWav()
      setPhase('recording')
    } catch {
      setError(t('Could not use the microphone. You can type your answer instead.'))
    }
  }

  const stopRecording = async () => {
    const take = recording.current
    recording.current = null
    if (!take || !active) return
    setPhase('judging')
    try {
      await apply(await api.drillSpeak(active.id, step, take.stop()), log)
    } catch (e) {
      setError((e as Error).message)
      setPhase('your-turn')
    }
  }

  const sendTyped = async (e: FormEvent) => {
    e.preventDefault()
    if (!active || !typed.trim()) return
    const reply = typed
    setTyped('')
    setPhase('judging')
    try {
      await apply(await api.drillSay(active.id, step, reply), log)
    } catch (err) {
      setError((err as Error).message)
      setPhase('your-turn')
    }
  }

  const answered = log.filter((x) => x.verdict)
  const safe = answered.filter((x) => x.verdict === 'safe').length
  const risky = answered.filter((x) => x.verdict === 'risky').length

  if (phase === 'choose') {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl text-slate-100">{t('Scam Drill')}</h1>
          <p className="mt-1 text-sm text-slate-400">
            {t('A practice call, for parents and grandparents to try with family beside them. A pretend scammer talks; answer out loud, as you really would. Nothing here is real, and nothing is saved.')}
          </p>
        </div>
        <ErrorNote message={error} />
        {demoVisitor && <GnaniKeyCard onChange={loadScenarios} need="You can type your answers without a key. Answering out loud needs your own Gnani key." />}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {scenarios.map((s) => (
            <button key={s.id} onClick={() => begin(s)} className="rounded-2xl border border-slate-800 bg-slate-900 p-5 text-left hover:border-cyan-500/60">
              <p className="text-lg font-medium text-slate-100">{s.title}</p>
              <p className="mt-1 text-sm text-cyan-400">{s.language === 'hi' ? 'हिंदी' : 'English'} · ▶ {s.language === 'hi' ? 'शुरू करें' : 'Start'}</p>
            </button>
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-2xl space-y-5">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl text-slate-100">{active?.title}</h1>
        <Button variant="ghost" onClick={() => { recording.current?.cancel(); recording.current = null; setPhase('choose') }}>{t('Exit')}</Button>
      </div>
      <ErrorNote message={error} />

      <div className="space-y-3" aria-live="polite">
        {log.map((x, i) => (
          <div key={i} className="space-y-2">
            <p className="rounded-2xl rounded-tl-sm bg-slate-800 p-4 text-lg leading-relaxed text-slate-100">{x.line}</p>
            {x.heard !== undefined && <p className="ml-8 rounded-2xl rounded-tr-sm bg-cyan-500/15 p-3 text-slate-100">{x.heard || t('(nothing heard)')}</p>}
            {x.verdict && (
              <div className="ml-8">
                <p className={`text-sm font-medium ${VERDICT[x.verdict].color}`}>{t(VERDICT[x.verdict].label)}</p>
                <p className="text-sm text-slate-300">{x.lesson}</p>
              </div>
            )}
          </div>
        ))}
      </div>

      {phase === 'done' ? (
        <Card>
          <p className="text-2xl font-medium text-slate-100">
            {hungUp ? t('You hung up. Perfect.') : t('{safe} of {total} safe answers', { safe, total: answered.length })}
          </p>
          <p className="mt-2 text-slate-300">
            {hungUp || risky === 0
              ? t('Hanging up early is always right. You owe a stranger no explanation.')
              : t('It is safe to get this wrong here, and that is the point. Try again: next time, hang up sooner.')}
          </p>
          <p className="mt-2 text-sm text-slate-500">{t('The real helpline for fraud is 1930.')}</p>
          <div className="mt-4"><Button onClick={() => setPhase('choose')}>{t('Practise another')}</Button></div>
        </Card>
      ) : (
        <Card>
          <div className="flex flex-wrap items-center gap-3">
            {phase === 'recording' ? (
              <Button variant="danger" onClick={stopRecording}>■ {t('Done speaking')}</Button>
            ) : (
              <Button onClick={startRecording} disabled={phase !== 'your-turn' || !canListen}>{t('Answer out loud')}</Button>
            )}
            <Button variant="danger" disabled={phase === 'judging'} onClick={() => { recording.current?.cancel(); recording.current = null; setHungUp(true); setPhase('done') }}>{t('Hang up')}</Button>
            {phase === 'speaking' && <span className="text-sm text-slate-400">{t('Listen…')}</span>}
            {phase === 'judging' && <span className="text-sm text-slate-400">…</span>}
          </div>
          <form onSubmit={sendTyped} className="mt-3 flex gap-2">
            <input
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              disabled={phase !== 'your-turn'}
              placeholder={t('or type your answer')}
              aria-label={t('Type your answer')}
              className="flex-1 rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 disabled:opacity-50"
            />
            <Button type="submit" variant="ghost" disabled={phase !== 'your-turn' || !typed.trim()}>{t('Send')}</Button>
          </form>
        </Card>
      )}
    </div>
  )
}
