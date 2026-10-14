import { useEffect, useMemo, useState } from 'react'
import { compareCrawls } from '../api/compare'
import { getCrawl, listCrawls, listExplanations } from '../api/crawls'
import IssueTable from '../components/IssueTable'
import { countsFromStatuses, tapeFromCounts } from '../components/progress/tapeText'
import Tape from '../components/tape/Tape'
import { countTape, describeTape } from '../components/tape/tapeModel'
import Button from '../components/ui/Button'
import Notice from '../components/ui/Notice'
import { href } from '../routes'
import type { CompareMeta, CompareResult } from '../types/compare'
import type { CrawlListItem, Explanation, Issue, Report } from '../types/crawls'
import { crawlLabel, defaultComparePair, isComparable, newestFirst } from './compare/pairs'
import './compare.css'

type Tab = 'new' | 'resolved' | 'persisting'
const TABS: Tab[] = ['new', 'resolved', 'persisting']
const TAB_LABEL: Record<Tab, string> = { new: 'New', resolved: 'Resolved', persisting: 'Persisting' }
const SEVERITIES = ['error', 'warning', 'info'] as const
const MAX_URLS = 1000

interface Loaded {
  key: string
  diff: CompareResult | null
  baseReport: Report | null
  currentReport: Report | null
  error: string
}

function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function signed(n: number): string {
  if (n > 0) return `+${n.toLocaleString()}`
  if (n < 0) return `−${Math.abs(n).toLocaleString()}`
  return '0'
}

function go(base: string, current: string) {
  window.location.hash = href({ view: 'compare', base, current })
}

// One side's tape. Reports saved by this version carry it in fetch order (coverage.tape); an
// older report falls back to a status-only tape in class order, and the caption says so. The
// wording comes from describeTape, the same sentence the report's own tape uses.
function CompareTape({ label, meta, report }: { label: string; meta: CompareMeta; report: Report | null }) {
  const when = meta.finished_at ? new Date(meta.finished_at).toLocaleString() : 'unknown time'
  const heading = `${label}: ${meta.seed}, ${when}`
  if (!report) {
    return (
      <div>
        <p className="compare-tape-label">{heading}</p>
        <p className="compare-tape-counts">
          {meta.pages_crawled.toLocaleString()} pages. The status tape is not available for this crawl.
        </p>
      </div>
    )
  }
  const saved = report.coverage?.tape ?? ''
  const hasOrder = saved !== ''
  const tape = hasOrder ? saved : tapeFromCounts(countsFromStatuses(report.summary.pages_by_status))
  const tapeOff = hasOrder ? (report.coverage?.tape_off ?? '') : ''
  const counts = describeTape(countTape(tape, tapeOff))
  return (
    <div>
      <p className="compare-tape-label">{heading}</p>
      <Tape tape={tape} tapeOff={tapeOff} summary={`${label} crawl. ${counts}`} compact />
      <p className="compare-tape-counts">
        {counts}{' '}
        {hasOrder
          ? 'Ticks are in fetch order.'
          : 'This report was saved without a tape, so its ticks are grouped by status, not in fetch order.'}
      </p>
    </div>
  )
}

function UrlList({ title, urls, empty }: { title: string; urls: string[]; empty: string }) {
  return (
    <section>
      <h2>
        {title} ({urls.length.toLocaleString()})
      </h2>
      {urls.length === 0 ? (
        <p className="muted">{empty}</p>
      ) : (
        <ul className="compare-url-list">
          {urls.slice(0, MAX_URLS).map((u) => (
            <li key={u}>
              <code>{u}</code>
            </li>
          ))}
        </ul>
      )}
      {urls.length > MAX_URLS && <p className="muted">and {(urls.length - MAX_URLS).toLocaleString()} more</p>}
    </section>
  )
}

function DeltaTable({ caption, entries, empty }: { caption: string; entries: [string, number][]; empty: string }) {
  return (
    <table>
      <caption>{caption}</caption>
      <tbody>
        {entries.length === 0 ? (
          <tr>
            <th scope="row">{empty}</th>
          </tr>
        ) : (
          entries.map(([k, v]) => (
            <tr key={k}>
              <th scope="row">{k}</th>
              <td>{signed(v)}</td>
            </tr>
          ))
        )}
      </tbody>
    </table>
  )
}

