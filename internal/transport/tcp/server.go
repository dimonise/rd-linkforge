package tcp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/link"
	"github.com/skskuzan/rd-linkforge/internal/transport"
)

var (
	ErrStarted     = errors.New("tcp: server was already started")
	errLineTooLong = errors.New("tcp: line too long")
)

// Resolver implementations must support concurrent Resolve calls.
type Resolver interface {
	Resolve(ctx context.Context, code string) (link.Link, error)
}

// Option configures a server during construction and is not concurrency-safe.
type Option func(*Server)

// Server is safe for concurrent use.
type Server struct {
	addr        string
	actualAddr  string
	resolver    Resolver
	idleTimeout time.Duration
	logger      *slog.Logger
	listener    net.Listener
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	wg          sync.WaitGroup
	conns       map[net.Conn]bool
	started     bool
	stopping    bool
}

var _ transport.Server = (*Server)(nil)

func WithIdleTimeout(d time.Duration) Option {
	return func(s *Server) {
		if d > 0 {
			s.idleTimeout = d
		}
	}
}

func WithLogger(l *slog.Logger) Option {
	return func(s *Server) {
		if l != nil {
			s.logger = l
		}
	}
}

func NewServer(addr string, r Resolver, opts ...Option) *Server {
	s := &Server{
		addr:        addr,
		resolver:    r,
		idleTimeout: 60 * time.Second,
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		conns:       make(map[net.Conn]bool),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started || s.stopping {
		return ErrStarted
	}

	lc := net.ListenConfig{KeepAlive: 30 * time.Second}
	ln, err := lc.Listen(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.addr, err)
	}

	s.listener = ln
	s.actualAddr = ln.Addr().String()
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.started = true

	s.wg.Add(1)
	go s.accept()

	return nil
}

func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return s.actualAddr
	}
	return s.addr
}

func (s *Server) accept() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			stopping := s.stopping
			s.mu.Unlock()
			if !stopping {
				s.logger.Error("accept connection", "error", err)
			}
			return
		}

		s.mu.Lock()
		if s.stopping {
			s.mu.Unlock()
			_ = conn.Close()
			continue
		}

		s.conns[conn] = false
		s.wg.Add(1)
		s.mu.Unlock()

		go s.serve(conn)
	}
}

func scanLine(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, bytes.TrimSuffix(data[:i], []byte{'\r'}), nil
	}
	if len(data) >= 4096 {
		return 0, nil, errLineTooLong
	}
	if atEOF {
		return 0, nil, nil
	}
	return 0, nil, nil
}

func (s *Server) serve(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
		s.wg.Done()
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 4096)
	scanner.Split(scanLine)

	for {
		s.mu.Lock()
		if s.stopping {
			s.mu.Unlock()
			return
		}
		err := conn.SetReadDeadline(time.Now().Add(s.idleTimeout))
		s.mu.Unlock()
		if err != nil {
			s.logger.Error("set read deadline", "error", err)
			return
		}

		if !scanner.Scan() {
			if errors.Is(scanner.Err(), errLineTooLong) {
				if _, err := io.WriteString(conn, "ERR line_too_long\n"); err != nil {
					s.logger.Error("write response", "error", err)
				}
			}
			return
		}

		s.mu.Lock()
		s.conns[conn] = true
		s.mu.Unlock()

		response, quit := s.execute(scanner.Text())
		if quit {
			return
		}

		if _, err := io.WriteString(conn, response+"\n"); err != nil {
			s.logger.Error("write response", "error", err)
			return
		}

		s.mu.Lock()
		s.conns[conn] = false
		stopping := s.stopping
		s.mu.Unlock()

		if stopping {
			return
		}
	}
}

func (s *Server) execute(line string) (string, bool) {
	fields := strings.Fields(line)

	if len(fields) == 1 {
		switch fields[0] {
		case "PING":
			return "PONG", false
		case "QUIT":
			return "", true
		}
	}

	if len(fields) == 2 && fields[0] == "RESOLVE" {
		result, err := s.resolver.Resolve(s.ctx, fields[1])
		if err != nil {
			if errors.Is(err, link.ErrNotFound) || errors.Is(err, link.ErrExpired) {
				return "ERR not_found", false
			}
			s.logger.Error("resolve link", "error", err)
			return "ERR internal", false
		}
		return "OK " + result.Target, false
	}

	return "ERR bad_request", false
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.stopping = true
	ln := s.listener
	s.mu.Unlock()

	if ln == nil {
		return nil
	}

	if err := ln.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		s.logger.Error("close listener", "error", err)
	}

	s.mu.Lock()
	for conn, busy := range s.conns {
		if !busy {
			_ = conn.SetReadDeadline(time.Now())
		}
	}
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.cancel()
		return nil
	case <-ctx.Done():
		s.mu.Lock()
		for conn := range s.conns {
			_ = conn.Close()
		}
		s.mu.Unlock()
		s.cancel()
		return ctx.Err()
	}
}
