// Package click holds the click-event model and the contracts of the analytics
// path. Events are append-only and best-effort.
package click

import "time"

// Click records one follow of a short link. Raw client addresses are never
// carried; the transport edge hashes them.
type Click struct {
	Code      string
	At        time.Time
	IPHash    string
	Referer   string
	UserAgent string
	Country   string
}

// Granularity is the bucket width of a time series.
type Granularity string

// Supported series granularities.
const (
	GranularityHour Granularity = "hour"
	GranularityDay  Granularity = "day"
)

// Window is a half-open time range [from, to).
type Window struct {
	From time.Time
	To   time.Time
}

// Bucket is one point of a click time series.
type Bucket struct {
	Start  time.Time
	Clicks int64
}

// Total is a link's aggregate click count, used for top-N rankings.
type Total struct {
	Code   string
	Clicks int64
}
