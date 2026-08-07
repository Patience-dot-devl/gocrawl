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

export interface PageIssue {
  severity: 'info' | 'warning' | 'error'
  code: string
  analyzer: string
  message: string
}

export interface SevCounts {
  error?: number
  warning?: number
  info?: number
}

export interface SiteMapNode {
  label: string
  url?: string
  title?: string
  status?: number
  depth?: number
  issues?: PageIssue[]
  counts?: SevCounts
  subtotal?: SevCounts
  children?: SiteMapNode[]
}

export interface SiteMapEntry {
  loc: string
  lastmod?: string
}

export interface SiteMap {
  seed: string
  host: string
  generated: string
  entries?: SiteMapEntry[]
  root?: SiteMapNode
  site_wide?: PageIssue[]
  totals?: SevCounts
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
  site_map?: SiteMap
}

export interface Explanation {
  what: string
  impact: string
  fix: string
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
  rate?: number
  max_duration?: string
  render?: string
  analyzers?: string[]
  specialized?: boolean
  security_audit?: boolean
  ignore_external_tagging?: boolean
  respect_robots?: boolean
  subdomains?: boolean
  follow_external?: boolean
  include?: string[]
  exclude?: string[]
  user_agent?: string
  user_agents?: string[]
  user_agent_rotation?: string
  proxy?: string
  proxies?: string[]
  proxy_rotation?: string
  basic_auth?: string
  save: boolean
}
