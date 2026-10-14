import { useEffect, useMemo, useState } from 'react'
import { ApiError } from '../api/client'
import { cancelRedirectCheck, getRedirectCheck, redirectExportURL } from '../api/redirects'
import Button from '../components/ui/Button'
import Notice from '../components/ui/Notice'
import StatusCode from '../components/ui/StatusCode'
import { usePageTitle } from '../pageTitle'
import { href } from '../routes'
import type { RedirectCheck, RedirectScope, RedirectVerdict } from '../types/redirects'
import { STATUS_LABEL } from './redirects/status'
import './redirects.css'

const POLL_MS = 1500

const VERDICTS: RedirectVerdict[] = ['ok', 'warning', 'error', 'skipped-external', 'skipped-dynamic']
const VERDICT_LABEL: Record<RedirectVerdict, string> = {
  ok: 'OK',
  warning: 'Warning',
  error: 'Error',
  'skipped-external': 'Skipped: external',
  'skipped-dynamic': 'Skipped: dynamic pattern',
}
// Verdicts are shown as text plus a glyph in the ink color: color is reserved for HTTP status.
const VERDICT_GLYPH: Record<RedirectVerdict, string> = {
  ok: '\u2713',
  warning: '\u25B2',
  error: '\u2715',
  'skipped-external': '\u2013',
  'skipped-dynamic': '\u2013',
}
const SCOPES: Exclude<RedirectScope, ''>[] = ['in-scope', 'external', 'dynamic-pattern']
const SCOPE_LABEL: Record<Exclude<RedirectScope, ''>, string> = {
  'in-scope': 'In scope',
  external: 'External',
  'dynamic-pattern': 'Dynamic pattern',
}

function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function Status({ code }: { code: number }) {
  return code > 0 ? <StatusCode code={code} /> : <span className="redirect-muted">—</span>
}

// A server error as a sentence: first letter capital, one full stop at the end.
function sentence(text: string): string {
  const t = text.trim().replace(/[.\s]+$/, '')
  return t ? `${t.charAt(0).toUpperCase()}${t.slice(1)}.` : ''
}

function yesNo(b: boolean): string {
  return b ? 'yes' : 'no'
}

