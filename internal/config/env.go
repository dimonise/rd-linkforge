package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

var (
	ErrMissing = errors.New("config: required variable is not set")
	ErrInvalid = errors.New("config: invalid value")
)

type EnvLoader struct {
	lookup func(string) (string, bool)
}

func NewEnvLoader(lookup func(string) (string, bool)) *EnvLoader {
	if lookup == nil {
		lookup = os.LookupEnv
	}

	return &EnvLoader{lookup: lookup}
}

var _ Loader = (*EnvLoader)(nil)

func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("log level: %w", ErrInvalid)
	}
}

func (l *EnvLoader) Load(ctx context.Context) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, err
	}

	var cfg Config
	var problems []error

	get := func(key, fallback string) string {
		value, ok := l.lookup(key)
		value = strings.TrimSpace(value)

		if !ok || value == "" {
			return fallback
		}

		return value
	}

	checkAddr := func(key, value string) {
		_, portStr, err := net.SplitHostPort(value)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", key, ErrInvalid))
			return
		}

		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			problems = append(problems, fmt.Errorf("%s: %w", key, ErrInvalid))
		}
	}

	cfg.HTTPAddr = get("LF_HTTP_ADDR", ":8080")
	cfg.AdminAddr = get("LF_ADMIN_ADDR", "127.0.0.1:6060")
	cfg.TCPAddr = get("LF_TCP_ADDR", ":7070")

	checkAddr("LF_HTTP_ADDR", cfg.HTTPAddr)
	checkAddr("LF_ADMIN_ADDR", cfg.AdminAddr)
	checkAddr("LF_TCP_ADDR", cfg.TCPAddr)

	required := func(key string) string {
		value := get(key, "")
		if value == "" {
			problems = append(problems, fmt.Errorf("%s: %w", key, ErrMissing))
		}
		return value
	}

	cfg.PGDSN = required("LF_PG_DSN")
	cfg.MongoURI = required("LF_MONGO_URI")
	cfg.JWTSecret = required("LF_JWT_SECRET")

	if cfg.JWTSecret != "" && len(cfg.JWTSecret) < 32 {
		problems = append(problems,
			fmt.Errorf("LF_JWT_SECRET: %w", ErrInvalid))
	}

	cacheStr := get("LF_CACHE_SIZE", "100000")
	cacheSize, err := strconv.Atoi(cacheStr)
	if err != nil || cacheSize <= 0 {
		problems = append(problems,
			fmt.Errorf("LF_CACHE_SIZE: %w", ErrInvalid))
	} else {
		cfg.CacheSize = cacheSize
	}

	levelStr := get("LF_LOG_LEVEL", "info")
	level, err := ParseLevel(levelStr)
	if err != nil {
		problems = append(problems,
			fmt.Errorf("LF_LOG_LEVEL: %w", err))
	} else {
		cfg.LogLevel = level
	}

	if len(problems) > 0 {
		return Config{}, errors.Join(problems...)
	}

	return cfg, nil
}
