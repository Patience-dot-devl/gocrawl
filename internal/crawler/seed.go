package crawler

import "net/url"

// SanitizeSeed strips any userinfo (user:pass@) embedded in a seed URL and returns it
// separately, so callers can route it through Options.BasicAuthUser/BasicAuthPass instead of
// letting it ride along in the URL string — where it would propagate into Result.Seed, every
// report format, and `gocrawl history`, and (via Go's URL-resolution semantics) into every
// link resolved against the seed once crawled. raw is expected to already have a scheme
// (callers prefix a bare host with "https://" before calling this).
func SanitizeSeed(raw string) (seed, user, pass string) {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw, "", ""
	}
	user = u.User.Username()
	pass, _ = u.User.Password()
	u.User = nil
	return u.String(), user, pass
}
