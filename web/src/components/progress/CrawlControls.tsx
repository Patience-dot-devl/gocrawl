import { useState } from 'react'
import Button from '../ui/Button'
import { pauseCrawl, resumeCrawl } from '../../api/pause'
import type { CrawlStatus } from '../../types/crawls'
import type { RunPhase } from '../../types/progress'

interface Props {
  id: string
  status: CrawlStatus
  phase?: RunPhase
  /** Rate for Resume crawl; undefined keeps the current rate. */
  resumeRate?: number
  /** Called after Pause or Resume crawl succeeds, to poll right away. */
  onChanged: () => void
  /** Called when Pause or Resume crawl fails; verb says which. */
  onError: (verb: 'pause' | 'resume', err: unknown) => void
  /** Stop crawl asks for confirmation, which CrawlReport owns. */
  onStop: () => void
}

/** Yellow Pause while running, green Resume crawl while paused, red Stop crawl in both. */
export default function CrawlControls({ id, status, phase, resumeRate, onChanged, onError, onStop }: Props) {
  const [busy, setBusy] = useState(false)

  // The button stays enabled in the DOM while a request runs (aria-disabled, with the click
  // ignored): a disabled button loses keyboard focus. Pause and Resume crawl are also one
  // button whose label changes, so the node, and focus with it, survives the swap.
  async function act(verb: 'pause' | 'resume', action: () => Promise<void>) {
    if (busy) return
    setBusy(true)
    try {
      await action()
      onChanged()
    } catch (err) {
      onError(verb, err)
    } finally {
      setBusy(false)
    }
  }

  const paused = status === 'paused'

  return (
    <>
      {(paused || phase !== 'analyzing') && (
        <Button
          variant={paused ? 'go' : 'pause'}
          aria-disabled={busy || undefined}
          onClick={() => void (paused ? act('resume', () => resumeCrawl(id, resumeRate)) : act('pause', () => pauseCrawl(id)))}
        >
          {paused ? 'Resume crawl' : 'Pause'}
        </Button>
      )}
      <Button variant="stop" onClick={onStop}>
        Stop crawl
      </Button>
    </>
  )
}
