import Tape from '../tape/Tape'
import { countTape, describeTape, type TapeCounts } from '../tape/tapeModel'
import type { CrawlListItemWithProgress } from '../../types/progress'
import { countsFromRecord, scaleTape, totalOf } from './tapeText'
import './progress.css'

const MINI_TICKS = 60

/**
 * A small tape for one crawl list row. The server sends tape_preview (at most 160 ticks, worst
 * status per run, in fetch order); reports saved before tapes were stored fall back to a
 * status-only tape scaled from status_classes.
 */
export default function MiniTape({ item }: { item: CrawlListItemWithProgress }) {
  const classCounts = countsFromRecord(item.status_classes)
  const total = totalOf(classCounts)
  const preview = item.tape_preview ?? ''
  if (total === 0 && !preview) return <span className="mini-tape-empty">—</span>
  // The preview is aggregated, so the words come from the full per-class counts when the list
  // item has them.
  const counts: TapeCounts =
    total > 0 ? { fetched: total, byClass: classCounts, off: 0 } : countTape(preview)
  return <Tape tape={preview || scaleTape(classCounts, MINI_TICKS)} summary={describeTape(counts)} compact />
}
