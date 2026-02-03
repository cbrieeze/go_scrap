package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"go_scrap/internal/app"
	"go_scrap/internal/config"
	"go_scrap/internal/fetch"
)

type Result struct {
	Options    app.Options
	SaveConfig bool
	ConfigPath string
	Config     config.Config
	RunNow     bool
}

func Run() (Result, error) {
	printBanner()
	state := newFormState()

	if err := manageConfigs(state); err != nil {
		return Result{}, err
	}

	form := buildForm(state).WithTheme(huh.ThemeDracula())
	if err := form.Run(); err != nil {
		return Result{}, err
	}

	return buildResult(state)
}

func printBanner() {
	fmt.Print(`
   __ _  ___   ___  ___ ___ _ __ __ _ _ __
  / _` + "`" + ` |/ _ \ / __|/ __/ __| '__/ _` + "`" + ` | '_ \
 | (_| | (_) | (__| (__\__ \ | | (_| | |_) |
  \__, |\___/ \___|\___|___/_|  \__,_| .__/
   __/ |                             | |
  |___/                              |_|
`)
}

func manageConfigs(state *formState) error {
	for {
		files, err := listConfigFiles()
		if err != nil {
			return fmt.Errorf("failed to list configs: %w", err)
		}

		if len(files) == 0 {
			return nil // No configs, just proceed
		}

		var selectedFile string
		opts := []huh.Option[string]{
			huh.NewOption("Start fresh (no config)", ""),
		}
		for _, f := range files {
			opts = append(opts, huh.NewOption(fmt.Sprintf("Manage %s", f), f))
		}

		selectForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Manage Configurations").
					Description("Select a config to load or manage, or start fresh.").
					Options(opts...).
					Value(&selectedFile),
			),
		).WithTheme(huh.ThemeDracula())

		if err := selectForm.Run(); err != nil {
			return err // User cancelled
		}

		if selectedFile == "" {
			return nil // User chose to start fresh
		}

		var action string
		actionForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("Action for %s", selectedFile)).
					Options(
						huh.NewOption("Load this config", "load"),
						huh.NewOption("Rename this config", "rename"),
						huh.NewOption("Clone this config", "clone"),
						huh.NewOption("Delete this config", "delete"),
						huh.NewOption("Back to list", "back"),
					).
					Value(&action),
			),
		).WithTheme(huh.ThemeDracula())

		if err := actionForm.Run(); err != nil {
			return err
		}

		shouldExit, err := executeConfigAction(action, selectedFile, state)
		if err != nil {
			return err
		}
		if shouldExit {
			return nil
		}
	}
}

func listConfigFiles() ([]string, error) {
	files, err := filepath.Glob("*.json")
	if err != nil {
		return nil, err
	}
	nested, err := filepath.Glob(filepath.Join("configs", "*.json"))
	if err != nil {
		return nil, err
	}
	return append(files, nested...), nil
}

func executeConfigAction(action, selectedFile string, state *formState) (bool, error) {
	switch action {
	case "load":
		data, err := os.ReadFile(selectedFile)
		if err != nil {
			return false, fmt.Errorf("failed to read %s: %w", selectedFile, err)
		}
		var cfg config.Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return false, fmt.Errorf("failed to parse %s: %w", selectedFile, err)
		}
		state.fromConfig(cfg)
		state.configPath = selectedFile
		return true, nil // Exit loop

	case "rename":
		var newName string
		if err := huh.NewInput().Title("New filename").Value(&newName).Validate(validateNewFilename).Run(); err != nil {
			return false, err
		}
		newName = ensureJSONExtension(newName)
		if err := os.Rename(selectedFile, newName); err != nil {
			return false, fmt.Errorf("failed to rename: %w", err)
		}

	case "clone":
		var newName string
		if err := huh.NewInput().Title("Clone as").Value(&newName).Validate(validateNewFilename).Run(); err != nil {
			return false, err
		}
		newName = ensureJSONExtension(newName)
		data, err := os.ReadFile(selectedFile)
		if err != nil {
			return false, fmt.Errorf("failed to read %s: %w", selectedFile, err)
		}
		if err := os.WriteFile(newName, data, 0600); err != nil {
			return false, fmt.Errorf("failed to write %s: %w", newName, err)
		}

	case "delete":
		var confirmDelete bool
		if err := huh.NewConfirm().Title(fmt.Sprintf("Really delete %s?", selectedFile)).Affirmative("Yes, delete it.").Negative("No, keep it.").Value(&confirmDelete).Run(); err != nil {
			return false, err
		}
		if confirmDelete {
			if err := os.Remove(selectedFile); err != nil {
				return false, fmt.Errorf("failed to delete %s: %w", selectedFile, err)
			}
		}
	}

	// For rename, clone, delete, back -> continue loop
	return false, nil
}

