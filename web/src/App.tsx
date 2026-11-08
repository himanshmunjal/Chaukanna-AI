import { Suspense, lazy, useEffect, useState, type ReactNode } from 'react'
import { BrowserRouter, NavLink, Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { AccessProvider, useAccess } from './lib/access'
import { useTheme } from './lib/theme'
import { LiveProvider, useLiveConnected } from './lib/live'
import { I18nProvider, useI18n, useT } from './i18n'
import { FirstRunLanguage, LanguagePicker } from './components/LanguagePicker'
import { InstallCard } from './components/InstallCard'
import { guideSeen } from './lib/guide'
import { CallsPage } from './pages/CallsPage'

// Other pages load when first opened, so the home screen appears sooner on a phone.
const CallDetailPage = lazy(() => import('./pages/CallDetailPage').then((m) => ({ default: m.CallDetailPage })))
const SimulatorPage = lazy(() => import('./pages/SimulatorPage').then((m) => ({ default: m.SimulatorPage })))
const SpeakerphonePage = lazy(() => import('./pages/SpeakerphonePage').then((m) => ({ default: m.SpeakerphonePage })))
const UsersPage = lazy(() => import('./pages/UsersPage').then((m) => ({ default: m.UsersPage })))
const CheckPage = lazy(() => import('./pages/CheckPage').then((m) => ({ default: m.CheckPage })))
const DrillPage = lazy(() => import('./pages/DrillPage').then((m) => ({ default: m.DrillPage })))
const GuidePage = lazy(() => import('./pages/GuidePage').then((m) => ({ default: m.GuidePage })))

export default function App() {
  return (
    <I18nProvider>
      <FirstRunGate>
        <AccessProvider>
          <Dashboard />
        </AccessProvider>
      </FirstRunGate>
    </I18nProvider>
  )
}

function FirstRunGate({ children }: { children: ReactNode }) {
  const { firstRun } = useI18n()
  // A family member opening an alert link during a live call must not be stopped by a question.
  const openingACall = window.location.pathname.startsWith('/calls/')
  return firstRun && !openingACall ? <FirstRunLanguage /> : <>{children}</>
}

function Dashboard() {
  const { access } = useAccess()
  const t = useT()
  return (
    <LiveProvider>
      <BrowserRouter>
        <div className="min-h-screen bg-slate-950 text-slate-200">
          <Header />
          <OfflineBanner />
          {access.demo && !access.family && (
            <p className="border-b border-amber-500/20 bg-amber-500/10 px-4 py-2 text-center text-sm text-amber-200">
              {t('Public demo. Your calls are visible only to you and deleted after an hour. No real alerts are sent.')}
            </p>
          )}
          <FirstVisitGuide />
          <main className="app-main mx-auto max-w-5xl px-5 py-6 sm:px-6 sm:py-10">
            <InstallCard />
            <Suspense fallback={<div className="min-h-[60vh]" />}>
            <Routes>
              <Route path="/" element={<CallsPage />} />
              <Route path="/calls/:id" element={<CallDetailPage />} />
              <Route path="/speakerphone" element={<SpeakerphonePage />} />
              <Route path="/simulator" element={<SimulatorPage />} />
              <Route path="/users" element={access.family ? <UsersPage /> : <Navigate to="/" replace />} />
              <Route path="/check" element={<CheckPage />} />
              <Route path="/drill" element={<DrillPage />} />
              <Route path="/guide" element={<GuidePage />} />
            </Routes>
            </Suspense>
          </main>
          <BottomNav />
        </div>
      </BrowserRouter>
    </LiveProvider>
  )
}

/** Opens the guide on a first visit to the home page. Links to a call are left alone,
 *  because a family member opening an alert during a live call must not be delayed. */
function FirstVisitGuide() {
  const navigate = useNavigate()
  const { pathname } = useLocation()
  useEffect(() => {
    if (pathname === '/' && !guideSeen()) navigate('/guide', { replace: true })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps
  return null
}

function Header() {
  const connected = useLiveConnected()
  const { access } = useAccess()
  const { theme, toggle } = useTheme()
  const t = useT()
  const link = ({ isActive }: { isActive: boolean }) =>
    `rounded-full px-3 py-1.5 text-sm transition-colors ${isActive ? 'bg-slate-800 font-medium text-slate-100' : 'text-slate-400 hover:text-slate-100'}`

  return (
    <header className="app-header sticky top-0 z-10 border-b border-slate-800/80 bg-slate-950/85 backdrop-blur-md">
      <div className="mx-auto flex max-w-5xl flex-wrap items-center gap-x-3 gap-y-2 px-4 py-2.5 sm:gap-x-4 sm:px-6 sm:py-3.5">
        <NavLink to="/" className="flex items-center gap-2.5">
          <span aria-hidden className="grid h-6 w-6 place-items-center rounded-lg bg-cyan-500"><span className="h-2 w-2 rounded-full bg-slate-950" /></span>
          <span className="font-display text-xl font-medium tracking-tight text-slate-100">Chaukanna</span>
        </NavLink>
        <nav className="hidden flex-wrap gap-1 sm:flex">
          <NavLink to="/" end className={link}>{t('Calls')}</NavLink>
          <NavLink to="/speakerphone" className={link}>{t('Speakerphone')}</NavLink>
          <NavLink to="/check" className={link}>{t('Check a call')}</NavLink>
          <NavLink to="/drill" className={link}>{t('Scam Drill')}</NavLink>
          <NavLink to="/simulator" className={link}>{t('Simulator')}</NavLink>
          {access.family && <NavLink to="/users" className={link}>{t('People')}</NavLink>}
          <NavLink to="/guide" className={link}>{t('Guide')}</NavLink>
        </nav>
        <AccountButton className="ml-auto hidden text-xs text-slate-400 underline-offset-4 hover:text-slate-100 hover:underline sm:inline" />
        <span className="ml-auto sm:ml-0"><LanguagePicker /></span>
        <span title={connected ? t('Live') : t('Connecting…')} className={`flex items-center gap-2 text-xs ${connected ? 'text-emerald-300' : 'text-slate-500'}`}>
          <span className={`h-2 w-2 rounded-full ${connected ? 'bg-emerald-400' : 'bg-slate-600'}`} />
          <span className="hidden sm:inline">{connected ? t('Live') : t('Connecting…')}</span>
        </span>
        <button
          onClick={toggle}
          aria-label={theme === 'dark' ? t('Switch to light theme') : t('Switch to dark theme')}
          title={theme === 'dark' ? t('Switch to light theme') : t('Switch to dark theme')}
          className="grid h-8 w-8 place-items-center rounded-full border border-slate-700 text-slate-400 transition-colors hover:text-slate-100"
        >
          {theme === 'dark' ? (
            <svg viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round"><circle cx="12" cy="12" r="4" /><path d="M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M5.6 18.4 7 17M17 7l1.4-1.4" /></svg>
          ) : (
            <svg viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M20 14.5A8 8 0 0 1 9.5 4 8 8 0 1 0 20 14.5Z" /></svg>
          )}
        </button>
      </div>
    </header>
  )
}

function AccountButton({ className = '' }: { className?: string }) {
  const { access, logout, goToLogin } = useAccess()
  const t = useT()
  if (access.logged_in) return <button onClick={logout} className={className}>{t('Log out')}</button>
  if (access.demo && !access.family && access.can_login) return <button onClick={goToLogin} className={className}>{t('Family login')}</button>
  return null
}

/** Says plainly when the phone has no internet, because nothing can protect a call then. */
function OfflineBanner() {
  const [online, setOnline] = useState(navigator.onLine)
  const t = useT()
  useEffect(() => {
    const update = () => setOnline(navigator.onLine)
    window.addEventListener('online', update)
    window.addEventListener('offline', update)
    return () => {
      window.removeEventListener('online', update)
      window.removeEventListener('offline', update)
    }
  }, [])
  if (online) return null
  return (
    <p role="alert" className="bg-red-700 px-4 py-3 text-center font-medium text-white">
      {t("No internet. Chaukanna can't protect calls right now. If anyone asks for an OTP or money, hang up and call 1930.")}
    </p>
  )
}

const NAV_ICONS: Record<string, ReactNode> = {
  calls: <path d="M4 6h16M4 12h16M4 18h10" />,
  protect: <path d="M12 3l8 3v6c0 4.5-3.2 7.7-8 9-4.8-1.3-8-4.5-8-9V6l8-3z" />,
  check: <><circle cx="11" cy="11" r="6.5" /><path d="M16 16l4.5 4.5" /></>,
  practice: <><circle cx="12" cy="12" r="8.5" /><circle cx="12" cy="12" r="4" /></>,
  more: <><circle cx="5.5" cy="12" r="1" /><circle cx="12" cy="12" r="1" /><circle cx="18.5" cy="12" r="1" /></>,
}

function NavIcon({ name }: { name: string }) {
  return (
    <svg aria-hidden viewBox="0 0 24 24" className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      {NAV_ICONS[name]}
    </svg>
  )
}

/** Thumb-reach navigation on a phone. Larger screens use the header instead. */
function BottomNav() {
  const { access } = useAccess()
  const t = useT()
  const [moreOpen, setMoreOpen] = useState(false)
  const location = useLocation()
  useEffect(() => setMoreOpen(false), [location.pathname])

  const tab = ({ isActive }: { isActive: boolean }) =>
    `flex min-h-14 min-w-0 flex-1 flex-col items-center justify-center gap-0.5 text-[11px] font-medium ${isActive ? 'text-cyan-400' : 'text-slate-400'}`
  const row = 'block w-full rounded-xl px-4 py-3 text-left text-base text-slate-200 hover:bg-slate-800'
  const inMore = location.pathname === '/simulator' || location.pathname === '/users' || location.pathname === '/guide'

  return (
    <nav className="app-bottom-nav fixed inset-x-0 bottom-0 z-40 border-t border-slate-800 bg-slate-900 sm:hidden">
      {moreOpen && (
        <div className="space-y-1 border-b border-slate-800 p-2">
          <NavLink to="/simulator" className={row}>{t('Simulator')}</NavLink>
          {access.family && <NavLink to="/users" className={row}>{t('People')}</NavLink>}
          <NavLink to="/guide" className={row}>{t('Guide')}</NavLink>
          <AccountButton className={row} />
        </div>
      )}
      <div className="flex">
        <NavLink to="/" end className={tab}><NavIcon name="calls" />{t('Calls')}</NavLink>
        <NavLink to="/speakerphone" className={tab}><NavIcon name="protect" />{t('Protect')}</NavLink>
        <NavLink to="/check" className={tab}><NavIcon name="check" />{t('Check')}</NavLink>
        <NavLink to="/drill" className={tab}><NavIcon name="practice" />{t('Practice')}</NavLink>
        <button aria-expanded={moreOpen} onClick={() => setMoreOpen((open) => !open)} className={tab({ isActive: moreOpen || inMore })}>
          <NavIcon name="more" />{t('More')}
        </button>
      </div>
    </nav>
  )
}
