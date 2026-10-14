import { useEffect, useId, useState, type FormEvent } from 'react'
import { listRedirectChecks, startRedirectCheck } from '../api/redirects'
import Button from '../components/ui/Button'
import Notice from '../components/ui/Notice'
import { usePageTitle } from '../pageTitle'
import { href, navigate } from '../routes'
import type { RedirectCheck } from '../types/redirects'
import './redirects.css'

import { STATUS_LABEL } from './redirects/status'

function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function formatStarted(s?: string): string {
  if (!s) return '—'
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? s : d.toLocaleString()
}

export default function RedirectsView() {
  usePageTitle('Redirects')
  const uid = useId()
  const [file, setFile] = useState<File | null>(null)
  const [domain, setDomain] = useState('')
  const [sitemapURL, setSitemapURL] = useState('')
  const [rate, setRate] = useState('')
  const [concurrency, setConcurrency] = useState('4')
  const [adaptiveDelay, setAdaptiveDelay] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [checks, setChecks] = useState<RedirectCheck[] | null>(null)
  const [listError, setListError] = useState('')

  useEffect(() => {
    listRedirectChecks()
      .then((r) => setChecks(r.checks))
      .catch((e) => setListError(errorText(e)))
  }, [])

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!file) {
      setError('Choose the redirect-rule CSV file first.')
      return
    }
    setSubmitting(true)
    setError('')
    try {
      const check = await startRedirectCheck({
        file,
        domain: domain.trim(),
        sitemapURL: sitemapURL.trim() || undefined,
        rate: rate.trim() === '' ? undefined : Number(rate),
        concurrency: concurrency.trim() === '' ? undefined : Number(concurrency),
        adaptiveDelay,
      })
      navigate({ view: 'redirect', id: check.id })
    } catch (err) {
      setError(errorText(err))
      setSubmitting(false)
    }
  }

  return (
    <div className="redirects">
      <h1>Check redirects</h1>
      <p className="redirect-hint">
        Checks every rule in a HubSpot URL Redirects export against the live site: does the old URL still redirect to the
        right place, is the target a live page, and do both agree with the sitemap.
      </p>

      <form className="redirect-form" onSubmit={onSubmit}>
        <div className="redirect-field">
          <label htmlFor={`${uid}-file`}>Redirect rules (CSV)</label>
          <input
            id={`${uid}-file`}
            aria-describedby={`${uid}-file-hint`}
            type="file"
            name="file"
            accept=".csv,text/csv"
            required
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
          <span className="redirect-hint" id={`${uid}-file-hint`}>
            A HubSpot URL Redirects export, up to 10 MB. A path such as /old is checked on https:// plus the domain below.
          </span>
        </div>
        <div className="redirect-field">
          <label htmlFor={`${uid}-domain`}>Domain</label>
          <input
            id={`${uid}-domain`}
            aria-describedby={`${uid}-domain-hint`}
            type="text"
            name="domain"
            required
            placeholder="example.com"
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
          />
          <span className="redirect-hint" id={`${uid}-domain-hint`}>
            The host only, such as example.com, with no https:// and no port. Subdomains count as in scope. Rules that
            point at other domains are skipped.
          </span>
        </div>
        <div className="redirect-field">
          <label htmlFor={`${uid}-sitemap`}>Sitemap URL (optional)</label>
          <input
            id={`${uid}-sitemap`}
            aria-describedby={`${uid}-sitemap-hint`}
            type="url"
            name="sitemap_url"
            placeholder="https://example.com/sitemap.xml"
            value={sitemapURL}
            onChange={(e) => setSitemapURL(e.target.value)}
          />
          <span className="redirect-hint" id={`${uid}-sitemap-hint`}>
            Leave empty to try /sitemap.xml and /sitemap_index.xml on https://. Fill it in for a site without https.
          </span>
        </div>
        <div className="redirect-row">
          <label>
            Rate (requests per second)
            <input
              type="number"
              name="rate"
              min={0}
              step="any"
              placeholder="Unlimited"
              value={rate}
              onChange={(e) => setRate(e.target.value)}
            />
          </label>
          <label>
            Concurrency
            <input
              type="number"
              name="concurrency"
              min={1}
              max={16}
              step={1}
              value={concurrency}
              onChange={(e) => setConcurrency(e.target.value)}
            />
          </label>
        </div>
        <label className="redirect-inline">
          <input
            type="checkbox"
            name="adaptive_delay"
            checked={adaptiveDelay}
            onChange={(e) => setAdaptiveDelay(e.target.checked)}
          />
          Slow down when the site answers 429 or 503 (adaptive delay)
        </label>
        {error && (
          <Notice tone="error">
            <div className="redirect-error">{error}</div>
          </Notice>
        )}
        <div>
          <Button variant="primary" type="submit" disabled={submitting}>
            {submitting ? 'Starting…' : 'Check redirects'}
          </Button>
        </div>
      </form>

      <h2>Checks this session</h2>
      {listError && (
        <Notice tone="error">
          <div className="redirect-error">Could not load the checks: {listError}</div>
        </Notice>
      )}
      {checks && checks.length === 0 && (
        <p className="muted">No redirect checks yet. Results stay in memory until gocrawl serve stops.</p>
      )}
      {checks && checks.length > 0 && (
        <div className="redirect-table">
          <table>
            <thead>
              <tr>
                <th scope="col">File</th>
                <th scope="col">Domain</th>
                <th scope="col">Status</th>
                <th scope="col">Rules checked</th>
                <th scope="col">Started</th>
              </tr>
            </thead>
            <tbody>
              {checks.map((c) => (
                <tr key={c.id}>
                  <td>
                    <a href={href({ view: 'redirect', id: c.id })}>{c.file_name || 'Redirect rules'}</a>
                  </td>
                  <td>{c.domain}</td>
                  <td>{STATUS_LABEL[c.status] ?? c.status}</td>
                  <td className="num">
                    {c.done.toLocaleString()} of {c.total.toLocaleString()}
                  </td>
                  <td className="redirect-muted">{formatStarted(c.started_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
