package combine

import (
	"slices"
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

func TestPresence(t *testing.T) {
	tests := []struct {
		name  string
		holds []hold
		want  []timeline.Interval
	}{
		{"nothing held", nil, nil},
		{"one entry", []hold{{rec: 0, held: []timeline.Interval{iv(0, 1000)}}}, []timeline.Interval{iv(0, 1000)}},
		{"two entries that overlap", []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 1000)}},
			{rec: 1, held: []timeline.Interval{iv(800, 2000)}},
		}, []timeline.Interval{iv(0, 2000)}},
		{"two clients that do not date alike", []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 1000)}},
			{rec: 1, held: []timeline.Interval{iv(1300, 2000)}},
		}, []timeline.Interval{iv(0, 2000)}},
		{"a gap longer than sameBound", []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 1000)}},
			{rec: 1, held: []timeline.Interval{iv(1501, 2000)}},
		}, []timeline.Interval{iv(0, 1000), iv(1501, 2000)}},
		{"a short gap one recording tells", []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 1000), iv(1200, 2000)}},
		}, []timeline.Interval{iv(0, 1000), iv(1200, 2000)}},
		{"a short gap another recording covers", []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 1000), iv(1200, 2000)}},
			{rec: 1, held: []timeline.Interval{iv(500, 1500)}},
		}, []timeline.Interval{iv(0, 2000)}},
	}
	for _, tt := range tests {
		if got := presence(tt.holds); !slices.Equal(got, tt.want) {
			t.Errorf("%s: presence = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSegments(t *testing.T) {
	var owners set
	owners.add(1)
	tests := []struct {
		name   string
		holds  []hold
		owners set
		want   []segment
	}{
		{"the longest view first", []hold{
			{rec: 0, entry: 1, track: iv(0, 3000)},
			{rec: 1, entry: 2, track: iv(1000, 9000)},
		}, 0, []segment{{iv(0, 999), 0, 1}, {iv(1000, 9000), 1, 2}}},
		{"views of the same length in the order of the recordings", []hold{
			{rec: 1, entry: 2, track: iv(1000, 4500)},
			{rec: 0, entry: 1, track: iv(0, 3000)},
		}, 0, []segment{{iv(0, 3000), 0, 1}, {iv(3001, 4500), 1, 2}}},
		{"the client of the player", []hold{
			{rec: 0, entry: 1, track: iv(0, 9000)},
			{rec: 1, entry: 2, track: iv(1000, 3000)},
		}, owners, []segment{{iv(0, 999), 0, 1}, {iv(1000, 3000), 1, 2}, {iv(3001, 9000), 0, 1}}},
		{"a view that blinks inside one that lasts", []hold{
			{rec: 0, entry: 1, track: iv(0, 9000)},
			{rec: 1, entry: 2, track: iv(1000, 3000)},
		}, 0, []segment{{iv(0, 9000), 0, 1}}},
		{"the same spawn dated some milliseconds apart", []hold{
			{rec: 0, entry: 1, track: iv(1000, 3000)},
			{rec: 1, entry: 2, track: iv(1040, 9000)},
		}, 0, []segment{{iv(1000, 9000), 1, 2}}},
	}
	for _, tt := range tests {
		if got := segments(tt.holds, tt.owners); !slices.Equal(got, tt.want) {
			t.Errorf("%s: segments = %v, want %v", tt.name, got, tt.want)
		}
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

// TestSegmentsOfOneRecording checks the entries a recording split: they
// speak in the order they were held.
func TestSegmentsOfOneRecording(t *testing.T) {
	got := segments([]hold{
		{rec: 0, entry: 2, track: iv(3000, 5500)},
		{rec: 0, entry: 1, track: iv(0, 2900)},
	}, 0)
	want := []segment{{iv(0, 2900), 0, 1}, {iv(3000, 5500), 0, 2}}
	if !slices.Equal(got, want) {
		t.Errorf("segments = %v, want %v", got, want)
	}
}
