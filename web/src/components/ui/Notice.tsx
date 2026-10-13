import type { ReactNode } from 'react'
import './Notice.css'

// A boxed message with a colored left edge: warning (yellow, e.g. partial coverage or a
// budget below the sitemap), error (red), info (violet, e.g. run notes). action holds a
// button or a small form that resolves the notice, such as "Raise budget".
export type NoticeTone = 'warning' | 'error' | 'info'

export default function Notice({ tone, children, action }: { tone: NoticeTone; children: ReactNode; action?: ReactNode }) {
  return (
    <div className={`notice notice-${tone}`} role={tone === 'error' ? 'alert' : 'status'}>
      <div className="notice-body">{children}</div>
      {action && <div className="notice-action">{action}</div>}
    </div>
  )
}
