// Live transcript with auto-scroll. Lines that triggered a red flag are highlighted.

import { useEffect, useRef } from 'react'
import type { RedFlag, TranscriptLine } from '../lib/api'
import { formatSeconds } from '../lib/format'
import { useT } from '../i18n'

const SPEAKER_STYLE: Record<string, { name: string; className: string }> = {
  call: { name: 'Call', className: 'text-slate-200' },
  caller: { name: 'Caller', className: 'text-slate-200' },
  chaukanna: { name: 'Chaukanna', className: 'text-cyan-300 font-medium' },
  family: { name: 'Family', className: 'text-emerald-300 font-medium' },
}

export function TranscriptView({ lines, partial, flags }: { lines: TranscriptLine[]; partial: string; flags: RedFlag[] }) {
  const bottom = useRef<HTMLDivElement>(null)
  const t = useT()
  const flaggedTimes = new Set(flags.map((f) => f.t))

  useEffect(() => {
    bottom.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [lines.length, partial])

  return (
    <div className="max-h-[60vh] space-y-2 overflow-y-auto pr-2">
      {lines.length === 0 && !partial && <p className="text-sm text-slate-500">{t('Waiting for speech…')}</p>}

      {lines.map((line, i) => {
        const speaker = SPEAKER_STYLE[line.speaker] ?? SPEAKER_STYLE.call
        const flagged = line.speaker !== 'chaukanna' && flaggedTimes.has(line.t)
        return (
          <div
            key={line.id ?? `${line.t}-${i}`}
            className={`flex gap-3 rounded-lg px-3 py-2 ${flagged ? 'bg-red-500/10 ring-1 ring-red-500/30' : ''} ${line.speaker === 'chaukanna' ? 'bg-cyan-500/10' : ''}`}
          >
            <span className="w-10 shrink-0 pt-0.5 font-mono text-xs text-slate-400">{formatSeconds(line.t)}</span>
            <div>
              <span className="text-[11px] text-slate-400">{t(speaker.name)}</span>
              <p className={`leading-relaxed ${speaker.className}`}>{line.text}</p>
            </div>
          </div>
        )
      })}

      {partial && (
        <div className="flex gap-3 px-3 py-2 opacity-60">
          <span className="w-10 shrink-0 font-mono text-xs text-slate-400">…</span>
          <p className="italic text-slate-300">{partial}</p>
        </div>
      )}
      <div ref={bottom} />
    </div>
  )
}
