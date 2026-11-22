package cache

import (
	"container/list"
	"errors"
	"sync"
)

var ErrInvalidCapacity = errors.New("cache: capacity must be positive")

type entry[K comparable, V any] struct {
	key   K
	value V
}

// LRU is a fixed-capacity least-recently-used cache.
// It is safe for concurrent use.
type LRU[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	items    map[K]*list.Element
	order    *list.List
}

var _ Cache[string, int] = (*LRU[string, int])(nil)

func NewLRU[K comparable, V any](capacity int) (*LRU[K, V], error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}

	return &LRU[K, V]{
		capacity: capacity,
		items:    make(map[K]*list.Element, capacity),
		order:    list.New(),
	}, nil
}

func (c *LRU[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[key]; ok {
		c.order.MoveToFront(element)
		return element.Value.(entry[K, V]).value, true
	}

	var zero V
	return zero, false
}

func (c *LRU[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[key]; ok {
		element.Value = entry[K, V]{key, value}
		c.order.MoveToFront(element)
		return
	}

	if c.order.Len() == c.capacity {
		oldest := c.order.Back()
		delete(c.items, oldest.Value.(entry[K, V]).key)
		c.order.Remove(oldest)
	}

	element := c.order.PushFront(entry[K, V]{key, value})
	c.items[key] = element
}

func (c *LRU[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[key]; ok {
		c.order.Remove(element)
		delete(c.items, key)
	}
}

func (c *LRU[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.order.Len()
}

func (c *LRU[K, V]) Keys() []K {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys := make([]K, 0, c.order.Len())

	for element := c.order.Front(); element != nil; element = element.Next() {
		keys = append(keys, element.Value.(entry[K, V]).key)
	}

	return keys
}
