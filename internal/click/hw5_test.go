package click

import (
 "context"
 "errors"
 "sync"
 "sync/atomic"
 "testing"
 "time"
)

// hw5Store records every batch. While block is not nil, Append waits for it
// to be closed (or for its context to end).
type hw5Store struct {
 mu       sync.Mutex
 batches  [][]Click
 block    chan struct{}
 failErr  error
 appended chan struct{} // receives one value per successful Append
}

func newHW5Store() *hw5Store { return &hw5Store{appended: make(chan struct{}, 1000)} }

func (s *hw5Store) Append(ctx context.Context, batch []Click) error {
 if s.block != nil {
  select {
  case <-s.block:
  case <-ctx.Done():
   return ctx.Err()
  }
 }
 if s.failErr != nil {
  return s.failErr
 }
 s.mu.Lock()
 s.batches = append(s.batches, batch)
 s.mu.Unlock()
 s.appended <- struct{}{}
 return nil
}

func (s *hw5Store) Range(context.Context, string, Window) ([]Click, error) { return nil, nil }

func (s *hw5Store) total() int {
 s.mu.Lock()
 defer s.mu.Unlock()
 n := 0
 for _, b := range s.batches {
  n += len(b)
 }
 return n
}

func (s *hw5Store) sizes() []int {
 s.mu.Lock()
 defer s.mu.Unlock()
 var out []int
 for _, b := range s.batches {
  out = append(out, len(b))
 }
 return out
}

type hw5Metrics struct {
 dropped atomic.Int64
 mu      sync.Mutex
 depths  []int
}

func (m *hw5Metrics) ObserveClickDropped() { m.dropped.Add(1) }
func (m *hw5Metrics) SetQueueDepth(n int) {
 m.mu.Lock()
 m.depths = append(m.depths, n)
 m.mu.Unlock()
}

type hw5Sink struct {
 mu     sync.Mutex
 calls  int
 clicks int
}

func (s *hw5Sink) Add(batch []Click) {
 s.mu.Lock()
 s.calls++
 s.clicks += len(batch)
 s.mu.Unlock()
}

func hw5Click(code string) Click {
 return Click{Code: code, At: time.Date(2030, 1, 2, 10, 0, 0, 0, time.UTC)}
}

func hw5Shutdown(t *testing.T, p *Pipeline) {
 t.Helper()
 ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)