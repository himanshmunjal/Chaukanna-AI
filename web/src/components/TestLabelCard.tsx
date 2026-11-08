// Marks what a test call really was, for scoring. Label by what the caller intended, not
// by what Chaukanna decided, or the score means nothing.

import { useState } from 'react'
import { api, type Call } from '../lib/api'
import { SCAM_TYPES } from '../lib/format'
import { Card, ErrorNote } from './ui'

export function TestLabelCard({ call, onChange }: { call: Call; onChange: (label: Call['test_label'], category: string) => void }) {
  const [error, setError] = useState<string | null>(null)

  const save = async (label: Call['test_label'], category: string) => {
    try {
      await api.labelCall(call.id, label, category)
      setError(null)
      onChange(label, label === 'scam' ? category : '')
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const button = (active: boolean) =>
    `rounded-lg border px-3 py-1.5 text-xs font-medium ${active ? 'border-cyan-400 bg-cyan-500/20 text-cyan-200' : 'border-slate-700 text-slate-300 hover:bg-slate-800'}`

  return (
    <Card title="Test label">
      <p className="text-xs text-slate-400">What did your friend intend? Ignore what Chaukanna decided.</p>
      <div className="mt-3 flex flex-wrap gap-2">
        <button aria-pressed={call.test_label === 'scam'} className={button(call.test_label === 'scam')} onClick={() => save(call.test_label === 'scam' ? '' : 'scam', call.test_category || 'general')}>Scam</button>
        <button aria-pressed={call.test_label === 'legit'} className={button(call.test_label === 'legit')} onClick={() => save(call.test_label === 'legit' ? '' : 'legit', '')}>Normal call</button>
      </div>
      {call.test_label === 'scam' && (
        <label className="mt-3 block text-sm">
          <span className="text-slate-400">Scam type</span>
          <select
            value={call.test_category || 'general'}
            onChange={(e) => save('scam', e.target.value)}
            className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-2 py-1.5 text-slate-100"
          >
            {Object.entries(SCAM_TYPES).map(([id, name]) => <option key={id} value={id}>{id === 'general' ? 'Other' : name}</option>)}
          </select>
        </label>
      )}
      <div className="mt-2"><ErrorNote message={error} /></div>
    </Card>
  )
}
