# Ranking structured-data opportunities

Date: 2026-10-09
Status: approved, not yet implemented

## Problem

The `structured` analyzer already finds the pages that are missing structured data: the
`structured-*-candidate` codes flag content shaped like a product, article, video, or
breadcrumb trail with no matching JSON-LD, and `structured-missing-required` flags markup
that is present but not eligible for its rich result.

What it does not do is say which of those to fix first. The candidate and missing-required
findings are per page and unordered, so a 500-page store produces several hundred warnings
of equal weight. The reader has to work out for themselves that 312 of them are one product
template, that the template matters more than the three blog posts with a video embed, and
which URLs are worth opening to check.

## Goals

1. Rank the missing-structured-data findings so the report answers "what do I fix first".
2. Rank fixes, not URLs. A schema gap usually lives in a template, so 312 product pages
   missing `Product` are one opportunity with 312 pages behind it, not 312 opportunities.
3. Name the highest-value example pages within each opportunity.
4. Keep the score explainable: every number that feeds it is in the finding's `Data`.
5. Leave a clear slot for real demand data (Search Console) to replace the importance proxy
   later.

## Non-goals

- Traffic or click-uplift estimates. A crawl sees structure, not demand. The score is a
  priority order, and nothing in the output presents it as anything else.
- Ranking recommended-tier and merchant-tier gaps. They are already rolled up once per type
  (`structured-missing-recommended`, `structured-missing-merchant`), and nearly every
  `Product` lacks `aggregateRating`, so including them would bury the "no rich result at
  all" gaps this feature exists to surface.
- New detection. This ranks what `structured` already detects. Page types it has no
  candidate heuristic for (a homepage with no `Organization`, a collection page with no
  `ItemList`) are not added here; on Shopify stores `shopify-template-schema-gap` keeps
  covering them.
- Generating the missing markup. That is the follow-up this ranking is meant to feed.
- Search Console, sitemap membership, or any other input that needs a fetch. `structured`
  stays pure and fetcher-free.

## Design

### What counts as an opportunity

An opportunity is a group of pages that share one fix. The group key is:

| Part | Values | Source |
| --- | --- | --- |
| `section` | `/` for the homepage, `/*` for single-segment paths, `/<first-segment>/*` otherwise | `Page.FinalURL` path |
| `type` | schema.org type | the candidate check or the node's `@type` |
| `gap` | `absent` or `ineligible` | see below |

- **`absent`**: a candidate check fired. The page reads like the type but carries no markup
  for it. Sources: the product, article, video, and breadcrumb candidate checks. The article
  family (`Article`, `NewsArticle`, `BlogPosting`, ...) is recorded as `Article`.
- **`ineligible`**: a node of the type exists but is missing a required field, which is
  exactly the condition that raises `structured-missing-required` today.

Both gaps mean the page earns no rich result for that type right now, so they are not
weighted against each other. `gap` is reported so the reader knows whether the fix is "add
a block" or "add a field".

`section` is the stand-in for "template". It is deliberately generic rather than borrowing
`shopify.Classify`: `structured` is platform-neutral and must work on any site. Collapsing
single-segment paths into `/*` matters, since otherwise a flat-URL site (`/my-post-slug`)
would produce one group per page. On a Shopify store the sections coincide with templates
(`/products/*`, `/collections/*`, `/blogs/*`, `/pages/*`).

### Which pages count

A page contributes to an opportunity only if schema on it could earn something:

- HTML with status 200 (already the precondition for every `structured` check).
- Indexable: no `noindex` in meta robots or `X-Robots-Tag`.
- Counted once per canonical URL, following the existing rollup convention
  (`analyze.CanonicalURL` compared through `analyze.URLKey`). A product reached at
  `/products/<h>` and `/collections/<c>/products/<h>` is one page, and its example URL is
  the canonical. Inbound links to every crawled duplicate are summed onto that one page.

Robots-blocked pages need no rule: they are not in the crawl.

### Score

```
importance(page) = 1 + log2(1 + inlinks)
score(group)     = type_weight × Σ importance(page)   over the group's counted pages
```

- **`inlinks`** is the number of internal links from other crawled pages, the same tally
  the `links` analyzer reports as `link-inbound`. The log keeps a homepage linked from every
  page from outweighing an entire template on its own, and the `1 +` keeps a page with no
  inbound links from counting as zero.
- **Summing over pages** is what makes a template outrank a one-off page: one fix, many
  pages. That is intended.
- **`type_weight`** is an editorial judgement of how visible the type's rich result is. It
  is not derived from data and the code comment must say so:

  | Weight | Types | Reason |
  | --- | --- | --- |
  | 10 | `Product`, `ProductGroup` | Price, stock, and rating in the result; shopping surfaces |
  | 8 | `Recipe`, `Event` | Large dedicated result formats |
  | 7 | `VideoObject` | Video thumbnail and video tab |
  | 6 | `Article`, `NewsArticle`, `BlogPosting`, `LocalBusiness` | Article enhancements; local pack |
  | 4 | `BreadcrumbList`, `Organization` | Small change to the result, but cheap and site-wide |
  | 2 | `FAQPage`, `ItemList`, `Person` | FAQ results are restricted to a few authoritative sites since 2023 |
  | 1 | `WebSite` | Sitelinks search box is deprecated |

  Every type in the `eligibility` table must have a weight; a test enforces it so a newly
  added type cannot silently score zero.

