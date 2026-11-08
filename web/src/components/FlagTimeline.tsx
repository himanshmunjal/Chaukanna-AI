// Vertical timeline of red flags: when each scam tactic appeared and the exact words used.

import type { RedFlag } from '../lib/api'
import { flagInfo, formatSeconds } from '../lib/format'
import { useT } from '../i18n'

export function FlagTimeline({ flags }: { flags: RedFlag[] }) {
  const t = useT()
  if (flags.length === 0) {
    return <p className="text-sm text-slate-500">{t('No red flags yet.')}</p>
  }
  const sorted = [...flags].sort((a, b) => a.t - b.t)

  return (
    <ol className="relative space-y-4 border-l border-slate-700 pl-6">
      {sorted.map((f) => {
        const info = flagInfo(f.type)
        return (
          <li key={f.type} className="relative">
            <span className="absolute -left-[29px] top-1.5 h-2 w-2 rounded-full bg-red-500 ring-4 ring-slate-900" />
            <div className="flex items-center gap-2 text-sm">
              <span className="font-mono text-xs text-slate-500">{formatSeconds(f.t)}</span>
              <span className="font-medium text-slate-100">
                {t(info.label)}
              </span>
              {f.source === 'evon' && (
                <span className="rounded-full bg-violet-500/10 px-2 py-0.5 text-[10px] font-medium text-violet-300">EVON</span>
              )}
            </div>
            <p className="mt-1 text-sm text-slate-400">“{f.quote}”</p>
          </li>
        )
      })}
    </ol>
  )
}