// The loaded comparison. CompareView keys it by the pair, so the tab and every issue table's
// filters start fresh for a new pair.
function CompareResultView({
  diff,
  baseReport,
  currentReport,
  explanations,
}: {
  diff: CompareResult
  baseReport: Report | null
  currentReport: Report | null
  explanations: Record<string, Explanation>
}) {
  const [tab, setTab] = useState<Tab>('new')
  // A tab's table mounts the first time it is shown and then stays mounted (hidden), so its
  // filters and search survive switching tabs without rendering all three lists up front.
  const [visited, setVisited] = useState<Tab[]>(['new'])
  const lists: Record<Tab, Issue[]> = {
    new: diff.issues.new ?? [],
    resolved: diff.issues.resolved ?? [],
    persisting: diff.issues.persisting ?? [],
  }

  function show(t: Tab) {
    setTab(t)
    setVisited((v) => (v.includes(t) ? v : [...v, t]))
  }

  return (
    <>
      <section className="compare-tapes" aria-label="Status tapes">
        <CompareTape label="Base" meta={diff.base} report={baseReport} />
        <CompareTape label="Current" meta={diff.current} report={currentReport} />
      </section>

      <p className="compare-headline">
        {lists.new.length.toLocaleString()} new, {lists.resolved.length.toLocaleString()} resolved,{' '}
        {lists.persisting.length.toLocaleString()} unchanged issues. Pages: {signed((diff.pages.added ?? []).length)} added,{' '}
        {signed(-(diff.pages.removed ?? []).length)} removed.
      </p>

      <div className="compare-deltas">
        <table>
          <caption>Issues by severity</caption>
          <thead>
            <tr>
              <th scope="col">Severity</th>
              <th scope="col" className="compare-num">
                New
              </th>
              <th scope="col" className="compare-num">
                Resolved
              </th>
              <th scope="col" className="compare-num">
                Net change
              </th>
            </tr>
          </thead>
          <tbody>
            {SEVERITIES.map((sev) => (
              <tr key={sev}>
                <th scope="row">{sev}</th>
                <td>{(diff.summary.new_by_severity?.[sev] ?? 0).toLocaleString()}</td>
                <td>{(diff.summary.resolved_by_severity?.[sev] ?? 0).toLocaleString()}</td>
                <td>{signed(diff.summary.by_severity?.[sev] ?? 0)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <DeltaTable
          caption="Issues by analyzer, net change"
          entries={Object.entries(diff.summary.by_analyzer ?? {}).sort(([a], [b]) => a.localeCompare(b))}
          empty="No change"
        />
        <DeltaTable
          caption="Pages by status, net change"
          entries={Object.entries(diff.summary.pages_by_status ?? {}).sort(([a], [b]) => Number(a) - Number(b))}
          empty="No change"
        />
      </div>

      <div className="cr-tabs">
        {TABS.map((t) => (
          <button
            key={t}
            type="button"
            className={`cr-tab${tab === t ? ' cr-tab-on' : ''}`}
            aria-pressed={tab === t}
            onClick={() => show(t)}
          >
            {TAB_LABEL[t]} ({lists[t].length.toLocaleString()})
          </button>
        ))}
      </div>
      {TABS.filter((t) => visited.includes(t)).map((t) => (
        <div key={t} hidden={tab !== t}>
          <IssueTable issues={lists[t]} explanations={explanations} />
        </div>
      ))}

      <div className="compare-pages">
        <UrlList title="Added pages" urls={diff.pages.added ?? []} empty="No pages were added." />
        <UrlList title="Removed pages" urls={diff.pages.removed ?? []} empty="No pages were removed." />
      </div>
    </>
  )
}

export default function CompareView({ base, current }: { base?: string; current?: string }) {
  const [crawls, setCrawls] = useState<CrawlListItem[] | null>(null)
  const [listError, setListError] = useState('')
  const [explanations, setExplanations] = useState<Record<string, Explanation>>({})
  const [loaded, setLoaded] = useState<Loaded | null>(null)

  useEffect(() => {
    listCrawls()
      .then((r) => setCrawls(r.crawls))
      .catch((e) => setListError(errorText(e)))
    listExplanations()
      .then((r) => setExplanations(r.explanations))
      .catch(() => undefined)
  }, [])

  const candidates = useMemo(() => newestFirst((crawls ?? []).filter(isComparable)), [crawls])
  const pair = useMemo(() => (base && current ? { base, current } : defaultComparePair(candidates)), [base, current, candidates])
  const pairBase = pair?.base ?? ''
  const pairCurrent = pair?.current ?? ''
  const key = `${pairBase}\n${pairCurrent}`

  useEffect(() => {
    if (!pairBase || !pairCurrent) return
    let stopped = false
    const loadKey = `${pairBase}\n${pairCurrent}`
    // A failed getCrawl (for example a "latest" ref) only costs that side its tape.
    const reportOf = (id: string) =>
      getCrawl(id)
        .then((j) => j.report ?? null)
        .catch(() => null)
    Promise.all([compareCrawls(pairBase, pairCurrent), reportOf(pairBase), reportOf(pairCurrent)])
      .then(([diff, baseReport, currentReport]) => {
        if (!stopped) setLoaded({ key: loadKey, diff, baseReport, currentReport, error: '' })
      })
      .catch((e) => {
        if (!stopped) setLoaded({ key: loadKey, diff: null, baseReport: null, currentReport: null, error: errorText(e) })
      })
    return () => {
      stopped = true
    }
  }, [pairBase, pairCurrent])

  if (listError) return <Notice tone="error">Could not load the crawl list: {listError}</Notice>
  if (!crawls) return <p className="muted">Loading crawls…</p>

  // Refs typed into the URL (a host, "latest") that are not in the list still get an option.
  const extraRefs = [pairBase, pairCurrent].filter((ref, i, all) => ref && all.indexOf(ref) === i && !candidates.some((c) => c.id === ref))

  function choose(side: 'base' | 'current', id: string) {
    const fallback = candidates.find((c) => c.id !== id)?.id ?? id
    const nextBase = side === 'base' ? id : pairBase || fallback
    const nextCurrent = side === 'current' ? id : pairCurrent || fallback
    go(nextBase, nextCurrent)
  }

  const view = loaded && loaded.key === key ? loaded : null

  function picker(side: 'base' | 'current', label: string, value: string) {
    return (
      <label>
        {label}
        <select value={value} onChange={(e) => choose(side, e.target.value)}>
          {!value && <option value="">Choose a crawl</option>}
          {candidates.map((c) => (
            <option key={c.id} value={c.id}>
              {crawlLabel(c)}
            </option>
          ))}
          {extraRefs.map((ref) => (
            <option key={ref} value={ref}>
              {ref}
            </option>
          ))}
        </select>
      </label>
    )
  }

  return (
    <div className="compare">
      <h1>Compare crawls</h1>
      {candidates.length < 2 && !pair ? (
        <p className="muted">
          Compare needs two finished crawls. Start another crawl, or import a saved report from{' '}
          <a href={href({ view: 'crawls' })}>Crawls</a>.
        </p>
      ) : (
        <div className="compare-pickers">
          {picker('base', 'Base (earlier)', pairBase)}
          {picker('current', 'Current (later)', pairCurrent)}
          <Button variant="secondary" disabled={!pair} onClick={() => go(pairCurrent, pairBase)}>
            Swap
          </Button>
        </div>
      )}

      {pair && pairBase === pairCurrent && <Notice tone="info">Both sides are the same crawl, so nothing can differ.</Notice>}
      {pair && !view && <p className="muted">Comparing…</p>}
      {view?.error && <Notice tone="error">{view.error}</Notice>}

      {view?.diff && (
        <CompareResultView
          key={key}
          diff={view.diff}
          baseReport={view.baseReport}
          currentReport={view.currentReport}
          explanations={explanations}
        />
      )}
    </div>
  )
}
