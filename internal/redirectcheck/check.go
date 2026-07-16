package redirectcheck

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
)

// CheckRule fetches rule's Original and Target URLs against the live site and returns the
// resulting verdict. sitemapURLs is the set discovered by DiscoverSitemap. Callers should only
// call this for rules classified ScopeInScope.
func CheckRule(ctx context.Context, fetcher crawler.Fetcher, domain string, rule Rule, sitemapURLs map[string]bool) RowResult {
	res := RowResult{Scope: ScopeInScope}
	originalURL := resolveURL(rule.Original, domain)
	targetURL := resolveURL(rule.Target, domain)

	sourcePage, sourceErr := fetcher.Fetch(ctx, originalURL)
	if sourceErr != nil || sourcePage == nil || sourcePage.Err != "" {
		res.Verdict = VerdictError
		res.Notes = append(res.Notes, "source fetch failed: "+fetchErrString(sourcePage, sourceErr))
		return res
	}

	res.SourceStatus = sourcePage.StatusCode
	res.SourceFinalURL = sourcePage.FinalURL

	ignoreProtocol := strings.EqualFold(rule.IgnoreProtocol, "TRUE")
	ignoreTrailingSlash := strings.EqualFold(rule.IgnoreTrailingSlash, "TRUE")
	disableIfExists := strings.EqualFold(rule.DisableIfPageExists, "TRUE")

	res.SourceMatchesTarget = urlsMatch(sourcePage.FinalURL, targetURL, ignoreProtocol, ignoreTrailingSlash)

	switch {
	case sourcePage.StatusCode >= 400:
		res.Verdict = escalate(res.Verdict, VerdictError)
		res.Notes = append(res.Notes, fmt.Sprintf("source URL returns %d", sourcePage.StatusCode))
	case len(sourcePage.Redirects) == 0:
		if disableIfExists {
			res.Verdict = escalate(res.Verdict, VerdictWarning)
			res.Notes = append(res.Notes, "redirect suppressed by config — source page still exists, consider removing rule")
		} else {
			res.Verdict = escalate(res.Verdict, VerdictError)
			res.Notes = append(res.Notes, "rule requires unconditional redirect but source is still live")
		}
	case !res.SourceMatchesTarget:
		res.Verdict = escalate(res.Verdict, VerdictError)
		res.Notes = append(res.Notes, fmt.Sprintf("redirects to unexpected destination %q", sourcePage.FinalURL))
	}

	targetPage, targetErr := fetcher.Fetch(ctx, targetURL)
	switch {
	case targetErr != nil || targetPage == nil || targetPage.Err != "":
		res.Verdict = escalate(res.Verdict, VerdictError)
		res.Notes = append(res.Notes, "target fetch failed: "+fetchErrString(targetPage, targetErr))
	default:
		res.TargetStatus = targetPage.StatusCode
		if targetPage.StatusCode >= 400 {
			res.Verdict = escalate(res.Verdict, VerdictError)
			res.Notes = append(res.Notes, fmt.Sprintf("redirect target returns %d", targetPage.StatusCode))
		}
	}

	res.OriginalInSitemap = sitemapURLs[normalizeForSitemap(originalURL)]
	res.TargetInSitemap = sitemapURLs[normalizeForSitemap(targetURL)]

	if res.OriginalInSitemap {
		res.Verdict = escalate(res.Verdict, VerdictWarning)
		res.Notes = append(res.Notes, "stale sitemap entry — source URL still listed as canonical")
	}
	if !res.TargetInSitemap {
		res.Verdict = escalate(res.Verdict, VerdictWarning)
		res.Notes = append(res.Notes, "target not confirmed in sitemap")
	}

	if res.Verdict == "" {
		res.Verdict = VerdictOK
	}
	return res
}

func fetchErrString(page *crawler.Page, err error) string {
	if page != nil && page.Err != "" {
		return page.Err
	}
	if err != nil {
		return err.Error()
	}
	return "unknown error"
}

// urlsMatch compares two absolute URLs for equality, treating scheme/trailing-slash
// differences as insignificant when the corresponding rule flags request it.
func urlsMatch(a, b string, ignoreProtocol, ignoreTrailingSlash bool) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	if errA != nil || errB != nil {
		return a == b
	}
	hostA, hostB := strings.ToLower(ua.Host), strings.ToLower(ub.Host)
	pathA, pathB := ua.Path, ub.Path
	if ignoreTrailingSlash {
		pathA = strings.TrimRight(pathA, "/")
		pathB = strings.TrimRight(pathB, "/")
	}
	if hostA != hostB || pathA != pathB {
		return false
	}
	if !ignoreProtocol && !strings.EqualFold(ua.Scheme, ub.Scheme) {
		return false
	}
	return true
}
