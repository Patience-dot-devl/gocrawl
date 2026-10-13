import { useEffect, useState } from 'react'
import { listAnalyzers, startCrawl } from '../api/crawls'
import type { AnalyzerInfo, StartCrawlParams } from '../types/crawls'
import Button from './ui/Button'
import Notice from './ui/Notice'
import './CrawlForm.css'

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
  const [ignoreExternalTagging, setIgnoreExternalTagging] = useState(true)
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
  const [cookieAuth, setCookieAuth] = useState(false)
  const [cookie, setCookie] = useState('')
  const [customHeaders, setCustomHeaders] = useState(false)
  const [headers, setHeaders] = useState('')
  const [headerProfile, setHeaderProfile] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listAnalyzers()
      .then((r) => setAnalyzers(r.analyzers))
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
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

  function selectAllAnalyzers() {
    setSelected(new Set(analyzers.map((a) => a.name)))
  }

  function selectNoAnalyzers() {
    setSelected(new Set())
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
      ignore_external_tagging: ignoreExternalTagging,
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
    if (cookieAuth && cookie.trim()) params.cookie = cookie.trim()
    if (customHeaders) {
      if (headers.trim()) params.headers = splitList(headers)
      if (headerProfile.trim()) params.header_profile = headerProfile.trim()
    }
    try {
      const job = await startCrawl(params)
      onStarted(job.id)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
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
          placeholder="https://example.com"
          name="url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          required
        />
      </label>

      <fieldset className="cf-section">
        <legend>Crawl behavior</legend>
        <div className="cf-grid">
          <label className="cf-field">
            <span className="cf-label">Depth</span>
            <input name="depth" type="number" min="0" placeholder="Unlimited" value={depth} onChange={(e) => setDepth(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Max pages</span>
            <input name="max_pages" type="number" min="0" placeholder="500" value={maxPages} onChange={(e) => setMaxPages(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Concurrency</span>
            <input name="concurrency" type="number" min="1" placeholder="4" value={concurrency} onChange={(e) => setConcurrency(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Rate limit (requests per second)</span>
            <input name="rate" type="number" min="0" step="any" placeholder="Unlimited" value={rate} onChange={(e) => setRate(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Max duration</span>
            <input name="max_duration" type="text" placeholder="Unlimited, e.g. 90m" value={maxDuration} onChange={(e) => setMaxDuration(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Render</span>
            <select name="render" value={render} onChange={(e) => setRender(e.target.value)}>
              <option value="raw">Raw HTML</option>
              <option value="headless">Headless browser</option>
            </select>
          </label>
        </div>
        <label className="cf-check">
          <input name="save" type="checkbox" checked={save} onChange={(e) => setSave(e.target.checked)} />
          Save to Crawls when finished
        </label>
      </fieldset>

      <fieldset className="cf-section">
        <legend>Scope</legend>
        <label className="cf-check">
          <input name="respect_robots" type="checkbox" checked={respectRobots} onChange={(e) => setRespectRobots(e.target.checked)} />
          Respect robots.txt
        </label>
        <label className="cf-check">
          <input name="subdomains" type="checkbox" checked={subdomains} onChange={(e) => setSubdomains(e.target.checked)} />
          Follow links to subdomains
        </label>
        <label className="cf-check">
          <input name="follow_external" type="checkbox" checked={followExternal} onChange={(e) => setFollowExternal(e.target.checked)} />
          Crawl links that leave the site
        </label>
      </fieldset>

      <fieldset className="cf-section">
        <legend>Analysis</legend>
        <label className="cf-check">
          <input name="specialized" type="checkbox" checked={specialized} onChange={(e) => setSpecialized(e.target.checked)} />
          Specialized checks (AEO and GEO heuristics, WordPress security probes)
        </label>
        <label className="cf-check">
          <input name="security_audit" type="checkbox" checked={securityAudit} onChange={(e) => setSecurityAudit(e.target.checked)} />
          Security audit (TLS and certificate, cookies, response headers)
        </label>
        <label className="cf-check">
          <input
            type="checkbox"
            name="ignore_external_tagging"
            checked={ignoreExternalTagging}
            onChange={(e) => setIgnoreExternalTagging(e.target.checked)}
          />
          Ignore UTM tagging issues on links that leave the site
        </label>

        {analyzers.length > 0 && (
          <fieldset className="cf-analyzers">
            <legend>Analyzers</legend>
            <p className="cf-hint">None checked runs all of them.</p>
            <div className="cf-analyzer-actions">
              <Button onClick={selectAllAnalyzers}>Select all</Button>
              <Button onClick={selectNoAnalyzers}>Select none</Button>
            </div>
            <div className="cf-analyzer-grid">
              {analyzers.map((a) => (
                <label key={a.name} className="cf-check" title={a.description}>
                  <input type="checkbox" name="analyzers" value={a.name} checked={selected.has(a.name)} onChange={() => toggleAnalyzer(a.name)} />
                  <span className="mono">{a.name}</span>
                </label>
              ))}
            </div>
          </fieldset>
        )}
      </fieldset>

      <details className="cf-advanced">
        <summary>Advanced</summary>
        <div className="cf-grid cf-grid-wide">
          <label className="cf-field">
            <span className="cf-label">Include (regex, one per line)</span>
            <span className="cf-hint">
              Matched against the full normalized URL. The seed must match too, or the crawl returns zero pages. No
              trailing slash.
            </span>
            <textarea name="include" rows={2} placeholder="/blog" value={include} onChange={(e) => setInclude(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Exclude (regex, one per line)</span>
            <span className="cf-hint">Matched against the full URL.</span>
            <textarea name="exclude" rows={2} placeholder="/admin/" value={exclude} onChange={(e) => setExclude(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent</span>
            <input name="user_agent" type="text" value={userAgent} onChange={(e) => setUserAgent(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent pool (one per line)</span>
            <textarea name="user_agents" rows={2} value={userAgents} onChange={(e) => setUserAgents(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent rotation</span>
            <select name="user_agent_rotation" value={userAgentRotation} onChange={(e) => setUserAgentRotation(e.target.value)}>
              <option value="off">Off</option>
              <option value="round-robin">Round robin</option>
              <option value="random">Random</option>
            </select>
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy</span>
            <input name="proxy" type="text" placeholder="http(s):// or socks5://" value={proxy} onChange={(e) => setProxy(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy pool (one per line)</span>
            <textarea name="proxies" rows={2} value={proxies} onChange={(e) => setProxies(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy rotation</span>
            <select name="proxy_rotation" value={proxyRotation} onChange={(e) => setProxyRotation(e.target.value)}>
              <option value="off">Off</option>
              <option value="round-robin">Round robin</option>
              <option value="random">Random</option>
              <option value="sticky-host">Sticky per host</option>
            </select>
          </label>
          <label className="cf-field">
            <span className="cf-label">Basic Auth username</span>
            <input name="basic_auth_user" type="text" autoComplete="off" value={basicAuthUser} onChange={(e) => setBasicAuthUser(e.target.value)} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Basic Auth password</span>
            <input name="basic_auth_password" type="password" autoComplete="new-password" value={basicAuthPass} onChange={(e) => setBasicAuthPass(e.target.value)} />
          </label>
        </div>

        <label className="cf-check">
          <input name="cookie_auth" type="checkbox" checked={cookieAuth} onChange={(e) => setCookieAuth(e.target.checked)} />
          The site needs a session cookie (for example a Shopify storefront password page)
        </label>
        {cookieAuth && (
          <label className="cf-field">
            <span className="cf-label">Cookie header</span>
            <span className="cf-hint">
              Paste the session cookie from a browser that is already past the gate (DevTools, Application or Storage,
              Cookies). Sent only to the seed host. Not supported with headless rendering.
            </span>
            <textarea
              rows={2}
              className="mono"
              placeholder="_shopify_essential=:AZ_..."
              name="cookie"
              autoComplete="off"
              value={cookie}
              onChange={(e) => setCookie(e.target.value)}
            />
          </label>
        )}

        <label className="cf-check">
          <input name="custom_headers" type="checkbox" checked={customHeaders} onChange={(e) => setCustomHeaders(e.target.checked)} />
          Send custom request headers (for example a Shopify crawler access signature)
        </label>
        {customHeaders && (
          <>
            <label className="cf-field">
              <span className="cf-label">Header profile</span>
              <span className="cf-hint">
                Name of a file at ~/.gocrawl/headers/&lt;name&gt;.headers on this machine: the way to use a stored
                secret without pasting it here.
              </span>
              <input name="header_profile" type="text" placeholder="uk" value={headerProfile} onChange={(e) => setHeaderProfile(e.target.value)} />
            </label>
            <label className="cf-field">
              <span className="cf-label">Extra headers (one &quot;Name: value&quot; per line)</span>
              <span className="cf-hint">
                Sent only to the seed host. They override the profile by name. Not supported with headless rendering.
              </span>
              <textarea
                rows={3}
                className="mono"
                placeholder="X-Access-Token: abc123"
                name="headers"
                autoComplete="off"
                value={headers}
                onChange={(e) => setHeaders(e.target.value)}
              />
            </label>
          </>
        )}
      </details>

      {error && <Notice tone="error">{error}</Notice>}
      <div className="cf-actions">
        <Button type="submit" variant="go" disabled={submitting}>
          {submitting ? 'Starting…' : 'Start crawl'}
        </Button>
      </div>
    </form>
  )
}
