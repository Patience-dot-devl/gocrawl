import type { RedirectCheckStatus } from '../../types/redirects'

// How a redirect check's status reads in the list and in the check's header.
export const STATUS_LABEL: Record<RedirectCheckStatus, string> = {
  running: 'Running',
  done: 'Done',
  error: 'Failed',
  canceled: 'Canceled',
}
