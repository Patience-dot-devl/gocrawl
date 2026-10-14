import { postForm, request } from './client'
import type { RedirectCheck, StartRedirectCheckParams } from '../types/redirects'

export function startRedirectCheck(p: StartRedirectCheckParams): Promise<RedirectCheck> {
  const form = new FormData()
  form.append('file', p.file, p.file.name)
  form.append('domain', p.domain)
  if (p.sitemapURL) form.append('sitemap_url', p.sitemapURL)
  if (p.concurrency !== undefined) form.append('concurrency', String(p.concurrency))
  if (p.rate !== undefined) form.append('rate', String(p.rate))
  form.append('adaptive_delay', String(p.adaptiveDelay))
  return postForm<RedirectCheck>('/api/redirect-checks', form)
}

export function listRedirectChecks(): Promise<{ checks: RedirectCheck[] }> {
  return request<{ checks: RedirectCheck[] }>('/api/redirect-checks')
}

// Rows come back only once the check is done. A 404 means the id is unknown (for example
// after a restart); the thrown ApiError carries the status.
export function getRedirectCheck(id: string): Promise<RedirectCheck> {
  return request<RedirectCheck>(`/api/redirect-checks/${encodeURIComponent(id)}`)
}

// Resolves on 204. Rejects with a 409 ApiError when the check is no longer running.
export function cancelRedirectCheck(id: string): Promise<void> {
  return request<void>(`/api/redirect-checks/${encodeURIComponent(id)}/cancel`, { method: 'POST' })
}

// A plain download link (409 until the check is done); never fetched from script.
export function redirectExportURL(id: string): string {
  return `/api/redirect-checks/${encodeURIComponent(id)}/export`
}
