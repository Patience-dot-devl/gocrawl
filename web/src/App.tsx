import { useEffect, useState } from 'react'
import CrawlForm from './components/CrawlForm'
import CrawlReport from './components/CrawlReport'
import History from './components/History'

// Hash-based routing: "" -> new crawl form, "#history" -> history list,
// "#crawl/<id>" -> report view. Keeps this small enough to skip a router dependency.
type Route = { view: 'new' } | { view: 'history' } | { view: 'crawl'; id: string }

function parseHash(hash: string): Route {
  const clean = hash.replace(/^#\/?/, '')
  if (clean === 'history') return { view: 'history' }
  if (clean.startsWith('crawl/')) return { view: 'crawl', id: clean.slice('crawl/'.length) }
  return { view: 'new' }
}

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
          <a href="#/" className={route.view === 'new' ? 'active' : ''}>
            New crawl
          </a>
          <a href="#/history" className={route.view === 'history' ? 'active' : ''}>
            History
          </a>
        </nav>
      </header>

      <main>
        {route.view === 'new' && <CrawlForm onStarted={(id) => (window.location.hash = `#/crawl/${id}`)} />}
        {route.view === 'history' && <History onSelect={(id) => (window.location.hash = `#/crawl/${id}`)} />}
        {route.view === 'crawl' && <CrawlReport id={route.id} />}
      </main>
    </div>
  )
}
