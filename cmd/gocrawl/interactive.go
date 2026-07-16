package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/Patience-dot-devl/gocrawl/internal/config"
	"github.com/Patience-dot-devl/gocrawl/internal/crawler"
	"github.com/Patience-dot-devl/gocrawl/internal/runner"
)

// runInteractive walks the user through gocrawl's options with an interactive form, then
// runs the crawl with the resulting configuration. It is invoked when gocrawl is run with
// no subcommand on an interactive terminal.
func runInteractive(cmd *cobra.Command) error {
	// Start from the layered defaults so the form is pre-populated with sensible values.
	configPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// huh inputs are string-based; bind to locals and parse after the form completes.
	var (
		seed        = cfg.Seed
		depth       = strconv.Itoa(cfg.Crawl.MaxDepth)
		maxPages    = strconv.Itoa(cfg.Crawl.MaxPages)
		concurrency = strconv.Itoa(cfg.Crawl.Concurrency)
		rate        = strconv.FormatFloat(cfg.Crawl.RatePerSecond, 'f', -1, 64)

		respectRobots   = cfg.Crawl.RespectRobots
		allowSubdomains = cfg.Crawl.AllowSubdomains
		followExternal  = cfg.Crawl.FollowExternal
		userAgent       = cfg.Crawl.UserAgent
		keepAwake       = false

		basicAuthUser, basicAuthPass, _ = strings.Cut(cfg.Crawl.BasicAuth, ":")

		render     = cfg.Render
		format     = cfg.Output.Format
		outputPath = cfg.Output.Path

		specialized = cfg.Analyzers.Specialized
	)
	if render == "" {
		render = "raw"
	}
	if format == "" {
		format = "json"
	}
	// A --user-agent passed to the bare `gocrawl` command pre-fills the menu field, so
	// `gocrawl --user-agent endeavour-bot` opens the menu with the UA set and still prompts
	// for everything else.
	if cmd.Flags().Changed("user-agent") {
		userAgent, _ = cmd.Flags().GetString("user-agent")
	}

	// Analyzer multi-select: options come from the live registry; default to all selected.
	all := runner.ListAnalyzers()
	analyzerOpts := make([]huh.Option[string], 0, len(all))
	for _, a := range all {
		label := a.Name
		if a.Description != "" {
			label = fmt.Sprintf("%s — %s", a.Name, a.Description)
		}
		analyzerOpts = append(analyzerOpts, huh.NewOption(label, a.Name).Selected(true))
	}
	selected := make([]string, 0, len(all))

	// Scope/behavior toggles. The keep-awake toggle is only meaningful where caffeinate(8)
	// exists (macOS), so it's appended conditionally rather than shown as a dead option.
	scopeFields := []huh.Field{
		huh.NewConfirm().Title("Respect robots.txt?").Value(&respectRobots),
		huh.NewConfirm().Title("Follow links to subdomains?").Value(&allowSubdomains),
		huh.NewConfirm().Title("Crawl links that leave the seed host?").Value(&followExternal),
	}
	if caffeinateSupported() {
		scopeFields = append(scopeFields, huh.NewConfirm().
			Title("Keep this Mac awake while crawling?").
			Description("Holds a power assertion (caffeinate) so a locked screen or idle sleep doesn't pause a long crawl.").
			Value(&keepAwake))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Seed URL").
				Description("The page to start crawling from.").
				Placeholder("https://example.com").
				Value(&seed).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("seed URL is required")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewInput().Title("Max depth").Description("Link hops from the seed (0 = unlimited; the page cap bounds the crawl).").Value(&depth).Validate(validInt),
			huh.NewInput().Title("Max pages").Description("Hard cap on pages crawled (0 = unlimited).").Value(&maxPages).Validate(validInt),
			huh.NewInput().Title("Concurrency").Description("Parallel fetch workers.").Value(&concurrency).Validate(validInt),
			huh.NewInput().Title("Rate limit").Description("Max requests per second (0 = unlimited).").Value(&rate).Validate(validFloat),
		),
		// Scope toggles kept in their own group: huh renders every field of a group on one
		// screen, so an overcrowded group overflows a short terminal and clips the top fields.
		huh.NewGroup(scopeFields...),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Rendering mode").
				Description("raw = HTTP fetch (fast); headless = Chrome render (JS + Core Web Vitals).").
				Options(huh.NewOption("Raw (HTTP)", "raw"), huh.NewOption("Headless (Chrome)", "headless")).
				Value(&render),
			huh.NewInput().
				Title("User-Agent").
				Description("Sent on every request. Blank = default; set a specific UA if the site allow-lists one (e.g. a CAPTCHA exemption).").
				Value(&userAgent),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Basic Auth username").
				Description("Only needed if the site is gated by server-level HTTP Basic Auth (common on staging/acceptance environments). Leave blank otherwise.").
				Value(&basicAuthUser),
			huh.NewInput().
				Title("Basic Auth password").
				EchoMode(huh.EchoModePassword).
				Value(&basicAuthPass),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Analyzers").
				Description("Space to toggle; all run by default.").
				Options(analyzerOpts...).
				Value(&selected),
			huh.NewConfirm().
				Title("Enable specialized checks?").
				Description("Opt-in: AEO answer-lead, GEO quotable-density, and WordPress security probes.").
				Value(&specialized),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Output format").
				Options(huh.NewOption("JSON", "json"), huh.NewOption("CSV", "csv"), huh.NewOption("HTML", "html")).
				Value(&format),
			huh.NewInput().
				Title("Output file").
				Description("Leave empty to write to stdout.").
				Value(&outputPath),
		),
	)

	if err := runForm(form); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil // clean exit on Ctrl-C / Esc
		}
		return err
	}

	// Map the answers back onto the config.
	cfg.Crawl.MaxDepth = atoiOr(depth, cfg.Crawl.MaxDepth)
	cfg.Crawl.MaxPages = atoiOr(maxPages, cfg.Crawl.MaxPages)
	cfg.Crawl.Concurrency = atoiOr(concurrency, cfg.Crawl.Concurrency)
	cfg.Crawl.RatePerSecond = atofOr(rate, cfg.Crawl.RatePerSecond)
	cfg.Crawl.RespectRobots = respectRobots
	cfg.Crawl.AllowSubdomains = allowSubdomains
	cfg.Crawl.FollowExternal = followExternal
	cfg.Crawl.UserAgent = strings.TrimSpace(userAgent)
	if user := strings.TrimSpace(basicAuthUser); user != "" {
		cfg.Crawl.BasicAuth = user + ":" + basicAuthPass
	} else {
		cfg.Crawl.BasicAuth = ""
	}
	cfg.Render = render
	cfg.Output.Format = format
	cfg.Output.Path = strings.TrimSpace(outputPath)
	cfg.Analyzers.Specialized = specialized

	cfg.Analyzers.Enabled, cfg.Analyzers.Disabled = analyzerSelection(selected, all)

	seed = strings.TrimSpace(seed)
	if !strings.Contains(seed, "://") {
		seed = "https://" + seed
	}
	var seedUser, seedPass string
	seed, seedUser, seedPass = crawler.SanitizeSeed(seed)
	if seedUser != "" && cfg.Crawl.BasicAuth == "" {
		cfg.Crawl.BasicAuth = seedUser + ":" + seedPass
	}

	// Keep the machine awake for the duration of the crawl + report write when requested.
	if keepAwake {
		defer startCaffeinate()()
	}

	rep, err := runner.Run(cmd.Context(), cfg, seed)
	if err != nil {
		return err
	}
	if err := writeReport(cfg, rep); err != nil {
		return err
	}
	for _, note := range rep.Notes {
		fmt.Fprintln(os.Stderr, "note:", note)
	}
	for _, line := range rep.SummaryLines() {
		fmt.Fprintln(os.Stderr, line)
	}
	return nil
}

