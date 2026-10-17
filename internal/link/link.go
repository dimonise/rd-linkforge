package link

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/base62"
)

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

var (
	ErrEmptyTarget       = errors.New("link: empty target")
	ErrUnsupportedScheme = errors.New("link: unsupported scheme")
	ErrInvalidCursor     = errors.New("link: invalid cursor")
)

type Link struct {
	ID         uint64
	Code       string
	Target     string
	OwnerID    string
	CreatedAt  time.Time
	ExpiresdAt time.Time
}

type ShortenRequest struct {
	Target    string
	Alias     string
	OwnerID   string
	ExpiresAt time.Time
}

type Page struct {
	Cursor string
	Limit  int
}

func (p Page) Normalize() Page {
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}

	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}

	return p
}

func (p Page) AfterID() (uint64, error) {
	if p.Cursor == "" {
		return 0, nil
	}

	id, err := base62.Decode(p.Cursor)
	if err != nil {
		return 0, fmt.Errorf("Decode cursor %q: %w", p.Cursor, ErrInvalidCursor)
	}

	return id, nil
}

func New(id uint64, target string) (Link, error) {
	if target == "" {
		return Link{}, ErrEmptyTarget
	}

	if !strings.HasPrefix(target, "http://") &&
		!strings.HasPrefix(target, "https://") {
		return Link{}, ErrUnsupportedScheme
	}

	return Link{
		ID:        id,
		Code:      base62.EncodeWidth(id, 6),
		Target:    target,
		CreatedAt: time.Now().UTC(),
	}, nil
}
