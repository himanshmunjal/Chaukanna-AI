// Interface translations. The English sentence is the key, so anything without a
// translation simply shows in English. Each language loads only when it's chosen.
// `npm run check:i18n` checks that every language has every string.

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

type Strings = Record<string, string>
type Vars = Record<string, string | number>
export type Translate = (key: string, vars?: Vars) => string

const LOADERS: Record<string, () => Promise<{ default: Strings }>> = {
  hi: () => import('./hi.json'), ta: () => import('./ta.json'), te: () => import('./te.json'),
  kn: () => import('./kn.json'), ml: () => import('./ml.json'), bn: () => import('./bn.json'),
  mr: () => import('./mr.json'), gu: () => import('./gu.json'), pa: () => import('./pa.json'),
}
const SUPPORTED = ['en', ...Object.keys(LOADERS)]
const STORAGE_KEY = 'chaukanna_lang' // public/offline.html reads this too

function firstLanguage(): string {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved && SUPPORTED.includes(saved)) return saved
  } catch { /* storage can be blocked */ }
  for (const tag of navigator.languages ?? []) {
    const base = tag.split('-')[0].toLowerCase()
    if (SUPPORTED.includes(base)) return base
  }
  return 'en'
}

function hasSavedLanguage(): boolean {
  try { return !!localStorage.getItem(STORAGE_KEY) } catch { return false }
}

function fill(text: string, vars?: Vars): string {
  if (!vars) return text
  return text.replace(/\{(\w+)\}/g, (whole, name) => (name in vars ? String(vars[name]) : whole))
}

interface I18nValue {
  lang: string
  setLang: (lang: string) => void
  /** Nobody has chosen a language on this device yet. */
  firstRun: boolean
  t: Translate
}

const I18nContext = createContext<I18nValue>({ lang: 'en', setLang: () => {}, firstRun: false, t: fill })

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState(firstLanguage)
  const [strings, setStrings] = useState<Strings>({})
  const [firstRun, setFirstRun] = useState(() => !hasSavedLanguage())

  useEffect(() => {
    document.documentElement.lang = lang
    let current = true
    const load = LOADERS[lang]
    if (!load) {
      setStrings({})
      return
    }
    load().then((m) => { if (current) setStrings(m.default) }).catch(() => { if (current) setStrings({}) })
    return () => { current = false }
  }, [lang])

  const setLang = useCallback((next: string) => {
    setLangState(next)
    setFirstRun(false)
    try { localStorage.setItem(STORAGE_KEY, next) } catch { /* the choice just won't be remembered */ }
  }, [])

  const t = useCallback<Translate>((key, vars) => fill(strings[key] ?? key, vars), [strings])
  const value = useMemo(() => ({ lang, setLang, firstRun, t }), [lang, setLang, firstRun, t])
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18nValue {
  return useContext(I18nContext)
}

export function useT(): Translate {
  return useContext(I18nContext).t
}
