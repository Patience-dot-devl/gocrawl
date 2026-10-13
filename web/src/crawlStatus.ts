import type { Phase } from './components/ui/PhasePill'
import type { CrawlStatus } from './types/crawls'

// How a crawl job's status reads in a PhasePill. Actions keep one name through the flow:
// "Stop crawl" leads to "Crawl stopped".
export const STATUS_PILL: Record<CrawlStatus, { phase: Phase; label: string }> = {
  running: { phase: 'crawling', label: 'Crawling' },
  done: { phase: 'done', label: 'Finished' },
  canceled: { phase: 'stopped', label: 'Crawl stopped' },
  error: { phase: 'failed', label: 'Failed' },
}
