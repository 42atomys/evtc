package combine

import (
	"cmp"
	"errors"
	"fmt"
)

// MaxLogs is the most logs Logs combines at once.
const MaxLogs = 64

var (
	// ErrNoLog is returned when no log is given, or a nil one.
	ErrNoLog = errors.New("combine: no log")
	// ErrTooManyLogs is returned when more than MaxLogs logs are given.
	ErrTooManyLogs = fmt.Errorf("combine: more than %d logs", MaxLogs)
	// ErrBuildMismatch is returned when the logs were not written by the
	// same arcdps build: two builds do not write the same events.
	ErrBuildMismatch = errors.New("combine: logs of several arcdps builds")
	// ErrFightMismatch is returned when a log shares fewer than MinStrikes
	// strikes with the others and fewer than MinTicks ticks of their
	// instance, or started at another time by the clock of the server: it
	// recorded another fight, or too little of this one for its clock to
	// be set.
	ErrFightMismatch = errors.New("combine: logs of several fights")
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
