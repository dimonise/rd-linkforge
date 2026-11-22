package click

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// Metrics records Pipeline statistics.
// Implementations must be safe for concurrent calls.
type Metrics interface {
	ObserveClickDropped()
	SetQueueDepth(n int)
}

// BatchSink accepts batches emitted by Pipeline workers.
// Implementations must be safe for concurrent calls.
type BatchSink interface {
	Add(batch []Click)
}

// PipelineOption configures a Pipeline during construction.
// Options must not be applied concurrently to the same Pipeline.
type PipelineOption func(*Pipeline)

// Pipeline is an Ingester that queues clicks and stores them in batches
// using worker goroutines. It is safe for concurrent use.
type Pipeline struct {
	store         Store
	metrics       Metrics
	sink          BatchSink
	queue         chan Click
	workers       int
	batchSize     int
	flushInterval time.Duration

	mu      sync.RWMutex
	started bool
	closed  bool
	wg      sync.WaitGroup
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
}

var _ Ingester = (*Pipeline)(nil)

func WithQueueSize(n int) PipelineOption {
	return func(p *Pipeline) {
		if n > 0 {
			p.queue = make(chan Click, n)
		}
	}
}

func WithWorkers(n int) PipelineOption {
	return func(p *Pipeline) {
		if n > 0 {
			p.workers = n
		}
	}
}

func WithBatchSize(n int) PipelineOption {
	return func(p *Pipeline) {
		if n > 0 {
			p.batchSize = n
		}
	}
}

func WithFlushInterval(d time.Duration) PipelineOption {
	return func(p *Pipeline) {
		if d > 0 {
			p.flushInterval = d
		}
	}
}

func WithMetrics(m Metrics) PipelineOption {
	return func(p *Pipeline) {
		if m != nil {
			p.metrics = m
		}
	}
}

func WithSink(s BatchSink) PipelineOption {
	return func(p *Pipeline) {
		p.sink = s
	}
}

func NewPipeline(store Store, opts ...PipelineOption) *Pipeline {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Pipeline{
		store:         store,
		queue:         make(chan Click, 10000),
		workers:       runtime.GOMAXPROCS(0),
		batchSize:     500,
		flushInterval: time.Second,
		done:          make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *Pipeline) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started || p.closed {
		return
	}

	p.started = true
	p.wg.Add(p.workers)

	for i := 0; i < p.workers; i++ {
		go p.worker()
	}

	go func() {
		p.wg.Wait()
		close(p.done)
		p.cancel()
	}()
}

func (p *Pipeline) Submit(c Click) bool {
	p.mu.RLock()

	if p.closed {
		p.mu.RUnlock()
		p.drop(1)
		return false
	}

	queued := false
	depth := 0

	select {
	case p.queue <- c:
		queued = true
		depth = len(p.queue)
	default:
	}

	p.mu.RUnlock()

	if queued {
		if p.metrics != nil {
			p.metrics.SetQueueDepth(depth)
		}
		return true
	}

	p.drop(1)
	return false
}

func (p *Pipeline) drop(n int) {
	if p.metrics == nil {
		return
	}

	for i := 0; i < n; i++ {
		p.metrics.ObserveClickDropped()
	}
}

func (p *Pipeline) worker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.flushInterval)
	defer ticker.Stop()

	batch := make([]Click, 0, p.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}

		current := batch
		batch = make([]Click, 0, p.batchSize)

		if err := p.store.Append(p.ctx, current); err != nil {
			p.drop(len(current))
			return
		}

		if p.sink != nil {
			p.sink.Add(current)
		}
	}

	for {
		select {
		case c, ok := <-p.queue:
			if !ok {
				flush()
				return
			}

			batch = append(batch, c)

			if len(batch) >= p.batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

func (p *Pipeline) Shutdown(ctx context.Context) error {
	p.mu.Lock()

	if !p.closed {
		p.closed = true
		close(p.queue)

		if !p.started {
			close(p.done)
			p.cancel()
		}
	}

	p.mu.Unlock()

	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		p.cancel()
		return ctx.Err()
	}
}
