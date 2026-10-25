package link

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	service := NewService(NoopStore{})

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "Shorten",
			call: func() error {
				_, err := service.Shorten(ctx, ShortenRequest{
					OwnerID: "user1",
					Target:  "https://example.com",
				})
				return err
			},
		},
		{
			name: "Resolve",
			call: func() error {
				_, err := service.Resolve(ctx, "000001")
				return err
			},
		},
		{
			name: "List",
			call: func() error {
				_, err := service.List(ctx, "user1", Page{})
				return err
			},
		},
		{
			name: "Delete",
			call: func() error {
				return service.Delete(ctx, "user1", "000001")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()

			if !errors.Is(err, context.Canceled) {
				t.Errorf("error = %v, want context.Canceled", err)
			}
		})
	}
}

func TestWithFirstID(t *testing.T) {
	tests := []struct {
		name string
		id   uint64
		want uint64
	}{
		{"default", 0, 1},
		{"custom ID", 100, 100},
		{"first ID", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(NoopStore{}, WithFirstID(tt.id))

			got, err := service.Shorten(
				context.Background(),
				ShortenRequest{
					OwnerID: "user1",
					Target:  "https://example.com",
				},
			)
			if err != nil {
				t.Fatalf("Shorten() error = %v", err)
			}

			if got.ID != tt.want {
				t.Errorf("ID = %d, want %d", got.ID, tt.want)
			}
		})
	}
}

func TestServiceDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(-time.Second),
	)
	defer cancel()

	service := NewService(NoopStore{})

	_, err := service.Shorten(ctx, ShortenRequest{
		OwnerID: "user1",
		Target:  "https://example.com",
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestCanceledShortenDoesNotConsumeID(t *testing.T) {
	service := NewService(NoopStore{}, WithFirstID(100))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.Shorten(ctx, ShortenRequest{
		OwnerID: "user1",
		Target:  "https://example.com",
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}

	if service.nextID != 100 {
		t.Errorf("nextID = %d, want 100", service.nextID)
	}
}

type countingStore struct {
	NoopStore
	createCalls int
}

func (s *countingStore) Create(_ context.Context, l Link) (Link, error) {
	s.createCalls++
	return l, nil
}

func TestCanceledShortenDoesNotCallStore(t *testing.T) {
	st := &countingStore{}
	service := NewService(st)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.Shorten(ctx, ShortenRequest{
		OwnerID: "user1",
		Target:  "https://example.com",
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}

	if st.createCalls != 0 {
		t.Errorf("Create() called %d times, want 0", st.createCalls)
	}
}
