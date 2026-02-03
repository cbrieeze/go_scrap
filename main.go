package main

import (
	"context"
	"fmt"
	"os"

	"go_scrap/internal/app"
	"go_scrap/internal/cli"
	"go_scrap/internal/tui"
)

func main() {
	// Subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "inspect":
			runInspect(os.Args[2:])
			return
		case "test-configs":
			runTestConfigs(os.Args[2:])
			return
		}
	}

	// If no CLI args are provided, launch the TUI and run using its settings.
	if len(os.Args) == 1 {
		res, err := tui.Run()
		if err != nil {
			fatal(err)
		}
		if !res.RunNow {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), res.Options.Timeout)
		defer cancel()
		if err := app.Run(ctx, res.Options); err != nil {
			fatal(err)
		}
		return
	}

	opts, initConfig, err := cli.ParseArgs(os.Args[1:])
	if err != nil {
		if exitErr, ok := err.(cli.ExitError); ok {
			fmt.Fprintln(os.Stderr, exitErr.Error())
			os.Exit(exitErr.Code)
		}
		fatal(err)
	}

	if initConfig {
		if err := cli.RunConfigWizard(); err != nil {
			fatal(err)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	if err := app.Run(ctx, opts); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
