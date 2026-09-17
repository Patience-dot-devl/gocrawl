# Structured-data & Shopify analyzer — follow-up work

Date: 2026-09-14
Status: **P0–P2, P4, P5 and F10–F13 fixed** (2026-09-15 `f3d714f..98498ae`, 2026-09-17 `7aff116..519d669`); P3 applied with one row reverted; P6–P9 open
Branch: `shopify-structured-data-analyzer`
Spec: [`../specs/2026-09-14-structured-data-shopify-design.md`](../specs/2026-09-14-structured-data-shopify-design.md)
Plan: [`../plans/2026-09-14-structured-data-shopify.md`](../plans/2026-09-14-structured-data-shopify.md)

The sixteen planned tasks are complete, reviewed and committed (`2f1b028..8b339cc`). This
document records what two end-of-run reviews and one live smoke test found *afterwards*.

**Update 2026-09-15.** A fix wave landed in `f3d714f..98498ae`, closing everything marked ✅ below.
Verified end-to-end against live Allbirds markup served locally: the sitewide false positive is
gone while all four genuine findings on that page survive, and eight crawls of the same page now
produce byte-identical output (before, it permuted between runs).

**Update 2026-09-17.** Second wave, `7aff116..519d669`: F10 (diff identity), F12 (relative
canonicals), F11 (four guard tests, each mutation-verified), F13 (doc comment), and F4 decided
as *check variants, rolled up*. Checking F4 against Google's product-variants page also showed
P3's `ProductGroup` row was wrong — see the note there.

Items are ordered by what they cost a user, not by effort. Each carries its status.

---

## P0 — Blocks merge

### ✅ F0. `schemaorg.walk` ranges a map, so every report churns between identical crawls

**FIXED** in `f3d714f`. Keys are sorted before descending; 3 "document order" doc comments corrected to "stable". Verified: 8 identical crawls of a real page now hash identically.

`internal/analyze/schemaorg/schemaorg.go:107` — `for key, val := range t`. Sibling typed children
are appended to `g.Nodes` in Go's randomized map order.

