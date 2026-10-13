// Pure model behind the crawl tape (Tape.tsx). The API sends the tape as two strings that
// grow by one character per recorded page: `tape` holds the status class ('2', '3', '4', '5',
// or 'x' for a fetch error) and `tapeOff` holds '1' when that page is not in the sitemap.
// Outline ticks follow the fetched ones: sitemap pages not fetched yet, then sitemap pages
// past the page budget.

import { STATUS_CLASSES, type StatusClass } from '../../status'

// Above this many ticks, each tick stands for a run of consecutive pages.
export const MAX_TICKS = 20000
// A compact tape (a row in a list) is one short line, so it aggregates much sooner.
export const MAX_COMPACT_TICKS = 160

export type TickKind = StatusClass | 'todo' | 'cut'

export interface Tick {
  kind: TickKind
  // A fetched page that is not in the sitemap (aggregated: any page in the run).
  off: boolean
}

// Worst first. A run takes the worst status among its pages; any fetched page outranks the
// outline ticks, and "not fetched yet" outranks "past the budget".
const RANK: Record<TickKind, number> = { '5': 6, x: 5, '4': 4, '3': 3, '2': 2, todo: 1, cut: 0 }

function charKind(c: string): StatusClass {
  return c === '2' || c === '3' || c === '4' || c === '5' ? c : 'x'
}

function whole(n: number): number {
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0
}

export function buildTicks(tape: string, tapeOff = '', notFetched = 0, pastBudget = 0, max = MAX_TICKS): Tick[] {
  const todo = whole(notFetched)
  const cut = whole(pastBudget)
  const total = tape.length + todo + cut
  const per = Math.max(1, Math.ceil(total / Math.max(1, max)))
  const kindAt = (i: number): TickKind => {
    if (i < tape.length) return charKind(tape[i])
    return i < tape.length + todo ? 'todo' : 'cut'
  }

  const ticks: Tick[] = []
  for (let start = 0; start < total; start += per) {
    const end = Math.min(total, start + per)
    let kind = kindAt(start)
    let off = false
    for (let i = start; i < end; i++) {
      const k = kindAt(i)
      if (RANK[k] > RANK[kind]) kind = k
      if (i < tape.length && tapeOff[i] === '1') off = true
    }
    ticks.push({ kind, off })
  }
  return ticks
}

export interface TapeCounts {
  fetched: number
  byClass: Record<StatusClass, number>
  off: number
}

export function countTape(tape: string, tapeOff = ''): TapeCounts {
  const byClass: Record<StatusClass, number> = { '2': 0, '3': 0, '4': 0, '5': 0, x: 0 }
  let off = 0
  for (let i = 0; i < tape.length; i++) {
    byClass[charKind(tape[i])]++
    if (tapeOff[i] === '1') off++
  }
  return { fetched: tape.length, byClass, off }
}

// How each status class reads after a count, singular and plural.
const PHRASE: Record<StatusClass, [string, string]> = {
  '2': ['OK', 'OK'],
  '3': ['redirected', 'redirected'],
  '4': ['not found or blocked', 'not found or blocked'],
  '5': ['server error', 'server errors'],
  x: ['without a response', 'without a response'],
}

function n(count: number): string {
  return count.toLocaleString('en-US')
}

function pages(count: number): string {
  return count === 1 ? 'page' : 'pages'
}

// Where the crawl stands, which decides what the slate outline ticks (sitemap pages not
// fetched) are called: still to come while it runs, "not reached by links" once it finished
// on its own, plain "not fetched" once it was stopped or cut short (links may exist to them).
export type TapeState = 'live' | 'done' | 'stopped'

export const OUTLINE_LABEL: Record<TapeState, string> = {
  live: 'Not fetched yet',
  done: 'Not reached by links',
  stopped: 'Not fetched',
}

// describeTape is the tape's text alternative (its aria-label), and the sentence a view can
// show under it: every count the tape draws, in words. state names the slate outline.
export function describeTape(counts: TapeCounts, notFetched = 0, pastBudget = 0, state: TapeState = 'live'): string {
  const parts = STATUS_CLASSES.filter((k) => counts.byClass[k] > 0).map((k) => {
    const c = counts.byClass[k]
    return `${n(c)} ${PHRASE[k][c === 1 ? 0 : 1]}`
  })
  let text = `${n(counts.fetched)} ${pages(counts.fetched)} fetched${parts.length ? `: ${parts.join(', ')}.` : '.'}`
  if (counts.off > 0) {
    text += ` ${n(counts.off)} fetched ${counts.off === 1 ? 'page is' : 'pages are'} not in the sitemap.`
  }
  const todo = whole(notFetched)
  if (todo > 0) {
    text += ` ${n(todo)} sitemap ${pages(todo)} ${OUTLINE_LABEL[state].toLowerCase()}.`
  }
  const cut = whole(pastBudget)
  if (cut > 0) {
    text += ` ${n(cut)} sitemap ${pages(cut)} past the page budget.`
  }
  return text
}
