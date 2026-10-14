import type { ReactNode } from 'react'
import './PhasePill.css'

// The state pill next to a crawl's heading. crawling: a green dot that pulses (static under
// prefers-reduced-motion). paused: a yellow dot. analyzing: a violet dot. done, stopped,
// failed: the finished states, with a green, slate, or red dot. live makes it a polite live
// region (role="status"), so a screen reader announces the change from Crawling to Crawl
// stopped. Only the pill in a crawl's own header sets it; pills in a list stay quiet.
export type Phase = 'crawling' | 'paused' | 'analyzing' | 'done' | 'stopped' | 'failed'

export default function PhasePill({ phase, live = false, children }: { phase: Phase; live?: boolean; children: ReactNode }) {
  return (
    <span className={`phase phase-${phase}`} role={live ? 'status' : undefined}>
      {children}
    </span>
  )
}
