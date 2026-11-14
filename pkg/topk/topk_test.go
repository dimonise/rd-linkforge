package topk

import (
	"errors"
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	for _, n := range []int{0, -1} {
		_, err := New(n)
		if !errors.Is(err, ErrInvalidN) {
			t.Fatalf("New(%d): expected ErrInvalidN, got %v", n, err)
		}
	}

	top, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(top.Result()) != 0 {
		t.Fatal("expected empty result")
	}
}

func TestTopK(t *testing.T) {
	top, err := New(3)
	if err != nil {
		t.Fatal(err)
	}

	counts := []int64{5, 1, 9, 3, 9, 7, 2}
	for i, count := range counts {
		top.Add("k"+string(rune('0'+i)), count)
	}

	want := []Item{
		{Key: "k2", Count: 9},
		{Key: "k4", Count: 9},
		{Key: "k5", Count: 7},
	}

	if got := top.Result(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopKTieBreak(t *testing.T) {
	top, err := New(2)
	if err != nil {
		t.Fatal(err)
	}

	top.Add("z", 10)
	top.Add("b", 10)
	top.Add("a", 10)
	top.Add("c", 10)

	want := []Item{
		{Key: "a", Count: 10},
		{Key: "b", Count: 10},
	}

	if got := top.Result(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopKDuplicateKeys(t *testing.T) {
	top, err := New(3)
	if err != nil {
		t.Fatal(err)
	}

	top.Add("same", 1)
	top.Add("same", 5)
	top.Add("same", 3)

	want := []Item{
		{"same", 5},
		{"same", 3},
		{"same", 1},
	}

	if got := top.Result(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTopKResultDoesNotMutate(t *testing.T) {
	top, err := New(2)
	if err != nil {
		t.Fatal(err)
	}

	top.Add("a", 1)
	top.Add("b", 2)

	result := top.Result()
	result[0].Count = 999

	want := []Item{
		{"b", 2},
		{"a", 1},
	}

	if got := top.Result(); !reflect.DeepEqual(got, want) {
		t.Fatalf("result was mutated: %v", got)
	}
}

func TestTopKNegativeCounts(t *testing.T) {
	top, err := New(1)
	if err != nil {
		t.Fatal(err)
	}

	top.Add("a", -10)
	top.Add("b", -5)
	top.Add("c", -20)

	want := []Item{{"b", -5}}

	if got := top.Result(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
