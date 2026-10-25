package main

import (
	"context"
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
	if len(args) != 0 {
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
