import { STATUS_CLASSES, STATUS_LABEL } from '../../status'
import { OUTLINE_LABEL, type TapeState } from './tapeModel'
import './Tape.css'

// The key under a full-size tape. sitemap adds the notch and outline entries, which only a
// crawl with a sitemap draws. state names the slate outline (see OUTLINE_LABEL).
export default function TapeLegend({ sitemap = false, state = 'live' }: { sitemap?: boolean; state?: TapeState }) {
  return (
    <ul className="tape-legend">
      {STATUS_CLASSES.map((k) => (
        <li key={k}>
          <i className={`tick tick-${k}`} aria-hidden="true" />
          {STATUS_LABEL[k]}
        </li>
      ))}
      {sitemap && (
        <>
          <li>
            <i className="tick tick-2 tick-off" aria-hidden="true" />
            Not in the sitemap (notched)
          </li>
          <li>
            <i className="tick tick-todo" aria-hidden="true" />
            {OUTLINE_LABEL[state]}
          </li>
          <li>
            <i className="tick tick-cut" aria-hidden="true" />
            Past the page budget
          </li>
        </>
      )}
    </ul>
  )
}
