package combine

import (
	"cmp"
	"slices"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// appearance is how far apart two recordings may date the moment an agent
// appeared and still speak of the same appearance.
const appearance = time.Second

// sameBound is how far apart two recordings may date the same spawn,
// leave or change of state.
const sameBound = 500 * time.Millisecond

// shared is the least time two entries of two recordings share to be held
// at once: the clocks of two recordings are set within some milliseconds,
// and an agent that leaves when the next one of its instance id comes
// shares an instant with it.
const shared = 100 * time.Millisecond

// forever is the until of an agent no other one follows.
const forever = time.Duration(1<<63 - 1)

// set holds recordings, one bit each.
type set uint64

func (s set) has(i int) bool { return s&(1<<uint(i)) != 0 }
func (s *set) add(i int)     { *s |= 1 << uint(i) }

// recording is one of the logs to combine.
type recording struct {
	tl    *timeline.Timeline
	typed bool
	// off is the time of this recording minus the time of the first one
	// for the same instant.
	off time.Duration
	// epoch is the raw time of the zero of the timeline.
	epoch int64
	// span is what the recording covers, on the clock of the first one.
	span timeline.Interval
	// entity holds the agent of the combined log for each entry, and
	// entry the number of each entry among those of every recording.
	entity map[*timeline.Agent]*entity
	entry  map[*timeline.Agent]int32
	// members lists the entries of this recording per agent of the
	// combined log.
	members map[*entity][]*timeline.Agent
	// unnamed holds the entries left out of the combined log: agents
	// without instance id that nothing tells apart from a recording to the
	// other.
	unnamed map[*timeline.Agent]bool
	// given holds the agent the timeline of the recording gave to the
	// events that do not name it fully: those written without address,
	// which name the last agent seen with their instance id, and those
	// written without instance id with the address the characters of a
	// player share.
	given map[*evtc.Event]*timeline.Agent
}

// at returns the time of an event on the clock of the first recording.
func (r *recording) at(e *evtc.Event) time.Duration { return r.tl.TimeOf(e) - r.off }

// entity is one agent of the combined log, whatever the number of table
// entries the recordings hold for it.
type entity struct {
	id   int32
	kind timeline.Kind
	inst uint16
	addr uint64
	raw  *evtc.Agent
	// holds lists what every entry held of the agent.
	holds []hold
	// speak tells which entry speaks for the agent over time: the
	// samples and the states of the agent come from one entry at a time.
	speak []segment
	// owners holds the recordings of the player of the agent: a client
	// knows its own character best.
	owners set
	// until is when the next agent of the same instance id starts: the
	// agent left before. seen holds the recordings that hold that next
	// agent from its start: what such a recording writes of this agent
	// afterwards, its own timeline gave to it.
	until time.Duration
	seen  set
	// spans lists the spans of presence of the agent: what the entries
	// held, united.
	spans []timeline.Interval
	// first is when the earliest of the recordings started.
	first time.Duration
	// known tells an agent a table lists, or that an event names by its
	// address, those of extensions aside: an extension keeps a list of
	// agents of its own.
	known bool
	// master is the agent a recording names as the master of this one,
	// and masterInst the instance id it writes for it. named holds the
	// entries that name it: a client learns the master of a minion per
	// entry, and may list the minion anew without learning it again.
	master     *entity
	masterInst uint16
	named      []int32
}

// hold is what one table entry of a recording held of an agent.
type hold struct {
	rec   int
	entry int32
	agent *timeline.Agent
	// life spans the events of the entry, track what the recording held:
	// from its spawn, or the start of the log, to its last leave, or the
	// end of the log. Both are on the clock of the first recording.
	life, track timeline.Interval
	// held lists the spans over which the entry wrote it held the agent:
	// a squad member keeps its entry when it leaves and comes back.
	held   []timeline.Interval
	spawns []time.Duration
	leaves []time.Duration
}

// segment is a span over which one entry speaks for an agent.
type segment struct {
	iv    timeline.Interval
	rec   int
	entry int32
}

// node is an entry on its way to an agent of the combined log.
type node struct {
	hold
	pos    timeline.Vec3
	hasPos bool
	still  bool
	events int
	parent int
	// skip leaves the entry out of the combined log.
	skip bool
	// known is what entity.known tells of the entry.
	known bool
}

// present reports whether an entry held the agent at t.
func (x *entity) present(t time.Duration) bool {
	for _, iv := range x.spans {
		if iv.Contains(t) {
			return true
		}
	}
	return false
}

// start returns when the first entry started to hold the agent.
func (x *entity) start() time.Duration {
	t := forever
	for i := range x.holds {
		t = min(t, x.holds[i].track.Start)
	}
	return t
}

// hold returns what the entry held of the agent.
func (x *entity) hold(entry int32) *hold {
	for i := range x.holds {
		if x.holds[i].entry == entry {
			return &x.holds[i]
		}
	}
	return nil
}

// span returns the number of the span of presence that holds t, or the
// nearest one.
func (x *entity) span(t time.Duration) int {
	best, dist := -1, forever
	for i, iv := range x.spans {
		if d := max(iv.Start-t, t-iv.End, 0); d < dist {
			best, dist = i, d
		}
	}
	return best
}

// inner reports whether a bound written at t lies inside the life of the
// agent: further than sameBound from the start of its span of presence
// for a spawn, from its end for a leave. A bound inside the span stays
// when every entry that held the agent across it wrote it: what the
// recordings agree on is told, odd as it is.
func (x *entity) inner(t time.Duration, spawn bool) bool {
	i := x.span(t)
	if i < 0 {
		return false
	}
	if sp := x.spans[i]; (spawn && t-sp.Start <= sameBound) || (!spawn && sp.End-t <= sameBound) {
		return false
	}
	covered, agreed := false, true
	for i := range x.holds {
		h := &x.holds[i]
		bounds := h.leaves
		if spawn {
			bounds = h.spawns
		}
		for _, iv := range h.held {
			across := iv.Start <= t && iv.End > t+sameBound
			if spawn {
				across = iv.Start < t-sameBound && iv.End >= t
			}
			if across {
				covered = true
				agreed = agreed && near(bounds, t)
			}
		}
	}
	return !covered || !agreed
}

// speaker returns the segment that holds t, nil when no entry speaks
// then.
func (x *entity) speaker(t time.Duration) *segment {
	for i := range x.speak {
		if x.speak[i].iv.Contains(t) {
			return &x.speak[i]
		}
	}
	return nil
}

// speaks reports whether the entry may tell a sample or a state of the
// agent at t: it speaks then, or nobody does.
func (x *entity) speaks(entry int32, t time.Duration) bool {
	s := x.speaker(t)
	return s == nil || s.entry == entry
}

// near reports whether one of the times is within sameBound of t.
func near(times []time.Duration, t time.Duration) bool {
	for _, b := range times {
		if b >= t-sameBound && b <= t+sameBound {
			return true
		}
	}
	return false
}

// presence returns the spans over which an entry at least held the agent,
// in time order. A gap is one when a recording tells it: it wrote that the
// agent left and that it came back. A leave of a recording and a spawn of
// another one less than sameBound apart are two clients that do not date
// alike what they see: no gap.
func presence(holds []hold) []timeline.Interval {
	type part struct {
		iv  timeline.Interval
		rec int
	}
	var parts []part
	for i := range holds {
		for _, iv := range holds[i].held {
			parts = append(parts, part{iv, holds[i].rec})
		}
	}
	if len(parts) == 0 {
		return nil
	}
	slices.SortFunc(parts, func(a, b part) int { return cmp.Compare(a.iv.Start, b.iv.Start) })
	var united []timeline.Interval
	for _, p := range parts {
		if n := len(united); n > 0 && p.iv.Start <= united[n-1].End {
			united[n-1].End = max(united[n-1].End, p.iv.End)
			continue
		}
		united = append(united, p.iv)
	}
	told := func(left, right timeline.Interval) bool {
		ends, starts := map[int]bool{}, map[int]bool{}
		for _, p := range parts {
			// What a recording held last of the span before, and first
			// of the span after.
			if p.iv.End >= left.Start && p.iv.End <= left.End && left.End-p.iv.End <= sameBound {
				ends[p.rec] = true
			}
			if p.iv.Start >= right.Start && p.iv.Start <= right.End && p.iv.Start-right.Start <= sameBound {
				starts[p.rec] = true
			}
		}
		for rec := range ends {
			if starts[rec] {
				return true
			}
		}
		return false
	}
	out := united[:1]
	for _, iv := range united[1:] {
		last := &out[len(out)-1]
		if iv.Start-last.End <= sameBound && !told(*last, iv) {
			last.End = max(last.End, iv.End)
			continue
		}
		out = append(out, iv)
	}
	return out
}

// segments shares the life of an agent between its entries. The client of
// the player speaks for its character. Otherwise the entry that held the
// agent the longest speaks over all it held, then the next one over what
// is left, and so on: a view that blinks does not interrupt one that
// lasts. Entries that held the agent as long, within a second, speak in
// the order of the recordings.
func segments(holds []hold, owners set) []segment {
	owned := func(h *hold) bool { return owners.has(h.rec) }
	order := make([]int, len(holds))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(i, j int) int {
		a, b := &holds[i], &holds[j]
		if owned(a) != owned(b) {
			if owned(a) {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(b.track.Duration()/time.Second, a.track.Duration()/time.Second); c != 0 {
			return c
		}
		if c := cmp.Compare(a.rec, b.rec); c != 0 {
			return c
		}
		return cmp.Compare(a.track.Start, b.track.Start)
	})
	var out []segment
	for _, i := range order {
		h := &holds[i]
		// What is left of the span once the spans given are taken off.
		left := []timeline.Interval{h.track}
		for _, s := range out {
			var next []timeline.Interval
			for _, iv := range left {
				if !iv.Overlaps(s.iv) {
					next = append(next, iv)
					continue
				}
				if iv.Start < s.iv.Start {
					next = append(next, timeline.Interval{Start: iv.Start, End: s.iv.Start - time.Millisecond})
				}
				if iv.End > s.iv.End {
					next = append(next, timeline.Interval{Start: s.iv.End + time.Millisecond, End: iv.End})
				}
			}
			left = next
		}
		for _, iv := range left {
			out = append(out, segment{iv: iv, rec: h.rec, entry: h.entry})
		}
	}
	slices.SortFunc(out, func(a, b segment) int { return cmp.Compare(a.iv.Start, b.iv.Start) })

	// Two clients date the same spawn some milliseconds apart: what the
	// first one holds alone before the other starts is no span of its
	// own, the other tells what the agent is when it gains it. What the
	// last one tells after the other lost the agent, nobody else tells.
	starts := map[int32]time.Duration{}
	for i := range holds {
		starts[holds[i].entry] = holds[i].track.Start
	}
	kept := out[:0]
	for i := 0; i < len(out); i++ {
		sg := out[i]
		if sg.iv.Duration() < sameBound {
			if i+1 < len(out) && out[i+1].entry != sg.entry && out[i+1].iv.Start == sg.iv.End+time.Millisecond &&
				starts[sg.entry] == sg.iv.Start && starts[out[i+1].entry] == out[i+1].iv.Start {
				out[i+1].iv.Start = sg.iv.Start
				continue
			}
		}
		kept = append(kept, sg)
	}
	return kept
}
