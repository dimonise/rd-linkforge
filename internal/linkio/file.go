package linkio

import (
	"errors"
	"fmt"
	"os"

	"github.com/skskuzan/rd-linkforge/internal/link"
)

func LoadFile(path string) ([]link.Link, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	links, err := ReadLinks(f)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	return links, nil
}

func SaveFile(path string, links []link.Link) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %s: %w", path, closeErr))
		}
	}()

	if err := WriteLinks(f, links); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}
