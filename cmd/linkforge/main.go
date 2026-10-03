package main

import (
	"fmt"
	"os"

	"github.com/skskuzan/rd-linkforge/internal/link"
	"github.com/skskuzan/rd-linkforge/internal/store"
)

func main() {
	s := store.New()

	var id uint64 = 1

	for _, target := range os.Args[1:] {
		l, err := link.New(id, target)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		if err := store.Add(s, l); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		id++
	}

	for _, l := range store.All(s) {
		fmt.Printf("%s  %s\n", l.Code, l.Target)
	}

	fmt.Printf("\nTotal: %d\n", store.Count(s))
}
