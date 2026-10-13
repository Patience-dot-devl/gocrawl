// Mirrors internal/webserver progressView (runner.Progress + tape fields) and the sitemap
// preview response. Keep in sync with the Go side by hand.
import type { CrawlListItem, JobView } from './crawls'

export type Phase = 'sitemap' | 'crawling' | 'analyzing'
export type PauseReason = 'user' | 'rate_limited'

export interface ProgressView {
  fetched: number
  queued: number
  current_url: string
  final_url?: string
  status: number
  duration_ms: number
  rate: number
  throttled: number
  paused: boolean
  pause_reason?: PauseReason
  retry_after_seconds?: number
  phase: Phase
  sitemap_found: boolean
  sitemap_total: number
  sitemap_fetched: number
  off_sitemap: number
  in_sitemap: boolean
  /** Tape characters from tape_from on, one per fetched page: '2' '3' '4' '5' or 'x'. */
  tape: string
  /** Parallel to tape: '1' when the page is not in the sitemap, else '0'. Empty without a sitemap. */
  tape_off: string
  tape_from: number
  max_pages: number
  active_ms: number
}

export type JobWithProgress = JobView & { progress?: ProgressView }

export type CrawlListItemWithProgress = CrawlListItem & {
  status_classes?: Record<string, number>
  sitemap_total?: number
  sitemap_fetched?: number
  /** At most 160 tape characters, worst status per run (crawler.TapePreview). */
  tape_preview?: string
}

export interface SitemapPreviewRequest {
  url: string
  include?: string[]
  exclude?: string[]
  allow_subdomains?: boolean
}

export interface SitemapPreview {
  found: boolean
  total: number
  sources: string[]
  via_robots: boolean
}
