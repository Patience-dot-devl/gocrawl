# Structured-data depth and a Shopify analyzer

Date: 2026-09-14
Status: approved, not yet implemented

## Problem

`gocrawl` can tell you *whether* a page carries JSON-LD and roughly what `@type` it
declares. It cannot tell you whether that JSON-LD would earn a rich result, whether it
contradicts the page it sits on, or whether a given page is missing the markup its
template calls for. For an e-commerce audit — the case that prompted this work, a Shopify
store reviewed for structured-data opportunities — that is most of the value.

Two real pages measured while designing this:

- A Kith product page (Shopify, near-default theme) emits `Product` with `brand`,
  `description`, `image`, `name`, `offers`, `sku`, `url`; a nested `Offer` with
  `availability`, `price`, `priceCurrency`, `sku`, `url`; and a separate `BreadcrumbList`.
  It carries no `gtin`/`mpn`, `priceValidUntil`, `aggregateRating`, `review`,
  `shippingDetails`, `hasMerchantReturnPolicy`, and no `ProductGroup`/`hasVariant` despite
  offering variants.
- An Allbirds product page emits a single block typed `["CollectionPage", "FAQPage"]` and
  no `Product` at all.

The current `structured` analyzer reports both pages as having structured data and raises
nothing actionable about either.

## Goals

1. Judge structured data against Google's rich-result requirements, separating fields that
   block eligibility from fields that merely improve the result.
2. Surface Google Merchant / free-listing fields on `Product`, which Shopify themes almost
   never emit and which materially affect shopping surfaces.
3. Catch structured data that contradicts itself or the page — the failure mode that
   silently disqualifies a page that looks correctly marked up.
4. Give Shopify stores template-aware coverage: which page types exist, what schema each
   should carry, and what is missing.
5. Keep a 500-page store's report readable.

## Non-goals

- Microdata and RDFa extraction. Considered and cut; JSON-LD is what Shopify themes and
  SEO apps emit, and adding two more syntaxes buys little for this use case.
- Full schema.org vocabulary validation. The field tables stay a curated subset tied to
  documented rich-result requirements.
- Fetching Google's Rich Results Test or any external validation API. Every check is
  local and offline.

## Architecture

Three packages, one new seam.

```
crawler.Page.Doc
  └─► schemaorg.Parse(doc) ──► Graph          (internal/analyze/schemaorg, pure, not an analyzer)
        ├─► structured  — eligibility, integrity, rollup   (platform-neutral)
        └─► shopify     — detection, templates, Shopify pitfalls
```

`schemaorg` follows the precedent set by `seaurl`: a shared parsing helper that is not
registered as an analyzer and emits no `Issue`. Both consumers stay pure per the project's
analyzer contract.

### `internal/analyze/schemaorg`

```go
// Node is one typed schema.org object, flattened out of whatever nesting it arrived in.
type Node struct {
    Types []string       // @type, normalized to a slice
    ID    string         // @id, empty if absent
    Props map[string]any
    Block int            // index of the <script> element it came from
    Path  string         // dotted position, e.g. "Product.offers"
}

type Graph struct{ Nodes []Node }

func Parse(doc *goquery.Document) (Graph, []ParseError)

func (g Graph) OfType(t string) []Node
func (g Graph) Resolve(id string) (Node, bool)
func (g Graph) Has(types ...string) bool
```

`Parse` walks every `<script type="application/ld+json">`, descending `@graph`, arrays, and
nested objects, recording each typed object as a `Node`. `Block` is what makes duplicate-
and conflict-detection possible: two `Product` nodes from the same block are a modelling
choice, two from different blocks are usually two competing sources. `Path` lets a finding
name where the problem sits rather than just which type it was on.

Parse failures return as `[]ParseError` rather than becoming issues. `structured` converts
them to `structured-invalid-jsonld`; `shopify` ignores them.

Typed property readers absorb schema.org's string-or-array-or-object looseness in one
place:

```go
func (n Node) Str(path string) string
func (n Node) Strs(path string) []string
func (g Graph) NodeAt(n Node, path string) (Node, bool)
func (g Graph) NodesAt(n Node, path string) []Node
```

Dotted paths (`offers.price`) resolve through nested objects and follow `@id` references
into the rest of the graph.

`collectTypes`, `asStrings`, and `hasField` move out of `structured` into this package.

### `internal/analyze/structured`

Keeps its registered name and every existing issue code. The 361-line single file splits
into `structured.go` (analyzer wiring), `eligibility.go`, `integrity.go`, and
`candidates.go` — the four existing `*-candidate` heuristics move to the last of these
unchanged.

