package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"

	"github.com/skskuzan/rd-linkforge/internal/config"
)

func runConfig(
	ctx context.Context,
	args []string,
	env func(string) (string, bool),
	stdout, stderr io.Writer,
	logger *slog.Logger,
) int {
	flags := flag.NewFlagSet("config", flag.ContinueOnError)
	flags.SetOutput(stderr)

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: linkctl config")
		return 2
	}

	cfg, err := config.NewEnvLoader(env).Load(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "invalid configuration:\n%v\n", err)
		return 1
	}

	logger.Info("configuration loaded", "config", cfg)
	fmt.Fprintln(stdout, "config ok")

	return 0
}
