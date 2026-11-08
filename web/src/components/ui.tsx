// Small presentational components used across pages.

import type { ReactNode } from 'react'
import type { CallState } from '../lib/api'
import { STATE_STYLE } from '../lib/format'
import { useT } from '../i18n'

export function Card({ title, children, className = '' }: { title?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-[0_1px_2px_rgb(25_24_21/0.03)] ${className}`}>
      {title && <h2 className="mb-5 text-xl text-slate-100">{title}</h2>}
      {children}
    </section>
  )
}

export function StateBadge({ state }: { state: CallState }) {
  const style = STATE_STYLE[state] ?? STATE_STYLE.listening
  const t = useT()
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${style.badge}`}>
      <span className="h-1.5 w-1.5 rounded-full" style={{ background: style.color }} />
      {t(style.label)}
    </span>
  )
}

export function LiveDot() {
  return (
    <span className="relative flex h-2.5 w-2.5">
      <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-red-400 opacity-60" />
      <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-red-500" />
    </span>
  )
}

/** A thin horizontal bar showing risk 0..1. */
export function RiskBar({ risk, state }: { risk: number; state: CallState }) {
  return (
    <div className="flex items-center gap-2">
      <div className="h-1 w-24 overflow-hidden rounded-full bg-slate-800">
        <div className="h-full rounded-full transition-all" style={{ width: `${Math.round(risk * 100)}%`, background: STATE_STYLE[state]?.color }} />
      </div>
      <span className="w-9 text-xs tabular-nums text-slate-400">{Math.round(risk * 100)}%</span>
    </div>
  )
}

export function Button({
  children, onClick, variant = 'primary', disabled, type = 'button', large = false,
}: {
  children: ReactNode
  onClick?: () => void
  variant?: 'primary' | 'ghost' | 'danger'
  disabled?: boolean
  type?: 'button' | 'submit'
  /** The main action of a screen: fills the width on a phone. */
  large?: boolean
}) {
  const styles = {
    primary: 'bg-cyan-500 text-slate-950 hover:bg-cyan-400',
    ghost: 'border border-slate-700 bg-slate-900 text-slate-200 hover:bg-slate-800',
    danger: 'border border-red-500/30 bg-slate-900 text-red-300 hover:bg-red-500/10',
  }
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className={`min-h-11 rounded-full px-4 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-0 ${large ? 'w-full py-4 text-lg sm:w-auto sm:py-2 sm:text-sm' : ''} ${styles[variant]}`}
    >
      {children}
    </button>
  )
}

export function ErrorNote({ message }: { message: string | null }) {
  if (!message) return null
  return <p className="rounded-xl border border-red-500/25 bg-red-500/10 px-4 py-3 text-sm text-red-300">{message}</p>
}