export default function RedirectView({ id }: { id: string }) {
  const [check, setCheck] = useState<RedirectCheck | null>(null)
  // pollError is the last failed poll or cancel (shown above the last good view); notFound is a
  // 404 before anything loaded.
  const [error, setError] = useState('')
  const [notFound, setNotFound] = useState(false)
  const [nonce, setNonce] = useState(0)
  const [canceling, setCanceling] = useState(false)
  const [verdict, setVerdict] = useState<RedirectVerdict | ''>('')
  const [scope, setScope] = useState<RedirectScope | 'all'>('all')
  usePageTitle(notFound ? 'Redirect check not found' : check ? `${check.domain} - Redirect check` : 'Redirect check')

  useEffect(() => {
    let stopped = false
    let timer: number | undefined
    let live = false
    let loaded = false
    async function poll() {
      try {
        const c = await getRedirectCheck(id)
        if (stopped) return
        loaded = true
        live = c.status === 'running'
        setCheck(c)
        setError('')
        if (live) timer = window.setTimeout(poll, POLL_MS)
      } catch (e) {
        if (stopped) return
        if (!loaded && e instanceof ApiError && e.status === 404) {
          setNotFound(true)
          return
        }
        setError(errorText(e))
        // Keep the last view; while the check is running, try again.
        if (live) timer = window.setTimeout(poll, POLL_MS)
      }
    }
    void poll()
    return () => {
      stopped = true
      window.clearTimeout(timer)
    }
  }, [id, nonce])

  const rows = check?.rows
  const verdictCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const r of rows ?? []) counts[r.verdict] = (counts[r.verdict] ?? 0) + 1
    return counts
  }, [rows])
  const shown = useMemo(
    () =>
      (rows ?? [])
        .map((row, index) => ({ row, index }))
        .filter(({ row }) => (verdict === '' || row.verdict === verdict) && (scope === 'all' || row.scope === scope)),
    [rows, verdict, scope],
  )

  async function onCancel() {
    setCanceling(true)
    try {
      await cancelRedirectCheck(id)
    } catch (e) {
      setError(errorText(e))
    } finally {
      setCanceling(false)
      // Pick up the new state now instead of waiting for the next poll (a 409 means it already ended).
      setNonce((n) => n + 1)
    }
  }

  const back = <a href={href({ view: 'redirects' })}>Back to Redirects</a>

  if (notFound) {
    return (
      <div className="redirects">
        <h1>Redirect check not found</h1>
        <p>Redirect checks stay in memory until gocrawl serve stops, so it may have ended with a restart.</p>
        <p>{back}</p>
      </div>
    )
  }
  if (!check) {
    return error ? (
      <Notice tone="error">
        <div className="redirect-error">{error}</div>
      </Notice>
    ) : (
      <p className="muted">Loading…</p>
    )
  }

  return (
    <div className="redirects">
      <h1>Redirect check: {check.domain}</h1>
      <p className="redirect-muted" role="status">
        {check.file_name || 'Redirect rules'} · {check.total.toLocaleString()} rules · {STATUS_LABEL[check.status] ?? check.status}
      </p>
      {error && (
        <Notice tone="error">
          <div className="redirect-error">{error}</div>
        </Notice>
      )}

      {check.status === 'running' && (
        <div className="redirect-progress">
          <progress max={Math.max(check.total, 1)} value={check.done} aria-label="Rules checked" />
          <span>
            {check.done.toLocaleString()} of {check.total.toLocaleString()} rules checked
          </span>
          <Button variant="stop" type="button" disabled={canceling} onClick={onCancel}>
            Cancel check
          </Button>
        </div>
      )}

      {check.status === 'canceled' && (
        <Notice tone="info" action={back}>
          Check canceled after {check.done.toLocaleString()} of {check.total.toLocaleString()} rules. A canceled check keeps
          no results; run it again for the full table.
        </Notice>
      )}

      {check.status === 'error' && (
        <Notice tone="error" action={back}>
          <p>
            <strong>The check could not finish.</strong>
          </p>
          <div className="redirect-error">{sentence(check.error ?? '')}</div>
          <p>Check the domain and the sitemap URL, then run the check again.</p>
        </Notice>
      )}

      {check.status === 'done' && rows && (
        <>
          {rows.length > 0 && rows.every((r) => r.verdict === 'skipped-external') && (
            <Notice tone="warning">
              Every rule was skipped as external. Check that the domain matches the URLs in the file, and that the file
              uses full URLs (https://example.com/old), not paths.
            </Notice>
          )}
          <p>
            {VERDICTS.filter((v) => verdictCounts[v])
              .map((v) => `${verdictCounts[v].toLocaleString()} ${VERDICT_LABEL[v].toLowerCase()}`)
              .join(' · ')}
          </p>
          <div className="redirect-filters">
            <label>
              Verdict
              <select name="verdict" value={verdict} onChange={(e) => setVerdict(e.target.value as RedirectVerdict | '')}>
                <option value="">All verdicts</option>
                {VERDICTS.map((v) => (
                  <option key={v} value={v}>
                    {VERDICT_LABEL[v]} ({(verdictCounts[v] ?? 0).toLocaleString()})
                  </option>
                ))}
              </select>
            </label>
            <label>
              Scope
              <select name="scope" value={scope} onChange={(e) => setScope(e.target.value as RedirectScope | 'all')}>
                <option value="all">All scopes</option>
                {SCOPES.map((s) => (
                  <option key={s} value={s}>
                    {SCOPE_LABEL[s]}
                  </option>
                ))}
              </select>
            </label>
            <span className="redirect-muted num">
              Showing {shown.length.toLocaleString()} of {rows.length.toLocaleString()} rules
            </span>
            <a className="btn btn-secondary" href={redirectExportURL(id)}>
              Export CSV
            </a>
          </div>

          <div className="redirect-table">
            <table>
              <thead>
                <tr>
                  <th scope="col">#</th>
                  <th scope="col">Original URL</th>
                  <th scope="col">Redirect to</th>
                  <th scope="col">Scope</th>
                  <th scope="col">Source status</th>
                  <th scope="col">Final URL</th>
                  <th scope="col">Target status</th>
                  <th scope="col">In sitemap (source / target)</th>
                  <th scope="col">Verdict</th>
                  <th scope="col">Notes</th>
                </tr>
              </thead>
              <tbody>
                {shown.map(({ row, index }) => (
                  <tr key={index}>
                    <td>{index + 1}</td>
                    <td className="redirect-url" title={row.original}>
                      {row.original}
                    </td>
                    <td className="redirect-url" title={row.target}>
                      {row.target}
                    </td>
                    <td>{row.scope ? SCOPE_LABEL[row.scope] : '—'}</td>
                    <td>
                      <Status code={row.source_status} />
                    </td>
                    <td className="redirect-url" title={row.source_final_url}>
                      {row.source_final_url || '—'}
                    </td>
                    <td>
                      <Status code={row.target_status} />
                    </td>
                    <td>
                      {row.scope === 'in-scope' ? `${yesNo(row.original_in_sitemap)} / ${yesNo(row.target_in_sitemap)}` : '—'}
                    </td>
                    <td className={row.verdict === 'error' ? 'redirect-verdict-error' : undefined}>
                      <span aria-hidden="true">{VERDICT_GLYPH[row.verdict]}</span> {VERDICT_LABEL[row.verdict]}
                    </td>
                    <td className="redirect-notes">{(row.notes ?? []).join('; ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  )
}
