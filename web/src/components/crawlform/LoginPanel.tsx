import { useEffect, useRef, useState } from 'react'
import { ApiError } from '../../api/client'
import { cancelLogin, captureLogin, getLogin, loginAvailability, startLogin } from '../../api/login'
import type { LoginAvailability, LoginSession } from '../../types/login'
import Button from '../ui/Button'
import Notice from '../ui/Notice'
import { hostOf, loginExpired } from './values'

const POLL_MS = 1500

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

interface Props {
  url: string
  // Why the button can't be used right now ('' when it can), e.g. headless rendering.
  blockedReason: string
  session: LoginSession | null
  onSession: (s: LoginSession | null) => void
  // Called once a browser window has opened, so the form can add the logout exclude.
  onStarted: () => void
  // Set to true by the form once a crawl has started with the captured login, so leaving the
  // form keeps that login instead of cancelling it.
  usedRef: { current: boolean }
}

export default function LoginPanel({ url, blockedReason, session, onSession, onStarted, usedRef }: Props) {
  const [availability, setAvailability] = useState<LoginAvailability | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    loginAvailability()
      .then(setAvailability)
      .catch((e: unknown) => setAvailability({ available: false, reason: message(e) }))
  }, [])

  // Poll while the window is open, so closing it or the one-hour expiry shows without a click.
  const waitingId = session?.state === 'waiting' ? session.id : ''
  // The id Forget/Cancel was pressed for: a poll answer for it that arrives late is ignored.
  const forgottenRef = useRef('')
  useEffect(() => {
    if (!waitingId) return
    let active = true
    const timer = window.setInterval(() => {
      getLogin(waitingId)
        .then((s) => {
          if (!active || forgottenRef.current === waitingId) return
          if (s.state !== 'waiting') onSession(s)
        })
        .catch((e: unknown) => {
          if (!active || forgottenRef.current === waitingId) return
          // 404: the server no longer knows this session. Other failures are transient; keep polling.
          if (e instanceof ApiError && e.status === 404) {
            setError('This login session ended. Log in again.')
            onSession(null)
          }
        })
    }, POLL_MS)
    return () => {
      active = false
      window.clearInterval(timer)
    }
  }, [waitingId, onSession])

  // Leaving the form closes a window that still waits and discards a captured login, unless a
  // crawl was started with it (usedRef): that crawl owns the cookies from then on.
  const openRef = useRef('')
  useEffect(() => {
    openRef.current = session && (session.state === 'waiting' || session.state === 'captured') ? session.id : ''
  }, [session])
  useEffect(
    () => () => {
      if (openRef.current && !usedRef.current) void cancelLogin(openRef.current).catch(() => undefined)
    },
    [usedRef],
  )

  async function start() {
    setBusy(true)
    setError('')
    try {
      onSession(await startLogin(url.trim()))
      onStarted()
    } catch (e) {
      setError(message(e))
    } finally {
      setBusy(false)
    }
  }

  async function capture() {
    if (!session) return
    setBusy(true)
    setError('')
    try {
      onSession(await captureLogin(session.id))
    } catch (e) {
      setError(message(e))
      // The window may have been closed; show the session's real state. If that lookup fails
      // too, keep the session (and its id) so Cancel or another try still works.
      getLogin(session.id)
        .then(onSession)
        .catch(() => undefined)
    } finally {
      setBusy(false)
    }
  }

  async function forget() {
    if (!session) return
    forgottenRef.current = session.id
    setBusy(true)
    setError('')
    try {
      await cancelLogin(session.id)
    } catch {
      // Already gone on the server (expired and pruned): nothing left to cancel.
    } finally {
      onSession(null)
      setBusy(false)
    }
  }

  const unavailable = availability !== null && !availability.available
  const host = hostOf(url)
  let body
  if (session?.state === 'waiting') {
    body = (
      <>
        <p role="status">
          A browser window is open on {session.host}. Log in there, then come back and press "I'm logged in".
        </p>
        <div className="cf-actions">
          <Button type="button" variant="primary" disabled={busy} onClick={() => void capture()}>
            I'm logged in
          </Button>
          <Button type="button" variant="secondary" disabled={busy} onClick={() => void forget()}>
            Cancel
          </Button>
        </div>
      </>
    )
  } else if (session?.state === 'captured') {
    body = (
      <>
        <p role="status">
          Logged in to {session.host}: {session.cookie_count} {session.cookie_count === 1 ? 'cookie' : 'cookies'} (
          {session.cookie_names.join(', ')}). Some cookies expire{' '}
          {session.expires_at
            ? new Date(session.expires_at).toLocaleString()
            : 'within an hour of opening the login window'}
          . gocrawl keeps the cookies in memory only and sends them to {session.host} only.
        </p>
        {loginExpired(session) && (
          <Notice tone="warning">Some login cookies have expired. If the crawl isn't logged in, log in again.</Notice>
        )}
        <Button type="button" variant="secondary" disabled={busy} onClick={() => void forget()}>
          Forget this login
        </Button>
      </>
    )
  } else {
    body = (
      <>
        {session?.state === 'closed' && (
          <Notice tone="warning">
            The browser window was closed before you pressed "I'm logged in", so nothing was captured. Log in again to retry.
          </Notice>
        )}
        {session?.state === 'expired' && (
          <Notice tone="warning">This login expired after an hour and its cookies were discarded. Log in again.</Notice>
        )}
        <Button
          type="button"
          variant="secondary"
          disabled={busy || availability === null || unavailable || blockedReason !== '' || host === ''}
          onClick={() => void start()}
        >
          Log in with browser…
        </Button>
        {unavailable ? (
          <p className="cf-hint">
            {availability?.reason || 'No Chromium-class browser found.'} Install Google Chrome, Chromium or Microsoft Edge
            to log in with a browser.
          </p>
        ) : blockedReason ? (
          <p className="cf-hint">{blockedReason}</p>
        ) : host === '' ? (
          <p className="cf-hint">Enter the site URL first.</p>
        ) : (
          <p className="cf-hint">
            Opens a browser window on the site URL in a fresh profile. Log in there (single sign-on, two-factor and
            CAPTCHAs work); the crawl then uses that session's cookies, sent to {host} only.
          </p>
        )}
      </>
    )
  }

  return (
    <fieldset className="cf-login">
      <legend>Browser login</legend>
      {body}
      {error && <Notice tone="error">{error}</Notice>}
    </fieldset>
  )
}
