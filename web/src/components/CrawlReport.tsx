import { useEffect, useRef, useState } from 'react'
import { cancelCrawl, exportURL, getCrawl, listExplanations } from '../api'
import type { Explanation, JobView } from '../types'

const POLL_MS = 1500

export default function CrawlReport({ id }: { id: string }) {
  const [job, setJob] = useState<JobView | null>(null)
  const [error, setError] = useState('')
  const [severityFilter, setSeverityFilter] = useState('')
  const [analyzerFilter, setAnalyzerFilter] = useState('')
  const [search, setSearch] = useState('')
  const [explanations, setExplanations] = useState<Record<string, Explanation>>({})
  const timer = useRef<number | undefined>(undefined)

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

  if (error) return <p className="error">{error}</p>
  if (!job) return <p>Loading…</p>

  const report = job.report

  return (
    <div className="card">
      <div className="report-header">
        <div>
          <h2>{job.seed}</h2>
          <p className={`status status-${job.status}`}>{job.status}</p>
        </div>
        {job.status === 'running' && (
          <button onClick={() => cancelCrawl(id).catch((e) => setError(String(e)))}>Cancel</button>
        )}
        {report && (
          <div className="export-buttons">
            <a href={exportURL(id, 'json')}>Export JSON</a>
            <a href={exportURL(id, 'csv')}>Export CSV</a>
            <a href={exportURL(id, 'html')}>Export HTML</a>
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

          {report.notes && report.notes.length > 0 && (
            <ul className="notes">
              {report.notes.map((n, i) => (
                <li key={i}>{n}</li>
              ))}
            </ul>
          )}

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

          <div className="filters">
            <label>
              Search
              <input
                type="search"
                placeholder="Search URL, message, code…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </label>
            <label>
              Severity
              <select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value)}>
                <option value="">all</option>
                {Object.keys(report.summary.by_severity).map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </label>
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
          </div>

          <table className="issues">
            <thead>
              <tr>
                <th>Severity</th>
                <th>Analyzer</th>
                <th>Code</th>
                <th>URL</th>
                <th>Message</th>
              </tr>
            </thead>
            <tbody>
              {report.issues
                .map((i, originalIndex) => ({ ...i, _key: originalIndex }))
                .filter((i) => (!severityFilter || i.severity === severityFilter) && (!analyzerFilter || i.analyzer === analyzerFilter))
                .filter((i) => {
                  if (!search.trim()) return true
                  const term = search.trim().toLowerCase()
                  return i.url.toLowerCase().includes(term) || i.message.toLowerCase().includes(term) || i.code.toLowerCase().includes(term)
                })
                .map((i) => {
                  const explanation = explanations[i.code]
                  return (
                    <tr key={i._key}>
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
                    </tr>
                  )
                })}
            </tbody>
          </table>
        </>
      )}
    </div>
  )
}
