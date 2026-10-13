import { useEffect, useState } from 'react'
import CrawlForm from './components/CrawlForm'
import CrawlReport from './components/CrawlReport'
import History from './components/History'
import { href, navigate, parseHash, type Route } from './routes'
import CompareView from './views/CompareView'
import RedirectView from './views/RedirectView'
import RedirectsView from './views/RedirectsView'
import SettingsView from './views/SettingsView'

// The navigation entries, in order, and which routes count as "inside" each one.
const NAV: { label: string; route: Route; owns: Route['view'][] }[] = [
  { label: 'New crawl', route: { view: 'new' }, owns: ['new'] },
  { label: 'Crawls', route: { view: 'crawls' }, owns: ['crawls', 'crawl'] },
  { label: 'Compare', route: { view: 'compare' }, owns: ['compare'] },
  { label: 'Redirects', route: { view: 'redirects' }, owns: ['redirects', 'redirect'] },
  { label: 'Settings', route: { view: 'settings' }, owns: ['settings'] },
]

export default function App() {
  const [route, setRoute] = useState<Route>(() => parseHash(window.location.hash))

  useEffect(() => {
    const onHashChange = () => setRoute(parseHash(window.location.hash))
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  return (
    <div className="app">
      <header className="app-header">
        <h1>gocrawl</h1>
        <nav>
          {NAV.map((item) => (
            <a key={item.label} href={href(item.route)} className={item.owns.includes(route.view) ? 'active' : ''}>
              {item.label}
            </a>
          ))}
        </nav>
      </header>

      <main>
        {route.view === 'new' && <CrawlForm onStarted={(id) => navigate({ view: 'crawl', id })} />}
        {route.view === 'crawls' && <History onSelect={(id) => navigate({ view: 'crawl', id })} />}
        {route.view === 'crawl' && <CrawlReport key={route.id} id={route.id} />}
        {route.view === 'compare' && <CompareView />}
        {route.view === 'redirects' && <RedirectsView />}
        {route.view === 'redirect' && <RedirectView key={route.id} id={route.id} />}
        {route.view === 'settings' && <SettingsView />}
      </main>
    </div>
  )
}
