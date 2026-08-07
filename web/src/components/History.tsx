import { useEffect, useState } from 'react'
import { listCrawls } from '../api'
import { useSort } from '../sort'
import type { CrawlListItem } from '../types'

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

const COLUMNS: { key: string; label: string }[] = [
  { key: 'seed', label: 'Seed' },
  { key: 'status', label: 'Status' },
  { key: 'pages_crawled', label: 'Pages' },
  { key: 'errors', label: 'Errors' },
  { key: 'warnings', label: 'Warnings' },
  { key: 'finished_at', label: 'Finished' },
]

export default function History({ onSelect }: { onSelect: (id: string) => void }) {
  const [items, setItems] = useState<CrawlListItem[]>([])
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')

  useEffect(() => {
    listCrawls()
      .then((r) => setItems(r.crawls))
      .catch((e) => setError(String(e)))
  }, [])

  const filtered = items.filter((item) => item.seed.toLowerCase().includes(search.trim().toLowerCase()))
  const { sorted, toggle, dirFor } = useSort(filtered, historySortValue)

  if (error) return <p className="error">{error}</p>
  if (items.length === 0) return <p>No crawls yet.</p>

  return (
    <div className="card">
      <h2>Crawls</h2>
      <div className="filters">
        <input type="search" placeholder="Search by seed URL…" value={search} onChange={(e) => setSearch(e.target.value)} />
        <span className="filter-count">
          Showing {sorted.length.toLocaleString()} of {items.length.toLocaleString()}
        </span>
      </div>
      <table className="history">
        <thead>
          <tr>
            {COLUMNS.map((col) => (
              <th key={col.key} className="sortable" onClick={() => toggle(col.key)} data-dir={dirFor(col.key)}>
                {col.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {sorted.map((item) => (
            <tr key={item.id} className="clickable" onClick={() => onSelect(item.id)}>
              <td>{item.seed}</td>
              <td className={`status-${item.status}`}>{item.status}</td>
              <td>{item.pages_crawled ?? '—'}</td>
              <td>{item.by_severity?.error ?? 0}</td>
              <td>{item.by_severity?.warning ?? 0}</td>
              <td>{formatFinished(item.finished_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
