import { useEffect, useState } from 'react'
import Button from '../ui/Button'
import Notice from '../ui/Notice'
import { previewSitemap } from '../../api/progress'
import type { SitemapPreview } from '../../types/progress'
import { budgetPercent, fileName, formatCount } from './tapeText'
import './progress.css'

const DEBOUNCE_MS = 600

interface Props {
  url: string
  include: string[]
  exclude: string[]
  subdomains: boolean
  /** Effective page budget; 0 = unlimited. */
  maxPages: number
  onRaiseBudget: (pages: number) => void
}

type Lookup = { key: string; kind: 'done'; preview: SitemapPreview } | { key: string; kind: 'error'; message: string }

/** Skips the lookup while the field can't be a site yet ("exa", "https://"). */
function looksLikeSite(u: string): boolean {
  return /^(https?:\/\/)?(localhost|[^\s/.]+\.[^\s/]+)/i.test(u)
}

export default function SitemapPreviewLine({ url, include, exclude, subdomains, maxPages, onRaiseBudget }: Props) {
  const target = url.trim()
  // One string for every input the lookup depends on: a stable effect dependency, and a tag
  // that lets a late answer for old inputs be ignored.
  const key = JSON.stringify([target, include, exclude, subdomains])
  const [lookup, setLookup] = useState<Lookup | null>(null)

  useEffect(() => {
    const [u, inc, exc, sub] = JSON.parse(key) as [string, string[], string[], boolean]
    if (!looksLikeSite(u)) return
    const ctrl = new AbortController()
    const timer = window.setTimeout(() => {
      previewSitemap({ url: u, include: inc, exclude: exc, allow_subdomains: sub }, ctrl.signal)
        .then((preview) => setLookup({ key, kind: 'done', preview }))
        .catch((err) => {
          if (!ctrl.signal.aborted) setLookup({ key, kind: 'error', message: err instanceof Error ? err.message : String(err) })
        })
    }, DEBOUNCE_MS)
    return () => {
      window.clearTimeout(timer)
      ctrl.abort()
    }
  }, [key])

  if (!looksLikeSite(target)) return null
  const current = lookup?.key === key ? lookup : null
  if (!current) {
    return (
      <p className="sp-line" aria-live="polite">
        Looking for a sitemap…
      </p>
    )
  }
  if (current.kind === 'error') {
    return (
      <p className="sp-line" aria-live="polite">
        Couldn't check for a sitemap: {current.message}
      </p>
    )
  }
  const { preview } = current
  if (!preview.found) {
    return (
      <p className="sp-line" aria-live="polite">
        Couldn't find a sitemap. Progress will count pages against the page budget instead.
      </p>
    )
  }
  const source = preview.sources.length > 0 ? fileName(preview.sources[0]) : 'a sitemap'
  const budgetLow = maxPages > 0 && preview.total > maxPages
  return (
    <div className="sp-wrap">
      <p className="sp-line" aria-live="polite">
        Found {source}
        {preview.via_robots ? ' via robots.txt' : ''}: {formatCount(preview.total)} {preview.total === 1 ? 'page' : 'pages'} in
        scope
      </p>
      {budgetLow && (
        <Notice
          tone="warning"
          action={
            <Button type="button" variant="secondary" onClick={() => onRaiseBudget(preview.total)}>
              Raise budget to {formatCount(preview.total)}
            </Button>
          }
        >
          <p>
            The page budget ({formatCount(maxPages)}) is below the sitemap total ({formatCount(preview.total)}). This crawl would
            cover at most {budgetPercent(maxPages, preview.total)}% of the sitemap.
          </p>
        </Notice>
      )}
    </div>
  )
}
