package combine

import (
	"testing"
	"time"

	"github.com/42atomys/evtc/timeline"
)

// iv returns the interval between two times in milliseconds.
func iv(start, end int) timeline.Interval {
	return timeline.Interval{Start: time.Duration(start) * time.Millisecond, End: time.Duration(end) * time.Millisecond}
}

// ms returns times in milliseconds.
func ms(v ...int) []time.Duration {
	out := make([]time.Duration, len(v))
	for i, x := range v {
		out[i] = time.Duration(x) * time.Millisecond
	}
	return out
}

func TestSet(t *testing.T) {
	var s set
	s.add(0)
	s.add(63)
	if !s.has(0) || !s.has(63) || s.has(1) || s.has(62) {
		t.Errorf("set = %#x", uint64(s))
	}
}

func TestInner(t *testing.T) {
	x := &entity{
		spans: []timeline.Interval{iv(1000, 9000)},
		holds: []hold{
			{rec: 0, held: []timeline.Interval{iv(1000, 5000)}, spawns: ms(1000), leaves: ms(5000)},
			{rec: 1, held: []timeline.Interval{iv(3000, 9000)}, spawns: ms(3000), leaves: ms(9000)},
		},
	}
	tests := []struct {
		at    int
		spawn bool
		want  bool
	}{
		{1000, true, false},  // the start of the span
		{1400, true, false},  // within sameBound of it
		{3000, true, true},   // the second client gains the agent
		{5000, false, true},  // the first one loses it
		{8600, false, false}, // within sameBound of the end
		{9000, false, false},
	}
	for _, tt := range tests {
		if got := x.inner(time.Duration(tt.at)*time.Millisecond, tt.spawn); got != tt.want {
			t.Errorf("inner(%d ms, spawn %v) = %v", tt.at, tt.spawn, got)
		}
	}
	// A bound every entry that held the agent wrote is told.
	agreed := &entity{
		spans: []timeline.Interval{iv(0, 9000)},
		holds: []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 4000), iv(4000, 9000)}, spawns: ms(4000), leaves: ms(4000)},
			{rec: 1, held: []timeline.Interval{iv(0, 4100), iv(4100, 9000)}, spawns: ms(4100), leaves: ms(4100)},
		},
	}
	if agreed.inner(4000*time.Millisecond, false) || agreed.inner(4100*time.Millisecond, true) {
		t.Error("a bound the recordings agree on is dropped")
	}
	if (&entity{}).inner(0, true) {
		t.Error("a bound of an agent nobody held is inside its life")
	}
}

func TestSpeaker(t *testing.T) {
	x := &entity{speak: []segment{{iv(0, 999), 0, 1}, {iv(1000, 3000), 1, 2}}}
	if s := x.speaker(500 * time.Millisecond); s == nil || s.entry != 1 {
		t.Errorf("speaker at 500ms = %v", s)
	}
	if s := x.speaker(4 * time.Second); s != nil {
		t.Errorf("speaker at 4s = %v", s)
	}
	if !x.speaks(2, 2*time.Second) || x.speaks(1, 2*time.Second) || !x.speaks(1, 4*time.Second) {
		t.Error("an entry speaks when it holds the instant, or when nobody does")
	}
}
