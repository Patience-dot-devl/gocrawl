package report

import "maps"

// Explanation describes a single issue code in plain language: what the finding
// means, why it matters, and what to do about it. It is keyed by an Issue's Code
// and surfaced in the HTML report so a reader can act on a finding without
// consulting external documentation.
type Explanation struct {
	// What the issue entails — a plain-language description of the finding.
	What string `json:"what"`
	// Impact — why it matters / the potential consequence if left unaddressed.
	Impact string `json:"impact"`
	// Fix — the recommended remediation.
	Fix string `json:"fix"`
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

// Explanations returns a copy of every registered issue-code explanation, keyed by Code. It
// backs the web API's /api/explanations endpoint, which lets the live report view show the
// same what/impact/fix text as the HTML report without duplicating this map in the frontend.
// Returns a copy so a caller can't mutate the package's single source of truth.
func Explanations() map[string]Explanation {
	return maps.Clone(explanations)
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
	"content-thin": {
		What:   "The page has very little textual content.",
		Impact: "Thin pages rank poorly and can dilute overall site quality in search engines' eyes.",
		Fix:    "Add substantive, useful content, consolidate with a richer page, or noindex if the page has no standalone value.",
	},
	"content-low": {
		What:   "The page's word count is well below the site average.",
		Impact: "May indicate an under-developed page that underperforms relative to its peers.",
		Fix:    "Review whether the page needs more depth, or confirm the short length is intentional (e.g. a contact page).",
	},

	// --- botwall: CAPTCHA / bot-protection challenge detection ---
	"botwall-challenge": {
		What:   "The page served a CAPTCHA or bot-protection challenge (e.g. reCAPTCHA, Turnstile, Cloudflare, DataDome) instead of the real content. Challenge walls often return HTTP 200, so the crawl looks successful while the body is just the wall.",
		Impact: "Every other finding on this page is unreliable — the analyzers audited the challenge HTML, not your page. If many pages are blocked, the whole crawl is compromised.",
		Fix:    "Crawl from an allow-listed IP/User-Agent, lower concurrency and rate, or coordinate with whoever manages the WAF/bot rules. For staging, allow-list the crawler. Re-crawl once access is granted.",
	},
	"botwall-captcha-widget": {
		What:   "The page embeds a CAPTCHA widget (reCAPTCHA, hCaptcha, or Turnstile) within otherwise-normal content — typically on a form, not a block page.",
		Impact: "Informational. The page was crawled fine; this just notes a CAPTCHA is present (which can add third-party scripts and affect form-completion metrics).",
		Fix:    "No action needed unless the widget is unexpected. Ensure it loads only where required to limit third-party script weight.",
	},

	// --- datalayer: GTM/gtag wiring and dataLayer event stream audit ---
	"datalayer-not-collected": {
		What:   "dataLayer/event checks were skipped because the crawl did not use headless rendering.",
		Impact: "Event inventory, GA4 e-commerce validation, duplicate-conversion detection, PII scanning, and tag-firing checks all require a rendered DOM and are not run.",
		Fix:    "Re-run the crawl with --render headless to enable the full dataLayer audit.",
	},
	"datalayer-gtm-noscript-missing": {
		What:   "A GTM container is present but the <noscript> fallback iframe is missing.",
		Impact: "Visitors with JavaScript disabled (or blocked before GTM loads) are not tracked at all for this container.",
		Fix:    "Add the GTM-provided <noscript><iframe src=\"https://www.googletagmanager.com/ns.html?id=GTM-XXXX\">...</iframe></noscript> snippet immediately after the opening <body> tag.",
	},
	"datalayer-gtm-snippet-not-in-head": {
		What:   "The GTM container snippet is not placed inside <head>.",
		Impact: "Loading GTM later in the document delays every tag it manages, including analytics and consent signals that ideally fire as early as possible.",
		Fix:    "Move the GTM snippet as high in <head> as possible, per Google's installation instructions.",
	},
	"datalayer-push-before-init": {
		What:   "dataLayer.push() is called before the dataLayer array is initialized.",
		Impact: "Pushes that run before initialization are silently lost, so early events (and any data they carry) never reach GTM/gtag.",
		Fix:    "Ensure the dataLayer initialization (e.g. window.dataLayer = window.dataLayer || []) runs before any push() call, ideally as the very first script in <head>.",
	},
	"datalayer-init-missing": {
		What:   "A tag manager (GTM or a GA4 measurement ID) is present, but no dataLayer initialization was found anywhere in the page HTML.",
		Impact: "Without an initialized dataLayer, events pushed by on-page scripts or tag configurations have nowhere to go, silently breaking measurement.",
		Fix:    "Add the standard dataLayer initialization snippet before any code that pushes to it or configures a tag.",
	},
	"datalayer-gtag-config-id-mismatch": {
		What:   "gtag('config', ID) targets an ID that isn't loaded by any googletagmanager.com/gtag/js script on the page.",
		Impact: "A config call with no matching loader typically means the tag never actually initializes, so its data (pageviews, conversions) is not collected.",
		Fix:    "Load the gtag.js library with the same ID via <script async src=\"https://www.googletagmanager.com/gtag/js?id=ID\">, or correct the ID in the config call.",
	},
	"datalayer-consent-mode-present": {
		What:   "Google Consent Mode default/update signals were detected.",
		Impact: "Positive signal. Consent Mode lets analytics/ads tags adjust behavior based on visitor consent, which is required in several jurisdictions.",
		Fix:    "No action needed. Keep the consent defaults in sync with your cookie/consent banner.",
	},
	"datalayer-consent-mode-missing": {
		What:   "Analytics and/or ads tags are present, but no Google Consent Mode default was found.",
		Impact: "Tags may fire before a visitor has made a consent choice, which can violate privacy regulations (e.g. GDPR) in applicable regions.",
		Fix:    "Call gtag('consent', 'default', {...}) with appropriate defaults before the tag configuration, and update it when the visitor responds to your consent banner.",
	},
	"datalayer-empty": {
		What:   "A tag manager is present in the HTML, but window.dataLayer was empty or absent after the page rendered.",
		Impact: "No events are reaching GTM/gtag at runtime, so analytics, conversions, and any dependent audits (e-commerce, duplicates, PII) have nothing to inspect — tracking is effectively broken.",
		Fix:    "Check the browser console for JavaScript errors, verify the dataLayer variable name matches what GTM expects, and confirm no consent/ad-blocking tool is stripping it.",
	},
	"datalayer-events": {
		What:   "Informational inventory of the distinct event names observed in the dataLayer and how many times each fired.",
		Impact: "No impact by itself; useful for confirming the expected events are present and firing the expected number of times.",
		Fix:    "No action needed. Cross-check the inventory against your tagging plan.",
	},
	"datalayer-page-view-missing": {
		What:   "No page_view (or GTM lifecycle event such as gtm.js/gtm.load) was found in the dataLayer.",
		Impact: "The page load itself isn't being measured, so pageview counts and any funnel that starts from a page view will undercount this page.",
		Fix:    "Confirm GTM's built-in page-view trigger or gtag's automatic page_view is enabled and not blocked by a misconfigured trigger or consent setting.",
	},
	"datalayer-ecommerce-event-invalid": {
		What:   "A GA4 recommended e-commerce event (e.g. purchase, add_to_cart) is missing one or more of its required parameters.",
		Impact: "Incomplete e-commerce events produce gaps or errors in GA4 revenue/e-commerce reporting.",
		Fix:    "Add the missing parameters listed in the issue's data (e.g. transaction_id, value, currency, items) to the event payload.",
	},
	"datalayer-param-type": {
		What:   "An e-commerce event parameter has the wrong shape (e.g. value/currency not the expected type, or items not a non-empty array).",
		Impact: "GA4 may silently drop or misinterpret the parameter, corrupting revenue and item-level reporting for the event.",
		Fix:    "Send value as a number, currency as an ISO-4217 code (e.g. \"USD\"), and items as a non-empty array of item objects.",
	},
	"datalayer-duplicate-event": {
		What:   "A conversion-tracked event (e.g. purchase, generate_lead, sign_up) fired more than once for the same page load.",
		Impact: "Double-firing inflates the reported conversion count, skewing campaign performance and ROAS calculations.",
		Fix:    "Guard the event push so it only fires once per conversion (e.g. dedupe on a transaction/session ID, or fix a trigger that's firing twice).",
	},
	"datalayer-duplicate-transaction": {
		What:   "The same purchase transaction_id appeared in more than one purchase event.",
		Impact: "The same order is counted as multiple purchases, inflating revenue and order-count metrics in GA4.",
		Fix:    "Ensure the purchase event fires exactly once per order (e.g. guard against a page refresh or back-navigation re-firing the tag).",
	},
	"datalayer-pii": {
		What:   "A value that looks like an email address or phone number was pushed into the dataLayer.",
		Impact: "Personal data flowing into Google/GTM's dataLayer without safeguards can violate GA4's terms of service and privacy regulations (e.g. GDPR).",
		Fix:    "Remove or hash/pseudonymize personal data before pushing to the dataLayer; if you need user-level identifiers, use GA4's User-ID feature with proper consent instead.",
	},
	"datalayer-tag-not-firing": {
		What:   "A tag detected in the page HTML (GA4, Google Ads, Meta Pixel, or GTM) issued no matching network beacon during render.",
		Impact: "The tag's data isn't reaching its destination — could be a broken installation, a trigger that never fires, consent gating, or ad-blocking, so its metrics are silently missing.",
		Fix:    "Check the tag's trigger conditions, confirm it isn't blocked by consent settings or an ad blocker, and verify the beacon fires in the browser's network tab.",
	},
	"datalayer-tags-firing": {
		What:   "Informational confirmation that a detected tag issued a matching network beacon during render.",
		Impact: "No impact; this confirms the tag is actually collecting data, not just installed.",
		Fix:    "No action needed.",
	},

	// --- wordpress: WordPress fingerprinting and WP-specific checks ---
	"wp-detected": {
		What:   "The site is built on WordPress.",
		Impact: "Informational. Enables the WordPress-specific checks below; no action needed by itself.",
		Fix:    "No action needed.",
	},
	"wp-version-exposed": {
		What:   "The WordPress core version is disclosed via the generator meta tag.",
		Impact: "Publishing the exact version makes it trivial for an attacker to look up known CVEs affecting that release.",
		Fix:    "Remove or blank the generator tag (many SEO plugins offer this, or filter it out with remove_action('wp_head', 'wp_generator')).",
	},
	"wp-emoji-enabled": {
		What:   "The wp-emoji script is loaded sitewide.",
		Impact: "Adds an extra script and inline configuration to every page for a feature modern browsers render natively, at minor performance cost.",
		Fix:    "Dequeue wp-emoji-release.min.js (e.g. via disable-emojis in a plugin or theme) unless you specifically need the polyfill.",
	},
	"wp-jquery-migrate": {
		What:   "The jQuery Migrate compatibility shim is loaded.",
		Impact: "Adds an extra script sitewide that only matters if older jQuery-plugin code depends on removed APIs.",
		Fix:    "Remove jQuery Migrate if no theme/plugin code needs it, or confirm it's still required before dropping it.",
	},
	"wp-many-plugin-assets": {
		What:   "A large number of distinct plugins are enqueuing their own front-end CSS/JS.",
		Impact: "Each plugin's assets add HTTP requests and often render-blocking resources, compounding page weight and load time.",
		Fix:    "Audit plugins for ones that can be consolidated or removed, and consider an asset-combining/critical-CSS strategy for the rest.",
	},
	"wp-default-tagline": {
		What:   `The site still uses WordPress's default "Just another WordPress site" tagline.`,
		Impact: "A visible sign the site wasn't fully configured, and a missed opportunity to use the tagline slot for something SEO-relevant.",
		Fix:    "Set a real tagline under Settings → General, or remove it from the theme if unused.",
	},
	"wp-no-seo-plugin": {
		What:   "No recognized SEO plugin (Yoast, Rank Math, All in One SEO) was detected.",
		Impact: "Without one, the site likely lacks convenient control over titles, meta descriptions, canonical tags, and sitemaps — informational, not necessarily a problem if these are handled another way.",
		Fix:    "Install a maintained SEO plugin if titles/meta/sitemaps aren't otherwise managed, or confirm they're handled via theme code or another mechanism.",
	},
	"wp-multiple-seo-plugins": {
		What:   "More than one SEO plugin is active at the same time.",
		Impact: "Competing plugins can each inject their own titles, meta tags, or sitemaps, producing conflicting or duplicate output that confuses search engines.",
		Fix:    "Deactivate all but one SEO plugin, and verify the surviving plugin's settings after removing the others.",
	},
	"wp-multilingual-detected": {
		What:   "A multilingual plugin (WPML, Polylang, TranslatePress, or Weglot) is active.",
		Impact: "Informational. Enables the hreflang-related checks below.",
		Fix:    "No action needed.",
	},
	"wp-i18n-no-hreflang": {
		What:   "A multilingual plugin is active, but no hreflang alternate links were found on any crawled page.",
		Impact: "Search engines can't see the language/region relationships between the site's translated pages, so they may serve the wrong language version in search results.",
		Fix:    "Configure the multilingual plugin to emit <link rel=\"alternate\" hreflang=\"...\"> tags for every translated page.",
	},
	"wp-ugly-permalink": {
		What:   "The page uses a default plain permalink (e.g. ?p=123) instead of a pretty URL.",
		Impact: "Plain permalinks are less readable, less memorable, and typically carry less descriptive/keyword signal than a pretty URL structure.",
		Fix:    "Switch to a pretty permalink structure under Settings → Permalinks, and redirect old plain-permalink URLs to their pretty equivalents.",
	},
	"wp-i18n-lang-query-param": {
		What:   "Language is selected via a ?lang= query parameter rather than a per-language path or subdomain.",
		Impact: "Query-parameter language negotiation is the weakest option for SEO: it's easy to miss in crawls, doesn't cleanly separate language variants as distinct URLs, and complicates hreflang mapping.",
		Fix:    "Configure the multilingual plugin to use per-language paths (e.g. /fr/) or subdomains instead of a query parameter.",
	},
	"wp-html-lang-mismatch": {
		What:   "The <html lang> attribute disagrees with the language requested in the URL.",
		Impact: "Browsers, screen readers, and search engines may apply the wrong language's rules (pronunciation, hyphenation, indexing) to the page.",
		Fix:    "Ensure the theme/plugin sets <html lang> to match the language actually being served for that URL.",
	},
	"wp-acf-leaked-markup": {
		What:   "Unrendered Advanced Custom Fields markup (a field tag or shortcode) is leaking into the visible page content instead of being executed.",
		Impact: "Visitors see raw template code or shortcode text instead of the intended field value, which looks broken and can expose implementation details.",
		Fix:    "Fix the template/shortcode context so the ACF tag or shortcode is actually executed (e.g. ensure the shortcode is registered and do_shortcode() is applied, or the PHP field call runs in a template, not printed as text).",
	},
	"wp-indexable-attachment": {
		What:   "A WordPress attachment page is indexable.",
		Impact: "Attachment pages are thin, auto-generated pages with little unique content; indexing them dilutes site quality and wastes crawl budget.",
		Fix:    "Set attachment pages to noindex, or redirect them to their parent post/page.",
	},
	"wp-indexable-search": {
		What:   "An internal search-results page is indexable.",
		Impact: "Search-results pages are typically thin, duplicative, and can expose internal query strings; indexing them adds low-value pages to search results.",
		Fix:    "Set internal search-results pages to noindex (most SEO plugins offer this as a toggle).",
	},
	"wp-indexable-author-archive": {
		What:   "An author archive page is indexable.",
		Impact: "On single-author sites, the author archive usually duplicates the blog index, diluting ranking signals across two near-identical pages.",
		Fix:    "Noindex author archives on single-author sites, or disable them entirely if they add no value.",
	},
	"wp-indexable-date-archive": {
		What:   "A date-based archive (year/month/day) is indexable.",
		Impact: "Date archives are thin, duplicate-prone listings of existing content that add little unique value and can dilute ranking signals.",
		Fix:    "Noindex date archives, or disable them if the theme doesn't rely on them.",
	},
	"wp-xmlrpc-enabled": {
		What:   "xmlrpc.php is reachable and responding.",
		Impact: "xmlrpc.php is a well-known brute-force amplification vector (multiple login attempts per request) and can be abused for pingback-based DDoS against other sites.",
		Fix:    "Disable XML-RPC entirely if not needed (e.g. via a security plugin or by blocking the endpoint at the server/WAF level), or restrict it to trusted IPs if a specific integration requires it.",
	},
	"wp-user-enumeration-rest": {
		What:   "The REST API exposes valid usernames via /wp-json/wp/v2/users.",
		Impact: "Leaked usernames narrow a brute-force or credential-stuffing attack to real accounts, removing half the guesswork for an attacker.",
		Fix:    "Restrict or filter the REST users endpoint (e.g. require authentication, or use a security plugin to disable public user enumeration).",
	},
	"wp-user-enumeration-author": {
		What:   "Requesting /?author=1 redirects to /author/<login>/, leaking a valid username.",
		Impact: "Like the REST endpoint, this narrows brute-force/credential-stuffing attempts to confirmed real usernames.",
		Fix:    "Block or rewrite author-ID-based query redirects (a security plugin, or custom rewrite rules, can prevent the username from leaking).",
	},
	"wp-directory-listing": {
		What:   "The wp-content/uploads directory is browsable (directory listing enabled).",
		Impact: "Visitors and attackers can browse and download every uploaded file, including anything not meant to be publicly linked.",
		Fix:    "Disable directory listing on the web server (e.g. Options -Indexes in Apache, or the equivalent Nginx/other-server configuration) for the uploads directory.",
	},
	"wp-readme-exposed": {
		What:   "readme.html is reachable and discloses the exact WordPress core version.",
		Impact: "Like the generator meta tag, this lets an attacker map the install to known CVEs for that specific release.",
		Fix:    "Delete or block access to readme.html in production (many security plugins/server rules can do this automatically).",
	},

	// --- shopify: Shopify storefront checks ---
	"shopify-detected": {
		What:   "The site was identified as a Shopify storefront, with the theme it runs.",
		Impact: "Informational. It tells the rest of this report which template each URL renders and which checks apply.",
		Fix:    "No action needed.",
	},
	"shopify-template-schema-gap": {
		What:   "One or more crawled pages of this Shopify template are missing a schema.org type that template should carry; the finding's page count says how many.",
		Impact: "Pages of this template win no rich result for the missing type — product pages without Product markup, say, get no price, rating, or availability treatment in search.",
		Fix:    "Add the markup to the template once (theme Liquid or an SEO app), and every page it renders gains it. Check the example URLs to confirm the template was identified correctly.",
	},
	"shopify-schema-app-conflict": {
		What:   "Two sources — typically the theme and an SEO app — each emit Product structured data on the same page, without knowing about each other.",
		Impact: "Search engines pick one and discard the other, so the page may be represented by stale or incomplete markup, and any disagreement between them risks a structured-data manual action.",
		Fix:    "Pick one owner. Either disable structured data in the theme (most themes expose a setting) or turn off the app's Product schema, so a single source emits it.",
	},
	"shopify-schema-client-injected": {
		What:   "A structured-data app is installed, but the HTML served to the crawler contains no JSON-LD — the app is injecting it with JavaScript.",
		Impact: "Google renders JavaScript and will usually see it, but rendering is deferred and other crawlers and AI answer engines often do not render at all, so the markup is invisible to them.",
		Fix:    "Re-run the crawl with --render headless to confirm what the app emits. Prefer server-rendered structured data in theme Liquid, where every crawler sees it on the first fetch.",
	},
	"shopify-flat-variant-product": {
		What:   "The page sells several variants but its markup describes a single Product, with no ProductGroup or hasVariant.",
		Impact: "Search engines see one item where the store sells several, so variant-level attributes — size, colour, per-variant price and availability — never reach Shopping or rich results.",
		Fix:    "Emit a ProductGroup with productGroupID and variesBy, and one Product per variant under hasVariant.",
	},
	"shopify-single-offer-range": {
		What:   "The product's variants are priced differently, but the markup states one Offer price.",
		Impact: "The stated price is wrong for every variant that does not match it, and a price that contradicts the page risks suppression of price-bearing rich results.",
		Fix:    "Use an AggregateOffer with lowPrice and highPrice, or give each variant its own Offer under a ProductGroup.",
	},
	"shopify-indexable-utility": {
		What:   "A Shopify utility page — /search, /cart, /account/*, /challenge, /checkouts, /orders or /password — is crawlable and not marked noindex. /policies/* is deliberately excluded: those pages are meant to be indexed.",
		Impact: "These pages carry no content worth ranking, and /search in particular generates an unbounded set of URLs from whatever anyone links to, wasting crawl budget and risking thin-content pages in the index.",
		Fix:    "Add <meta name=\"robots\" content=\"noindex,follow\"> to the utility templates in theme.liquid, or disallow the paths in robots.txt.liquid.",
	},
	"shopify-indexable-facet": {
		What:   "A sorted or filtered collection URL (?sort_by=, ?filter.*=) is indexable and does not canonicalise to the unfiltered collection.",
		Impact: "Each permutation is the same products in a different order, competing with the collection it came from and multiplying crawl budget across near-identical pages.",
		Fix:    "Emit a canonical pointing at the unfiltered collection URL on every faceted variant, and consider disallowing the parameters in robots.txt.",
	},
	"shopify-duplicate-product-path": {
		What:   "A product is served at /collections/<collection>/products/<handle> without a canonical pointing at /products/<handle>.",
		Impact: "Shopify serves a product once per collection it belongs to, so a product in ten collections becomes ten competing URLs, splitting link signals across all of them.",
		Fix:    "Most themes already emit the right canonical; if yours does not, set it to {{ product.url }} prefixed with the shop URL rather than {{ canonical_url }} in a collection context.",
	},
	"shopify-products-json-exposed": {
		What:   "The store's /products.json endpoint answers unauthenticated requests with the product catalogue.",
		Impact: "Titles, handles, variants and prices can be scraped wholesale by competitors and repricing bots. Shopify enables this by default, so it is worth a deliberate decision rather than an accident.",
		Fix:    "If the catalogue is not meant to be public, block /products.json (and /collections/*/products.json) at the CDN or in robots.txt. Note that robots.txt deters crawlers but does not prevent access.",
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
	"http-fetch-error": {
		What:   "The page could not be fetched at all.",
		Impact: "Users and crawlers cannot reach the page; it cannot rank and damages site reliability.",
		Fix:    "Investigate DNS, TLS, timeouts, or server availability for the URL and ensure it responds.",
	},
	"http-server-error": {
		What:   "The URL returned a 5xx server error.",
		Impact: "The page is unavailable; persistent 5xx responses cause de-indexing and lost traffic.",
		Fix:    "Check server logs and fix the application/infrastructure fault causing the error.",
	},
	"http-client-error": {
		What:   "The URL returned a 4xx client error (e.g. 404, 403, 410).",
		Impact: "The page is unreachable; inbound links and crawl budget are wasted.",
		Fix:    "Restore the page, 301-redirect to a relevant URL, or fix the access/permission issue.",
	},
	"http-redirect-loop": {
		What:   "The URL redirects in a cycle that never resolves to a final page.",
		Impact: "Users and crawlers get trapped; the content is effectively unreachable.",
		Fix:    "Break the loop so the chain terminates at a single 200-status destination.",
	},
	"http-redirect-chain": {
		What:   "The URL passes through multiple redirects before reaching its final destination.",
		Impact: "Each hop adds latency and dilutes ranking signals; long chains risk being abandoned by crawlers.",
		Fix:    "Collapse the chain to a single redirect that points straight to the final URL.",
	},
	"http-redirect": {
		What:   "The URL redirects to another location.",
		Impact: "Informational. A single redirect is normal, but update internal links to point at the destination.",
		Fix:    "Where possible, link directly to the final URL to avoid unnecessary hops.",
	},
	"http-slow-response": {
		What:   "The server's response was slower than the configured threshold.",
		Impact: "Slow responses hurt user experience, crawl efficiency, and Core Web Vitals (TTFB).",
		Fix:    "Optimise backend processing, caching, or CDN delivery to lower response time.",
	},
	"http-mixed-content": {
		What:   "An HTTPS page loads insecure http:// resources.",
		Impact: "Browsers may block the resources or warn users, breaking functionality and eroding trust.",
		Fix:    "Update all subresource URLs (scripts, images, styles, iframes) to https://.",
	},
	"http-body-truncated": {
		What:   "The page body was cut short — either it exceeded the crawler's fetch size limit, or the connection failed partway through the read.",
		Impact: "Other findings on this page (missing title/H1, invalid structured data, broken markup) may be false positives caused by content past the cutoff never being seen, not an actual issue with the page.",
		Fix:    "If the page is legitimately large, raise crawl.max_body_bytes in the config (or GOCRAWL_CRAWL_MAX_BODY_BYTES) and re-crawl. If it's not expected to be large, investigate why the response was cut off (a slow/unstable connection, or a server bug producing an unbounded response).",
	},

	// --- images ---
	"img-missing-alt": {
		What:   "One or more images have no alt attribute.",
		Impact: "Hurts accessibility for screen-reader users and removes image-search/context signals.",
		Fix:    "Add descriptive alt text to meaningful images; use empty alt=\"\" for purely decorative ones.",
	},
	"img-empty-alt": {
		What:   "One or more images have an explicit empty alt=\"\". This is informational, not a defect: alt=\"\" is the correct markup for a purely decorative image, and gocrawl cannot tell from markup alone whether an image carries meaning.",
		Impact: "If the images are decorative, nothing is wrong. If they are content images (product shots, diagrams, photos), an empty alt hides them from screen readers and from image search, and a CMS or theme that emits alt=\"\" for everything looks identical to a correctly decorated page unless you review the list.",
		Fix:    "Review the listed images. Give the meaningful ones descriptive alt text; leave alt=\"\" on the ones that are purely decorative.",
	},
	"img-duplicate-alt": {
		What:   "Several images on the same page share the same alt text.",
		Impact: "Repeated alt text stops distinguishing the images: screen-reader users hear the same label many times, and image search gets no signal about what each variant actually shows.",
		Fix:    "Write alt text that describes each specific image (angle, variant, context) rather than reusing one product or page name.",
	},
	"img-alt-is-filename": {
		What:   "The alt text is just the image's filename restated (e.g. kantoor.jpg with alt=\"Kantoor\").",
		Impact: "A filename is not a description, so the alt adds no accessibility or image-search value while looking populated to automated checks.",
		Fix:    "Replace it with a sentence fragment describing what the image shows.",
	},
	"img-nondescriptive-filename": {
		What:   "Image filenames carry no meaning — pure numbers (7.jpg), camera or export defaults (IMG_1234.jpg, unnamed.png), content hashes, or names too short to be words (hs1.png).",
		Impact: "Filenames are a ranking signal for image search and are used as a fallback label; meaningless ones waste that signal.",
		Fix:    "Rename files to short, hyphenated descriptions of their content before upload (aed-wall-cabinet.jpg).",
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
	"link-broken": {
		What:   "An internal link points to a page that returns an error status.",
		Impact: "Creates dead ends for users and crawlers and wastes crawl budget and link equity.",
		Fix:    "Fix or remove the link, or repair the destination URL.",
	},
	"link-to-redirect": {
		What:   "An internal link points to a URL that redirects.",
		Impact: "Adds an unnecessary hop, slowing navigation and diluting link signals.",
		Fix:    "Update the link to point directly to the redirect's final destination.",
	},
	"link-empty-anchor": {
		What:   "One or more links have empty anchor text.",
		Impact: "Empty anchors give no context to users or search engines and hurt accessibility.",
		Fix:    "Add descriptive anchor text, or aria-label/alt where the link wraps an image/icon.",
	},
	"link-summary": {
		What:   "A count of total, external, and nofollow links on the page.",
		Impact: "Informational. Useful for understanding the page's link profile.",
		Fix:    "No action needed. Review if external/nofollow ratios look unexpected.",
	},
	"link-inbound": {
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
	"perf-cwv-render-failed": {
		What:   "Headless rendering failed, so Core Web Vitals were not collected for the page.",
		Impact: "Informational. No CWV data is available for this page in this run.",
		Fix:    "Re-run with rendering enabled and check that the headless browser can load the page.",
	},
	"perf-cwv-measured": {
		What:   "Core Web Vitals were measured in lab mode (LCP, FCP, CLS, TBT, TTFB).",
		Impact: "Informational. These lab metrics approximate field performance.",
		Fix:    "No action needed. Review the individual metric findings for any that need improvement.",
	},
	"perf-render-incomplete": {
		What:   "In headless mode the rendered DOM came back much smaller than the raw HTML, so the page had likely not finished rendering when it was snapshotted. gocrawl analyzed the raw HTML for this page instead, so structural checks (H1, meta tags, content) are still accurate.",
		Impact: "The page's Core Web Vitals for this run are unreliable. Without the raw-HTML fallback this would also cause false 'missing H1', 'missing meta description', and 'thin content' findings.",
		Fix:    "Usually harmless. If you need trustworthy CWV for slow pages, re-run headless with a longer settle / fewer concurrent workers, or crawl the page on its own. For SEO structure, raw mode (the default) is reliable.",
	},
	"perf-lcp-needs-improvement": {
		What:   "Largest Contentful Paint is above the 'good' threshold (2.5s).",
		Impact: "Slower perceived load; borderline LCP can reduce rankings and conversions.",
		Fix:    "Optimise the LCP element: faster server/TTFB, preloaded hero image/font, fewer render-blocking resources.",
	},
	"perf-lcp-poor": {
		What:   "Largest Contentful Paint is poor (above ~4s).",
		Impact: "Users perceive the page as slow to load; a clear negative ranking and UX factor.",
		Fix:    "Significantly speed up the main content paint: optimise images, server response, and critical-path CSS/JS.",
	},
	"perf-fcp-needs-improvement": {
		What:   "First Contentful Paint is above the 'good' threshold (1.8s).",
		Impact: "Users wait longer to see any content, hurting perceived speed.",
		Fix:    "Reduce render-blocking resources and improve TTFB so first paint happens sooner.",
	},
	"perf-fcp-poor": {
		What:   "First Contentful Paint is poor (above ~3s).",
		Impact: "The page appears blank for too long, increasing bounce risk.",
		Fix:    "Cut render-blocking CSS/JS, inline critical CSS, and improve server response time.",
	},
	"perf-cls-needs-improvement": {
		What:   "Cumulative Layout Shift is above the 'good' threshold (0.1).",
		Impact: "Visible layout jumps frustrate users and can cause misclicks.",
		Fix:    "Reserve space for images/ads/embeds with explicit dimensions and avoid inserting content above existing content.",
	},
	"perf-cls-poor": {
		What:   "Cumulative Layout Shift is poor (above 0.25).",
		Impact: "Significant layout instability; a strong negative UX and ranking signal.",
		Fix:    "Set dimensions on all media, preload fonts to avoid FOUT/FOIT, and stabilise dynamic content insertion.",
	},
	"perf-tbt-needs-improvement": {
		What:   "Total Blocking Time (a lab proxy for INP) is above the 'good' threshold (200ms).",
		Impact: "The main thread is busy enough to make the page feel sluggish to interact with.",
		Fix:    "Break up long JavaScript tasks, defer non-critical scripts, and reduce third-party JS.",
	},
	"perf-tbt-poor": {
		What:   "Total Blocking Time (lab proxy for INP) is poor (above 600ms).",
		Impact: "Interactions are noticeably delayed; a strong negative responsiveness signal.",
		Fix:    "Aggressively reduce and split main-thread JavaScript; remove or lazy-load heavy third-party scripts.",
	},
	"perf-ttfb-needs-improvement": {
		What:   "Time to First Byte is above the 'good' threshold (800ms).",
		Impact: "A slow server response delays everything downstream, including LCP and FCP.",
		Fix:    "Improve backend performance, add caching/CDN, and reduce redirects before the document loads.",
	},
	"perf-ttfb-poor": {
		What:   "Time to First Byte is poor (above ~1.8s).",
		Impact: "The server is slow to respond, dragging down all other load metrics.",
		Fix:    "Investigate slow backend queries, enable caching/CDN, and right-size hosting.",
	},
	"perf-cwv-not-collected": {
		What:   "Core Web Vitals were not collected because headless rendering was not enabled.",
		Impact: "Informational. No CWV data for this run.",
		Fix:    "Run with --render headless to collect Core Web Vitals.",
	},
	"perf-response-time": {
		What:   "The measured server response time (a TTFB proxy) for the page.",
		Impact: "Informational. High values indicate slow server responses.",
		Fix:    "No action needed unless the value is high; then optimise backend/caching.",
	},

	// --- robots: robots.txt ---
	"robots-missing": {
		What:   "No robots.txt file was found.",
		Impact: "Crawling defaults to fully allowed; you lose a place to declare sitemaps and crawl directives.",
		Fix:    "Add a robots.txt (even a permissive one) and declare your sitemap location.",
	},
	"robots-no-sitemap-declared": {
		What:   "robots.txt does not declare a Sitemap directive.",
		Impact: "Search engines have to discover the sitemap by convention rather than being told where it is.",
		Fix:    "Add a 'Sitemap: https://…/sitemap.xml' line to robots.txt.",
	},
	"robots-sitemaps-declared": {
		What:   "robots.txt declares one or more sitemaps.",
		Impact: "Positive signal. Helps crawlers find your sitemap(s).",
		Fix:    "No action needed. Ensure the declared sitemap URLs are valid.",
	},
	"robots-crawled-disallowed": {
		What:   "A URL that robots.txt disallows was nonetheless crawled (per crawler configuration).",
		Impact: "Indicates a mismatch between intended crawl rules and actual crawling; compliant bots would skip it.",
		Fix:    "Confirm the disallow rule is intentional, and that production bots respect it.",
	},

	// --- consent: CMP, Google Consent Mode, and pre-consent tracking ---
	"consent-cmp-detected": {
		What:   "A consent management platform (CMP) was detected on the site.",
		Impact: "Positive signal. A CMP is the mechanism that collects and records the visitor's tracking choice, which GDPR/ePrivacy require before non-essential cookies may be set.",
		Fix:    "No action needed. Verify the CMP actually blocks tags before consent — the other consent findings test exactly that.",
	},
	"consent-no-cmp": {
		What:   "The site loads analytics or advertising tags but no consent management platform was detected.",
		Impact: "Non-essential tracking without a consent mechanism is a direct GDPR/ePrivacy exposure in the EU, and Google requires a certified CMP for Ads and AdSense traffic in the EEA.",
		Fix:    "Deploy a CMP and configure it to block analytics/advertising tags until the visitor consents. A CMP injected by a tag manager may not be visible in static HTML — confirm manually before acting on this one.",
	},
	"consent-mode-v1-only": {
		What:   "Google Consent Mode is configured, but the v2 signals (ad_user_data, ad_personalization) are not declared.",
		Impact: "Google has required Consent Mode v2 for EEA traffic since March 2024. Without these signals Google Ads stops collecting for affected users, so remarketing audiences and conversion measurement degrade.",
		Fix:    "Add ad_user_data and ad_personalization to the gtag('consent', 'default', {…}) call alongside ad_storage and analytics_storage, and update all four when the visitor responds.",
	},
	"consent-mode-default-granted": {
		What:   "A Consent Mode default call grants tracking storage before the visitor has made a choice.",
		Impact: "This defeats the mechanism: tags behave as though consent was given, so cookies are set and data collected from visitors who never agreed.",
		Fix:    "Default the consent-gated signals to 'denied' and grant them only in the update call the CMP fires after acceptance. If defaults intentionally differ by region, scope them with the `region` key.",
	},
	"consent-mode-no-wait-for-update": {
		What:   "The Consent Mode default sets no wait_for_update value.",
		Impact: "A CMP that loads asynchronously may deliver the visitor's real choice after tags have already read the defaults, so a returning visitor's stored consent is missed and measurement is lost.",
		Fix:    "Add wait_for_update (commonly 500–2000 ms) to the default call so tags hold until the CMP has had a chance to update the state.",
	},
	"consent-mode-after-tags": {
		What:   "The Consent Mode default is declared later in the document than the gtag.js / gtm.js loader.",
		Impact: "Ordering is what makes Consent Mode work. Defaults declared after the loader cannot restrain tags that have already run, so the first page view tracks regardless of the configuration.",
		Fix:    "Move the gtag('consent', 'default', {…}) call above the tag manager or gtag.js snippet, as the first script in <head>.",
	},
	"consent-preconsent-tracking-cookie": {
		What:   "A known analytics or advertising cookie was set during a crawl that never accepted a consent banner.",
		Impact: "Setting non-essential cookies before consent is the most commonly enforced GDPR/ePrivacy violation, and it is trivially provable by anyone who loads the page with a clean profile.",
		Fix:    "Configure the CMP to block the tag that sets this cookie until consent is granted — not merely to hide the banner. Verify in a fresh browser profile by inspecting the cookie jar before clicking anything.",
	},
	"consent-preconsent-tracker-request": {
		What:   "The page contacted third-party measurement or advertising endpoints during a render that never accepted a consent banner.",
		Impact: "Stronger evidence than a cookie: the request itself hands the visitor's IP address and page context to the vendor before consent, which no 'strictly necessary' argument covers.",
		Fix:    "Ensure the CMP blocks tags from loading, not just from setting cookies. Google Consent Mode alone still sends cookieless pings — use tag blocking where those pings are unacceptable.",
	},
	"consent-cookie-inventory": {
		What:   "A rollup of every cookie observed before any consent was given, split into tracking and other.",
		Impact: "Informational. It is the evidence base for a cookie policy and the starting point of a consent audit; the 'other' list is what still needs classifying by hand.",
		Fix:    "No action needed. Reconcile the list against your published cookie policy. The data's `source` field says whether it came from the full browser jar (headless) or Set-Cookie headers alone (raw).",
	},

	// --- security: headers & forms ---
	"security-missing-hsts": {
		What:   "An HTTPS response has no Strict-Transport-Security header.",
		Impact: "Leaves users exposed to protocol-downgrade/man-in-the-middle attacks on first or subsequent visits.",
		Fix:    "Send Strict-Transport-Security with an appropriate max-age (and includeSubDomains where applicable).",
	},
	"security-missing-csp": {
		What:   "The response has no Content-Security-Policy header.",
		Impact: "Without CSP the page is more exposed to XSS and content-injection attacks.",
		Fix:    "Define a Content-Security-Policy restricting allowed script/style/resource origins.",
	},
	"security-missing-x-content-type-options": {
		What:   "The response has no X-Content-Type-Options: nosniff header.",
		Impact: "Browsers may MIME-sniff responses, enabling some content-type confusion attacks.",
		Fix:    "Send 'X-Content-Type-Options: nosniff' on responses.",
	},
	"security-insecure-form": {
		What:   "A form submits over insecure http://.",
		Impact: "Submitted data can be intercepted; browsers warn users, harming trust and conversions.",
		Fix:    "Point the form action at an https:// endpoint.",
	},

	// --- security audit: TLS, certificates, cookies (opt-in, --security-audit) ---
	"security-no-https": {
		What:   "Pages are served over plain HTTP and do not redirect to HTTPS.",
		Impact: "Traffic is readable and modifiable in transit, browsers label the site 'Not secure', and Google has treated HTTPS as a ranking signal for a decade.",
		Fix:    "Obtain a certificate and redirect all HTTP traffic to the HTTPS equivalent with a 301, then add Strict-Transport-Security.",
	},
	"security-tls-obsolete-version": {
		What:   "The connection negotiated TLS 1.0 or 1.1, protocol versions retired in 2020.",
		Impact: "These versions have known weaknesses, and current browsers refuse them outright — so affected visitors cannot load the site at all.",
		Fix:    "Disable TLS 1.0/1.1 in the server or CDN and require TLS 1.2 as a minimum, ideally offering TLS 1.3.",
	},
	"security-tls-legacy-version": {
		What:   "The connection negotiated TLS 1.2 rather than TLS 1.3.",
		Impact: "TLS 1.2 is still secure, but it needs an extra network round trip to handshake, which costs measurable time on every new connection.",
		Fix:    "Enable TLS 1.3 in the server or CDN; clients that don't support it keep negotiating 1.2 automatically.",
	},
	"security-tls-weak-cipher": {
		What:   "The connection negotiated a cipher suite classified as insecure (RC4, 3DES, or a vulnerable CBC mode).",
		Impact: "These suites have practical attacks against them, and they fail PCI DSS and most other compliance baselines.",
		Fix:    "Restrict the server's cipher list to modern AEAD suites (AES-GCM, ChaCha20-Poly1305) and remove the legacy ones.",
	},
	"security-tls-cert-expired": {
		What:   "The certificate the server presented is past its expiry date.",
		Impact: "Browsers show a full-page interstitial warning and crawlers stop indexing — the site is effectively offline for most visitors.",
		Fix:    "Renew the certificate immediately, then fix the renewal automation that let it lapse.",
	},
	"security-tls-cert-expiring-soon": {
		What:   "The certificate expires within 30 days (escalated to an error inside 14 days).",
		Impact: "If renewal fails, the site disappears behind a browser security warning with no grace period.",
		Fix:    "Confirm automated renewal is running and succeeding; ACME clients normally renew at 30 days, so anything closer means it has already failed once.",
	},
	"security-tls-cert-not-yet-valid": {
		What:   "The certificate's validity period starts in the future.",
		Impact: "Browsers reject it exactly as they would an expired certificate, blocking every visitor.",
		Fix:    "Check the server's system clock, and confirm the deployed certificate is the current one rather than a pre-issued replacement.",
	},
	"security-tls-cert-self-signed": {
		What:   "The server presented a self-signed certificate rather than one issued by a trusted CA.",
		Impact: "No browser trusts it, so every visitor sees a security warning, and it offers no protection against impersonation.",
		Fix:    "Replace it with a certificate from a publicly trusted CA — Let's Encrypt issues them free and automates renewal.",
	},
	"security-tls-incomplete-chain": {
		What:   "The server sent only its own certificate, omitting the intermediate CA certificate(s) that link it to a trusted root.",
		Impact: "Browsers usually recover by fetching the missing intermediate, at the cost of a round trip; clients that can't (many mobile apps, older Android, curl) fail the connection entirely.",
		Fix:    "Configure the server to serve the full chain — most CAs ship a 'fullchain' bundle for exactly this.",
	},
	"security-tls-weak-signature": {
		What:   "A certificate in the chain is signed with a broken hash algorithm (MD2, MD5, or SHA-1).",
		Impact: "Collision attacks against these algorithms are practical, so the certificate's authenticity can be forged; browsers reject them for publicly trusted certificates.",
		Fix:    "Reissue the certificate with a SHA-256 or stronger signature, and replace any intermediate still using the old algorithm.",
	},
	"security-tls-weak-key": {
		What:   "A certificate in the chain uses a public key below the CA/Browser Forum minimum (2048-bit RSA, 256-bit elliptic curve).",
		Impact: "Undersized keys are within reach of a well-resourced attacker, and no CA will renew a certificate that uses one.",
		Fix:    "Generate a new key of at least 2048-bit RSA (or a P-256 elliptic-curve key) and reissue the certificate against it.",
	},
	"security-tls-ok": {
		What:   "The TLS configuration and certificate chain passed every audit check.",
		Impact: "Positive signal. Transport security is in good order; the finding's data records the negotiated protocol, cipher, and remaining certificate lifetime.",
		Fix:    "No action needed. Keep automated certificate renewal in place.",
	},
	"security-cookie-no-secure": {
		What:   "A cookie is set over HTTPS without the Secure attribute.",
		Impact: "The browser will also send it over plain HTTP, so a single downgraded request — an http:// link, an ad, a stray redirect — leaks it in cleartext.",
		Fix:    "Add the Secure attribute to every cookie the site sets over HTTPS.",
	},
	"security-cookie-samesite-none-insecure": {
		What:   "A cookie declares SameSite=None but is not marked Secure.",
		Impact: "Browsers reject this combination outright, so the cookie is never stored — a common cause of broken cross-site embeds, payment returns, and consent state.",
		Fix:    "Add the Secure attribute alongside SameSite=None, or switch to SameSite=Lax if cross-site delivery isn't needed.",
	},
	"security-cookie-no-samesite": {
		What:   "A cookie has no SameSite attribute.",
		Impact: "The browser picks a default, and those defaults differ between browsers and keep changing — a CSRF exposure on session cookies and a source of attribution loss on marketing cookies.",
		Fix:    "Set SameSite explicitly: Lax for ordinary session cookies, None (with Secure) for cookies that must survive cross-site navigation.",
	},
	"security-cookie-no-httponly": {
		What:   "A session-style cookie (its name suggests a session, token, or credential) is readable by JavaScript.",
		Impact: "Any cross-site scripting flaw anywhere on the domain can read the cookie and hijack the session.",
		Fix:    "Add HttpOnly to cookies carrying session or authentication state; omit it only when JavaScript genuinely needs the value.",
	},
	"security-cookie-prefix-violation": {
		What:   "A cookie uses the reserved __Host- or __Secure- name prefix without meeting that prefix's requirements.",
		Impact: "Browsers refuse to store such a cookie, so whatever depends on it silently stops working.",
		Fix:    "Meet the prefix's rules (__Secure- requires Secure; __Host- also requires Path=/ and no Domain attribute), or drop the prefix.",
	},
	"security-cookie-long-lived": {
		What:   "A cookie requests a lifetime longer than the 400 days browsers now allow.",
		Impact: "Chrome and Safari silently truncate it to 400 days, so the retention period the site records — and any consent notice quoting it — is inaccurate.",
		Fix:    "Set a lifetime of 400 days or less, and align the figure with what the cookie/consent policy tells visitors.",
	},
	"security-hsts-short-max-age": {
		What:   "Strict-Transport-Security is present but its max-age is under 180 days.",
		Impact: "The HTTPS-only guarantee lapses that soon after a visit, reopening the downgrade window; the HSTS preload list also rejects anything this short.",
		Fix:    "Raise max-age to at least 15552000 (180 days), or 31536000 (one year) if you intend to preload.",
	},
	"security-hsts-no-subdomains": {
		What:   "Strict-Transport-Security does not include the includeSubDomains directive.",
		Impact: "Subdomains stay reachable over plain HTTP, and a cookie stolen there can often be replayed against the main site.",
		Fix:    "Add includeSubDomains once every subdomain is confirmed to serve HTTPS — the directive applies to all of them at once.",
	},
	"security-missing-referrer-policy": {
		What:   "The response sets no Referrer-Policy header and the page declares no meta referrer.",
		Impact: "Full URLs — including any tokens or campaign parameters in the query string — are sent to third parties in the Referer header.",
		Fix:    "Send 'Referrer-Policy: strict-origin-when-cross-origin', which keeps same-site referrers intact while trimming cross-site ones to the origin.",
	},
	"security-missing-frame-protection": {
		What:   "The response has neither an X-Frame-Options header nor a CSP frame-ancestors directive.",
		Impact: "Any site can embed these pages in an iframe, enabling clickjacking against forms and authenticated actions.",
		Fix:    "Send \"Content-Security-Policy: frame-ancestors 'self'\" (the modern form), optionally with X-Frame-Options: SAMEORIGIN for older browsers.",
	},
	"security-version-disclosure": {
		What:   "A response header publishes the exact version of the server software or framework.",
		Impact: "It hands an attacker a precise CVE shortlist for the stack without them having to probe for it.",
		Fix:    "Suppress the version in the header (nginx 'server_tokens off', Apache 'ServerTokens Prod', or remove X-Powered-By) — the software name alone is harmless.",
	},

	// --- seo: on-page technical SEO ---
	"seo-missing-title": {
		What:   "The page has no <title> element.",
		Impact: "Title is a primary ranking and SERP-display signal; its absence severely hurts visibility.",
		Fix:    "Add a unique, descriptive <title> (roughly 50–60 characters).",
	},
	"seo-short-title": {
		What:   "The <title> is very short.",
		Impact: "A too-short title likely under-describes the page and wastes SERP space.",
		Fix:    "Expand the title to clearly describe the page using relevant keywords.",
	},
	"seo-long-title": {
		What:   "The <title> may be truncated in search results.",
		Impact: "Truncated titles lose meaning and can lower click-through.",
		Fix:    "Trim the title to roughly 50–60 characters, front-loading the important words.",
	},
	"seo-missing-meta-description": {
		What:   "The page has no meta description.",
		Impact: "Search engines auto-generate snippet text, often less compelling, reducing CTR.",
		Fix:    "Add a unique meta description (~150–160 characters) summarising the page.",
	},
	"seo-short-meta-description": {
		What:   "The meta description is short.",
		Impact: "Under-uses the available snippet space and may under-sell the page.",
		Fix:    "Expand toward ~150–160 characters with a compelling, accurate summary.",
	},
	"seo-long-meta-description": {
		What:   "The meta description may be truncated.",
		Impact: "The tail of the description is cut off in SERPs, potentially losing the call to action.",
		Fix:    "Trim to roughly 150–160 characters, leading with the key message.",
	},
	"seo-meta-noindex": {
		What:   "The page is marked noindex via the robots meta tag.",
		Impact: "The page is excluded from search indexes. This may be intentional, or an accidental loss of visibility.",
		Fix:    "Remove the noindex directive if the page should rank; otherwise no action is needed.",
	},
	"seo-meta-nofollow": {
		What:   "The page is marked nofollow via the robots meta tag.",
		Impact: "Search engines won't follow links on the page, limiting crawl flow and link equity.",
		Fix:    "Remove the page-level nofollow unless intentionally sandboxing the page's links.",
	},
	"seo-x-robots-noindex": {
		What:   "An X-Robots-Tag HTTP header marks the page noindex.",
		Impact: "The page is excluded from search indexes via headers — easy to overlook since it's not in the HTML.",
		Fix:    "Remove noindex from the X-Robots-Tag header if the page should be indexed.",
	},
	"seo-x-robots-nofollow": {
		What:   "An X-Robots-Tag HTTP header marks the page nofollow.",
		Impact: "Links on the page won't be followed; this header-level directive is easy to miss.",
		Fix:    "Remove nofollow from the X-Robots-Tag header unless intentional.",
	},
	"seo-meta-refresh": {
		What:   "The page uses a meta-refresh redirect.",
		Impact: "Meta refreshes are slower, hurt UX/accessibility, and pass signals less reliably than HTTP redirects.",
		Fix:    "Replace with a server-side HTTP 301/302 redirect.",
	},
	"seo-multiple-canonical": {
		What:   "The page declares more than one canonical link.",
		Impact: "Conflicting canonicals confuse search engines, which may ignore them entirely.",
		Fix:    "Keep exactly one rel=canonical pointing to the preferred URL.",
	},
	"seo-missing-canonical": {
		What:   "The page has no canonical link.",
		Impact: "Without a canonical, duplicate/parameterised variants can compete and split signals.",
		Fix:    "Add a self-referencing rel=canonical (or point to the preferred variant).",
	},
	"seo-missing-h1": {
		What:   "The page has no <h1> element.",
		Impact: "The primary heading reinforces topic relevance for users and search engines.",
		Fix:    "Add a single, descriptive <h1> that states the page's main topic.",
	},
	"seo-multiple-h1": {
		What:   "The page has multiple <h1> elements.",
		Impact: "Generally tolerated by modern search engines but can dilute heading clarity.",
		Fix:    "Prefer one <h1> per page and use <h2>–<h6> for the heading hierarchy.",
	},
	"seo-skipped-heading-level": {
		What:   "The heading hierarchy skips a level (e.g. an <h1> followed directly by an <h3>).",
		Impact: "Breaks the logical outline of the page, hurting accessibility (screen readers rely on heading order) and making content structure harder to parse.",
		Fix:    "Use headings in strict descending order without skipping levels (h1 → h2 → h3 ...).",
	},
	"seo-empty-heading": {
		What:   "A heading element (<h1>–<h6>) contains no text.",
		Impact: "Empty headings provide no topical signal and confuse screen readers and search engines relying on the heading outline.",
		Fix:    "Remove the empty heading or give it descriptive text.",
	},
	"seo-missing-lang": {
		What:   "The <html> element has no lang attribute.",
		Impact: "Hurts accessibility (screen-reader pronunciation) and language targeting.",
		Fix:    "Set the document language, e.g. <html lang=\"en\">.",
	},
	"seo-missing-viewport": {
		What:   "The page has no viewport meta tag.",
		Impact: "The page won't be mobile-friendly, hurting mobile UX and mobile-first ranking.",
		Fix:    "Add <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">.",
	},
	"seo-missing-charset": {
		What:   "The page declares no character set.",
		Impact: "Browsers must guess the encoding, risking garbled text (mojibake).",
		Fix:    "Add <meta charset=\"utf-8\"> early in the <head>.",
	},
	"seo-missing-opengraph": {
		What:   "The page has no OpenGraph tags.",
		Impact: "Shared links on social platforms get poor or no preview cards, reducing engagement.",
		Fix:    "Add og:title, og:description, og:image, and og:url (plus Twitter Card tags) to the <head>.",
	},

	// --- sitemap: sitemap.xml ---
	"sitemap-invalid": {
		What:   "A sitemap could not be parsed as a urlset or sitemap index.",
		Impact: "Search engines can't use a malformed sitemap, undermining discovery of your URLs.",
		Fix:    "Validate the sitemap XML structure against the sitemaps.org schema and fix errors.",
	},
	"sitemap-missing": {
		What:   "No sitemap was found at the robots.txt declaration or conventional locations.",
		Impact: "Search engines must rely solely on link discovery, which can miss pages.",
		Fix:    "Publish a sitemap.xml and reference it from robots.txt.",
	},
	"sitemap-truncated": {
		What:   "A declared sitemap is larger than the crawler's fetch size limit, so it was cut off before it could be parsed.",
		Impact: "The sitemap's URLs weren't read at all, so the coverage cross-check is incomplete or skipped — this is not the same as an invalid/malformed sitemap.",
		Fix:    "Raise crawl.max_body_bytes (or GOCRAWL_CRAWL_MAX_BODY_BYTES) enough to cover the sitemap and re-crawl, or split the sitemap into smaller files per the sitemaps.org 50MB/50,000-URL limits.",
	},
	"sitemap-coverage": {
		What:   "A comparison of sitemap URLs against crawled pages (gaps in both directions).",
		Impact: "Informational. Reveals pages in the sitemap but not crawled, and crawled pages absent from the sitemap.",
		Fix:    "No action needed; review gaps to keep the sitemap aligned with the live site.",
	},

	// --- structured: JSON-LD ---
	"structured-breadcrumb-candidate": {
		What:   "Pages render breadcrumb navigation but carry no BreadcrumbList structured data. The trail is theme chrome, so this is reported once for the site, with the number of affected pages, up to five examples, and the largest number of breadcrumb links seen.",
		Impact: "You miss eligibility for the breadcrumb rich result in search, which improves click-through and clarifies page hierarchy.",
		Fix:    "Add BreadcrumbList JSON-LD whose itemListElement mirrors the visible breadcrumb trail.",
	},
	"structured-product-candidate": {
		What:   "The page reads like a product page (a price co-located with an add-to-cart/buy control, in the same form or a small enclosing container, not just anywhere on the page) but has no Product structured data.",
		Impact: "You miss eligibility for product rich results (price, availability, reviews) in search.",
		Fix:    "Add Product (with a nested Offer) JSON-LD describing the item, price, and availability.",
	},
	"structured-article-candidate": {
		What:   "The page reads like an article (substantial <article> copy with an author or publish date) but has no Article/NewsArticle/BlogPosting structured data.",
		Impact: "You miss eligibility for article rich results and give search/AI engines fewer explicit signals about authorship and publish date.",
		Fix:    "Add Article, NewsArticle, or BlogPosting JSON-LD with headline, author, and date fields.",
	},
	"structured-video-candidate": {
		What:   "The page embeds a video (native <video> or a YouTube/Vimeo iframe) but has no VideoObject structured data.",
		Impact: "You miss eligibility for video rich results and video search surfaces.",
		Fix:    "Add VideoObject JSON-LD with name, description, thumbnailUrl, and uploadDate for the embedded video.",
	},
	"structured-invalid-jsonld": {
		What:   "A JSON-LD block is not valid JSON.",
		Impact: "Malformed structured data is ignored, forfeiting rich-result eligibility.",
		Fix:    "Fix the JSON syntax so the block parses; validate with a structured-data testing tool.",
	},
	"structured-missing-required": {
		What:   "A structured-data object is missing required schema.org fields.",
		Impact: "Incomplete markup is ineligible for the corresponding rich results.",
		Fix:    "Add the required properties for the schema type, per the required tier documented for that type (per schema.org / Google's documentation).",
	},
	"structured-missing-recommended": {
		What:   "Structured data of this type omits fields Google recommends for its rich result, across the pages listed.",
		Impact: "The page stays eligible for the rich result but renders a plainer one — no ratings, no author, no imagery — so it wins fewer clicks than a fully described competitor.",
		Fix:    "Add the listed properties to the template that emits this type. Because the gap repeats site-wide, one template edit fixes every affected page.",
	},
	"structured-missing-merchant": {
		What:   "Product or ProductGroup markup omits the fields Google Shopping and free product listings read: a product identifier, price validity, shipping, return policy, and item condition. For a ProductGroup, these are also satisfied if every variant's Offer carries them.",
		Impact: "Products are ineligible for, or downranked in, Shopping and free listing surfaces, and shoppers see no shipping, returns, or condition detail before clicking.",
		Fix:    "Emit gtin (or mpn) on the Product (for a ProductGroup, on each variant), and priceValidUntil, shippingDetails, hasMerchantReturnPolicy and itemCondition on the offer. Most of the offer fields can be templated once from store-level shipping and return settings. When the identifiers already sit on the Offer, structured-identifier-on-offer is reported instead of the identifier gap.",
	},
	"structured-identifier-on-offer": {
		What:   "Product or ProductGroup markup carries no gtin or mpn of its own, but its offers do (for a ProductGroup, also its variants' offers). The fields listed are the identifier properties found on offers, such as offers.gtin12. These pages are not also reported as missing an identifier under structured-missing-merchant.",
		Impact: "Google's merchant-listing documentation places gtin and mpn on the Product and does not document reading them from an Offer, so the identifiers the store already has may not be matched to the product in Shopping and free listings.",
		Fix:    "Move the identifiers onto the Product, one per variant. On Shopify that means modelling variants as a ProductGroup with hasVariant, each variant a Product carrying its own gtin taken from the variant's Barcode field in the Shopify admin (variant.barcode in Liquid), with its Offer beneath it.",
	},
	"structured-variant-incomplete": {
		What:   "A ProductGroup declares variants inline under hasVariant, but those variants omit fields Google requires on each one: name, image, price and currency, a sku or GTIN, and the attribute the group varies by (size, color, ...). Variants listed only by url are references to other pages and are not checked.",
		Impact: "Google cannot match the incomplete variants to a distinct purchasable item, so they are dropped from variant-aware Shopping and product results, and the group shows fewer options than the store sells.",
		Fix:    "Emit every listed field on each hasVariant entry. On Shopify these come from variant data the theme already has: variant.title, variant.featured_image, variant.price, variant.sku or variant.barcode, and the option values. Brand, description and ratings may stay on the ProductGroup.",
	},
	"structured-duplicate-type": {
		What:   "A page-level type (Product, Organization, WebSite, ...) is declared in more than one JSON-LD block, usually because a theme and an SEO/marketing app each emit their own copy.",
		Impact: "Search engines pick one declaration and ignore the rest; which one is not up to the site, and the ignored copy's fields are wasted.",
		Fix:    "Consolidate to a single JSON-LD source for the type, or make the duplicate blocks agree exactly.",
	},
	"structured-conflicting-value": {
		What:   "Duplicate declarations of the same type disagree on a key value (name, SKU, price, currency, or availability).",
		Impact: "One of the two blocks is simply wrong; a price that contradicts the page can trigger a manual action from Google's Merchant Center.",
		Fix:    "Correct the source that is out of date, or remove the redundant block so only the accurate one remains.",
	},
	"structured-unresolved-id": {
		What:   "A JSON-LD {\"@id\": ...} reference points at a node that is not declared anywhere on the page.",
		Impact: "The reference silently drops whatever it was meant to convey (a publisher, a brand, a parent product); engines see the property as absent.",
		Fix:    "Declare the referenced node on the page, or replace the reference with the inline object.",
	},
	"structured-relative-url": {
		What:   "A structured-data URL property holds a relative path rather than an absolute URL.",
		Impact: "Structured data is consumed outside the page's context, so a relative path resolves against nothing and the image or link is discarded.",
		Fix:    "Emit absolute URLs (including scheme and host) for url, image, logo, thumbnailUrl, contentUrl, embedUrl and sameAs.",
	},
	"structured-empty-url": {
		What:   "Structured data declares a URL property (url, image, logo, thumbnailUrl, contentUrl, embedUrl or sameAs) with an empty or whitespace-only string. The blanks usually come from the theme and repeat on every page, so this is reported once per type for the site, with the properties affected, the number of pages, up to five examples, and the largest number of empty entries seen in one property on one page.",
		Impact: "An empty string is not a URL: the entry tells search engines nothing, and Google's Rich Results Test flags it as an invalid URL. No rich-result eligibility impact is documented, so this is housekeeping rather than a blocker.",
		Fix:    "Fill in the missing values or stop emitting blank ones. On Shopify, empty sameAs entries come from the theme's social-link settings (Online Store → Themes → Customize → Theme settings → Social media): fill them in, or change the snippet that builds the JSON-LD to skip blanks, e.g. wrap each entry in {% if settings.social_x_link != blank %}.",
	},
	"structured-invalid-date": {
		What:   "A structured-data date property is not in ISO 8601 format.",
		Impact: "An unparseable date is ignored, costing whatever it signalled — article freshness, event timing, or an offer's expiry.",
		Fix:    "Format dates as YYYY-MM-DD or a full ISO 8601 timestamp such as 2026-09-14T08:30:00+02:00.",
	},
	"structured-malformed-price": {
		What:   "A price property carries a currency symbol, a thousands separator, or a range instead of a bare decimal number.",
		Impact: "The price fails to parse, which makes the offer invalid and removes the product from price-bearing rich results.",
		Fix:    "Write the price as digits with an optional decimal point (19.99, not $1,299.00) and put the currency in priceCurrency.",
	},
	"structured-price-mismatch": {
		What:   "The price in Product structured data differs from the price rendered on the page.",
		Impact: "Markup that contradicts visible content violates Google's structured-data guidelines and risks a manual action suppressing every rich result on the site.",
		Fix:    "Generate the markup price from the same data that renders the visible price, so discounts and currency changes cannot drift apart.",
	},
	"structured-none": {
		What:   "The page has no JSON-LD structured data.",
		Impact: "The page is ineligible for rich results and gives engines fewer explicit entity signals.",
		Fix:    "Add relevant JSON-LD (e.g. Article, Product, Organization, BreadcrumbList) where appropriate.",
	},
	"structured-data": {
		What:   "Valid JSON-LD structured data was found, with its declared types, including types nested inside other objects (an Offer inside a Product, an Author inside an Article).",
		Impact: "Positive signal. Enables rich results and clearer entity understanding.",
		Fix:    "No action needed. Keep the markup accurate and aligned with visible content.",
	},

	// --- tracking: analytics & marketing tags ---
	"tracking-none": {
		What:   "No analytics or marketing tags were detected in the static HTML.",
		Impact: "Either the page is untracked, or tags load via a tag manager and aren't visible in static HTML.",
		Fix:    "Confirm tracking is intentional; if expected, verify tags fire (e.g. via a tag manager or browser tools).",
	},
	"tracking-duplicate-tag": {
		What:   "The same tag is installed more than once on the page.",
		Impact: "Duplicate tags can double-count traffic/conversions and distort analytics.",
		Fix:    "Remove the redundant install so each tag loads exactly once.",
	},
	"tracking-tags": {
		What:   "Analytics/marketing tags were detected, with their IDs.",
		Impact: "Informational. Documents which tracking is present on the page.",
		Fix:    "No action needed. Verify the detected IDs are the intended ones.",
	},
	"tracking-mixed-ga-versions": {
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
