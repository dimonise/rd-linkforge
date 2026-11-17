package click

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/skskuzan/rd-linkforge/pkg/topk"
)

var (
	ErrInvalidGranularity = errors.New("click: invalid granularity")
	ErrInvalidN           = errors.New("click: n must be positive")
	ErrClosed             = errors.New("click: counters are closed")
)

type Counters struct {
	requests chan func(map[string]map[time.Time]int64)
	done     chan struct{}
	once     sync.Once
	wg       sync.WaitGroup
}

var (
	_ Aggregator = (*Counters)(nil)
	_ BatchSink  = (*Counters)(nil)
)

func NewCounters() *Counters {
	c := &Counters{
		requests: make(chan func(map[string]map[time.Time]int64)),
		done:     make(chan struct{}),
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		counts := make(map[string]map[time.Time]int64)

		for {
			select {
			case <-c.done:
				return
			case request := <-c.requests:
				request(counts)
			}
		}
	}()

	return c
}

func (c *Counters) Add(batch []Click) {
	select {
	case <-c.done:
		return
	default:
	}

	clicks := append([]Click(nil), batch...)

	select {
	case <-c.done:
	case c.requests <- func(counts map[string]map[time.Time]int64) {
		for _, click := range clicks {
			if click.Code == "" {
				continue
			}

			if counts[click.Code] == nil {
				counts[click.Code] = make(map[time.Time]int64)
			}

			hour := click.At.UTC().Truncate(time.Hour)
			counts[click.Code][hour]++
		}
	}:
	}
}

func (c *Counters) Series(ctx context.Context, code string, w Window, g Granularity) ([]Bucket, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if g != GranularityHour && g != GranularityDay {
		return nil, ErrInvalidGranularity
	}

	select {
	case <-c.done:
		return nil, ErrClosed
	default:
	}

	result := make(chan []Bucket, 1)

	request := func(counts map[string]map[time.Time]int64) {
		buckets := make(map[time.Time]int64)

		for hour, count := range counts[code] {
			if hour.Before(w.From) || !hour.Before(w.To) {
				continue
			}

			start := hour
			if g == GranularityDay {
				start = time.Date(hour.Year(), hour.Month(), hour.Day(), 0, 0, 0, 0, time.UTC)
			}

			buckets[start] += count
		}

		out := make([]Bucket, 0, len(buckets))
		for start, count := range buckets {
			out = append(out, Bucket{Start: start, Clicks: count})
		}

		slices.SortFunc(out, func(a, b Bucket) int {
			return a.Start.Compare(b.Start)
		})

		result <- out
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, ErrClosed
	case c.requests <- request:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, ErrClosed
	case out := <-result:
		return out, nil
	}
}

func (c *Counters) Top(ctx context.Context, w Window, n int) ([]Total, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if n < 1 {
		return nil, ErrInvalidN
	}

	select {
	case <-c.done:
		return nil, ErrClosed
	default:
	}

	result := make(chan []Total, 1)
	errResult := make(chan error, 1)

	request := func(counts map[string]map[time.Time]int64) {
		ranking, err := topk.New(n)
		if err != nil {
			errResult <- err
			return
		}

		for code, hours := range counts {
			var total int64

			for hour, count := range hours {
				if !hour.Before(w.From) && hour.Before(w.To) {
					total += count
				}
			}

			if total > 0 {
				ranking.Add(code, total)
			}
		}

		items := ranking.Result()
		out := make([]Total, 0, len(items))

		for _, item := range items {
			out = append(out, Total{
				Code:   item.Key,
				Clicks: item.Count,
			})
		}

		result <- out
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, ErrClosed
	case c.requests <- request:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, ErrClosed
	case err := <-errResult:
		return nil, err
	case out := <-result:
		return out, nil
	}
}

func (c *Counters) Close() {
	c.once.Do(func() {
		close(c.done)
	})
	c.wg.Wait()
}
