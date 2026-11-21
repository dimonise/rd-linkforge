package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/link"
	"github.com/skskuzan/rd-linkforge/internal/linkio"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func testEnv(t *testing.T, overrides map[string]string) func(string) (string, bool) {
	t.Helper()

	values := map[string]string{
		"LF_PG_DSN":     "unused",
		"LF_MONGO_URI":  "unused",
		"LF_JWT_SECRET": testSecret,
		"LF_TCP_ADDR":   "127.0.0.1:7070",
	}

	for key, value := range overrides {
		values[key] = value
	}

	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func testDatabase(t *testing.T, links []link.Link) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "links.jsonl")
	if err := linkio.SaveFile(path, links); err != nil {
		t.Fatal(err)
	}
	return path
}

func testFreeAddress(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func TestRunFailures(t *testing.T) {
	db := testDatabase(t, []link.Link{
		{ID: 1, Code: "000001", Target: "https://example.com"},
	})

	tests := []struct {
		name      string
		args      []string
		overrides map[string]string
		wantCode  int
		wantText  string
	}{
		{
			name:     "missing db",
			wantCode: 2,
			wantText: "usage:",
		},
		{
			name:     "unknown flag",
			args:     []string{"-unknown"},
			wantCode: 2,
			wantText: "usage:",
		},
		{
			name:     "extra argument",
			args:     []string{"-db", db, "extra"},
			wantCode: 2,
			wantText: "usage:",
		},
		{
			name:      "invalid log level",
			args:      []string{"-db", db},
			overrides: map[string]string{"LF_LOG_LEVEL": "invalid"},
			wantCode:  2,
			wantText:  "invalid LF_LOG_LEVEL",
		},
		{
			name:      "invalid configuration",
			args:      []string{"-db", db},
			overrides: map[string]string{"LF_PG_DSN": ""},
			wantCode:  1,
			wantText:  "invalid configuration:",
		},
		{
			name:     "missing database",
			args:     []string{"-db", filepath.Join(t.TempDir(), "missing.jsonl")},
			wantCode: 1,
			wantText: "load database:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer

			code := run(
				context.Background(),
				tc.args,
				testEnv(t, tc.overrides),
				&stderr,
			)

			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d", code, tc.wantCode)
			}
			if !strings.Contains(stderr.String(), tc.wantText) {
				t.Errorf("stderr = %q, want %q", stderr.String(), tc.wantText)
			}
			if strings.Contains(stderr.String(), testSecret) {
				t.Error("credentials leaked")
			}
		})
	}

	t.Run("duplicate code", func(t *testing.T) {
		path := testDatabase(t, []link.Link{
			{ID: 1, Code: "abc", Target: "https://example.com"},
			{ID: 2, Code: "abc", Target: "https://go.dev"},
		})

		var stderr bytes.Buffer
		code := run(context.Background(), []string{"-db", path}, testEnv(t, nil), &stderr)

		if code != 1 || !strings.Contains(stderr.String(), "restore link") {
			t.Errorf("code = %d, stderr = %q", code, stderr.String())
		}
	})

	t.Run("busy address", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()

		var stderr bytes.Buffer
		env := testEnv(t, map[string]string{
			"LF_TCP_ADDR": ln.Addr().String(),
		})

		code := run(context.Background(), []string{"-db", db}, env, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), "start resolver") {
			t.Errorf("code = %d, stderr = %q", code, stderr.String())
		}
	})
}

type observedLog struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	once    sync.Once
	started chan struct{}
}

func (w *observedLog) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.buffer.Write(p)
	if bytes.Contains(w.buffer.Bytes(), []byte(`"msg":"resolver started"`)) {
		w.once.Do(func() {
			close(w.started)
		})
	}
	return n, err
}

func (w *observedLog) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

func testStartedAddress(t *testing.T, logs string) string {
	t.Helper()

	for _, line := range strings.Split(logs, "\n") {
		var entry struct {
			Msg  string `json:"msg"`
			Addr string `json:"addr"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err == nil &&
			entry.Msg == "resolver started" {
			return entry.Addr
		}
	}

	t.Fatal("resolver started log not found")
	return ""
}

func TestRunLifecycle(t *testing.T) {
	now := time.Now().UTC()

	db := testDatabase(t, []link.Link{
		{
			ID:        1,
			Code:      "000001",
			Target:    "https://example.com",
			OwnerID:   "test",
			CreatedAt: now,
		},
		{
			ID:        2,
			Code:      "old",
			Target:    "https://old.example",
			OwnerID:   "test",
			CreatedAt: now.Add(-2 * time.Hour),
			ExpiresAt: now.Add(-time.Hour),
		},
	})

	addr := testFreeAddress(t)
	env := testEnv(t, map[string]string{"LF_TCP_ADDR": addr})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logs := &observedLog{started: make(chan struct{})}
	done := make(chan int, 1)

	go func() {
		done <- run(ctx, []string{"-db", db}, env, logs)
	}()

	select {
	case <-logs.started:
	case code := <-done:
		t.Fatalf("server exited early: %d, logs: %s", code, logs.String())
	case <-time.After(5 * time.Second):
		t.Fatal("server startup timeout")
	}

	addr = testStartedAddress(t, logs.String())

	queries := []struct {
		request string
		want    string
	}{
		{"RESOLVE 000001", "OK https://example.com\n"},
		{"RESOLVE 000001", "OK https://example.com\n"},
		{"RESOLVE missing", "ERR not_found\n"},
		{"RESOLVE old", "ERR not_found\n"},
		{"PING", "PONG\n"},
	}

	var wg sync.WaitGroup
	failures := make(chan error, 8)

	for i := 0; i < 8; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			conn, err := net.DialTimeout("tcp", addr, time.Second)
			if err != nil {
				failures <- err
				return
			}
			defer conn.Close()

			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				failures <- err
				return
			}

			reader := bufio.NewReader(conn)

			for _, query := range queries {
				if _, err := io.WriteString(conn, query.request+"\n"); err != nil {
					failures <- err
					return
				}

				response, err := reader.ReadString('\n')
				if err != nil {
					failures <- err
					return
				}

				if response != query.want {
					failures <- fmt.Errorf("%q: got %q, want %q",
						query.request, response, query.want)
					return
				}
			}
		}()
	}

	wg.Wait()
	close(failures)

	for err := range failures {
		t.Error(err)
	}

	idle, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close()

	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("exit code = %d, logs = %s", code, logs.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timeout")
	}

	if !strings.Contains(logs.String(), "resolver stopped") {
		t.Error("resolver stopped log not found")
	}

	if err := idle.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := bufio.NewReader(idle).ReadString('\n'); err == nil {
		t.Error("idle connection remained open")
	}

	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Error("server still accepts connections")
	}
}
