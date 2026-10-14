import { useEffect, useRef, useState } from 'react'
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
}

export default function LoginPanel({ url, blockedReason, session, onSession, onStarted }: Props) {
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
  useEffect(() => {
    if (!waitingId) return
    const timer = window.setInterval(() => {
      getLogin(waitingId)
        .then((s) => {
          if (s.state !== 'waiting') onSession(s)
        })
        .catch(() => undefined)
    }, POLL_MS)
    return () => window.clearInterval(timer)
  }, [waitingId, onSession])

  // Leaving the form while a window waits closes that window. A captured login is kept: the
  // crawl the form just started uses it.
  const waitingRef = useRef('')
  useEffect(() => {
    waitingRef.current = waitingId
  }, [waitingId])
  useEffect(
    () => () => {
      if (waitingRef.current) void cancelLogin(waitingRef.current).catch(() => undefined)
    },
    [],
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
      // The window may have been closed; show the session's real state.
      getLogin(session.id)
        .then(onSession)
        .catch(() => onSession(null))
    } finally {
      setBusy(false)
    }
  }

  async function forget() {
    if (!session) return
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
    const expired = loginExpired(session)
    body = (
      <>
        {expired ? (
          <Notice tone="warning">Login expired — log in again. The cookies for {session.host} have expired.</Notice>
        ) : (
          <p role="status">
            Logged in to {session.host}: {session.cookie_count} {session.cookie_count === 1 ? 'cookie' : 'cookies'} (
            {session.cookie_names.join(', ')}). This login expires{' '}
            {session.expires_at
              ? new Date(session.expires_at).toLocaleString()
              : 'within an hour of opening the login window'}
            . gocrawl keeps the cookies in memory only and sends them to {session.host} only.
          </p>
        )}
        <Button type="button" variant="secondary" disabled={busy} onClick={() => void forget()}>
          {expired ? 'Clear this login' : 'Forget this login'}
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