type formState struct {
	urlStr          string
	mode            string
	timeoutSecStr   string
	rateLimitStr    string
	userAgent       string
	waitFor         string
	headless        bool
	outputDir       string
	navSel          string
	contentSel      string
	navWalk         bool
	strict          bool
	dryRun          bool
	yes             bool
	maxSectionsStr  string
	maxMenuItemsStr string
	configPath      string
	finalAction     string
	excludeSel      string
	crawl           bool
	sitemapURL      string
	maxPagesStr     string
	crawlDepthStr   string
}

func newFormState() *formState {
	return &formState{
		mode:            "dynamic",
		timeoutSecStr:   "60",
		rateLimitStr:    "0",
		userAgent:       "go_scrap/1.0",
		waitFor:         "body",
		headless:        true,
		yes:             true,
		maxSectionsStr:  "0",
		maxMenuItemsStr: "0",
		configPath:      "config.json",
		finalAction:     "run",
		maxPagesStr:     "100",
		crawlDepthStr:   "2",
	}
}

func (s *formState) fromConfig(cfg config.Config) {
	if cfg.URL != "" {
		s.urlStr = cfg.URL
	}
	if cfg.Mode != "" {
		s.mode = cfg.Mode
	}
	if cfg.TimeoutSeconds > 0 {
		s.timeoutSecStr = strconv.Itoa(cfg.TimeoutSeconds)
	}
	if cfg.RateLimitPerSecond > 0 {
		s.rateLimitStr = strconv.FormatFloat(cfg.RateLimitPerSecond, 'f', -1, 64)
	}
	if cfg.UserAgent != "" {
		s.userAgent = cfg.UserAgent
	}
	if cfg.WaitForSelector != "" {
		s.waitFor = cfg.WaitForSelector
	}
	if cfg.Headless != nil {
		s.headless = *cfg.Headless
	}
	if cfg.OutputDir != "" {
		s.outputDir = cfg.OutputDir
	}
	if cfg.NavSelector != "" {
		s.navSel = cfg.NavSelector
	}
	if cfg.ContentSelector != "" {
		s.contentSel = cfg.ContentSelector
	}
	if cfg.ExcludeSelector != "" {
		s.excludeSel = cfg.ExcludeSelector
	}
	s.navWalk = cfg.NavWalk
	s.crawl = cfg.Crawl
	if cfg.SitemapURL != "" {
		s.sitemapURL = cfg.SitemapURL
	}
	if cfg.MaxPages > 0 {
		s.maxPagesStr = strconv.Itoa(cfg.MaxPages)
	}
	if cfg.CrawlDepth > 0 {
		s.crawlDepthStr = strconv.Itoa(cfg.CrawlDepth)
	}
}

func buildForm(state *formState) *huh.Form {
	return huh.NewForm(
		buildTargetGroup(state),
		buildCrawlGroup(state),
		buildExtractionGroup(state),
		buildNetworkGroup(state),
		buildOutputGroup(state),
		buildExecutionGroup(state),
		buildFinishGroup(state),
	)
}

func buildTargetGroup(state *formState) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().Title("URL").Placeholder("https://example.com").Value(&state.urlStr).
			Description("Target website URL to scrape.").
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("url is required")
				}
				return nil
			}),
		huh.NewSelect[string]().Title("Mode").Description("Fetching strategy.").Value(&state.mode).Options(
			huh.NewOption("auto", "auto"),
			huh.NewOption("static", "static"),
			huh.NewOption("dynamic", "dynamic"),
		),
		huh.NewConfirm().Title("Enable Crawl").Description("Follow links to multiple pages?").Value(&state.crawl),
	).Title("Target")
}

