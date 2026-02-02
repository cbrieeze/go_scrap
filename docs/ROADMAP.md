# Project Roadmap

This document outlines the future development plans for `go_scrap`. The priority is stability and reusable scraping primitives before larger crawl features.

## Recommended implementation order (dependency-driven)

1) **Foundation**: Hardening, CI/CD, Docker, Retries (Done)
   - Reason: prevents scaling up bad extraction; gives fast feedback loops.
2) **Structure & Data Contract**: Menu mapping, Stable IDs, Indexing (Done)
   - Reason: ensures high-quality, structured content and prevents re-embedding churn.
3) **Content Fidelity**: Anchors, Tables, Link/Asset handling (Current)
   - Reason: improves the quality of the markdown content itself.
4) **Scale & Politeness**: Crawl queue, robots.txt, throttling, resume
   - Reason: crawling amplifies failure modes; do it after extraction/indexing are trusted.
5) **Ecosystem & Pipelines**: Offline RAG, export bundles, observability, plugins
   - Reason: valuable add-ons once the core pipeline is solid.

## Phase 1: Foundation & Core Refinement (Completed)

- [x] **CI/CD**: Set up GitHub Actions for linting (`golangci-lint`) and build verification.
- [x] **Docker Support**: Provide a `Dockerfile` for consistent Playwright execution environments.
- [x] **Retry Mechanism**: Implement exponential backoff for network requests to handle transient failures.
- [x] **Disk Caching**: Implement a local cache for raw HTML responses to speed up development (`--cache`).
- [x] **Exclusion Selectors**: Add `--exclude-selector` flag to remove specific elements before processing.
- [x] **Playwright Hints**: Pin Playwright driver install steps and provide clear runtime hints (in README).
- [x] **Selector Inspection**: Add selector validation/inspection helpers (`inspect` subcommand).

## Phase 2: Structure & Data Contract (Completed)

- [x] **Menu Tree Output**: Extract menu tree into `menu.json` and map to section files.
- [x] **Completeness Reporting**: Add section completeness reporting with actionable hints.
- [x] **Inspect Integration**: Merge `cmd/inspect/main.go` into the main application as a subcommand.
- [x] **Search Index Output (RAG)**: Emit an `index.jsonl` with stable IDs for RAG pipelines.
- [x] **Stable Section IDs**: Generate deterministic `section_id` values (hash of `url + heading_path + anchor`).

## Phase 3: Content Fidelity & Hardening (Current)\n\n*(library-backed = can be implemented mostly with existing Go packages)*

- [x] **Anchor-only Pages**: Support anchor-only pages by mapping menu anchors to the closest heading.
- [x] **Asset Downloading**: Add an option to download referenced images locally and rewrite Markdown links to point to the `output/` directory.
- [ ] **Complex Table Support** (library-backed): Improve the Markdown converter to handle HTML tables with `rowspan` and `colspan` more gracefully. (e.g., `github.com/JohannesKaufmann/html-to-markdown` plugins)
- [ ] **Conversion Hardening** (library-backed): Strengthen HTML-to-Markdown conversions (code blocks, lists, nested elements). (custom rules on `github.com/JohannesKaufmann/html-to-markdown`)
- [ ] **Link Rewriting** (library-backed): Resolve relative links to absolute (or local if downloaded) and preserve `source_url` per section. (built-in `net/url`)
- [ ] **Code Block Intelligence**: Detect/infer missing language tags and strip UI artifacts (e.g., "Copy" buttons).
- [ ] **Better Errors**: Improve error messages (selectors not found, empty content, timeouts).
- [ ] **Test Coverage**: Add unit tests for `internal/parse` and `internal/markdown` packages.

## Phase 4: Scale, Crawl & Politeness

- [ ] **URL Queue + De-dupe** (library-backed): Implement a crawl queue with URL normalization and de-dupe. (e.g., `github.com/gocolly/colly` or `github.com/PuerkitoBio/fetchbot`)
  - Depends on: canonical URL normalization.
