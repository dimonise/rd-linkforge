package store

import (
	"context"
	"errors"
	"sort"

	"github.com/skskuzan/rd-linkforge/internal/link"
)

var ErrDuplicateID = errors.New("store: duplicate id")

type Store struct {
	byID   map[uint64]link.Link
	byCode map[string]uint64
}

var _ link.Store = (*Store)(nil)

func New() *Store {
	return &Store{
		byID:   make(map[uint64]link.Link),
		byCode: make(map[string]uint64),
	}
}

func (s *Store) Create(ctx context.Context, l link.Link) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}

	if _, exists := s.byID[l.ID]; exists {
		return link.Link{}, ErrDuplicateID
	}

	if _, exists := s.byCode[l.Code]; exists {
		return link.Link{}, link.ErrAliasTaken
	}

	s.byID[l.ID] = l
	s.byCode[l.Code] = l.ID

	return l, nil
}

func (s *Store) Get(id uint64) (link.Link, bool) {
	l, ok := s.byID[id]
	return l, ok
}

func (s *Store) All() []link.Link {
	ids := sortedIDs(s.byID)
	result := make([]link.Link, 0, len(ids))

	for _, id := range ids {
		result = append(result, s.byID[id])
	}

	return result
}

func (s *Store) Count() int {
	return len(s.byID)
}

func sortedIDs(m map[uint64]link.Link) []uint64 {
	ids := make([]uint64, 0, len(m))

	for id := range m {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	return ids
}

func (s *Store) ByCode(ctx context.Context, code string) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	
	id, exists := s.byCode[code]
	if !exists {
		return link.Link{}, link.ErrNotFound
	}

	return s.byID[id], nil
}

func (s *Store) ListByOwner(ctx context.Context, ownerID string, page link.Page) ([]link.Link, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	page = page.Normalize()

	afterID, err := page.AfterID()
	if err != nil {
		return nil, err
	}

	result := make([]link.Link, 0, page.Limit)

	for _, id := range sortedIDs(s.byID) {
		if id <= afterID {
			continue
		}

		l := s.byID[id]

		if l.OwnerID != ownerID {
			continue
		}

		result = append(result, l)

		if len(result) == page.Limit {
			break
		}
	}

	return result, nil
}

func (s *Store) Delete(ctx context.Context, id uint64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	l, exists := s.byID[id]
	if !exists {
		return link.ErrNotFound
	}

	delete(s.byID, id)
	delete(s.byCode, l.Code)

	return nil
}
