import PhasePill, { type Phase as PillPhase } from '../ui/PhasePill'
import type { RunPhase } from '../../types/progress'

const PILL: Record<RunPhase, { phase: PillPhase; label: string }> = {
  sitemap: { phase: 'analyzing', label: 'Reading sitemap' },
  crawling: { phase: 'crawling', label: 'Crawling' },
  analyzing: { phase: 'analyzing', label: 'Analyzing' },
}

/**
 * The pill for a running crawl's current phase: a green pulsing dot while crawling, violet while
 * reading the sitemap or analyzing. Before the first snapshot arrives it reads as plain Crawling.
 */
export default function RunPhasePill({ phase, live = false }: { phase?: RunPhase; live?: boolean }) {
  const p = phase ? PILL[phase] : PILL.crawling
  return (
    <PhasePill phase={p.phase} live={live}>
      {p.label}
    </PhasePill>
  )
}
