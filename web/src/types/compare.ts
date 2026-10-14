// Mirrors internal/diff.Diff (GET /api/compare) and the POST /api/reports/import response.
// Issue and page lists can be null in the JSON when empty (Go nil slices); read them with `?? []`.
import type { Issue } from './crawls'

export interface CompareMeta {
  seed: string
  started_at: string
  finished_at: string
  pages_crawled: number
}

export interface CompareSummary {
  by_severity: Record<string, number>
  by_analyzer: Record<string, number>
  pages_by_status: Record<string, number>
  new_by_severity: Record<string, number>
  resolved_by_severity: Record<string, number>
}

export interface CompareResult {
  base: CompareMeta
  current: CompareMeta
  issues: {
    new: Issue[] | null
    resolved: Issue[] | null
    persisting: Issue[] | null
  }
  summary: CompareSummary
  pages: {
    added: string[] | null
    removed: string[] | null
  }
}

export interface ImportResult {
  id: string
}
