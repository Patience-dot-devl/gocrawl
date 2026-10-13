import type { ReactNode } from 'react'
import './PhasePill.css'

// The state pill next to a crawl's heading. crawling: a green dot that pulses (static under
// prefers-reduced-motion). paused: a yellow dot. analyzing: a violet dot. done, stopped,
// failed: the finished states, with a green, slate, or red dot.
export type Phase = 'crawling' | 'paused' | 'analyzing' | 'done' | 'stopped' | 'failed'

export default function PhasePill({ phase, children }: { phase: Phase; children: ReactNode }) {
  return <span className={`phase phase-${phase}`}>{children}</span>
}
