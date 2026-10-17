// Package click holds the click-event model and the contracts of the analytics
// path. Events are append-only and best-effort.
package click

// Click records one follow of a short link. Raw client addresses are never
// carried; the transport edge hashes them.
type Click struct{}

// Granularity is the bucket width of a time series.
type Granularity string

// Supported series granularities.
const (
	GranularityHour Granularity = "hour"
	GranularityDay  Granularity = "day"
)

// Window is a half-open time range [from, to).
type Window struct{}

// Bucket is one point of a click time series.
type Bucket struct{}

// Total is a link's aggregate click count, used for top-N rankings.
type Total struct{}
