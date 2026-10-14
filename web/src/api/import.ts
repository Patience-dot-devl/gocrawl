import { postForm } from './client'
import type { ImportResult } from '../types/compare'

// Saves a gocrawl JSON report to the crawl store; resolves to its crawl id.
export function importReport(file: File): Promise<ImportResult> {
  const form = new FormData()
  form.append('file', file, file.name)
  return postForm<ImportResult>('/api/reports/import', form)
}
