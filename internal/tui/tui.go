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
	var (
		urlStr          string
		mode            string = "dynamic"
		timeoutSecStr   string = "60"
		rateLimitStr    string = "0"
		userAgent       string = "go_scrap/1.0"
		waitFor         string = "body"
		headless        bool   = true
		outputDir       string
		navSel          string
		contentSel      string
		navWalk         bool
		strict          bool
		dryRun          bool
		yes             bool   = true
		maxSectionsStr  string = "0"
		maxMenuItemsStr string = "0"

		saveConfig bool
		configPath string = "config.json"
		runNow     bool   = true
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("URL").Placeholder("https://example.com").Value(&urlStr).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("url is required")
					}
					return nil
				}),
			huh.NewSelect[string]().Title("Mode").Value(&mode).Options(
				huh.NewOption("auto", "auto"),
				huh.NewOption("static", "static"),
				huh.NewOption("dynamic", "dynamic"),
			),
			huh.NewInput().Title("Timeout (seconds)").Value(&timeoutSecStr).
				Validate(validateIntString(1, 3600)),
			huh.NewInput().Title("Rate limit (requests/sec, 0=off)").Value(&rateLimitStr).
				Validate(validateFloatString(0, 1000)),
			huh.NewInput().Title("Wait-for selector (dynamic)").Value(&waitFor),
			huh.NewConfirm().Title("Headless (dynamic)").Value(&headless),
			huh.NewInput().Title("User-Agent").Value(&userAgent),
		),
		huh.NewGroup(
			huh.NewInput().Title("Output dir (optional)").Placeholder("output/<host>").Value(&outputDir),
			huh.NewInput().Title("Nav selector (optional)").Placeholder(".nav").Value(&navSel),
			huh.NewInput().Title("Content selector (optional)").Placeholder(".content").Value(&contentSel),
			huh.NewConfirm().Title("Nav walk (click each menu anchor)").Value(&navWalk),
			huh.NewInput().Title("Max sections (0=all)").Value(&maxSectionsStr).
				Validate(validateIntString(0, 1000000)),
			huh.NewInput().Title("Max menu items (0=all)").Value(&maxMenuItemsStr).
				Validate(validateIntString(0, 1000000)),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("Dry run (no files written)").Value(&dryRun),
			huh.NewConfirm().Title("Strict (fail on completeness issues)").Value(&strict),
			huh.NewConfirm().Title("Skip confirmation prompt (--yes)").Value(&yes),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("Save config file").Value(&saveConfig),
			huh.NewInput().Title("Config path").Value(&configPath).
				Validate(func(s string) error {
					if !saveConfig {
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
			huh.NewConfirm().Title("Run now").Value(&runNow),
		),
	)

	if err := form.Run(); err != nil {
		return Result{}, err
	}

	timeoutSec, err := parseInt(timeoutSecStr)
	if err != nil || timeoutSec <= 0 {
		return Result{}, errors.New("timeout must be a positive integer")
	}
	rateLimit, err := parseFloat(rateLimitStr)
	if err != nil || rateLimit < 0 {
		return Result{}, errors.New("rate limit must be a number >= 0")
	}
	maxSections, err := parseInt(maxSectionsStr)
	if err != nil || maxSections < 0 {
		return Result{}, errors.New("max sections must be an integer >= 0")
	}
	maxMenuItems, err := parseInt(maxMenuItemsStr)
	if err != nil || maxMenuItems < 0 {
		return Result{}, errors.New("max menu items must be an integer >= 0")
	}

	cfg := config.Config{
		Mode:               mode,
		OutputDir:          strings.TrimSpace(outputDir),
		TimeoutSeconds:     timeoutSec,
		RateLimitPerSecond: rateLimit,
		UserAgent:          strings.TrimSpace(userAgent),
		WaitForSelector:    strings.TrimSpace(waitFor),
		Headless:           &headless,
		NavSelector:        strings.TrimSpace(navSel),
		ContentSelector:    strings.TrimSpace(contentSel),
		NavWalk:            navWalk,
	}

	if saveConfig {
		data, err := config.Marshal(cfg)
		if err != nil {
			return Result{}, err
		}
		if err := os.WriteFile(configPath, data, 0600); err != nil {
			return Result{}, err
		}
	}

	opts := app.Options{
		URL:                strings.TrimSpace(urlStr),
		Mode:               fetch.Mode(strings.ToLower(strings.TrimSpace(mode))),
		OutputDir:          strings.TrimSpace(outputDir),
		Timeout:            time.Duration(timeoutSec) * time.Second,
		RateLimitPerSecond: rateLimit,
		UserAgent:          strings.TrimSpace(userAgent),
		WaitFor:            strings.TrimSpace(waitFor),
		Headless:           headless,
		Yes:                yes,
		Strict:             strict,
		DryRun:             dryRun,
		NavSelector:        strings.TrimSpace(navSel),
		ContentSelector:    strings.TrimSpace(contentSel),
		NavWalk:            navWalk,
		MaxSections:        maxSections,
		MaxMenuItems:       maxMenuItems,
	}

	return Result{Options: opts, SaveConfig: saveConfig, ConfigPath: configPath, Config: cfg, RunNow: runNow}, nil
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
