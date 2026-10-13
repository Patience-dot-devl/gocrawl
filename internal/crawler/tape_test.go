package crawler

import (
	"strings"
	"testing"
)

func TestTapeChar(t *testing.T) {
	for status, want := range map[int]byte{200: '2', 204: '2', 301: '3', 404: '4', 429: '4', 503: '5', 0: 'x', 99: 'x', 600: 'x'} {
		if got := TapeChar(status); got != want {
			t.Errorf("TapeChar(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestTapePreview(t *testing.T) {
	if got := TapePreview("2234", TapePreviewLen); got != "2234" {
		t.Errorf("a tape that fits = %q, want it unchanged", got)
	}
	if got := TapePreview("2222", 0); got != "" {
		t.Errorf("limit 0 = %q, want empty", got)
	}
	// Worst status wins inside a run: 5xx > failed fetch > 4xx > 3xx > 2xx.
	for in, want := range map[string]string{"2345x": "5", "234x": "x", "2343": "4", "2223": "3"} {
		if got := TapePreview(in, 1); got != want {
			t.Errorf("TapePreview(%q, 1) = %q, want %q", in, got, want)
		}
	}
	// 301 pages, at most 160 ticks: runs of 2, so 151 ticks, the last one the lone 5xx.
	got := TapePreview(strings.Repeat("2", 300)+"5", TapePreviewLen)
	if len(got) != 151 || got[150] != '5' || strings.Count(got, "5") != 1 {
		t.Errorf("TapePreview(300 x 2, then 5) = %d chars %q, want 151 ending in the single 5", len(got), got)
	}
}

// A very large crawl (100,000 pages) still previews in TapePreviewLen characters and keeps the
// one failure that would otherwise drown in green.
func TestTapePreviewLargeCrawl(t *testing.T) {
	in := []byte(strings.Repeat("2", 100_000))
	in[73_210] = '4'
	got := TapePreview(string(in), TapePreviewLen)
	if len(got) > TapePreviewLen {
		t.Fatalf("preview is %d chars, want at most %d", len(got), TapePreviewLen)
	}
	if strings.Count(got, "4") != 1 {
		t.Errorf("preview %q lost the single 4xx", got)
	}
}
