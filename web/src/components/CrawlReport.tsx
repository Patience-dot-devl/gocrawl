import { useEffect, useRef, useState } from 'react'
import { cancelCrawl, exportURL, listExplanations } from '../api/crawls'
import { ApiError } from '../api/client'
import { href } from '../routes'
import SiteMap from './SiteMap'
import { STATUS_PILL } from '../crawlStatus'
import type { Explanation } from '../types/crawls'
import IssueTable, { type ReviewMap } from './IssueTable'
import Button from './ui/Button'
import ConfirmDialog from './ui/ConfirmDialog'
import Notice from './ui/Notice'
import PhasePill from './ui/PhasePill'
import StatusCode from './ui/StatusCode'
import FinishedTape from './progress/FinishedTape'
import AutoPauseNotice from './progress/AutoPauseNotice'
import CrawlControls from './progress/CrawlControls'
import RunningCrawl from './progress/RunningCrawl'
import { runPill } from './progress/runPill'
import { halfRate, hostOf } from './progress/tapeText'
import { isLive, useCrawlPoll } from './progress/useCrawlPoll'
import './CrawlReport.css'

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

export default function CrawlReport({ id }: { id: string }) {
  // loadError replaces the whole view (nothing loaded, or the crawl does not exist); pollError
  // is an inline notice above the report (a refresh failed, polling goes on); actionError is an
  // inline notice for a failed action (Pause, Resume crawl, Stop crawl).
  const { job, progress, tape, tapeOff, recent, receivedAt, loadError, pollError, refresh } = useCrawlPoll(id)
  const [actionError, setActionError] = useState('')
  const [confirmStop, setConfirmStop] = useState(false)
  // null until the user edits "Resume at": the field then shows half the current rate.
  const [resumeRateInput, setResumeRateInput] = useState<string | null>(null)
  const [headingFocus, setHeadingFocus] = useState(0)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const [review, setReview] = useState<ReviewMap>({})
  const [tab, setTab] = useState<'issues' | 'sitemap'>('issues')
  const [explanations, setExplanations] = useState<Record<string, Explanation>>({})

  useEffect(() => {
    listExplanations()
      .then((r) => setExplanations(r.explanations))
      .catch(() => undefined)
  }, [])

  // The server attaches the report once, when the crawl finishes, and App keys this component
  // by crawl id, so the report-derived UI state below starts fresh for every report without a
  // reset effect.
  const report = job?.report

  // The dialog is only meaningful while the crawl runs: if the crawl finishes while it is open,
  // it goes away, and the Stop crawl button that opened it goes with it. Focus then has nowhere
  // to return to, so it moves to the report heading. Stopping the crawl moves it there too.
  const live = isLive(job?.status)
  const dialogOpen = confirmStop && live
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

  // One PhasePill for every state, so its live region persists and a screen reader hears
  // "Crawling" become "Crawl stopped".
  const pill = job.status === 'running' ? runPill(progress?.phase) : STATUS_PILL[job.status]

  // A 404 or 409 means the crawl is no longer in the state the action needs, usually because it
  // finished or changed phase meanwhile. That is not a failure for the user: show where it is now.
  function actionFailed(label: string, err: unknown) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 409)) {
      refresh()
      return
    }
    setActionError(
      `Could not ${label.toLowerCase()}: ${trimEnd(err instanceof Error ? err.message : String(err))}. Try ${label} again.`,
    )
  }

  function stopCrawl() {
    setConfirmStop(false)
    setHeadingFocus((n) => n + 1)
    setActionError('')
    cancelCrawl(id)
      .then(refresh)
      .catch((err) => actionFailed('Stop crawl', err))
  }

  // An automatic pause after repeated 429/503 responses asks the user for a lower rate.
  const autoPaused = job.status === 'paused' && progress?.pause_reason === 'rate_limited'
  const rateText = resumeRateInput ?? (progress ? halfRate(progress.rate) : '')

  return (
    <div className="cr">
      <header className="cr-head">
        <h1 ref={headingRef} tabIndex={-1}>
          {hostOf(job.seed)}
        </h1>
        <PhasePill phase={pill.phase} live>
          {pill.label}
        </PhasePill>
        <div className="cr-actions">
          {live && (
            <CrawlControls
              id={id}
              status={job.status}
              phase={progress?.phase}
              resumeRate={autoPaused && Number(rateText) > 0 ? Number(rateText) : undefined}
              onChanged={() => {
                setActionError('')
                setResumeRateInput(null)
                refresh()
              }}
              onError={(verb, err) => actionFailed(verb === 'pause' ? 'Pause' : 'Resume crawl', err)}
              onStop={() => setConfirmStop(true)}
            />
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
      {pollError && (
        <Notice tone="warning">
          {`Could not refresh this crawl: ${trimEnd(pollError)}. ${live ? 'Trying again.' : 'Reload the page to try again.'}`}
        </Notice>
      )}

      {live && (
        <RunningCrawl
          job={job}
          progress={progress}
          tape={tape}
          tapeOff={tapeOff}
          recent={recent}
          receivedAt={receivedAt}
          notice={
            autoPaused && progress ? (
              <AutoPauseNotice progress={progress} rate={rateText} onRateChange={setResumeRateInput} />
            ) : undefined
          }
        />
      )}

      {job.error && <Notice tone="error">{job.error}</Notice>}

      {report && (
        <>
          <FinishedTape report={report} status={job.status} />

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

          {tab === 'sitemap' && report.site_map && <SiteMap siteMap={report.site_map} />}
          {/* Stays mounted behind the Site map tab, so its filters and search survive a tab switch. */}
          <div hidden={tab === 'sitemap' && !!report.site_map}>
            <IssueTable issues={report.issues} explanations={explanations} review={{ map: review, set: setReview }} />
          </div>
        </>
      )}
    </div>
  )
}
