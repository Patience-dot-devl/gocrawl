import { useId } from 'react'
import Notice from '../ui/Notice'
import type { ProgressView } from '../../types/progress'
import { formatCount, formatRate } from './tapeText'

interface Props {
  progress: ProgressView
  /** The "Resume at" field's text; starts at half the current rate. */
  rate: string
  onRateChange: (value: string) => void
}

/** Explains an automatic pause after repeated 429/503 responses and asks for a lower rate. */
export default function AutoPauseNotice({ progress, rate, onRateChange }: Props) {
  const inputId = useId()
  const retry = progress.retry_after_seconds ?? 0
  return (
    <Notice
      tone="warning"
      action={
        <div className="ap-rate">
          <label htmlFor={inputId}>Resume at (requests per second)</label>
          <input
            id={inputId}
            name="resume-rate"
            type="number"
            min="0.001"
            max="1000"
            step="any"
            inputMode="decimal"
            value={rate}
            onChange={(e) => onRateChange(e.target.value)}
          />
        </div>
      }
    >
      <p>
        The site kept answering 429 Too Many Requests (or 503), even after slowing to {formatRate(progress.rate)} requests per
        second, so the crawl paused itself.
        {retry > 0 ? ` The server asked for a ${formatCount(retry)}-second wait (Retry-After).` : ''} Resume at a lower rate, or
        stop and keep the {formatCount(progress.fetched)} pages fetched so far.
      </p>
    </Notice>
  )
}
