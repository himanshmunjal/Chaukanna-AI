// Replay a scripted call through the real pipeline (no phone needed).
// Great for development and for recording the demo video.

import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type Scenario } from '../lib/api'
import { useAccess } from '../lib/access'
import { LANGUAGES } from '../lib/format'
import { Button, Card, ErrorNote } from '../components/ui'

export function SimulatorPage() {
  const navigate = useNavigate()
  const [scenarios, setScenarios] = useState<Scenario[]>([])
  const [mode, setMode] = useState<'guardian' | 'screen'>('guardian')
  const [speed, setSpeed] = useState(1)
  const [callerNumber, setCallerNumber] = useState('')
  const [custom, setCustom] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const { access } = useAccess()
  const demoVisitor = access.demo && !access.family

  useEffect(() => {
    api.scenarios().then(setScenarios).catch((e) => setError(e.message))
  }, [])

  const run = async (body: Parameters<typeof api.simulate>[0]) => {
    setBusy(true)
    setError(null)
    try {
      const { call_id } = await api.simulate({ ...body, caller_number: demoVisitor ? undefined : callerNumber || undefined })
      navigate(`/calls/${call_id}`)
    } catch (e) {
      setError((e as Error).message)
      setBusy(false)
    }
  }

  // Custom script: one sentence per line, spoken 5 seconds apart.
  const runCustom = () => {
    const lines = custom
      .split('\n')
      .map((text) => text.trim())
      .filter(Boolean)
      .map((text, i) => ({ t: 1 + i * 5, speaker: 'call', text }))
    if (lines.length) run({ lines, mode, speed })
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl text-slate-100">Call simulator</h1>
        <p className="mt-1 text-sm text-slate-400">
          Replays a conversation through the real pipeline (pre-filter → Evon → Timbre → alerts), skipping only the phone line and Prisma.
        </p>
        {!demoVisitor && <p className="mt-1 text-sm text-slate-500">
          Tip: run a scam with a caller number, then run any scenario again with the <b>same number</b>: Chaukanna recognises it and blocks it at once.
        </p>}
      </div>

      <ErrorNote message={error} />

      <Card>
        <div className="flex flex-wrap items-center gap-6 text-sm">
          <label className="flex min-w-0 max-w-full items-center gap-2 text-slate-300">
            Mode
            <select value={mode} onChange={(e) => setMode(e.target.value as 'guardian' | 'screen')} className="min-w-0 max-w-full flex-1 rounded-lg border border-slate-700 bg-slate-900 px-2 py-1.5">
              <option value="guardian">Live protection (listen silently, warn if scam)</option>
              <option value="screen">Call screening (Chaukanna answers an unknown caller)</option>
            </select>
          </label>
          <label className="flex items-center gap-2 text-slate-300">
            Speed
            <select value={speed} onChange={(e) => setSpeed(Number(e.target.value))} className="rounded-lg border border-slate-700 bg-slate-900 px-2 py-1.5">
              <option value={1}>1× real time</option>
              <option value={2}>2×</option>
              <option value={4}>4×</option>
            </select>
          </label>
          {!demoVisitor && <label className="flex items-center gap-2 text-slate-300" title="Scam calls from a number are remembered; the next call from it is blocked">
            Caller number
            <input
              value={callerNumber}
              onChange={(e) => setCallerNumber(e.target.value)}
              placeholder="optional, e.g. 98765 43210"
              className="w-44 rounded-lg border border-slate-700 bg-slate-900 px-2 py-1.5"
            />
          </label>}
        </div>
      </Card>

      <div className="grid gap-4 md:grid-cols-2">
        {scenarios.map((s) => (
          <div key={s.id} className="flex flex-col rounded-2xl border border-slate-800 bg-slate-900 p-4">
            <div className="flex items-center gap-2">
              <span className={`rounded px-2 py-0.5 text-xs font-medium ${s.label === 'scam' ? 'bg-red-500/15 text-red-300' : 'bg-emerald-500/15 text-emerald-300'}`}>
                {s.label.toUpperCase()}
              </span>
              <span className="text-xs text-slate-500">{LANGUAGES[s.language] ?? s.language} · {s.lines.length} lines</span>
            </div>
            <h3 className="mt-2 font-medium text-slate-100">{s.title}</h3>
            <p className="mt-1 line-clamp-2 flex-1 text-sm italic text-slate-400">“{s.lines[0]?.text}”</p>
            <div className="mt-3">
              <Button disabled={busy} onClick={() => run({ scenario_id: s.id, mode, speed, language: s.language })}>▶ Run</Button>
            </div>
          </div>
        ))}
      </div>

      <Card title="Custom script">
        <textarea
          value={custom}
          onChange={(e) => setCustom(e.target.value)}
          rows={5}
          placeholder={'One sentence per line, e.g.\nMain customs office se bol raha hoon\nAapke parcel mein drugs mile hain\nAnyDesk download kijiye'}
          className="w-full rounded-xl border border-slate-700 bg-slate-950 p-3 text-sm text-slate-200 placeholder:text-slate-600"
        />
        <div className="mt-3">
          <Button disabled={busy || !custom.trim()} onClick={runCustom}>▶ Run custom script</Button>
        </div>
      </Card>
    </div>
  )
}
