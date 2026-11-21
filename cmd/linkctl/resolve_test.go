package main

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/link"
	"github.com/skskuzan/rd-linkforge/internal/transport/tcp"
)

type testResolver struct {
	links map[string]link.Link
}

func (r *testResolver) Resolve(ctx context.Context, code string) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}

	l, ok := r.links[code]
	if !ok {
		return link.Link{}, link.ErrNotFound
	}

	return l, nil
}

func startTestResolver(t *testing.T) *tcp.Server {
	t.Helper()

	srv := tcp.NewServer("127.0.0.1:0", &testResolver{
		links: map[string]link.Link{
			"abc": {
				Code:   "abc",
				Target: "https://example.com",
			},
		},
	})

	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})

	return srv
}

func TestRunResolve(t *testing.T) {
	srv := startTestResolver(t)

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "known code",
			args:       []string{"resolve", "-addr", srv.Addr(), "abc"},
			wantCode:   0,
			wantStdout: "https://example.com\n",
		},
		{
			name:       "unknown code",
			args:       []string{"resolve", "-addr", srv.Addr(), "missing"},
			wantCode:   1,
			wantStderr: "resolver answered: ERR not_found",
		},
		{
			name:       "missing code",
			args:       []string{"resolve", "-addr", srv.Addr()},
			wantCode:   2,
			wantStderr: "usage:",
		},
		{
			name:       "extra argument",
			args:       []string{"resolve", "-addr", srv.Addr(), "abc", "extra"},
			wantCode:   2,
			wantStderr: "usage:",
		},
		{
			name:       "empty code",
			args:       []string{"resolve", "-addr", srv.Addr(), ""},
			wantCode:   2,
			wantStderr: "usage:",
		},
		{
			name:       "space in code",
			args:       []string{"resolve", "-addr", srv.Addr(), "a b"},
			wantCode:   2,
			wantStderr: "usage:",
		},
		{
			name:       "newline in code",
			args:       []string{"resolve", "-addr", srv.Addr(), "a\nb"},
			wantCode:   2,
			wantStderr: "usage:",
		},
		{
			name:       "unknown flag",
			args:       []string{"resolve", "-unknown"},
			wantCode:   2,
			wantStderr: "flag provided but not defined",
		},
		{
			name:       "missing address value",
			args:       []string{"resolve", "-addr"},
			wantCode:   2,
			wantStderr: "flag needs an argument",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run(
				context.Background(),
				tc.args,
				testLookup(nil),
				&stdout,
				&stderr,
			)

			if code != tc.wantCode {
				t.Errorf("code = %d, want %d; stderr = %s",
					code, tc.wantCode, stderr.String())
			}

			if stdout.String() != tc.wantStdout {
				t.Errorf("stdout = %q, want %q",
					stdout.String(), tc.wantStdout)
			}

			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want %q",
					stderr.String(), tc.wantStderr)
			}
		})
	}
}

func TestRunResolveUnavailable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	code := run(
		context.Background(),
		[]string{"resolve", "-addr", addr, "abc"},
		testLookup(nil),
		&stdout,
		&stderr,
	)

	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "connect resolver:") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunResolveTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	acceptErrors := make(chan error, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			acceptErrors <- err
			return
		}
		accepted <- conn
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var stdout, stderr bytes.Buffer
	started := time.Now()

	code := run(
		ctx,
		[]string{"resolve", "-addr", ln.Addr().String(), "abc"},
		testLookup(nil),
		&stdout,
		&stderr,
	)

	elapsed := time.Since(started)

	select {
	case conn := <-accepted:
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	case err := <-acceptErrors:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("connection was not accepted")
	}

	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}

	if elapsed > time.Second {
		t.Errorf("timeout took %v", elapsed)
	}

	if !strings.Contains(stderr.String(), "read response:") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunUsageListsResolve(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(
		context.Background(),
		nil,
		testLookup(nil),
		&stdout,
		&stderr,
	)

	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}

	if !strings.Contains(stderr.String(), "resolve") {
		t.Errorf("usage does not mention resolve: %s", stderr.String())
	}
}