- [ ] **Canonical URL Normalization** (library-backed): Normalize scheme, host, trailing slashes, and fragments. (built-in `net/url`)
  - Depends on: URL queue.
- [ ] **Recursive Crawling**: Follow links within the same domain with a configurable depth (`--depth`).
  - Depends on: URL queue + de-dupe.
- [ ] **Sitemap Ingestion** (library-backed): Allow passing a `sitemap.xml` URL to batch scrape all pages on a site. (e.g., `github.com/oxffaa/gopher-parse-sitemap` or `encoding/xml`)
  - Depends on: URL queue + canonicalization.
- [ ] **Robots.txt Compliance** (library-backed): Respect `robots.txt` and provide an override flag. (e.g., `golang.org/x/robotstxt` or `github.com/temoto/robotstxt`)
  - Depends on: URL queue.
- [ ] **Rate Limiting / AutoThrottle** (library-backed): Polite delays and adaptive throttling under load. (e.g., `golang.org/x/time/rate`)
  - Depends on: URL queue + request metrics.
- [ ] **Resume / Incremental Sync** (library-backed): Update existing outputs based on last-fetch timestamps or content hashes. (e.g., SQLite + hashes)
  - Depends on: stable IDs + output index.
- [ ] **Authentication** (library-backed): Support custom headers or cookies for scraping behind logins. (cookie jars in `net/http`)
- [ ] **Proxy Support** (library-backed): Add support for HTTP/SOCKS proxies (`--proxy`). (transport config in `net/http`)
- [ ] **Crawl Index**: Generate a crawl index file to summarize pages and section counts.
  - Depends on: URL queue.

## Phase 5: Ecosystem & Pipelines

- [ ] **Offline RAG (SQLite-vec)** (library-backed): Tooling to ingest `index.jsonl` into a local SQLite vector database. (sqlite-vec + `modernc.org/sqlite` or `github.com/mattn/go-sqlite3`)
  - Depends on: stable IDs + index.jsonl.
- [ ] **AI-Driven Enrichment**: Generate synthetic Q&A, summaries, and keywords for sections using LLMs.
- [ ] **Framework Auto-Detection**: Detect common docs frameworks (Docusaurus, MkDocs) and auto-configure selectors.
- [ ] **Additional Export Formats** (library-backed): Add JSONL and CSV exports for downstream pipelines. (`encoding/json`, `encoding/csv`)
- [ ] **Export Bundles**: Provide a "one file per page" export bundle.
- [ ] **Heading Normalization**: Normalize heading hierarchy.
- [ ] **Boilerplate Stripping**: Strip boilerplate navigation/footers when possible.
- [ ] **De-duplication**: De-duplicate repeating blocks across pages.
- [ ] **Chunking Controls**: Support `--max-chars`/`--max-tokens` with smart sub-splitting by subheadings/paragraphs.
- [ ] **Pipeline Hooks**: Add a post-processing pipeline interface (validate, dedupe, enrich, export).
- [ ] **Rich Output Formats** (library-backed): Support exporting to single-file HTML, PDF, or EPUB. (e.g., `github.com/go-shiori/go-epub`, or HTML→PDF via Playwright/Chromedp)
- [ ] **Screenshot Capture** (library-backed): Optional screenshot capture per page. (Playwright)
- [ ] **Rendered HTML + Metadata** (library-backed): Save rendered HTML + response metadata to aid debugging. (Playwright response capture)
- [ ] **Network Traces** (library-backed): Add network trace recording for JS-heavy sites. (Playwright tracing)
- [ ] **Observability** (library-backed): Per-page logs, crawl stats, and error summaries. (`log/slog` or `zerolog`)

## Tracking

- Each phase should ship with a small test suite covering core parsing and conversion behavior.
- Keep CLI flags stable and avoid site-specific defaults by using config files.

## Known Limitations (To Be Addressed)

Ref: `README.md`

- Sections are currently strictly split by headings.
- Anchors must map to element IDs.
