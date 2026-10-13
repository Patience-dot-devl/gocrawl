import { useEffect, useId, useRef } from 'react'
import type { ReactNode } from 'react'
import Button from './Button'
import type { ButtonVariant } from './Button'
import './ConfirmDialog.css'

// A small modal that asks one yes-or-no question, in place of window.confirm (which ignores the
// theme and fonts). While open, focus sits on the cancel button (the safe choice) and Tab stays
// inside the dialog; Escape cancels; closing returns focus to whatever opened it. Render it only
// while the question is open.
type ConfirmDialogProps = {
  title: string
  children: ReactNode
  confirmLabel: string
  cancelLabel: string
  confirmVariant?: ButtonVariant
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmDialog({
  title,
  children,
  confirmLabel,
  cancelLabel,
  confirmVariant = 'primary',
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const titleId = useId()
  const dialogRef = useRef<HTMLDivElement>(null)
  const cancelRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    const trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
    cancelRef.current?.focus()
    return () => trigger?.focus()
  }, [])

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation()
      onCancel()
      return
    }
    if (e.key !== 'Tab') return
    const buttons = dialogRef.current?.querySelectorAll<HTMLButtonElement>('button')
    if (!buttons || buttons.length === 0) return
    const first = buttons[0]
    const last = buttons[buttons.length - 1]
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault()
      last.focus()
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault()
      first.focus()
    }
  }

  return (
    <div className="confirm-backdrop" onKeyDown={onKeyDown}>
      <div ref={dialogRef} className="confirm" role="dialog" aria-modal="true" aria-labelledby={titleId}>
        <h2 id={titleId}>{title}</h2>
        <div className="confirm-body">{children}</div>
        <div className="confirm-actions">
          <Button variant={confirmVariant} onClick={onConfirm}>
            {confirmLabel}
          </Button>
          <Button ref={cancelRef} onClick={onCancel}>
            {cancelLabel}
          </Button>
        </div>
      </div>
    </div>
  )
}
