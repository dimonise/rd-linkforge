# Linkforge

Linkforge is a URL shortener project written in Go.

It currently supports Base62 encoding and decoding, link creation and validation, and in-memory storage.

Build:

go build ./...

Run:

go run ./cmd/linkforge https://example.com https://go.dev