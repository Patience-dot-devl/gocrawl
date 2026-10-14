import type { ReactNode } from 'react'
import './Notice.css'

// A boxed message with a colored left edge: warning (yellow, e.g. partial coverage or a
// budget below the sitemap), error (red), info (violet, e.g. run notes). action holds a
// button or a small form that resolves the notice, such as "Raise budget". role defaults to
// alert for an error and status for the rest; pass role="alert" to announce a warning at once.
export type NoticeTone = 'warning' | 'error' | 'info'

type NoticeProps = { tone: NoticeTone; children: ReactNode; action?: ReactNode; role?: 'alert' | 'status' }

export default function Notice({ tone, children, action, role }: NoticeProps) {
  return (
    <div className={`notice notice-${tone}`} role={role ?? (tone === 'error' ? 'alert' : 'status')}>
      <div className="notice-body">{children}</div>
      {action && <div className="notice-action">{action}</div>}
    </div>
  )
}
