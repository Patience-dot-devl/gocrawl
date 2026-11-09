// Package jsonopts holds the encoding/json/v2 options gocrawl decodes JSON with.
//
// gocrawl decodes two kinds of JSON. Its own data (web API requests, saved reports, settings
// files) gets the strict v2 defaults. JSON taken from crawled sites (JSON-LD, dataLayer
// snapshots, Shopify and WordPress endpoints) is third-party input that gocrawl reports on
// rather than trusts, so it is decoded with Crawled, which keeps the leniency of
// encoding/json v1: a malformed byte or a repeated key must not hide a page's markup from the
// analyzers.
package jsonopts

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
)

// Crawled decodes JSON from crawled pages the way encoding/json v1 did:
//
//   - invalid UTF-8 and lone surrogate escapes become U+FFFD instead of an error;
//   - a repeated object key is accepted, and the last value wins;
//   - struct field names match case-insensitively.
var Crawled = json.JoinOptions(
	jsontext.AllowInvalidUTF8(true),
	jsontext.AllowDuplicateNames(true),
	json.MatchCaseInsensitiveNames(true),
)
