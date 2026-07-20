import { useEffect, useState } from 'react'
import { listCrawls } from '../api'
import type { CrawlListItem } from '../types'

export default function History({ onSelect }: { onSelect: (id: string) => void }) {
  const [items, setItems] = useState<CrawlListItem[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    listCrawls()
      .then((r) => setItems(r.crawls))
      .catch((e) => setError(String(e)))
  }, [])

  if (error) return <p className="error">{error}</p>
  if (items.length === 0) return <p>No crawls yet.</p>

  return (
    <div className="card">
      <h2>Crawls</h2>
      <table className="history">
        <thead>
          <tr>
            <th>Seed</th>
            <th>Status</th>
            <th>Pages</th>
            <th>Errors</th>
            <th>Warnings</th>
            <th>Finished</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.id} className="clickable" onClick={() => onSelect(item.id)}>
              <td>{item.seed}</td>
              <td className={`status-${item.status}`}>{item.status}</td>
              <td>{item.pages_crawled ?? '—'}</td>
              <td>{item.by_severity?.error ?? 0}</td>
              <td>{item.by_severity?.warning ?? 0}</td>
              <td>{item.finished_at ?? '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
