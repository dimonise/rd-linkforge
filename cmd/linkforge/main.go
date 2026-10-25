package main

import (
	"context"
	"fmt"
	"os"

	"github.com/skskuzan/rd-linkforge/internal/link"
	"github.com/skskuzan/rd-linkforge/internal/store"
)

func main() {
	ctx := context.Background()

	s := store.New()
	svc := link.NewService(s)

	for _, target := range os.Args[1:] {
		_, err := svc.Shorten(ctx, link.ShortenRequest{
			Target:  target,
			OwnerID: "demo",
		})

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
	}

	for _, l := range s.All() {
		fmt.Printf("%s  %s\n", l.Code, l.Target)
	}

	fmt.Printf("\nTotal: %d\n", s.Count())
}
