package cli

import (
	"errors"
	"flag"
	"strings"
	"time"

	"go_scrap/internal/app"
	"go_scrap/internal/config"
	"go_scrap/internal/fetch"
)

type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "error"
}

func ParseArgs(args []string) (app.Options, bool, error) {
	parsed, err := parseFlags(args)
	if err != nil {
		return app.Options{}, false, ExitError{Code: 2, Err: err}
	}
	if parsed.initConfig {
		return app.Options{}, true, nil
	}

	if parsed.stdout.Value {
		parsed.yes = true
	}

	cfg, err := loadConfig(parsed.configStr)
	if err != nil {
		return app.Options{}, false, err
	}

	applyConfigDefaults(&parsed, cfg)
	return buildOptions(parsed)
}

type parsedFlags struct {
	urlStr             string
	configStr          string
	initConfig         bool
	dryRun             bool
	modeStr            stringFlag
	outputDir          stringFlag
	timeout            intFlag
	userAgent          stringFlag
	waitFor            stringFlag
	headless           boolFlag
	rateLimit          floatFlag
	yes                bool
	strict             bool
	navSel             stringFlag
	contentSel         stringFlag
	navWalk            bool
	stdout             boolFlag
	excludeSel         stringFlag
	maxSections        int
	maxMenuItems       int
	useCache           bool
	downloadAssetsFlag bool
}

func parseFlags(args []string) (parsedFlags, error) {
	fs := flag.NewFlagSet("go_scrap", flag.ContinueOnError)
	parsed := parsedFlags{}

	fs.StringVar(&parsed.urlStr, "url", "", "Target URL to scrape")
	fs.StringVar(&parsed.configStr, "config", "", "Path to JSON config file")
	fs.BoolVar(&parsed.initConfig, "init-config", false, "Interactive config wizard")
	fs.BoolVar(&parsed.dryRun, "dry-run", false, "Fetch and analyze only; do not write outputs")
	parsed.modeStr.Value = "auto"
	fs.Var(&parsed.modeStr, "mode", "Fetch mode: auto|static|dynamic")
	fs.Var(&parsed.outputDir, "output-dir", "Output directory (default: output/<host>)")
	parsed.timeout.Value = 45
	fs.Var(&parsed.timeout, "timeout", "Timeout seconds")
	fs.Var(&parsed.userAgent, "user-agent", "User-Agent header")
	fs.Var(&parsed.waitFor, "wait-for", "CSS selector to wait for (dynamic mode)")
	parsed.headless.Value = true
	fs.Var(&parsed.headless, "headless", "Run browser headless (dynamic mode)")
	parsed.rateLimit.Value = 0
	fs.Var(&parsed.rateLimit, "rate-limit", "Requests per second (0 = off)")
	fs.BoolVar(&parsed.yes, "yes", false, "Skip confirmation prompt")
	fs.BoolVar(&parsed.strict, "strict", false, "Fail if completeness checks report issues")
	fs.Var(&parsed.navSel, "nav-selector", "CSS selector for left menu/navigation")
	fs.Var(&parsed.contentSel, "content-selector", "CSS selector for main content container")
	fs.BoolVar(&parsed.navWalk, "nav-walk", false, "Click each menu anchor and capture content")
	fs.Var(&parsed.stdout, "stdout", "Print Markdown to stdout (implies --yes, suppresses logs)")
	fs.Var(&parsed.excludeSel, "exclude-selector", "CSS selector to remove from HTML before processing")
	fs.IntVar(&parsed.maxSections, "max-sections", 0, "Limit number of sections written (0 = all)")
	fs.IntVar(&parsed.maxMenuItems, "max-menu-items", 0, "Limit number of menu-based section files written (0 = all)")
	fs.BoolVar(&parsed.useCache, "cache", false, "Use disk cache for HTML content")
	fs.BoolVar(&parsed.downloadAssetsFlag, "download-assets", false, "Download referenced images to local assets directory")

	if err := fs.Parse(args); err != nil {
		return parsed, err
	}

	return parsed, nil
}

func loadConfig(path string) (config.Config, error) {
	if path == "" {
		return config.Config{}, nil
	}
	return config.Load(path)
}