#### Eligibility

One table entry per type:

```go
type spec struct {
    required    []string // absence blocks the rich result
    recommended []string // absence degrades it
    merchant    []string // Product only: Merchant / free-listing surfaces
}
```

`Product`:

| Tier | Fields |
| --- | --- |
| required | `name`, `image`, `offers.price`, `offers.priceCurrency`, `offers.availability` |
| recommended | `brand`, `sku`, `description`, `aggregateRating`, `review` |
| merchant | `gtin` or `mpn`, `priceValidUntil`, `offers.shippingDetails`, `hasMerchantReturnPolicy` |

The existing `requiredFields` map supplies the starting point for the other types
(`Article`, `NewsArticle`, `BlogPosting`, `Recipe`, `Event`, `Organization`,
`LocalBusiness`, `Person`, `BreadcrumbList`, `FAQPage`, `VideoObject`); each gains a
`recommended` list drawn from the matching rich-result documentation.

One deliberate behaviour change: `Offer` stops being a top-level table entry, because its
fields are now reached through `Product` via dotted paths. A page emitting a bare
top-level `Offer` with no `price` currently raises `structured-missing-required` and will
stop doing so. This is the only intended regression in the phase 1 refactor; the pinning
tests are written to expect it explicitly rather than to pass by accident.

Codes:

| Code | Severity | Scope | Data |
| --- | --- | --- | --- |
| `structured-missing-required` | warning | per page | `type`, `missing` (unchanged) |
| `structured-missing-recommended` | info | site | `type`, `missing` (field → page count), `pages`, `examples` |
| `structured-missing-merchant` | info | site | same shape |

Site-scoped issues attach to the site base URL and aggregate across the crawl, the same
way the `security` analyzer's audit aggregates per host. `examples` caps at five URLs.
Recommended and merchant gaps get separate codes so that merchant opportunities — the
high-value ones for a store — do not drown in generic recommended-field noise.

#### Integrity

All per page.

| Code | Severity | Fires when |
| --- | --- | --- |
| `structured-duplicate-type` | warning | Two blocks each emit `Product`, `Organization`, or `BreadcrumbList` on one page |
| `structured-conflicting-value` | error | A duplicated type disagrees between blocks on `name`, `offers.price`, or `sku` |
| `structured-price-mismatch` | warning | `offers.price` differs from the price rendered on the page |
| `structured-unresolved-id` | warning | An `{"@id": "..."}` reference has no matching node in the page graph |
| `structured-relative-url` | warning | `url`, `image`, or `logo` holds a relative path |
| `structured-invalid-date` | warning | `datePublished`, `startDate`, or `priceValidUntil` is not ISO 8601 |
| `structured-malformed-price` | warning | `price` carries a currency symbol, thousands separator, or a range (`"$1,299.00"`, `"19-29"`) |

`structured-price-mismatch` reuses the existing `priceRe` regexp. It compares numeric
values after stripping formatting, and stays silent when the page shows more than one
distinct price (a variant selector or a strikethrough sale price) so that a legitimately
ambiguous page does not produce a false positive.

### `internal/analyze/shopify`

Modelled on `wordpress`: aggregate detection across the crawl, bail silently when the site
is not Shopify, then run per-page and site-level checks.

Detection signals, any one sufficient: `cdn.shopify.com` asset URLs, a `Shopify.theme`
inline JavaScript object, the `shopify-features` script, a `myshopify.com` host reference,
or an `X-ShopId` / `X-Shopify-Stage` response header. Emits `shopify-detected` carrying the
theme name and version read from `Shopify.theme` when present.

Template classification, from the URL path:

| Path pattern | Template | Expected schema |
| --- | --- | --- |
| `/products/<handle>` | product | `Product`, `BreadcrumbList` |
| `/collections/<c>/products/<handle>` | product (duplicate path) | `Product`, `BreadcrumbList` |
| `/collections/<handle>` | collection | `CollectionPage` or `ItemList`, `BreadcrumbList` |
| `/blogs/<blog>/<article>` | article | `BlogPosting` or `Article`, `BreadcrumbList` |
| `/blogs/<blog>` | blog index | `Blog` |
| `/pages/<handle>` | page | `WebPage` |
| `/` | home | `Organization`, `WebSite` |
| `/cart`, `/search`, `/account/*` | utility | none expected |

Gaps report as `shopify-template-schema-gap` (warning, site-scoped) with
`{template, expected, present, pages, examples}` — one issue per template, not per page.

