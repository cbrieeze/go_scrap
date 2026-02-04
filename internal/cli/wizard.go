package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go_scrap/internal/app"
	"go_scrap/internal/config"
)

func RunConfigWizard() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Config wizard (press Enter to accept defaults)")

	path := promptString(reader, "Config file path", config.DefaultConfigPath())
	urlStr := promptString(reader, "URL", "")
	mode := promptString(reader, "Mode (auto|static|dynamic)", "dynamic")
	outputDir := promptString(reader, "Output dir (optional)", "")
	timeout := promptInt(reader, "Timeout seconds", app.DefaultTimeoutSeconds)
	waitFor := promptString(reader, "Wait for selector", "body")
	headless := promptBool(reader, "Headless (true/false)", true)
	navSel := promptString(reader, "Nav selector (optional)", "")
	contentSel := promptString(reader, "Content selector (optional)", "")
	hooks := promptString(reader, "Pipeline hooks (comma-separated, optional)", "")
	postCmds := promptString(reader, "Post commands (one line; optional)", "")

	cfg := config.Config{
		URL:             strings.TrimSpace(urlStr),
		Mode:            mode,
		OutputDir:       strings.TrimSpace(outputDir),
		TimeoutSeconds:  timeout,
		WaitForSelector: waitFor,
		Headless:        &headless,
		NavSelector:     navSel,
		ContentSelector: contentSel,
		PipelineHooks:   splitCommaList(hooks),
		PostCommands:    splitNonEmptyLines(postCmds),
	}

	data, err := config.Marshal(cfg)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}

	fmt.Printf("Wrote %s\n", path)
	return nil
}

func splitCommaList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func splitNonEmptyLines(s string) []string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func promptString(reader *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptInt(reader *bufio.Reader, label string, def int) int {
	fmt.Printf("%s [%d]: ", label, def)
	line, err := reader.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	var val int
	_, err = fmt.Sscanf(line, "%d", &val)
	if err != nil {
		return def
	}
	return val
}

func promptBool(reader *bufio.Reader, label string, def bool) bool {
	defStr := "false"
	if def {
		defStr = "true"
	}
	fmt.Printf("%s [%s]: ", label, defStr)
	line, err := reader.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return def
	}
	return line == "true" || line == "1" || line == "yes" || line == "y"
}
