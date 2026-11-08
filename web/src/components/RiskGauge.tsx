// Semicircle gauge showing the live scam risk.

import type { CallState } from '../lib/api'
import { STATE_STYLE } from '../lib/format'
import { useT } from '../i18n'

export function RiskGauge({ risk, state }: { risk: number; state: CallState }) {
  const t = useT()
  const pct = Math.max(0, Math.min(1, risk))
  const radius = 80
  const arcLength = Math.PI * radius // length of a half circle
  const color = STATE_STYLE[state]?.color ?? '#3f8a5f'

  return (
    <div className="flex flex-col items-center">
      <svg viewBox="0 0 200 115" className="w-full max-w-[240px]">
        {/* background track */}
        <path d="M 20 100 A 80 80 0 0 1 180 100" fill="none" stroke="var(--color-slate-800)" strokeWidth="10" strokeLinecap="round" />
        {/* risk arc */}
        <path
          d="M 20 100 A 80 80 0 0 1 180 100"
          fill="none"
          stroke={color}
          strokeWidth="10"
          strokeLinecap="round"
          strokeDasharray={arcLength}
          strokeDashoffset={arcLength * (1 - pct)}
          style={{ transition: 'stroke-dashoffset 600ms ease, stroke 300ms' }}
        />
        <text x="100" y="92" textAnchor="middle" className="fill-slate-100" style={{ font: '500 38px Fraunces, Georgia, serif' }}>
          {Math.round(pct * 100)}%
        </text>
      </svg>
      <p className="-mt-1 text-xs text-slate-500">{t('Scam risk')}</p>
    </div>
  )
}
