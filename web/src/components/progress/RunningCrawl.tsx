import { useEffect, useMemo, useState, type ReactNode } from 'react'
import Notice from '../ui/Notice'
import StatusCode from '../ui/StatusCode'
import Tape from '../tape/Tape'
import TapeLegend from '../tape/TapeLegend'
import { countTape, describeTape } from '../tape/tapeModel'
import type { RecentFetch } from './useCrawlPoll'
import type { JobWithProgress, ProgressView } from '../../types/progress'
import {
  CLASS_LABEL,
  CLASS_ORDER,
  budgetPercent,
  formatCount,
  formatDuration,
  formatElapsed,
  formatRate,
  outlineTicks,
  pathOf,
} from './tapeText'
import './progress.css'

interface Props {
  job: JobWithProgress
  progress: ProgressView | null
  tape: string
  tapeOff: string
  recent: RecentFetch[]
  receivedAt: number
  /** Shown above the tape, e.g. the auto-pause explanation. */
  notice?: ReactNode
}

// The body of a running or paused crawl: the tape, the counts around it, and the pages fetched
// most recently. CrawlReport owns the header (heading, state pill, Stop crawl), so the heading
// and its focus handling stay the same element when the crawl finishes and the report replaces
// this view.
export default function RunningCrawl({ job, progress, tape, tapeOff, recent, receivedAt, notice }: Props) {
  const paused = job.status === 'paused'
  const ticking = !paused && progress?.phase === 'crawling'
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!ticking) return
    const t = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(t)
  }, [ticking])

  const fetched = progress?.fetched ?? 0
  // Without a sitemap the live off_sitemap equals fetched, so every sitemap-relative figure is
  // gated on sitemap_found.
  const sitemapFound = progress?.sitemap_found ?? false
  const sitemapTotal = progress?.sitemap_total ?? 0
  const sitemapFetched = progress?.sitemap_fetched ?? 0
  const offSitemap = sitemapFound ? (progress?.off_sitemap ?? 0) : 0
  const maxPages = progress?.max_pages ?? 0
  const rate = progress?.rate ?? 0
  const throttled = progress?.throttled ?? 0
  const counts = useMemo(() => countTape(tape, sitemapFound ? tapeOff : ''), [tape, tapeOff, sitemapFound])
  const { notFetched, pastBudget } = outlineTicks(fetched, sitemapTotal, sitemapFetched, maxPages)
  const elapsed = (progress?.active_ms ?? 0) + (ticking && receivedAt > 0 ? Math.max(0, now - receivedAt) : 0)
  const budgetBelow = maxPages > 0 && sitemapTotal > maxPages
  const summary = describeTape(counts, notFetched, pastBudget)

  return (
    <section className="running-crawl">
      {notice}
      {budgetBelow && (
        <Notice tone="warning">
          <p>
            The page budget ({formatCount(maxPages)}) is below the sitemap total ({formatCount(sitemapTotal)}). This crawl will
            cover at most {budgetPercent(maxPages, sitemapTotal)}% of the sitemap.
          </p>
        </Notice>
      )}
      {progress?.phase === 'sitemap' && <p className="rc-sub">Reading robots.txt and the sitemap before the crawl starts…</p>}
      <Tape
        tape={tape}
        tapeOff={sitemapFound ? tapeOff : undefined}
        notFetched={notFetched}
        pastBudget={pastBudget}
        summary={summary}
        live
      />
      <p className="rc-sub" aria-hidden="true">
        {summary}
      </p>
      <TapeLegend sitemap={sitemapFound} />
      <div className="rc-stats">
        {sitemapTotal > 0 ? (
          <Stat value={formatCount(sitemapFetched)} label={`of ${formatCount(sitemapTotal)} sitemap pages fetched`} />
        ) : (
          <Stat
            value={formatCount(fetched)}
            label={maxPages > 0 ? `pages fetched of a ${formatCount(maxPages)}-page budget` : 'pages fetched'}
          />
        )}
        {sitemapFound && <Stat value={formatCount(offSitemap)} label="fetched pages not in the sitemap" />}
        <Stat value={formatCount(progress?.queued ?? 0)} label="in queue" />
        <Stat value={formatElapsed(elapsed)} label="elapsed (pauses don't count)" />
        <Stat value={rate > 0 ? `${formatRate(rate)}/s` : 'Unlimited'} label={paused ? 'request rate when paused' : 'request rate'} />
        {throttled > 0 && <Stat value={formatCount(throttled)} label="429 or 503 responses in the last minute" />}
      </div>
      <ul className="rc-classes" aria-label="Pages by status class">
        {CLASS_ORDER.map((c) => (
          <li key={c} className={`rc-class-${c}`}>
            <span className="rc-swatch" aria-hidden="true" />
            {CLASS_LABEL[c]} <b>{formatCount(counts.byClass[c])}</b>
          </li>
        ))}
      </ul>
      <h2 className="rc-h2">Fetching now</h2>
      {recent.length === 0 ? (
        <p className="rc-sub">
          {progress?.phase === 'sitemap' ? 'Pages appear here once the crawl starts.' : 'Waiting for the first page…'}
        </p>
      ) : (
        <table className="rc-recent">
          <thead>
            <tr>
              <th scope="col">Status</th>
              <th scope="col">URL</th>
              <th scope="col" className="rc-ms">
                Time
              </th>
            </tr>
          </thead>
          <tbody>
            {recent.map((r) => (
              <tr key={r.fetched}>
                <td>{r.status > 0 ? <StatusCode code={r.status} /> : <span className="rc-failed">Failed</span>}</td>
                <td className="rc-url" title={r.url}>
                  {pathOf(r.url, job.seed)}
                  {sitemapFound && !r.inSitemap && <span className="rc-off"> · not in the sitemap</span>}
                </td>
                <td className="rc-ms">{formatDuration(r.durationMs)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}

function Stat({ value, label }: { value: string; label: string }) {
  return (
    <div className="rc-stat">
      <b>{value}</b>
      <span>{label}</span>
    </div>
  )
}
