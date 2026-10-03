package link

import (
	"errors"
	"strings"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/base62"
)

var (
	ErrEmptyTarget       = errors.New("link: empty target")
	ErrUnsupportedScheme = errors.New("link: unsupported scheme")
)

type Link struct {
	ID        uint64
	Code      string
	Target    string
	CreatedAt time.Time
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
