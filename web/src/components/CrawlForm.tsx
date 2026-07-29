import { useEffect, useState } from 'react'
import { listAnalyzers, startCrawl } from '../api'
import type { AnalyzerInfo, StartCrawlParams } from '../types'

export default function CrawlForm({ onStarted }: { onStarted: (id: string) => void }) {
  const [analyzers, setAnalyzers] = useState<AnalyzerInfo[]>([])
  const [url, setUrl] = useState('')
  const [depth, setDepth] = useState('')
  const [maxPages, setMaxPages] = useState('')
  const [concurrency, setConcurrency] = useState('')
  const [rate, setRate] = useState('')
  const [maxDuration, setMaxDuration] = useState('')
  const [render, setRender] = useState('raw')
  const [respectRobots, setRespectRobots] = useState(true)
  const [subdomains, setSubdomains] = useState(false)
  const [followExternal, setFollowExternal] = useState(false)
  const [specialized, setSpecialized] = useState(false)
  const [securityAudit, setSecurityAudit] = useState(false)
  const [save, setSave] = useState(true)
  const [include, setInclude] = useState('')
  const [exclude, setExclude] = useState('')
  const [userAgent, setUserAgent] = useState('')
  const [userAgents, setUserAgents] = useState('')
  const [userAgentRotation, setUserAgentRotation] = useState('round-robin')
  const [proxy, setProxy] = useState('')
  const [proxies, setProxies] = useState('')
  const [proxyRotation, setProxyRotation] = useState('round-robin')
  const [basicAuthUser, setBasicAuthUser] = useState('')
  const [basicAuthPass, setBasicAuthPass] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listAnalyzers()
      .then((r) => setAnalyzers(r.analyzers))
      .catch((e) => setError(String(e)))
  }, [])

  // Newline-separated rather than comma-separated: a comma is a completely ordinary character
  // in a User-Agent string ("... (KHTML, like Gecko) ...") and in a regex ("{2,4}"), so
  // splitting on it would silently corrupt otherwise-valid entries.
  function splitList(s: string): string[] {
    return s
      .split('\n')
      .map((v) => v.trim())
      .filter(Boolean)
  }

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
      respect_robots: respectRobots,
      subdomains,
      follow_external: followExternal,
      specialized,
      security_audit: securityAudit,
      save,
    }
    if (depth) params.depth = Number(depth)
    if (maxPages) params.max_pages = Number(maxPages)
    if (concurrency) params.concurrency = Number(concurrency)
    if (rate) params.rate = Number(rate)
    if (maxDuration.trim()) params.max_duration = maxDuration.trim()
    if (selected.size > 0) params.analyzers = Array.from(selected)
    if (include.trim()) params.include = splitList(include)
    if (exclude.trim()) params.exclude = splitList(exclude)
    if (userAgent.trim()) params.user_agent = userAgent.trim()
    if (userAgents.trim()) {
      params.user_agents = splitList(userAgents)
      params.user_agent_rotation = userAgentRotation
    }
    if (proxy.trim()) params.proxy = proxy.trim()
    if (proxies.trim()) {
      params.proxies = splitList(proxies)
      params.proxy_rotation = proxyRotation
    }
    if (basicAuthUser.trim()) params.basic_auth = `${basicAuthUser.trim()}:${basicAuthPass}`
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
          <input type="number" min="0" placeholder="500" value={maxPages} onChange={(e) => setMaxPages(e.target.value)} />
        </label>
        <label>
          Concurrency
          <input type="number" min="1" placeholder="4" value={concurrency} onChange={(e) => setConcurrency(e.target.value)} />
        </label>
        <label>
          Rate limit
          <input type="number" min="0" step="any" placeholder="unlimited" value={rate} onChange={(e) => setRate(e.target.value)} />
        </label>
        <label>
          Max duration
          <input type="text" placeholder="unlimited, e.g. 90m" value={maxDuration} onChange={(e) => setMaxDuration(e.target.value)} />
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
        <input type="checkbox" checked={respectRobots} onChange={(e) => setRespectRobots(e.target.checked)} />
        Respect robots.txt
      </label>
      <label className="checkbox">
        <input type="checkbox" checked={subdomains} onChange={(e) => setSubdomains(e.target.checked)} />
        Follow links to subdomains
      </label>
      <label className="checkbox">
        <input type="checkbox" checked={followExternal} onChange={(e) => setFollowExternal(e.target.checked)} />
        Crawl links that leave the seed host
      </label>
      <label className="checkbox">
        <input type="checkbox" checked={specialized} onChange={(e) => setSpecialized(e.target.checked)} />
        Enable specialized checks (AEO/GEO heuristics, WordPress security probes)
      </label>
      <label className="checkbox">
        <input type="checkbox" checked={securityAudit} onChange={(e) => setSecurityAudit(e.target.checked)} />
        Enable security audit (TLS/certificate, cookies, response headers)
      </label>
      <label className="checkbox">
        <input type="checkbox" checked={save} onChange={(e) => setSave(e.target.checked)} />
        Save to history when finished
      </label>

      <details className="advanced">
        <summary>Advanced</summary>
        <div className="grid">
          <label>
            Include (regex, one per line, matched against the full normalized URL — the seed must
            match too, or the crawl returns zero pages; no trailing slash)
            <textarea rows={2} placeholder="/blog" value={include} onChange={(e) => setInclude(e.target.value)} />
          </label>
          <label>
            Exclude (regex, one per line, matched against the full URL)
            <textarea rows={2} placeholder="/admin/" value={exclude} onChange={(e) => setExclude(e.target.value)} />
          </label>
          <label>
            User-Agent
            <input type="text" value={userAgent} onChange={(e) => setUserAgent(e.target.value)} />
          </label>
          <label>
            User-Agent pool (one per line)
            <textarea rows={2} value={userAgents} onChange={(e) => setUserAgents(e.target.value)} />
          </label>
          <label>
            User-Agent rotation
            <select value={userAgentRotation} onChange={(e) => setUserAgentRotation(e.target.value)}>
              <option value="off">off</option>
              <option value="round-robin">round-robin</option>
              <option value="random">random</option>
            </select>
          </label>
          <label>
            Proxy
            <input type="text" placeholder="http(s):// or socks5://" value={proxy} onChange={(e) => setProxy(e.target.value)} />
          </label>
          <label>
            Proxy pool (one per line)
            <textarea rows={2} value={proxies} onChange={(e) => setProxies(e.target.value)} />
          </label>
          <label>
            Proxy rotation
            <select value={proxyRotation} onChange={(e) => setProxyRotation(e.target.value)}>
              <option value="off">off</option>
              <option value="round-robin">round-robin</option>
              <option value="random">random</option>
              <option value="sticky-host">sticky-host</option>
            </select>
          </label>
          <label>
            Basic Auth username
            <input type="text" value={basicAuthUser} onChange={(e) => setBasicAuthUser(e.target.value)} />
          </label>
          <label>
            Basic Auth password
            <input type="password" value={basicAuthPass} onChange={(e) => setBasicAuthPass(e.target.value)} />
          </label>
        </div>
      </details>

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
