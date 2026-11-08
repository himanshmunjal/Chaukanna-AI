// The family dashboard's login.

import { useState, type FormEvent } from 'react'
import { api } from '../lib/api'
import { Button, ErrorNote } from '../components/ui'
import { LanguagePicker } from '../components/LanguagePicker'
import { useT } from '../i18n'

export function LoginPage({ onLoggedIn, onCancel }: { onLoggedIn: () => void; onCancel?: () => void }) {
  const t = useT()
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await api.login(password)
      onLoggedIn()
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950 p-4 text-slate-200">
      <form onSubmit={submit} className="w-full max-w-sm space-y-5 rounded-3xl border border-slate-800 bg-slate-900 p-8 shadow-[0_8px_30px_rgb(25_24_21/0.05)]">
        <div>
          <div className="flex items-center justify-between gap-3">
            <h1 className="text-3xl text-slate-100">Chaukanna</h1>
            <LanguagePicker />
          </div>
          <p className="mt-1 text-sm text-slate-400">{t('Sign in to your family dashboard.')}</p>
        </div>
        <label className="block text-sm">
          <span className="text-slate-400">{t('Family password')}</span>
          <input
            type="password"
            autoFocus
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1.5 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2.5 text-slate-100"
          />
        </label>
        <ErrorNote message={error} />
        <div className="flex gap-2">
          <Button type="submit" disabled={busy || !password}>{busy ? t('Logging in…') : t('Log in')}</Button>
          {onCancel && <Button variant="ghost" onClick={onCancel}>{t('Back to the demo')}</Button>}
        </div>
      </form>
    </div>
  )
}
