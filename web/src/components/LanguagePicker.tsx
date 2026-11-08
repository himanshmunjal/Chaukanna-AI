// Switches the interface language. Each language is listed in its own script, so a
// person who can't read the current screen can still find theirs.

import { useI18n } from '../i18n'
import { LANGUAGES } from '../lib/format'

export function LanguagePicker() {
  const { lang, setLang, t } = useI18n()
  return (
    <select
      value={lang}
      onChange={(e) => setLang(e.target.value)}
      aria-label={t('Language')}
      className="min-h-9 max-w-24 sm:max-w-28 rounded-full border border-slate-700 bg-slate-900 px-2.5 py-1 text-sm text-slate-200"
    >
      {Object.entries(LANGUAGES).map(([code, name]) => <option key={code} value={code}>{name}</option>)}
    </select>
  )
}

const ENGLISH_NAMES: Record<string, string> = {
  hi: 'Hindi', en: 'English', ta: 'Tamil', te: 'Telugu', kn: 'Kannada',
  ml: 'Malayalam', bn: 'Bengali', mr: 'Marathi', gu: 'Gujarati', pa: 'Punjabi',
}

/** Asked once, before anything else, in a form someone can use without reading English. */
export function FirstRunLanguage() {
  const { lang, setLang } = useI18n()
  return (
    <div role="dialog" aria-label="Choose your language" className="min-h-screen bg-slate-950 px-5 py-10 text-slate-200">
      <div className="mx-auto max-w-md">
        <img src="/icon.svg" alt="" className="h-20 w-20" />
        <h1 className="mt-5 text-2xl text-slate-100">भाषा चुनें · மொழி · ভাষা · భాష · Language</h1>
        <div className="mt-6 space-y-2">
          {Object.entries(LANGUAGES).map(([code, name]) => (
            <button
              key={code}
              onClick={() => setLang(code)}
              className={`flex min-h-14 w-full items-center justify-between rounded-2xl border bg-slate-900 px-5 py-3 text-left text-lg font-medium text-slate-100 hover:bg-slate-800 ${code === lang ? 'border-cyan-500' : 'border-slate-800'}`}
            >
              {name}
              {code !== 'en' && <span className="text-sm font-normal text-slate-400">{ENGLISH_NAMES[code]}</span>}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
