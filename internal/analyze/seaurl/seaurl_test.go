package seaurl_test

import (
	"reflect"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze/seaurl"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		tagged      bool
		values      map[string]string
		duplicates  []string
		casingMixed []string
		empty       []string
	}{
		{
			name:   "fully tagged",
			url:    "https://example.com/?utm_source=newsletter&utm_medium=email&utm_campaign=spring&utm_term=shoes&utm_content=cta",
			tagged: true,
			values: map[string]string{
				"utm_source": "newsletter", "utm_medium": "email", "utm_campaign": "spring",
				"utm_term": "shoes", "utm_content": "cta",
			},
		},
		{
			name:   "partial tagging",
			url:    "https://example.com/?utm_source=newsletter",
			tagged: true,
			values: map[string]string{"utm_source": "newsletter"},
		},
		{
			name:        "mixed casing",
			url:         "https://example.com/?UTM_Source=newsletter&utm_medium=email",
			tagged:      true,
			values:      map[string]string{"utm_source": "newsletter", "utm_medium": "email"},
			casingMixed: []string{"UTM_Source"},
		},
		{
			name:       "duplicate param",
			url:        "https://example.com/?utm_term=a&utm_term=b",
			tagged:     true,
			values:     map[string]string{"utm_term": "a"},
			duplicates: []string{"utm_term"},
		},
		{
			name:   "empty value",
			url:    "https://example.com/?utm_campaign=",
			tagged: true,
			values: map[string]string{},
			empty:  []string{"utm_campaign"},
		},
		{
			name:   "no utm",
			url:    "https://example.com/?q=1&page=2",
			tagged: false,
			values: map[string]string{},
		},
		{
			name:   "non-utm params ignored",
			url:    "https://example.com/?utm_source=x&ref=abc",
			tagged: true,
			values: map[string]string{"utm_source": "x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := seaurl.Parse(tt.url)
			if got.Tagged() != tt.tagged {
				t.Errorf("Tagged() = %v, want %v", got.Tagged(), tt.tagged)
			}
			if !reflect.DeepEqual(got.Values, tt.values) {
				t.Errorf("Values = %v, want %v", got.Values, tt.values)
			}
			if !reflect.DeepEqual(got.Duplicates, tt.duplicates) {
				t.Errorf("Duplicates = %v, want %v", got.Duplicates, tt.duplicates)
			}
			if !reflect.DeepEqual(got.CasingMixed, tt.casingMixed) {
				t.Errorf("CasingMixed = %v, want %v", got.CasingMixed, tt.casingMixed)
			}
			if !reflect.DeepEqual(got.Empty, tt.empty) {
				t.Errorf("Empty = %v, want %v", got.Empty, tt.empty)
			}
		})
	}
}

func TestMissing(t *testing.T) {
	u := seaurl.Parse("https://example.com/?utm_source=x")
	missing := u.Missing(seaurl.RequiredUTMKeys)
	want := []string{"utm_medium", "utm_campaign"}
	if !reflect.DeepEqual(missing, want) {
		t.Errorf("Missing = %v, want %v", missing, want)
	}
}

func TestPresentKeysIncludesEmpty(t *testing.T) {
	u := seaurl.Parse("https://example.com/?utm_source=x&utm_medium=")
	got := u.PresentKeys()
	want := []string{"utm_source", "utm_medium"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PresentKeys = %v, want %v", got, want)
	}
}

func TestParseUnparseable(t *testing.T) {
	// A control character makes url.Parse fail; Parse must not panic and reports untagged.
	if got := seaurl.Parse("http://\x7f/"); got.Tagged() {
		t.Errorf("expected untagged for unparseable URL, got %+v", got)
	}
}
