package entrypoint

import (
	"context"
	"errors"

	"go_scrap/internal/app"
	"go_scrap/internal/cli"
	"go_scrap/internal/subcommands/inspect"
	"go_scrap/internal/subcommands/testconfigs"
	"go_scrap/internal/tui"
)

func Execute(args []string) (int, error) {
	if len(args) > 1 {
		switch args[1] {
		case "inspect":
			return 0, inspect.Run(args[2:])
		case "test-configs":
			return 0, testconfigs.Run(args[2:])
		}
	}

	if len(args) == 1 {
		res, err := tui.Run()
		if err != nil {
			return 1, err
		}
		if !res.RunNow {
			return 0, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), res.Options.Timeout)
		defer cancel()
		return 0, app.Run(ctx, res.Options)
	}

	opts, initConfig, err := cli.ParseArgs(args[1:])
	if err != nil {
		var exitErr cli.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.Code, exitErr.Err
		}
		return 1, err
	}

	if initConfig {
		return 0, cli.RunConfigWizard()
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()
	return 0, app.Run(ctx, opts)
}
