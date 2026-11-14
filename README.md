# Linkforge

Linkforge is a URL shortener project written in Go.

It currently supports Base62 encoding and decoding, link creation and validation, and in-memory storage.

Build:

go build ./...

Run:

go run ./cmd/linkforge https://example.com https://go.dev

## Shortener Service

The Shortener service provides operations for creating, resolving,
listing, and deleting short links.

It supports custom aliases, link expiration, and cursor-based pagination.
Links are stored using the Store interface with an in-memory implementation.

## Testing

Run all tests:

```bash
go test ./...
```

## Layout

```
cmd/      linkforge         demo binary — creates and prints shortened links
          linkctl           operator CLI   — prints its version, nothing more

pkg/                        reusable libraries — a separate Go module
  shortid/  Generator       code generator interface                    M4
            CodeGenerator   6-character random Base62 codes            M4
  cache/    Cache[K,V]      generic cache interface                     M4
            LRU[K,V]        least-recently-used cache                   M4
  trie/     Trie            prefix tree for autocomplete               M4
  topk/     TopK            top-N ranking using a min-heap             M4

internal/                   application code
  base62/                   base62 encode/decode                       M1
  store/                    in-memory link store                       M1
  link/     Store           link persistence, declared by its consumer M1 / M7
            Shortener       the domain service behind every transport  M2
  click/    Ingester        non-blocking submission from the hot path  M5
            Store           append-only raw event persistence          M7
            Aggregator      time series and top-N queries              M5
            Publisher       fan-out to live WebSocket subscribers      M9
  auth/     Issuer          access token minting                       M9
            Verifier        token validation for middleware            M9
            UserStore       account persistence                        M7
  transport/Server          uniform listener lifecycle                 M6 / M8
  config/   Loader          environment-only configuration             M3
  observability/Metrics     counters and histograms                    M10
```

The `pkg/` tree is a second module so that reusable libraries stay free of any dependency on application code; `go.work` ties the two together for local development.

## Design rules

These hold for every milestone.

1. **Interfaces are declared by the consumer.** `link.Store` lives in `internal/link` because the shortener is what needs it — not beside any implementation. Constructors return concrete types.
2. **The dependency arrow points inwards.** `internal/link` and `internal/click` may not import a transport or a storage package.
3. **Analytics never blocks a redirect.** `Ingester.Submit` drops events under saturation rather than applying backpressure.
4. **Every listener shares one lifecycle contract**, which is what makes a single coordinated graceful shutdown possible.

## Working on it

```bash
make            # vet, lint and test — the same gates CI runs
make build
make test
make cover
make bench
make tidy
```

Requires Go 1.25+. Linting uses [golangci-lint](https://golangci-lint.run) (`brew install golangci-lint`).

## Milestones

| # | Lessons | Scope |
|---|---|---|
| M0 | 1 | Toolchain, module layout, CI |
| M1 | 2–3 | base62 codec, `Link` model, in-memory store |
| M2 | 4–5 | Methods, interfaces, generics, error values |
| M3 | 6–7 | Files and `context`, CLI, `slog`, tests |
| M4 | 8–9 | Own hash table, LRU, trie, top-N heap |
| M5 | 10–11 | Ingestion pipeline, worker pool, synchronisation |
| M6 | 12–13 | TCP resolver, graceful shutdown, Docker |
| M7 | 14 | PostgreSQL and MongoDB stores |
| M8 | 15–16 | `net/http` first, then a framework; REST and Swagger |
| M9 | 17–19 | Validation, JWT and middleware, WebSockets |
| M10 | 20–21 | Production hardening and project defence |

## Configuration

Linkforge reads configuration from environment variables.

| Variable | Default | Description |
|---|---|---|
| `LF_HTTP_ADDR` | `:8080` | HTTP server address |
| `LF_ADMIN_ADDR` | `127.0.0.1:6060` | Admin server address |
| `LF_TCP_ADDR` | `:7070` | TCP server address |
| `LF_PG_DSN` | Required | PostgreSQL connection string |
| `LF_MONGO_URI` | Required | MongoDB connection URI |
| `LF_JWT_SECRET` | Required | JWT secret (minimum 32 bytes) |
| `LF_CACHE_SIZE` | `100000` | Cache size (positive integer) |
| `LF_LOG_LEVEL` | `info` | Logging level: debug, info, warn, error |

Configuration is validated at startup. Credentials are redacted in logs.

## CLI usage

The `linkctl` command supports `import`, `export`, `config`, and `version`.

### Import links

Create a text file `urls.txt`:

```text
https://example.com
https://go.dev
```

Import the URLs into a JSON Lines database:

```bash
go run ./cmd/linkctl import -in urls.txt -db links.jsonl
```

Expected output:

```text
imported: 2
skipped: 0
```

### Export links

Export the saved links to CSV:

```bash
go run ./cmd/linkctl export -db links.jsonl -out links.csv
```

Or print CSV to standard output:

```bash
go run ./cmd/linkctl export -db links.jsonl
```

### Validate configuration

```bash
go run ./cmd/linkctl config
```

The command requires `LF_PG_DSN`, `LF_MONGO_URI`, and `LF_JWT_SECRET`.


## Complexity

| Structure | Operation | Average | Worst |
|-----------|-----------|---------|-------|
| LRU | Get | O(1) | O(n) |
| LRU | Put | O(1) | O(n) |
| Trie | Insert | O(L) | O(L) |
| Trie | WithPrefix | O(L + k × M) | O(L + n × M) |
| TopK | Add | O(log N) | O(log N) |
| TopK | Result | O(N log N) | O(N log N) |

Here, n is the number of stored elements, L is the word or prefix length, k is the number of returned words, M is the maximum length of a returned suffix, and N is the heap capacity.

LRU.Get remains O(1) on average because the map provides direct access to the list element. The doubly linked list allows moving that element to the front in constant time without searching or shifting elements.

## Benchmarks

Environment: Windows, amd64, Intel Core i5-12400F.

| Benchmark | Time | Memory | Allocations |
|-----------|------|--------|-------------|
| BenchmarkLRUGetHit | 11.06 ns/op | 0 B/op | 0 allocs/op |
| BenchmarkTrieWithPrefix | 12699 ns/op | 680 B/op | 17 allocs/op |

The LRU cache hit requires no heap allocations, while trie prefix search allocates memory for the resulting words.

Command:

`go test ./pkg/cache ./pkg/trie -run=none -bench=Benchmark -benchmem -count=1`

## Concurrency

The Counters data is owned by a single goroutine, which processes requests received through a channel.
Other goroutines communicate with the owner by sending batches and queries through this channel.
Pipeline uses sync.RWMutex to protect its lifecycle state and prevent sending to a closed queue.
Submit uses a read lock because multiple submissions can happen concurrently without changing the lifecycle state.
Shutdown obtains the write lock, stops accepting clicks, and closes the queue.
Workers drain the remaining clicks, flush their batches to the store, and then exit.
Shutdown waits for all workers to finish or cancels the pipeline context when its deadline expires.
