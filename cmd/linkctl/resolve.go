package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
	"unicode"
)

const resolveTimeout = 5 * time.Second

func runResolve(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("resolve", flag.ContinueOnError)
	flags.SetOutput(stderr)

	addr := flags.String("addr", "127.0.0.1:7070", "")

	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(stderr, "usage: linkctl resolve [-addr HOST:PORT] CODE")
		return 2
	}

	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: linkctl resolve [-addr HOST:PORT] CODE")
		return 2
	}

	code := flags.Arg(0)
	if code == "" || strings.IndexFunc(code, unicode.IsSpace) >= 0 {
		fmt.Fprintln(stderr, "usage: linkctl resolve [-addr HOST:PORT] CODE")
		return 2
	}

	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", *addr)
	if err != nil {
		fmt.Fprintf(stderr, "connect resolver: %v\n", err)
		return 1
	}
	defer conn.Close()

	stop := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
	})
	defer stop()

	if _, err := io.WriteString(conn, "RESOLVE "+code+"\n"); err != nil {
		fmt.Fprintf(stderr, "send request: %v\n", err)
		return 1
	}

	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Fprintf(stderr, "read response: %v\n", err)
		return 1
	}

	response = strings.TrimSuffix(response, "\n")
	response = strings.TrimSuffix(response, "\r")

	if strings.HasPrefix(response, "OK ") {
		fmt.Fprintln(stdout, strings.TrimPrefix(response, "OK "))
		return 0
	}

	fmt.Fprintf(stderr, "resolver answered: %s\n", response)
	return 1
}
