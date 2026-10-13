import { useEffect, useMemo, useRef, useState } from 'react'
import { cancelCrawl, exportURL, getCrawl, listExplanations } from '../api/crawls'
import { ApiError } from '../api/client'
import { href } from '../routes'
import SiteMap from './SiteMap'
import { useSort } from '../sort'
import { STATUS_PILL } from '../crawlStatus'
import type { Explanation, Issue, JobView } from '../types/crawls'
import Button from './ui/Button'
import ConfirmDialog from './ui/ConfirmDialog'
import Notice from './ui/Notice'
import PhasePill from './ui/PhasePill'
import StatusCode from './ui/StatusCode'
import './CrawlReport.css'

const POLL_MS = 1500
const SEVERITY_RANK: Record<string, number> = { error: 0, warning: 1, info: 2 }

interface ReviewState {
  resolved: boolean
  nonissue: boolean
  note: string
}

type IssueRow = Issue & { _key: number }

function formatCount(n: number): string {
  return n.toLocaleString('en-US')
}

// The noun in the right number: "1 error", "2 errors".
function noun(n: number, singular: string): string {
  return n === 1 ? singular : `${singular}s`
}

// An error message without its final full stop, so a sentence can follow it cleanly.
function trimEnd(message: string): string {
  return message.replace(/[.\s]+$/, '')
}

