// Helpers for the crawl tape and the counts around it. components/tape/Tape.tsx draws the
// tape and tapeModel.ts (buildTicks, countTape, describeTape) builds its ticks and its text
// alternative; this module holds the rest: counts from list items and reports, scaled
// status-only tapes, and number, rate and URL formatting. Status classes, their order and
// their labels come from web/src/status.ts, so the legend, StatusCode and these counts all
// use one vocabulary.
import { STATUS_CLASSES, STATUS_LABEL, statusClass, type StatusClass } from '../../status'

export type TapeClass = StatusClass
export type ClassCounts = Record<TapeClass, number>

export const CLASS_ORDER: TapeClass[] = STATUS_CLASSES

/** Legend labels, one per status class. */
export const CLASS_LABEL: Record<TapeClass, string> = STATUS_LABEL

/** The server's page budget when the form leaves max pages empty (crawler.DefaultOptions). */
export const DEFAULT_MAX_PAGES = 500

export function emptyCounts(): ClassCounts {
  return { '2': 0, '3': 0, '4': 0, '5': 0, x: 0 }
}

export const classOf: (status: number) => TapeClass = statusClass

function isClass(c: string): c is TapeClass {
  return c === '2' || c === '3' || c === '4' || c === '5' || c === 'x'
}

export function countClasses(tape: string): ClassCounts {
  const counts = emptyCounts()
  for (const ch of tape) if (isClass(ch)) counts[ch] += 1
  return counts
}

/** Counts from a list item's status_classes ("2" -> n). */
export function countsFromRecord(rec?: Record<string, number>): ClassCounts {
  const counts = emptyCounts()
  if (!rec) return counts
  for (const [k, n] of Object.entries(rec)) if (isClass(k)) counts[k] += n
  return counts
}

/** Counts from a report's pages_by_status ("404" -> n). */
export function countsFromStatuses(byStatus?: Record<string, number>): ClassCounts {
  const counts = emptyCounts()
  if (!byStatus) return counts
  for (const [code, n] of Object.entries(byStatus)) counts[classOf(Number(code))] += n
  return counts
}

export function totalOf(counts: ClassCounts): number {
  return CLASS_ORDER.reduce((n, c) => n + counts[c], 0)
}

/** A status-only tape in class order (fallback for reports saved before coverage.tape existed). */
export function tapeFromCounts(counts: ClassCounts): string {
  return CLASS_ORDER.map((c) => c.repeat(counts[c])).join('')
}

/** A status-only tape scaled to at most maxTicks; every non-empty class keeps one tick. */
export function scaleTape(counts: ClassCounts, maxTicks: number): string {
  const total = totalOf(counts)
  if (total <= maxTicks) return tapeFromCounts(counts)
  const scaled = emptyCounts()
  for (const c of CLASS_ORDER) {
    if (counts[c] > 0) scaled[c] = Math.max(1, Math.round((counts[c] / total) * maxTicks))
  }
  return tapeFromCounts(scaled)
}

export function formatCount(n: number): string {
  return n.toLocaleString('en-US')
}

export function percentOf(part: number, whole: number): number {
  return whole > 0 ? Math.floor((part * 100) / whole) : 0
}

/** "At most N%" of the sitemap a budget can cover; never shows 0%. */
export function budgetPercent(maxPages: number, sitemapTotal: number): number {
  return Math.max(1, percentOf(maxPages, sitemapTotal))
}

export function formatElapsed(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const ss = String(total % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
}

export function formatRate(rate: number): string {
  if (rate >= 10) return rate.toFixed(0)
  if (rate >= 1) return rate.toFixed(1)
  return String(Number(rate.toPrecision(2)))
}

export function formatDuration(ms: number): string {
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`
}

/** Half the current rate, as a form value ('' when unlimited). */
export function halfRate(rate: number): string {
  return rate > 0 ? String(Number((rate / 2).toPrecision(2))) : ''
}

export interface Outline {
  notFetched: number
  pastBudget: number
}

/**
 * Outline ticks after the fetched ones: sitemap pages the budget still allows (not fetched yet,
 * or after the crawl, not reached by links) and sitemap pages past the page budget. Without a
 * sitemap there are none: the counts fall back to the page budget as text instead.
 */
export function outlineTicks(fetched: number, sitemapTotal: number, sitemapFetched: number, maxPages: number): Outline {
  if (sitemapTotal <= 0) return { notFetched: 0, pastBudget: 0 }
  const remaining = Math.max(0, sitemapTotal - sitemapFetched)
  if (maxPages <= 0) return { notFetched: remaining, pastBudget: 0 }
  const notFetched = Math.min(remaining, Math.max(0, maxPages - fetched))
  return { notFetched, pastBudget: remaining - notFetched }
}

/** "1,020 of 1,240 (82%)" for a crawl list row, or an em dash without sitemap data. */
export function sitemapCoverageText(total?: number, fetched?: number): string {
  if (!total) return '—'
  const f = fetched ?? 0
  return `${formatCount(f)} of ${formatCount(total)} (${percentOf(f, total)}%)`
}

export function hostOf(url: string): string {
  try {
    return new URL(url).host
  } catch {
    return url
  }
}

/** Path and query when url is on the seed's host, else the full URL. */
export function pathOf(url: string, seed: string): string {
  try {
    const u = new URL(url)
    return u.host === new URL(seed).host ? `${u.pathname}${u.search}` : url
  } catch {
    return url
  }
}

/** Last path segment of a URL ("sitemap_index.xml"). */
export function fileName(url: string): string {
  try {
    const parts = new URL(url).pathname.split('/')
    return parts[parts.length - 1] || url
  } catch {
    return url
  }
}
