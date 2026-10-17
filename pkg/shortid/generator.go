// Package shortid issues the short codes used in short URLs.
package shortid

// Generator issues fresh short codes. Implementations put a counter in the low
// bits and CSPRNG output in the high bits so the keyspace cannot be walked.
type Generator interface {
	Next() (string, error)
}

// NoopGenerator satisfies Generator and does nothing. Implemented in M4.
type NoopGenerator struct{}

var _ Generator = NoopGenerator{}

func (NoopGenerator) Next() (string, error) { return "", nil }
