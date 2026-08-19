package timeline

import (
	"slices"
	"time"
)

// Interval is a closed time range [Start, End] relative to the timeline
// origin. Both bounds are included, so an Interval whose Start equals its
// End describes a single instant.
type Interval struct {
	// Start is the first instant of the interval.
	Start time.Duration
	// End is the last instant of the interval, included.
	End time.Duration
}

// NewInterval builds the interval [start, end]. The bounds are swapped when
// given out of order so the result is always well formed.
func NewInterval(start, end time.Duration) Interval {
	if end < start {
		start, end = end, start
	}
	return Interval{Start: start, End: end}
}

// At builds the degenerate interval [t, t].
func At(t time.Duration) Interval { return Interval{Start: t, End: t} }

// Duration returns End - Start.
func (iv Interval) Duration() time.Duration { return iv.End - iv.Start }

// Contains reports whether t lies within the interval, bounds included.
func (iv Interval) Contains(t time.Duration) bool { return iv.Start <= t && t <= iv.End }

// Overlaps reports whether the two intervals share at least one instant.
func (iv Interval) Overlaps(o Interval) bool { return iv.Start <= o.End && o.Start <= iv.End }

// Intersect returns the common part of the two intervals. The boolean is
// false when they do not overlap.
func (iv Interval) Intersect(o Interval) (Interval, bool) {
	if !iv.Overlaps(o) {
		return Interval{}, false
	}
	return Interval{Start: max(iv.Start, o.Start), End: min(iv.End, o.End)}, true
}

// Union returns the smallest interval covering both intervals.
func (iv Interval) Union(o Interval) Interval {
	return Interval{Start: min(iv.Start, o.Start), End: max(iv.End, o.End)}
}

// Clamp returns t moved inside the interval when it falls outside.
func (iv Interval) Clamp(t time.Duration) time.Duration {
	return min(max(t, iv.Start), iv.End)
}

// String formats the interval as "[start, end]" using the Duration
// notation of the time package.
func (iv Interval) String() string { return "[" + iv.Start.String() + ", " + iv.End.String() + "]" }

// Around builds the interval [t-d, t+d].
func Around(t, d time.Duration) Interval { return NewInterval(t-d, t+d) }

// Split cuts the interval at the given times and returns the pieces in
// time order. Times outside the interval and duplicates are ignored, and
// adjacent pieces share their bound. Without any cut inside, the result
// holds the interval alone.
func (iv Interval) Split(times ...time.Duration) []Interval {
	cuts := make([]time.Duration, 0, len(times))
	for _, t := range times {
		if iv.Start < t && t < iv.End {
			cuts = append(cuts, t)
		}
	}
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	out := make([]Interval, 0, len(cuts)+1)
	start := iv.Start
	for _, t := range cuts {
		out = append(out, Interval{Start: start, End: t})
		start = t
	}
	return append(out, Interval{Start: start, End: iv.End})
}
