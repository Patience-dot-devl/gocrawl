import { useEffect, useState } from 'react'
import { listAnalyzers, startCrawl } from '../api'
import type { AnalyzerInfo, StartCrawlParams } from '../types'

export default function CrawlForm({ onStarted }: { onStarted: (id: string) => void }) {
  const [analyzers, setAnalyzers] = useState<AnalyzerInfo[]>([])
  const [url, setUrl] = useState('')
  const [depth, setDepth] = useState('')
  const [maxPages, setMaxPages] = useState('')
  const [concurrency, setConcurrency] = useState('')
  const [render, setRender] = useState('raw')
  const [specialized, setSpecialized] = useState(false)
  const [save, setSave] = useState(true)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listAnalyzers()
      .then((r) => setAnalyzers(r.analyzers))
      .catch((e) => setError(String(e)))
  }, [])

  function toggleAnalyzer(name: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!url.trim()) return
    setSubmitting(true)
    setError('')
    const params: StartCrawlParams = {
      url: url.trim(),
      render,
      specialized,
      save,
    }
    if (depth) params.depth = Number(depth)
    if (maxPages) params.max_pages = Number(maxPages)
    if (concurrency) params.concurrency = Number(concurrency)
    if (selected.size > 0) params.analyzers = Array.from(selected)
    try {
      const job = await startCrawl(params)
      onStarted(job.id)
    } catch (err) {
      setError(String(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="card crawl-form" onSubmit={handleSubmit}>
      <h2>New crawl</h2>
      <label>
        Seed URL
        <input
          type="text"
          placeholder="https://example.com"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          required
        />
      </label>

      <div className="grid">
        <label>
          Depth
          <input type="number" min="0" placeholder="unlimited" value={depth} onChange={(e) => setDepth(e.target.value)} />
        </label>
        <label>
          Max pages
          <input type="number" min="1" placeholder="500" value={maxPages} onChange={(e) => setMaxPages(e.target.value)} />
        </label>
        <label>
          Concurrency
          <input type="number" min="1" placeholder="4" value={concurrency} onChange={(e) => setConcurrency(e.target.value)} />
        </label>
        <label>
          Render
          <select value={render} onChange={(e) => setRender(e.target.value)}>
            <option value="raw">raw</option>
            <option value="headless">headless</option>
          </select>
        </label>
      </div>

      <label className="checkbox">
        <input type="checkbox" checked={specialized} onChange={(e) => setSpecialized(e.target.checked)} />
        Enable specialized checks (AEO/GEO heuristics, WordPress security probes)
      </label>
      <label className="checkbox">
        <input type="checkbox" checked={save} onChange={(e) => setSave(e.target.checked)} />
        Save to history when finished
      </label>

      {analyzers.length > 0 && (
        <fieldset className="analyzers">
          <legend>Analyzers (none checked = run all)</legend>
          <div className="analyzer-grid">
            {analyzers.map((a) => (
              <label key={a.name} className="checkbox" title={a.description}>
                <input type="checkbox" checked={selected.has(a.name)} onChange={() => toggleAnalyzer(a.name)} />
                {a.name}
              </label>
            ))}
          </div>
        </fieldset>
      )}

      {error && <p className="error">{error}</p>}
      <button type="submit" disabled={submitting}>
        {submitting ? 'Starting…' : 'Start crawl'}
      </button>
    </form>
  )
}
