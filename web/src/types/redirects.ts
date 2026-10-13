// Mirrors internal/webserver redirectJobView and redirectRowView (redirectcheck.Rule and
// redirectcheck.RowResult flattened). Keep in sync with the Go side by hand.
export type RedirectVerdict = 'ok' | 'warning' | 'error' | 'skipped-external' | 'skipped-dynamic'

// '' when the rule could not be classified (its verdict is then 'error').
export type RedirectScope = 'in-scope' | 'external' | 'dynamic-pattern' | ''

// A redirect check never pauses, so this is a subset of the server's shared job statuses.
export type RedirectCheckStatus = 'running' | 'done' | 'error' | 'canceled'

export interface RedirectRow {
  original: string
  target: string
  redirect_type: string
  redirect_style: string
  priority: string
  match_query_strings: string
  ignore_trailing_slash: string
  ignore_protocol: string
  disable_if_page_exists: string
  note: string
  scope: RedirectScope
  source_status: number
  source_final_url: string
  source_matches_target: boolean
  target_status: number
  original_in_sitemap: boolean
  target_in_sitemap: boolean
  verdict: RedirectVerdict
  notes?: string[]
}

export interface RedirectCheck {
  id: string
  domain: string
  file_name?: string
  status: RedirectCheckStatus
  error?: string
  started_at?: string
  finished_at?: string
  // Rules checked so far, out of total. Both are present on every response.
  done: number
  total: number
  // Only on GET /api/redirect-checks/{id}, and only once status is 'done'. The list and the
  // start response never carry rows, and a canceled or failed check keeps none.
  rows?: RedirectRow[]
}

export interface StartRedirectCheckParams {
  file: File
  domain: string
  sitemapURL?: string
  concurrency?: number
  rate?: number
  adaptiveDelay: boolean
}
