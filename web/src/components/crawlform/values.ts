import type { LoginSession } from '../../types/login'
import type { CrawlDefaults } from '../../types/settings'

// 'crawl' is the New crawl form; 'defaults' is the Settings view, which never holds secrets.
export type FieldsMode = 'crawl' | 'defaults'

// The crawl form's state, as the strings and booleans the inputs bind to. toParams and
// fromParams map it to and from the API's crawl request (crawlrequest.Params in Go).
export interface FormValues {
  url: string
  depth: string
  maxPages: string
  concurrency: string
  rate: string
  maxDuration: string
  timeout: string
  render: string
  adaptiveDelay: boolean
  save: boolean
  respectRobots: boolean
  subdomains: boolean
  followExternal: boolean
  followNofollow: boolean
  stripQuery: boolean
  specialized: boolean
  securityAudit: boolean
  ignoreExternalTagging: boolean
  analyzers: string[]
  include: string
  exclude: string
  userAgent: string
  userAgents: string
  userAgentRotation: string
  proxy: string
  proxies: string
  proxyRotation: string
  basicAuthUser: string
  basicAuthPass: string
  bearerToken: string
  cookieAuth: boolean
  cookie: string
  customHeaders: boolean
  headers: string
  headerProfile: string
}

export const blankValues: FormValues = {
  url: '',
  depth: '',
  maxPages: '',
  concurrency: '',
  rate: '',
  maxDuration: '',
  timeout: '',
  render: 'raw',
  adaptiveDelay: true,
  save: true,
  respectRobots: true,
  subdomains: false,
  followExternal: false,
  followNofollow: false,
  stripQuery: false,
  specialized: false,
  securityAudit: false,
  ignoreExternalTagging: true,
  analyzers: [],
  include: '',
  exclude: '',
  userAgent: '',
  userAgents: '',
  userAgentRotation: 'round-robin',
  proxy: '',
  proxies: '',
  proxyRotation: 'round-robin',
  basicAuthUser: '',
  basicAuthPass: '',
  bearerToken: '',
  cookieAuth: false,
  cookie: '',
  customHeaders: false,
  headers: '',
  headerProfile: '',
}

// Newline-separated rather than comma-separated: a comma is an ordinary character in a
// User-Agent string ("(KHTML, like Gecko)") and in a regex ("{2,4}").
export function splitList(s: string): string[] {
  return s
    .split('\n')
    .map((v) => v.trim())
    .filter(Boolean)
}

function joinList(list: string[] | undefined): string {
  return (list ?? []).join('\n')
}

function numberText(n: number | undefined): string {
  return n === undefined ? '' : String(n)
}

export function fromParams(p: CrawlDefaults | undefined): FormValues {
  if (!p) return { ...blankValues }
  return {
    ...blankValues,
    url: p.url ?? '',
    depth: numberText(p.depth),
    maxPages: numberText(p.max_pages),
    concurrency: numberText(p.concurrency),
    rate: numberText(p.rate),
    maxDuration: p.max_duration ?? '',
    timeout: p.timeout ?? '',
    render: p.render || 'raw',
    adaptiveDelay: p.adaptive_delay ?? true,
    respectRobots: p.respect_robots ?? true,
    subdomains: p.subdomains ?? false,
    followExternal: p.follow_external ?? false,
    followNofollow: p.follow_nofollow ?? false,
    stripQuery: p.strip_query ?? false,
    specialized: p.specialized ?? false,
    securityAudit: p.security_audit ?? false,
    ignoreExternalTagging: p.ignore_external_tagging ?? true,
    analyzers: p.analyzers ?? [],
    include: joinList(p.include),
    exclude: joinList(p.exclude),
    userAgent: p.user_agent ?? '',
    userAgents: joinList(p.user_agents),
    userAgentRotation: p.user_agent_rotation || 'round-robin',
    proxy: p.proxy ?? '',
    proxies: joinList(p.proxies),
    proxyRotation: p.proxy_rotation || 'round-robin',
    customHeaders: Boolean(p.header_profile) || (p.headers?.length ?? 0) > 0,
    headers: joinList(p.headers),
    headerProfile: p.header_profile ?? '',
  }
}

