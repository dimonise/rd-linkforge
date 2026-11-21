// Command linkctl is the Linkforge operator CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/skskuzan/rd-linkforge/internal/config"
)

// version is overwritten at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	code := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr)

	stop()
	os.Exit(code)
}

func run(
	ctx context.Context,
	args []string,
	env func(string) (string, bool),
	stdout, stderr io.Writer,
) int {
	level := slog.LevelInfo

	if value, ok := env("LF_LOG_LEVEL"); ok && strings.TrimSpace(value) != "" {
		parsed, err := config.ParseLevel(value)
		if err != nil {
			fmt.Fprintf(stderr, "LF_LOG_LEVEL: %v\n", err)
			return 2
		}

		level = parsed
	}

	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{
		Level: level,
	}))

	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "import":
		return runImport(ctx, args[1:], stdout, stderr, logger)

	case "export":
		return runExport(ctx, args[1:], stdout, stderr)

	case "config":
		return runConfig(ctx, args[1:], env, stdout, stderr, logger)

	case "version":
		flags := flag.NewFlagSet("version", flag.ContinueOnError)
		flags.SetOutput(stderr)

		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		if flags.NArg() != 0 {
			printUsage(stderr)
			return 2
		}

		fmt.Fprintf(stdout, "linkctl %s\n", version)
		return 0
	case "resolve":
		return runResolve(ctx, args[1:], stdout, stderr)

	default:
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: linkctl <import|export|config|version|resolve>")
}
