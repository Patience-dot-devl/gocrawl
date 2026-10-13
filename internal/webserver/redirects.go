package webserver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/redirectcheck"
)

const (
	// maxRunningRedirectChecks caps concurrent redirect-check jobs, separately from crawls.
	maxRunningRedirectChecks = 2
	// maxRedirectConcurrency bounds the concurrency field, so one request cannot open hundreds
	// of connections to the checked site.
	maxRedirectConcurrency     = 16
	defaultRedirectConcurrency = 4
)

// maxRedirectCSVBytes caps the POST /api/redirect-checks request body. It is a var so tests can
// lower it.
var maxRedirectCSVBytes int64 = 10 << 20

// newRedirectFetcher builds the fetcher a redirect-check job uses, the same one `gocrawl
// check-redirects` uses. Tests replace it to point jobs at an httptest site.
var newRedirectFetcher = func() crawler.Fetcher {
	return redirectcheck.NewFetcher(redirectcheck.DefaultTimeout, redirectcheck.DefaultUserAgent)
}

// redirectJob is the kind-specific data of a redirect-check job. Done is updated under the job
// lock while the check runs. Rows is set once, when the check finishes without an error, and is
// index-aligned with Rules.
type redirectJob struct {
	Domain   string
	FileName string
	Total    int
	Done     int
	Rules    []redirectcheck.Rule
	Rows     []redirectcheck.RowResult
}

// redirectRowView is one row of the API response: the CSV columns and the check result,
// flattened into one JSON object.
type redirectRowView struct {
	redirectcheck.Rule
	redirectcheck.RowResult
}

// redirectJobView is the API representation of a redirect-check job.
type redirectJobView struct {
	ID         string            `json:"id"`
	Domain     string            `json:"domain"`
	FileName   string            `json:"file_name,omitempty"`
	Status     string            `json:"status"`
	Err        string            `json:"error,omitempty"`
	StartedAt  string            `json:"started_at,omitempty"`
	FinishedAt string            `json:"finished_at,omitempty"`
	Done       int               `json:"done"`
	Total      int               `json:"total"`
	Rows       []redirectRowView `json:"rows,omitempty"`
}

// redirectJobViewOf builds the view from a snapshot. Rows are included only when asked for and
// only for a check that finished normally. A canceled check keeps none, because its unchecked
// rows would read as fetch failures.
func redirectJobViewOf(j Job[redirectJob], withRows bool) redirectJobView {
	v := redirectJobView{
		ID:         j.ID,
		Domain:     j.Data.Domain,
		FileName:   j.Data.FileName,
		Status:     string(j.Status),
		Err:        j.Err,
		StartedAt:  formatTime(j.StartedAt),
		FinishedAt: formatTime(j.FinishedAt),
		Done:       j.Data.Done,
		Total:      j.Data.Total,
	}
	if withRows && j.Status == StatusDone && len(j.Data.Rows) == len(j.Data.Rules) {
		v.Rows = make([]redirectRowView, len(j.Data.Rows))
		for i := range j.Data.Rows {
			v.Rows[i] = redirectRowView{Rule: j.Data.Rules[i], RowResult: j.Data.Rows[i]}
		}
	}
	return v
}

func (s *Server) redirectRoutes() {
	s.mux.HandleFunc("GET /api/redirect-checks", s.handleListRedirectChecks)
	s.mux.HandleFunc("POST /api/redirect-checks", s.handleStartRedirectCheck)
	s.mux.HandleFunc("GET /api/redirect-checks/{id}", s.handleGetRedirectCheck)
	s.mux.HandleFunc("POST /api/redirect-checks/{id}/cancel", s.handleCancelRedirectCheck)
	s.mux.HandleFunc("GET /api/redirect-checks/{id}/export", s.handleExportRedirectCheck)
}

// redirectCheckParams are the validated non-file form fields of POST /api/redirect-checks.
type redirectCheckParams struct {
	domain      string
	sitemapURL  string
	concurrency int
	rate        float64
	adaptive    bool
}

func parseRedirectCheckForm(r *http.Request) (redirectCheckParams, error) {
	p := redirectCheckParams{concurrency: defaultRedirectConcurrency, adaptive: true}
	domain, err := normalizeRedirectDomain(r.FormValue("domain"))
	if err != nil {
		return p, err
	}
	p.domain = domain
	if raw := strings.TrimSpace(r.FormValue("sitemap_url")); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return p, errors.New("sitemap_url must be an http(s) URL, for example https://example.com/sitemap.xml")
		}
		if u.User != nil {
			return p, errors.New("sitemap_url must not contain a user name or password")
		}
		p.sitemapURL = raw
	}
	if raw := strings.TrimSpace(r.FormValue("concurrency")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxRedirectConcurrency {
			return p, fmt.Errorf("concurrency must be a whole number from 1 to %d", maxRedirectConcurrency)
		}
		p.concurrency = n
	}
	if raw := strings.TrimSpace(r.FormValue("rate")); raw != "" {
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
			return p, errors.New("rate must be a number of requests per second, or 0 for unlimited")
		}
		p.rate = f
	}
	if raw := strings.TrimSpace(r.FormValue("adaptive_delay")); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return p, errors.New("adaptive_delay must be true or false")
		}
		p.adaptive = b
	}
	return p, nil
}

// errNoRedirectCheck is the 404 body for an unknown id. It does not echo the id.
var errNoRedirectCheck = errors.New("no redirect check with that id")

// errNotHostName is returned without echoing the input, which could carry URL userinfo.
var errNotHostName = errors.New("domain is not a host name; use the bare domain, for example example.com")

