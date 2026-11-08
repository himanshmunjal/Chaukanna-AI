// A demo visitor's own Gnani key. It stays in this browser tab, and the server uses it
// for each request without storing it.

import { useState, type FormEvent } from 'react'
import { getGnaniKey, setGnaniKey } from '../lib/api'
import { useAccess } from '../lib/access'
import { Button } from './ui'

export function GnaniKeyCard({ need, onChange }: { need: string; onChange?: (hasKey: boolean) => void }) {
  const [saved, setSaved] = useState(!!getGnaniKey())
  const [value, setValue] = useState('')
  const { access } = useAccess()
  const [ownKey, setOwnKey] = useState(false)

  const save = (e: FormEvent) => {
    e.preventDefault()
    if (!value.trim()) return
    setGnaniKey(value.trim())
    setValue('')
    setSaved(true)
    onChange?.(true)
  }
  const clear = () => {
    setGnaniKey('')
    setSaved(false)
    onChange?.(false)
  }

  return (
    <section className="rounded-2xl border border-cyan-500/30 bg-cyan-500/5 p-4 text-sm">
      {!saved && access.shared_key && !ownKey ? (
        <div className="flex flex-wrap items-center gap-3">
          <span className="text-emerald-300">✓ Speech works here on the demo's shared Gnani key, up to a daily limit. No key needed.</span>
          <Button variant="ghost" onClick={() => setOwnKey(true)}>Use my own key</Button>
        </div>
      ) : saved ? (
        <div className="flex flex-wrap items-center gap-3">
          <span className="text-emerald-300">✓ Using your Gnani key in this tab.</span>
          <Button variant="ghost" onClick={clear}>Remove key</Button>
        </div>
      ) : (
        <form onSubmit={save}>
          <p className="text-slate-300">{need}</p>
          <div className="mt-2 flex flex-wrap gap-2">
            <input
              type="password"
              autoComplete="off"
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder="Your Gnani API key"
              aria-label="Your Gnani API key"
              className="min-w-0 flex-1 rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            />
            <Button type="submit" variant="ghost" disabled={!value.trim()}>Use my key</Button>
          </div>
          <p className="mt-2 text-xs text-slate-500">
            Kept only in this browser tab. It goes to this server to reach Gnani, and is not stored or logged. Get one at app.gnani.ai.
          </p>
        </form>
      )}
    </section>
  )
}
