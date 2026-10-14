import type { Phase as PillPhase } from '../ui/PhasePill'
import type { RunPhase } from '../../types/progress'

const PILL: Record<RunPhase, { phase: PillPhase; label: string }> = {
  sitemap: { phase: 'analyzing', label: 'Reading sitemap' },
  crawling: { phase: 'crawling', label: 'Crawling' },
  analyzing: { phase: 'analyzing', label: 'Analyzing' },
}

/**
 * How a running crawl's phase reads in a PhasePill: a green pulsing dot while crawling, violet
 * while reading the sitemap or analyzing. Before the first snapshot arrives it reads as Crawling.
 * CrawlReport feeds this (or the finished status) to one PhasePill, so the live region persists
 * across the change from Crawling to Crawl stopped.
 */
export function runPill(phase?: RunPhase): { phase: PillPhase; label: string } {
  return phase ? PILL[phase] : PILL.crawling
}
