package cache

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewLRU(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		_, err := NewLRU[string, int](capacity)
		if !errors.Is(err, ErrInvalidCapacity) {
			t.Fatalf("capacity %d: expected ErrInvalidCapacity, got %v", capacity, err)
		}
	}

	c, err := NewLRU[string, int](2)
	if err != nil {
		t.Fatal(err)
	}
	if c.Len() != 0 {
		t.Fatalf("expected empty cache, got %d", c.Len())
	}
}

func TestLRUGetPut(t *testing.T) {
	c, err := NewLRU[string, int](2)
	if err != nil {
		t.Fatal(err)
	}

	if value, ok := c.Get("missing"); ok || value != 0 {
		t.Fatalf("expected cache miss, got %d, %v", value, ok)
	}

	c.Put("a", 1)
	c.Put("b", 2)

	if value, ok := c.Get("a"); !ok || value != 1 {
		t.Fatalf("expected a=1, got %d, %v", value, ok)
	}

	if !reflect.DeepEqual(c.Keys(), []string{"a", "b"}) {
		t.Fatalf("unexpected order: %v", c.Keys())
	}

	c.Put("a", 10)
	if value, ok := c.Get("a"); !ok || value != 10 {
		t.Fatalf("expected updated a=10, got %d, %v", value, ok)
	}

	if c.Len() != 2 {
		t.Fatalf("expected length 2, got %d", c.Len())
	}
}

func TestLRUEviction(t *testing.T) {
	c, err := NewLRU[string, int](2)
	if err != nil {
		t.Fatal(err)
	}

	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")
	c.Put("c", 3)

	if _, ok := c.Get("b"); ok {
		t.Fatal("expected b to be evicted")
	}

	if !reflect.DeepEqual(c.Keys(), []string{"c", "a"}) {
		t.Fatalf("unexpected keys: %v", c.Keys())
	}
}

func TestLRUDelete(t *testing.T) {
	c, err := NewLRU[string, int](1)
	if err != nil {
		t.Fatal(err)
	}

	c.Put("a", 1)
	c.Delete("missing")
	c.Delete("a")

	if c.Len() != 0 {
		t.Fatalf("expected empty cache, got %d", c.Len())
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("deleted key still exists")
	}

	c.Put("b", 2)
	c.Put("c", 3)
	if _, ok := c.Get("b"); ok {
		t.Fatal("expected b to be evicted")
	}
}

func TestLRUGenericValues(t *testing.T) {
	type value struct {
		Name string
	}

	c, err := NewLRU[int, value](2)
	if err != nil {
		t.Fatal(err)
	}

	c.Put(1, value{Name: "test"})
	got, ok := c.Get(1)
	if !ok || got.Name != "test" {
		t.Fatalf("unexpected value: %+v", got)
	}
}

func TestLRUKeysDoNotChangeOrder(t *testing.T) {
	c, err := NewLRU[string, int](3)
	if err != nil {
		t.Fatal(err)
	}

	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	expected := []string{"c", "b", "a"}
	for i := 0; i < 2; i++ {
		if got := c.Keys(); !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	}
}
