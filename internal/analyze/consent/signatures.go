package consent

import "strings"

// cmpSignature identifies one consent management platform by substrings that appear in a
// page's scripts, script sources, resource hints, or banner class/id attributes. The first
// match wins, so more specific vendors come before the generic IAB TCF fallback.
//
// Every signal must be *integration-shaped* — a hostname, a script filename, a JavaScript
// global, or a CSS class prefix — and never a bare brand name. Inline scripts routinely carry
// arbitrary JSON, including navigation menus: cookiebot.com ships a menu blob containing
// {"title":"Cookiebot vs OneTrust"}, which a bare "onetrust" signal read as a OneTrust
// install. Requiring the shape of an actual integration is what separates a deployed CMP from
// a competitor's name in a comparison link.
type cmpSignature struct {
	name    string
	signals []string // lowercased
}

// cmpSignatures covers the platforms that dominate European deployments, plus a generic
// IAB TCF fallback for the long tail — every TCF-compliant CMP exposes the __tcfapi global,
// so a site using one gocrawl has never heard of is still recognised as having *a* CMP.
var cmpSignatures = []cmpSignature{
	{"Cookiebot", []string{"consent.cookiebot.com", "cookiebot.js", "cookieconsent.min.js"}},
	{"OneTrust", []string{"cdn.cookielaw.org", "otsdkstub.js", "onetrust.js", "optanon", "onetrustactivegroups", "window.onetrust"}},
	{"Usercentrics", []string{"app.usercentrics.eu", "usercentrics.eu", "uc.usercentrics"}},
	{"CookieYes", []string{"cdn-cookieyes.com", "cookieyes.com"}},
	{"Cookie-Script", []string{"cdn.cookie-script.com", "cookiescript_", "cookiescriptreport"}},
	{"Cookie Information", []string{"policy.app.cookieinformation.com", "coiconsent"}},
	// Complianz prefixes its CSS classes with cmplz- and its JS globals with cmplz_, so match
	// the bare token rather than either separator. The plugin directory is the other tell;
	// "complianz" on its own is a brand name and would fire on a comparison page.
	{"Complianz", []string{"cmplz", "complianz-gdpr"}},
	{"Didomi", []string{"sdk.privacy-center.org", "didomi.io", "window.didomi"}},
	{"Iubenda", []string{"cdn.iubenda.com", "iubenda_cs"}},
	{"Termly", []string{"app.termly.io", "termly.io"}},
	{"Osano", []string{"cmp.osano.com", "osano.com/uhb"}},
	// Large publishers proxy Sourcepoint through their own domain, so match the hostname label
	// "sourcepoint." as well as the hosted endpoints. The trailing dot is what keeps it from
	// firing on a /sourcepoint-alternative/ link.
	{"Sourcepoint", []string{"sourcepoint.", "cdn.privacy-mgmt.com", "window._sp_"}},
	{"TrustArc", []string{"consent.trustarc.com", "trustarc.com/notice"}},
	{"Quantcast Choice", []string{"quantcast.mgr.consensu.org", "cmp.quantcast.com"}},
	{"Axeptio", []string{"axeptio.imgix.net", "static.axept.io"}},
	{"CookieFirst", []string{"consent.cookiefirst.com", "cookiefirst.com"}},
	{"Borlabs Cookie", []string{"borlabs-cookie"}},
	{"Real Cookie Banner", []string{"real-cookie-banner"}},
	{"Klaro", []string{"klaro.js", "klaro-config"}},
	{"tarteaucitron", []string{"tarteaucitron"}},
	{"Cookie Notice", []string{"cookie-notice/js", "cookie-law-info"}},
	// Generic fallbacks, checked last: any IAB TCF v2 CMP exposes __tcfapi, and Google's
	// Funding Choices / Privacy & Messaging exposes __gpp or the fundingchoices host.
	{"IAB TCF CMP (vendor not identified)", []string{"__tcfapi(", "window.__tcfapi"}},
	{"Google Funding Choices", []string{"fundingchoicesmessages.google.com"}},
}