function formatElapsed(ms: number): string {
  const totalSec = Math.max(0, Math.floor(ms / 1000))
  const m = Math.floor(totalSec / 60)
  const s = totalSec % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

function hostOf(seed: string): string {
  try {
    return new URL(seed).host
  } catch {
    return seed
  }
}

function issueSortValue(row: IssueRow, key: string): string | number {
  switch (key) {
    case 'severity':
      return SEVERITY_RANK[row.severity] ?? 9
    case 'analyzer':
      return row.analyzer
    case 'code':
      return row.code
    case 'url':
      return row.url
    case 'message':
      return row.message
    default:
      return ''
  }
}

export default function CrawlReport({ id }: { id: string }) {
  const [job, setJob] = useState<JobView | null>(null)
  // loadError replaces the whole view (nothing loaded, or the crawl does not exist); actionError
  // is an inline notice above the report, for a failed action such as Stop crawl.
  const [loadError, setLoadError] = useState<{ message: string; notFound: boolean } | null>(null)
  const [actionError, setActionError] = useState('')
  const [pollError, setPollError] = useState('')
  const [confirmStop, setConfirmStop] = useState(false)
  const [headingFocus, setHeadingFocus] = useState(0)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const [severityOn, setSeverityOn] = useState<Record<string, boolean>>({ error: true, warning: true, info: true })
  const [analyzerFilter, setAnalyzerFilter] = useState('')
  const [search, setSearch] = useState('')
  const [codeOn, setCodeOn] = useState<Record<string, boolean>>({})
  const [codeMenuOpen, setCodeMenuOpen] = useState(false)
  const [hideResolved, setHideResolved] = useState(false)
  const [hideNonissue, setHideNonissue] = useState(false)
  const [review, setReview] = useState<Record<number, ReviewState>>({})
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [tab, setTab] = useState<'issues' | 'sitemap'>('issues')
  const [now, setNow] = useState(() => Date.now())
  const [explanations, setExplanations] = useState<Record<string, Explanation>>({})
  const timer = useRef<number | undefined>(undefined)
  const selectAllRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    listExplanations()
      .then((r) => setExplanations(r.explanations))
      .catch(() => undefined)
  }, [])

  useEffect(() => {
    let stopped = false
    let wasRunning = false
    let loaded = false
    async function poll() {
      try {
        const j = await getCrawl(id)
        if (stopped) return
        loaded = true
        wasRunning = j.status === 'running'
        setJob(j)
        setPollError('')
        if (wasRunning) {
          timer.current = window.setTimeout(poll, POLL_MS)
        }
      } catch (err) {
        if (stopped) return
        const message = err instanceof Error ? err.message : String(err)
        if (loaded) {
          // A report is already on screen: keep it, say what happened, and try again.
          setPollError(`Could not refresh this crawl: ${trimEnd(message)}. Trying again.`)
          if (wasRunning) timer.current = window.setTimeout(poll, POLL_MS)
          return
        }
        setLoadError({ message, notFound: err instanceof ApiError && err.status === 404 })
      }
    }
    poll()
    return () => {
      stopped = true
      window.clearTimeout(timer.current)
    }
  }, [id])

  // The server attaches the report once, when the crawl finishes, and App keys this component
  // by crawl id, so the report-derived UI state below starts fresh for every report without a
  // reset effect. A code missing from codeOn counts as shown.
  const report = job?.report

  useEffect(() => {
    if (job?.status !== 'running') return
    const t = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(t)
  }, [job?.status])

  useEffect(() => {
    if (!codeMenuOpen) return
    function close() {
      setCodeMenuOpen(false)
    }
    document.addEventListener('click', close)
    return () => document.removeEventListener('click', close)
  }, [codeMenuOpen])

  const codeCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    report?.issues.forEach((i) => {
      counts[i.code] = (counts[i.code] ?? 0) + 1
    })
    return counts
  }, [report])
  const codeList = useMemo(() => Object.keys(codeCounts).sort(), [codeCounts])
  const hiddenCodeCount = codeList.filter((c) => codeOn[c] === false).length

  const rows: IssueRow[] = useMemo(
    () => (report ? report.issues.map((i, originalIndex) => ({ ...i, _key: originalIndex })) : []),
    [report],
  )

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase()
    return rows.filter((i) => {
      if (severityOn[i.severity] === false) return false
      if (analyzerFilter && i.analyzer !== analyzerFilter) return false
      if (codeOn[i.code] === false) return false
      if (term && !(i.url.toLowerCase().includes(term) || i.message.toLowerCase().includes(term) || i.code.toLowerCase().includes(term))) {
        return false
      }
      const r = review[i._key]
      if (hideResolved && r?.resolved) return false
      if (hideNonissue && r?.nonissue) return false
      return true
    })
  }, [rows, severityOn, analyzerFilter, codeOn, search, hideResolved, hideNonissue, review])

  const { sorted, toggle, dirFor } = useSort(filtered, issueSortValue, { key: 'severity' })

  // The dialog is only meaningful while the crawl runs: if the crawl finishes while it is open,
  // it goes away, and the Stop crawl button that opened it goes with it. Focus then has nowhere
  // to return to, so it moves to the report heading. Stopping the crawl moves it there too.
  const dialogOpen = confirmStop && job?.status === 'running'
  const wasOpen = useRef(false)
  useEffect(() => {
    if (wasOpen.current && !dialogOpen && (!document.activeElement || document.activeElement === document.body)) {
      headingRef.current?.focus()
    }
    wasOpen.current = dialogOpen
  }, [dialogOpen])
  useEffect(() => {
    if (headingFocus > 0) headingRef.current?.focus()
  }, [headingFocus])

  const selectedShownCount = sorted.filter((r) => selected.has(r._key)).length
  useEffect(() => {
    if (!selectAllRef.current) return
    selectAllRef.current.indeterminate = selectedShownCount > 0 && selectedShownCount < sorted.length
  }, [selectedShownCount, sorted.length])

  if (loadError?.notFound) {
    return (
      <div className="cr">
        <h1>Crawl not found</h1>
        <p>It may have been removed, or it was never saved.</p>
        <p>
          <a href={href({ view: 'crawls' })}>Go to Crawls</a>
        </p>
      </div>
    )
  }
  if (loadError) return <Notice tone="error">{loadError.message}</Notice>
  if (!job) return <p className="muted">Loading…</p>

  const emptyReview: ReviewState = { resolved: false, nonissue: false, note: '' }

  function setReviewFlag(key: number, patch: Partial<ReviewState>) {
    setReview((prev) => ({
      ...prev,
      [key]: { ...emptyReview, ...prev[key], ...patch },
    }))
  }

  function toggleSelected(key: number) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  function selectShown(checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      sorted.forEach((r) => (checked ? next.add(r._key) : next.delete(r._key)))
      return next
    })
  }

  function bulkApply(kind: 'resolved' | 'nonissue' | 'clear') {
    setReview((prev) => {
      const next = { ...prev }
      sorted.forEach((r) => {
        if (!selected.has(r._key)) return
        if (kind === 'clear') {
          next[r._key] = { ...emptyReview, note: prev[r._key]?.note ?? '' }
        } else {
          next[r._key] = { ...emptyReview, ...prev[r._key], [kind]: true }
        }
      })
      return next
    })
    setSelected(new Set())
  }

  function clearFilters() {
    setSeverityOn({ error: true, warning: true, info: true })
    setAnalyzerFilter('')
    setSearch('')
    setCodeOn(Object.fromEntries(codeList.map((c) => [c, true])))
    setHideResolved(false)
    setHideNonissue(false)
  }

  function exportReviewed() {
    if (!report) return
    const annotated = {
      ...report,
      issues: report.issues.map((issue, idx) => {
        const r = review[idx]
        if (!r || (!r.resolved && !r.nonissue && !r.note.trim())) return issue
        return { ...issue, review: r }
      }),
    }
    const blob = new Blob([JSON.stringify(annotated, null, 2)], { type: 'application/json' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'gocrawl-report-reviewed.json'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(a.href)
  }

  const filtersActive =
    search.trim() !== '' ||
    analyzerFilter !== '' ||
    hiddenCodeCount > 0 ||
    hideResolved ||
    hideNonissue ||
    Object.values(severityOn).some((v) => v === false)

  const pill = STATUS_PILL[job.status]

  function stopCrawl() {
    setConfirmStop(false)
    setHeadingFocus((n) => n + 1)
    setActionError('')
    cancelCrawl(id).catch((err) => {
      // A 404 or 409 means the crawl is no longer running, usually because it finished while the
      // dialog was open. That is not a failure for the user: show the finished crawl.
      if (err instanceof ApiError && (err.status === 404 || err.status === 409)) {
        getCrawl(id)
          .then(setJob)
          .catch(() => undefined)
        return
      }
      setActionError(
        `Could not stop the crawl: ${trimEnd(err instanceof Error ? err.message : String(err))}. Try Stop crawl again.`,
      )
    })
  }

  return (
    <div className="cr">
      <header className="cr-head">
        <h1 ref={headingRef} tabIndex={-1}>
          {hostOf(job.seed)}
        </h1>
        <PhasePill phase={pill.phase} live>{pill.label}</PhasePill>
        <div className="cr-actions">
          {job.status === 'running' && (
            <Button variant="stop" onClick={() => setConfirmStop(true)}>
              Stop crawl
            </Button>
          )}
          {report && (
            <>
              <a className="btn btn-secondary" href={exportURL(id, 'json')}>
                Export JSON
              </a>
              <a className="btn btn-secondary" href={exportURL(id, 'csv')}>
                Export CSV
              </a>
              <a className="btn btn-secondary" href={exportURL(id, 'html')}>
                Export HTML
              </a>
              <Button onClick={exportReviewed} title="Download this report with your review marks and comments merged in">
                Export reviewed JSON
              </Button>
            </>
          )}
        </div>
      </header>
      <p className="cr-seed mono">{job.seed}</p>

      {dialogOpen && (
        <ConfirmDialog
          title="Stop this crawl?"
          confirmLabel="Stop crawl"
          confirmVariant="stop"
          cancelLabel="Keep crawling"
          onConfirm={stopCrawl}
          onCancel={() => setConfirmStop(false)}
        >
          <p>The report keeps the pages fetched so far.</p>
        </ConfirmDialog>
      )}

      {actionError && <Notice tone="error">{actionError}</Notice>}
      {pollError && <Notice tone="warning">{pollError}</Notice>}

      {job.status === 'running' && job.started_at && (
        <div className="cr-stats">
          <div className="cr-stat">
            <b className="num">{formatElapsed(now - Date.parse(job.started_at))}</b>
            <span>elapsed</span>
          </div>
        </div>
      )}

      {job.error && <Notice tone="error">{job.error}</Notice>}

      {report && (
        <>
          {report.coverage && !report.coverage.complete && (
            <Notice tone="warning" role="alert">
              <p>
                <strong>Partial coverage: this crawl did not reach the whole site.</strong>{' '}
                {report.coverage.duration_limit_reached ? (
                  <>
                    The crawl stopped when it reached its max duration. Findings cover only what was fetched in that
                    time, and many pages may not have been discovered at all. Raise the max duration (or leave it
                    empty for no limit) and crawl again for full coverage.
                  </>
                ) : report.coverage.interrupted ? (
                  <>
                    The crawl was stopped before it finished (for example with Stop crawl). Findings cover only what
                    was fetched before that, and many pages may not have been discovered at all. Crawl again and let
                    it finish for full coverage.
                  </>
                ) : (
                  <>
                    {formatCount(report.coverage.discovered_not_crawled ?? 0)} in-scope URL
                    {report.coverage.discovered_not_crawled !== 1 ? 's were' : ' was'} discovered but not crawled
                    {report.coverage.page_limit_reached &&
                      `, because the page budget (${formatCount(report.coverage.max_pages ?? 0)}) was reached`}
                    {report.coverage.depth_limit_reached &&
                      `${report.coverage.page_limit_reached ? ' and' : ', because'} the depth limit (${formatCount(report.coverage.max_depth ?? 0)}) was reached`}
                    . Page-level findings, broken links especially, may be incomplete.{' '}
                    {report.coverage.page_limit_reached && report.coverage.depth_limit_reached
                      ? 'Raise the page budget and the depth, or leave them empty for no limit,'
                      : report.coverage.depth_limit_reached
                        ? 'Raise the depth, or leave it empty for no limit,'
                        : 'Raise the page budget, or leave it empty for no limit,'}{' '}
                    and crawl again for full coverage.
                  </>
                )}
              </p>
            </Notice>
          )}

          {(() => {
            // runner.Run appends the same coverageNote() text to report.notes whenever
            // coverage is incomplete, always prefixed "partial coverage: ", which the
            // notice above already covers more richly. Filter it out here so the web view
            // doesn't show it twice; report.html.tmpl never renders .notes at all, so this
            // duplication is specific to the live view.
            const otherNotes = (report.notes ?? []).filter((n) => !n.startsWith('partial coverage:'))
            return (
              otherNotes.length > 0 && (
                <Notice tone="info">
                  <ul className="cr-notes">
                    {otherNotes.map((n, i) => (
                      <li key={i}>{n}</li>
                    ))}
                  </ul>
                </Notice>
              )
            )
          })()}

          <div className="cr-stats">
            <div className="cr-stat">
              <b className="num">{formatCount(report.pages_crawled)}</b>
              <span>{noun(report.pages_crawled, 'page')} crawled</span>
            </div>
            {(['error', 'warning', 'info'] as const)
              // Errors always show, even at 0, so the row keeps its shape between crawls.
              .filter((sev) => sev === 'error' || report.summary.by_severity[sev] !== undefined)
              .map((sev) => {
                const count = report.summary.by_severity[sev] ?? 0
                return (
                  <div className="cr-stat" key={sev}>
                    <b className={`num sev-${sev}`}>{formatCount(count)}</b>
                    <span>{sev === 'info' ? 'info' : noun(count, sev)}</span>
                  </div>
                )
              })}
          </div>

          <div className="cr-panels">
            <section className="cr-panel">
              <h2>By analyzer</h2>
              {Object.keys(report.summary.by_analyzer).length > 0 ? (
                <table>
                  <tbody>
                    {Object.entries(report.summary.by_analyzer).map(([a, count]) => (
                      <tr key={a}>
                        <td className="mono">{a}</td>
                        <td className="num cr-count">{count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <p className="muted">No analyzer reported an issue.</p>
              )}
            </section>
            <section className="cr-panel">
              <h2>Pages by status</h2>
              {Object.keys(report.summary.pages_by_status).length > 0 ? (
                <table>
                  <tbody>
                    {Object.entries(report.summary.pages_by_status).map(([s, count]) => (
                      <tr key={s}>
                        <td>{/^\d+$/.test(s) ? <StatusCode code={Number(s)} /> : s}</td>
                        <td className="num cr-count">{count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <p className="muted">No pages were crawled.</p>
              )}
            </section>
          </div>

          {report.site_map && (
            <div className="cr-tabs">
              <button
                type="button"
                className={`cr-tab${tab === 'issues' ? ' cr-tab-on' : ''}`}
                aria-pressed={tab === 'issues'}
                onClick={() => setTab('issues')}
              >
                Issues
              </button>
              <button
                type="button"
                className={`cr-tab${tab === 'sitemap' ? ' cr-tab-on' : ''}`}
                aria-pressed={tab === 'sitemap'}
                onClick={() => setTab('sitemap')}
              >
                Site map
              </button>
            </div>
          )}

          {tab === 'sitemap' && report.site_map ? (
            <SiteMap siteMap={report.site_map} />
          ) : (
            <>
              <div className="cr-filters">
                <div className="cr-sevs" role="group" aria-label="Severity">
                  {(['error', 'warning', 'info'] as const).map((sev) => (
                    <button
                      key={sev}
                      type="button"
                      className={`cr-sev sev-${sev}`}
                      aria-pressed={severityOn[sev]}
                      onClick={() => setSeverityOn((prev) => ({ ...prev, [sev]: !prev[sev] }))}
                    >
                      {sev.charAt(0).toUpperCase() + sev.slice(1)}
                    </button>
                  ))}
                </div>
                <label className="cr-inline">
                  Analyzer
                  <select value={analyzerFilter} onChange={(e) => setAnalyzerFilter(e.target.value)}>
                    <option value="">All</option>
                    {Object.keys(report.summary.by_analyzer).map((a) => (
                      <option key={a} value={a}>
                        {a}
                      </option>
                    ))}
                  </select>
                </label>
                <div className={`cr-codes${hiddenCodeCount > 0 ? ' cr-codes-hidden' : ''}`}>
                  <Button
                    aria-expanded={codeMenuOpen}
                    onClick={(e) => {
                      e.stopPropagation()
                      setCodeMenuOpen((v) => !v)
                    }}
                  >
                    {hiddenCodeCount > 0 ? `Codes (${hiddenCodeCount} hidden)` : 'Codes'}
                    <span aria-hidden="true">▾</span>
                  </Button>
                  <div className="cr-codes-panel" hidden={!codeMenuOpen} onClick={(e) => e.stopPropagation()}>
                    <div className="cr-codes-actions">
                      <Button onClick={() => setCodeOn(Object.fromEntries(codeList.map((c) => [c, true])))}>
                        Show all
                      </Button>
                      <Button onClick={() => setCodeOn(Object.fromEntries(codeList.map((c) => [c, false])))}>
                        Hide all
                      </Button>
                    </div>
                    {codeList.map((c) => (
                      <label key={c} className="cr-code-option">
                        <input
                          type="checkbox"
                          checked={codeOn[c] !== false}
                          onChange={(e) => setCodeOn((prev) => ({ ...prev, [c]: e.target.checked }))}
                        />
                        <code>{c}</code>
                        <span className="num muted">{codeCounts[c]}</span>
                      </label>
                    ))}
                  </div>
                </div>
                <input
                  type="search"
                  className="cr-search"
                  placeholder="Search URL, message, code…"
                  aria-label="Search issues"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
                <label className="cr-inline">
                  <input type="checkbox" checked={hideResolved} onChange={(e) => setHideResolved(e.target.checked)} />
                  Hide resolved
                </label>
                <label className="cr-inline">
                  <input type="checkbox" checked={hideNonissue} onChange={(e) => setHideNonissue(e.target.checked)} />
                  Hide non-issues
                </label>
                <span className="cr-count-line num muted">
                  Showing {formatCount(sorted.length)} of {formatCount(rows.length)} {noun(rows.length, 'issue')}
                </span>
                {filtersActive && <Button onClick={clearFilters}>Clear filters</Button>}
              </div>

              <div className="cr-bulk">
                <label className="cr-inline">
                  <input
                    ref={selectAllRef}
                    type="checkbox"
                    checked={sorted.length > 0 && selectedShownCount === sorted.length}
                    onChange={(e) => selectShown(e.target.checked)}
                  />
                  Select shown
                </label>
                <Button disabled={selected.size === 0} onClick={() => bulkApply('nonissue')}>
                  Mark as non-issue
                </Button>
                <Button disabled={selected.size === 0} onClick={() => bulkApply('resolved')}>
                  Mark as resolved
                </Button>
                <Button disabled={selected.size === 0} onClick={() => bulkApply('clear')}>
                  Clear review
                </Button>
                {selected.size > 0 && <span className="num muted">{selected.size} selected</span>}
              </div>

              <div className="cr-table-scroll">
                <table className="cr-issues">
                  <thead>
                    <tr>
                      {(['severity', 'analyzer', 'code', 'url', 'message'] as const).map((col) => (
                        <th
                          key={col}
                          className="cr-sortable"
                          data-dir={dirFor(col)}
                          aria-sort={dirFor(col) === 'asc' ? 'ascending' : dirFor(col) === 'desc' ? 'descending' : undefined}
                        >
                          <button type="button" onClick={() => toggle(col)}>
                            {col === 'url' ? 'URL' : col.charAt(0).toUpperCase() + col.slice(1)}
                          </button>
                        </th>
                      ))}
                      <th>Review</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sorted.map((i) => {
                      const explanation = explanations[i.code]
                      const r = review[i._key]
                      return (
                        <tr key={i._key} className={r?.resolved || r?.nonissue ? 'cr-reviewed' : undefined}>
                          <td className={`sev-${i.severity}`}>{i.severity}</td>
                          <td className="mono">{i.analyzer}</td>
                          <td>
                            <code>{i.code}</code>
                          </td>
                          <td className="cr-url mono" title={i.url}>
                            {i.url}
                          </td>
                          <td>
                            {i.message}
                            {explanation && (
                              <details className="cr-explain">
                                <summary>What this means and how to fix it</summary>
                                <dl>
                                  <dt>What it is</dt>
                                  <dd>{explanation.what}</dd>
                                  <dt>Impact</dt>
                                  <dd>{explanation.impact}</dd>
                                  <dt>How to fix it</dt>
                                  <dd>{explanation.fix}</dd>
                                </dl>
                              </details>
                            )}
                            {i.data && (
                              <details className="cr-explain">
                                <summary>Data</summary>
                                <pre>{JSON.stringify(i.data, null, 2)}</pre>
                              </details>
                            )}
                          </td>
                          <td className="cr-review">
                            <label className="cr-inline">
                              <input type="checkbox" checked={selected.has(i._key)} onChange={() => toggleSelected(i._key)} />
                              Select
                            </label>
                            <label className="cr-inline">
                              <input
                                type="checkbox"
                                checked={!!r?.resolved}
                                onChange={(e) => setReviewFlag(i._key, { resolved: e.target.checked })}
                              />
                              Resolved
                            </label>
                            <label className="cr-inline">
                              <input
                                type="checkbox"
                                checked={!!r?.nonissue}
                                onChange={(e) => setReviewFlag(i._key, { nonissue: e.target.checked })}
                              />
                              Non-issue
                            </label>
                            <textarea
                              rows={2}
                              placeholder="Add a comment…"
                              aria-label="Comment"
                              value={r?.note ?? ''}
                              onChange={(e) => setReviewFlag(i._key, { note: e.target.value })}
                            />
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </>
      )}
    </div>
  )
}
