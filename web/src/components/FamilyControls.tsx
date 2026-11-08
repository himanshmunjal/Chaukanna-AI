// Scammers win by keeping the victim alone on the line. These let a family member
// watching the dashboard step into the call themselves.

import { useState, type FormEvent } from 'react'
import { api } from '../lib/api'
import { useAccess } from '../lib/access'
import { useT } from '../i18n'
import { Button, ErrorNote } from './ui'

export function FamilyControls({ callId, language, safe }: { callId: number; language: string; safe: boolean }) {
  const [text, setText] = useState('')
  const [confirmSafe, setConfirmSafe] = useState(false)
  const [note, setNote] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const { access } = useAccess()
  const t = useT()
  const demoVisitor = access.demo && !access.family

  const run = async (action: () => Promise<string>) => {
    setError(null)
    setNote(null)
    try {
      setNote(await action())
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const speak = (e: FormEvent) => {
    e.preventDefault()
    const message = text.trim()
    if (!message) return
    run(async () => {
      const { played } = await api.speakIntoCall(callId, message)
      setText('')
      return played ? t('Spoken on the call.') : demoVisitor ? 'Add your Gnani key to hear your own words spoken.' : t('Could not play it on the call. Please phone them.')
    })
  }

  return (
    <section className="rounded-2xl border border-cyan-500/40 bg-cyan-500/5 p-4">
      <h2 className="text-sm font-medium text-cyan-300">{t('Step in')}</h2>
      <div className="mt-3 flex flex-wrap items-center gap-3">
        <Button variant="danger" onClick={() => run(async () => { await api.warnNow(callId); return t('Warning sent to the call.') })}>{t('Warn now')}</Button>
        <form onSubmit={speak} className="flex min-w-64 flex-1 gap-2">
          <input
            value={text}
            maxLength={200}
            onChange={(e) => setText(e.target.value)}
            placeholder={language.startsWith('en') ? 'Mum, please hang up, I am calling you' : 'माँ, फ़ोन काट दीजिए, मैं कॉल कर रहा हूँ'}
            aria-label={t('Message to speak on the call')}
            className="flex-1 rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100"
          />
          <Button type="submit" variant="ghost" disabled={!text.trim()}>{t('Speak')}</Button>
        </form>
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-3 border-t border-slate-800 pt-3">
        {safe ? (
          <Button variant="ghost" onClick={() => run(async () => { await api.markCallSafe(callId, false); return t('Chaukanna is protecting this call again.') })}>{t('Start warning again')}</Button>
        ) : confirmSafe ? (
          <>
            <span className="text-sm text-slate-300">{t('Are you sure? Chaukanna will stop warning on this call.')}</span>
            <Button variant="ghost" onClick={() => { setConfirmSafe(false); run(async () => { await api.markCallSafe(callId, true); return t('Marked safe.') }) }}>{t("Yes, it's safe")}</Button>
            <Button variant="ghost" onClick={() => setConfirmSafe(false)}>{t('Cancel')}</Button>
          </>
        ) : (
          <Button variant="ghost" onClick={() => setConfirmSafe(true)}>{t('This call is safe')}</Button>
        )}
        <span className="text-xs text-slate-500">{t('For the family, after checking who is calling. Never press this because a caller asked you to.')}</span>
      </div>
      {note && <p className="mt-2 text-sm text-emerald-300">{note}</p>}
      <div className="mt-2"><ErrorNote message={error} /></div>
      <p className="mt-2 text-xs text-slate-500">{t('The caller can hear these too.')}</p>
    </section>
  )
}
