package shortid

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestCodeGeneratorSequence(t *testing.T) {
	g := NewCodeGenerator(bytes.NewReader([]byte{
		0, 0, 1,
		0, 0, 1,
		0, 0, 1,
	}))

	want := []string{"000H32", "000H33", "000H34"}

	for _, expected := range want {
		got, err := g.Next()
		if err != nil {
			t.Fatal(err)
		}
		if got != expected {
			t.Fatalf("got %q, want %q", got, expected)
		}
	}
}

func TestCodeGeneratorMaxRandom(t *testing.T) {
	g := NewCodeGenerator(bytes.NewReader([]byte{255, 255, 255}))

	got, err := g.Next()
	if err != nil {
		t.Fatal(err)
	}
	if got != "bVJgVU" {
		t.Fatalf("got %q, want bVJgVU", got)
	}
}

func TestCodeGeneratorZero(t *testing.T) {
	g := NewCodeGenerator(bytes.NewReader(make([]byte, 6)))

	first, err := g.Next()
	if err != nil {
		t.Fatal(err)
	}
	second, err := g.Next()
	if err != nil {
		t.Fatal(err)
	}

	if first != "000000" || second != "000001" {
		t.Fatalf("unexpected codes: %q, %q", first, second)
	}
}

func TestCodeGeneratorReadError(t *testing.T) {
	g := NewCodeGenerator(bytes.NewReader([]byte{1, 2}))

	got, err := g.Next()
	if got != "" || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %q, error %v", got, err)
	}

	if g.counter != 0 {
		t.Fatal("counter changed after failed read")
	}
}

func TestCodeGeneratorCounterWrap(t *testing.T) {
	g := NewCodeGenerator(bytes.NewReader(make([]byte, 6)))
	g.counter = 65535

	first, err := g.Next()
	if err != nil {
		t.Fatal(err)
	}
	second, err := g.Next()
	if err != nil {
		t.Fatal(err)
	}

	if first != "000H31" || second != "000000" {
		t.Fatalf("unexpected wrap: %q, %q", first, second)
	}
}

func TestCodeGeneratorShortReader(t *testing.T) {
	g := NewCodeGenerator(bytes.NewReader(nil))

	_, err := g.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
	if g.counter != 0 {
		t.Fatal("counter changed after EOF")
	}
}
