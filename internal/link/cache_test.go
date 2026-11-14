package link

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/skskuzan/rd-linkforge/pkg/cache"
)

type cacheTestStore struct {
	*hw2Store
	lookups   int
	deletions int
	lookupErr error
	deleteErr error
}

func (s *cacheTestStore) ByCode(ctx context.Context, code string) (Link, error) {
	s.lookups++
	if s.lookupErr != nil {
		return Link{}, s.lookupErr
	}
	return s.hw2Store.ByCode(ctx, code)
}

func (s *cacheTestStore) Delete(ctx context.Context, id uint64) error {
	s.deletions++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	return s.hw2Store.Delete(ctx, id)
}

func newCacheTestService(t *testing.T) (*Service, *cacheTestStore, *time.Time) {
	t.Helper()

	c, err := cache.NewLRU[string, Link](10)
	if err != nil {
		t.Fatal(err)
	}

	store := &cacheTestStore{hw2Store: newHW2Store()}
	now := hw2Epoch

	svc := NewService(
		store,
		WithClock(func() time.Time { return now }),
		WithCache(c),
	)
	return svc, store, &now
}

func TestServiceCacheHit(t *testing.T) {
	svc, store, _ := newCacheTestService(t)
	ctx := context.Background()

	created, err := svc.Shorten(ctx, hw2Req("https://example.com"))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		got, err := svc.Resolve(ctx, created.Code)
		if err != nil || got.ID != created.ID {
			t.Fatalf("Resolve() = %+v, %v", got, err)
		}
	}

	if store.lookups != 1 {
		t.Fatalf("expected 1 store lookup, got %d", store.lookups)
	}
}

func TestServiceCacheMissNotStored(t *testing.T) {
	svc, store, _ := newCacheTestService(t)

	for i := 0; i < 2; i++ {
		_, err := svc.Resolve(context.Background(), "missing")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	}

	if store.lookups != 2 {
		t.Fatalf("expected 2 lookups, got %d", store.lookups)
	}
}

func TestServiceCacheExpired(t *testing.T) {
	svc, store, now := newCacheTestService(t)
	ctx := context.Background()

	created, err := svc.Shorten(ctx, ShortenRequest{
		Target:    "https://example.com",
		OwnerID:   "u1",
		ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Resolve(ctx, created.Code); err != nil {
		t.Fatal(err)
	}

	*now = now.Add(2 * time.Hour)

	_, err = svc.Resolve(ctx, created.Code)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
	if store.lookups != 1 {
		t.Fatalf("expected cache hit, got %d lookups", store.lookups)
	}
}

func TestServiceCacheDelete(t *testing.T) {
	svc, store, _ := newCacheTestService(t)
	ctx := context.Background()

	created, err := svc.Shorten(ctx, hw2Req("https://example.com"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Resolve(ctx, created.Code); err != nil {
		t.Fatal(err)
	}

	if err := svc.Delete(ctx, "u1", created.Code); err != nil {
		t.Fatal(err)
	}

	_, err = svc.Resolve(ctx, created.Code)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after deletion, got %v", err)
	}
	if store.lookups != 3 {
		t.Fatalf("expected 3 store lookups, got %d", store.lookups)
	}
}

func TestServiceCacheFailedDelete(t *testing.T) {
	svc, store, _ := newCacheTestService(t)
	ctx := context.Background()

	created, err := svc.Shorten(ctx, hw2Req("https://example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Resolve(ctx, created.Code); err != nil {
		t.Fatal(err)
	}

	if err := svc.Delete(ctx, "", created.Code); !errors.Is(err, ErrEmptyOwner) {
		t.Fatalf("expected ErrEmptyOwner, got %v", err)
	}
	if err := svc.Delete(ctx, "other", created.Code); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	store.deleteErr = ErrNotFound
	if err := svc.Delete(ctx, "u1", created.Code); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected delete error, got %v", err)
	}

	lookups := store.lookups
	if _, err := svc.Resolve(ctx, created.Code); err != nil {
		t.Fatal(err)
	}
	if store.lookups != lookups {
		t.Fatal("cache was invalidated after failed deletion")
	}
}

func TestServiceCacheCanceledContext(t *testing.T) {
	svc, store, _ := newCacheTestService(t)
	ctx := context.Background()

	created, err := svc.Shorten(ctx, hw2Req("https://example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Resolve(ctx, created.Code); err != nil {
		t.Fatal(err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()

	_, err = svc.Resolve(canceled, created.Code)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if store.lookups != 1 {
		t.Fatal("unexpected store lookup")
	}
}

func TestServiceCacheStoreError(t *testing.T) {
	svc, store, _ := newCacheTestService(t)
	store.lookupErr = ErrNotFound

	_, err := svc.Resolve(context.Background(), "test")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected wrapped error, got %v", err)
	}

	store.lookupErr = nil
	_, err = svc.Resolve(context.Background(), "test")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected second store lookup, got %v", err)
	}
	if store.lookups != 2 {
		t.Fatalf("got %d lookups, want 2", store.lookups)
	}
}

func TestServiceCacheShortenDoesNotPopulate(t *testing.T) {
	svc, store, _ := newCacheTestService(t)

	created, err := svc.Shorten(context.Background(), hw2Req("https://example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if store.lookups != 0 {
		t.Fatal("Shorten accessed the cache lookup path")
	}

	if _, err := svc.Resolve(context.Background(), created.Code); err != nil {
		t.Fatal(err)
	}
	if store.lookups != 1 {
		t.Fatal("Shorten unexpectedly populated the cache")
	}
}
