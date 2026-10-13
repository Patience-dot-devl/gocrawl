import { useEffect, useState } from 'react'
import { listCrawls } from '../api/crawls'
import { STATUS_PILL } from '../crawlStatus'
import { href } from '../routes'
import { useSort } from '../sort'
import type { CrawlListItem } from '../types/crawls'
import Notice from './ui/Notice'
import PhasePill from './ui/PhasePill'
import './History.css'

function historySortValue(item: CrawlListItem, key: string): string | number {
  switch (key) {
    case 'seed':
      return item.seed
    case 'status':
      return item.status
    case 'pages_crawled':
      return item.pages_crawled ?? -1
    case 'errors':
      return item.by_severity?.error ?? 0
    case 'warnings':
      return item.by_severity?.warning ?? 0
    case 'finished_at':
      return item.finished_at ?? ''
    default:
      return ''
  }
}

function formatFinished(finishedAt?: string): string {
  if (!finishedAt) return '—'
  const d = new Date(finishedAt)
  return Number.isNaN(d.getTime()) ? finishedAt : d.toLocaleString()
}

const COLUMNS: { key: string; label: string; numeric?: boolean }[] = [
  { key: 'seed', label: 'Site' },
  { key: 'status', label: 'Status' },
  { key: 'pages_crawled', label: 'Pages', numeric: true },
  { key: 'errors', label: 'Errors', numeric: true },
  { key: 'warnings', label: 'Warnings', numeric: true },
  { key: 'finished_at', label: 'Finished' },
]

export default function History({ onSelect }: { onSelect: (id: string) => void }) {
  const [items, setItems] = useState<CrawlListItem[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')

  useEffect(() => {
    listCrawls()
      .then((r) => setItems(r.crawls))
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoaded(true))
  }, [])

  const filtered = items.filter((item) => item.seed.toLowerCase().includes(search.trim().toLowerCase()))
  const { sorted, toggle, dirFor } = useSort(filtered, historySortValue)

  if (error) return <Notice tone="error">Could not load your crawls: {error}. Reload the page to try again.</Notice>
  if (!loaded) return <p className="muted">Loading…</p>

  return (
    <div className="hist">
      <h1>Crawls</h1>
      {items.length === 0 ? (
        <p className="hist-empty">
          No saved crawls yet. Turn on &lsquo;Save to Crawls&rsquo; when you <a href={href({ view: 'new' })}>start a crawl</a>.
        </p>
      ) : (
        <>
          <div className="hist-filters">
            <input
              type="search"
              placeholder="Search by site URL…"
              aria-label="Search crawls"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <span className="num muted">
              Showing {sorted.length.toLocaleString()} of {items.length.toLocaleString()}
            </span>
          </div>
          <div className="hist-table">
            <table>
              <thead>
                <tr>
                  {COLUMNS.map((col) => (
                    <th
                      key={col.key}
                      className={`hist-sortable${col.numeric ? ' hist-num' : ''}`}
                      data-dir={dirFor(col.key)}
                      aria-sort={dirFor(col.key) === 'asc' ? 'ascending' : dirFor(col.key) === 'desc' ? 'descending' : undefined}
                    >
                      <button type="button" onClick={() => toggle(col.key)}>
                        {col.label}
                      </button>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {sorted.map((item) => {
                  const pill = STATUS_PILL[item.status]
                  return (
                    <tr key={item.id} className="hist-row" onClick={() => onSelect(item.id)}>
                      <td>
                        <a className="mono" href={href({ view: 'crawl', id: item.id })} onClick={(e) => e.stopPropagation()}>
                          {item.seed}
                        </a>
                      </td>
                      <td>
                        <PhasePill phase={pill.phase}>{pill.label}</PhasePill>
                      </td>
                      <td className="hist-num num">{item.pages_crawled?.toLocaleString() ?? '—'}</td>
                      <td className="hist-num num">{item.by_severity?.error ?? 0}</td>
                      <td className="hist-num num">{item.by_severity?.warning ?? 0}</td>
                      <td className="muted">{formatFinished(item.finished_at)}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  )
}