func buildCrawlGroup(state *formState) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().Title("Sitemap URL").Description("Optional: Start crawl from sitemap.").Value(&state.sitemapURL),
		huh.NewInput().Title("Max Pages").Description("Limit pages crawled.").Value(&state.maxPagesStr).Validate(validateIntString(0, 100000)),
		huh.NewInput().Title("Crawl Depth").Description("Links to follow from start.").Value(&state.crawlDepthStr).Validate(validateIntString(0, 100)),
	).Title("Crawl Settings")
}

func buildExtractionGroup(state *formState) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().Title("Content Selector").Description("CSS selector for main content area.").Placeholder(".content").Value(&state.contentSel),
		huh.NewInput().Title("Nav Selector").Description("CSS selector for navigation menu.").Placeholder(".nav").Value(&state.navSel),
		huh.NewInput().Title("Exclude Selector").Description("CSS selector to remove elements (ads, etc).").Placeholder(".ads").Value(&state.excludeSel),
		huh.NewConfirm().Title("Nav Walk").Description("Click menu items to load content (SPA)?").Value(&state.navWalk),
	).Title("Extraction")
}

func buildNetworkGroup(state *formState) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().Title("Timeout (seconds)").Value(&state.timeoutSecStr).
			Validate(validateIntString(1, 3600)),
		huh.NewInput().Title("Rate limit (requests/sec, 0=off)").Value(&state.rateLimitStr).
			Validate(validateFloatString(0, 1000)),
		huh.NewInput().Title("Wait-for selector").Description("Dynamic mode: wait for this element.").Value(&state.waitFor),
		huh.NewConfirm().Title("Headless").Description("Hide browser window (dynamic)?").Value(&state.headless),
		huh.NewInput().Title("User-Agent").Value(&state.userAgent),
	).Title("Network & Browser")
}

func buildOutputGroup(state *formState) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().Title("Output dir").Description("Optional: defaults to output/<host>").Placeholder("output/<host>").Value(&state.outputDir),
		huh.NewInput().Title("Max sections (0=all)").Value(&state.maxSectionsStr).
			Validate(validateIntString(0, 1000000)),
		huh.NewInput().Title("Max menu items (0=all)").Value(&state.maxMenuItemsStr).
			Validate(validateIntString(0, 1000000)),
	).Title("Output Limits")
}

func buildExecutionGroup(state *formState) *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().Title("Dry run").Description("Simulate without writing files.").Value(&state.dryRun),
		huh.NewConfirm().Title("Strict").Description("Fail on completeness issues.").Value(&state.strict),
		huh.NewConfirm().Title("Skip confirmation").Description("Don't ask to proceed after analysis.").Value(&state.yes),
	).Title("Execution")
}

func buildFinishGroup(state *formState) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().Title("Action").Value(&state.finalAction).Options(
			huh.NewOption("Run scraper now", "run"),
			huh.NewOption("Save config and run", "save_and_run"),
			huh.NewOption("Only save config", "save_only"),
		),
		huh.NewInput().Title("Config path").
			Description("Path for 'Save' actions.").
			Value(&state.configPath).
			Validate(func(s string) error {
				isSaveAction := state.finalAction == "save_and_run" || state.finalAction == "save_only"
				if !isSaveAction {
					return nil
				}
				return validateNewFilename(s)
			}),
	).Title("Finish")
}

