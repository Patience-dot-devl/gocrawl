import { postJSON, request } from './client'

export function pauseCrawl(id: string): Promise<void> {
  return request(`/api/crawls/${encodeURIComponent(id)}/pause`, { method: 'POST' })
}

/** Resumes a paused crawl; a positive rate replaces the request rate, otherwise it is kept. */
export function resumeCrawl(id: string, rate?: number): Promise<void> {
  return postJSON(`/api/crawls/${encodeURIComponent(id)}/resume`, rate !== undefined && rate > 0 ? { rate } : {})
}
