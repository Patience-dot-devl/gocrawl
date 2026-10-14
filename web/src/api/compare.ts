import { request } from './client'
import type { CompareResult } from '../types/compare'

// A ref is a crawl id from listCrawls, a host such as example.com, or "latest".
export function compareCrawls(base: string, current: string): Promise<CompareResult> {
  const q = new URLSearchParams({ base, current })
  return request<CompareResult>(`/api/compare?${q.toString()}`)
}
