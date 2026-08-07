import { useEffect, useMemo, useRef, useState } from 'react'
import { cancelCrawl, exportURL, getCrawl, listExplanations } from '../api'
import SiteMap from './SiteMap'
import { useSort } from '../sort'
import type { Explanation, Issue, JobView } from '../types'

const POLL_MS = 1500
const SEVERITY_RANK: Record<string, number> = { error: 0, warning: 1, info: 2 }

interface ReviewState {
  resolved: boolean
  nonissue: boolean
  note: string
}

type IssueRow = Issue & { _key: number }

function formatElapsed(ms: number): string {
  const totalSec = Math.max(0, Math.floor(ms / 1000))
  const m = Math.floor(totalSec / 60)
  const s = totalSec % 60
  return `${m}:${String(s).padStart(2, '0')}`
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
  const [error, setError] = useState('')
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
    setJob(null)
    setError('')

    let stopped = false
    async function poll() {
      try {
        const j = await getCrawl(id)
        if (stopped) return
        setJob(j)
        if (j.status === 'running') {
          timer.current = window.setTimeout(poll, POLL_MS)
        }
      } catch (err) {
        if (!stopped) setError(String(err))
      }
    }
    poll()
    return () => {
      stopped = true
      window.clearTimeout(timer.current)
    }
  }, [id])

  const report = job?.report

  // Reset all report-derived UI state whenever a new report loads, and seed the codes filter
  // with every code present (default: everything shown).
  useEffect(() => {
    if (!report) return
    const codes = Array.from(new Set(report.issues.map((i) => i.code))).sort()
    setCodeOn(Object.fromEntries(codes.map((c) => [c, true])))
    setSeverityOn({ error: true, warning: true, info: true })
    setAnalyzerFilter('')
    setSearch('')
    setHideResolved(false)
    setHideNonissue(false)
    setReview({})
    setSelected(new Set())
    setTab('issues')
  }, [report])

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

  const selectedShownCount = sorted.filter((r) => selected.has(r._key)).length
  useEffect(() => {
    if (!selectAllRef.current) return
    selectAllRef.current.indeterminate = selectedShownCount > 0 && selectedShownCount < sorted.length
  }, [selectedShownCount, sorted.length])

  if (error) return <p className="error">{error}</p>
  if (!job) return <p>Loading…</p>

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

  return (
    <div className="card">
      <div className="report-header">
        <div>
          <h2>{job.seed}</h2>
          <p className={`status status-${job.status}`}>{job.status}</p>
          {job.status === 'running' && job.started_at && (
            <div className="progress">
              <span className="progress-elapsed">{formatElapsed(now - Date.parse(job.started_at))} elapsed</span>
              <span className="progress-bar" role="progressbar" aria-label="Crawl in progress" />
            </div>
          )}
        </div>
        {job.status === 'running' && (
          <button
            onClick={() => {
              if (!window.confirm('Cancel this crawl? The report will only include pages already fetched.')) return
              cancelCrawl(id).catch((e) => setError(String(e)))
            }}
          >
            Cancel
          </button>
        )}
        {report && (
          <div className="export-buttons">
            <a href={exportURL(id, 'json')}>Export JSON</a>
            <a href={exportURL(id, 'csv')}>Export CSV</a>
            <a href={exportURL(id, 'html')}>Export HTML</a>
            <button type="button" onClick={exportReviewed} title="Download this report with your resolved/non-issue/comment state merged in">
              Export reviewed (JSON)
            </button>
          </div>
        )}
      </div>

      {job.error && <p className="error">{job.error}</p>}

      {report && (
        <>
          {report.coverage && !report.coverage.complete && (
            <div className="cov-banner" role="alert">
              <strong>Partial coverage — this crawl did not reach the whole site.</strong>{' '}
              {report.coverage.duration_limit_reached ? (
                <>
                  The crawl was stopped after reaching its max-duration time budget. Findings reflect only what was
                  fetched within that window, and many in-scope pages may not have been discovered at all. Raise the
                  max duration (or leave it unlimited) and re-run for full coverage.
                </>
              ) : report.coverage.interrupted ? (
                <>
                  The crawl was interrupted before it finished (e.g. the Cancel button). Findings reflect only what
                  was fetched before the interruption, and many in-scope pages may not have been discovered at all.
                  Re-run to completion for full coverage.
                </>
              ) : (
                <>
                  {report.coverage.discovered_not_crawled ?? 0} in-scope URL
                  {report.coverage.discovered_not_crawled !== 1 ? 's were' : ' was'} discovered but not crawled
                  {report.coverage.page_limit_reached && (
                    <>
                      , because the page limit (<code>--max-pages {report.coverage.max_pages}</code>) was reached
                    </>
                  )}
                  {report.coverage.depth_limit_reached && (
                    <>
                      {report.coverage.page_limit_reached ? ' and' : ', because'} the depth limit (
                      <code>--depth {report.coverage.max_depth}</code>) was reached
                    </>
                  )}
                  . Page-level findings — <strong>broken links especially</strong> — may be incomplete. Re-crawl with
                  a higher limit (or <code>0</code> for unlimited) for full coverage.
                </>
              )}
            </div>
          )}

          {(() => {
            // runner.Run appends the same coverageNote() text to report.notes whenever
            // coverage is incomplete — always prefixed "partial coverage: " — which the
            // banner above already covers more richly. Filter it out here so the web view
            // doesn't show it twice; report.html.tmpl never renders .notes at all, so this
            // duplication is specific to the live view.
            const otherNotes = (report.notes ?? []).filter((n) => !n.startsWith('partial coverage:'))
            return (
              otherNotes.length > 0 && (
                <ul className="notes">
                  {otherNotes.map((n, i) => (
                    <li key={i}>{n}</li>
                  ))}
                </ul>
              )
            )
          })()}

          <div className="summary-cards">
            <div className="stat">
              <span className="stat-value">{report.pages_crawled}</span>
              <span className="stat-label">pages crawled</span>
            </div>
            {Object.entries(report.summary.by_severity).map(([sev, count]) => (
              <div className="stat" key={sev}>
                <span className={`stat-value severity-${sev}`}>{count}</span>
                <span className="stat-label">{sev}</span>
              </div>
            ))}
          </div>

          <div className="subcards">
            <div className="subcard">
              <h3>By analyzer</h3>
              {Object.keys(report.summary.by_analyzer).length > 0 ? (
                <table>
                  <tbody>
                    {Object.entries(report.summary.by_analyzer).map(([a, count]) => (
                      <tr key={a}>
                        <td>{a}</td>
                        <td>{count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <p className="empty">No analyzers reported.</p>
              )}
            </div>
            <div className="subcard">
              <h3>Pages by status</h3>
              {Object.keys(report.summary.pages_by_status).length > 0 ? (
                <table>
                  <tbody>
                    {Object.entries(report.summary.pages_by_status).map(([s, count]) => (
                      <tr key={s}>
                        <td>{s}</td>
                        <td>{count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <p className="empty">No pages crawled.</p>
              )}
            </div>
          </div>

          {report.site_map && (
            <nav className="tabs">
              <button type="button" className={`tab${tab === 'issues' ? ' active' : ''}`} onClick={() => setTab('issues')}>
                Issues &amp; summary
              </button>
              <button type="button" className={`tab${tab === 'sitemap' ? ' active' : ''}`} onClick={() => setTab('sitemap')}>
                Site map
              </button>
            </nav>
          )}

          {tab === 'sitemap' && report.site_map ? (
            <SiteMap siteMap={report.site_map} />
          ) : (
            <>
              <div className="filters">
                <div className="sevfilters">
                  {(['error', 'warning', 'info'] as const).map((sev) => (
                    <button
                      key={sev}
                      type="button"
                      className={`sevbtn sev-${sev}${severityOn[sev] ? ' active' : ''}`}
                      onClick={() => setSeverityOn((prev) => ({ ...prev, [sev]: !prev[sev] }))}
                    >
                      {sev.charAt(0).toUpperCase() + sev.slice(1)}
                    </button>
                  ))}
                </div>
                <label>
                  Analyzer
                  <select value={analyzerFilter} onChange={(e) => setAnalyzerFilter(e.target.value)}>
                    <option value="">all</option>
                    {Object.keys(report.summary.by_analyzer).map((a) => (
                      <option key={a} value={a}>
                        {a}
                      </option>
                    ))}
                  </select>
                </label>
                <div className={`msel${hiddenCodeCount > 0 ? ' has-hidden' : ''}`}>
                  <button
                    type="button"
                    aria-expanded={codeMenuOpen}
                    onClick={(e) => {
                      e.stopPropagation()
                      setCodeMenuOpen((v) => !v)
                    }}
                  >
                    {hiddenCodeCount > 0 ? `Codes (${hiddenCodeCount} hidden)` : 'Codes'}
                    <span className="caret">▾</span>
                  </button>
                  <div className="msel-panel" hidden={!codeMenuOpen} onClick={(e) => e.stopPropagation()}>
                    <div className="msel-acts">
                      <button type="button" onClick={() => setCodeOn(Object.fromEntries(codeList.map((c) => [c, true])))}>
                        Select all
                      </button>
                      <button type="button" onClick={() => setCodeOn(Object.fromEntries(codeList.map((c) => [c, false])))}>
                        Hide all
                      </button>
                    </div>
                    {codeList.map((c) => (
                      <label key={c}>
                        <input
                          type="checkbox"
                          checked={codeOn[c] !== false}
                          onChange={(e) => setCodeOn((prev) => ({ ...prev, [c]: e.target.checked }))}
                        />
                        <code>{c}</code>
                        <span className="msel-n">{codeCounts[c]}</span>
                      </label>
                    ))}
                  </div>
                </div>
                <input
                  type="search"
                  placeholder="Search URL, message, code…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
                <label>
                  <input type="checkbox" checked={hideResolved} onChange={(e) => setHideResolved(e.target.checked)} />
                  Hide resolved
                </label>
                <label>
                  <input type="checkbox" checked={hideNonissue} onChange={(e) => setHideNonissue(e.target.checked)} />
                  Hide non-issues
                </label>
                <span className="filter-count">
                  Showing {sorted.length.toLocaleString()} of {rows.length.toLocaleString()} issues
                </span>
                {filtersActive && (
                  <button type="button" className="filter-clear" onClick={clearFilters}>
                    Clear filters
                  </button>
                )}
                <div className="bulkbar">
                  <label>
                    <input
                      ref={selectAllRef}
                      type="checkbox"
                      checked={sorted.length > 0 && selectedShownCount === sorted.length}
                      onChange={(e) => selectShown(e.target.checked)}
                    />
                    Select shown
                  </label>
                  <button type="button" disabled={selected.size === 0} onClick={() => bulkApply('nonissue')}>
                    → Non-issue
                  </button>
                  <button type="button" disabled={selected.size === 0} onClick={() => bulkApply('resolved')}>
                    → Resolved
                  </button>
                  <button type="button" disabled={selected.size === 0} onClick={() => bulkApply('clear')}>
                    Clear
                  </button>
                  {selected.size > 0 && <span className="seln">{selected.size} selected</span>}
                </div>
              </div>

              <div className="table-scroll">
                <table className="issues">
                  <thead>
                    <tr>
                      {(['severity', 'analyzer', 'code', 'url', 'message'] as const).map((col) => (
                        <th key={col} className="sortable" onClick={() => toggle(col)} data-dir={dirFor(col)}>
                          {col.charAt(0).toUpperCase() + col.slice(1)}
                        </th>
                      ))}
                      <th className="nosort">Review</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sorted.map((i) => {
                      const explanation = explanations[i.code]
                      const r = review[i._key]
                      return (
                        <tr key={i._key} className={r?.resolved || r?.nonissue ? 'reviewed' : undefined}>
                          <td className={`severity-${i.severity}`}>{i.severity}</td>
                          <td>{i.analyzer}</td>
                          <td>{i.code}</td>
                          <td className="url-cell" title={i.url}>
                            {i.url}
                          </td>
                          <td>
                            {i.message}
                            {explanation && (
                              <details className="explain">
                                <summary>what this means &amp; how to fix</summary>
                                <dl>
                                  <dt>What it is</dt>
                                  <dd>{explanation.what}</dd>
                                  <dt>Impact</dt>
                                  <dd>{explanation.impact}</dd>
                                  <dt>How to fix</dt>
                                  <dd>{explanation.fix}</dd>
                                </dl>
                              </details>
                            )}
                            {i.data && (
                              <details>
                                <summary>data</summary>
                                <pre>{JSON.stringify(i.data, null, 2)}</pre>
                              </details>
                            )}
                          </td>
                          <td className="review">
                            <label>
                              <input type="checkbox" checked={selected.has(i._key)} onChange={() => toggleSelected(i._key)} />
                              Select
                            </label>
                            <label>
                              <input
                                type="checkbox"
                                checked={!!r?.resolved}
                                onChange={(e) => setReviewFlag(i._key, { resolved: e.target.checked })}
                              />
                              Resolved
                            </label>
                            <label>
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
