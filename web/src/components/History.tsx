import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import { listCrawls } from '../api/crawls'
import { importReport } from '../api/import'
import { STATUS_PILL } from '../crawlStatus'
import { usePageTitle } from '../pageTitle'
import { href } from '../routes'
import { useSort } from '../sort'
import type { CrawlListItemWithProgress } from '../types/progress'
import { isComparable, orderPair } from '../views/compare/pairs'
import Button from './ui/Button'
import Notice from './ui/Notice'
import PhasePill from './ui/PhasePill'
import MiniTape from './progress/MiniTape'
import { formatCount, percentOf, sitemapCoverageText } from './progress/tapeText'
import { isLive } from './progress/useCrawlPoll'
import '../views/compare.css'
import './History.css'

function historySortValue(item: CrawlListItemWithProgress, key: string): string | number {
  switch (key) {
    case 'seed':
      return item.seed
    case 'status':
      return item.status
    case 'pages_crawled':
      return item.pages_crawled ?? -1
    case 'sitemap':
      return item.sitemap_total ? percentOf(item.sitemap_fetched ?? 0, item.sitemap_total) : -1
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
  { key: 'sitemap', label: 'Sitemap coverage' },
  { key: 'errors', label: 'Errors', numeric: true },
  { key: 'warnings', label: 'Warnings', numeric: true },
  { key: 'finished_at', label: 'Finished' },
]

export default function History({ onSelect }: { onSelect: (id: string) => void }) {
  usePageTitle('Crawls')
  const [items, setItems] = useState<CrawlListItemWithProgress[]>([])
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')
  const [picked, setPicked] = useState<string[]>([])
  const [importing, setImporting] = useState(false)
  const [importError, setImportError] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    listCrawls()
      .then((r) => setItems(r.crawls))
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoaded(true))
  }, [])

  function togglePicked(id: string) {
    setPicked((prev) => (prev.includes(id) ? prev.filter((p) => p !== id) : prev.length >= 2 ? prev : [...prev, id]))
  }

  function openCompare() {
    const chosen = picked.map((id) => items.find((i) => i.id === id)).filter((i) => i !== undefined)
    if (chosen.length !== 2) return
    const pair = orderPair(chosen[0], chosen[1])
    window.location.hash = href({ view: 'compare', base: pair.base, current: pair.current })
  }

  async function onImportFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    setImporting(true)
    setImportError('')
    try {
      const { id } = await importReport(file)
      window.location.hash = href({ view: 'crawl', id })
    } catch (err) {
      // The server appends the JSON decoder's complaint ("invalid character 'h' ..."), which
      // helps nobody choosing a file; keep the first part.
      const text = err instanceof Error ? err.message : String(err)
      setImportError(text.replace(/^(not a gocrawl JSON report): [\s\S]*$/, '$1'))
    } finally {
      setImporting(false)
    }
  }

  const filtered = items.filter((item) => item.seed.toLowerCase().includes(search.trim().toLowerCase()))
  const { sorted, toggle, dirFor } = useSort(filtered, historySortValue)

  if (error) return <Notice tone="error">Could not load your crawls: {error}. Reload the page to try again.</Notice>
  if (!loaded) return <p className="muted">Loading…</p>

  const toolbar = (
    <>
      <div className="crawls-toolbar">
        <Button variant={picked.length === 2 ? 'primary' : 'secondary'} disabled={picked.length !== 2} onClick={openCompare}>
          Compare selected ({picked.length} of 2)
        </Button>
        <Button
          variant="secondary"
          aria-disabled={importing || undefined}
          onClick={() => {
            if (!importing) fileRef.current?.click()
          }}
        >
          {importing ? 'Importing…' : 'Import report…'}
        </Button>
        <input ref={fileRef} type="file" accept=".json,application/json" hidden onChange={onImportFile} />
      </div>
      {importError && <Notice tone="error">Could not import that file: {importError}. Choose a JSON report written by gocrawl.</Notice>}
    </>
  )

  return (
    <div className="hist">
      <h1>Crawls</h1>
      {toolbar}
      {items.length === 0 ? (
        <p className="hist-empty">
          No saved crawls yet. Turn on &lsquo;Save to Crawls&rsquo; when you <a href={href({ view: 'new' })}>start a crawl</a>,
          or import a JSON report.
        </p>
      ) : (
        <>
          <div className="hist-filters">
            <input
              type="search"
              name="search"
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
                  <th scope="col" className="hist-pick">
                    <span className="sr-only">Select to compare</span>
                  </th>
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
                  <th scope="col">Pages by status</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((item) => {
                  const pill = STATUS_PILL[item.status]
                  // Analysis has not run yet, so 0 errors would be a claim the crawl has not earned.
                  const live = isLive(item.status)
                  return (
                    <tr key={item.id} className="hist-row" onClick={() => onSelect(item.id)}>
                      <td className="hist-pick" onClick={(e) => e.stopPropagation()}>
                        <input
                          type="checkbox"
                          name="compare"
                          aria-label={`Compare ${item.seed}, finished ${formatFinished(item.finished_at)}`}
                          checked={picked.includes(item.id)}
                          disabled={!isComparable(item) || (picked.length >= 2 && !picked.includes(item.id))}
                          onChange={() => togglePicked(item.id)}
                        />
                      </td>
                      <td>
                        <a className="mono" href={href({ view: 'crawl', id: item.id })} onClick={(e) => e.stopPropagation()}>
                          {item.seed}
                        </a>
                      </td>
                      <td>
                        <PhasePill phase={pill.phase}>{pill.label}</PhasePill>
                      </td>
                      <td className="hist-num num">{item.pages_crawled === undefined ? '—' : formatCount(item.pages_crawled)}</td>
                      <td>{sitemapCoverageText(item.sitemap_total, item.sitemap_fetched)}</td>
                      <td className="hist-num num">{live ? '—' : formatCount(item.by_severity?.error ?? 0)}</td>
                      <td className="hist-num num">{live ? '—' : formatCount(item.by_severity?.warning ?? 0)}</td>
                      <td className="muted">{formatFinished(item.finished_at)}</td>
                      <td className="mini-tape-cell">
                        <MiniTape item={item} />
                      </td>
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
