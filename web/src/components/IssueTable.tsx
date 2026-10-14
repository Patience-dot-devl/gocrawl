import { useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from 'react'
import { useSort } from '../sort'
import type { Explanation, Issue } from '../types/crawls'
import { formatCount } from './progress/tapeText'
import Button from './ui/Button'
import './CrawlReport.css'

const SEVERITY_RANK: Record<string, number> = { error: 0, warning: 1, info: 2 }

export interface ReviewState {
  resolved: boolean
  nonissue: boolean
  note: string
}

// Review state keyed by index into the issues array.
export type ReviewMap = Record<number, ReviewState>

// The table draws this many rows at a time. Filters, sort, counts and bulk review work on the
// whole list; only the drawing is batched, so a 4,000-issue crawl stays responsive and keeps
// a sane number of tab stops.
const PAGE_SIZE = 100

const EMPTY_REVIEW: ReviewState = { resolved: false, nonissue: false, note: '' }

type IssueRow = Issue & { _key: number }

function issueSortValue(row: IssueRow, key: string): string | number {
  switch (key) {
    case 'severity':
      return SEVERITY_RANK[row.severity] ?? 9
    case 'analyzer':
      return row.analyzer
    case 'code':
      return row.code
    case 'url':
      return row.url
    case 'message':
      return row.message
    default:
      return ''
  }
}

interface IssueTableProps {
  issues: Issue[]
  explanations: Record<string, Explanation>
  // When given, the table shows the Review column, the bulk bar and the hide-resolved /
  // hide-non-issue toggles, and writes review state here. Compare passes nothing: its issue
  // lists are read-only.
  review?: { map: ReviewMap; set: Dispatch<SetStateAction<ReviewMap>> }
  // Shown instead of the table when there are no issues at all.
  empty?: string
}

// The report's issue table, moved out of CrawlReport: severity / analyzer / code / text
// filters, sortable columns, inline explanations and data, and (for a single crawl's report)
// per-issue review state.
export default function IssueTable({ issues, explanations, review, empty = 'No issues were found.' }: IssueTableProps) {
  const [severityOn, setSeverityOn] = useState<Record<string, boolean>>({ error: true, warning: true, info: true })
  const [analyzerFilter, setAnalyzerFilter] = useState('')
  const [search, setSearch] = useState('')
  // A code missing from codeOn counts as shown.
  const [codeOn, setCodeOn] = useState<Record<string, boolean>>({})
  const [codeMenuOpen, setCodeMenuOpen] = useState(false)
  const [hideResolved, setHideResolved] = useState(false)
  const [hideNonissue, setHideNonissue] = useState(false)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [limit, setLimit] = useState(PAGE_SIZE)
  const selectAllRef = useRef<HTMLInputElement>(null)
  const endRef = useRef<HTMLParagraphElement>(null)
  const reviewMap = review?.map

  useEffect(() => {
    if (!codeMenuOpen) return
    function close() {
      setCodeMenuOpen(false)
    }
    document.addEventListener('click', close)
    return () => document.removeEventListener('click', close)
  }, [codeMenuOpen])

  const analyzers = useMemo(() => [...new Set(issues.map((i) => i.analyzer))].sort(), [issues])
  const codeCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    issues.forEach((i) => {
      counts[i.code] = (counts[i.code] ?? 0) + 1
    })
    return counts
  }, [issues])
  const codeList = useMemo(() => Object.keys(codeCounts).sort(), [codeCounts])
  const hiddenCodeCount = codeList.filter((c) => codeOn[c] === false).length

  const rows: IssueRow[] = useMemo(() => issues.map((i, originalIndex) => ({ ...i, _key: originalIndex })), [issues])

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase()
    return rows.filter((i) => {
      if (severityOn[i.severity] === false) return false
      if (analyzerFilter && i.analyzer !== analyzerFilter) return false
      if (codeOn[i.code] === false) return false
      if (term && !(i.url.toLowerCase().includes(term) || i.message.toLowerCase().includes(term) || i.code.toLowerCase().includes(term))) {
        return false
      }
      const r = reviewMap?.[i._key]
      if (hideResolved && r?.resolved) return false
      if (hideNonissue && r?.nonissue) return false
      return true
    })
  }, [rows, severityOn, analyzerFilter, codeOn, search, hideResolved, hideNonissue, reviewMap])

  const { sorted, toggle, dirFor } = useSort(filtered, issueSortValue, { key: 'severity' })

  // A new filter starts again from the first batch. Review marks do not: marking a row
  // resolved must not collapse the rows the user has paged through.
  const filterSignature = JSON.stringify([severityOn, analyzerFilter, codeOn, search, hideResolved, hideNonissue])
  const [signature, setSignature] = useState(filterSignature)
  if (signature !== filterSignature) {
    setSignature(filterSignature)
    setLimit(PAGE_SIZE)
  }
  const visible = sorted.slice(0, limit)
  const hiddenCount = sorted.length - visible.length

  const selectedShownCount = sorted.filter((r) => selected.has(r._key)).length
  useEffect(() => {
    if (!selectAllRef.current) return
    selectAllRef.current.indeterminate = selectedShownCount > 0 && selectedShownCount < sorted.length
  }, [selectedShownCount, sorted.length])

  function setReviewFlag(key: number, patch: Partial<ReviewState>) {
    review?.set((prev) => ({
      ...prev,
      [key]: { ...EMPTY_REVIEW, ...prev[key], ...patch },
    }))
  }

  function toggleSelected(key: number) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  function selectShown(checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      sorted.forEach((r) => (checked ? next.add(r._key) : next.delete(r._key)))
      return next
    })
  }

  function bulkApply(kind: 'resolved' | 'nonissue' | 'clear') {
    if (!review) return
    review.set((prev) => {
      const next = { ...prev }
      sorted.forEach((r) => {
        if (!selected.has(r._key)) return
        if (kind === 'clear') {
          next[r._key] = { ...EMPTY_REVIEW, note: prev[r._key]?.note ?? '' }
        } else {
          next[r._key] = { ...EMPTY_REVIEW, ...prev[r._key], [kind]: true }
        }
      })
      return next
    })
    setSelected(new Set())
  }

  function clearFilters() {
    setSeverityOn({ error: true, warning: true, info: true })
    setAnalyzerFilter('')
    setSearch('')
    setCodeOn(Object.fromEntries(codeList.map((c) => [c, true])))
    setHideResolved(false)
    setHideNonissue(false)
  }

  const filtersActive =
    search.trim() !== '' ||
    analyzerFilter !== '' ||
    hiddenCodeCount > 0 ||
    hideResolved ||
    hideNonissue ||
    Object.values(severityOn).some((v) => v === false)

  if (rows.length === 0) return <p className="muted">{empty}</p>

  return (
    <>
      <button type="button" className="skip-link" onClick={() => endRef.current?.focus()}>
        Skip the issue table
      </button>
      <div className="cr-filters">
        <div className="cr-sevs" role="group" aria-label="Severity">
          {(['error', 'warning', 'info'] as const).map((sev) => (
            <button
              key={sev}
              type="button"
              className={`cr-sev sev-${sev}`}
              aria-pressed={severityOn[sev]}
              onClick={() => setSeverityOn((prev) => ({ ...prev, [sev]: !prev[sev] }))}
            >
              {sev.charAt(0).toUpperCase() + sev.slice(1)}
            </button>
          ))}
        </div>
        <label className="cr-inline">
          Analyzer
          <select name="analyzer" value={analyzerFilter} onChange={(e) => setAnalyzerFilter(e.target.value)}>
            <option value="">All</option>
            {analyzers.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </label>
        <div className={`cr-codes${hiddenCodeCount > 0 ? ' cr-codes-hidden' : ''}`}>
          <Button
            aria-expanded={codeMenuOpen}
            onClick={(e) => {
              e.stopPropagation()
              setCodeMenuOpen((v) => !v)
            }}
          >
            {hiddenCodeCount > 0 ? `Codes (${hiddenCodeCount} hidden)` : 'Codes'}
            <span aria-hidden="true">▾</span>
          </Button>
          <div className="cr-codes-panel" hidden={!codeMenuOpen} onClick={(e) => e.stopPropagation()}>
            <div className="cr-codes-actions">
              <Button onClick={() => setCodeOn(Object.fromEntries(codeList.map((c) => [c, true])))}>
                Show all
              </Button>
              <Button onClick={() => setCodeOn(Object.fromEntries(codeList.map((c) => [c, false])))}>
                Hide all
              </Button>
            </div>
            {codeList.map((c) => (
              <label key={c} className="cr-code-option">
                <input
                  type="checkbox"
                  name="code"
                  value={c}
                  checked={codeOn[c] !== false}
                  onChange={(e) => setCodeOn((prev) => ({ ...prev, [c]: e.target.checked }))}
                />
                <code>{c}</code>
                <span className="num muted">{codeCounts[c]}</span>
              </label>
            ))}
          </div>
        </div>
        <input
          type="search"
          name="search"
          className="cr-search"
          placeholder="Search URL, message, code…"
          aria-label="Search issues"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        {review && (
          <>
            <label className="cr-inline">
              <input type="checkbox" name="hide_resolved" checked={hideResolved} onChange={(e) => setHideResolved(e.target.checked)} />
              Hide resolved
            </label>
            <label className="cr-inline">
              <input type="checkbox" name="hide_nonissue" checked={hideNonissue} onChange={(e) => setHideNonissue(e.target.checked)} />
              Hide non-issues
            </label>
          </>
        )}
        <span className="cr-count-line num muted">
          Showing {formatCount(sorted.length)} of {formatCount(rows.length)} {rows.length === 1 ? 'issue' : 'issues'}
        </span>
        {filtersActive && <Button onClick={clearFilters}>Clear filters</Button>}
      </div>

      {review && (
        <div className="cr-bulk">
          <label className="cr-inline">
            <input
              ref={selectAllRef}
              type="checkbox"
              name="select_all"
              checked={sorted.length > 0 && selectedShownCount === sorted.length}
              onChange={(e) => selectShown(e.target.checked)}
            />
            Select all matching
          </label>
          <Button disabled={selected.size === 0} onClick={() => bulkApply('nonissue')}>
            Mark as non-issue
          </Button>
          <Button disabled={selected.size === 0} onClick={() => bulkApply('resolved')}>
            Mark as resolved
          </Button>
          <Button disabled={selected.size === 0} onClick={() => bulkApply('clear')}>
            Clear review
          </Button>
          {selected.size > 0 && <span className="num muted">{formatCount(selected.size)} selected</span>}
        </div>
      )}

      <div className="cr-table-scroll">
        <table className="cr-issues">
          <thead>
            <tr>
              {(['severity', 'analyzer', 'code', 'url', 'message'] as const).map((col) => (
                <th
                  key={col}
                  className="cr-sortable"
                  data-dir={dirFor(col)}
                  aria-sort={dirFor(col) === 'asc' ? 'ascending' : dirFor(col) === 'desc' ? 'descending' : undefined}
                >
                  <button type="button" onClick={() => toggle(col)}>
                    {col === 'url' ? 'URL' : col.charAt(0).toUpperCase() + col.slice(1)}
                  </button>
                </th>
              ))}
              {review && <th>Review</th>}
            </tr>
          </thead>
          <tbody>
            {visible.map((i) => {
              const explanation = explanations[i.code]
              const r = reviewMap?.[i._key]
              return (
                <tr key={i._key} className={r?.resolved || r?.nonissue ? 'cr-reviewed' : undefined}>
                  <td className={`sev-${i.severity}`}>{i.severity}</td>
                  <td className="mono">{i.analyzer}</td>
                  <td>
                    <code>{i.code}</code>
                  </td>
                  <td className="cr-url mono" title={i.url}>
                    {i.url}
                  </td>
                  <td>
                    {i.message}
                    {explanation && (
                      <details className="cr-explain">
                        <summary>What this means and how to fix it</summary>
                        <dl>
                          <dt>What it is</dt>
                          <dd>{explanation.what}</dd>
                          <dt>Impact</dt>
                          <dd>{explanation.impact}</dd>
                          <dt>How to fix it</dt>
                          <dd>{explanation.fix}</dd>
                        </dl>
                      </details>
                    )}
                    {i.data && (
                      <details className="cr-explain">
                        <summary>Data</summary>
                        <pre>{JSON.stringify(i.data, null, 2)}</pre>
                      </details>
                    )}
                  </td>
                  {review && (
                    <td className="cr-review">
                      <label className="cr-inline">
                        <input type="checkbox" name="select" checked={selected.has(i._key)} onChange={() => toggleSelected(i._key)} />
                        Select
                      </label>
                      <label className="cr-inline">
                        <input
                          type="checkbox"
                          name="resolved"
                          checked={!!r?.resolved}
                          onChange={(e) => setReviewFlag(i._key, { resolved: e.target.checked })}
                        />
                        Resolved
                      </label>
                      <label className="cr-inline">
                        <input
                          type="checkbox"
                          name="nonissue"
                          checked={!!r?.nonissue}
                          onChange={(e) => setReviewFlag(i._key, { nonissue: e.target.checked })}
                        />
                        Non-issue
                      </label>
                      <textarea
                        name="comment"
                        rows={2}
                        placeholder="Add a comment…"
                        aria-label="Comment"
                        value={r?.note ?? ''}
                        onChange={(e) => setReviewFlag(i._key, { note: e.target.value })}
                      />
                    </td>
                  )}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
      <p className="cr-more num muted" ref={endRef} tabIndex={-1}>
        {sorted.length === 0
          ? 'No issues match these filters.'
          : `Showing ${formatCount(visible.length)} of ${formatCount(sorted.length)} matching ${sorted.length === 1 ? 'issue' : 'issues'}.`}
        {hiddenCount > 0 && (
          <>
            {' '}
            <Button onClick={() => setLimit((n) => n + PAGE_SIZE)}>
              Show {formatCount(Math.min(PAGE_SIZE, hiddenCount))} more
            </Button>
            {hiddenCount > PAGE_SIZE && <Button onClick={() => setLimit(sorted.length)}>Show all {formatCount(sorted.length)}</Button>}
          </>
        )}
      </p>
    </>
  )
}
