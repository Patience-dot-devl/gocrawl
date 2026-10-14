import { useEffect, useState } from 'react'
import { listAnalyzers, startCrawl } from '../api/crawls'
import { listHeaderProfiles } from '../api/profiles'
import type { AnalyzerInfo, StartCrawlParams } from '../types/crawls'
import type { LoginSession } from '../types/login'
import CrawlFields from './crawlform/CrawlFields'
import LoginPanel from './crawlform/LoginPanel'
import { LOGOUT_EXCLUDE, fromParams, hostOf, loginExpired, splitList, toParams, withLine, type FormValues } from './crawlform/values'
import { useSettings } from '../settings/context'
import SitemapPreviewLine from './progress/SitemapPreviewLine'
import { DEFAULT_MAX_PAGES } from './progress/tapeText'
import Button from './ui/Button'
import Notice from './ui/Notice'
import './CrawlForm.css'

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

export default function CrawlForm({ onStarted }: { onStarted: (id: string) => void }) {
  const { settings, saveDefaults } = useSettings()
  const defaults = settings?.crawl_defaults
  const [values, setValues] = useState<FormValues>(() => fromParams(defaults))
  const [dirty, setDirty] = useState(false)
  const [defaultsNote, setDefaultsNote] = useState('')
  // Settings load after the first render. Until the user touches the form, follow them.
  const [seenDefaults, setSeenDefaults] = useState(defaults)
  if (defaults !== seenDefaults) {
    setSeenDefaults(defaults)
    if (!dirty) setValues(fromParams(defaults))
  }
  const [analyzers, setAnalyzers] = useState<AnalyzerInfo[]>([])
  const [profiles, setProfiles] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [login, setLogin] = useState<LoginSession | null>(null)
  const loginBlocked =
    values.render === 'headless'
      ? 'Browser login works with raw rendering only. Set Render to raw first.'
      : values.cookieAuth && values.cookie.trim()
        ? 'Clear the cookie header first: the browser login supplies the cookies.'
        : ''

  function addLogoutExclude() {
    setDirty(true)
    setValues((v) => ({ ...v, exclude: withLine(v.exclude, LOGOUT_EXCLUDE) }))
  }

  useEffect(() => {
    listAnalyzers()
      .then((r) => setAnalyzers(r.analyzers))
      .catch((e: unknown) => setError(message(e)))
    listHeaderProfiles()
      .then((r) => setProfiles(r.profiles))
      .catch((e: unknown) => setError(message(e)))
  }, [])

  function update(patch: Partial<FormValues>) {
    setDirty(true)
    setDefaultsNote('')
    setValues((v) => ({ ...v, ...patch }))
  }

  async function handleSaveDefaults() {
    setError('')
    setDefaultsNote('')
    try {
      await saveDefaults(toParams(values, 'defaults'))
      setDefaultsNote('Saved. New crawls start with these settings. Passwords, cookies, tokens and inline headers are never saved.')
    } catch (err) {
      setError(`Couldn't save the defaults: ${message(err)}`)
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!values.url.trim()) return
    if (values.bearerToken.trim() && values.basicAuthUser.trim()) {
      setError('Use a bearer token or Basic Auth, not both: they set the same Authorization header.')
      return
    }
    if (login?.state === 'waiting') {
      setError('Finish the browser login first: press "I\'m logged in", or cancel it.')
      return
    }
    if (login?.state === 'captured' && loginExpired(login)) {
      setError('Login expired — log in again.')
      return
    }
    if (login?.state === 'captured' && hostOf(values.url) !== login.host) {
      setError(`The browser login is for ${login.host}, but the seed URL is on ${hostOf(values.url) || 'another host'}. Log in again for this site.`)
      return
    }
    setSubmitting(true)
    setError('')
    const params: StartCrawlParams = { ...toParams(values, 'crawl'), url: values.url.trim(), save: values.save }
    if (login?.state === 'captured') {
      params.login_session = login.id
      delete params.cookie
    }
    try {
      const job = await startCrawl(params)
      onStarted(job.id)
    } catch (err) {
      setError(message(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="cf" onSubmit={handleSubmit}>
      <h1>New crawl</h1>
      <label className="cf-field cf-url">
        <span className="cf-label">Site URL</span>
        <input
          type="text"
          inputMode="url"
          name="url"
          placeholder="https://example.com"
          value={values.url}
          onChange={(e) => update({ url: e.target.value })}
          required
        />
      </label>
      <SitemapPreviewLine
        url={values.url}
        include={splitList(values.include)}
        exclude={splitList(values.exclude)}
        subdomains={values.subdomains}
        maxPages={values.maxPages.trim() === '' ? DEFAULT_MAX_PAGES : Number(values.maxPages)}
        onRaiseBudget={(pages) => update({ maxPages: String(pages) })}
      />
      <CrawlFields
        values={values}
        onChange={update}
        mode="crawl"
        analyzers={analyzers}
        profiles={profiles}
        cookieLocked={login?.state === 'captured'}
        loginSlot={
          <LoginPanel
            url={values.url}
            blockedReason={loginBlocked}
            session={login}
            onSession={setLogin}
            onStarted={addLogoutExclude}
          />
        }
      />
      {error && <Notice tone="error">{error}</Notice>}
      {defaultsNote && <Notice tone="info">{defaultsNote}</Notice>}
      <div className="cf-actions">
        <Button type="submit" variant="go" disabled={submitting}>
          {submitting ? 'Starting…' : 'Start crawl'}
        </Button>
        <Button type="button" variant="secondary" onClick={() => void handleSaveDefaults()}>
          Save as defaults
        </Button>
      </div>
    </form>
  )
}
