package report

import "testing"

// TestExplanationsComplete guards against half-filled entries: every registered
// explanation must populate all three fields so the HTML report never shows a
// blank What/Impact/Fix row.
func TestExplanationsComplete(t *testing.T) {
	for code, e := range explanations {
		if e.What == "" {
			t.Errorf("explanation %q has empty What", code)
		}
		if e.Impact == "" {
			t.Errorf("explanation %q has empty Impact", code)
		}
		if e.Fix == "" {
			t.Errorf("explanation %q has empty Fix", code)
		}
	}
}

func TestExplainUnknownCode(t *testing.T) {
	if got := explain("no-such-code-exists"); got != nil {
		t.Errorf("explain(unknown) = %v, want nil", got)
	}
}

func TestExplainKnownCode(t *testing.T) {
	got := explain("missing-title")
	if got == nil {
		t.Fatal("explain(missing-title) = nil, want explanation")
	}
	if got.Fix == "" {
		t.Error("expected a non-empty Fix for missing-title")
	}
}
