// Package causal holds the causal metadata eventlog records with each Event
// occurrence (D-064): hybrid logical clock timestamps.
package causal

import (
	"math"
	"time"
)

// Timestamp is a hybrid logical clock reading: wall-clock milliseconds since
// the Unix epoch, and a logical counter that orders readings within the same
// millisecond or while the wall clock stands behind the last reading.
type Timestamp struct {
	WallMS  int64  `json:"wall_ms"`
	Logical uint32 `json:"logical"`
}

// Compare returns -1, 0, or 1 as t is before, equal to, or after u.
func (t Timestamp) Compare(u Timestamp) int {
	switch {
	case t.WallMS < u.WallMS, t.WallMS == u.WallMS && t.Logical < u.Logical:
		return -1
	case t == u:
		return 0
	default:
		return 1
	}
}

// Clock issues strictly increasing timestamps that follow the wall clock and
// never go backwards when it does. It is not safe for concurrent use.
type Clock struct {
	last Timestamp
	now  func() time.Time
}

// NewClock returns a clock whose readings all come after last, reading the
// wall clock from now (time.Now when nil).
func NewClock(last Timestamp, now func() time.Time) *Clock {
	if now == nil {
		now = time.Now
	}
	return &Clock{last: last, now: now}
}

// Observe records a reading from another device, so every later reading of
// this clock comes after it: a reading that causally follows another is
// always the greater.
func (c *Clock) Observe(t Timestamp) {
	if t.Compare(c.last) == 1 {
		c.last = t
	}
}

// Tick returns the next timestamp: the wall clock when it has moved past the
// last reading, otherwise the last reading with its counter advanced.
func (c *Clock) Tick() Timestamp {
	wall := c.now().UnixMilli()
	switch {
	case wall > c.last.WallMS:
		c.last = Timestamp{WallMS: wall}
	case c.last.Logical == math.MaxUint32:
		c.last = Timestamp{WallMS: c.last.WallMS + 1}
	default:
		c.last.Logical++
	}
	return c.last
}
