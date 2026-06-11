package report

// Explanation describes a single issue code in plain language: what the finding
// means, why it matters, and what to do about it. It is keyed by an Issue's Code
// and surfaced in the HTML report so a reader can act on a finding without
// consulting external documentation.
type Explanation struct {
	// What the issue entails — a plain-language description of the finding.
	What string
	// Impact — why it matters / the potential consequence if left unaddressed.
	Impact string
	// Fix — the recommended remediation.
	Fix string
}

// explain returns the Explanation for an issue code, or nil when no explanation
// is registered. Returning a pointer lets the HTML template skip the block with
// {{with explain .Code}} when a code is unknown. It is exposed to the template as
// the "explain" function.
func explain(code string) *Explanation {
	if e, ok := explanations[code]; ok {
		return &e
	}
	return nil
}

// explanations is the single source of truth mapping every analyzer issue code
// to a short, actionable explanation. Keep this in sync with the codes emitted by
// the analyzers under internal/analyze/*.
var explanations = map[string]Explanation{
	// --- aeo: Answer Engine Optimization ---
	"aeo-answer-schema": {
		What:   "The page exposes answer-engine structured data (FAQPage, QAPage, or HowTo).",
		Impact: "Positive signal. This markup helps answer engines and AI assistants extract and cite direct answers from the page.",
		Fix:    "No action needed. Keep the structured data accurate and in sync with the visible content.",
	},
	"aeo-answer-too-long": {
		What:   "An answer that follows a question-style heading is too long to be lifted as a featured snippet.",
		Impact: "Search engines and answer engines may skip the answer for snippets/voice results, losing position-zero visibility.",
		Fix:    "Lead with a concise 40–60 word direct answer immediately under the question, then expand with detail below it.",
	},
	"aeo-faq-candidate": {
		What:   "The page has question-style headings but no FAQPage/QAPage structured data.",
		Impact: "You miss eligibility for FAQ rich results and make it harder for AI engines to recognise and cite the Q&A pairs.",
		Fix:    "Add FAQPage (or QAPage) JSON-LD whose questions and answers mirror the on-page headings and text.",
	},
	"aeo-no-answer-lead": {
		What:   "A question-titled page does not open with a concise, direct answer.",
		Impact: "Answer engines prefer pages that answer up front; burying the answer reduces the chance of being quoted.",
		Fix:    "Open the body with a 1–2 sentence direct answer to the page's question before any preamble.",
	},
	"aeo-no-list-format": {
		What:   "Long-form content contains no lists or tables that snippet extractors can pull from.",
		Impact: "List and table snippets are common in SERPs and AI answers; without them the page is harder to summarise.",
		Fix:    "Break suitable content into ordered/unordered lists or tables (steps, comparisons, specs, pros/cons).",
	},

	// --- amp: Accelerated Mobile Pages ---
	"amp-detected": {
		What:   "The page is an AMP document.",
		Impact: "Informational. AMP constrains HTML/JS but can speed mobile delivery.",
		Fix:    "No action needed. Ensure the AMP page has a valid canonical and loads the AMP runtime.",
	},
	"amp-missing-canonical": {
		What:   "An AMP page has no canonical link to its non-AMP counterpart.",
		Impact: "Search engines may treat the AMP and canonical versions as duplicates and index the wrong one.",
		Fix:    "Add <link rel=\"canonical\" href=\"…\"> pointing to the canonical (usually non-AMP) URL.",
	},
	"amp-missing-runtime": {
		What:   "An AMP document does not load the AMP runtime (v0.js).",
		Impact: "Without the runtime the page is invalid AMP and will not be served from AMP caches or eligible for AMP features.",
		Fix:    "Include <script async src=\"https://cdn.ampproject.org/v0.js\"></script> in the <head>.",
	},
	"amp-amphtml-linked": {
		What:   "A standard page links to an AMP version via rel=amphtml.",
		Impact: "Informational. Confirms the AMP pairing is declared.",
		Fix:    "No action needed. Verify the linked AMP page resolves with HTTP 200.",
	},
	"amp-broken-amphtml": {
		What:   "The rel=amphtml link points to a broken or redirecting target.",
		Impact: "Search engines cannot reach the AMP version, so AMP eligibility is lost.",
		Fix:    "Update the amphtml link to a working AMP URL that returns HTTP 200 without redirecting.",
	},

	// --- content: thin content detection ---
	"thin-content": {
		What:   "The page has very little textual content.",
		Impact: "Thin pages rank poorly and can dilute overall site quality in search engines' eyes.",
		Fix:    "Add substantive, useful content, consolidate with a richer page, or noindex if the page has no standalone value.",
	},
	"low-content": {
		What:   "The page's word count is well below the site average.",
		Impact: "May indicate an under-developed page that underperforms relative to its peers.",
		Fix:    "Review whether the page needs more depth, or confirm the short length is intentional (e.g. a contact page).",
	},

	// --- duplicates: cross-page duplicate detection ---
	"duplicate-content": {
		What:   "The page body is identical to one or more other crawled pages.",
		Impact: "Duplicate content splits ranking signals and can cause search engines to index the wrong URL.",
		Fix:    "Consolidate duplicates, set a canonical to the preferred URL, or differentiate the content.",
	},
	"duplicate-title": {
		What:   "The <title> is identical to other pages.",
		Impact: "Duplicate titles confuse users in SERPs and weaken each page's topical distinctiveness.",
		Fix:    "Write a unique, descriptive title for each page that reflects its specific content.",
	},
	"duplicate-meta-description": {
		What:   "The meta description is identical to other pages.",
		Impact: "Duplicate descriptions reduce SERP click-through and are often rewritten by search engines.",
		Fix:    "Write a unique meta description per page summarising that page's content.",
	},

	// --- geo: Generative Engine Optimization ---
	"geo-ai-crawler-blocked": {
		What:   "robots.txt disallows one or more AI crawlers at the site root.",
		Impact: "Blocked AI engines (e.g. GPTBot, ClaudeBot) cannot read the site, so it won't be cited in AI answers. This may be intentional.",
		Fix:    "If AI visibility is desired, allow the relevant user-agents; if not, no action is needed.",
	},
	"geo-llms-txt": {
		What:   "The site publishes an /llms.txt content map.",
		Impact: "Positive signal. /llms.txt helps LLMs locate and prioritise your key content.",
		Fix:    "No action needed. Keep the file current as important pages change.",
	},
	"geo-no-llms-txt": {
		What:   "No /llms.txt content map was found at the site root.",
		Impact: "Optional. Without it, AI engines have no curated guide to your most important pages.",
		Fix:    "Consider publishing /llms.txt listing key pages and summaries if AI discoverability matters to you.",
	},
	"geo-missing-author": {
		What:   "An article-like page has no author attribution.",
		Impact: "Author signals support E-E-A-T and help AI engines assess trustworthiness before citing content.",
		Fix:    "Add a visible byline and/or author markup (e.g. Article.author in JSON-LD).",
	},
	"geo-missing-date": {
		What:   "An article-like page has no published or modified date.",
		Impact: "Freshness signals matter for both search and AI answers; undated content may be deprioritised.",
		Fix:    "Add visible and structured published/modified dates (datePublished, dateModified).",
	},
	"geo-no-main-landmark": {
		What:   "A content-heavy page has no <main> or <article> landmark.",
		Impact: "Crawlers and AI extractors rely on landmarks to isolate primary content from navigation/boilerplate.",
		Fix:    "Wrap the principal content in a <main> or <article> element.",
	},
	"geo-js-dependent-content": {
		What:   "Most content appears only after JavaScript runs; the raw HTML is nearly empty.",
		Impact: "Non-executing AI crawlers and some search bots see little or no content, harming indexing and citation.",
		Fix:    "Server-render or pre-render the primary content so it exists in the initial HTML response.",
	},
	"geo-low-quotable-density": {
		What:   "A content-heavy page has few concrete, citable data points (numbers, stats, dates).",
		Impact: "AI engines preferentially quote specific, verifiable facts; vague prose is less likely to be cited.",
		Fix:    "Add concrete figures, statistics, dates, and named entities to make the content more quotable.",
	},

	// --- hreflang: multilingual annotations ---
	"hreflang-invalid-code": {
		What:   "An hreflang value is not a valid language (or language-region) tag.",
		Impact: "Search engines ignore invalid hreflang entries, so language/region targeting fails for that pairing.",
		Fix:    "Use valid ISO codes, e.g. \"en\", \"en-GB\", \"pt-BR\", or \"x-default\".",
	},
	"hreflang-missing-x-default": {
		What:   "An hreflang cluster has no x-default entry.",
		Impact: "Users whose language doesn't match any variant have no defined fallback page.",
		Fix:    "Add a rel=alternate hreflang=\"x-default\" pointing to the default/language-selector page.",
	},
	"hreflang-missing-self": {
		What:   "An hreflang cluster has no self-referencing entry for the current page.",
		Impact: "Valid hreflang requires each page to reference itself; omitting it can invalidate the whole cluster.",
		Fix:    "Include an hreflang entry for the page's own language/region URL.",
	},
	"hreflang-no-return-link": {
		What:   "An hreflang target does not link back to this page.",
		Impact: "hreflang annotations must be bidirectional; non-reciprocal links are ignored by search engines.",
		Fix:    "Ensure each referenced alternate page lists this page in its own hreflang set.",
	},

	// --- redirects / httpx: HTTP responses ---
	"fetch-error": {
		What:   "The page could not be fetched at all.",
		Impact: "Users and crawlers cannot reach the page; it cannot rank and damages site reliability.",
		Fix:    "Investigate DNS, TLS, timeouts, or server availability for the URL and ensure it responds.",
	},
	"server-error": {
		What:   "The URL returned a 5xx server error.",
		Impact: "The page is unavailable; persistent 5xx responses cause de-indexing and lost traffic.",
		Fix:    "Check server logs and fix the application/infrastructure fault causing the error.",
	},
	"client-error": {
		What:   "The URL returned a 4xx client error (e.g. 404, 403, 410).",
		Impact: "The page is unreachable; inbound links and crawl budget are wasted.",
		Fix:    "Restore the page, 301-redirect to a relevant URL, or fix the access/permission issue.",
	},
	"redirect-loop": {
		What:   "The URL redirects in a cycle that never resolves to a final page.",
		Impact: "Users and crawlers get trapped; the content is effectively unreachable.",
		Fix:    "Break the loop so the chain terminates at a single 200-status destination.",
	},
	"redirect-chain": {
		What:   "The URL passes through multiple redirects before reaching its final destination.",
		Impact: "Each hop adds latency and dilutes ranking signals; long chains risk being abandoned by crawlers.",
		Fix:    "Collapse the chain to a single redirect that points straight to the final URL.",
	},
	"redirect": {
		What:   "The URL redirects to another location.",
		Impact: "Informational. A single redirect is normal, but update internal links to point at the destination.",
		Fix:    "Where possible, link directly to the final URL to avoid unnecessary hops.",
	},
	"slow-response": {
		What:   "The server's response was slower than the configured threshold.",
		Impact: "Slow responses hurt user experience, crawl efficiency, and Core Web Vitals (TTFB).",
		Fix:    "Optimise backend processing, caching, or CDN delivery to lower response time.",
	},
	"mixed-content": {
		What:   "An HTTPS page loads insecure http:// resources.",
		Impact: "Browsers may block the resources or warn users, breaking functionality and eroding trust.",
		Fix:    "Update all subresource URLs (scripts, images, styles, iframes) to https://.",
	},

	// --- images ---
	"img-missing-alt": {
		What:   "One or more images have no alt attribute.",
		Impact: "Hurts accessibility for screen-reader users and removes image-search/context signals.",
		Fix:    "Add descriptive alt text to meaningful images; use empty alt=\"\" for purely decorative ones.",
	},
	"img-missing-dimensions": {
		What:   "One or more images lack explicit width/height attributes.",
		Impact: "Missing dimensions cause layout shift (CLS), degrading Core Web Vitals and user experience.",
		Fix:    "Set width and height (or CSS aspect-ratio) so the browser can reserve space before load.",
	},

	// --- landing: paid landing page checks ---
	"landing-not-https": {
		What:   "A paid landing page is not served over HTTPS.",
		Impact: "Ad platforms may disapprove the ad, and users see insecure warnings, lowering conversions.",
		Fix:    "Serve the landing page over HTTPS with a valid certificate.",
	},
	"landing-noindex": {
		What:   "A paid landing page is marked noindex.",
		Impact: "Some ad platforms penalise or reject noindex landing pages, and it blocks any organic value.",
		Fix:    "Remove the noindex directive unless intentionally keeping the page out of organic search.",
	},
	"landing-missing-title": {
		What:   "A landing page has no <title>.",
		Impact: "Weakens relevance signals and quality scoring, and produces poor browser/SERP labelling.",
		Fix:    "Add a clear, keyword-relevant <title> aligned to the campaign.",
	},
	"landing-missing-h1": {
		What:   "A landing page has no <h1>.",
		Impact: "A missing primary heading weakens message match and on-page clarity for users and bots.",
		Fix:    "Add a single descriptive <h1> stating the page's main offer/value.",
	},
	"landing-missing-description": {
		What:   "A landing page has no meta description.",
		Impact: "Minor for paid traffic, but reduces organic CTR if the page is also indexed.",
		Fix:    "Add a concise meta description summarising the offer.",
	},
	"landing-keyword-mismatch": {
		What:   "Campaign keywords are largely absent from the landing page title and headings.",
		Impact: "Poor message match lowers ad Quality Score/relevance and conversion rate.",
		Fix:    "Incorporate the campaign's core keywords naturally into the title and headings.",
	},
	"landing-keyword-weak": {
		What:   "Campaign keywords are only weakly reflected in the title/headings.",
		Impact: "Partial message match leaves Quality Score and relevance on the table.",
		Fix:    "Strengthen the use of campaign terms in prominent on-page elements.",
	},
	"landing-keyword-aligned": {
		What:   "Campaign keywords align well with the landing page title/headings.",
		Impact: "Positive signal for strong message match and ad relevance.",
		Fix:    "No action needed. Maintain alignment as campaigns evolve.",
	},

	// --- links: internal link analysis ---
	"broken-link": {
		What:   "An internal link points to a page that returns an error status.",
		Impact: "Creates dead ends for users and crawlers and wastes crawl budget and link equity.",
		Fix:    "Fix or remove the link, or repair the destination URL.",
	},
	"link-to-redirect": {
		What:   "An internal link points to a URL that redirects.",
		Impact: "Adds an unnecessary hop, slowing navigation and diluting link signals.",
		Fix:    "Update the link to point directly to the redirect's final destination.",
	},
	"empty-anchor": {
		What:   "One or more links have empty anchor text.",
		Impact: "Empty anchors give no context to users or search engines and hurt accessibility.",
		Fix:    "Add descriptive anchor text, or aria-label/alt where the link wraps an image/icon.",
	},
	"link-summary": {
		What:   "A count of total, external, and nofollow links on the page.",
		Impact: "Informational. Useful for understanding the page's link profile.",
		Fix:    "No action needed. Review if external/nofollow ratios look unexpected.",
	},
	"inbound-links": {
		What:   "The number of internal pages linking to this page, with sample anchors.",
		Impact: "Informational. Inbound internal links indicate a page's importance within the site.",
		Fix:    "No action needed. Boost internal links to important pages that have few.",
	},

	// --- pagination ---
	"pagination-detected": {
		What:   "The page declares rel=next/prev pagination links.",
		Impact: "Informational. Signals a paginated series to crawlers.",
		Fix:    "No action needed. Ensure the next/prev targets are valid and consistent.",
	},
	"pagination-broken": {
		What:   "A rel=next/prev pagination link points to a broken or redirecting target.",
		Impact: "Breaks crawl traversal of the paginated series and can strand later pages.",
		Fix:    "Point the pagination links at working, non-redirecting URLs.",
	},

	// --- perf: Core Web Vitals ---
	"cwv-render-failed": {
		What:   "Headless rendering failed, so Core Web Vitals were not collected for the page.",
		Impact: "Informational. No CWV data is available for this page in this run.",
		Fix:    "Re-run with rendering enabled and check that the headless browser can load the page.",
	},
	"cwv-measured": {
		What:   "Core Web Vitals were measured in lab mode (LCP, FCP, CLS, TBT, TTFB).",
		Impact: "Informational. These lab metrics approximate field performance.",
		Fix:    "No action needed. Review the individual metric findings for any that need improvement.",
	},
	"lcp-needs-improvement": {
		What:   "Largest Contentful Paint is above the 'good' threshold (2.5s).",
		Impact: "Slower perceived load; borderline LCP can reduce rankings and conversions.",
		Fix:    "Optimise the LCP element: faster server/TTFB, preloaded hero image/font, fewer render-blocking resources.",
	},
	"lcp-poor": {
		What:   "Largest Contentful Paint is poor (above ~4s).",
		Impact: "Users perceive the page as slow to load; a clear negative ranking and UX factor.",
		Fix:    "Significantly speed up the main content paint: optimise images, server response, and critical-path CSS/JS.",
	},
	"fcp-needs-improvement": {
		What:   "First Contentful Paint is above the 'good' threshold (1.8s).",
		Impact: "Users wait longer to see any content, hurting perceived speed.",
		Fix:    "Reduce render-blocking resources and improve TTFB so first paint happens sooner.",
	},
	"fcp-poor": {
		What:   "First Contentful Paint is poor (above ~3s).",
		Impact: "The page appears blank for too long, increasing bounce risk.",
		Fix:    "Cut render-blocking CSS/JS, inline critical CSS, and improve server response time.",
	},
	"cls-needs-improvement": {
		What:   "Cumulative Layout Shift is above the 'good' threshold (0.1).",
		Impact: "Visible layout jumps frustrate users and can cause misclicks.",
		Fix:    "Reserve space for images/ads/embeds with explicit dimensions and avoid inserting content above existing content.",
	},
	"cls-poor": {
		What:   "Cumulative Layout Shift is poor (above 0.25).",
		Impact: "Significant layout instability; a strong negative UX and ranking signal.",
		Fix:    "Set dimensions on all media, preload fonts to avoid FOUT/FOIT, and stabilise dynamic content insertion.",
	},
	"tbt-needs-improvement": {
		What:   "Total Blocking Time (a lab proxy for INP) is above the 'good' threshold (200ms).",
		Impact: "The main thread is busy enough to make the page feel sluggish to interact with.",
		Fix:    "Break up long JavaScript tasks, defer non-critical scripts, and reduce third-party JS.",
	},
	"tbt-poor": {
		What:   "Total Blocking Time (lab proxy for INP) is poor (above 600ms).",
		Impact: "Interactions are noticeably delayed; a strong negative responsiveness signal.",
		Fix:    "Aggressively reduce and split main-thread JavaScript; remove or lazy-load heavy third-party scripts.",
	},
	"ttfb-needs-improvement": {
		What:   "Time to First Byte is above the 'good' threshold (800ms).",
		Impact: "A slow server response delays everything downstream, including LCP and FCP.",
		Fix:    "Improve backend performance, add caching/CDN, and reduce redirects before the document loads.",
	},
	"ttfb-poor": {
		What:   "Time to First Byte is poor (above ~1.8s).",
		Impact: "The server is slow to respond, dragging down all other load metrics.",
		Fix:    "Investigate slow backend queries, enable caching/CDN, and right-size hosting.",
	},
	"cwv-not-collected": {
		What:   "Core Web Vitals were not collected because headless rendering was not enabled.",
		Impact: "Informational. No CWV data for this run.",
		Fix:    "Run with --render headless to collect Core Web Vitals.",
	},
	"response-time": {
		What:   "The measured server response time (a TTFB proxy) for the page.",
		Impact: "Informational. High values indicate slow server responses.",
		Fix:    "No action needed unless the value is high; then optimise backend/caching.",
	},

	// --- robots: robots.txt ---
	"no-robots": {
		What:   "No robots.txt file was found.",
		Impact: "Crawling defaults to fully allowed; you lose a place to declare sitemaps and crawl directives.",
		Fix:    "Add a robots.txt (even a permissive one) and declare your sitemap location.",
	},
	"no-sitemap-declared": {
		What:   "robots.txt does not declare a Sitemap directive.",
		Impact: "Search engines have to discover the sitemap by convention rather than being told where it is.",
		Fix:    "Add a 'Sitemap: https://…/sitemap.xml' line to robots.txt.",
	},
	"sitemaps-declared": {
		What:   "robots.txt declares one or more sitemaps.",
		Impact: "Positive signal. Helps crawlers find your sitemap(s).",
		Fix:    "No action needed. Ensure the declared sitemap URLs are valid.",
	},
	"crawled-disallowed": {
		What:   "A URL that robots.txt disallows was nonetheless crawled (per crawler configuration).",
		Impact: "Indicates a mismatch between intended crawl rules and actual crawling; compliant bots would skip it.",
		Fix:    "Confirm the disallow rule is intentional, and that production bots respect it.",
	},

	// --- security: headers & forms ---
	"missing-hsts": {
		What:   "An HTTPS response has no Strict-Transport-Security header.",
		Impact: "Leaves users exposed to protocol-downgrade/man-in-the-middle attacks on first or subsequent visits.",
		Fix:    "Send Strict-Transport-Security with an appropriate max-age (and includeSubDomains where applicable).",
	},
	"missing-csp": {
		What:   "The response has no Content-Security-Policy header.",
		Impact: "Without CSP the page is more exposed to XSS and content-injection attacks.",
		Fix:    "Define a Content-Security-Policy restricting allowed script/style/resource origins.",
	},
	"missing-x-content-type-options": {
		What:   "The response has no X-Content-Type-Options: nosniff header.",
		Impact: "Browsers may MIME-sniff responses, enabling some content-type confusion attacks.",
		Fix:    "Send 'X-Content-Type-Options: nosniff' on responses.",
	},
	"insecure-form": {
		What:   "A form submits over insecure http://.",
		Impact: "Submitted data can be intercepted; browsers warn users, harming trust and conversions.",
		Fix:    "Point the form action at an https:// endpoint.",
	},

	// --- seo: on-page technical SEO ---
	"missing-title": {
		What:   "The page has no <title> element.",
		Impact: "Title is a primary ranking and SERP-display signal; its absence severely hurts visibility.",
		Fix:    "Add a unique, descriptive <title> (roughly 50–60 characters).",
	},
	"short-title": {
		What:   "The <title> is very short.",
		Impact: "A too-short title likely under-describes the page and wastes SERP space.",
		Fix:    "Expand the title to clearly describe the page using relevant keywords.",
	},
	"long-title": {
		What:   "The <title> may be truncated in search results.",
		Impact: "Truncated titles lose meaning and can lower click-through.",
		Fix:    "Trim the title to roughly 50–60 characters, front-loading the important words.",
	},
	"missing-meta-description": {
		What:   "The page has no meta description.",
		Impact: "Search engines auto-generate snippet text, often less compelling, reducing CTR.",
		Fix:    "Add a unique meta description (~150–160 characters) summarising the page.",
	},
	"short-meta-description": {
		What:   "The meta description is short.",
		Impact: "Under-uses the available snippet space and may under-sell the page.",
		Fix:    "Expand toward ~150–160 characters with a compelling, accurate summary.",
	},
	"long-meta-description": {
		What:   "The meta description may be truncated.",
		Impact: "The tail of the description is cut off in SERPs, potentially losing the call to action.",
		Fix:    "Trim to roughly 150–160 characters, leading with the key message.",
	},
	"meta-noindex": {
		What:   "The page is marked noindex via the robots meta tag.",
		Impact: "The page is excluded from search indexes. This may be intentional, or an accidental loss of visibility.",
		Fix:    "Remove the noindex directive if the page should rank; otherwise no action is needed.",
	},
	"meta-nofollow": {
		What:   "The page is marked nofollow via the robots meta tag.",
		Impact: "Search engines won't follow links on the page, limiting crawl flow and link equity.",
		Fix:    "Remove the page-level nofollow unless intentionally sandboxing the page's links.",
	},
	"x-robots-noindex": {
		What:   "An X-Robots-Tag HTTP header marks the page noindex.",
		Impact: "The page is excluded from search indexes via headers — easy to overlook since it's not in the HTML.",
		Fix:    "Remove noindex from the X-Robots-Tag header if the page should be indexed.",
	},
	"x-robots-nofollow": {
		What:   "An X-Robots-Tag HTTP header marks the page nofollow.",
		Impact: "Links on the page won't be followed; this header-level directive is easy to miss.",
		Fix:    "Remove nofollow from the X-Robots-Tag header unless intentional.",
	},
	"meta-refresh": {
		What:   "The page uses a meta-refresh redirect.",
		Impact: "Meta refreshes are slower, hurt UX/accessibility, and pass signals less reliably than HTTP redirects.",
		Fix:    "Replace with a server-side HTTP 301/302 redirect.",
	},
	"multiple-canonical": {
		What:   "The page declares more than one canonical link.",
		Impact: "Conflicting canonicals confuse search engines, which may ignore them entirely.",
		Fix:    "Keep exactly one rel=canonical pointing to the preferred URL.",
	},
	"missing-canonical": {
		What:   "The page has no canonical link.",
		Impact: "Without a canonical, duplicate/parameterised variants can compete and split signals.",
		Fix:    "Add a self-referencing rel=canonical (or point to the preferred variant).",
	},
	"missing-h1": {
		What:   "The page has no <h1> element.",
		Impact: "The primary heading reinforces topic relevance for users and search engines.",
		Fix:    "Add a single, descriptive <h1> that states the page's main topic.",
	},
	"multiple-h1": {
		What:   "The page has multiple <h1> elements.",
		Impact: "Generally tolerated by modern search engines but can dilute heading clarity.",
		Fix:    "Prefer one <h1> per page and use <h2>–<h6> for the heading hierarchy.",
	},
	"missing-lang": {
		What:   "The <html> element has no lang attribute.",
		Impact: "Hurts accessibility (screen-reader pronunciation) and language targeting.",
		Fix:    "Set the document language, e.g. <html lang=\"en\">.",
	},
	"missing-viewport": {
		What:   "The page has no viewport meta tag.",
		Impact: "The page won't be mobile-friendly, hurting mobile UX and mobile-first ranking.",
		Fix:    "Add <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">.",
	},
	"missing-charset": {
		What:   "The page declares no character set.",
		Impact: "Browsers must guess the encoding, risking garbled text (mojibake).",
		Fix:    "Add <meta charset=\"utf-8\"> early in the <head>.",
	},
	"missing-opengraph": {
		What:   "The page has no OpenGraph tags.",
		Impact: "Shared links on social platforms get poor or no preview cards, reducing engagement.",
		Fix:    "Add og:title, og:description, og:image, and og:url (plus Twitter Card tags) to the <head>.",
	},

	// --- sitemap: sitemap.xml ---
	"invalid-sitemap": {
		What:   "A sitemap could not be parsed as a urlset or sitemap index.",
		Impact: "Search engines can't use a malformed sitemap, undermining discovery of your URLs.",
		Fix:    "Validate the sitemap XML structure against the sitemaps.org schema and fix errors.",
	},
	"no-sitemap": {
		What:   "No sitemap was found at the robots.txt declaration or conventional locations.",
		Impact: "Search engines must rely solely on link discovery, which can miss pages.",
		Fix:    "Publish a sitemap.xml and reference it from robots.txt.",
	},
	"sitemap-coverage": {
		What:   "A comparison of sitemap URLs against crawled pages (gaps in both directions).",
		Impact: "Informational. Reveals pages in the sitemap but not crawled, and crawled pages absent from the sitemap.",
		Fix:    "No action needed; review gaps to keep the sitemap aligned with the live site.",
	},

	// --- structured: JSON-LD ---
	"invalid-jsonld": {
		What:   "A JSON-LD block is not valid JSON.",
		Impact: "Malformed structured data is ignored, forfeiting rich-result eligibility.",
		Fix:    "Fix the JSON syntax so the block parses; validate with a structured-data testing tool.",
	},
	"structured-missing-required": {
		What:   "A structured-data object is missing required schema.org fields.",
		Impact: "Incomplete markup is ineligible for the corresponding rich results.",
		Fix:    "Add the required properties for the schema type (per schema.org / Google's documentation).",
	},
	"no-structured-data": {
		What:   "The page has no JSON-LD structured data.",
		Impact: "The page is ineligible for rich results and gives engines fewer explicit entity signals.",
		Fix:    "Add relevant JSON-LD (e.g. Article, Product, Organization, BreadcrumbList) where appropriate.",
	},
	"structured-data": {
		What:   "Valid JSON-LD structured data was found, with its declared types.",
		Impact: "Positive signal. Enables rich results and clearer entity understanding.",
		Fix:    "No action needed. Keep the markup accurate and aligned with visible content.",
	},

	// --- tracking: analytics & marketing tags ---
	"no-tracking-tags": {
		What:   "No analytics or marketing tags were detected in the static HTML.",
		Impact: "Either the page is untracked, or tags load via a tag manager and aren't visible in static HTML.",
		Fix:    "Confirm tracking is intentional; if expected, verify tags fire (e.g. via a tag manager or browser tools).",
	},
	"duplicate-tracking-tag": {
		What:   "The same tag is installed more than once on the page.",
		Impact: "Duplicate tags can double-count traffic/conversions and distort analytics.",
		Fix:    "Remove the redundant install so each tag loads exactly once.",
	},
	"tracking-tags": {
		What:   "Analytics/marketing tags were detected, with their IDs.",
		Impact: "Informational. Documents which tracking is present on the page.",
		Fix:    "No action needed. Verify the detected IDs are the intended ones.",
	},
	"mixed-ga-versions": {
		What:   "Both Universal Analytics and GA4 tags are present.",
		Impact: "Usually a leftover from migration; UA is deprecated and may add noise/overhead.",
		Fix:    "Confirm GA4 is primary and remove obsolete Universal Analytics tags once migration is complete.",
	},

	// --- urls: URL hygiene ---
	"url-uppercase": {
		What:   "The URL path contains uppercase letters.",
		Impact: "URLs are case-sensitive on most servers, risking duplicate-content and broken-link issues.",
		Fix:    "Use lowercase paths and redirect mixed-case variants to the canonical lowercase URL.",
	},
	"url-underscore": {
		What:   "The URL path contains underscores.",
		Impact: "Search engines treat hyphens, not underscores, as word separators, weakening keyword parsing.",
		Fix:    "Prefer hyphens between words in URLs; redirect old underscore URLs if changed.",
	},
	"url-non-ascii": {
		What:   "The URL contains non-ASCII characters.",
		Impact: "Non-ASCII URLs may be percent-encoded inconsistently, causing ugly or broken links.",
		Fix:    "Use ASCII, hyphen-separated slugs where practical.",
	},
	"url-too-long": {
		What:   "The URL is excessively long.",
		Impact: "Overly long URLs are harder to share, may be truncated, and can signal poor structure.",
		Fix:    "Shorten the path to a concise, descriptive slug.",
	},

	// --- utm: campaign tagging ---
	"utm-internal-tagged": {
		What:   "A UTM-tagged link points to the same site.",
		Impact: "Internal UTM links start a new analytics session, breaking attribution and inflating source counts.",
		Fix:    "Remove UTM parameters from internal links; use them only on inbound/external campaign links.",
	},
	"utm-partial-tagging": {
		What:   "A link has some but not all of utm_source/utm_medium/utm_campaign.",
		Impact: "Incomplete tagging produces gaps in campaign attribution reports.",
		Fix:    "Include at least utm_source, utm_medium, and utm_campaign on campaign links.",
	},
	"utm-empty-value": {
		What:   "A link has UTM parameters with empty values.",
		Impact: "Empty UTM values record blank dimensions, polluting analytics reports.",
		Fix:    "Populate every UTM parameter with a meaningful value, or remove it.",
	},
	"utm-duplicate-param": {
		What:   "A link repeats the same UTM parameter.",
		Impact: "Duplicated parameters are ambiguous; analytics tools may pick an unexpected value.",
		Fix:    "Keep each UTM parameter once per URL.",
	},
	"utm-inconsistent-casing": {
		What:   "UTM parameter keys are not lowercase.",
		Impact: "Analytics tools are case-sensitive, so mixed-case keys fragment campaign data.",
		Fix:    "Use lowercase UTM keys (utm_source, utm_medium, utm_campaign, etc.) consistently.",
	},
	"utm-summary": {
		What:   "Counts of tagged vs. untagged links (internal/external) on the page.",
		Impact: "Informational. Overview of the page's UTM tagging hygiene.",
		Fix:    "No action needed. Review if internal links are tagged or campaign links are untagged.",
	},
}