// toParams builds a crawl request. In 'defaults' mode it leaves out the URL and every secret
// (the server strips them anyway; not sending them keeps them out of the request too).
export function toParams(v: FormValues, mode: FieldsMode): CrawlDefaults {
  const p: CrawlDefaults = {
    render: v.render,
    adaptive_delay: v.adaptiveDelay,
    respect_robots: v.respectRobots,
    subdomains: v.subdomains,
    follow_external: v.followExternal,
    specialized: v.specialized,
    security_audit: v.securityAudit,
    ignore_external_tagging: v.ignoreExternalTagging,
  }
  if (mode === 'crawl') p.url = v.url.trim()
  if (v.depth) p.depth = Number(v.depth)
  if (v.maxPages) p.max_pages = Number(v.maxPages)
  if (v.concurrency) p.concurrency = Number(v.concurrency)
  if (v.rate) p.rate = Number(v.rate)
  if (v.maxDuration.trim()) p.max_duration = v.maxDuration.trim()
  if (v.timeout.trim()) p.timeout = v.timeout.trim()
  if (v.followNofollow) p.follow_nofollow = true
  if (v.stripQuery) p.strip_query = true
  if (v.analyzers.length > 0) p.analyzers = [...v.analyzers]
  if (v.include.trim()) p.include = splitList(v.include)
  if (v.exclude.trim()) p.exclude = splitList(v.exclude)
  if (v.userAgent.trim()) p.user_agent = v.userAgent.trim()
  if (v.userAgents.trim()) {
    p.user_agents = splitList(v.userAgents)
    p.user_agent_rotation = v.userAgentRotation
  }
  if (v.proxy.trim()) p.proxy = v.proxy.trim()
  if (v.proxies.trim()) {
    p.proxies = splitList(v.proxies)
    p.proxy_rotation = v.proxyRotation
  }
  if (v.customHeaders) {
    if (v.headerProfile.trim()) p.header_profile = v.headerProfile.trim()
    if (mode === 'crawl' && v.headers.trim()) p.headers = splitList(v.headers)
  }
  if (mode === 'crawl') {
    if (v.basicAuthUser.trim()) p.basic_auth = `${v.basicAuthUser.trim()}:${v.basicAuthPass}`
    if (v.cookieAuth && v.cookie.trim()) p.cookie = v.cookie.trim()
    if (v.bearerToken.trim()) p.bearer_token = v.bearerToken.trim()
  }
  return p
}

// Added to the exclude field when a browser login starts, so the crawl doesn't follow a
// logout link and end the session. It stays visible in the field and can be removed.
export const LOGOUT_EXCLUDE = '(?i)(log|sign)-?out'

// withLine appends line to a newline-separated list unless it is already there.
export function withLine(list: string, line: string): string {
  const lines = splitList(list)
  return lines.includes(line) ? list : [...lines, line].join('\n')
}

// hostOf returns the lower-case host name of a seed URL as the server computes it
// (https:// assumed when missing), or '' when it doesn't parse.
export function hostOf(url: string): string {
  const raw = url.trim()
  if (!raw) return ''
  try {
    return new URL(raw.includes('://') ? raw : `https://${raw}`).hostname.toLowerCase().replace(/^\[|\]$/g, '')
  } catch {
    return ''
  }
}

// loginExpired is true once a captured login's earliest cookie has expired. A session without
// expires_at (only session cookies) lasts until the server drops it, which the panel's poll
// and the start request report.
export function loginExpired(session: LoginSession, now: number = Date.now()): boolean {
  if (!session.expires_at) return false
  const t = Date.parse(session.expires_at)
  return !Number.isNaN(t) && t <= now
}
