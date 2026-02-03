# go_scrap

Purpose: a CLI scraper that pulls documentation-like sites into AI-friendly Markdown + JSON, while preserving section structure and menu-based navigation. Supports single-page and multi-page crawling with sitemap support.

CLI web scraper in Go that:
- **Single-page mode**: Fetches full HTML once (static/dynamic/auto)
  - Enumerates heading IDs and `href="#..."` anchors
  - Slices content into sections (heading + content until next heading)
  - Converts each section to Markdown (with table helper)
  - Exports Markdown + JSON and runs completeness checks
  - Optional nav-walk mode for JS docs that load content per anchor

- **Multi-page crawl mode**: Crawls multiple pages with intelligent rate limiting
  - Link-following crawl with configurable depth and max pages
  - Sitemap URL support (including sitemap indexes)
  - Per-URL output directories for organized multi-page scrapes
  - Crawl statistics and error tracking

## Requirements

- Go 1.22+
- Playwright (for dynamic mode)

## Install dependencies

From the `go_scrap` folder:

```bash
go mod tidy
```

Install Playwright browsers (required for dynamic mode):

```bash
cd go_scrap
playwright install chromium
```

## Quickstart

```bash
cd go_scrap
go run . --url https://example.com --mode auto
```

TUI mode (no arguments):

```bash
cd go_scrap
go run .
```

Dynamic + menu + content selectors:

```bash
go run . --url https://api.freshservice.com/ --mode dynamic --wait-for "body" --nav-selector ".nav" --content-selector ".content" --yes
```

Multi-page crawl (link following):

```bash
go run . --url https://docs.example.com --crawl --max-pages 50 --crawl-depth 2 --yes
```

Crawl from sitemap:

```bash
go run . --sitemap https://docs.example.com/sitemap.xml --max-pages 200 --yes
```

Crawl with URL filtering:

```bash
go run . --url https://docs.example.com --crawl --crawl-filter "/docs/" --max-pages 100 --yes
```

## Usage

Common flags:

```bash
# Fetch & parse
--mode auto|static|dynamic
--output-dir output/<host>
--wait-for ".selector"      # dynamic mode
--headless true|false
--yes                        # skip confirmation prompt
--strict                     # fail if completeness checks report issues
--dry-run                    # fetch/analyze only; write nothing

# Single-page mode
--max-sections 25            # limit number of sections written (0 = all)
--max-menu-items 50          # limit number of menu-based section files written (0 = all)
--max-md-bytes 20000         # split section markdown files before this size (0 = no split)
--max-chars 20000            # split section markdown files before this character count (0 = no split)
--max-tokens 4000            # split section markdown files before this token estimate (0 = no split)
--nav-selector ".nav"        # extract menu tree
--content-selector ".content" # focus on content container
--nav-walk                   # click each menu anchor and capture content
--exclude-selector ".ads"    # remove elements before processing

# Multi-page crawl mode
--crawl                      # enable multi-page crawl mode
--sitemap URL                # crawl from sitemap.xml (enables --crawl)
--max-pages 100              # maximum pages to crawl (default: 100)
--crawl-depth 2              # max link depth from start URL (default: 2)
--crawl-filter "regex"       # regex to filter URLs during crawl

# General
--rate-limit 2.5             # requests per second (0 = off)
--config config.json         # load JSON config
--init-config                # interactive config wizard
```

Run directly from `main.go`:

```bash
cd go_scrap
go run main.go --url https://example.com --mode auto
```

## Subcommands

- Inspect selectors:

```bash
go run . inspect --url https://example.com --wait-for "body"
```

- Test configs (batch, optional dry-run):

```bash
go run . test-configs --dir configs --dry-run --max-sections 3 --max-menu-items 5
```

## VS Code tasks

This repo includes VS Code tasks in `.vscode/tasks.json` to speed up common workflows:

- `go_scrap: test` — Run all tests.
- `go_scrap: test (no cache)` — Run tests without Go test caching.
- `go_scrap: test (markdown)` — Run the markdown conversion tests only.
- `go_scrap: clean test cache` — Clear Go's test cache.
- `go_scrap: vet` — Run `go vet` for static analysis.
- `go_scrap: build` — Compile all packages.
- `go_scrap: tidy` — Sync `go.mod`/`go.sum`.
- `go_scrap: mod download` — Pre-download dependencies.
- `go_scrap: fmt` — Run `gofmt -w .` on the repo.
- `go_scrap: lint` — Run `golangci-lint` if installed locally.
- `go_scrap: lint (module)` — Run lint via `go run` (slower, needs network).
- `go_scrap: run (TUI)` — Start the interactive UI.
- `go_scrap: run (dry-run)` — Fetch + analyze without writing output.
- `go_scrap: run (sample)` — Small run to validate output quickly.
- `go_scrap: inspect` — Selector discovery helper.
- `go_scrap: test-configs` — Batch-validate configs with limited output.

