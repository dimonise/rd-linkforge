package topk

import (
	"container/heap"
	"errors"
	"slices"
)

var ErrInvalidN = errors.New("topk: n must be positive")

type Item struct {
	Key   string
	Count int64
}

type itemHeap []Item

func (h itemHeap) Len() int { return len(h) }

func (h itemHeap) Less(i, j int) bool {
	return worse(h[i], h[j])
}

func (h itemHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *itemHeap) Push(x any) {
	*h = append(*h, x.(Item))
}

func (h *itemHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func worse(a, b Item) bool {
	if a.Count != b.Count {
		return a.Count < b.Count
	}
	return a.Key > b.Key
}

func better(a, b Item) bool {
	return worse(b, a)
}

type TopK struct {
	n     int
	items itemHeap
}

func New(n int) (*TopK, error) {
	if n <= 0 {
		return nil, ErrInvalidN
	}

	return &TopK{
		n:     n,
		items: make(itemHeap, 0, n),
	}, nil
}

func (t *TopK) Add(key string, count int64) {
	item := Item{Key: key, Count: count}

	if t.items.Len() < t.n {
		heap.Push(&t.items, item)
		return
	}

	if better(item, t.items[0]) {
		heap.Pop(&t.items)
		heap.Push(&t.items, item)
	}
}

func (t *TopK) Result() []Item {
	result := make([]Item, len(t.items))
	copy(result, t.items)

	slices.SortFunc(result, func(a, b Item) int {
		if better(a, b) {
			return -1
		}
		if better(b, a) {
			return 1
		}
		return 0
	})

	return result
}
