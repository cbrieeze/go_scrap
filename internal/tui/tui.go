package tui

import (
	"errors"
	"fmt"
	"os"
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
	state := newFormState()
	form := buildForm(state)
	if err := form.Run(); err != nil {
		return Result{}, err
	}

	return buildResult(state)
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
	saveConfig      bool
	configPath      string
	runNow          bool
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
		runNow:          true,
	}
}

func buildForm(state *formState) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("URL").Placeholder("https://example.com").Value(&state.urlStr).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("url is required")
					}
					return nil
				}),
			huh.NewSelect[string]().Title("Mode").Value(&state.mode).Options(
				huh.NewOption("auto", "auto"),
				huh.NewOption("static", "static"),
				huh.NewOption("dynamic", "dynamic"),
			),
			huh.NewInput().Title("Timeout (seconds)").Value(&state.timeoutSecStr).
				Validate(validateIntString(1, 3600)),
			huh.NewInput().Title("Rate limit (requests/sec, 0=off)").Value(&state.rateLimitStr).
				Validate(validateFloatString(0, 1000)),
			huh.NewInput().Title("Wait-for selector (dynamic)").Value(&state.waitFor),
			huh.NewConfirm().Title("Headless (dynamic)").Value(&state.headless),
			huh.NewInput().Title("User-Agent").Value(&state.userAgent),
		),
		huh.NewGroup(
			huh.NewInput().Title("Output dir (optional)").Placeholder("output/<host>").Value(&state.outputDir),
			huh.NewInput().Title("Nav selector (optional)").Placeholder(".nav").Value(&state.navSel),
			huh.NewInput().Title("Content selector (optional)").Placeholder(".content").Value(&state.contentSel),
			huh.NewConfirm().Title("Nav walk (click each menu anchor)").Value(&state.navWalk),
			huh.NewInput().Title("Max sections (0=all)").Value(&state.maxSectionsStr).
				Validate(validateIntString(0, 1000000)),
			huh.NewInput().Title("Max menu items (0=all)").Value(&state.maxMenuItemsStr).
				Validate(validateIntString(0, 1000000)),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("Dry run (no files written)").Value(&state.dryRun),
			huh.NewConfirm().Title("Strict (fail on completeness issues)").Value(&state.strict),
			huh.NewConfirm().Title("Skip confirmation prompt (--yes)").Value(&state.yes),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("Save config file").Value(&state.saveConfig),
			huh.NewInput().Title("Config path").Value(&state.configPath).
				Validate(func(s string) error {
					if !state.saveConfig {
						return nil
					}
					if strings.TrimSpace(s) == "" {
						return errors.New("config path is required")
					}
					if _, err := os.Stat(s); err == nil {
						return fmt.Errorf("file already exists: %s", s)
					}
					return nil
				}),
			huh.NewConfirm().Title("Run now").Value(&state.runNow),
		),
	)
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
		NavWalk:            state.navWalk,
	}

	if state.saveConfig {
		if err := writeConfig(state.configPath, cfg); err != nil {
			return Result{}, err
		}
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
		NavWalk:            state.navWalk,
		MaxSections:        maxSections,
		MaxMenuItems:       maxMenuItems,
	}

	return Result{Options: opts, SaveConfig: state.saveConfig, ConfigPath: state.configPath, Config: cfg, RunNow: state.runNow}, nil
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

func validateIntString(min, max int) func(string) error {
	return func(s string) error {
		v, err := parseInt(s)
		if err != nil {
			return errors.New("must be an integer")
		}
		if v < min || v > max {
			return fmt.Errorf("must be between %d and %d", min, max)
		}
		return nil
	}
}

func validateFloatString(min, max float64) func(string) error {
	return func(s string) error {
		v, err := parseFloat(s)
		if err != nil {
			return errors.New("must be a number")
		}
		if v < min || v > max {
			return fmt.Errorf("must be between %.2f and %.2f", min, max)
		}
		return nil
	}
}
