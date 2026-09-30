package combine

import (
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// windows are the widths within which a message of a recording looks for
// the same message among those gathered, the nearest first: most pairs are
// a few milliseconds apart, a client that lags makes some far ones.
var windows = []time.Duration{30 * time.Millisecond, 100 * time.Millisecond, 500 * time.Millisecond, 2 * time.Second}

// widest is the last of the windows.
const widest = 2 * time.Second

// gain is how long after it starts holding an agent a client writes what
// the agent is: its weapon set, whether it fights, and so on.
const gain = 250 * time.Millisecond

// relay is how late an event of an extension may reach a client: the
// healing addon sends what a client computes to the others, which write
// it up to a minute later.
const relay = time.Minute

// enough is the number of messages told once by two recordings that
// tells how late one of them gets the messages of a path.
const enough = 8

// The roles of a buff event towards the stacks of its wearer.
const (
	stackNone uint8 = iota
	stackOpens
	stackCloses
	stackClosesBuff
	stackClosesAll
	stackChanges
	stackActive
	stackDeactive
)

// key names a message of the server by what the server put in it.
type key struct {
	kind     evtc.StateChange
	src, dst int32
	skill    uint32
	p        payload
}

// path is the way of the messages of a kind between two agents.
type path struct {
	kind     evtc.StateChange
	src, dst int32
}

// resolved is the agent of the combined log an event names.
type resolved struct {
	x     *entity
	id    int32
	addr  uint64
	entry int32
	// unnamed tells an agent left out of the combined log.
	unnamed bool
}

// cand is an event of a recording on its way to the combined log.
type cand struct {
	e      evtc.Event
	t      time.Duration
	k      key
	origin int8
	// ent is the agent a state or a stack is about, entry its table entry
	// in the recording.
	ent, entry int32
	slot       slot
	value      state
	// stack is the role of a buff event, stackNone for the other kinds.
	stack uint8
	// bound is -1 for a spawn or a start, 1 for a leave or an end.
	bound int8
	// owner is the recording of the player who wears the buff, -1 when
	// none is.
	owner int8
	// claims holds the recordings that tell the message.
	claims           set
	pairable, paired bool
	// carried tells a state carried over at a change of speaker. was tells
	// what a state was before the first change an entry tells of it.
	carried, was bool
	// fight tells a start or an end of the squad combat, or a change of
	// the target of the log. ofLog tells the first start and the last end
	// a recording holds: when its log started and ended.
	fight, ofLog bool
	// gained tells an event of a client that had just gained the agent.
	gained bool
	// names tells a state an entry that does not speak for the agent
	// wrote with its master: dropped, unless no other event names it.
	names bool
	// told is when a state carried over was told, as a raw time.
	told uint64
	// ki is the key of a message of the server under the instance ids of
	// its agents, byInst tells it is set.
	ki     key
	byInst bool
	// seq is the place of the event among those of its recording.
	seq int
	// speaker tells a message about an agent from the entry that speaks
	// for it.
	speaker bool
	// moved tells a bound dated otherwise than its recording did, and
	// clamped one dated before the next agent of its instance id.
	moved, clamped bool
	// src and dst tell the fields that hold an address, srcOf and dstOf
	// the agents they name: 0 for none, and for the events whose masters
	// stay as they were written.
	src, dst     bool
	srcOf, dstOf int32
	// srcThere and dstThere tell, for an event of an extension, whether
	// the agent it names was there.
	srcThere, dstThere bool
}

// path returns the way of the message.
func (c *cand) path() path { return path{c.k.kind, c.k.src, c.k.dst} }

// place is a gathered message in the list of its key.
type place struct {
	t time.Duration
	// claim is the last recording that told the message.
	claim int
	// at is the place of the message in kept.
	at int
}

// told is what an entry told of a slot.
type told struct {
	t time.Duration
	c cand
}

// audit keeps what the tests want to know of a run.
type audit struct {
	// origin is the recording of each event of the combined log.
	origin []int8
	// pairs counts, per key, the messages that found a pair.
	pairs map[key]int
	// gaps holds, per kind, how far apart two recordings date each pair
	// made, and alone how far a message that found no pair stands from
	// the nearest one of its content.
	gaps, alone map[evtc.StateChange][]time.Duration
	// order holds, per pair made, the kind, the time of the gathered
	// message and the time of the one that claimed it.
	order []paired
	// lone holds, per recording, the times of the messages that found no
	// pair, per key.
	lone []map[key][]time.Duration
}

// paired is a pair of messages for the audit.
type paired struct {
	kind evtc.StateChange
	a, b time.Duration
}

// combiner holds the state of one run.
type combiner struct {
	recs   []*recording
	ents   []*entity
	byInst map[uint16][]*entity
	// tellings holds what every entry told of every slot, in time order,
	// kept or not: what a speaker held before it spoke.
	tellings map[int32]map[slot][]told
	// before holds, per entry, the first change of a state that names
	// the value it replaces: what the state was until then.
	before map[int32]map[slot]told

	// kept holds the events gathered, index the messages of every key in
	// time order and again the same messages under the instance ids of
	// their agents.
	kept  []cand
	index map[key][]*place
	again map[key][]*place
	// defs holds the definitions gathered, exts counts the registrations
	// of extensions per payload.
	defs map[[64]byte]bool
	exts map[[56]byte]int

	audit *audit
}

// raw returns the raw time of the combined log for a time on the clock of
// the first recording.
func (m *combiner) raw(t time.Duration) uint64 {
	return uint64(m.recs[0].epoch + t.Milliseconds())
}

// entityAt finds the agent that carried an instance id at t: the one
// whose events hold t, or else the closest within a second. It tells which
// message of another recording is the same one: a client writes what an
// agent does some milliseconds before it writes that the agent came.
func (m *combiner) entityAt(inst uint16, t time.Duration) *entity {
	var best *entity
	bestDist := time.Second + 1
	for _, x := range m.byInst[inst] {
		for i := range x.holds {
			iv := x.holds[i].life
			if d := max(iv.Start-t, t-iv.End, 0); d < bestDist {
				best, bestDist = x, d
			}
		}
	}
	return best
}

// entityHeld finds the agent a recording held under an instance id at t,
// or left less than a second before: an agent seen, never one to come.
func (m *combiner) entityHeld(inst uint16, t time.Duration) *entity {
	var best *entity
	bestDist := time.Second + 1
	for _, x := range m.byInst[inst] {
		for i := range x.holds {
			iv := x.holds[i].life
			if iv.Start > t {
				continue
			}
			if d := max(t-iv.End, 0); d < bestDist {
				best, bestDist = x, d
			}
		}
	}
	return best
}

// resolve returns the agent of the combined log an event of a recording
// names by an address and an instance id.
func (m *combiner) resolve(r *recording, addr uint64, inst uint16, t time.Duration) resolved {
	var x *entity
	var entry int32
	if addr != 0 {
		a := r.tl.AgentOf(addr, inst)
		if a == nil {
			// An address the timeline knows no agent at: the first
			// recording keeps what it wrote, and the address means nothing
			// among those of another one.
			out := resolved{id: -int32(inst)}
			if r == m.recs[0] {
				out.addr = addr
			}
			return out
		}
		if inst == 0 && a.Player != nil {
			// The characters of a player share their address: without
			// instance id, the one on the field then. The players a log
			// lists without account share none.
			if c := a.Player.CharacterAt(t + r.off); c.Addr == a.Addr {
				a = c.Agent
			}
		}
		if r.unnamed[a] {
			return resolved{unnamed: true}
		}
		x, entry = r.entity[a], r.entry[a]
	} else if inst != 0 {
		if x = m.entityAt(inst, t); x != nil {
			// The entry of this recording that held the agent then.
			best := 2 * time.Second
			for _, a := range r.members[x] {
				ts := t + r.off
				if d := max(a.Lifetime.Start-ts, ts-a.Lifetime.End, 0); d < best {
					best, entry = d, r.entry[a]
				}
			}
		}
	}
	switch {
	case x != nil && addr == 0:
		// The timeline of the recording gave the event to no agent: the
		// combined log names none either. The agent found tells which
		// message of another recording is the same one.
		return resolved{x: x, id: x.id, entry: entry}
	case x != nil:
		return resolved{x: x, id: x.id, addr: x.addr, entry: entry}
	case inst != 0:
		return resolved{id: -int32(inst)}
	}
	return resolved{}
}

// given returns the agent of the combined log for the agent the timeline
// of a recording gave to an event. The combined log names it by its
// address: the last agent seen with an instance id is not the same one
// once the recordings are put together.
func (m *combiner) given(r *recording, a *timeline.Agent) resolved {
	x := r.entity[a]
	if x == nil {
		return resolved{unnamed: r.unnamed[a]}
	}
	return resolved{x: x, id: x.id, addr: x.addr, entry: r.entry[a]}
}

// owner returns the recording whose player plays one of the agents, the
// first recording first, and that covers t within pad; -1 when none does.
func (m *combiner) owner(t, pad time.Duration, xs ...*entity) int {
	for _, x := range xs {
		if x == nil {
			continue
		}
		for ri, r := range m.recs {
			if x.owners.has(ri) && r.span.Start-pad <= t && t <= r.span.End+pad {
				return ri
			}
		}
	}
	return -1
}

// wearer returns the agent that wears the buff a buff event tells of.
func wearer(k evtc.StateChange, src, dst resolved) (resolved, bool) {
	switch k {
	case evtc.StateBuffInitial, evtc.StateBuffApply, evtc.StateBuffChange:
		return dst, true
	case evtc.StateBuffRemoveSingle, evtc.StateBuffRemoveAll, evtc.StateBuffActive, evtc.StateBuffDeactive:
		return src, true
	}
	return resolved{}, false
}

// startOf returns the start of the span the entry held at t when t is
// within gain of it.
func (x *entity) startOf(entry int32, t time.Duration) (time.Duration, bool) {
	h := x.hold(entry)
	if h == nil {
		return 0, false
	}
	for _, iv := range h.held {
		if t >= iv.Start && t-iv.Start <= gain {
			return iv.Start, true
		}
	}
	return 0, false
}

// started reports whether the entry had just started to hold the agent
// at t, or its recording to record: a client writes then what the agent
// is.
func (x *entity) started(entry int32, t time.Duration) bool {
	_, ok := x.startOf(entry, t)
	return ok
}

// gained is started for an agent that came while a recording was going
// on: the entry wrote a spawn, or its recording started after another one.
func (x *entity) gained(entry int32, t time.Duration) bool {
	from, ok := x.startOf(entry, t)
	return ok && (from-x.first > sameBound || near(x.hold(entry).spawns, from))
}

// at returns the time of an event for the order of the combined log: 0
// when its time field holds something else.
func at(e *evtc.Event) uint64 {
	if timed(e.IsStateChange) {
		return e.Time
	}
	return 0
}
