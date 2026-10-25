package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/skskuzan/rd-linkforge/internal/linkio"
)

func testLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(
		context.Background(),
		[]string{"version"},
		testLookup(nil),
		&stdout,
		&stderr,
	)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %s", code, stderr.String())
	}

	if stdout.String() != "linkctl "+version+"\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRunImportExport(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "urls.txt")
	db := filepath.Join(dir, "links.jsonl")

	writeTestFile(t, input, "https://example.com\nhttps://go.dev\n")

	var stdout, stderr bytes.Buffer

	code := run(
		context.Background(),
		[]string{"import", "-in", input, "-db", db},
		testLookup(nil),
		&stdout,
		&stderr,
	)

	if code != 0 {
		t.Fatalf("import exit code = %d; stderr = %s", code, stderr.String())
	}

	if stdout.String() != "imported: 2\nskipped: 0\n" {
		t.Errorf("import stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()

	code = run(
		context.Background(),
		[]string{"export", "-db", db},
		testLookup(nil),
		&stdout,
		&stderr,
	)

	if code != 0 {
		t.Fatalf("export exit code = %d; stderr = %s", code, stderr.String())
	}

	csv := stdout.String()

	if !strings.HasPrefix(csv, "id,code,target,owner_id,created_at,expires_at\n") {
		t.Errorf("unexpected CSV header: %s", csv)
	}

	if !strings.Contains(csv, "https://example.com") ||
		!strings.Contains(csv, "https://go.dev") {
		t.Errorf("CSV missing links: %s", csv)
	}
}

func TestRunSecondImport(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "urls.txt")
	db := filepath.Join(dir, "links.jsonl")

	var stdout, stderr bytes.Buffer

	for _, url := range []string{
		"https://example.com",
		"https://go.dev",
	} {
		writeTestFile(t, input, url+"\n")

		stdout.Reset()
		stderr.Reset()

		code := run(
			context.Background(),
			[]string{"import", "-in", input, "-db", db},
			testLookup(nil),
			&stdout,
			&stderr,
		)

		if code != 0 {
			t.Fatalf("import exit code = %d; stderr = %s", code, stderr.String())
		}
	}

	links, err := linkio.LoadFile(db)
	if err != nil {
		t.Fatal(err)
	}

	if len(links) != 2 {
		t.Fatalf("links count = %d, want 2", len(links))
	}

	if links[0].ID != 1 || links[1].ID != 2 {
		t.Errorf("IDs = %d, %d; want 1, 2", links[0].ID, links[1].ID)
	}
}

func TestRunImportSkipped(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "urls.txt")
	db := filepath.Join(dir, "links.jsonl")

	writeTestFile(t, input, "https://example.com\nnot-a-url\n")

	var stdout, stderr bytes.Buffer

	code := run(
		context.Background(),
		[]string{"import", "-in", input, "-db", db},
		testLookup(nil),
		&stdout,
		&stderr,
	)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	if stdout.String() != "imported: 1\nskipped: 1\n" {
		t.Errorf("stdout = %q", stdout.String())
	}

	if !strings.Contains(stderr.String(), "line=2") {
		t.Errorf("missing line=2 warning: %s", stderr.String())
	}

	links, err := linkio.LoadFile(db)
	if err != nil {
		t.Fatal(err)
	}

	if len(links) != 1 {
		t.Errorf("saved links = %d, want 1", len(links))
	}
}

func TestRunCanceledImport(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "urls.txt")
	db := filepath.Join(dir, "links.jsonl")

	writeTestFile(t, input, "https://example.com\n")
	writeTestFile(t, db, "existing database content\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout, stderr bytes.Buffer

	code := run(
		ctx,
		[]string{"import", "-in", input, "-db", db},
		testLookup(nil),
		&stdout,
		&stderr,
	)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	data, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "existing database content\n" {
		t.Error("database changed after cancellation")
	}
}

func TestRunUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no command", nil},
		{"unknown command", []string{"unknown"}},
		{"import missing flags", []string{"import"}},
		{"import missing db", []string{"import", "-in", "urls.txt"}},
		{"export missing db", []string{"export"}},
		{"unknown import flag", []string{"import", "-unknown"}},
		{"unknown export flag", []string{"export", "-unknown"}},
		{"config extra argument", []string{"config", "extra"}},
		{"version extra argument", []string{"version", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run(
				context.Background(),
				tt.args,
				testLookup(nil),
				&stdout,
				&stderr,
			)

			if code != 2 {
				t.Errorf("exit code = %d, want 2; stderr = %s", code, stderr.String())
			}
		})
	}
}

func TestRunInvalidLogLevel(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(
		context.Background(),
		[]string{"version"},
		testLookup(map[string]string{
			"LF_LOG_LEVEL": "verbose",
		}),
		&stdout,
		&stderr,
	)

	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunConfig(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		wantCode int
	}{
		{
			name:     "missing credentials",
			env:      nil,
			wantCode: 1,
		},
		{
			name: "valid config",
			env: map[string]string{
				"LF_PG_DSN":     "postgres-secret",
				"LF_MONGO_URI":  "mongo-secret",
				"LF_JWT_SECRET": strings.Repeat("x", 32),
			},
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run(
				context.Background(),
				[]string{"config"},
				testLookup(tt.env),
				&stdout,
				&stderr,
			)

			if code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr = %s",
					code, tt.wantCode, stderr.String())
			}

			if tt.wantCode == 0 && stdout.String() != "config ok\n" {
				t.Errorf("stdout = %q", stdout.String())
			}

			if tt.wantCode == 1 &&
				!strings.Contains(stderr.String(), "invalid configuration:") {
				t.Errorf("stderr = %q", stderr.String())
			}

			for key, secret := range tt.env {
				if key != "LF_PG_DSN" &&
					key != "LF_MONGO_URI" &&
					key != "LF_JWT_SECRET" {
					continue
				}

				if strings.Contains(stderr.String(), secret) {
					t.Errorf("secret %s leaked into stderr", key)
				}
			}
		})
	}
}

func TestRunExportFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "urls.txt")
	db := filepath.Join(dir, "links.jsonl")
	output := filepath.Join(dir, "links.csv")

	writeTestFile(t, input, "https://example.com\n")

	var stdout, stderr bytes.Buffer

	if code := run(
		context.Background(),
		[]string{"import", "-in", input, "-db", db},
		testLookup(nil),
		&stdout,
		&stderr,
	); code != 0 {
		t.Fatalf("import failed: %s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()

	code := run(
		context.Background(),
		[]string{"export", "-db", db, "-out", output},
		testLookup(nil),
		&stdout,
		&stderr,
	)

	if code != 0 {
		t.Fatalf("export failed: %s", stderr.String())
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "https://example.com") {
		t.Errorf("unexpected CSV: %s", data)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", stdout.String())
	}
}

func TestRunExportMissingDatabase(t *testing.T) {
	db := filepath.Join(t.TempDir(), "missing.jsonl")

	var stdout, stderr bytes.Buffer

	code := run(
		context.Background(),
		[]string{"export", "-db", db},
		testLookup(nil),
		&stdout,
		&stderr,
	)

	if code != 1 {
		t.Errorf("exit code = %s, want 1", strconv.Itoa(code))
	}

	if !strings.Contains(stderr.String(), db) {
		t.Errorf("error should contain database path: %s", stderr.String())
	}
}
