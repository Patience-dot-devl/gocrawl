import { useMemo } from 'react'
import { buildTicks, MAX_COMPACT_TICKS, MAX_TICKS } from './tapeModel'
import './Tape.css'

export interface TapeProps {
  // One status-class character per fetched page, in fetch order.
  tape: string
  // '1' where that page is not in the sitemap; drawn as a notch near the tick's base.
  tapeOff?: string
  // Sitemap pages not fetched (yet): slate outline ticks after the fetched ones.
  notFetched?: number
  // Sitemap pages beyond the page budget: amber outline ticks at the end.
  pastBudget?: number
  // Text alternative; build it with describeTape so every drawn count is also in words.
  summary: string
  // One short row, for lists (Crawls, Compare pickers).
  compact?: boolean
  // New fetched ticks drop in (off under prefers-reduced-motion).
  live?: boolean
}

// The crawl tape: one 3px tick per page. Height carries the status as well as color, so the
// tape still reads without color: short 2xx, mid 3xx, tall 4xx, tall 5xx with a light cap.
// Fetched ticks and outline ticks have different keys, so when a page is fetched its tick
// remounts and the drop-in animation plays only for it.
//
// The full-size tape scrolls (max-height), so it takes focus: keyboard users can then scroll
// it with the arrow keys (WCAG 2.1.1). The compact tape never scrolls and is not focusable.
export default function Tape({ tape, tapeOff = '', notFetched = 0, pastBudget = 0, summary, compact = false, live = false }: TapeProps) {
  const ticks = useMemo(
    () => buildTicks(tape, tapeOff, notFetched, pastBudget, compact ? MAX_COMPACT_TICKS : MAX_TICKS),
    [tape, tapeOff, notFetched, pastBudget, compact],
  )
  const classes = `tape${compact ? ' tape-compact' : ''}${live ? ' tape-live' : ''}`
  return (
    <div className={classes} role="img" aria-label={summary} tabIndex={compact ? undefined : 0}>
      {ticks.map((t, i) => {
        const fetched = t.kind !== 'todo' && t.kind !== 'cut'
        return (
          <i
            key={fetched ? `f${i}` : `o${i}`}
            className={`tick tick-${t.kind}${t.off ? ' tick-off' : ''}${fetched ? ' tick-fetched' : ''}`}
          />
        )
      })}
    </div>
  )
}
