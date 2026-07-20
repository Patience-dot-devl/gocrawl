// Mirrors internal/report.Report, internal/analyze.Issue, and the internal/webserver API
// response shapes. Keep in sync with the Go side by hand — there is no codegen for this yet.

export interface Issue {
  analyzer: string
  url: string
  severity: 'info' | 'warning' | 'error'
  code: string
  message: string
  data?: Record<string, unknown>
}

export interface Summary {
  by_severity: Record<string, number>
  by_analyzer: Record<string, number>
  pages_by_status: Record<string, number>
}

export interface Coverage {
  complete: boolean
  discovered_not_crawled?: number
  page_limit_reached?: boolean
  depth_limit_reached?: boolean
  interrupted?: boolean
  duration_limit_reached?: boolean
  max_pages?: number
  max_depth?: number
}

export interface Report {
  seed: string
  started_at: string
  finished_at: string
  pages_crawled: number
  summary: Summary
  issues: Issue[]
  notes?: string[]
  coverage?: Coverage
}

export type CrawlStatus = 'running' | 'done' | 'error' | 'canceled'

export interface JobView {
  id: string
  seed: string
  status: CrawlStatus
  error?: string
  started_at?: string
  finished_at?: string
  persisted: boolean
  report?: Report
}

export interface CrawlListItem {
  id: string
  seed: string
  status: CrawlStatus
  error?: string
  started_at?: string
  finished_at?: string
  pages_crawled?: number
  by_severity?: Record<string, number>
  persisted: boolean
}

export interface AnalyzerInfo {
  name: string
  description: string
}

export interface StartCrawlParams {
  url: string
  depth?: number
  max_pages?: number
  concurrency?: number
  render?: string
  analyzers?: string[]
  specialized?: boolean
  respect_robots?: boolean
  subdomains?: boolean
  include?: string[]
  exclude?: string[]
  save: boolean
}