// analyzerSelection maps the interactive analyzer multi-select onto Enabled/Disabled. An empty
// Enabled list conventionally means "run all" (see runner/analyze.Registry.Select), so
// deselecting every analyzer can't be expressed that way — it would silently run all of them
// instead of none. Disabling every analyzer by name achieves the same "run none" result
// through the existing deny-list mechanism instead.
func analyzerSelection(selected []string, all []runner.AnalyzerInfo) (enabled, disabled []string) {
	switch {
	case len(selected) == 0:
		names := make([]string, len(all))
		for i, a := range all {
			names[i] = a.Name
		}
		return nil, names
	case len(selected) < len(all):
		return selected, nil
	default:
		return nil, nil
	}
}

// runForm runs the form, converting the panic that huh v1.0.0 raises when bubbletea cannot
// initialize a terminal (form.go:690 dereferences a nil model before checking the error)
// into a clean, actionable error instead of a raw stack trace.
func runForm(f *huh.Form) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not start the interactive menu (no usable terminal): %v\n"+
				"run a crawl directly instead, e.g. `gocrawl crawl https://example.com`", r)
		}
	}()
	return f.Run()
}

func validInt(s string) error {
	if _, err := strconv.Atoi(strings.TrimSpace(s)); err != nil {
		return errors.New("must be a whole number")
	}
	return nil
}

func validFloat(s string) error {
	if _, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err != nil {
		return errors.New("must be a number")
	}
	return nil
}

func atoiOr(s string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return fallback
}

func atofOr(s string, fallback float64) float64 {
	if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return f
	}
	return fallback
}
