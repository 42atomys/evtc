package combine

import (
	"time"

	"github.com/42atomys/evtc"
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
