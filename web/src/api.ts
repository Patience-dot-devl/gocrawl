import type { AnalyzerInfo, CrawlListItem, JobView, StartCrawlParams } from './types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    throw new Error(body?.error || `${res.status} ${res.statusText}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export function listAnalyzers(): Promise<{ analyzers: AnalyzerInfo[] }> {
  return request('/api/analyzers')
}

export function startCrawl(params: StartCrawlParams): Promise<JobView> {
  return request('/api/crawls', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

export function listCrawls(): Promise<{ crawls: CrawlListItem[] }> {
  return request('/api/crawls')
}

export function getCrawl(id: string): Promise<JobView> {
  return request(`/api/crawls/${encodeURIComponent(id)}`)
}

export function cancelCrawl(id: string): Promise<void> {
  return request(`/api/crawls/${encodeURIComponent(id)}/cancel`, { method: 'POST' })
}

export function exportURL(id: string, format: 'json' | 'csv' | 'html'): string {
  return `/api/crawls/${encodeURIComponent(id)}/export?format=${format}`
}
