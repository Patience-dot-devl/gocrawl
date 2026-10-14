import { useEffect, useId, useRef } from 'react'
import type { ReactNode } from 'react'
import Button from './Button'
import type { ButtonVariant } from './Button'
import './ConfirmDialog.css'

// A small modal that asks one yes-or-no question, in place of window.confirm (which ignores the
// theme and fonts). It is a native <dialog> opened with showModal(), so the browser makes the
// page behind it inert, keeps Tab inside, and turns Escape into a cancel event. Focus starts on
// the cancel button (the safe choice) and returns to whatever opened the dialog when it closes.
// Render it only while the question is open.
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
  const dialogRef = useRef<HTMLDialogElement>(null)
  const cancelRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    const dialog = dialogRef.current
    const trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
    if (dialog && !dialog.open) dialog.showModal()
    cancelRef.current?.focus()
    return () => {
      if (dialog?.open) dialog.close()
      trigger?.focus()
    }
  }, [])

  return (
    <dialog
      ref={dialogRef}
      className="confirm"
      aria-labelledby={titleId}
      onCancel={(e) => {
        // Escape: keep the dialog mounted until the parent decides, so state stays in one place.
        e.preventDefault()
        onCancel()
      }}
    >
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
    </dialog>
  )
}
