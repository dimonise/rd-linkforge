package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/skskuzan/rd-linkforge/internal/linkio"
)

func runExport(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
) int {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(stderr)

	dbPath := flags.String("db", "", "database file")
	outPath := flags.String("out", "", "CSV output file")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *dbPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: linkctl export -db FILE [-out FILE]")
		return 2
	}

	if err := ctx.Err(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	links, err := linkio.LoadFile(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "load database %s: %v\n", *dbPath, err)
		return 1
	}

	if err := ctx.Err(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if *outPath == "" {
		if err := linkio.WriteCSV(stdout, links); err != nil {
			fmt.Fprintf(stderr, "export CSV: %v\n", err)
			return 1
		}

		return 0
	}

	file, err := os.Create(*outPath)
	if err != nil {
		fmt.Fprintf(stderr, "create output file %s: %v\n", *outPath, err)
		return 1
	}

	writeErr := linkio.WriteCSV(file, links)
	closeErr := file.Close()

	if err := errors.Join(writeErr, closeErr); err != nil {
		fmt.Fprintf(stderr, "export CSV to %s: %v\n", *outPath, err)
		return 1
	}

	return 0
}