// detectCMP returns the name of the first consent platform whose signature appears in the
// page blob, and whether one was found.
func detectCMP(lowerBlob string) (string, bool) {
	for _, sig := range cmpSignatures {
		for _, s := range sig.signals {
			if strings.Contains(lowerBlob, s) {
				return sig.name, true
			}
		}
	}
	return "", false
}

// trackerSignals mark a page as carrying analytics or advertising tags — the precondition
// for consent being required in the first place. A page with no tags needs no CMP, and
// flagging one would be noise.
var trackerSignals = []string{
	"googletagmanager.com", "google-analytics.com", "gtag(", "ga(",
	"connect.facebook.net", "fbq(", "snap.licdn.com", "_linkedin_partner_id",
	"bat.bing.com", "analytics.tiktok.com", "static.hotjar.com", "clarity.ms",
	"js.hs-scripts.com", "cdn.matomo.cloud", "script.crazyegg.com", "cdn.segment.com",
	"static.ads-twitter.com", "s.pinimg.com", "sc-static.net", "mc.yandex.ru",
	"googleadservices.com", "doubleclick.net",
}

// hasTrackers reports whether the page carries any analytics or advertising tag.
func hasTrackers(lowerBlob string) bool {
	for _, s := range trackerSignals {
		if strings.Contains(lowerBlob, s) {
			return true
		}
	}
	return false
}

// trackerCookie names a cookie that exists to profile or measure a visitor. Consent is
// required before any of these may be set in the EU, so finding one on an un-consented crawl
// is the finding this analyzer exists for.
type trackerCookie struct {
	// prefix matches names beginning with it (analytics vendors mint per-property names like
	// _ga_XXXXXXX); exact matches the whole name.
	prefix, exact string
	vendor        string
	purpose       string
}

// trackerCookies is checked in order; the first match classifies the cookie.
var trackerCookies = []trackerCookie{
	{prefix: "_ga", vendor: "Google Analytics", purpose: "analytics"},
	{prefix: "_gid", vendor: "Google Analytics", purpose: "analytics"},
	{prefix: "_gat", vendor: "Google Analytics", purpose: "analytics"},
	{prefix: "__utm", vendor: "Google Analytics (legacy)", purpose: "analytics"},
	{prefix: "_gcl_", vendor: "Google Ads", purpose: "advertising"},
	{exact: "IDE", vendor: "Google DoubleClick", purpose: "advertising"},
	{exact: "test_cookie", vendor: "Google DoubleClick", purpose: "advertising"},
	{exact: "DSID", vendor: "Google DoubleClick", purpose: "advertising"},
	{exact: "NID", vendor: "Google", purpose: "advertising"},
	{exact: "1P_JAR", vendor: "Google", purpose: "advertising"},
	{prefix: "_fbp", vendor: "Meta Pixel", purpose: "advertising"},
	{prefix: "_fbc", vendor: "Meta Pixel", purpose: "advertising"},
	{exact: "fr", vendor: "Meta", purpose: "advertising"},
	{prefix: "_uet", vendor: "Microsoft Advertising", purpose: "advertising"},
	{exact: "MUID", vendor: "Microsoft", purpose: "advertising"},
	{exact: "li_sugr", vendor: "LinkedIn Insight", purpose: "advertising"},
	{exact: "bcookie", vendor: "LinkedIn", purpose: "advertising"},
	{exact: "bscookie", vendor: "LinkedIn", purpose: "advertising"},
	{exact: "lidc", vendor: "LinkedIn", purpose: "advertising"},
	{exact: "UserMatchHistory", vendor: "LinkedIn", purpose: "advertising"},
	{exact: "AnalyticsSyncHistory", vendor: "LinkedIn", purpose: "analytics"},
	{prefix: "_ttp", vendor: "TikTok Pixel", purpose: "advertising"},
	{prefix: "_tt_", vendor: "TikTok Pixel", purpose: "advertising"},
	{prefix: "_hj", vendor: "Hotjar", purpose: "analytics"},
	{prefix: "_clck", vendor: "Microsoft Clarity", purpose: "analytics"},
	{prefix: "_clsk", vendor: "Microsoft Clarity", purpose: "analytics"},
	{prefix: "__hs", vendor: "HubSpot", purpose: "analytics"},
	{exact: "hubspotutk", vendor: "HubSpot", purpose: "analytics"},
	{prefix: "_pk_", vendor: "Matomo", purpose: "analytics"},
	{prefix: "_pin_", vendor: "Pinterest", purpose: "advertising"},
	{prefix: "_epik", vendor: "Pinterest", purpose: "advertising"},
	{prefix: "_scid", vendor: "Snapchat", purpose: "advertising"},
	{prefix: "_ym_", vendor: "Yandex Metrica", purpose: "analytics"},
	{prefix: "cto_", vendor: "Criteo", purpose: "advertising"},
	{exact: "personalization_id", vendor: "X (Twitter)", purpose: "advertising"},
	{exact: "muc_ads", vendor: "X (Twitter)", purpose: "advertising"},
	{prefix: "ajs_", vendor: "Segment", purpose: "analytics"},
	{prefix: "_vwo", vendor: "VWO", purpose: "analytics"},
	{prefix: "_omappvp", vendor: "OptinMonster", purpose: "advertising"},
	{prefix: "intercom-", vendor: "Intercom", purpose: "analytics"},
}

