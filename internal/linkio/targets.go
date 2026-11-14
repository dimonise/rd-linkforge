package linkio

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Target struct {
	Line int
	URL  string
}

func ReadTargets(r io.Reader) ([]Target, error) {
	scanner := bufio.NewScanner(r)

	var targets []Target
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++

		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		targets = append(targets, Target{
			Line: lineNumber,
			URL:  line,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read targets: %w", err)
	}

	return targets, nil
}
