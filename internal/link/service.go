package link

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/base62"
)

type Service struct {
	store  Store
	now    func() time.Time
	nextID uint64
}

var _ Shortener = (*Service)(nil)

type Option func(*Service)

func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		s.now = now
	}
}

func WithFirstID(id uint64) Option {
	return func(s *Service) {
		if id >= 1 {
			s.nextID = id
		}
	}
}

func NewService(store Store, opts ...Option) *Service {
	s := &Service{
		store:  store,
		now:    time.Now,
		nextID: 1,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Service) Shorten(ctx context.Context, req ShortenRequest) (Link, error) {
	if err := ctx.Err(); err != nil {
		return Link{}, err
	}

	if req.OwnerID == "" {
		return Link{}, ErrEmptyOwner
	}

	l, err := New(s.nextID, req.Target)
	if err != nil {
		return Link{}, err
	}

	now := s.now()

	if !req.ExpiresAt.IsZero() && !req.ExpiresAt.After(now) {
		return Link{}, ErrInvalidExpiry
	}

	if req.Alias != "" {
		if !validAlias(req.Alias) {
			return Link{}, &AliasError{
				Alias: req.Alias,
				Err:   ErrInvalidAlias,
			}
		}

		if reservedAlias(req.Alias) {
			return Link{}, &AliasError{
				Alias: req.Alias,
				Err:   ErrReservedAlias,
			}
		}
	}

	l.OwnerID = req.OwnerID
	l.ExpiresAt = req.ExpiresAt
	l.CreatedAt = now.UTC()

	if req.Alias != "" {
		l.Code = req.Alias

		created, err := s.store.Create(ctx, l)
		if err != nil {
			if errors.Is(err, ErrAliasTaken) {
				return Link{}, &AliasError{
					Alias: req.Alias,
					Err:   ErrAliasTaken,
				}
			}

			return Link{}, fmt.Errorf("create link: %w", err)
		}

		s.nextID++

		return created, nil
	}

	for attempt := 0; attempt < 3; attempt++ {
		l.ID = s.nextID
		l.Code = base62.EncodeWidth(l.ID, 6)

		created, err := s.store.Create(ctx, l)
		if err == nil {
			s.nextID++
			return created, nil
		}

		if errors.Is(err, ErrAliasTaken) {
			s.nextID++
			continue
		}

		return Link{}, fmt.Errorf("create link: %w", err)
	}

	return Link{}, fmt.Errorf("generate link code: %w", ErrAliasTaken)
}

func (s *Service) Resolve(ctx context.Context, code string) (Link, error) {
	if err := ctx.Err(); err != nil {
		return Link{}, err
	}

	l, err := s.store.ByCode(ctx, code)

	if err != nil {
		return Link{}, fmt.Errorf("resolve link: %w", err)
	}

	if !l.ExpiresAt.IsZero() && !s.now().Before(l.ExpiresAt) {
		return Link{}, ErrExpired
	}

	return l, nil
}

func (s *Service) List(ctx context.Context, ownerID string, page Page) ([]Link, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if ownerID == "" {
		return nil, ErrEmptyOwner
	}

	return s.store.ListByOwner(ctx, ownerID, page.Normalize())
}

func (s *Service) Delete(ctx context.Context, ownerID, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if ownerID == "" {
		return ErrEmptyOwner
	}

	l, err := s.store.ByCode(ctx, code)

	if err != nil {
		return fmt.Errorf("find link for deletion: %w", err)
	}

	if l.OwnerID != ownerID {
		return ErrForbidden
	}

	if err := s.store.Delete(ctx, l.ID); err != nil {
		return fmt.Errorf("delete link: %w", err)
	}

	return nil
}

func validAlias(alias string) bool {
	if len(alias) < 3 || len(alias) > 32 {
		return false
	}

	for _, ch := range alias {
		if (ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '_' || ch == '-' {
			continue
		}

		return false
	}

	return true
}

func reservedAlias(alias string) bool {
	switch strings.ToLower(alias) {
	case "api", "ws", "healthz", "readyz", "metrics", "admin":
		return true
	}

	return false
}
