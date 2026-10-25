package store

import (
	"context"
	"errors"
	"testing"

	"github.com/skskuzan/rd-linkforge/internal/link"
)

func TestStoreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	st := New()

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "Create",
			call: func() error {
				_, err := st.Create(ctx, link.Link{
					ID:     1,
					Code:   "000001",
					Target: "https://example.com",
				})
				return err
			},
		},
		{
			name: "ByCode",
			call: func() error {
				_, err := st.ByCode(ctx, "000001")
				return err
			},
		},
		{
			name: "ListByOwner",
			call: func() error {
				_, err := st.ListByOwner(ctx, "user1", link.Page{})
				return err
			},
		},
		{
			name: "Delete",
			call: func() error {
				return st.Delete(ctx, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()

			if !errors.Is(err, context.Canceled) {
				t.Errorf("error = %v, want context.Canceled", err)
			}
		})
	}

	if st.Count() != 0 {
		t.Errorf("canceled operations modified store: count = %d", st.Count())
	}
}
