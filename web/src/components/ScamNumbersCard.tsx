// Numbers caught scamming. They are blocked (screening) or flagged (live
// protection) the moment they call any protected person again.

import { useEffect, useState, type FormEvent } from 'react'
import { api, type ScamNumber } from '../lib/api'
import { timeAgo } from '../lib/format'
import { useLiveEvents } from '../lib/live'
import { Button, Card, ErrorNote } from './ui'

export function ScamNumbersCard() {
  const [numbers, setNumbers] = useState<ScamNumber[]>([])
  const [phone, setPhone] = useState('')
  const [error, setError] = useState<string | null>(null)

  const load = () => api.scamNumbers().then(setNumbers).catch((e) => setError(e.message))
  useEffect(() => {
    load()
  }, [])
  // A call that ends as a scam may add a number.
  useLiveEvents((event) => {
    if (event.type === 'call_ended') window.setTimeout(load, 1000)
  })

  const report = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await api.reportScamNumber(phone, 'Reported by family')
      setPhone('')
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const unblock = async (id: number) => {
    await api.unblockScamNumber(id).catch((e) => setError(e.message))
    load()
  }

  return (
    <Card title={`Known scam numbers (${numbers.length})`}>
      <p className="mb-4 text-sm text-slate-400">
        Numbers caught scamming are remembered. If one calls again, Chaukanna stops it straight away.
      </p>

      {numbers.length > 0 && (
        <ul className="mb-4 divide-y divide-slate-800">
          {numbers.map((n) => (
            <li key={n.id} className="flex flex-wrap items-center gap-3 py-2.5">
              <div className="min-w-0 flex-1">
                <p className="font-mono text-slate-100">{n.phone}</p>
                <p className="truncate text-xs text-slate-500">
                  {n.source === 'manual' ? 'reported by family' : 'caught by Chaukanna'} · {n.reports} report{n.reports === 1 ? '' : 's'}
                  {n.blocked > 0 && ` · blocked ${n.blocked}×`} · {timeAgo(n.updated_at)}
                  {n.last_reason && ` · ${n.last_reason}`}
                </p>
              </div>
              <Button variant="ghost" onClick={() => unblock(n.id)}>Unblock</Button>
            </li>
          ))}
        </ul>
      )}

      <form onSubmit={report} className="flex gap-2">
        <input
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
          placeholder="Report a number, e.g. 98765 43210"
          className="flex-1 rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100"
        />
        <Button type="submit" variant="danger" disabled={!phone.trim()}>Report</Button>
      </form>
      <div className="mt-2">
        <ErrorNote message={error} />
      </div>
    </Card>
  )
}
