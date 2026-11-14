package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/skskuzan/rd-linkforge/internal/link"
	"github.com/skskuzan/rd-linkforge/internal/linkio"
	"github.com/skskuzan/rd-linkforge/internal/store"
)

func runImport(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	logger *slog.Logger,
) int {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	flags.SetOutput(stderr)

	inPath := flags.String("in", "", "input file")
	dbPath := flags.String("db", "", "database file")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	if strings.TrimSpace(*inPath) == "" ||
		strings.TrimSpace(*dbPath) == "" ||
		flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: linkctl import -in FILE -db FILE")
		return 2
	}

	if err := ctx.Err(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	logger.Info("import started")

	input, err := os.Open(*inPath)
	if err != nil {
		fmt.Fprintf(stderr, "open input %s: %v\n", *inPath, err)
		return 1
	}

	targets, readErr := linkio.ReadTargets(input)
	closeErr := input.Close()

	if err := errors.Join(readErr, closeErr); err != nil {
		fmt.Fprintf(stderr, "read input %s: %v\n", *inPath, err)
		return 1
	}

	existing, err := linkio.LoadFile(*dbPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "load database %s: %v\n", *dbPath, err)
			return 1
		}
		existing = nil
	}

	if err := ctx.Err(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	st := store.New()
	var maxID uint64

	for _, l := range existing {
		if err := ctx.Err(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		if _, err := st.Create(ctx, l); err != nil {
			fmt.Fprintf(stderr, "restore database: %v\n", err)
			return 1
		}

		if l.ID > maxID {
			maxID = l.ID
		}
	}

	service := link.NewService(st, link.WithFirstID(maxID+1))

	imported := 0
	skipped := 0

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		created, err := service.Shorten(ctx, link.ShortenRequest{
			OwnerID: "cli",
			Target:  target.URL,
		})

		if err != nil {
			if ctx.Err() != nil {
				fmt.Fprintln(stderr, ctx.Err())
				return 1
			}

			skipped++
			logger.Warn("import skipped",
				"line", target.Line,
				"error", err,
			)
			continue
		}

		imported++
		logger.Debug("link imported",
			"line", target.Line,
			"id", created.ID,
			"code", created.Code,
		)
	}

	if err := ctx.Err(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if err := linkio.SaveFile(*dbPath, st.All()); err != nil {
		fmt.Fprintf(stderr, "save database %s: %v\n", *dbPath, err)
		return 1
	}

	logger.Info("import finished",
		"imported", imported,
		"skipped", skipped,
	)

	fmt.Fprintf(stdout, "imported: %d\nskipped: %d\n", imported, skipped)

	if skipped > 0 {
		return 1
	}

	return 0
}
