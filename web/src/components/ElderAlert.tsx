// The full red screen an elderly person sees: one instruction, as large as it can be,
// in their own language. The fine print belongs on the family's dashboard, not here.

import { useEffect, useState } from 'react'
import { HANG_UP } from '../lib/format'
import { useT } from '../i18n'

export function ElderAlert({ text, language }: { text: string; language: string }) {
  const [closed, setClosed] = useState(false)
  const t = useT()
  // Felt as well as seen, for someone holding the phone or not looking at it.
  useEffect(() => { navigator.vibrate?.([500, 200, 500, 200, 500]) }, [])
  if (closed) return null
  const lang = language.split('-')[0]
  return (
    <div role="alertdialog" aria-label={t('Scam warning')} className="fixed inset-0 z-50 flex flex-col items-center justify-center gap-6 overflow-y-auto bg-red-700 p-6 text-center text-white">
      <span aria-hidden className="text-7xl">🛑</span>
      <p className="text-5xl font-extrabold leading-tight sm:text-7xl">{HANG_UP[lang] ?? HANG_UP.hi}</p>
      <p className="max-w-3xl text-xl leading-relaxed sm:text-2xl">{text}</p>
      <a href="tel:1930" className="rounded-2xl bg-white px-10 py-5 text-4xl font-extrabold text-red-700">📞 1930</a>
      <button onClick={() => setClosed(true)} aria-label={t('Close')} className="mt-2 min-h-12 rounded-lg border border-white/60 px-6 py-3 text-lg">✕</button>
    </div>
  )
}
