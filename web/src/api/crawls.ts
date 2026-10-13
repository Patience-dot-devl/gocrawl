import { postJSON, request } from './client'
import type { AnalyzerInfo, CrawlListItem, Explanation, JobView, StartCrawlParams } from '../types/crawls'

export function listAnalyzers(): Promise<{ analyzers: AnalyzerInfo[] }> {
  return request('/api/analyzers')
}

export function listExplanations(): Promise<{ explanations: Record<string, Explanation> }> {
  return request('/api/explanations')
}

export function startCrawl(params: StartCrawlParams): Promise<JobView> {
  return postJSON('/api/crawls', params)
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
