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

  async function act(verb: 'pause' | 'resume', action: () => Promise<void>) {
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

  return (
    <>
      {status === 'paused' ? (
        <Button variant="go" disabled={busy} onClick={() => void act('resume', () => resumeCrawl(id, resumeRate))}>
          Resume crawl
        </Button>
      ) : (
        phase !== 'analyzing' && (
          <Button variant="pause" disabled={busy} onClick={() => void act('pause', () => pauseCrawl(id))}>
            Pause
          </Button>
        )
      )}
      <Button variant="stop" onClick={onStop}>
        Stop crawl
      </Button>
    </>
  )
}
