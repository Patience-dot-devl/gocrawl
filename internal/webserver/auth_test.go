package webserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Patience-dot-devl/gocrawl/internal/store"
)

const testToken = "s3cret-token"

func newTokenServer(t *testing.T) http.Handler {
	t.Helper()
	return New(store.New(t.TempDir()), AllowAnyHost(), WithAuthToken(testToken)).Handler()
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	req.Host = "203.0.113.7:8080"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuthTokenRejectsMissingOrWrongToken(t *testing.T) {
	h := newTokenServer(t)
	for name, req := range map[string]*http.Request{
		"no token":     httptest.NewRequest(http.MethodGet, "/api/analyzers", nil),
		"wrong bearer": withHeader(httptest.NewRequest(http.MethodGet, "/api/analyzers", nil), "Authorization", "Bearer nope"),
		"wrong cookie": withCookie(httptest.NewRequest(http.MethodGet, "/api/analyzers", nil), "nope"),
		"wrong query":  httptest.NewRequest(http.MethodGet, "/?token=nope", nil),
		"token on POST": httptest.NewRequest(http.MethodPost, "/api/crawls?token="+testToken,
			strings.NewReader(`{"url":"https://example.com"}`)),
	} {
		if rec := serve(h, req); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, rec.Code)
		}
	}
}

func TestAuthTokenAcceptsBearerAndCookie(t *testing.T) {
	h := newTokenServer(t)
	for name, req := range map[string]*http.Request{
		"bearer": withHeader(httptest.NewRequest(http.MethodGet, "/api/analyzers", nil), "Authorization", "Bearer "+testToken),
		"cookie": withCookie(httptest.NewRequest(http.MethodGet, "/api/analyzers", nil), testToken),
	} {
		if rec := serve(h, req); rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", name, rec.Code)
		}
	}
}

func TestAuthTokenQueryTradesForCookie(t *testing.T) {
	h := newTokenServer(t)
	rec := serve(h, httptest.NewRequest(http.MethodGet, "/crawls?view=all&token="+testToken, nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/crawls?view=all" {
		t.Errorf("Location = %q, want the same URL without the token", loc)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != AuthCookie || cookies[0].Value != testToken ||
		!cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookies = %+v, want one HttpOnly SameSite=Strict %s cookie", cookies, AuthCookie)
	}
	follow := httptest.NewRequest(http.MethodGet, "/api/analyzers", nil)
	follow.AddCookie(cookies[0])
	if rec := serve(h, follow); rec.Code != http.StatusOK {
		t.Errorf("request with the issued cookie: status = %d, want 200", rec.Code)
	}
}

func TestNoAuthTokenLeavesServerOpen(t *testing.T) {
	h := New(store.New(t.TempDir()), AllowAnyHost()).Handler()
	if rec := serve(h, httptest.NewRequest(http.MethodGet, "/api/analyzers", nil)); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 without a configured token", rec.Code)
	}
}

func withHeader(r *http.Request, k, v string) *http.Request {
	r.Header.Set(k, v)
	return r
}

func withCookie(r *http.Request, v string) *http.Request {
	r.AddCookie(&http.Cookie{Name: AuthCookie, Value: v})
	return r
}
