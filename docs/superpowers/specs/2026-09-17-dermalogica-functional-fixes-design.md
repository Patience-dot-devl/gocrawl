# Fixes from the dermalogica.nl functional test

Date: 2026-09-17
Status: implemented (`d088a8f..c9dda14`), accepted against a live re-crawl on 2026-09-17
Branch: `shopify-structured-data-analyzer` (PR #62)
Parent spec: [`2026-09-14-structured-data-shopify-design.md`](2026-09-14-structured-data-shopify-design.md)

## Context

The branch was run against a live Shopify store, dermalogica.nl: three crawls, up to 60 pages
each, 1–2 req/s. Every finding was checked by hand against the raw HTML fetched with curl.
Most findings held up. Five did not, and this spec fixes them.

What the store's markup looks like, so fixtures can mirror it:

- Every page has `Organization`. Its `sameAs` array holds 3 real profile URLs and **6 empty
  strings**, left by social-link theme settings that were never filled in.
- Product pages have one `Product` with `name`, `url`, `image`, `description`, `sku`, `brand`
  and `aggregateRating`. Its `offers` is an **array of 3–4 `Offer`s**, one per size. Each
  offer has its own `sku`, a numeric **`gtin12`**, `price`, `priceCurrency`, `availability`,
  and a `url` ending `?variant=<id>`. The `Product` itself has **no** `gtin*`.
- Each product page renders **two** `form[action="/cart/add"]` (main plus sticky), each with a
  hidden `<input type="hidden" name="id" value="<same variant id>">`. There is one
  `[data-variant-id]` and no `select[name="id"]`. The size picker uses markup none of the
  `variantSelectors` match.
- Collection and product pages render visible breadcrumbs:
  `<nav class="breadcrumbs" aria-label="breadcrumbs"><ol><li><a href="/">Home</a></li><li><a …>`.
  None of them has `BreadcrumbList` markup.
- Every product is also reachable at `/collections/<c>/products/<handle>`. That copy has a
  correct absolute canonical pointing at `/products/<handle>`.

## Conventions every fix follows

- **Fixtures are hand-written**, shaped like the markup above. Do not commit the store's HTML.
- **Tests must bite.** Each new test has to fail when the behaviour it covers is removed.
  Mutate the fix, watch the test fail, restore, and report the mutation you used.
- **Determinism.** Never let Go map iteration order reach output: sort keys or walk a slice.
  Reports get diffed between crawls.
- **Contract.** A new issue code needs, in the same commit, an entry in
  `internal/report/explanations.go` and a row in `docs/analyzers.md`. A code built from a
  variable (a rollup code) also goes in `TestDynamicallyCodedIssuesHaveExplanations`. A new
  site-scoped rollup sets `data.instance` (`analyze.InstanceKey`), as the existing rollups do.
- **Verification before commit:** `gofmt -l .` prints nothing, `go vet ./...` passes,
  `go test -race ./...` passes, and `golangci-lint run` reports 0 issues.
- **One commit per fix**, conventional-commit subject, ending with the attribution trailer.
  Do not push.
- When a fix lands, record it in
  `docs/superpowers/findings/2026-09-14-structured-data-shopify-followups.md` under a new
  "P10 — Functional test on dermalogica.nl" section: one bullet per fix with its commit.

## Fix 1 — Site-wide counts treat duplicate URLs of one product as separate pages

**Observed.** The `Product` merchant and recommended rollups and the product
`shopify-template-schema-gap` reported `pages: 16`. The crawl reached about 8 products, each
counted twice: once at `/products/<h>` and once at `/collections/<c>/products/<h>`. The
nested copy is canonicalised to the first, so it is the same page.

**Change.**
- Add `analyze.CanonicalURL(p *crawler.Page) string` in `internal/analyze/analyze.go`. It
  returns the page's `head link[rel="canonical"]` href resolved against `p.FinalURL`, or
  `p.FinalURL` when there is no canonical or it does not parse. Move the resolution logic
  out of `shopify.canonicalOf` (`internal/analyze/shopify/seo.go`); `canonicalOf` keeps its
  "" meaning "no canonical declared", because `seoIssues` relies on it. Build it on the
  shared helper, or share an unexported core, but keep one resolution implementation.
- `structured`'s rollup (`internal/analyze/structured/rollup.go`) and shopify's
  `templateGapIssues` (`internal/analyze/shopify/template.go`) count each page once per
  **canonical URL**. A second page with the same canonical adds nothing: not to `pages`, not
  to `missing`, not to `examples`. Examples hold the canonical URL. Compare canonicals after
  stripping the fragment and trailing slash, as shopify's `sameURL` already does.
- Per-page findings do not change. This fix only affects the counts in aggregated findings.

**Tests.** Two pages, `/products/tee` and `/collections/all/products/tee`, where the nested
one has a canonical to `/products/tee`: both rollups report `pages: 1` and the example is
`/products/tee`. A third page `/products/cap` with no canonical makes it `pages: 2`. Add a
relative-canonical variant of the duplicate as well.

## Fix 2 — `shopify-flat-variant-product` counts one variant twice and misses the real ones

**Observed.** Every product reported `variants: 2`. Both counts came from two hidden
`input[name="id"]` elements holding the **same** variant id (main form and sticky form). The
products really have 3–4 sizes, which the DOM selectors never saw, but the JSON-LD lists
them as 3–4 `Offer`s with distinct `?variant=` URLs. So a single-variant product with a
sticky add-to-cart would be flagged wrongly, and the count reported is wrong.

**Change** (`internal/analyze/shopify/schema.go`).
- `variantCount` counts **distinct non-empty values**, not elements. Use the `value`
  attribute for `input`/`option` selectors and the attribute value for
  `[data-variant-id]`. Keep the maximum across selectors, and keep the placeholder-option
  filter.
- Add a second source: distinct `variant` query-parameter values in the `url` of every
  `Offer` reachable from a page-level `Product`, meaning not under a list property. Parse
  with `net/url`. The page's variant count is the larger of the DOM count and the offer
  count.
- Add `data.source` with value `"dom"`, `"offers"` or `"dom+offers"` (both sources agree on
  the maximum), so a reader can see where the number came from. `shopify-single-offer-range`
  reuses `variants` and gets the same improvement for free.
- Update `variantCount`'s doc comment. The caveat that a repeated `input[name="id"]` still
  overcounts is no longer true.
- Leave `[data-variant-id]` in `variantSelectors`. Counting distinct values removes the
  swatch-grid overcount the findings doc raised (P7), so note that item as resolved.

**Tests.**
- Two forms with the same hidden id value, no offers: no finding, because only one variant
  exists.
- Two forms with the same id, plus a `Product` whose `offers` array has 3 offers with
  distinct `?variant=` URLs: finding with `variants: 3`, `source: "offers"`.
- A `select[name="id"]` with 3 distinct options: `variants: 3`, `source: "dom"`.
- The existing max-not-sum and placeholder tests keep passing.

## Fix 3 — "No GTIN" reported when every offer carries one

**Observed.** `structured-missing-merchant` listed `gtin|gtin8|gtin12|gtin13|gtin14|mpn` as
missing on all product pages. Every `Offer` carries `gtin12`. Google's merchant-listing
documentation places GTINs on `Product` and does not document reading them from `Offer`, so
the gap is real by that standard. But telling the store "no GTIN" sends them hunting for data
they already have. The real fix for them is to move the GTINs, and the variant modelling
(Fix 2's finding) is how.

**Change** (`internal/analyze/structured/eligibility.go`, `rollup.go`).
- When the merchant identifier group is unsatisfied on the node (and, for `ProductGroup`,
  through its variants), but an identifier is present on the node's offers, do two things.
  Offers here means `offers.gtin`, `offers.gtin8` … `offers.gtin14`, or `offers.mpn`; for
  `ProductGroup`, also `hasVariant.offers.<same>`.
  - Drop the identifier group from that page's `structured-missing-merchant` fields.
  - Add the page to a new site rollup, **`structured-identifier-on-offer`**. It is a
    warning, instance is the type, and the message reads "`<Type>` markup declares
    GTIN/MPN on Offer, where Google's merchant listings do not document reading it".
    `missing` carries which identifier properties were found on offers, for example
    `offers.gtin12`.
- Explanation Fix text, Shopify-specific: GTINs belong on the `Product`, one per variant.
  That means modelling variants as `ProductGroup` + `hasVariant` with each variant's
  `gtin`, taken from the variant **Barcode** field in Shopify admin (`variant.barcode` in
  Liquid).
- Satisfied numeric values (the store emits `gtin12` as a JSON number) must count as
  present. Verify `HasValue` already handles that, and add a test either way.

**Tests.**
- Dermalogica shape (no root gtin, 3 offers each with numeric `gtin12`): merchant rollup does
  not contain the identifier group, and `structured-identifier-on-offer` fires once with
  `pages: 1` and `offers.gtin12` in `missing`.
- `Product` with root `gtin13`: neither code mentions identifiers.
- `Product` with no identifier anywhere: merchant rollup still lists the group and the new
  code does not fire.
- `ProductGroup` whose variants carry `offers.gtin13` only: the new code fires.

## Fix 4 — `structured-breadcrumb-candidate` repeats one theme fact on every page

**Observed.** The code fired on 46 of 47 pages. That is true on each page, but it is one
theme decision, and `shopify-template-schema-gap` already reports the same missing
`BreadcrumbList` once per template. The per-page repetition is the noise this branch's own
policy says to aggregate: required gaps per page, opportunities site-wide.

**Change** (`internal/analyze/structured/candidates.go`, `structured.go`, `rollup.go`).
- `structured-breadcrumb-candidate` becomes a **site-scoped** finding built through the
  rollup accumulator. It is still a warning, and there is one issue per crawl with
  instance `BreadcrumbList`. `data` has `pages`, `examples`, `missing` (use `{"BreadcrumbList": pages}`
  so the rollup shape holds), and `links`, the largest breadcrumb link count seen.
  Message: "Pages render breadcrumb navigation but carry no BreadcrumbList structured data".
  It inherits Fix 1's canonical dedupe.
- This code exists on `main`, so moving it from page to site scope re-keys it once in
  `gocrawl compare` against saved reports. Say so in `docs/analyzers.md` (Scope column
  becomes **site**) and in the commit body.
- Only the breadcrumb candidate changes. `product`, `article` and `video` candidates stay per
  page: they depend on page content, not template chrome.

**Tests.** Three pages with breadcrumb nav and no `BreadcrumbList`: exactly one issue, URL is
the site base, `pages: 3`, severity warning. A page with `BreadcrumbList`: it is not counted.
Existing breadcrumb candidate tests are updated to the new shape, not deleted.

## Fix 5 — Empty strings in URL properties go unreported

**Observed.** `Organization.sameAs` on every page holds 6 empty strings. `Graph.Strs` skips
empty values, so the `urlProperties` loop in `integrity.go` never sees them. Google's Rich
Results Test flags these as invalid URLs. They come from theme settings that were never
filled in, so they repeat on every page.

**Change** (`internal/analyze/structured/integrity.go`, `rollup.go`).
- For each node and each entry in `urlProperties`, look at the raw values (`g.Values`, not
  `g.Strs`). A string value that is empty or whitespace-only is an empty URL. Only strings
  count: a missing property is not a finding.
- Report through a new site rollup, **`structured-empty-url`**. It is `info`, because
  no rich-result eligibility impact is documented and the Test only warns. Instance is the
  type, `missing` maps property to page count, and the message reads "`<Type>` markup
  has empty URL values". Add an `empty` data field with the largest number of empty entries
  seen for any property on one page (6 for dermalogica), so the reader sees scale.
- Explanation Fix text: in Shopify, these come from the theme's social-link settings
  (Online Store → Themes → Customize → Theme settings → Social media). Fill them in, or fix
  the snippet to skip blanks (`{% if settings.social_x_link != blank %}`).

**Tests.** Dermalogica shape: `sameAs` with 3 URLs and 6 `""`, on 2 pages → one issue,
`pages: 2`, `empty: 6`, and `missing` includes `sameAs`. `url: "  "` is counted. A node with no
`sameAs` does not fire. A non-empty relative URL still raises only `structured-relative-url`.

## Order

The fixes run one at a time, in this order, each on top of the previous commit:
**Fix 1** (the other rollups build on its dedupe) → **Fix 2** → **Fix 3** → **Fix 4** →
**Fix 5**.

## Out of scope

- `botwall-captcha-widget` firing on every page because of a newsletter reCAPTCHA. The
  analyzer is pre-existing and unrelated to this branch; it goes in the findings doc.
- Cloudflare returning 429 at 2 req/s. This is also pre-existing (findings doc P9).

## Acceptance

After all five commits, re-crawl dermalogica.nl with the same product-focused crawl
(`--include '/products/|/collections/|^https://www.dermalogica.nl/$' --max-pages 50 --rate 1
--specialized`). Expect:

- Product rollups report the deduplicated product count.
- `shopify-flat-variant-product` shows `variants` 3–4 with `source` containing `offers`.
- `structured-missing-merchant` does not list identifiers.
- `structured-identifier-on-offer` fires once.
- `structured-breadcrumb-candidate` is a single site issue.
- `structured-empty-url` fires once for `Organization` with `empty: 6`.
