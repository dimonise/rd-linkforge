package store

import (
	"errors"
	"sort"

	"github.com/skskuzan/rd-linkforge/internal/link"
)

var ErrDuplicateID = errors.New("store: duplicate id")

type Store struct {
	byID map[uint64]link.Link
}

func New() Store {
	return Store{
		byID: make(map[uint64]link.Link),
	}
}

func Add(store Store, link link.Link) error {
	if _, exists := store.byID[link.ID]; exists {
		return ErrDuplicateID
	}

	store.byID[link.ID] = link

	return nil
}

func Get(store Store, id uint64) (link.Link, bool) {
	l, ok := store.byID[id]
	return l, ok
}

func All(store Store) []link.Link {
	result := make([]link.Link, 0, len(store.byID))

	for _, l := range store.byID {
		result = append(result, l)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

func Count(store Store) int {
	return len(store.byID)
}
