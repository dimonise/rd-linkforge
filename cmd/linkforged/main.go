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
	"time"

	"github.com/skskuzan/rd-linkforge/internal/config"
	"github.com/skskuzan/rd-linkforge/internal/link"
	"github.com/skskuzan/rd-linkforge/internal/linkio"
	"github.com/skskuzan/rd-linkforge/internal/store"
	"github.com/skskuzan/rd-linkforge/internal/transport"
	"github.com/skskuzan/rd-linkforge/internal/transport/tcp"
	"github.com/skskuzan/rd-linkforge/pkg/cache"
)

const shutdownTimeout = 15 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.LookupEnv, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, env func(string) (string, bool), stderr io.Writer) int {
	if env == nil {
		env = os.LookupEnv
	}

	flags := flag.NewFlagSet("linkforged", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", "", "")

	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(stderr, "usage: linkforged -db FILE")
		return 2
	}

	if *dbPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: linkforged -db FILE")
		return 2
	}

	levelName, ok := env("LF_LOG_LEVEL")
	if !ok || strings.TrimSpace(levelName) == "" {
		levelName = "info"
	}

	level, err := config.ParseLevel(levelName)
	if err != nil {
		fmt.Fprintf(stderr, "invalid LF_LOG_LEVEL: %v\n", err)
		fmt.Fprintln(stderr, "usage: linkforged -db FILE")
		return 2
	}

	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{
		Level: level,
	}))

	cfg, err := config.NewEnvLoader(env).Load(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "invalid configuration: %v\n", err)
		return 1
	}

	links, err := linkio.LoadFile(*dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "load database: %v\n", err)
		return 1
	}

	st := store.New()
	for _, l := range links {
		if _, err := st.Create(ctx, l); err != nil {
			fmt.Fprintf(stderr, "restore link %q: %v\n", l.Code, err)
			return 1
		}
	}

	lru, err := cache.NewLRU[string, link.Link](cfg.CacheSize)
	if err != nil {
		fmt.Fprintf(stderr, "create cache: %v\n", err)
		return 1
	}
	svc := link.NewService(st, link.WithCache(lru))

	var srv transport.Server = tcp.NewServer(
		cfg.TCPAddr,
		svc,
		tcp.WithLogger(logger),
	)

	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(stderr, "start resolver: %v\n", err)
		return 1
	}

	logger.Info("resolver started",
		"addr", srv.Addr(),
		"links", len(links),
	)

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown resolver", "error", err)
		return 1
	}

	logger.Info("resolver stopped")
	return 0
}
