import { href, type Route } from '../routes'
import './Rail.css'

// The navigation entries, in order, and which views count as "inside" each one (a single
// crawl belongs to Crawls, a single redirect check to Redirects).
const ITEMS: { label: string; route: Route; owns: Route['view'][] }[] = [
  { label: 'New crawl', route: { view: 'new' }, owns: ['new'] },
  { label: 'Crawls', route: { view: 'crawls' }, owns: ['crawls', 'crawl'] },
  { label: 'Compare', route: { view: 'compare' }, owns: ['compare'] },
  { label: 'Redirects', route: { view: 'redirects' }, owns: ['redirects', 'redirect'] },
  { label: 'Settings', route: { view: 'settings' }, owns: ['settings'] },
]

export default function Rail({ current }: { current: Route }) {
  return (
    <nav className="rail" aria-label="Main">
      <a className="rail-mark" href={href({ view: 'new' })}>
        gocrawl
      </a>
      <ul>
        {ITEMS.map((item) => {
          const active = item.owns.includes(current.view)
          return (
            <li key={item.label}>
              <a
                href={href(item.route)}
                className={active ? 'rail-item rail-item-on' : 'rail-item'}
                aria-current={active ? 'page' : undefined}
              >
                {item.label}
              </a>
            </li>
          )
        })}
      </ul>
    </nav>
  )
}
