package jsonopts_test

import (
	json "encoding/json/v2"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/jsonopts"
)

func TestCrawledIsLenientLikeV1(t *testing.T) {
	cases := []struct {
		name, in string
	}{
		{"invalid UTF-8", "{\"name\":\"caf\xe9\"}"},
		{"lone surrogate escape", `{"name":"x\ud800y"}`},
		{"duplicate key", `{"name":"first","name":"last"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var strict map[string]any
			if err := json.Unmarshal([]byte(tc.in), &strict); err == nil {
				t.Fatalf("v2 defaults accepted %q; the test no longer shows why Crawled is needed", tc.in)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(tc.in), &got, jsonopts.Crawled); err != nil {
				t.Fatalf("Crawled rejected %q: %v", tc.in, err)
			}
		})
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(`{"name":"first","name":"last"}`), &m, jsonopts.Crawled); err != nil || m["name"] != "last" {
		t.Errorf("duplicate key: got %v, %v; want the last value", m, err)
	}

	var s struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(`{"Title":"Shoe"}`), &s, jsonopts.Crawled); err != nil || s.Title != "Shoe" {
		t.Errorf("case-insensitive match: got %q, %v", s.Title, err)
	}
}
