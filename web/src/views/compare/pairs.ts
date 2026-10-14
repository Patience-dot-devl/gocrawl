import type { CrawlListItem } from '../../types/crawls'
import { hostOf } from '../../components/progress/tapeText'

// A crawl can be compared once it has a report: finished, or stopped with a partial report.
export function isComparable(item: CrawlListItem): boolean {
  return item.status === 'done' || item.status === 'canceled'
}

function finishedMs(item: CrawlListItem): number {
  const t = Date.parse(item.finished_at ?? '')
  return Number.isNaN(t) ? 0 : t
}

export function newestFirst(items: CrawlListItem[]): CrawlListItem[] {
  return [...items].sort((a, b) => finishedMs(b) - finishedMs(a))
}

// The newest crawl that has an older crawl of the same host, paired with that older crawl.
// Expects items already sorted newest first.
export function defaultComparePair(items: CrawlListItem[]): { base: string; current: string } | null {
  for (let i = 0; i < items.length; i++) {
    const host = hostOf(items[i].seed)
    const older = items.slice(i + 1).find((o) => hostOf(o.seed) === host)
    if (older) return { base: older.id, current: items[i].id }
  }
  return null
}

// Puts the earlier crawl on the base side.
export function orderPair(a: CrawlListItem, b: CrawlListItem): { base: string; current: string } {
  return finishedMs(a) <= finishedMs(b) ? { base: a.id, current: b.id } : { base: b.id, current: a.id }
}

export function crawlLabel(item: CrawlListItem): string {
  const when = item.finished_at ? new Date(item.finished_at).toLocaleString() : 'not finished'
  return `${hostOf(item.seed)} · ${when} · ${(item.pages_crawled ?? 0).toLocaleString()} pages`
}
