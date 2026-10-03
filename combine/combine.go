package combine

import (
	"cmp"
)

// before orders the events of the combined log: by time, the definitions
// first.
func before(a, b cand) int {
	if c := cmp.Compare(at(&a.e), at(&b.e)); c != 0 {
		return c
	}
	if c := cmp.Compare(a.rank(), b.rank()); c != 0 {
		return c
	}
	// The events a recording wrote at one instant keep the order it
	// wrote them in: a client that gives the number of a stack it removes
	// to the next one writes both at once. Those of the first recording
	// come first.
	if c := cmp.Compare(a.origin, b.origin); c != 0 {
		return c
	}
	return cmp.Compare(a.seq, b.seq)
}

// rank places an event among those of its instant. A bound dated
// otherwise than its recording did comes first for a spawn and after the
// rest for a leave, whatever the recording that tells it; one that stayed
// where it was written keeps its place among the events of its recording.
// What is carried over comes before what the new speaker tells; what an
// entry that does not speak tells comes last of all.
func (c *cand) rank() int {
	switch {
	case c.bound < 0 && c.moved:
		return -2
	case c.carried:
		return -1
	case c.names:
		return 2
	case c.bound > 0 && c.moved:
		return 1
	}
	return 0
}