This is the **fourth** map-iteration determinism bug in this work, and by far the worst placed: it
sits upstream of everything. `Graph.Types()` (whose doc comment at :160 promises "document
order"), `OfType`, `index()`'s first-declaration-wins for duplicate `@id`, `requiredIssues`'
iteration order, and therefore `rollup.order`.

Measured, not theorised: `Types()` on one `Product` with five typed siblings produced **5 distinct
orders in 300 parses**; a full `structured.Analyze` on one page produced **51 distinct outputs in
300 runs** — `structured-data`'s `Data["types"]` permutes, the `structured-missing-required`
issues permute, and the site-wide rollup issues permute. Reports are diffed between crawls, so
this is a guaranteed spurious diff on an unchanged page, on every page.

Why no task review caught it: it lives in Task 1's file, which passed with zero findings, and only
manifests through Task 3 and Task 4's consumers. No task-scoped reviewer could see it.

Fix:
```go
keys := make([]string, 0, len(t))
for k := range t { keys = append(keys, k) }
sort.Strings(keys)
for _, key := range keys { val := t[key]; /* ... */ }
```
Then correct the `Types()` doc comment — JSON objects have no document order; the honest word is
"stable".

### ✅ F0b. The test named for this is vacuous

**FIXED** in `f3d714f`. Replaced with `TestTypesAreDeduplicatedAndStableAcrossParses` — object fixture, 50 repeated parses. Confirmed 5/5 failures on revert.

`internal/analyze/schemaorg/schemaorg_test.go:107`, `TestTypesAreDeduplicatedInDocumentOrder`,
uses a top-level JSON **array**, whose order is a slice — so it never exercises the map-key path
it is named after, and passes identically under the correct and the broken implementation.
Replace the fixture with one nested object carrying 3+ typed sibling properties and assert across
repeated parses.

---

## P1 — Bugs in the merchant tier (the feature this work exists for)

### ✅ F1. `priceValidUntil` and `hasMerchantReturnPolicy` are checked at paths that can never match

**FIXED** in `9ccf949`. Both are now any-of groups accepting either placement.

`internal/analyze/structured/eligibility.go:32,34` (`Product`) and `:48,50` (`ProductGroup`)
list bare `priceValidUntil` and `hasMerchantReturnPolicy` in the `merchant` tier. Google
documents both on `offers`. The bare paths never resolve, so **a store that emits these fields
correctly is still reported as missing them.**

The codebase already knows this: `internal/analyze/structured/integrity.go:195-196` lists both
`priceValidUntil` *and* `offers.priceValidUntil` in `dateProperties`.

Fix: change to any-of groups so either placement satisfies the check —
`priceValidUntil|offers.priceValidUntil` and
`hasMerchantReturnPolicy|offers.hasMerchantReturnPolicy`. `missingFields` already splits on `|`.

Test: a `Product` emitting `offers.priceValidUntil` must raise NO merchant gap for that field.
That test fails today.

### ✅ F1b. The `ProductGroup` merchant fix (commit `8b339cc`) introduced a false positive

**FIXED** in `9ccf949` via a `satisfiedOnVariants` fallback scoped to the merchant tier. The Allbirds regression guard still fires.

`internal/analyze/structured/eligibility.go:46-51`. The comment at :40-45 says Google accepts
these fields on the ProductGroup itself **or on each variant's Offer** — but the code only checks
the group. Verified with a fixture where every `hasVariant` child carries `gtin13`,
`priceValidUntil`, `offers.shippingDetails` and `hasMerchantReturnPolicy`: all four are still
reported missing.

So the fix traded the false negative it cured for a **false positive on the same population of
stores** — the shape this branch's own `shopify-flat-variant-product` tells owners to adopt is now
told it is missing everything. That is backwards for this package's house rule.

Fix: let satisfaction fall through to the variants. `g.HasValue(n, "hasVariant.gtin13")` and
`"hasVariant.offers.shippingDetails"` already resolve through the existing array-walking
machinery, so no engine change is needed. Do **not** simply widen the any-of group string — it
works, but the reported label becomes `gtin|gtin8|…|hasVariant.gtin|…`, unreadable in a report,
and it breaks `TestProductGroupWithHasVariantStillRaisesMerchantGap`'s label assertions. Prefer a
small `satisfiedOnVariants(g, n, field)` fallback in `missingFields`, gated to the merchant tier,
keeping the printed group name as-is. The Allbirds fixture (variant carries only `url`) still
fires, so that test's behaviour assertions stay green.

Note this interacts with F4 below: if the `.hasVariant` exemption is revisited, revisit this too.

### ✅ F2. `structured-missing-merchant` sits at `info`

**FIXED** in `9ccf949`. `rollupSeverity` now splits merchant (warning) from recommended (info).

It is the answer to "what structured-data opportunities does this store have", and `info` is the
severity that also means "no action needed". Raise to `warning` — **but only after F1**, or it
amplifies a false alarm.

---

## P2 — A false positive firing on every page of real stores

### ✅ F3. `structured-product-candidate` is satisfied by sitewide boilerplate

**FIXED** in `423c169`, and **A/B-verified on the live page that caused it**: before, 5 issues including `product-candidate $100`; after, the same 4 genuine findings with the false positive gone. Type suppression was rejected in favour of co-location — see the rejected-proposals note below, it still applies.

Observed three times on live stores: all 25 crawled pages of one store (signal `"$35.00"`), and
an Allbirds **collection** page (signal `"$100"`).

Cause: `hasProductSignal` (`internal/analyze/structured/candidates.go`) reads
`doc.Find("body").Text()` and requires only that a price and a cart call-to-action both appear
*somewhere* on the page. A Shopify mini-cart drawer puts "Add to cart" into every page; a
free-shipping-threshold widget puts a price into every page. The conjunction is satisfied by
boilerplate sitewide.

**A previously proposed fix — suppress when the page declares `CollectionPage`/`ItemList`/
`SearchResultsPage` — was considered and rejected.** It is insufficient (the 25/25 store fired on
homepages, `/pages/*`, `/policies/*` and blog articles, none of which declare those types) and
unsafe (the spec's own motivating example is an Allbirds *product* page emitting
`["CollectionPage","FAQPage"]` and no `Product`; suppression would silence the feature's headline
true positive).

Both end-of-run reviews agreed the suppression idea is directionally right but incomplete, and the
code review added one correction: **suppressing on a bare `ItemList` is unsafe.** `ItemList` is a
*component* type — a Shopify product template with an app-injected "related products" rail
declares one — so suppressing on it would silence the check on exactly the page where it matters.
`CollectionPage` and `SearchResultsPage` are page-level declarations and are safe. If `ItemList`
is used at all, scope it to a block-root (`Path == "ItemList"`, no dot).

Correct narrowing:
- Require the price and the cart CTA to be **co-located** inside `form[action*="/cart/add"]`,
  which is universal for Shopify's product form and is not what the mini-cart drawer posts to.
- Exclude text inside `<script>`, `<template>` and `aria-hidden`/`hidden` subtrees.
- Until narrowed, drop the severity to `info`.

---

## ✅ P3 — Field tiers that misstate Google's requirements

**APPLIED** in `98498ae` — all 10 rows, plus `offers.itemCondition` added to both merchant tiers. Three caveats raised by the implementer and left open: `WebSite.url → required` is the weakest row now that Sitelinks Search Box is gone; `brand` stays in `recommended` because Merchant Center's real rule is conditional (required only when there is no GTIN) and the tier model cannot express that; and `ProductGroup.variesBy` looks like it should also be required but was outside the instructed set.

`eligibility.go`'s tables were written from memory and never checked against Google's published
requirements. A field in the wrong tier either cries wolf or stays silent on something real.

| Type | Field | Now | Should be | Consequence today |
| --- | --- | --- | --- | --- |
| `Event` | `location` | recommended | **required** | Silent on markup Google will not show |
| `VideoObject` | `uploadDate` | recommended | **required** | Silent; no video rich result without it |
| `VideoObject` | `description` | recommended | **required** | Table has 2 of Google's 4 required fields |
| `Recipe` | `image` | recommended | **required** | Silent. (`recipeIngredient`/`recipeInstructions` are correctly recommended) |
| `LocalBusiness` | `address` | recommended | **required** | Silent |
| `Organization` | `logo`, `url` | recommended | required | Knowledge-panel logo under-reported |
| `ProductGroup` | `hasVariant`, `productGroupID` | recommended | ~~required~~ **recommended** | **Reverted in `519d669`.** Google's product-variants page requires only `name` on the group; variants may join via `isVariantOf` |
| `Product` | `offers.availability` | required | recommended | Cries wolf — Google lists it as recommended |
| `WebSite` | `potentialAction` | recommended | **drop** | Stale advice: sitelinks searchbox deprecated Nov 2023 |

Also missing from the merchant set: `offers.itemCondition`; the varying attribute
(`color`/`size`/`material`/`pattern`) Google requires on variants; and `brand` — Merchant Center
requires `brand`+`mpn` when there is no GTIN, but `brand` currently sits in `recommended`.

---

## ✅ P4 — Severity calibration

**APPLIED** in `98498ae` — `structured-invalid-jsonld`, `structured-malformed-price` and `structured-price-mismatch` are now `error`.

Users find the actionable set by filtering `severity == "error"`. These are invisible there today.

| Code | Now | Should be | Why |
| --- | --- | --- | --- |
| `structured-invalid-jsonld` | warning | **error** | Block is discarded whole; zero judgement, zero false-positive risk |
| `structured-malformed-price` | warning | **error** | Invalidates the Offer; Merchant Center disapproval |
| `structured-price-mismatch` | warning | **error** | Its own Impact text cites a manual action suppressing every rich result |
| `structured-missing-merchant` | info | warning | See F2 — after F1 only |
| `structured-product-candidate` | warning | info | Until F3 is narrowed |
| `structured-video-candidate` | warning | info | Until F7 is narrowed |

Note: `shopify-schema-app-conflict` is already `error`, so `structured-conflicting-value` was
never the only one.

---

## P5 — Open question, needs a human decision

### ✅ F4. Is the `.hasVariant` eligibility exemption right? — DECIDED: check, rolled up

**FIXED** in `519d669`. Inline variants are checked against Google's variant rules (name, image,
price, currency, sku/GTIN, each `variesBy` dimension) and reported once per type as
`structured-variant-incomplete`, counting pages not variants. url-only variants stay exempt:
they are Google's own multi-page reference shape, so the Allbirds pattern stays silent.
Untyped inline variants are not graph nodes and are not checked.

