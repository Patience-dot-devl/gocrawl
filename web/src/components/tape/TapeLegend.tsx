import { STATUS_CLASSES, STATUS_LABEL } from '../../status'
import './Tape.css'

// The key under a full-size tape. sitemap adds the notch and outline entries, which only a
// live crawl with a sitemap draws (saved crawls rebuild a status-only tape). finished renames
// the slate outline from "Not fetched yet" to "Not reached by links".
export default function TapeLegend({ sitemap = false, finished = false }: { sitemap?: boolean; finished?: boolean }) {
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
            {finished ? 'Not reached by links' : 'Not fetched yet'}
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
