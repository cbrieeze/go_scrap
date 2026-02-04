package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"go_scrap/internal/parse"
	"go_scrap/internal/report"
)

type RenderedSection struct {
	HeadingID  string
	ContentIDs []string
	Markdown   string
}

type Rendered struct {
	Markdown string
	Sections []RenderedSection
}

type WriteResult struct {
	OutputDir    string
	MarkdownPath string
	JSONPath     string
	IndexPath    string
	MenuPath     string
}

type Hook interface {
	Name() string
	BeforeRender(ctx context.Context, opts Options, doc *parse.Document, rep *report.Report) error
	AfterRender(ctx context.Context, opts Options, doc *parse.Document, rep *report.Report, rendered *Rendered) error
	AfterWrite(ctx context.Context, opts Options, doc *parse.Document, rep *report.Report, rendered Rendered, written WriteResult) error
}

type HookBase struct{}

func (HookBase) BeforeRender(context.Context, Options, *parse.Document, *report.Report) error {
	return nil
}
func (HookBase) AfterRender(context.Context, Options, *parse.Document, *report.Report, *Rendered) error {
	return nil
}
func (HookBase) AfterWrite(context.Context, Options, *parse.Document, *report.Report, Rendered, WriteResult) error {
	return nil
}

type hookFactory func(opts Options) (Hook, error)

func buildHooks(opts Options) ([]Hook, error) {
	if len(opts.PipelineHooks) == 0 {
		return nil, nil
	}

	registry := map[string]hookFactory{
		"strict-report": func(Options) (Hook, error) { return strictReportHook{}, nil },
		"exec":          func(Options) (Hook, error) { return execHook{}, nil },
	}

	names := dedupePreserveOrder(opts.PipelineHooks)
	out := make([]Hook, 0, len(names))
	for _, name := range names {
		factory, ok := registry[name]
		if !ok {
			return nil, fmt.Errorf("unknown pipeline hook %q (available: %s)", name, strings.Join(sortedKeys(registry), ", "))
		}
		h, err := factory(opts)
		if err != nil {
			return nil, fmt.Errorf("init hook %q: %w", name, err)
		}
		out = append(out, h)
	}
	return out, nil
}

func (p *pipeline) runBeforeRenderHooks(ctx context.Context, opts Options, doc *parse.Document, rep *report.Report) error {
	for _, h := range p.hooks {
		if err := h.BeforeRender(ctx, opts, doc, rep); err != nil {
			return fmt.Errorf("hook %q failed (before render): %w", h.Name(), err)
		}
		*rep = report.Analyze(doc)
	}
	return nil
}

func (p *pipeline) runAfterRenderHooks(ctx context.Context, opts Options, doc *parse.Document, rep *report.Report, rendered *Rendered) error {
	for _, h := range p.hooks {
		if err := h.AfterRender(ctx, opts, doc, rep, rendered); err != nil {
			return fmt.Errorf("hook %q failed (after render): %w", h.Name(), err)
		}
	}
	return nil
}

func (p *pipeline) runAfterWriteHooks(ctx context.Context, opts Options, doc *parse.Document, rep *report.Report, rendered Rendered, written WriteResult) error {
	for _, h := range p.hooks {
		if err := h.AfterWrite(ctx, opts, doc, rep, rendered, written); err != nil {
			return fmt.Errorf("hook %q failed (after write): %w", h.Name(), err)
		}
	}
	return nil
}

func toRenderedSections(sections []sectionMarkdown) []RenderedSection {
	out := make([]RenderedSection, 0, len(sections))
	for _, s := range sections {
		out = append(out, RenderedSection{
			HeadingID:  s.HeadingID,
			ContentIDs: append([]string(nil), s.ContentIDs...),
			Markdown:   s.Markdown,
		})
	}
	return out
}

func fromRendered(rendered Rendered) (string, []sectionMarkdown) {
	out := make([]sectionMarkdown, 0, len(rendered.Sections))
	for _, s := range rendered.Sections {
		out = append(out, sectionMarkdown{
			HeadingID:  s.HeadingID,
			ContentIDs: append([]string(nil), s.ContentIDs...),
			Markdown:   s.Markdown,
		})
	}
	return rendered.Markdown, out
}

func dedupePreserveOrder(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, raw := range items {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func sortedKeys[M ~map[string]V, V any](m M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// small list, bubble-free
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

type strictReportHook struct {
	HookBase
}

func (strictReportHook) Name() string { return "strict-report" }

func (strictReportHook) BeforeRender(_ context.Context, _ Options, _ *parse.Document, rep *report.Report) error {
	if rep == nil {
		return errors.New("missing report")
	}
	if reportHasIssues(*rep) {
		return errors.New("completeness checks failed")
	}
	return nil
}

type execHook struct {
	HookBase
}

func (execHook) Name() string { return "exec" }

func (execHook) AfterWrite(ctx context.Context, opts Options, _ *parse.Document, _ *report.Report, _ Rendered, written WriteResult) error {
	commands := make([]string, 0, len(opts.PostCommands))
	for _, c := range opts.PostCommands {
		c = strings.TrimSpace(c)
		if c == "" || strings.HasPrefix(c, "#") {
			continue
		}
		commands = append(commands, c)
	}
	if len(commands) == 0 {
		return nil
	}

	for _, cmdStr := range commands {
		cmd, err := commandForShell(ctx, cmdStr)
		if err != nil {
			return err
		}
		cmd.Env = append(os.Environ(),
			"GO_SCRAP_URL="+opts.URL,
			"GO_SCRAP_OUTPUT_DIR="+written.OutputDir,
			"GO_SCRAP_MARKDOWN_PATH="+written.MarkdownPath,
			"GO_SCRAP_JSON_PATH="+written.JSONPath,
			"GO_SCRAP_INDEX_PATH="+written.IndexPath,
			"GO_SCRAP_MENU_PATH="+written.MenuPath,
		)
		if written.OutputDir != "" {
			cmd.Dir = written.OutputDir
		}
		if opts.Stdout {
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("post command failed %q: %w", cmdStr, err)
		}
	}
	return nil
}

func commandForShell(ctx context.Context, command string) (*exec.Cmd, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("empty command")
	}
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command), nil
	}
	return exec.CommandContext(ctx, "sh", "-c", command), nil
}
