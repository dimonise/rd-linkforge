package linkio

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/skskuzan/rd-linkforge/internal/link"
)

func ReadLinks(r io.Reader) ([]link.Link, error) {
	scanner := bufio.NewScanner(r)

	var links []link.Link
	record := 0

	for scanner.Scan() {
		record++

		var l link.Link
		if err := json.Unmarshal(scanner.Bytes(), &l); err != nil {
			return nil, fmt.Errorf("record %d: %w", record, err)
		}

		links = append(links, l)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read links: %w", err)
	}

	return links, nil
}

func WriteLinks(w io.Writer, links []link.Link) error {
	encoder := json.NewEncoder(w)

	for _, l := range links {
		if err := encoder.Encode(l); err != nil {
			return fmt.Errorf("write links: %w", err)
		}
	}

	return nil
}