// classifyTracker returns the tracker entry matching a cookie name, if any.
func classifyTracker(name string) (trackerCookie, bool) {
	for _, t := range trackerCookies {
		if t.exact != "" && name == t.exact {
			return t, true
		}
		if t.prefix != "" && strings.HasPrefix(name, t.prefix) {
			return t, true
		}
	}
	return trackerCookie{}, false
}

// consentStateCookies are the cookies a consent platform uses to remember the visitor's own
// choice. They are strictly necessary by definition — the banner cannot work without them —
// so they are never reported, even though several look like tracker cookies.
var consentStateCookies = []string{
	"cookieconsent", "cookie_notice_accepted", "cookielawinfo", "cmplz_", "consentuuid",
	"optanonconsent", "optanonalertboxclosed", "oneTrust", "euconsent-v2", "addtl_consent",
	"usercentrics", "uc_settings", "cookieyes-consent", "cky-", "borlabs-cookie",
	"didomi_token", "iubenda", "termly", "osano_consentmanager", "axeptio_",
	"real_cookie_banner", "tarteaucitron", "klaro", "cookiefirst-consent", "_sp_",
}

// isConsentState reports whether a cookie is a consent platform's own record of the
// visitor's choice.
func isConsentState(lowerName string) bool {
	for _, c := range consentStateCookies {
		if strings.HasPrefix(lowerName, strings.ToLower(c)) {
			return true
		}
	}
	return false
}

// trackerRequestHosts are third-party endpoints whose sole purpose is measurement or
// advertising. A request to one during a render that never answered a consent banner is
// direct evidence that tags fired before consent — stronger than a cookie, because it can't
// be explained away as a functional necessity.
var trackerRequestHosts = []string{
	"google-analytics.com", "analytics.google.com", "googletagmanager.com/gtag",
	"stats.g.doubleclick.net", "doubleclick.net", "googleadservices.com",
	"facebook.com/tr", "connect.facebook.net/signals",
	"bat.bing.com", "analytics.tiktok.com", "px.ads.linkedin.com",
	"static.hotjar.com", "in.hotjar.com", "clarity.ms/collect",
	"track.hubspot.com", "api.segment.io", "mc.yandex.ru/watch",
	"ct.pinterest.com", "tr.snapchat.com", "analytics.twitter.com",
}

// classifyTrackerRequest returns the tracker host a request URL belongs to, if any.
func classifyTrackerRequest(lowerURL string) (string, bool) {
	for _, h := range trackerRequestHosts {
		if strings.Contains(lowerURL, h) {
			return h, true
		}
	}
	return "", false
}
