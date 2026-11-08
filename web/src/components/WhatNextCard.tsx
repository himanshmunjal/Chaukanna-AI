// After a scam call: what to do, a complaint draft to check and file, and the family's verdict.

import { useEffect, useState } from 'react'
import { api, type Call, type CallReport } from '../lib/api'
import { Button, Card, ErrorNote } from './ui'
import { useT } from '../i18n'

const OUTCOMES = [
  { id: 'confirmed_scam', label: 'It was a scam' },
  { id: 'money_lost', label: 'Money was lost' },
  { id: 'false_alarm', label: 'False alarm' },
]

export function WhatNextCard({ call, onChange }: { call: Call; onChange: (outcome: string) => void }) {
  const t = useT()
  const [report, setReport] = useState<CallReport | null>(null)
  const [copied, setCopied] = useState(false)
  const [showDraft, setShowDraft] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    api.callReport(call.id).then(setReport).catch((e) => setError(e.message))
  }, [call.id, call.state])

  const choose = async (outcome: string) => {
    const next = call.outcome === outcome ? '' : outcome
    try {
      await api.setOutcome(call.id, next)
      onChange(next)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const copy = async () => {
    if (!report) return
    await navigator.clipboard.writeText(report.draft).then(() => setCopied(true)).catch(() => setShowDraft(true))
  }

  return (
    <Card title={t('What to do now')}>
      <ErrorNote message={error} />
      <div className="flex flex-wrap gap-2" role="group" aria-label="What happened on this call">
        {OUTCOMES.map((o) => (
          <button
            key={o.id}
            aria-pressed={call.outcome === o.id}
            onClick={() => choose(o.id)}
            className={`rounded-lg border px-2.5 py-1.5 text-xs font-medium ${call.outcome === o.id ? 'border-cyan-400 bg-cyan-500/20 text-cyan-200' : 'border-slate-700 text-slate-300 hover:bg-slate-800'}`}
          >
            {t(o.label)}
          </button>
        ))}
      </div>
      {call.outcome === 'false_alarm' && <p className="mt-2 text-xs text-slate-400">{t('Noted. The number was released if Chaukanna had blocked it.')}</p>}

      {report && call.outcome !== 'false_alarm' && (
        <>
          {call.outcome === 'money_lost' && (
            <ol className="mt-4 list-decimal space-y-2 rounded-lg bg-red-500/10 p-3 pl-7 text-sm text-red-100">
              {report.money_lost_steps.map((step) => <li key={step}>{t(step)}</li>)}
            </ol>
          )}
          <ul className="mt-4 list-disc space-y-2 pl-5 text-sm text-slate-300">
            {report.steps.map((step) => <li key={step}>{t(step)}</li>)}
          </ul>
          <div className="mt-4 space-y-2">
            {report.links.map((l) => (
              <a key={l.url} href={l.url} target="_blank" rel="noreferrer" className="block rounded-lg border border-slate-700 px-3 py-2 text-sm font-medium text-cyan-300 hover:bg-slate-800">
                {t(l.label)} →
              </a>
            ))}
          </div>
          <div className="mt-4 flex gap-2">
            <Button variant="ghost" onClick={copy}>{copied ? `✓ ${t('Copied')}` : t('Copy complaint draft')}</Button>
            <Button variant="ghost" onClick={() => setShowDraft((v) => !v)}>{showDraft ? t('Hide') : t('View')}</Button>
          </div>
          {showDraft && <pre className="mt-3 max-h-72 overflow-auto whitespace-pre-wrap rounded-lg bg-slate-950 p-3 text-xs text-slate-300">{report.draft}</pre>}
          <p className="mt-2 text-xs text-slate-500">{t('Chaukanna files nothing itself. Check the draft, then submit it yourself.')}</p>
        </>
      )}
    </Card>
  )
}
