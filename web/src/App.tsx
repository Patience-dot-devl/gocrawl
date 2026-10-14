import { useEffect, useRef, useState } from 'react'
import CrawlForm from './components/CrawlForm'
import CrawlReport from './components/CrawlReport'
import History from './components/History'
import Rail from './components/Rail'
import { navigate, parseHash, type Route } from './routes'
import CompareView from './views/CompareView'
import RedirectView from './views/RedirectView'
import RedirectsView from './views/RedirectsView'
import SettingsView from './views/SettingsView'

export default function App() {
  const [route, setRoute] = useState<Route>(() => parseHash(window.location.hash))

  useEffect(() => {
    const onHashChange = () => setRoute(parseHash(window.location.hash))
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  // Moving to another view (not the first render, and not a change inside a view such as the
  // Compare pickers) puts focus on the page content, so a keyboard user does not have to
  // Tab back through the rail after every link or redirect.
  const mainRef = useRef<HTMLElement>(null)
  const viewKey = route.view === 'crawl' || route.view === 'redirect' ? `${route.view}/${route.id}` : route.view
  const firstView = useRef(true)
  useEffect(() => {
    if (firstView.current) {
      firstView.current = false
      return
    }
    mainRef.current?.focus()
  }, [viewKey])

  return (
    <div className="layout">
      <a className="skip-link" href="#main" onClick={(e) => {
        e.preventDefault()
        mainRef.current?.focus()
      }}>
        Skip to main content
      </a>
      <Rail current={route} />
      <main className="layout-main" id="main" tabIndex={-1} ref={mainRef}>
        {route.view === 'new' && <CrawlForm onStarted={(id) => navigate({ view: 'crawl', id })} />}
        {route.view === 'crawls' && <History onSelect={(id) => navigate({ view: 'crawl', id })} />}
        {route.view === 'crawl' && <CrawlReport key={route.id} id={route.id} />}
        {route.view === 'compare' && <CompareView base={route.base} current={route.current} />}
        {route.view === 'redirects' && <RedirectsView />}
        {route.view === 'redirect' && <RedirectView key={route.id} id={route.id} />}
        {route.view === 'settings' && <SettingsView />}
      </main>
    </div>
  )
}
