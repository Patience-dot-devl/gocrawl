import type { ReactNode } from 'react'
import type { AnalyzerInfo } from '../../types/crawls'
import Button from '../ui/Button'
import type { FieldsMode, FormValues } from './values'
import '../CrawlForm.css'

interface Props {
  values: FormValues
  onChange: (patch: Partial<FormValues>) => void
  mode: FieldsMode
  analyzers: AnalyzerInfo[]
  profiles: string[]
  // Rendered after the cookie field in crawl mode: the browser-login panel.
  loginSlot?: ReactNode
  // True while a captured browser login supplies the Cookie header.
  cookieLocked?: boolean
}

// Every crawl option, in the New crawl form's markup (W1-VIS-6: fieldsets with sentence-case
// legends, cf- classes from CrawlForm.css), shared by New crawl and the Settings view's crawl
// defaults. The seed URL and the form's own actions stay in the caller.
export default function CrawlFields({ values: v, onChange, mode, analyzers, profiles, loginSlot, cookieLocked = false }: Props) {
  const crawl = mode === 'crawl'
  const selected = new Set(v.analyzers)
  const profileOptions = v.headerProfile && !profiles.includes(v.headerProfile) ? [...profiles, v.headerProfile] : profiles

  function toggleAnalyzer(name: string) {
    const next = new Set(selected)
    if (next.has(name)) next.delete(name)
    else next.add(name)
    onChange({ analyzers: Array.from(next) })
  }

  return (
    <>
      <fieldset className="cf-section">
        <legend>Crawl behavior</legend>
        <div className="cf-grid">
          <label className="cf-field">
            <span className="cf-label">Depth</span>
            <input name="depth" type="number" min="0" placeholder="Unlimited" value={v.depth} onChange={(e) => onChange({ depth: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Max pages</span>
            <input name="max_pages" type="number" min="0" placeholder="500" value={v.maxPages} onChange={(e) => onChange({ maxPages: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Concurrency</span>
            <input name="concurrency" type="number" min="1" placeholder="4" value={v.concurrency} onChange={(e) => onChange({ concurrency: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Rate limit (requests per second)</span>
            <input name="rate" type="number" min="0" step="any" placeholder="Unlimited" value={v.rate} onChange={(e) => onChange({ rate: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Max duration</span>
            <input name="max_duration" type="text" placeholder="Unlimited, e.g. 90m" value={v.maxDuration} onChange={(e) => onChange({ maxDuration: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Request timeout</span>
            <input name="timeout" type="text" placeholder="15s" value={v.timeout} onChange={(e) => onChange({ timeout: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Render</span>
            <select name="render" value={v.render} onChange={(e) => onChange({ render: e.target.value })}>
              <option value="raw">Raw HTML</option>
              <option value="headless">Headless browser</option>
            </select>
          </label>
        </div>
        <label className="cf-check">
          <input name="adaptive_delay" type="checkbox" checked={v.adaptiveDelay} onChange={(e) => onChange({ adaptiveDelay: e.target.checked })} />
          Slow down automatically when the site answers 429 or 503
        </label>
        {crawl && (
          <label className="cf-check">
            <input name="save" type="checkbox" checked={v.save} onChange={(e) => onChange({ save: e.target.checked })} />
            Save to Crawls when finished
          </label>
        )}
      </fieldset>

      <fieldset className="cf-section">
        <legend>Scope</legend>
        <label className="cf-check">
          <input name="respect_robots" type="checkbox" checked={v.respectRobots} onChange={(e) => onChange({ respectRobots: e.target.checked })} />
          Respect robots.txt
        </label>
        <label className="cf-check">
          <input name="subdomains" type="checkbox" checked={v.subdomains} onChange={(e) => onChange({ subdomains: e.target.checked })} />
          Follow links to subdomains
        </label>
        <label className="cf-check">
          <input name="follow_external" type="checkbox" checked={v.followExternal} onChange={(e) => onChange({ followExternal: e.target.checked })} />
          Crawl links that leave the site
        </label>
        <label className="cf-check">
          <input name="follow_nofollow" type="checkbox" checked={v.followNofollow} onChange={(e) => onChange({ followNofollow: e.target.checked })} />
          Follow links marked rel=&quot;nofollow&quot;
        </label>
        <label className="cf-check">
          <input name="strip_query" type="checkbox" checked={v.stripQuery} onChange={(e) => onChange({ stripQuery: e.target.checked })} />
          Ignore query strings (?a=1 and ?a=2 count as one URL; skips UTM and tracking analysis)
        </label>
      </fieldset>

      <fieldset className="cf-section">
        <legend>Analysis</legend>
        <label className="cf-check">
          <input name="specialized" type="checkbox" checked={v.specialized} onChange={(e) => onChange({ specialized: e.target.checked })} />
          Specialized checks (AEO and GEO heuristics, WordPress security probes)
        </label>
        <label className="cf-check">
          <input name="security_audit" type="checkbox" checked={v.securityAudit} onChange={(e) => onChange({ securityAudit: e.target.checked })} />
          Security audit (TLS and certificate, cookies, response headers)
        </label>
        <label className="cf-check">
          <input name="ignore_external_tagging"
            type="checkbox"
            checked={v.ignoreExternalTagging}
            onChange={(e) => onChange({ ignoreExternalTagging: e.target.checked })}
          />
          Ignore UTM tagging issues on links that leave the site
        </label>

        {analyzers.length > 0 && (
          <fieldset className="cf-analyzers">
            <legend>Analyzers</legend>
            <p className="cf-hint">None checked runs all of them.</p>
            <div className="cf-analyzer-actions">
              <Button onClick={() => onChange({ analyzers: analyzers.map((a) => a.name) })}>Select all</Button>
              <Button onClick={() => onChange({ analyzers: [] })}>Select none</Button>
            </div>
            <div className="cf-analyzer-grid">
              {analyzers.map((a) => (
                <label key={a.name} className="cf-check" title={a.description}>
                  <input name="analyzers" value={a.name} type="checkbox" checked={selected.has(a.name)} onChange={() => toggleAnalyzer(a.name)} />
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
            <textarea name="include" rows={2} placeholder="/blog" value={v.include} onChange={(e) => onChange({ include: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Exclude (regex, one per line)</span>
            <span className="cf-hint">Matched against the full URL.</span>
            <textarea name="exclude" rows={2} placeholder="/admin/" value={v.exclude} onChange={(e) => onChange({ exclude: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent</span>
            <input name="user_agent" type="text" value={v.userAgent} onChange={(e) => onChange({ userAgent: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent pool (one per line)</span>
            <textarea name="user_agents" rows={2} value={v.userAgents} onChange={(e) => onChange({ userAgents: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">User-Agent rotation</span>
            <select name="user_agent_rotation" value={v.userAgentRotation} onChange={(e) => onChange({ userAgentRotation: e.target.value })}>
              <option value="off">Off</option>
              <option value="round-robin">Round robin</option>
              <option value="random">Random</option>
            </select>
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy</span>
            <input name="proxy" type="text" placeholder="http(s):// or socks5://" value={v.proxy} onChange={(e) => onChange({ proxy: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy pool (one per line)</span>
            <textarea name="proxies" rows={2} value={v.proxies} onChange={(e) => onChange({ proxies: e.target.value })} />
          </label>
          <label className="cf-field">
            <span className="cf-label">Proxy rotation</span>
            <select name="proxy_rotation" value={v.proxyRotation} onChange={(e) => onChange({ proxyRotation: e.target.value })}>
              <option value="off">Off</option>
              <option value="round-robin">Round robin</option>
              <option value="random">Random</option>
              <option value="sticky-host">Sticky per host</option>
            </select>
          </label>
        </div>

        {crawl ? (
          <fieldset className="cf-section">
            <legend>Authentication</legend>
            <div className="cf-grid cf-grid-wide">
              <label className="cf-field">
                <span className="cf-label">Basic Auth username</span>
                <input autoComplete="off" name="basic_auth_user" type="text" value={v.basicAuthUser} onChange={(e) => onChange({ basicAuthUser: e.target.value })} />
              </label>
              <label className="cf-field">
                <span className="cf-label">Basic Auth password</span>
                <input autoComplete="new-password" name="basic_auth_password" type="password" value={v.basicAuthPass} onChange={(e) => onChange({ basicAuthPass: e.target.value })} />
              </label>
              <label className="cf-field">
                <span className="cf-label">Bearer token</span>
                <span className="cf-hint">
                  Sent as &quot;Authorization: Bearer …&quot; to the seed host only. Not together with Basic Auth or a custom
                  Authorization header. Raw rendering only.
                </span>
                <input name="bearer_token"
                  type="password"
                  autoComplete="off"
                  value={v.bearerToken}
                  onChange={(e) => onChange({ bearerToken: e.target.value })}
                />
              </label>
            </div>
            <label className="cf-check">
              <input name="cookie_auth"
                type="checkbox"
                checked={v.cookieAuth && !cookieLocked}
                disabled={cookieLocked}
                onChange={(e) => onChange({ cookieAuth: e.target.checked })}
              />
              The site needs a session cookie (for example a Shopify storefront password page)
            </label>
            {cookieLocked && <p className="cf-hint">Your browser login supplies the cookies for this crawl.</p>}
            {v.cookieAuth && !cookieLocked && (
              <label className="cf-field">
                <span className="cf-label">Cookie header</span>
                <span className="cf-hint">
                  Paste the session cookie from a browser that is already past the gate (DevTools, Application or Storage,
                  Cookies). Sent only to the seed host. Not supported with headless rendering.
                </span>
                <textarea autoComplete="off" name="cookie"
                  rows={2}
                  className="mono"
                  placeholder="_shopify_essential=:AZ_..."
                  value={v.cookie}
                  onChange={(e) => onChange({ cookie: e.target.value })}
                />
              </label>
            )}
            {loginSlot}
          </fieldset>
        ) : (
          <p className="cf-hint">
            Passwords, cookies, bearer tokens and inline headers are never saved as defaults. Put headers every crawl
            needs in a header profile.
          </p>
        )}

        <label className="cf-check">
          <input name="custom_headers" type="checkbox" checked={v.customHeaders} onChange={(e) => onChange({ customHeaders: e.target.checked })} />
          Send custom request headers (for example a Shopify crawler access signature)
        </label>
        {v.customHeaders && (
          <>
            <label className="cf-field">
              <span className="cf-label">Header profile</span>
              <span className="cf-hint">Managed under Settings; stored in ~/.gocrawl/headers on this machine.</span>
              <select name="header_profile" value={v.headerProfile} onChange={(e) => onChange({ headerProfile: e.target.value })}>
                <option value="">None</option>
                {profileOptions.map((name) => (
                  <option key={name} value={name}>
                    {profiles.includes(name) ? name : `${name} (missing)`}
                  </option>
                ))}
              </select>
            </label>
            {crawl && (
              <label className="cf-field">
                <span className="cf-label">Extra headers (one &quot;Name: value&quot; per line)</span>
                <span className="cf-hint">
                  Sent only to the seed host. They override the profile by name. Not supported with headless rendering.
                </span>
                <textarea autoComplete="off" name="headers"
                  rows={3}
                  className="mono"
                  placeholder="X-Access-Token: abc123"
                  value={v.headers}
                  onChange={(e) => onChange({ headers: e.target.value })}
                />
              </label>
            )}
          </>
        )}
      </details>
    </>
  )
}
