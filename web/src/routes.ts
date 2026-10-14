// Hash routes for the single-page app. There is no router dependency: parseHash turns
// location.hash into a Route and href turns a Route back into a hash, so every link and
// redirect goes through these two functions. Hash routing also keeps working when the
// desktop shell serves the UI from a different 127.0.0.1 port on every launch.

export type Route =
  | { view: 'new' }
  | { view: 'crawls' }
  | { view: 'crawl'; id: string }
  | { view: 'compare'; base?: string; current?: string }
  | { view: 'redirects' }
  | { view: 'redirect'; id: string }
  | { view: 'settings' }

// Store IDs contain a slash (host/timestamp). href encodes it; parseHash also accepts the
// raw form, which older links and the first version of this UI used.
function decode(s: string): string {
  try {
    return decodeURIComponent(s)
  } catch {
    return s
  }
}

export function parseHash(hash: string): Route {
  const clean = hash.replace(/^#\/?/, '')
  const q = clean.indexOf('?')
  const path = q === -1 ? clean : clean.slice(0, q)
  const params = new URLSearchParams(q === -1 ? '' : clean.slice(q + 1))
  const slash = path.indexOf('/')
  const head = slash === -1 ? path : path.slice(0, slash)
  const rest = slash === -1 ? '' : decode(path.slice(slash + 1))

  switch (head) {
    case 'crawls':
    case 'history': // the first version of the UI called this view History
      return { view: 'crawls' }
    case 'crawl':
      return rest ? { view: 'crawl', id: rest } : { view: 'crawls' }
    case 'compare':
      return {
        view: 'compare',
        base: params.get('base') || undefined,
        current: params.get('current') || undefined,
      }
    case 'redirects':
      return { view: 'redirects' }
    case 'redirect':
      return rest ? { view: 'redirect', id: rest } : { view: 'redirects' }
    case 'settings':
      return { view: 'settings' }
    default:
      return { view: 'new' }
  }
}

export function href(route: Route): string {
  switch (route.view) {
    case 'new':
      return '#/'
    case 'crawls':
      return '#/crawls'
    case 'crawl':
      return `#/crawl/${encodeURIComponent(route.id)}`
    case 'compare': {
      const params = new URLSearchParams()
      if (route.base) params.set('base', route.base)
      if (route.current) params.set('current', route.current)
      const qs = params.toString()
      return qs ? `#/compare?${qs}` : '#/compare'
    }
    case 'redirects':
      return '#/redirects'
    case 'redirect':
      return `#/redirect/${encodeURIComponent(route.id)}`
    case 'settings':
      return '#/settings'
  }
}

// navigate moves to route; App re-renders from the hashchange event.
export function navigate(route: Route): void {
  window.location.hash = href(route)
}