Structured-data checks:

| Code | Severity | Fires when |
| --- | --- | --- |
| `shopify-schema-app-conflict` | error | Two `Product` blocks whose source scripts fingerprint to different emitters (theme vs. JSON-LD for SEO, Schema Plus, SearchPie, Yoast for Shopify) |
| `shopify-flat-variant-product` | warning | The page exposes a variant selector but emits one flat `Product` with no `ProductGroup` / `hasVariant` |
| `shopify-single-offer-range` | info | Variants carry distinct prices but the markup uses a single `Offer` rather than an `AggregateOffer` |
| `shopify-schema-client-injected` | info | A known SEO-app script is present and the page has no JSON-LD at all — suggests re-running with `--render headless` |

App fingerprinting reads the script URLs and inline markers on the page and attributes each
JSON-LD block to the nearest preceding recognized emitter, defaulting to "theme". A
conflict requires two *different* attributed sources, so a theme that legitimately emits two
blocks does not trip it.

`shopify-schema-client-injected` is the concession to raw-mode crawling: Shopify themes emit
JSON-LD server-side, but several SEO apps inject it client-side, and a raw crawl would
otherwise report those pages as bare.

General SEO checks (phase 3):

| Code | Severity | Fires when |
| --- | --- | --- |
| `shopify-indexable-search` | warning | `/search` is crawlable and not `noindex` |
| `shopify-indexable-facet` | warning | Crawlable `?sort_by=` / faceted collection URLs without a self-referential canonical |
| `shopify-duplicate-product-path` | warning | A product reachable at both `/products/<h>` and `/collections/<c>/products/<h>` without a canonical pointing at the former |
| `shopify-products-json-exposed` | info | `/products.json` returns a product feed (opt-in; an extra fetch) |
| `shopify-default-theme-meta` | warning | A default theme title or description template survives in production |

### Plumbing

`shopify` registers in `runner.BuildRegistry` immediately after `wordpress`. It takes the
`crawler.Fetcher`, used only for the `/products.json` probe, which rides the existing
`Specialized` flag since it is an extra fetch. No new `RegistryOptions` field: the deep
checks run by default, with volume controlled by site-level aggregation rather than by a
gate.

Every new code gets an entry in `internal/report/explanations.go`. `docs/analyzers.md`
gains a `shopify` section and an expanded `structured` table. `CLAUDE.md`'s registered-
analyzer list and package map gain `shopify` and `schemaorg`.

## Phases

Each phase is independently shippable and testable.

1. **`schemaorg` + `structured` depth.** Extract the parsing seam, refactor `structured`
   onto it with behaviour pinned by tests first, then add the eligibility tables, the
   integrity checks, and the site-level rollup.
2. **`shopify` structured-data checks.** Detection, template classification, template gap
   reporting, app-conflict fingerprinting, variant modelling, client-injection warning.
3. **`shopify` general SEO checks.** The five checks in the table above.

## Testing

Every phase adds HTML fixtures under the owning package's `testdata/`.

- Phase 1 begins by pinning the current `structured` output — every existing code, on the
  existing fixtures — so the `schemaorg` extraction is provably behaviour-neutral before
  any new check lands.
- Eligibility tests cover a complete `Product`, one missing a required field, one missing
  only recommended fields, and one missing only merchant fields, asserting each lands in
  the right tier.
- Rollup tests assert that N pages with the same gap produce one issue carrying
  `pages == N`, not N issues.
- Integrity tests each get a fixture built to trip exactly one code, plus a negative
  fixture for the ambiguous-price case that `structured-price-mismatch` must stay silent on.
- Phase 2 fixtures derive from the two real page shapes measured above (a near-default
  Shopify `Product` + `BreadcrumbList` page, and a product page whose only block is
  `CollectionPage`/`FAQPage`), plus a synthetic page carrying two competing `Product`
  blocks from different attributed sources.
- A non-Shopify fixture asserts the `shopify` analyzer emits nothing at all.

## Risks

- **The `structured` refactor touches a widely-referenced analyzer.** Mitigated by pinning
  behaviour with tests before extracting, and by keeping every existing code and its `Data`
  shape unchanged.
- **App fingerprinting is heuristic.** Script-URL attribution can mis-assign a block. The
  conflict check requires two distinct sources before firing, and the finding names the
  attributed sources so a reader can judge it.
- **Field tables drift** as Google changes rich-result requirements. They are data, in one
  file, with the tier split documented here — a drift is an edit, not a redesign.
