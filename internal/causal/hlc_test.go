package causal

import (
	"math"
	"testing"
	"time"
)

// manualTime is a wall clock a test sets by hand.
type manualTime struct{ at time.Time }

func (m *manualTime) now() time.Time { return m.at }

// Readings increase strictly, follow the wall clock forwards, and never go
// backwards when the wall clock is stepped back.
func TestClockIsMonotonic(t *testing.T) {
	base := time.UnixMilli(1_800_000_000_000)
	wall := &manualTime{at: base}
	clock := NewClock(Timestamp{}, wall.now)
	var last Timestamp
	tick := func(want Timestamp) {
		t.Helper()
		got := clock.Tick()
		if got != want {
			t.Fatalf("tick = %+v, want %+v", got, want)
		}
		if got.Compare(last) != 1 {
			t.Fatalf("tick %+v does not follow %+v", got, last)
		}
		last = got
	}
	tick(Timestamp{WallMS: base.UnixMilli()})
	tick(Timestamp{WallMS: base.UnixMilli(), Logical: 1})
	wall.at = base.Add(5 * time.Millisecond)
	tick(Timestamp{WallMS: base.UnixMilli() + 5})
	wall.at = base.Add(-time.Hour)
	tick(Timestamp{WallMS: base.UnixMilli() + 5, Logical: 1})
	tick(Timestamp{WallMS: base.UnixMilli() + 5, Logical: 2})
	wall.at = base.Add(6 * time.Millisecond)
	tick(Timestamp{WallMS: base.UnixMilli() + 6})
}

// A clock continues after the last reading it is given, even when the wall
// clock now stands behind it, as after a restart on a machine whose clock was
// set back.
func TestClockContinuesAfterLastReading(t *testing.T) {
	last := Timestamp{WallMS: 2_000_000_000_000, Logical: 7}
	clock := NewClock(last, func() time.Time { return time.UnixMilli(1_000) })
	if got := clock.Tick(); got != (Timestamp{WallMS: last.WallMS, Logical: 8}) {
		t.Fatalf("tick = %+v", got)
	}
}

// When the counter is exhausted the clock moves to the next millisecond.
func TestClockCounterOverflow(t *testing.T) {
	last := Timestamp{WallMS: 100, Logical: math.MaxUint32}
	clock := NewClock(last, func() time.Time { return time.UnixMilli(50) })
	if got := clock.Tick(); got != (Timestamp{WallMS: 101}) {
		t.Fatalf("tick = %+v", got)
	}
}

// Observing another device's reading puts every later reading after it, and
// an earlier reading changes nothing.
func TestClockObserve(t *testing.T) {
	wall := &manualTime{at: time.UnixMilli(1000)}
	c := NewClock(Timestamp{}, wall.now)
	first := c.Tick()
	c.Observe(Timestamp{WallMS: 5000, Logical: 3})
	if got := c.Tick(); got != (Timestamp{WallMS: 5000, Logical: 4}) {
		t.Fatalf("after observing a later reading: %+v", got)
	}
	c.Observe(first)
	if got := c.Tick(); got != (Timestamp{WallMS: 5000, Logical: 5}) {
		t.Fatalf("after observing an earlier reading: %+v", got)
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b Timestamp
		want int
	}{
		{Timestamp{1, 0}, Timestamp{2, 0}, -1},
		{Timestamp{2, 0}, Timestamp{1, 9}, 1},
		{Timestamp{2, 1}, Timestamp{2, 2}, -1},
		{Timestamp{2, 2}, Timestamp{2, 2}, 0},
	}
	for _, c := range cases {
		if got := c.a.Compare(c.b); got != c.want {
			t.Errorf("%+v.Compare(%+v) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
