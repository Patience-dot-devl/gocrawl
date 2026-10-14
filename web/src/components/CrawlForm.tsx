import { useEffect, useState } from 'react'
import { listAnalyzers, startCrawl } from '../api/crawls'
import { listHeaderProfiles } from '../api/profiles'
import type { AnalyzerInfo, StartCrawlParams } from '../types/crawls'
import CrawlFields from './crawlform/CrawlFields'
import { blankValues, splitList, toParams, type FormValues } from './crawlform/values'
import SitemapPreviewLine from './progress/SitemapPreviewLine'
import { DEFAULT_MAX_PAGES } from './progress/tapeText'
import Button from './ui/Button'
import Notice from './ui/Notice'
import './CrawlForm.css'

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

export default function CrawlForm({ onStarted }: { onStarted: (id: string) => void }) {
  const [values, setValues] = useState<FormValues>(blankValues)
  const [analyzers, setAnalyzers] = useState<AnalyzerInfo[]>([])
  const [profiles, setProfiles] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listAnalyzers()
      .then((r) => setAnalyzers(r.analyzers))
      .catch((e: unknown) => setError(message(e)))
    listHeaderProfiles()
      .then((r) => setProfiles(r.profiles))
      .catch((e: unknown) => setError(message(e)))
  }, [])

  function update(patch: Partial<FormValues>) {
    setValues((v) => ({ ...v, ...patch }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!values.url.trim()) return
    if (values.bearerToken.trim() && values.basicAuthUser.trim()) {
      setError('Use a bearer token or Basic Auth, not both: they set the same Authorization header.')
      return
    }
    setSubmitting(true)
    setError('')
    const params: StartCrawlParams = { ...toParams(values, 'crawl'), url: values.url.trim(), save: values.save }
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
      <CrawlFields values={values} onChange={update} mode="crawl" analyzers={analyzers} profiles={profiles} />
      {error && <Notice tone="error">{error}</Notice>}
      <div className="cf-actions">
        <Button type="submit" variant="go" disabled={submitting}>
          {submitting ? 'Starting…' : 'Start crawl'}
        </Button>
      </div>
    </form>
  )
}