func applyConfigDefaults(parsed *parsedFlags, cfg config.Config) {
	applyURL(parsed, cfg)
	applyMode(parsed, cfg)
	applyOutputDir(parsed, cfg)
	applyTimeout(parsed, cfg)
	applyUserAgent(parsed, cfg)
	applyWaitFor(parsed, cfg)
	applyHeadless(parsed, cfg)
	applyNavSelector(parsed, cfg)
	applyContentSelector(parsed, cfg)
	applyNavWalk(parsed, cfg)
	applyRateLimit(parsed, cfg)
	applyExcludeSelector(parsed, cfg)
}

func applyURL(parsed *parsedFlags, cfg config.Config) {
	if parsed.urlStr == "" && cfg.URL != "" {
		parsed.urlStr = cfg.URL
	}
}

func applyMode(parsed *parsedFlags, cfg config.Config) {
	if !parsed.modeStr.WasSet && cfg.Mode != "" {
		parsed.modeStr.Value = cfg.Mode
	}
}

func applyOutputDir(parsed *parsedFlags, cfg config.Config) {
	if !parsed.outputDir.WasSet && cfg.OutputDir != "" {
		parsed.outputDir.Value = cfg.OutputDir
	}
}

func applyTimeout(parsed *parsedFlags, cfg config.Config) {
	if !parsed.timeout.WasSet && cfg.TimeoutSeconds > 0 {
		parsed.timeout.Value = cfg.TimeoutSeconds
	}
}

func applyUserAgent(parsed *parsedFlags, cfg config.Config) {
	if !parsed.userAgent.WasSet && cfg.UserAgent != "" {
		parsed.userAgent.Value = cfg.UserAgent
	}
}

func applyWaitFor(parsed *parsedFlags, cfg config.Config) {
	if !parsed.waitFor.WasSet && cfg.WaitForSelector != "" {
		parsed.waitFor.Value = cfg.WaitForSelector
	}
}

func applyHeadless(parsed *parsedFlags, cfg config.Config) {
	if !parsed.headless.WasSet && cfg.Headless != nil {
		parsed.headless.Value = *cfg.Headless
	}
}

func applyNavSelector(parsed *parsedFlags, cfg config.Config) {
	if !parsed.navSel.WasSet && cfg.NavSelector != "" {
		parsed.navSel.Value = cfg.NavSelector
	}
}

func applyContentSelector(parsed *parsedFlags, cfg config.Config) {
	if !parsed.contentSel.WasSet && cfg.ContentSelector != "" {
		parsed.contentSel.Value = cfg.ContentSelector
	}
}

func applyNavWalk(parsed *parsedFlags, cfg config.Config) {
	if !parsed.navWalk && cfg.NavWalk {
		parsed.navWalk = true
	}
}

func applyRateLimit(parsed *parsedFlags, cfg config.Config) {
	if !parsed.rateLimit.WasSet && cfg.RateLimitPerSecond > 0 {
		parsed.rateLimit.Value = cfg.RateLimitPerSecond
	}
}

func applyExcludeSelector(parsed *parsedFlags, cfg config.Config) {
	if !parsed.excludeSel.WasSet && cfg.ExcludeSelector != "" {
		parsed.excludeSel.Value = cfg.ExcludeSelector
	}
}

func buildOptions(parsed parsedFlags) (app.Options, bool, error) {
	if parsed.urlStr == "" {
		return app.Options{}, false, ExitError{Code: 2, Err: errors.New("--url is required")}
	}
	opts := app.Options{
		URL:                parsed.urlStr,
		Mode:               fetch.Mode(strings.ToLower(strings.TrimSpace(parsed.modeStr.Value))),
		OutputDir:          parsed.outputDir.Value,
		Timeout:            time.Duration(parsed.timeout.Value) * time.Second,
		UserAgent:          parsed.userAgent.Value,
		WaitFor:            parsed.waitFor.Value,
		Headless:           parsed.headless.Value,
		RateLimitPerSecond: parsed.rateLimit.Value,
		Yes:                parsed.yes,
		Strict:             parsed.strict,
		DryRun:             parsed.dryRun,
		Stdout:             parsed.stdout.Value,
		UseCache:           parsed.useCache,
		DownloadAssets:     parsed.downloadAssetsFlag,
		NavSelector:        parsed.navSel.Value,
		ContentSelector:    parsed.contentSel.Value,
		ExcludeSelector:    parsed.excludeSel.Value,
		NavWalk:            parsed.navWalk,
		MaxSections:        parsed.maxSections,
		MaxMenuItems:       parsed.maxMenuItems,
	}
	return opts, false, nil
}
