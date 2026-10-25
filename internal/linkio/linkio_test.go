package linkio

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/link"
)

func TestReadTargets(t *testing.T) {
	input := `# comment
https://example.com

  https://go.dev
   # another comment
not-a-url
`

	got, err := ReadTargets(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadTargets() error = %v", err)
	}

	want := []Target{
		{Line: 2, URL: "https://example.com"},
		{Line: 4, URL: "https://go.dev"},
		{Line: 6, URL: "not-a-url"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadTargets() = %#v, want %#v", got, want)
	}
}

func TestReadTargetsError(t *testing.T) {
	_, err := ReadTargets(failingReader{})
	if err == nil || !strings.Contains(err.Error(), testIOErrorText) {
		t.Errorf("ReadTargets() error = %v, want %s", err, testIOErrorText)
	}
}

func TestLinksRoundTrip(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	links := []link.Link{
		{
			ID:        1,
			Code:      "000001",
			Target:    "https://example.com",
			OwnerID:   "cli",
			CreatedAt: now,
		},
		{
			ID:        2,
			Code:      "000002",
			Target:    "https://go.dev",
			OwnerID:   "cli",
			CreatedAt: now,
			ExpiresAt: now.Add(24 * time.Hour),
		},
	}

	var buf bytes.Buffer

	if err := WriteLinks(&buf, links); err != nil {
		t.Fatalf("WriteLinks() error = %v", err)
	}

	got, err := ReadLinks(&buf)
	if err != nil {
		t.Fatalf("ReadLinks() error = %v", err)
	}

	if !reflect.DeepEqual(got, links) {
		t.Errorf("round trip = %#v, want %#v", got, links)
	}
}

func TestReadLinksInvalid(t *testing.T) {
	input := "{\"ID\":1}\n{\"ID\":2}\ninvalid-json\n"

	_, err := ReadLinks(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "record 3") {
		t.Errorf("error = %v, want record 3", err)
	}
}

func TestReadLinksEmpty(t *testing.T) {
	links, err := ReadLinks(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ReadLinks() error = %v", err)
	}

	if len(links) != 0 {
		t.Errorf("expected no links, got %d", len(links))
	}
}

func TestWriteCSV(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 30, 0, 0, time.UTC)

	links := []link.Link{
		{
			ID:        1,
			Code:      "000001",
			Target:    "https://example.com/?q=hello,world",
			OwnerID:   "cli",
			CreatedAt: now,
		},
		{
			ID:        2,
			Code:      "000002",
			Target:    "https://go.dev",
			OwnerID:   "cli",
			CreatedAt: now,
			ExpiresAt: now.Add(time.Hour),
		},
	}

	var buf bytes.Buffer

	if err := WriteCSV(&buf, links); err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}

	want := "id,code,target,owner_id,created_at,expires_at\n" +
		"1,000001,\"https://example.com/?q=hello,world\",cli,2026-10-25T12:30:00Z,\n" +
		"2,000002,https://go.dev,cli,2026-10-25T12:30:00Z,2026-10-25T13:30:00Z\n"

	if buf.String() != want {
		t.Errorf("WriteCSV():\ngot:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestSaveLoadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "links.jsonl")

	links := []link.Link{
		{
			ID:     1,
			Code:   "000001",
			Target: "https://example.com",
		},
	}

	if err := SaveFile(path, links); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if !reflect.DeepEqual(got, links) {
		t.Errorf("LoadFile() = %#v, want %#v", got, links)
	}
}

func TestLoadFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.jsonl")

	_, err := LoadFile(path)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("LoadFile() error = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadFileInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.jsonl")

	if err := os.WriteFile(path, []byte("invalid-json\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("LoadFile() error = %v, want error containing path", err)
	}
}

func TestSaveFileInvalidPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "links.jsonl")

	err := SaveFile(path, nil)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("SaveFile() error = %v, want fs.ErrNotExist", err)
	}
}

func TestWriteErrors(t *testing.T) {
	tests := []struct {
		name  string
		write func(io.Writer) error
	}{
		{
			name: "JSONL",
			write: func(w io.Writer) error {
				return WriteLinks(w, []link.Link{{ID: 1}})
			},
		},
		{
			name: "CSV",
			write: func(w io.Writer) error {
				return WriteCSV(w, []link.Link{{ID: 1}})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.write(failingWriter{})

			if err == nil || !strings.Contains(err.Error(), testIOErrorText) {
				t.Errorf("error = %v, want %s", err, testIOErrorText)
			}
		})
	}
}

const testIOErrorText = "test I/O failure"

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New(testIOErrorText)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New(testIOErrorText)
}
