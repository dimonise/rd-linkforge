package config

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func testEnv(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"LF_PG_DSN":     "postgres://localhost/db",
		"LF_MONGO_URI":  "mongodb://localhost:27017",
		"LF_JWT_SECRET": strings.Repeat("x", 32),
	}
}

func TestEnvLoaderDefaults(t *testing.T) {
	loader := NewEnvLoader(testEnv(validEnv()))

	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddr != ":8080" ||
		cfg.AdminAddr != "127.0.0.1:6060" ||
		cfg.TCPAddr != ":7070" ||
		cfg.CacheSize != 100000 ||
		cfg.LogLevel != slog.LevelInfo {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}

func TestEnvLoaderValidation(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr error
	}{
		{"missing DSN", "LF_PG_DSN", "", ErrMissing},
		{"missing Mongo URI", "LF_MONGO_URI", " ", ErrMissing},
		{"short JWT secret", "LF_JWT_SECRET", "short", ErrInvalid},
		{"invalid HTTP port", "LF_HTTP_ADDR", ":abc", ErrInvalid},
		{"port too large", "LF_TCP_ADDR", ":65536", ErrInvalid},
		{"invalid cache size", "LF_CACHE_SIZE", "abc", ErrInvalid},
		{"zero cache size", "LF_CACHE_SIZE", "0", ErrInvalid},
		{"invalid log level", "LF_LOG_LEVEL", "verbose", ErrInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnv()
			env[tt.key] = tt.value

			loader := NewEnvLoader(testEnv(env))
			cfg, err := loader.Load(context.Background())

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Load() error = %v, want %v", err, tt.wantErr)
			}

			if cfg != (Config{}) {
				t.Errorf("Load() returned non-zero config: %+v", cfg)
			}
		})
	}
}

func TestEnvLoaderCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	loader := NewEnvLoader(testEnv(validEnv()))

	_, err := loader.Load(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Load() error = %v, want context.Canceled", err)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{" INFO ", slog.LevelInfo},
		{"Warn", slog.LevelWarn},
		{"ERROR", slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLevel(tt.input)
			if err != nil {
				t.Fatalf("ParseLevel() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}

	_, err := ParseLevel("trace")
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("ParseLevel() error = %v, want ErrInvalid", err)
	}
}

func TestEnvLoaderMultipleErrors(t *testing.T) {
	loader := NewEnvLoader(testEnv(nil))

	cfg, err := loader.Load(context.Background())

	if !errors.Is(err, ErrMissing) {
		t.Fatalf("Load() error = %v, want ErrMissing", err)
	}

	for _, key := range []string{"LF_PG_DSN", "LF_MONGO_URI", "LF_JWT_SECRET"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not contain %s", key)
		}
	}

	if cfg != (Config{}) {
		t.Errorf("expected zero Config, got %+v", cfg)
	}
}

func TestEnvLoaderOverrides(t *testing.T) {
	env := validEnv()

	env["LF_HTTP_ADDR"] = " :9090 "
	env["LF_ADMIN_ADDR"] = " localhost:6061 "
	env["LF_TCP_ADDR"] = " :7071 "
	env["LF_CACHE_SIZE"] = " 500 "
	env["LF_LOG_LEVEL"] = " DEBUG "

	loader := NewEnvLoader(testEnv(env))
	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddr != ":9090" ||
		cfg.AdminAddr != "localhost:6061" ||
		cfg.TCPAddr != ":7071" ||
		cfg.CacheSize != 500 ||
		cfg.LogLevel != slog.LevelDebug {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestConfigLogValue(t *testing.T) {
	env := validEnv()

	env["LF_PG_DSN"] = "postgres-secret"
	env["LF_MONGO_URI"] = "mongo-secret"
	env["LF_JWT_SECRET"] = strings.Repeat("jwt-secret", 4)

	loader := NewEnvLoader(testEnv(env))
	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	logger.Info("configuration loaded", "config", cfg)

	logText := output.String()

	for _, secret := range []string{
		cfg.PGDSN,
		cfg.MongoURI,
		cfg.JWTSecret,
	} {
		if strings.Contains(logText, secret) {
			t.Errorf("secret leaked into log")
		}
	}

	if !strings.Contains(logText, "[redacted]") {
		t.Error("expected redacted credentials")
	}
}
