package trie

import (
	"reflect"
	"testing"
)

func TestTrieInsertContains(t *testing.T) {
	tr := New()

	if tr.Insert("") || tr.Contains("") {
		t.Fatal("empty word must not be accepted")
	}
	if !tr.Insert("doc") {
		t.Fatal("failed to insert doc")
	}
	if tr.Insert("doc") {
		t.Fatal("duplicate insertion must return false")
	}
	if !tr.Contains("doc") || tr.Contains("do") || tr.Contains("missing") {
		t.Fatal("unexpected Contains result")
	}

	if !tr.Insert("docs") || !tr.Contains("docs") {
		t.Fatal("failed to insert docs")
	}
}

func TestTrieWithPrefix(t *testing.T) {
	tr := New()
	words := []string{"docs", "doc", "dog", "do-it", "day", "api", "dock"}

	for _, word := range words {
		if !tr.Insert(word) {
			t.Fatalf("failed to insert %q", word)
		}
	}

	tests := []struct {
		prefix string
		limit  int
		want   []string
	}{
		{"do", 0, []string{"do-it", "doc", "dock", "docs", "dog"}},
		{"do", 2, []string{"do-it", "doc"}},
		{"doc", -1, []string{"doc", "dock", "docs"}},
		{"unknown", 0, []string{}},
		{"", 0, []string{"api", "day", "do-it", "doc", "dock", "docs", "dog"}},
	}

	for _, tt := range tests {
		got := tr.WithPrefix(tt.prefix, tt.limit)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("WithPrefix(%q, %d) = %v, want %v",
				tt.prefix, tt.limit, got, tt.want)
		}
	}
}

func TestTrieDelete(t *testing.T) {
	tr := New()
	tr.Insert("doc")
	tr.Insert("docs")
	tr.Insert("dog")

	if tr.Delete("") || tr.Delete("missing") {
		t.Fatal("deleting unknown word must return false")
	}

	if !tr.Delete("doc") {
		t.Fatal("failed to delete doc")
	}
	if tr.Contains("doc") || !tr.Contains("docs") {
		t.Fatal("deletion damaged longer word")
	}

	if !tr.Delete("docs") || !tr.Delete("dog") {
		t.Fatal("failed to delete remaining words")
	}
	if len(tr.WithPrefix("", 0)) != 0 {
		t.Fatal("trie should be empty")
	}
}

func TestTrieByteOrder(t *testing.T) {
	tr := New()
	for _, word := range []string{"a", "A", "a1", "a!", "z"} {
		tr.Insert(word)
	}

	want := []string{"A", "a", "a!", "a1", "z"}
	if got := tr.WithPrefix("", 0); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
