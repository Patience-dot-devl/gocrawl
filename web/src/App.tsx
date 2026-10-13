import { useEffect, useState } from 'react'
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

  return (
    <div className="layout">
      <Rail current={route} />
      <main className="layout-main">
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
