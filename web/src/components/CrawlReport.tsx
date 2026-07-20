import { useEffect, useRef, useState } from 'react'
import { cancelCrawl, exportURL, getCrawl } from '../api'
import type { JobView } from '../types'

const POLL_MS = 1500

export default function CrawlReport({ id }: { id: string }) {
  const [job, setJob] = useState<JobView | null>(null)
  const [error, setError] = useState('')
  const [severityFilter, setSeverityFilter] = useState('')
  const [analyzerFilter, setAnalyzerFilter] = useState('')
  const timer = useRef<number | undefined>(undefined)

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

          <div className="filters">
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
                .filter((i) => (!severityFilter || i.severity === severityFilter) && (!analyzerFilter || i.analyzer === analyzerFilter))
                .map((i, idx) => (
                  <tr key={idx}>
                    <td className={`severity-${i.severity}`}>{i.severity}</td>
                    <td>{i.analyzer}</td>
                    <td>{i.code}</td>
                    <td className="url-cell" title={i.url}>
                      {i.url}
                    </td>
                    <td>{i.message}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  )
}
