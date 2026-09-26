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

// union joins the groups of two entries unless that would put in one
// agent two entries a recording holds at the same time: by its own
// account they are two agents.
func union(nodes []node, groups map[int][]int, i, j int) {
	ri, rj := find(nodes, i), find(nodes, j)
	if ri == rj {
		return
	}
	gi, gj := groups[ri], groups[rj]
	if gi == nil {
		gi = []int{ri}
	}
	if gj == nil {
		gj = []int{rj}
	}
	for _, a := range gi {
		for _, b := range gj {
			x, y := &nodes[a], &nodes[b]
			if x.rec == y.rec && x.agent.Kind != timeline.KindPlayer && x.events > 0 && y.events > 0 &&
				x.life.Start < y.life.End && y.life.Start < x.life.End {
				return
			}
		}
	}
	nodes[rj].parent = ri
	groups[ri] = append(gi, gj...)
	delete(groups, rj)
}

// find returns the root of the group of an entry.
func find(nodes []node, i int) int {
	for nodes[i].parent != i {
		nodes[i].parent = nodes[nodes[i].parent].parent
		i = nodes[i].parent
	}
	return i
}

// sameIdentity reports whether two entries may name the same agent: same
// kind, same instance id, same species for an NPC.
func sameIdentity(a, b *timeline.Agent) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == timeline.KindPlayer {
		if a.Player == nil || b.Player == nil || a.Player.Account != b.Player.Account || a.Name != b.Name {
			return false
		}
		// A log of a public instance names the other players by their
		// profession and leaves their account out: the instance id
		// tells them apart.
		return a.Player.Account != "" || a.InstanceID == b.InstanceID
	}
	if a.InstanceID != b.InstanceID {
		return false
	}
	if a.InstanceID == 0 {
		// Nothing but the name and the species to go by.
		return a.Name == b.Name && a.SpeciesID == b.SpeciesID
	}
	return a.Kind != timeline.KindNPC || a.SpeciesID == b.SpeciesID
}

// shares reports whether two tracks share time: shared or more, or all of
// the shorter one.
func shares(a, b timeline.Interval) bool {
	o := min(a.End, b.End) - max(a.Start, b.Start)
	if o <= 0 {
		return false
	}
	within := (a.Start >= b.Start && a.End <= b.End) || (b.Start >= a.Start && b.End <= a.End)
	return o >= shared || within
}

// lagged reports whether two entries of two recordings share time only
// because one client learned later than the other that an agent left or
// that the next one came: the entry that starts first ends within
// sameBound after the other one starts, and the recording of either wrote
// the same bound for another entry. Both recordings tell of two agents
// then.
func lagged(nodes []node, ids []int, i, j int) bool {
	a, b := &nodes[i], &nodes[j]
	if b.track.Start < a.track.Start {
		a, b = b, a
	}
	if a.track.End >= b.track.End || a.track.End-b.track.Start > sameBound {
		return false
	}
	for _, o := range ids {
		n := &nodes[o]
		if n == a || n == b || !sameIdentity(n.agent, a.agent) {
			continue
		}
		// The recording of the second entry held the first agent too,
		// and wrote that it left.
		if n.rec == b.rec && len(n.leaves) > 0 && len(a.leaves) > 0 && n.track.End <= b.track.Start && a.track.End-n.track.End <= sameBound {
			return true
		}
		// The recording of the first entry holds the second agent too,
		// and wrote that it came.
		if n.rec == a.rec && len(n.spawns) > 0 && len(b.spawns) > 0 && n.track.Start >= a.track.End && n.track.Start-b.track.Start <= sameBound {
			return true
		}
	}
	return false
}

// join joins the entries of one bucket that name the same agent of the
// game. Entries of two recordings are the same appearance when they
// started together, one entry of each recording at most; entries that held
// the agent at the same time are the same agent, which joins the entries a
// recording split when another one went on holding the agent; an agent
// that never moved, met by two recordings at other times, is told by its
// position when its instance id served once.
func join(nodes []node, ids []int) {
	type pair struct {
		i, j int
		d    time.Duration
	}
	if a := nodes[ids[0]].agent; a.Kind != timeline.KindPlayer && a.InstanceID == 0 {
		// Nothing tells apart the agents without instance id that carry
		// the same name: the first recording that holds some keeps its
		// own.
		perRec := map[int]int{}
		first := nodes[ids[0]].rec
		for _, i := range ids {
			perRec[nodes[i].rec]++
			first = min(first, nodes[i].rec)
		}
		for _, n := range perRec {
			if n > 1 {
				for _, i := range ids {
					nodes[i].skip = nodes[i].rec > first
				}
				return
			}
		}
	}
	groups := map[int][]int{}
	var pairs []pair
	for x := range ids {
		for y := x + 1; y < len(ids); y++ {
			i, j := ids[x], ids[y]
			if nodes[i].rec == nodes[j].rec || !sameIdentity(nodes[i].agent, nodes[j].agent) {
				continue
			}
			d := nodes[i].track.Start - nodes[j].track.Start
			if d < 0 {
				d = -d
			}
			pairs = append(pairs, pair{i, j, d})
		}
	}
	slices.SortStableFunc(pairs, func(a, b pair) int { return cmp.Compare(a.d, b.d) })
	type seat struct{ node, rec int }
	taken := map[seat]bool{}
	for _, p := range pairs {
		if nodes[p.i].agent.Kind == timeline.KindPlayer {
			union(nodes, groups, p.i, p.j)
			continue
		}
		if p.d > appearance {
			break
		}
		si, sj := seat{p.i, nodes[p.j].rec}, seat{p.j, nodes[p.i].rec}
		if taken[si] || taken[sj] {
			continue
		}
		taken[si], taken[sj] = true, true
		union(nodes, groups, p.i, p.j)
	}
	for _, p := range pairs {
		x, y := &nodes[p.i], &nodes[p.j]
		if shares(x.track, y.track) {
			// An instance id names one agent at a time: entries whose
			// events share time are the same agent; those that meet at
			// one instant follow each other. What a recording held beyond
			// its events is a guess, and there a still agent that stands
			// elsewhere is another one.
			events := x.life.Start < y.life.End && y.life.Start < x.life.End
			if !events && x.hasPos && y.hasPos && x.still && y.still && x.pos.DistTo(y.pos) > 50 {
				continue
			}
			if lagged(nodes, ids, p.i, p.j) {
				continue
			}
			union(nodes, groups, p.i, p.j)
		}
	}
	// The position tells nothing of an instance id that served several
	// times: creatures come back where they were.
	perRec := map[int]int{}
	for _, i := range ids {
		perRec[nodes[i].rec]++
	}
	for _, n := range perRec {
		if n > 1 {
			return
		}
	}
	for _, p := range pairs {
		x, y := &nodes[p.i], &nodes[p.j]
		if x.hasPos && y.hasPos && x.still && y.still && x.pos.DistTo(y.pos) < 5 {
			union(nodes, groups, p.i, p.j)
		}
	}
}
