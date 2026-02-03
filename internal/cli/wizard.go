package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"go_scrap/internal/config"
)

func RunConfigWizard() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Config wizard (press Enter to accept defaults)")

	path := promptString(reader, "Config file path", "config.json")
	urlStr := promptString(reader, "URL", "")
	mode := promptString(reader, "Mode (auto|static|dynamic)", "dynamic")
	outputDir := promptString(reader, "Output dir (optional)", "")
	timeout := promptInt(reader, "Timeout seconds", 60)
	waitFor := promptString(reader, "Wait for selector", "body")
	headless := promptBool(reader, "Headless (true/false)", true)
	navSel := promptString(reader, "Nav selector (optional)", "")
	contentSel := promptString(reader, "Content selector (optional)", "")

	cfg := config.Config{
		URL:             strings.TrimSpace(urlStr),
		Mode:            mode,
		OutputDir:       strings.TrimSpace(outputDir),
		TimeoutSeconds:  timeout,
		WaitForSelector: waitFor,
		Headless:        &headless,
		NavSelector:     navSel,
		ContentSelector: contentSel,
	}

	data, err := config.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}

	fmt.Printf("Wrote %s\n", path)
	return nil
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
