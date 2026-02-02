# AGENTS.md

## Purpose
This repository contains `go_scrap`, a CLI-first documentation scraper that converts web docs into structured Markdown and JSON, preserving section and menu structure.

## Key behaviors
- No args launches the TUI: `go run .`
- CLI mode uses flags: `go run . --url https://example.com --mode auto --yes`
- Config files live in `.codex/CONFIGS/`

## Common commands
- Run (TUI): `go run .`
- Run (CLI): `go run . --url https://example.com --mode auto --yes`
- Tests: `go test ./...`
- Vet: `go vet ./...`
- Tidy: `go mod tidy`

## Files to know
- `README.md` ? usage and flags
- `ROADMAP.md` ? full roadmap
- `.codex/` ? project notes and templates

## Notes
- Prefer config files for site-specific selectors.
- Use `--dry-run` before large scrapes.
- Update `README.md` and `.gitignore` after changes that affect usage or outputs.
