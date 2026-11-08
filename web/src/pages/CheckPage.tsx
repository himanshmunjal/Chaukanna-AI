// A second opinion without any setup: paste what the caller said, or upload a recording.

import { useState } from 'react'
import { api, type CheckResult } from '../lib/api'
import { useAccess } from '../lib/access'
import { useI18n } from '../i18n'
import { GnaniKeyCard } from '../components/GnaniKeyCard'
import { LANGUAGES, flagInfo } from '../lib/format'
import { Button, Card, ErrorNote } from '../components/ui'

const VERDICT = {
  scam: { title: 'This looks like a scam', box: 'border-red-500/30 bg-red-500/10 text-red-100' },
  caution: { title: 'Be careful: this is doubtful', box: 'border-amber-500/30 bg-amber-500/10 text-amber-100' },
  safe: { title: 'No red flags found', box: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-100' },
}

export function CheckPage() {
  const { t, lang } = useI18n()
  const [text, setText] = useState('')
  const [language, setLanguage] = useState(lang)
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<CheckResult | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const { access } = useAccess()
  const demoVisitor = access.demo && !access.family

  const check = async () => {
    setBusy(true)
    setError(null)
    setResult(null)
    try {
      setResult(file ? await api.checkFile(file, language) : await api.checkText(text, language))
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl text-slate-100">{t('Check a call')}</h1>
        <p className="mt-1 text-sm text-slate-400">{t('Got a strange call or voice note? Paste the words or upload the recording. Nothing is saved.')}</p>
      </div>

      {demoVisitor && <GnaniKeyCard need="Pasted text works without a key. Uploading a recording needs your own Gnani key." />}

      <Card>
        <label className="block text-sm text-slate-400" htmlFor="check-text">{t('What did the caller say?')}</label>
        <textarea
          id="check-text"
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={5}
          disabled={!!file}
          placeholder="e.g. Aapka KYC expire ho gaya hai, abhi OTP bata dijiye warna account band ho jayega"
          className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 p-3 text-sm text-slate-200 placeholder:text-slate-600 disabled:opacity-50"
        />
        <div className="mt-3 flex flex-wrap items-center gap-4 text-sm text-slate-300">
          <label className="flex min-w-0 flex-wrap items-center gap-2">
            {t('or upload a recording')}
            <input type="file" accept="audio/*,.wav,.mp3,.m4a,.ogg,.flac,.aac" onChange={(e) => setFile(e.target.files?.[0] ?? null)} className="max-w-56 text-xs" />
          </label>
          <label className="flex items-center gap-2">
            {t('Language')}
            <select value={language} onChange={(e) => setLanguage(e.target.value)} className="rounded-lg border border-slate-700 bg-slate-900 px-2 py-1.5">
              {Object.entries(LANGUAGES).map(([code, name]) => <option key={code} value={code}>{name}</option>)}
            </select>
          </label>
          <Button onClick={check} disabled={busy || (!file && !text.trim())}>{busy ? t('Checking…') : t('Check')}</Button>
        </div>
        <p className="mt-2 text-xs text-slate-500">{t('WAV files can be any length. Other formats: up to 30 seconds.')}</p>
      </Card>

      <ErrorNote message={error} />

      {result && (
        <div className="space-y-4" aria-live="polite">
          <div className={`rounded-2xl border p-5 ${VERDICT[result.verdict].box}`}>
            <p className="text-xl font-medium">{t(VERDICT[result.verdict].title)}</p>
            {result.verdict === 'scam' && <p className="mt-1 text-sm opacity-80">{t(result.scam_type_label)} · {t('Scam risk')} {Math.round(result.risk * 100)}%</p>}
            {result.advice && <p className="mt-3 text-lg leading-relaxed">{result.advice}</p>}
            {result.verdict === 'safe' && <p className="mt-2 text-sm opacity-80">{t('This is not a guarantee. If anyone asks for an OTP, money or an app, hang up.')}</p>}
            {result.links && (
              <div className="mt-4 flex flex-wrap gap-2">
                {result.links.map((l) => <a key={l.url} href={l.url} target="_blank" rel="noreferrer" className="rounded-lg bg-white/90 px-3 py-2 text-sm font-medium text-red-800">{t(l.label)}</a>)}
              </div>
            )}
          </div>
          {result.flags.length > 0 && (
            <Card title={t('Red flags ({n})', { n: result.flags.length })}>
              <ul className="space-y-3 text-sm">
                {result.flags.map((f) => (
                  <li key={f.type}>
                    <p className="font-medium text-slate-100">{t(flagInfo(f.type).label)}</p>
                    <p className="italic text-slate-400">“{f.quote}”</p>
                  </li>
                ))}
              </ul>
            </Card>
          )}
          {file && <Card title={t('What Prisma heard')}><p className="text-sm text-slate-300">{result.transcript}</p></Card>}
        </div>
      )}
    </div>
  )
}
