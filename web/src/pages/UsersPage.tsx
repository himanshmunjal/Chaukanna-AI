// Manage protected people (e.g. parents) and who gets alerted.

import { useEffect, useState, type FormEvent } from 'react'
import { api, type ProtectedUser } from '../lib/api'
import { LANGUAGES } from '../lib/format'
import { Button, Card, ErrorNote } from '../components/ui'
import { ScamNumbersCard } from '../components/ScamNumbersCard'
import { FamilySetup } from '../components/FamilySetup'

const EMPTY = { name: '', phone: '', language: 'hi', family_name: '', family_phone: '' }

export function UsersPage() {
  const [users, setUsers] = useState<ProtectedUser[]>([])
  const [form, setForm] = useState(EMPTY)
  const [consent, setConsent] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = () => api.users().then(setUsers).catch((e) => setError(e.message))
  useEffect(() => {
    load()
  }, [])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await api.createUser({ ...form, consent })
      setForm(EMPTY)
      setConsent(false)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const remove = async (id: number) => {
    await api.deleteUser(id).catch((e) => setError(e.message))
    load()
  }

  const field = (key: keyof typeof EMPTY, label: string, placeholder = '') => (
    <label className="block text-sm">
      <span className="text-slate-400">{label}</span>
      <input
        value={form[key]}
        onChange={(e) => setForm({ ...form, [key]: e.target.value })}
        placeholder={placeholder}
        className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
      />
    </label>
  )

  return (
    <div className="space-y-6">
      <div className="grid gap-6 lg:grid-cols-3">
        <Card title="Protected people" className="lg:col-span-2">
          {users.length === 0 ? (
            <p className="text-sm text-slate-500">Nobody yet. Add a parent or grandparent on the right.</p>
          ) : (
            <ul className="divide-y divide-slate-800">
              {users.map((u) => (
                <li key={u.id} className="py-3">
                  <div className="flex flex-wrap items-center gap-4">
                  <div className="flex-1">
                    <p className="font-medium text-slate-100">{u.name} <span className="text-sm text-slate-500">{u.phone}</span></p>
                    <p className="text-xs text-slate-500">
                      Speaks {LANGUAGES[u.language] ?? u.language} · alerts go to {u.family_name || 'family'} {u.family_phone || '(not set)'}
                    </p>
                  </div>
                  <Button variant="danger" onClick={() => remove(u.id)}>Remove</Button>
                  </div>
                  <FamilySetup user={u} onChange={load} />
                </li>
              ))}
            </ul>
          )}
        </Card>
  
        <Card title="Add a person">
          <form onSubmit={submit} className="space-y-3">
            {field('name', 'Name', 'Kamla Devi')}
            {field('phone', 'Phone', '+91 98xxx xxxxx')}
            <label className="block text-sm">
              <span className="text-slate-400">Language</span>
              <select
                value={form.language}
                onChange={(e) => setForm({ ...form, language: e.target.value })}
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              >
                {Object.entries(LANGUAGES).map(([code, name]) => (
                  <option key={code} value={code}>{name}</option>
                ))}
              </select>
            </label>
            {field('family_name', 'Family member', 'Rahul (son)')}
            {field('family_phone', 'Family phone (gets alerts)', '+91 98xxx xxxxx')}
            <label className="flex items-start gap-2 text-sm text-slate-300">
              <input type="checkbox" checked={consent} onChange={(e) => setConsent(e.target.checked)} className="mt-1" />
              <span>This person has agreed that Chaukanna may listen to calls they choose to protect, and alert their family.</span>
            </label>
            <ErrorNote message={error} />
            <Button type="submit" disabled={!form.name || !form.phone || !consent}>Add</Button>
          </form>
        </Card>
      </div>
      <ScamNumbersCard />
    </div>
  )
}