### What is `problemMatcher`?

`problemMatcher` tells VS Code how to parse task output and surface errors/warnings in the Problems panel.
We leave it empty (`[]`) because Go tools already print clear errors, and we don't rely on VS Code's
built-in matchers here. If you want richer diagnostics, you can add a Go matcher (for example,
`"$go"`) to the tasks that emit compiler errors.

## Outputs

Outputs:
- `content.md`
- `content.json`
- `menu.json` (if --nav-selector provided)
- `sections/` (if --nav-selector provided)

### Crawl mode outputs

In crawl mode (`--crawl` or `--sitemap`), outputs are organized per-URL with a summary index:

- `crawl-index.json` - Summary with per-page section counts and errors
- `pages/<path>/` - Per-URL directories containing standard outputs

The `crawl-index.json` includes:
```json
{
  "started_at": "2024-01-01T10:00:00Z",
  "completed_at": "2024-01-01T10:05:00Z",
  "base_url": "https://docs.example.com",
  "pages_crawled": 42,
  "pages_failed": 2,
  "total_sections": 156,
  "pages": [
    { "url": "...", "status": "success", "section_count": 5, "fetched_at": "..." },
    { "url": "...", "status": "error", "error": "timeout", "fetched_at": "..." }
  ],
  "errors": ["..."]
}
```

### Chunking behavior

When you set `--max-md-bytes`, `--max-chars`, or `--max-tokens`, the scraper splits outputs at **section boundaries**:

- `content.md` becomes an index that points to `content/part-###.md` files.
- `sections/<name>.md` becomes an index when split, with parts in `sections/<name>/part-###.md`.
- Splits prefer `###`/`####` subheadings, then fall back to paragraph boundaries.
- A single section is never split across files; if a section has no subheadings and exceeds the limit, it stays intact.

Example output layout:

```
output/<host>/
  content.md
  content.json
  menu.json
  sections/
    introduction.md
    api-index.md
    api-index/
      part-001.md
    tickets/
      create_ticket.md
```

## Config schema

Create a JSON file and pass it with `--config`.

```json
{
  "url": "https://example.com",
  "mode": "auto|static|dynamic",
  "output_dir": "output/<host>",
  "timeout_seconds": 60,
  "user_agent": "go_scrap/1.0",
  "wait_for": "body",
  "headless": true,
  "nav_selector": ".nav",
  "content_selector": ".content",
  "exclude_selector": ".ads, .cookie-banner",
  "nav_walk": false,
  "rate_limit_per_second": 2.5,
  "max_markdown_bytes": 20000,
  "max_chars": 20000,
  "max_tokens": 4000,
  "crawl": false,
  "sitemap_url": "",
  "max_pages": 100,
  "crawl_depth": 2,
  "crawl_filter": ""
}
```

## Dynamic vs static

- Use `--mode static` for simple HTML pages (fast).
- Use `--mode dynamic` for JS-heavy docs or missing content.
- `--wait-for` should target a stable container that appears when content is ready.

## Troubleshooting

- **No headings/anchors**: remove `--content-selector` or adjust it to the main article container.
- **Selector not found**: confirm the selector with browser dev tools, or omit to parse the full page.
- **Timeouts**: increase `--timeout` or use `--wait-for "body"` for a lighter wait condition (error messages now include timeout hints).
- **Nav-walk returns few sections**: the nav may be outside the content container; adjust `--content-selector` or remove it.

## Performance tips

- Prefer `--content-selector` to reduce parsing time.
- Use `--wait-for` to avoid waiting on large single-page app loads.
- Use `--mode static` when possible.
- Use `--nav-walk` only when the site loads content per anchor.

## Limitations

- Sections are split by headings; pages without headings will produce few sections.
- Tables with complex row/col spans may not convert perfectly to Markdown.
- Anchors must map to element IDs to be stitched into menu-based sections.

## Notes

- Table conversion uses a dedicated helper to preserve row/column structure.
- The CLI prints discovered IDs/anchors before asking to continue.
- Selector failures now include the selector value to speed debugging.
\n## Docs\n\n- docs/ROADMAP.md\n- docs/CLAUDE.md\n