func buildResult(state *formState) (Result, error) {
	timeoutSec, err := parsePositiveInt(state.timeoutSecStr, "timeout must be a positive integer")
	if err != nil {
		return Result{}, err
	}
	rateLimit, err := parseNonNegativeFloat(state.rateLimitStr, "rate limit must be a number >= 0")
	if err != nil {
		return Result{}, err
	}
	maxSections, err := parseNonNegativeInt(state.maxSectionsStr, "max sections must be an integer >= 0")
	if err != nil {
		return Result{}, err
	}
	maxMenuItems, err := parseNonNegativeInt(state.maxMenuItemsStr, "max menu items must be an integer >= 0")
	if err != nil {
		return Result{}, err
	}
	maxPages, err := parseNonNegativeInt(state.maxPagesStr, "max pages must be an integer >= 0")
	if err != nil {
		return Result{}, err
	}
	crawlDepth, err := parseNonNegativeInt(state.crawlDepthStr, "crawl depth must be an integer >= 0")
	if err != nil {
		return Result{}, err
	}

	cfg := config.Config{
		URL:                strings.TrimSpace(state.urlStr),
		Mode:               state.mode,
		OutputDir:          strings.TrimSpace(state.outputDir),
		TimeoutSeconds:     timeoutSec,
		RateLimitPerSecond: rateLimit,
		UserAgent:          strings.TrimSpace(state.userAgent),
		WaitForSelector:    strings.TrimSpace(state.waitFor),
		Headless:           &state.headless,
		NavSelector:        strings.TrimSpace(state.navSel),
		ContentSelector:    strings.TrimSpace(state.contentSel),
		ExcludeSelector:    strings.TrimSpace(state.excludeSel),
		NavWalk:            state.navWalk,
		Crawl:              state.crawl,
		SitemapURL:         strings.TrimSpace(state.sitemapURL),
		MaxPages:           maxPages,
		CrawlDepth:         crawlDepth,
	}

	opts := app.Options{
		URL:                strings.TrimSpace(state.urlStr),
		Mode:               fetch.Mode(strings.ToLower(strings.TrimSpace(state.mode))),
		OutputDir:          strings.TrimSpace(state.outputDir),
		Timeout:            time.Duration(timeoutSec) * time.Second,
		RateLimitPerSecond: rateLimit,
		UserAgent:          strings.TrimSpace(state.userAgent),
		WaitFor:            strings.TrimSpace(state.waitFor),
		Headless:           state.headless,
		Yes:                state.yes,
		Strict:             state.strict,
		DryRun:             state.dryRun,
		NavSelector:        strings.TrimSpace(state.navSel),
		ContentSelector:    strings.TrimSpace(state.contentSel),
		ExcludeSelector:    strings.TrimSpace(state.excludeSel),
		NavWalk:            state.navWalk,
		MaxSections:        maxSections,
		MaxMenuItems:       maxMenuItems,
		Crawl:              state.crawl,
		SitemapURL:         strings.TrimSpace(state.sitemapURL),
		MaxPages:           maxPages,
		CrawlDepth:         crawlDepth,
	}

	res := Result{
		Options:    opts,
		ConfigPath: state.configPath,
		Config:     cfg,
	}

	switch state.finalAction {
	case "run":
		res.RunNow = true
	case "save_and_run":
		res.RunNow = true
		res.SaveConfig = true
	case "save_only":
		res.SaveConfig = true
	}

	if res.SaveConfig {
		if err := writeConfig(state.configPath, cfg); err != nil {
			return Result{}, err
		}
	}

	return res, nil
}

func writeConfig(path string, cfg config.Config) error {
	data, err := config.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func parsePositiveInt(s, errMsg string) (int, error) {
	val, err := parseInt(s)
	if err != nil || val <= 0 {
		return 0, errors.New(errMsg)
	}
	return val, nil
}

func parseNonNegativeInt(s, errMsg string) (int, error) {
	val, err := parseInt(s)
	if err != nil || val < 0 {
		return 0, errors.New(errMsg)
	}
	return val, nil
}

func parseNonNegativeFloat(s, errMsg string) (float64, error) {
	val, err := parseFloat(s)
	if err != nil || val < 0 {
		return 0, errors.New(errMsg)
	}
	return val, nil
}

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

func validateIntString(minVal, maxVal int) func(string) error {
	return func(s string) error {
		v, err := parseInt(s)
		if err != nil {
			return errors.New("must be an integer")
		}
		if v < minVal || v > maxVal {
			return fmt.Errorf("must be between %d and %d", minVal, maxVal)
		}
		return nil
	}
}

func validateNewFilename(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("filename cannot be empty")
	}
	if strings.ContainsAny(s, `/\:*?"<>|`) {
		return errors.New("invalid characters")
	}
	target := ensureJSONExtension(s)
	if _, err := os.Stat(target); err == nil {
		return errors.New("file already exists")
	}
	return nil
}

func ensureJSONExtension(s string) string {
	if !strings.HasSuffix(s, ".json") {
		return s + ".json"
	}
	return s
}

func validateFloatString(minVal, maxVal float64) func(string) error {
	return func(s string) error {
		v, err := parseFloat(s)
		if err != nil {
			return errors.New("must be a number")
		}
		if v < minVal || v > maxVal {
			return fmt.Errorf("must be between %.2f and %.2f", minVal, maxVal)
		}
		return nil
	}
}
