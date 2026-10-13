import { statusClass } from '../../status'
import './StatusCode.css'

// An HTTP status code in the mono face, colored by class: 2xx green, 3xx yellow, 4xx orange,
// 5xx red (the spec's status colors, as on the tape). 0 means the fetch failed and reads
// "Error".
export default function StatusCode({ code }: { code: number }) {
  return <span className={`status-code status-code-${statusClass(code)}`}>{code > 0 ? code : 'Error'}</span>
}
