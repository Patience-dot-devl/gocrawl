package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/report"
)

// TestReadReportRoundTripsGolden reads the report golden file (internal/report/testdata) back
// through readReport, which decodes with strict encoding/json/v2 defaults, and writes it out
// again. The bytes must not change: a report gocrawl wrote must always load, for render,
// history and compare, and lose nothing on the way.
func TestReadReportRoundTripsGolden(t *testing.T) {
	path := filepath.Join("..", "report", "testdata", "report.golden.json")
	rep, err := readReport(path)
	if err != nil {
		t.Fatalf("readReport(golden): %v", err)
	}
	var buf bytes.Buffer
	if err := (report.JSONReporter{}).Write(&buf, rep); err != nil {
		t.Fatal(err)
	}
	want := mustRead(t, path)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("golden report changed in a read/write round trip.\n got:\n%s\nwant:\n%s", buf.Bytes(), want)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