Depth is not a separate factor. It correlates strongly with inbound links and a second
importance knob would make the score harder to explain without changing the order much.

Groups are ordered by score descending, then by `section`, `type`, `gap` ascending, so the
output is deterministic.

### Output

One issue per opportunity, in rank order, capped at the top 20:

- `Analyzer`: `structured`
- `Code`: `structured-opportunity`
- `Severity`: `info`. The underlying findings are already warnings; raising these as
  warnings too would double-count them in the report's totals.
- `URL`: the site base (`analyze.SiteBase`), like the other site-wide rollups.
- `Message`: rank-free and readable without the code, for example
  `312 pages under /products/* have no Product structured data` or
  `48 pages under /blogs/* have Article markup missing required fields`.

`Data`:

| Key | Meaning |
| --- | --- |
| `rank` | 1-based position in the ranking |
| `score` | Group score, rounded to one decimal |
| `type` | schema.org type |
| `gap` | `absent` or `ineligible` |
| `section` | URL section the pages share |
| `pages` | Counted pages in the group |
| `inlinks` | Total inbound internal links across those pages |
| `type_weight` | The weight applied |
| `missing` | `ineligible` only: sorted union of the required fields missing across the group |
| `examples` | Up to 5 canonical URLs, highest importance first, ties broken by URL |
| `instance` | `analyze.InstanceKey`: `<section>|<type>|<gap>` |

`examples` is the direct answer to "which pages": the most-linked pages in the group, the
ones to open first and to verify the fix on.

The per-page candidate and missing-required findings are unchanged. The opportunity issues
sit on top of them as a prioritised summary.

The ranking is on by default. It is pure, adds no fetches, and emits at most 20 issues.

### Code layout

In `internal/analyze/structured`:

- **`opportunity.go`** (new): the `opportunities` accumulator, the type-weight table,
  `section()`, scoring, and rendering to issues. Created per `Analyze` call and never stored
  on the `Analyzer`, like `rollup`.
- **`structured.go`**: create the accumulator alongside the rollup, pass it into the
  per-page checks, and append its issues after the rollup's.
- **`candidates.go`, `eligibility.go`**: record into the accumulator at the points that
  already raise a candidate or a missing-required finding. No detection logic changes.

Two helpers move into `internal/analyze` so the ranking does not copy them:

- **`analyze.Inbound(result)`**: the per-page inbound tally currently private to the `links`
  analyzer as `buildInbound`. `links` switches to the shared helper.
- **`analyze.Indexable(p)`**: the meta-robots / `X-Robots-Tag` check currently private to
  `shopify/seo.go` as `indexable`. `shopify` switches to the shared helper.

Plumbing:

- `internal/report/explanations.go`: a `structured-opportunity` entry. It must state that
  the score is a priority order built from rich-result type, page count, and internal links,
  and is not a traffic estimate.
- `docs/analyzers.md`: a row in the `structured` table and a short paragraph on the score.
- `docs/roadmap.md`: note the ranking as shipped and Search Console weighting as the
  follow-up.

## Testing

- **Scoring**: a product template with many pages outranks a single article; equal scores
  fall back to the documented tie-break; the same pages in a different order produce the
  same ranking, scores, and examples.
- **Gating**: `noindex` pages (meta and header) are excluded; two URLs sharing a canonical
  count once and pool their inbound links.
- **Sectioning**: homepage, single-segment, and nested paths map to `/`, `/*`, and
  `/<seg>/*`; a flat-URL site yields one `/*` group per type, not one per page.
- **Gap sources**: each candidate check and a missing-required node land in the right
  `type` and `gap`; `missing` is the sorted union for `ineligible` and absent for `absent`.
- **Cap**: more than 20 groups emits exactly the top 20.
- **Weights**: every key of `eligibility` has a `type_weight`.
- **End to end**: an analyzer test over HTML fixtures under `testdata/` asserting the
  emitted `structured-opportunity` issues.
- Existing `links` and `shopify` tests keep passing after the helper moves.

## Risks

- **The importance proxy is weak.** Internal links measure how the site treats a page, not
  how many people search for it. The explanation text and docs must say so. When Search
  Console data lands, `importance(page)` is the one function to replace; the grouping,
  gating, and output shape stay.
- **Candidate false positives are amplified.** A wrong candidate heuristic now also shows
  up in a ranked list. The candidate checks were already tightened to be low-noise, and the
  ranking adds no heuristics of its own.
- **Client-injected markup.** In raw mode a page whose JSON-LD is injected by JavaScript
  looks `absent`. This is an existing limitation of the candidate checks
  (`shopify-schema-client-injected` covers the Shopify case); the explanation should point
  to `--render headless` to confirm.
- **Locale prefixes split groups.** `/fr/products/x` lands in `/fr/*`, not `/products/*`.
  Type and gap still separate the groups sensibly, so this is accepted for now rather than
  guessing which two-letter segments are locales on an arbitrary site.
- **The cap interacts with `gocrawl compare`.** A group that drops from rank 20 to 21
  between crawls disappears from the report and reads as resolved. The cap is set high
  enough that this should be rare on real sites.
- **Type weights are opinion.** They will need revisiting as Google changes which rich
  results it shows, the same maintenance the `eligibility` table already carries.

## Follow-ups

- Weight importance by Search Console impressions once the API integration exists.
- Feed the top-ranked opportunities into a structured-data generator (skeleton or patch
  per type).
- A homepage `Organization` / `WebSite` candidate check, which would then rank here for
  free.
