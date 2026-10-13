import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError } from '../../api/client'
import { getCrawlProgress } from '../../api/progress'
import type { JobWithProgress, ProgressView } from '../../types/progress'

export const POLL_MS = 1500
const RECENT_MAX = 8

/** One row of the "Fetching now" table, taken from a progress snapshot. */
export interface RecentFetch {
  fetched: number
  url: string
  status: number
  durationMs: number
  inSitemap: boolean
}

export interface CrawlPoll {
  /** The latest job; kept when a later poll fails. */
  job: JobWithProgress | null
  progress: ProgressView | null
  tape: string
  tapeOff: string
  recent: RecentFetch[]
  /** Date.now() when the latest poll landed; lets the elapsed clock tick between polls. */
  receivedAt: number
  /** Set while nothing has loaded: the first poll failed. notFound is a 404 (unknown crawl id). */
  loadError: { message: string; notFound: boolean } | null
  /** The message of a failed poll after a job loaded; the view keeps the job and polling goes on. */
  pollError: string
  /** Poll right away (after stop, or when the crawl may have finished) instead of waiting for the next tick. */
  refresh: () => void
}

export function isLive(status: string | undefined): boolean {
  return status === 'running' || status === 'paused'
}

/**
 * Polls GET /api/crawls/{id} every POLL_MS while the crawl is running or paused. The tape is
 * accumulated client-side: each poll asks only for characters from the current length on
 * (tape_from), and the server says where its slice starts, so a shorter server tape truncates
 * instead of duplicating. A failed poll keeps the last job and, while the crawl is live, tries
 * again; only a failure before anything loaded is a loadError.
 */
export function useCrawlPoll(id: string): CrawlPoll {
  const [job, setJob] = useState<JobWithProgress | null>(null)
  const [tape, setTape] = useState({ tape: '', off: '' })
  const [recent, setRecent] = useState<RecentFetch[]>([])
  const [receivedAt, setReceivedAt] = useState(0)
  const [loadError, setLoadError] = useState<CrawlPoll['loadError']>(null)
  const [pollError, setPollError] = useState('')
  const [nonce, setNonce] = useState(0)
  const tapeRef = useRef({ tape: '', off: '' })
  // The last status that loaded; survives a refresh, which restarts the effect below.
  const lastStatus = useRef<string | undefined>(undefined)

  useEffect(() => {
    let stopped = false
    let timer: number | undefined
    async function poll() {
      try {
        const j = await getCrawlProgress(id, tapeRef.current.tape.length)
        if (stopped) return
        lastStatus.current = j.status
        const p = j.progress
        if (p) {
          const cur = tapeRef.current
          const keep = Math.min(p.tape_from, cur.tape.length)
          const next = { tape: cur.tape.slice(0, keep) + p.tape, off: cur.off.slice(0, keep) + p.tape_off }
          tapeRef.current = next
          setTape(next)
          if (p.fetched > 0 && p.current_url) {
            const entry: RecentFetch = {
              fetched: p.fetched,
              url: p.current_url,
              status: p.status,
              durationMs: p.duration_ms,
              inSitemap: p.in_sitemap,
            }
            setRecent((prev) =>
              prev.length > 0 && prev[0].fetched === entry.fetched ? prev : [entry, ...prev].slice(0, RECENT_MAX),
            )
          }
        }
        setJob(j)
        setReceivedAt(Date.now())
        setLoadError(null)
        setPollError('')
        if (isLive(j.status)) timer = window.setTimeout(poll, POLL_MS)
      } catch (err) {
        if (stopped) return
        const message = err instanceof Error ? err.message : String(err)
        if (lastStatus.current === undefined) {
          setLoadError({ message, notFound: err instanceof ApiError && err.status === 404 })
          return
        }
        // A job is already on screen: keep it, say what happened, and try again while it runs.
        setPollError(message)
        if (isLive(lastStatus.current)) timer = window.setTimeout(poll, POLL_MS)
      }
    }
    void poll()
    return () => {
      stopped = true
      window.clearTimeout(timer)
    }
  }, [id, nonce])

  const refresh = useCallback(() => setNonce((n) => n + 1), [])
  return {
    job,
    progress: job?.progress ?? null,
    tape: tape.tape,
    tapeOff: tape.off,
    recent,
    receivedAt,
    loadError,
    pollError,
    refresh,
  }
}
