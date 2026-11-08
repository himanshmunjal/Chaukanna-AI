// Offers to put Chaukanna on the home screen. Android and desktop Chrome show their own
// install dialog; iPhones have none, so they get the steps instead.

import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { useT } from '../i18n'
import { Button } from './ui'

interface InstallEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

const DISMISSED_KEY = 'chaukanna_install_dismissed'

function wasDismissed(): boolean {
  try { return localStorage.getItem(DISMISSED_KEY) === '1' } catch { return false }
}

function isInstalled(): boolean {
  return window.matchMedia('(display-mode: standalone)').matches || (navigator as Navigator & { standalone?: boolean }).standalone === true
}

function isIPhone(): boolean {
  return /iphone|ipad|ipod/i.test(navigator.userAgent)
}

export function InstallCard() {
  const [event, setEvent] = useState<InstallEvent | null>(null)
  const [hidden, setHidden] = useState(() => wasDismissed() || isInstalled())
  const t = useT()
  const onHome = useLocation().pathname === '/' // never in the way of protecting a call

  useEffect(() => {
    const onPrompt = (e: Event) => {
      e.preventDefault() // we show our own card, at a moment that isn't mid-call
      setEvent(e as InstallEvent)
    }
    const onInstalled = () => setHidden(true)
    window.addEventListener('beforeinstallprompt', onPrompt)
    window.addEventListener('appinstalled', onInstalled)
    return () => {
      window.removeEventListener('beforeinstallprompt', onPrompt)
      window.removeEventListener('appinstalled', onInstalled)
    }
  }, [])

  if (hidden || !onHome || (!event && !isIPhone())) return null

  const dismiss = () => {
    setHidden(true)
    try { localStorage.setItem(DISMISSED_KEY, '1') } catch { /* it just shows again next time */ }
  }
  const install = async () => {
    if (!event) return
    await event.prompt()
    const { outcome } = await event.userChoice
    setEvent(null) // the browser allows each prompt only once
    if (outcome === 'accepted') setHidden(true)
  }

  return (
    <section className="mb-8 flex flex-wrap items-center gap-3 rounded-2xl border border-cyan-500/40 bg-cyan-500/10 p-4">
      <img src="/icon-192.png" alt="" className="h-12 w-12 rounded-xl" />
      <div className="min-w-0 flex-1 basis-48">
        <p className="font-semibold text-slate-100">{t('Install Chaukanna')}</p>
        <p className="text-sm text-slate-300">{event ? t('One tap from your home screen, with no app store.') : t('On iPhone: tap Share, then Add to Home Screen.')}</p>
      </div>
      <div className="flex gap-2">
        {event && <Button onClick={install}>{t('Install')}</Button>}
        <Button variant="ghost" onClick={dismiss}>{t('Not now')}</Button>
      </div>
    </section>
  )
}
