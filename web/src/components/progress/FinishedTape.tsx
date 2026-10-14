import { useMemo } from 'react'
import Tape from '../tape/Tape'
import TapeLegend from '../tape/TapeLegend'
import { countTape, describeTape, type TapeState } from '../tape/tapeModel'
import type { CrawlStatus, Report } from '../../types/crawls'
import { countsFromStatuses, formatCount, outlineTicks, percentOf, tapeFromCounts } from './tapeText'
import './progress.css'

interface Props {
  report: Report
  status: CrawlStatus
}

// The tape and the sitemap coverage line on a finished or stopped crawl.
//
// Every report saved by this version carries the tape in fetch order (coverage.tape, with
// coverage.tape_off when a sitemap was found), built from the same events as the live tape,
// so a crawl reopened after a restart looks as it did live. A report saved before that falls
// back to a status-only tape in class order, without notches or outline ticks.
//
// The slate outline ticks are sitemap pages the crawl did not fetch. Only a crawl that ran to
// its natural end can say they were "not reached by links"; after Stop crawl or a max
// duration they are plain "not fetched", because links to them may exist.
export default function FinishedTape({ report, status }: Props) {
  const cov = report.coverage
  const pages = report.pages_crawled
  const sitemapTotal = cov?.sitemap_total ?? 0
  const sitemapFetched = cov?.sitemap_fetched ?? 0
  const saved = cov?.tape ?? ''
  const savedOff = saved !== '' ? (cov?.tape_off ?? '') : ''
  const hasOrder = saved !== ''
  // tape_off is empty exactly when no sitemap was found.
  const sitemapFound = sitemapTotal > 0 || savedOff !== ''
  const tape = hasOrder ? saved : tapeFromCounts(countsFromStatuses(report.summary.pages_by_status))
  const state: TapeState = status === 'done' && !cov?.interrupted && !cov?.duration_limit_reached ? 'done' : 'stopped'
  const showSitemap = hasOrder && sitemapFound

  const { notFetched, pastBudget } = showSitemap
    ? outlineTicks(tape.length, sitemapTotal, sitemapFetched, cov?.max_pages ?? 0)
    : { notFetched: 0, pastBudget: 0 }
  const summary = useMemo(
    () => describeTape(countTape(tape, showSitemap ? savedOff : ''), notFetched, pastBudget, state),
    [tape, savedOff, showSitemap, notFetched, pastBudget, state],
  )
  const offSitemap = sitemapTotal > 0 ? Math.max(0, pages - sitemapFetched) : 0

  return (
    <section className="finished-tape">
      {(tape !== '' || notFetched + pastBudget > 0) && (
        <>
          <Tape
            tape={tape}
            tapeOff={showSitemap ? savedOff : undefined}
            notFetched={notFetched}
            pastBudget={pastBudget}
            summary={summary}
          />
          <TapeLegend sitemap={showSitemap} state={state} />
        </>
      )}
      <p className="ft-coverage">
        {sitemapTotal > 0 ? (
          <>
            Fetched <b>{formatCount(sitemapFetched)}</b> of <b>{formatCount(sitemapTotal)}</b> sitemap pages (
            {percentOf(sitemapFetched, sitemapTotal)}%).
            {offSitemap > 0 && (
              <> {formatCount(offSitemap)} fetched {offSitemap === 1 ? 'page is' : 'pages are'} not in the sitemap.</>
            )}
          </>
        ) : (
          <>
            {formatCount(pages)} {pages === 1 ? 'page' : 'pages'} fetched. No sitemap coverage for this crawl.
          </>
        )}
      </p>
    </section>
  )
}