// normalizeRedirectDomain turns what a user types into the bare domain redirectcheck expects:
// it lowercases the input, takes the host when given a URL, and drops a trailing slash. A
// value with a path is rejected, because redirectcheck builds https://<domain>/... URLs.
func normalizeRedirectDomain(raw string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(raw))
	if d == "" {
		return "", errors.New("domain is required, for example example.com")
	}
	if strings.Contains(d, "://") {
		u, err := url.Parse(d)
		if err != nil || u.Host == "" || u.User != nil {
			return "", errNotHostName
		}
		d = u.Host
	}
	d = strings.TrimSuffix(d, "/")
	if d == "" || strings.ContainsAny(d, "/?#@ \t") {
		return "", errNotHostName
	}
	return d, nil
}

func (s *Server) handleStartRedirectCheck(w http.ResponseWriter, r *http.Request) {
	if !parseMultipart(w, r, maxRedirectCSVBytes) {
		return
	}
	// Release the spooled upload (temp files above the in-memory limit) once the CSV is parsed.
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	params, err := parseRedirectCheckForm(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New(`missing CSV file: send it as the multipart field "file"`))
		return
	}
	defer func() { _ = file.Close() }()
	// Parse up front, so a wrong export format is a 400 here and not a failed job later.
	rules, err := redirectcheck.ParseCSV(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("parsing the CSV: %w", err))
		return
	}
	if len(rules) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("the CSV has a header but no redirect rules"))
		return
	}

	fetcher := newRedirectFetcher()
	ctx, cancel := context.WithCancel(context.Background())
	job, ok := s.redirects.start(redirectJob{
		Domain:   params.domain,
		FileName: header.Filename,
		Total:    len(rules),
		Rules:    rules,
	}, cancel, maxRunningRedirectChecks)
	if !ok {
		cancel()
		writeError(w, http.StatusTooManyRequests, fmt.Errorf("%d redirect checks already running; wait for one to finish or cancel it", maxRunningRedirectChecks))
		return
	}

	go s.runRedirectCheck(ctx, cancel, job.ID, rules, redirectcheck.RunOptions{
		Domain:        params.domain,
		SitemapURL:    params.sitemapURL,
		Fetcher:       fetcher,
		Concurrency:   params.concurrency,
		RatePerSecond: params.rate,
		AdaptiveDelay: params.adaptive,
	})

	writeJSON(w, http.StatusAccepted, redirectJobViewOf(job, false))
}

// runRedirectCheck runs one check to completion on its own goroutine and records the outcome.
func (s *Server) runRedirectCheck(ctx context.Context, cancel context.CancelFunc, id string, rules []redirectcheck.Rule, opts redirectcheck.RunOptions) {
	defer cancel()
	opts.OnRow = func(done, _ int) {
		s.redirects.with(id, func(j *Job[redirectJob]) { j.Data.Done = done })
	}
	rows, err := redirectcheck.Run(ctx, rules, opts)
	if err != nil && ctx.Err() != nil {
		// A cancel during sitemap discovery surfaces as "could not fetch a sitemap". Clear it
		// so finish records the job as canceled, which is what the user asked for.
		err = nil
	}
	if err != nil && opts.SitemapURL != "" {
		// The discovery error quotes the override URL, which may carry a token in its query.
		err = errors.New(strings.ReplaceAll(err.Error(), opts.SitemapURL, "the sitemap URL"))
	}
	s.redirects.finish(id, err, func(d *redirectJob) {
		if err == nil {
			d.Rows = rows
		}
	})
}

func (s *Server) handleListRedirectChecks(w http.ResponseWriter, _ *http.Request) {
	jobs := s.redirects.list()
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].StartedAt.After(jobs[k].StartedAt) })
	views := make([]redirectJobView, 0, len(jobs))
	for _, j := range jobs {
		views = append(views, redirectJobViewOf(j, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"checks": views})
}

func (s *Server) handleGetRedirectCheck(w http.ResponseWriter, r *http.Request) {
	job, ok := s.redirects.get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errNoRedirectCheck)
		return
	}
	writeJSON(w, http.StatusOK, redirectJobViewOf(job, true))
}

func (s *Server) handleCancelRedirectCheck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.redirects.get(id); !ok {
		writeError(w, http.StatusNotFound, errNoRedirectCheck)
		return
	}
	// get and cancel are two locked calls. If the check finishes between them, cancel reports
	// false and 409 is the right answer for a check that is no longer running.
	if !s.redirects.cancel(id) {
		writeError(w, http.StatusConflict, errors.New("that redirect check is not running"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleExportRedirectCheck writes the input CSV back with the verdict columns appended, the
// same file `gocrawl check-redirects --output` writes.
func (s *Server) handleExportRedirectCheck(w http.ResponseWriter, r *http.Request) {
	job, ok := s.redirects.get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errNoRedirectCheck)
		return
	}
	if job.Status != StatusDone {
		writeError(w, http.StatusConflict, errors.New("only a finished redirect check can be exported"))
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="redirect-check-`+safeFileNamePart(job.Data.Domain)+`.csv"`)
	// The status line is already sent once WriteCSV starts writing, so a failure partway
	// through (a client that went away) has no better response to give.
	_ = redirectcheck.WriteCSV(w, job.Data.Rules, job.Data.Rows)
}

// safeFileNamePart keeps letters, digits, dot and hyphen and turns every other character
// into a hyphen, so a user-typed domain cannot break out of the quoted Content-Disposition
// filename or inject header lines.
func safeFileNamePart(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-':
			return r
		}
		return '-'
	}, s)
}
