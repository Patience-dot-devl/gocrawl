// Package botwall detects when a crawl was served a CAPTCHA or bot-protection challenge
// instead of the real page — reCAPTCHA, hCaptcha, Cloudflare Turnstile, and the full-page
// interstitials of Cloudflare, DataDome, AWS WAF, PerimeterX/HUMAN, and Imperva Incapsula.
//
// This matters because a challenge page looks like a successful (often HTTP 200) fetch, so
// every downstream analyzer then audits the challenge HTML — producing a page's worth of
// misleading "missing title / thin content / no structured data" findings. Surfacing the
// block tells the reader those findings are artefacts of being blocked, not real problems.
//
// It is a pure analyzer: it reads each page's status, response headers, body, title, and (in
// headless mode) the outbound request URLs the renderer captured, and emits Issues. It never
// fetches or solves anything.
package botwall

import (
	"context"
	"sort"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/analyze"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// Analyzer flags pages that are CAPTCHA / bot-protection challenge walls.
type Analyzer struct{}

// New returns a botwall analyzer.
func New() *Analyzer { return &Analyzer{} }

func (Analyzer) Name() string { return "botwall" }
func (Analyzer) Description() string {
	return "Detects CAPTCHA / bot-protection challenges (reCAPTCHA, Turnstile, hCaptcha, Cloudflare, DataDome, AWS WAF, Imperva) blocking the crawl"
}

// signature identifies one anti-bot vendor by substrings that appear in the page, its headers,
// or its outbound requests.
type signature struct {
	provider string
	markers  []string
	// interstitial markers appear only on a vendor's challenge/block page, so a match alone is
	// a confident "we were blocked". Non-interstitial markers identify a CAPTCHA widget that a
	// site may embed legitimately (e.g. on a contact form), so they need corroboration — a
	// blocking status, a challenge-style title, or a page with almost no other content.
	interstitial bool
}

var signatures = []signature{
	{"Cloudflare", []string{"/cdn-cgi/challenge-platform/", "cf-browser-verification", "cf_chl_opt", "_cf_chl_", "window._cf_chl", "cf-mitigated"}, true},
	{"AWS WAF", []string{"captcha.awswaf.com", "token.awswaf.com", "awswafintegration", "aws-waf-token"}, true},
	{"DataDome", []string{"geo.captcha-delivery.com", "captcha-delivery.com", "datadome-captcha"}, true},
	{"PerimeterX / HUMAN", []string{"px-captcha", "_pxcaptcha", "captcha.px-cdn.net"}, true},
	{"Imperva Incapsula", []string{"_incapsula_resource", "incapsula incident", "subject=incident"}, true},
	{"Cloudflare Turnstile", []string{"challenges.cloudflare.com/turnstile", "cf-turnstile", "turnstile/v0/"}, false},
	{"Google reCAPTCHA", []string{"g-recaptcha", "google.com/recaptcha/", "gstatic.com/recaptcha/", "recaptcha/api.js", "grecaptcha.execute"}, false},
	{"hCaptcha", []string{"h-captcha", "hcaptcha.com/1/", "js.hcaptcha.com"}, false},
}

// titleHints are page-title phrases typical of a challenge interstitial. They corroborate a
// CAPTCHA-widget match and, with a blocking status, can stand in for an unrecognised vendor.
var titleHints = []string{
	"just a moment", "attention required", "are you a robot", "are you human",
	"verify you are human", "access denied", "security check", "checking your browser",
	"one more step", "please verify",
}

// thinTextLimit is the visible-text length below which a page is "almost all challenge" — a
// CAPTCHA widget on such a page is the page, not an embedded form on real content.
const thinTextLimit = 1200

func (a Analyzer) Analyze(_ context.Context, result *crawler.Result) []analyze.Issue {
	return analyze.EachPage(result, func(p *crawler.Page) []analyze.Issue {
		hay := haystack(p)
		if hay == "" {
			return nil
		}
		title, textLen := pageText(p)

		var providers, signals []string
		widgetOnly := true
		for _, sig := range signatures {
			for _, m := range sig.markers {
				if strings.Contains(hay, m) {
					signals = append(signals, m)
					providers = appendUnique(providers, sig.provider)
					if sig.interstitial {
						widgetOnly = false
					}
					break
				}
			}
		}

		blocking := isBlockingStatus(p.StatusCode)
		titleChallenge := containsAny(title, titleHints)

		switch {
		case len(providers) == 0:
			// No known vendor. A challenge-style title on a blocking response is still a clear
			// "we were blocked" — flag it as an unrecognised challenge so it isn't silent.
			if blocking && titleChallenge {
				return []analyze.Issue{challenge(p, "Unknown", []string{"title: " + strings.TrimSpace(title)})}
			}
			return nil
		case !widgetOnly:
			// An interstitial vendor marker matched: confident block.
			return []analyze.Issue{challenge(p, primary(providers), signals)}
		case blocking || titleChallenge || textLen < thinTextLimit:
			// Only CAPTCHA-widget markers matched, but the page looks like a wall, not a form.
			return []analyze.Issue{challenge(p, primary(providers), signals)}
		default:
			// A CAPTCHA widget embedded in an otherwise-real page (e.g. a contact form). Not a
			// block; report at info so it's visible without alarming.
			return []analyze.Issue{{
				Analyzer: "botwall", URL: p.FinalURL, Severity: analyze.Info,
				Code: "captcha-widget", Message: "Page embeds a CAPTCHA widget (" + primary(providers) + ")",
				Data: map[string]any{"provider": primary(providers), "providers": providers, "signals": signals},
			}}
		}
	})
}

// challenge builds the warning Issue for a detected bot-protection wall.
func challenge(p *crawler.Page, provider string, signals []string) analyze.Issue {
	return analyze.Issue{
		Analyzer: "botwall", URL: p.FinalURL, Severity: analyze.Warning,
		Code:    "bot-challenge",
		Message: "Bot-protection challenge served (" + provider + "); the crawler was likely blocked, so this page's content is not the real page",
		Data:    map[string]any{"provider": provider, "signals": signals, "status": p.StatusCode},
	}
}

// haystack assembles a single lowercased string to scan: the response body, every response
// header key/value (catches cf-mitigated, x-datadome, challenge cookies, …), and any outbound
// request URLs captured during headless rendering (catches challenge scripts even when the DOM
// is a shell).
func haystack(p *crawler.Page) string {
	var b strings.Builder
	b.Write(p.Body)
	if p.Header != nil {
		for k, vs := range p.Header {
			b.WriteByte(' ')
			b.WriteString(k)
			for _, v := range vs {
				b.WriteByte(' ')
				b.WriteString(v)
			}
		}
	}
	if p.Render != nil {
		for _, r := range p.Render.Requests {
			b.WriteByte(' ')
			b.WriteString(r)
		}
	}
	return strings.ToLower(b.String())
}

// pageText returns the lowercased <title> and the length of the visible body text, used to tell
// a challenge interstitial (tiny) from a real page that merely embeds a CAPTCHA.
func pageText(p *crawler.Page) (title string, textLen int) {
	if p.Doc == nil {
		return "", len(p.Body)
	}
	title = strings.ToLower(strings.TrimSpace(p.Doc.Find("title").First().Text()))
	textLen = len(strings.TrimSpace(p.Doc.Find("body").Text()))
	return title, textLen
}

func isBlockingStatus(code int) bool {
	switch code {
	case 401, 403, 429, 503:
		return true
	}
	return false
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func appendUnique(xs []string, x string) []string {
	for _, e := range xs {
		if e == x {
			return xs
		}
	}
	return append(xs, x)
}

// primary returns the provider to name in the message: interstitial vendors are listed first in
// `signatures`, so the first matched provider is the most specific block signal.
func primary(providers []string) string {
	if len(providers) == 0 {
		return "Unknown"
	}
	if len(providers) == 1 {
		return providers[0]
	}
	rest := append([]string(nil), providers[1:]...)
	sort.Strings(rest)
	return providers[0] + " (+ " + strings.Join(rest, ", ") + ")"
}
