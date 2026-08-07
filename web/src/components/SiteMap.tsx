import { useState } from 'react'
import type { PageIssue, SevCounts, SiteMap as SiteMapData, SiteMapNode } from '../types'

function statusClass(status?: number): string {
  if (!status) return 'st-none'
  if (status >= 200 && status < 300) return 'st-ok'
  if (status >= 300 && status < 400) return 'st-redirect'
  return 'st-error'
}

function healthClass(node: SiteMapNode): string {
  if (!node.url) return 'sm-h-none'
  if (node.counts?.error) return 'sm-h-error'
  if (node.counts?.warning) return 'sm-h-warning'
  if (node.counts?.info) return 'sm-h-info'
  return 'sm-h-ok'
}

function total(c?: SevCounts): number {
  return (c?.error ?? 0) + (c?.warning ?? 0) + (c?.info ?? 0)
}

function NodeBadges({ counts }: { counts?: SevCounts }) {
  if (!counts) return null
  return (
    <>
      {!!counts.error && (
        <span className="sm-sev sm-error" title="errors">
          {counts.error}
        </span>
      )}
      {!!counts.warning && (
        <span className="sm-sev sm-warning" title="warnings">
          {counts.warning}
        </span>
      )}
      {!!counts.info && (
        <span className="sm-sev sm-info" title="info">
          {counts.info}
        </span>
      )}
    </>
  )
}

function IssueList({ issues }: { issues: PageIssue[] }) {
  return (
    <ul className="sm-issuelist">
      {issues.map((it, i) => (
        <li key={i} className={`sm-issue severity-${it.severity}`}>
          <span className="sm-dot" />
          <code>{it.code}</code>
          <span className="sm-an">{it.analyzer}</span>
          <span className="sm-msg">{it.message}</span>
        </li>
      ))}
    </ul>
  )
}

function TreeNode({
  node,
  path,
  collapsed,
  toggleCollapsed,
  openPath,
  setOpenPath,
}: {
  node: SiteMapNode
  path: string
  collapsed: Set<string>
  toggleCollapsed: (path: string) => void
  openPath: string | null
  setOpenPath: (path: string | null) => void
}) {
  const hasChildren = !!node.children?.length
  const isCollapsed = collapsed.has(path)
  const hasPopup = !!node.issues?.length
  const isOpen = openPath === path
  const rollup = hasChildren && total(node.subtotal) > total(node.counts)

  return (
    <li className={`sm-li${hasChildren ? ' sm-haskids' : ''}${isCollapsed ? ' sm-collapsed' : ''}`}>
      <div
        className={`sm-box ${healthClass(node)}${hasPopup ? ' sm-haspop' : ''}${isOpen ? ' sm-sel' : ''}`}
        onClick={
          hasPopup
            ? (e) => {
                e.stopPropagation()
                setOpenPath(isOpen ? null : path)
              }
            : undefined
        }
      >
        <div className="sm-box-head">
          {hasChildren && (
            <button
              type="button"
              className="sm-box-toggle"
              title="collapse / expand subtree"
              onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
                toggleCollapsed(path)
              }}
            >
              {isCollapsed ? '+' : '−'}
            </button>
          )}
          {node.url ? (
            <a
              href={node.url}
              className="sm-box-label"
              title={node.url}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
            >
              {node.label || '/'}
            </a>
          ) : (
            <span className="sm-box-label">{node.label}</span>
          )}
          {node.status ? <span className={`sm-st ${statusClass(node.status)}`}>{node.status}</span> : null}
        </div>
        {node.title && (
          <div className="sm-box-title" title={node.title}>
            {node.title}
          </div>
        )}
        <div className="sm-box-foot">
          <NodeBadges counts={node.counts} />
          {rollup && (
            <span className="sm-roll" title="issues in this branch">
              {total(node.subtotal)}
            </span>
          )}
        </div>
        {hasPopup && (
          <div className="sm-pop" hidden={!isOpen} onClick={(e) => e.stopPropagation()}>
            <div className="sm-pop-h">
              {node.issues!.length} issue{node.issues!.length !== 1 ? 's' : ''} on this page
            </div>
            <IssueList issues={node.issues!} />
          </div>
        )}
      </div>
      {hasChildren && (
        <ul>
          {node.children!.map((child, i) => (
            <TreeNode
              key={i}
              node={child}
              path={`${path}.${i}`}
              collapsed={collapsed}
              toggleCollapsed={toggleCollapsed}
              openPath={openPath}
              setOpenPath={setOpenPath}
            />
          ))}
        </ul>
      )}
    </li>
  )
}

// Branches deeper than two levels start collapsed so the chart opens compact, matching
// report.html.tmpl's default.
function defaultCollapsed(node: SiteMapNode, path: string, depth: number, out: Set<string>) {
  if (node.children?.length && depth >= 2) out.add(path)
  node.children?.forEach((c, i) => defaultCollapsed(c, `${path}.${i}`, depth + 1, out))
}

function allBranchPaths(node: SiteMapNode, path: string, out: Set<string>) {
  if (node.children?.length) {
    out.add(path)
    node.children.forEach((c, i) => allBranchPaths(c, `${path}.${i}`, out))
  }
}

export default function SiteMap({ siteMap }: { siteMap: SiteMapData }) {
  const root = siteMap.root
  const [collapsed, setCollapsed] = useState<Set<string>>(() => {
    const s = new Set<string>()
    if (root) defaultCollapsed(root, '0', 0, s)
    return s
  })
  const [openPath, setOpenPath] = useState<string | null>(null)

  function toggleCollapsed(path: string) {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }

  function expandAll() {
    setCollapsed(new Set())
  }

  function collapseAll() {
    if (!root) return
    const s = new Set<string>()
    root.children?.forEach((c, i) => allBranchPaths(c, `0.${i}`, s))
    setCollapsed(s)
  }

  if (!root) return <p className="empty">No site map available.</p>

  const hasTotals = total(siteMap.totals) > 0

  return (
    <div onClick={() => setOpenPath(null)}>
      <div className="sm-meta">
        <code>{siteMap.host}</code> · {siteMap.entries?.length ?? 0} indexable page
        {siteMap.entries?.length !== 1 ? 's' : ''}
        {hasTotals && (
          <>
            {' '}
            · <NodeBadges counts={siteMap.totals} /> total
          </>
        )}
      </div>
      <div className="sm-controls">
        <button type="button" onClick={expandAll}>
          Expand all
        </button>
        <button type="button" onClick={collapseAll}>
          Collapse all
        </button>
        <span className="sm-hint">Click a node to see its issues · click a label to open the page · −/+ collapses a branch</span>
      </div>
      <div className="sm-chartwrap">
        <ul className="sm-chart">
          <TreeNode
            node={root}
            path="0"
            collapsed={collapsed}
            toggleCollapsed={toggleCollapsed}
            openPath={openPath}
            setOpenPath={setOpenPath}
          />
        </ul>
      </div>
      {!!siteMap.site_wide?.length && (
        <div className="sm-sitewide">
          <h3>
            Site-wide issues <span className="sm-muted">({siteMap.site_wide.length}, not tied to a single page)</span>
          </h3>
          <IssueList issues={siteMap.site_wide} />
        </div>
      )}
    </div>
  )
}
