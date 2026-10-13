package crawler

// The crawl tape is one character per recorded page, in recording order. The web API streams
// it live, runner.Run saves it in Coverage.Tape / TapeOff, and crawl lists show a preview. All
// three use these helpers, so a saved tape matches the live one character for character.

// TapePreviewLen is the length of a crawl list's tape preview: one compact row in the UI
// (MAX_COMPACT_TICKS in web/src/components/tape/tapeModel.ts).
const TapePreviewLen = 160

// TapeChar is the tape character for a recorded page's Progress.Status: '2', '3', '4' or '5'
// for the status class, 'x' for a failed fetch (status 0) or anything outside 200-599.
func TapeChar(status int) byte {
	switch {
	case status >= 200 && status < 300:
		return '2'
	case status >= 300 && status < 400:
		return '3'
	case status >= 400 && status < 500:
		return '4'
	case status >= 500 && status < 600:
		return '5'
	default:
		return 'x'
	}
}

// tapeRank orders tape characters worst first, the rule the UI's Tape component uses when one
// tick stands for several pages: 5xx, then failed fetches, then 4xx, 3xx, 2xx. An unknown
// character ranks as a failed fetch ('x'), as the UI's charKind does.
func tapeRank(c byte) int {
	switch c {
	case '5':
		return 5
	case '4':
		return 3
	case '3':
		return 2
	case '2':
		return 1
	default: // 'x' and anything unknown
		return 4
	}
}

// TapePreview shortens tape to at most limit characters for a crawl list. Each output
// character stands for a run of ceil(len/limit) consecutive pages and takes the worst status
// among them, so a single 5xx in a long crawl still shows. A tape that already fits is
// returned unchanged; limit <= 0 returns "".
func TapePreview(tape string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(tape) <= limit {
		return tape
	}
	per := (len(tape) + limit - 1) / limit
	out := make([]byte, 0, (len(tape)+per-1)/per)
	for start := 0; start < len(tape); start += per {
		end := min(start+per, len(tape))
		worst := tape[start]
		for i := start + 1; i < end; i++ {
			if tapeRank(tape[i]) > tapeRank(worst) {
				worst = tape[i]
			}
		}
		out = append(out, worst)
	}
	return string(out)
}
