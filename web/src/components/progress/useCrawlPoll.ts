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
  /** The message of a failed poll after a job loaded; the view keeps the job and, while the crawl is live, polling goes on. */
  pollError: string
  /** Poll right away (after stop, or when the crawl may have finished) instead of waiting for the next tick. */
  refresh: () => void
}

export function isLive(status: string | undefined): boolean {
  return status === 'running' || status === 'paused'
}

// Everything the hook knows about one crawl; id says which, so a new id starts from nothing.
interface PollData {
  id: string
  job: JobWithProgress | null
  tape: string
  tapeOff: string
  recent: RecentFetch[]
  receivedAt: number
  loadError: CrawlPoll['loadError']
  pollError: string
}

function emptyData(id: string): PollData {
  return { id, job: null, tape: '', tapeOff: '', recent: [], receivedAt: 0, loadError: null, pollError: '' }
}

/**
 * Polls GET /api/crawls/{id} every POLL_MS while the crawl is running or paused. The tape is
 * accumulated client-side: each poll asks only for characters from the current length on
 * (tape_from), and the server says where its slice starts, so a shorter server tape truncates
 * instead of duplicating. A failed poll keeps the last job and, while the crawl is live, tries
 * again; only a failure before anything loaded is a loadError. Unmounting, refreshing or
 * changing id aborts the request in flight.
 */
export function useCrawlPoll(id: string): CrawlPoll {
  const [data, setData] = useState<PollData>(() => emptyData(id))
  const [nonce, setNonce] = useState(0)
  // The tape so far and the last status that loaded. They survive a refresh (which restarts the
  // effect below) but are reset when id changes.
  const tapeRef = useRef({ id, tape: '', off: '' })
  const lastStatus = useRef<string | undefined>(undefined)

  useEffect(() => {
    if (tapeRef.current.id !== id) {
      tapeRef.current = { id, tape: '', off: '' }
      lastStatus.current = undefined
    }
    const ctrl = new AbortController()
    let timer: number | undefined
    const update = (fn: (d: PollData) => PollData) => setData((prev) => fn(prev.id === id ? prev : emptyData(id)))

    async function poll() {
      try {
        const j = await getCrawlProgress(id, tapeRef.current.tape.length, ctrl.signal)
        if (ctrl.signal.aborted) return
        lastStatus.current = j.status
        const p = j.progress
        const cur = tapeRef.current
        let tape = cur.tape
        let off = cur.off
        if (p) {
          const keep = Math.min(p.tape_from, cur.tape.length)
          // Nothing new and nothing cut off: leave the tape alone.
          if (p.tape !== '' || keep !== cur.tape.length) {
            tape = cur.tape.slice(0, keep) + p.tape
            off = cur.off.slice(0, keep) + p.tape_off
            tapeRef.current = { id, tape, off }
          }
        }
        const entry: RecentFetch | null =
          p && p.fetched > 0 && p.current_url
            ? { fetched: p.fetched, url: p.current_url, status: p.status, durationMs: p.duration_ms, inSitemap: p.in_sitemap }
            : null
        update((d) => ({
          ...d,
          job: j,
          tape,
          tapeOff: off,
          recent:
            entry && d.recent[0]?.fetched !== entry.fetched ? [entry, ...d.recent].slice(0, RECENT_MAX) : d.recent,
          receivedAt: Date.now(),
          loadError: null,
          pollError: '',
        }))
        if (isLive(j.status)) timer = window.setTimeout(poll, POLL_MS)
      } catch (err) {
        if (ctrl.signal.aborted) return
        const message = err instanceof Error ? err.message : String(err)
        if (lastStatus.current === undefined) {
          update((d) => ({ ...d, loadError: { message, notFound: err instanceof ApiError && err.status === 404 } }))
          return
        }
        // A job is already on screen: keep it, say what happened, and try again while it runs.
        update((d) => ({ ...d, pollError: message }))
        if (isLive(lastStatus.current)) timer = window.setTimeout(poll, POLL_MS)
      }
    }
    void poll()
    return () => {
      ctrl.abort()
      window.clearTimeout(timer)
    }
  }, [id, nonce])

  const refresh = useCallback(() => setNonce((n) => n + 1), [])
  const d = data.id === id ? data : emptyData(id)
  return {
    job: d.job,
    progress: d.job?.progress ?? null,
    tape: d.tape,
    tapeOff: d.tapeOff,
    recent: d.recent,
    receivedAt: d.receivedAt,
    loadError: d.loadError,
    pollError: d.pollError,
    refresh,
  }
}
