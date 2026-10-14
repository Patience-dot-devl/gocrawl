import { request } from './client'
import type { JobWithProgress, SitemapPreview, SitemapPreviewRequest } from '../types/progress'

/** One poll of a crawl: tape characters from tapeFrom on, plus the latest snapshot. */
export function getCrawlProgress(id: string, tapeFrom: number, signal?: AbortSignal): Promise<JobWithProgress> {
  return request(`/api/crawls/${encodeURIComponent(id)}?tape_from=${tapeFrom}`, { signal })
}

/** Runs only the sitemap discovery step for a crawl scope. */
export function previewSitemap(body: SitemapPreviewRequest, signal?: AbortSignal): Promise<SitemapPreview> {
  return request('/api/sitemap-preview', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal,
  })
}
