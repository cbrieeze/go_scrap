package testconfigs

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go_scrap/internal/app"
	"go_scrap/internal/config"
	"go_scrap/internal/fetch"
)

func Run(args []string) error {
	dir, maxSec, maxMenu, dryRun, timeout, headless, err := parseOptions(args)
	if err != nil {
		return err
	}

	resolvedDir := resolveDir(dir)
	files, err := os.ReadDir(resolvedDir)
	if err != nil {
		return fmt.Errorf("read configs dir: %w", err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		path := filepath.Join(resolvedDir, f.Name())
		cfg, err := config.Load(path)
		if err != nil {
			fmt.Printf("%s: INVALID (%v)\n", f.Name(), err)
			continue
		}
		if strings.TrimSpace(cfg.URL) == "" {
			fmt.Printf("%s: SKIP (no url)\n", f.Name())
			continue
		}

		opts := app.Options{
			URL:             cfg.URL,
			Mode:            fetch.Mode(cfg.Mode),
			OutputDir:       cfg.OutputDir,
			Timeout:         time.Duration(timeout) * time.Second,
			UserAgent:       cfg.UserAgent,
			WaitFor:         cfg.WaitForSelector,
			Headless:        headless,
			Yes:             true,
			Strict:          false,
			DryRun:          dryRun,
			NavSelector:     cfg.NavSelector,
			ContentSelector: cfg.ContentSelector,
			MaxSections:     maxSec,
			MaxMenuItems:    maxMenu,
		}

		if cfg.TimeoutSeconds > 0 {
			opts.Timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
		}
		if cfg.Headless != nil {
			opts.Headless = *cfg.Headless
		}

		fmt.Printf("\n=== %s ===\n", f.Name())
		err = app.Run(context.Background(), opts)
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
		} else {
			fmt.Printf("OK\n")
		}
	}

	return nil
}

func parseOptions(args []string) (dir string, maxSec int, maxMenu int, dryRun bool, timeout int, headless bool, err error) {
	fs := flag.NewFlagSet("test-configs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&dir, "dir", config.DefaultConfigDir, "Directory of config JSON files")
	fs.IntVar(&maxSec, "max-sections", 3, "Limit number of sections written (0 = all)")
	fs.IntVar(&maxMenu, "max-menu-items", 5, "Limit number of menu section files written (0 = all)")
	fs.BoolVar(&dryRun, "dry-run", true, "Dry-run (no files written)")
	fs.IntVar(&timeout, "timeout", app.DefaultTimeoutSeconds, "Timeout seconds")
	fs.BoolVar(&headless, "headless", true, "Run browser headless")
	err = fs.Parse(args)
	return
}

func resolveDir(dir string) string {
	if strings.TrimSpace(dir) != "" {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	for _, candidate := range config.SearchDirs() {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return dir
}
