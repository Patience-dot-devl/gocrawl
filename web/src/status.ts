// HTTP status classes, shared by StatusCode, the crawl tape, and anything else that colors by
// status. 'x' is "no response": a fetch error, recorded with status 0.

export type StatusClass = '2' | '3' | '4' | '5' | 'x'

export const STATUS_CLASSES: StatusClass[] = ['2', '3', '4', '5', 'x']

export function statusClass(code: number): StatusClass {
  if (code >= 200 && code < 300) return '2'
  if (code >= 300 && code < 400) return '3'
  if (code >= 400 && code < 500) return '4'
  if (code >= 500 && code < 600) return '5'
  return 'x'
}

// Sentence-case names for legends and summaries.
export const STATUS_LABEL: Record<StatusClass, string> = {
  '2': 'OK',
  '3': 'Redirected',
  '4': 'Not found or blocked',
  '5': 'Server error',
  x: 'No response',
}