`listProperties` in `eligibility.go` exempts nodes under `.hasVariant` from all tier checks, on
the grounds that variants are deliberately thin copies (Allbirds' carry only a `url`).

**Two informed reviews disagree.** The audit-quality review argues variants are *not* thin like
collection tiles — Google requires `name`, `image`, `offers.price`/`priceCurrency`, `sku` and the
varying attribute on each — so the exemption is a blind spot on exactly the best-structured
stores. The implementer who added `ProductGroup.merchant` independently *agreed with keeping* the
exemption after reading the code.

Un-exempting would also produce per-variant findings on every product page, which is a real noise
cost. This needs a decision, not a ruling: it changes what the tool reports on every
`ProductGroup` store.

---

## P6 — New checks worth building (features, not fixes)

Ranked by commercial value to a store.

1. **Visible review stars with no `aggregateRating`.** Judge.me, Loox, Okendo, Yotpo and Stamped
   render a star widget (`[data-average-rating]`, `.jdgm-prev-badge`, `.spr-badge[data-rating]`,
   `.loox-rating`) while the theme's `Product` JSON-LD carries no rating. Review stars are the
   highest-CTR structured-data win in e-commerce, and the failure is invisible to the merchant —
   they see stars on the page. Sub-check: `aggregateRating` without `reviewCount`/`ratingCount` is
   invalid and Google drops it.
2. **`offers.availability` contradicting the page.** Shopify derives it from `product.available`,
   which is true if *any* variant is in stock — so a sold-out selected variant is routinely marked
   `InStock`. The page shows it ("Sold out" text, a `disabled` add-to-cart). This is a named
   Merchant Center disapproval reason, and it is the same comparison shape as the existing
   `structured-price-mismatch` check.
3. **`priceCurrency` contradicting the Markets locale.** A storefront served under `/en-ca/` or
   `/de/` emitting `"USD"` gets items disapproved. `Shopify.currency` is in the same bootstrap
   object the analyzer already parses for `Shopify.theme`.

---

## P7 — Noise and dead checks

- **`structured-video-candidate`** misfires on decorative hero video, which is standard in modern
  themes (Dawn ships a video section). A 3-second muted loop earns no rich result, so "add
  VideoObject" is bad advice. Exclude `<video autoplay muted loop>`; require `controls` or a
  `poster`. Also, a finding whose evidence string is the literal word `"video"` is unpersuasive.
- **`structured-article-candidate`** can pick a product card — many themes wrap them in
  `<article>`, and a sitewide `<time datetime>` in an announcement bar satisfies the date signal.
  Requiring the `<article>` not to contain a `/cart/add` form would close most of it.
- **`shopify-flat-variant-product`** is *true* on most stores (Shopify's own `| structured_data`
  filter emits a flat `Product`), which means ~300 identical per-page warnings on a 300-product
  crawl. It is a template fact — roll it up like `shopify-template-schema-gap`.
- **`shopify-single-offer-range`** depends on a `ProductJson`/`product-json` script id that is
  Debut-era. Dawn and most modern themes do not emit it, so the check is a silent no-op on the
  majority of current stores. Do not mistake its silence for a clean result.
- **`structured-missing-required` on `ItemList`** fires once per collection page on stores with a
  stub `ItemList` (Allbirds). The finding is true but is another per-page repetition of one
  template fact.
- **`shopify-schema-client-injected`** fires on `/pages/*` and policy pages because apps load
  sitewide. It is `info`, so tolerable; scoping to product/collection/article would tidy it.
- **`[data-variant-id]`** in `variantSelectors` overcounts on swatch/thumbnail grids.
  `select[name="id"] option` and `input[name="id"]` alone are sufficient and safe.

---

## P8 — Explanation text that is not actionable

`internal/report/explanations.go`. The Fix field should tell a Shopify operator what to change.

- `structured-missing-required` — circular; does not even name the fields it already has in
  `Data.missing`.
- `structured-missing-merchant` — generic where it could be platform-specific: on Shopify, GTIN is
  the variant **Barcode** field in admin; shipping and returns come from Markets/Shipping settings
  and the free Google & YouTube channel.
- `shopify-flat-variant-product` — names no file. The JSON-LD is usually in
  `sections/main-product.liquid`, and the culprit is Shopify's own `| structured_data` filter.
- `structured-none` and the four `*-candidate` Fixes restate the finding.

The standard to match is `shopify-schema-app-conflict`: it names the two owners, says pick one,
and says where the setting lives.

---

## P9 — Smaller items

- ✅ **CLI help text is stale.** FIXED in `fdf8023`. Also found `crawlrequest.go` — the schema MCP clients and the web API read — had been omitting the WordPress probes entirely, not just the Shopify one.
- **(remaining items below are open)**
- **Original note:** `cmd/gocrawl/crawl.go`'s `--specialized` flag still reads "AEO
  answer-lead, GEO quotable-density, WordPress security probes" and does not mention the Shopify
  `/products.json` probe. Same omission in `interactive.go` and `crawlrequest.go`. A user running
  `--help` cannot discover it.
- **`sameURL` detects only self-canonical-or-empty.** A faceted collection canonicalised to a
  genuinely *different, wrong* collection is not flagged. Inherited from the plan's pseudocode.
- **App-attribution allowlist is nine entries.** An unlisted SEO app attributes to `"theme"`, so a
  real theme-vs-app conflict goes unreported. A generic fallback keyed on the
  `/extensions/<uuid>/<handle>/` URL shape was proposed and **rejected**: bundled apps (Vitals)
  register multiple extensions and the fallback would manufacture a conflict between an app and
  itself. To close it safely, key on the extension UUID so one vendor's extensions collapse to one
  source, or require two unseen prefixes before flagging — either needs a fixture proving a
  same-app multi-extension page stays silent.
- **`structured-unresolved-id` uses page-scoped `@id` resolution.** A store declaring
  `Organization` once on its homepage and referencing it elsewhere would warn per page. Live
  evidence: 0 fires on real product and collection pages, so it ships. If it ever storms, the fix
  is a site-wide `@id` index built in `Analyze` before the per-page pass.
- **Crawling large Shopify stores needs `--include`.** A breadth-first crawl with a small
  `--max-pages` never descends to `/products/` — a homepage fans out to thousands of depth-1 URLs.
  Two runs totalling 55 pages never reached a product page. Use `--include '/products/'`.
- **Some stores 429 gocrawl while `curl` succeeds**, including with a browser User-Agent set.
  Pre-existing, outside this work, but it affects real Shopify audits.
- `docs/architecture.md` and `docs/roadmap.md` are stale (roadmap says "Twenty-one analyzers"; the
  architecture package map omits `shopify`, `botwall` and `consent`). Pre-existing.

---

## P2b — Structural findings from the whole-branch review

### ✅ F10. The two site-wide rollups collide in the report-diff key

**FIXED** in `7aff116`. Issues may carry `data.instance` (`analyze.InstanceKey`), which joins the
diff identity; both rollups set it. Pairing is now count-based too, which also fixes the
pre-existing case of several `link-broken` findings on one page collapsing. Existing codes were
not given an instance, so saved reports do not re-key.

`internal/diff/diff.go:74` keys a finding on `[Analyzer, Code, URL]`. But `rollup.issues`
(`rollup.go:65-84`) emits one issue **per type**, all at the same base URL with the same code, and
`templateGapIssues` (`template.go:210-224`) emits one **per template+label**, likewise.

So a crawl producing `structured-missing-recommended` for `Product`, `Organization` and
`VideoObject` collapses to a **single entry** in `gocrawl diff` — `baseByKey[...] = is` keeps
whichever came last, which (until F0 is fixed) is a coin flip.

This is a real hole in the diff feature, created by this branch's introduction of site-scoped
multi-instance findings. Minimal fix: fold the discriminator into the code, or less invasively
into the issue URL (`base + "#Product"`). If the contract should not change now, it at least needs
a note in `docs/output.md`.

### ✅ F11. Four guards have no test at all

**FIXED** in the test commit after `7aff116`; each new test fails with its guard removed.

Each was found by mutation and each survived the full suite. None is a defect today; all are the
kind of guard a future refactor deletes as dead code.

- `integrity.go:72` — `exemptFromEligibility` inside `topLevelOfType`. It is what stops a
  collection page whose theme and app each emit an `ItemList` of tiles from reporting
  `structured-duplicate-type: Product`. The shared helper's semantics are right for both
  consumers, but only one consumer is covered by tests.
- `integrity.go:168` — `if len(t) != 1` in `referencedIDs`. This is the entire
  bare-stub-vs-declaration distinction; without it, `{"@id": "...", "name": "Y"}` becomes a
  dangling-reference false positive.
- `schema.go:132` — the `tmpl != TemplateUtility && tmpl != TemplateUnknown` gate on
  `shopify-schema-client-injected`.
- `schema.go:120` — `sort.Strings(attributed)`, a determinism sort.

### ✅ F12. A relative canonical false-positives `shopify-duplicate-product-path`

**FIXED**: `canonicalOf` resolves against the page URL and searches only `<head>`. Both directions tested.

`internal/analyze/shopify/seo.go:79` compares `canonicalOf(p.Doc)` against an absolute `want`
without resolving it against `p.FinalURL`. A theme emitting
`<link rel="canonical" href="/products/tee">` on `/collections/all/products/tee` is flagged
wrongly. Stock Shopify emits absolute canonicals, so this is bounded to custom themes and apps.

Fix: resolve in `canonicalOf` via `url.Parse(p.FinalURL).ResolveReference(...)`. Note the same
omission flips `shopify-indexable-facet` (`seo.go:64`) the *other* way, into silence — worth making
both consistent. Also `canonicalOf:37` searches the whole document, where the `seo` analyzer scopes
to `head link[rel="canonical"]`.

### ✅ F13. The package doc claims a caching property the code does not have

**FIXED** by correcting the comment. The triple parse itself remains; caching was not done.

`internal/analyze/schemaorg/schemaorg.go:3-4` says "JSON-LD is parsed once per page." It is parsed
**three** times — `templateGapIssues` (`template.go:183`), `shopify.analyzePage`
(`shopify.go:115`) and `structured.analyzePage` (`structured.go:38`). Either correct the comment
or hang a per-page graph off `crawler.Page`. On a 5k-page crawl this is the branch's main new cost.

---

## Confirmed sound — do not re-open

Both reviews independently verified these; they are settled.

- **Block-index alignment** between `schemaorg.Parse` and `shopify.blockSources` cannot drift.
  `Parse` skips empty and malformed blocks from the *walk* but not from the `Each` index, and
  `blockSources` appends for every `ld+json` script including those. Both sides match the `type`
  attribute exactly, case-sensitively, untrimmed.
- **`pathSegments` / `localeOffset` have exactly two callers** (`Classify`,
  `canonicalProductURL`); both handle the locale prefix correctly and no third caller exists.
- **The purity exception is airtight.** The probe is unreachable before `detect()`
  (`shopify.go:53-54` returns nil first), guarded by `!a.probe || a.fetcher == nil` (`:82`), makes
  exactly one `Fetch` (`:86`), and every failure path — transport error, nil page, non-200, bad
  JSON, empty list — returns nil rather than a finding.
- **The public contract is intact.** All 18 literal codes plus the 2 variable-built rollup codes
  have explanations and docs rows; no pre-existing code was renamed; `structured-missing-required`
  gained `path` additively.
- **`shopify-template-schema-gap` can no longer mislead.** The message is a literal count of
  affected pages with no totality quantifier, `pageCountVerb` handles the singular, and `Data`
  carries `pages` + `examples`.
- **The app-attribution allowlist rejection was right.** A bundled vendor's multi-extension page
  manufacturing a conflict with itself is unbounded wrong output; a missing finding is bounded
  silence. The recorded exit condition — key on the extension UUID, not the handle, so one
  vendor's N extensions collapse to one source — is the correct one.
- **Page-scoped `@id` resolution ships.** 0 live fires on real product and collection pages, it is
  `warning` not `error`, and a site-wide index is a real design cost for a theoretical risk.

---

## What is good and should not be changed

Recorded because it is as important to know what not to touch as what to fix.

- **The strong/weak fingerprint tiering** (`shopify.go:131-160`) — evidence-driven from five real
  storefronts, names its own failure mode (Hydrogen), and its comment states the governing
  principle rather than describing the code.
- **`localeOffset` as an offset, not a strip**, with `canonicalProductURL` re-prepending the
  prefix. A locale-stripping canonical would have pointed stores at 404s.
- **`disambiguateSeparators`' five-rule table and its ambiguity-returns-false discipline**, and
  `distinctPagePrices` abandoning the whole page rather than silently dropping a price. These make
  the check fire *less* often, never more. Do not let anyone "optimise" this away — the code says
  so, and it is right.
- **`TemplatePolicy` split from `TemplateUtility`**, with `TestPolicyPageNeverFlaggedAsUtility` as
  a standing guardrail. The guard was kept, not just the fix.
- **`variantCount`'s max-not-sum** with a discriminating single-variant/two-selector fixture.
- **`.itemListElement` exemption** from eligibility checking — collection tiles are deliberately
  thin.
- **The comment culture throughout** — `facetParam:91-98` explaining why the loop nesting is
  load-bearing, `distinctPagePrices:308-315` explaining the accidental safety margin. Described as
  the best in the repo and the standard the rest should be held to.
