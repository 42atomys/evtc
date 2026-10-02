package combine

import (
	"cmp"
	"slices"
	"sort"
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

// read returns the events of a recording that may go on, as the combined
// log writes them.
func (m *combiner) read(ri int) []cand {
	r := m.recs[ri]
	events := r.tl.Log.Events
	order := make([]int, len(events))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(i, j int) int { return cmp.Compare(at(&events[i]), at(&events[j])) })

	cands := make([]cand, 0, len(events))
	claimed := map[[56]byte]int{}
	for seq, i := range order {
		e := events[i]
		k := kindOf(&e, r.typed)
		rl := ruleOf(&e, k, r.typed)
		cd := cand{origin: int8(ri), owner: -1, seq: seq}
		switch rl.class {
		case classDefinition:
			if b := e.Bytes(); ri == 0 || !m.defs[b] {
				m.defs[b] = true
				cd.e = e
				cands = append(cands, cd)
			}
			continue
		case classDropped:
			continue
		}
		t := r.at(&e)
		cd.t = t
		var src, dst resolved
		if rl.src {
			switch a := r.given[&events[i]]; {
			case a != nil:
				src = m.given(r, a)
			case e.SrcAgent == 0 && tracks(k):
				// A state written without address of an agent the
				// recording had not seen: that of the agent another one
				// held then, or else the state of nobody.
				if x := m.entityHeld(e.SrcInstanceID, t); x != nil {
					src = resolved{x: x, id: x.id}
				} else if e.SrcInstanceID != 0 {
					src = resolved{id: -int32(e.SrcInstanceID)}
				}
			default:
				src = m.resolve(r, e.SrcAgent, e.SrcInstanceID, t)
			}
		}
		if rl.dst {
			dst = m.resolve(r, e.DstAgent, e.DstInstanceID, t)
		}
		if src.unnamed || dst.unnamed {
			continue
		}

		w := e
		if rl.src {
			w.SrcAgent = src.addr
		}
		if rl.dst {
			w.DstAgent = dst.addr
		}
		w.Time = m.raw(t)
		cd.e = w
		cd.src, cd.dst = rl.src, rl.dst
		if rl.class != classExtension && !rl.class.ofRecording() {
			cd.srcOf, cd.dstOf = src.id, dst.id
		}
		if (k == evtc.StateTeamChange || k == evtc.StateWeaponSwap) && e.Value != 0 && src.x != nil && src.entry != 0 {
			m.remember(r, cd, k, src)
		}

		switch rl.class {
		case classRecording:
			if ri > 0 {
				continue
			}
			// The first recording keeps what it wrote, but for the
			// address of an entry joined into another one.
			cd.e = e
			if rl.src && src.x != nil {
				cd.e.SrcAgent = src.addr
			}
		case classLocal:
			// A log that started late or stopped early leaves the others
			// to tell.
			if m.covered(ri, t) {
				continue
			}
		case classFight:
			cd.k, cd.pairable, cd.fight = key{kind: k}, true, true
			switch k {
			case evtc.StateSquadCombatStart:
				cd.bound = -1
			case evtc.StateSquadCombatEnd:
				cd.bound = 1
			default:
				cd.k.dst, cd.k.p = dst.id, payload{e.SrcAgent}
			}
		case classExtension:
			if k == evtc.StateExtension {
				// A registration is told again when all of it but its
				// time is.
				b := e.Bytes()
				p := [56]byte(b[8:])
				if claimed[p]++; ri > 0 && claimed[p] <= m.exts[p] {
					continue
				}
				m.exts[p]++
				cd.e = e
				cd.e.Time = w.Time
				break
			}
			// Paired on everything but the time and the addresses. An
			// extension keeps its own list of agents, and names one that
			// left when its instance id serves again: the instance ids
			// name the agents of the message, the addresses may not.
			cd.k = key{kind: k, src: -int32(e.SrcInstanceID), dst: -int32(e.DstInstanceID), skill: e.SkillID,
				p: payload{u32(e.Value)<<32 | u32(e.BuffDamage), uint64(e.OverstackValue)<<32 | uint64(e.IFF)<<24 | uint64(e.Buff)<<16 | uint64(e.IsShields)<<8 | uint64(e.IsOffcycle), trackable(&e)}}
			cd.pairable = true
			// An agent of the table that was there then.
			cd.srcThere = src.x != nil && src.x.raw != nil && src.x.present(t)
			cd.dstThere = dst.x != nil && dst.x.raw != nil && dst.x.present(t)
		case classClient:
			about := src
			if rl.dst && dst.x != nil {
				about = dst
			}
			if k == evtc.StateCombat {
				// A client is told what its character deals and takes,
				// and estimates the rest.
				if m.owner(t, 0, src.x, dst.x) == ri {
					break
				}
				// Around the bounds of the recording of that client, a
				// tick lost is better than one told twice.
				if m.owner(t, gain, src.x, dst.x) >= 0 {
					continue
				}
			}
			if about.x == nil {
				// Nobody to speak for: the first recording that was
				// going on then keeps what it wrote.
				if m.covered(ri, t) {
					continue
				}
				break
			}
			if rl.slot != slotNone {
				cd.ent, cd.entry, cd.slot, cd.value = about.id, about.entry, rl.slot, rl.value(&e)
				cd.gained = about.x.started(about.entry, t)
				m.tell(cd)
			}
			if !about.x.speaks(about.entry, t) {
				// A state told with the master of the agent goes to
				// settle, which keeps it when no other event names the
				// master. A state that moves is a sample of the clock of
				// its client.
				if rl.slot == slotNone || rl.slot.moving() || e.SrcMasterInstanceID == 0 || about.x.master == nil {
					continue
				}
				cd.names = true
			}
		case classBound:
			if src.x != nil {
				cd.ent = src.id
				within := src.x.inner(t, k == evtc.StateSpawn)
				if k != evtc.StateSpawn && t >= src.x.until && !src.x.seen.has(ri) {
					// The next agent of the instance id is there and
					// this recording does not know: a leave written
					// without address would name it.
					was := m.raw(t)
					t = src.x.until - time.Millisecond
					if (m.raw(t)^was)&1 != 0 {
						t -= time.Millisecond
					}
					cd.t, cd.e.Time, cd.moved, cd.clamped = t, m.raw(t), true, true
				}
				if within {
					continue
				}
				if k == evtc.StateDespawn {
					cd.stack = stackClosesAll
				}
			}
			cd.bound = 1
			if k == evtc.StateSpawn {
				cd.bound = -1
			}
			cd.k, cd.pairable = key{kind: k, src: src.id}, true
			if src.x != nil {
				// A bound pairs within its span of presence.
				cd.k.p[0] = uint64(src.x.span(t) + 1)
			}
		case classServer:
			if !m.message(&cd, k, rl, &e, src, dst) {
				continue
			}
		}
		cands = append(cands, cd)
	}
	// A client writes a start of the squad combat when its log starts and
	// an end when it ends, whatever the squad does then.
	for i := range cands {
		if c := &cands[i]; c.fight && c.bound < 0 {
			c.ofLog = true
			break
		}
	}
	for i := len(cands) - 1; i >= 0; i-- {
		if c := &cands[i]; c.fight && c.bound > 0 {
			c.ofLog = true
			break
		}
	}
	return cands
}

// remember keeps what a team or a weapon set was before the first change
// an entry tells of it: the change told backwards, to what was, from
// nothing.
func (m *combiner) remember(r *recording, cd cand, k evtc.StateChange, src resolved) {
	sl := slotTeam
	if k == evtc.StateWeaponSwap {
		sl = slotWeaponSet
	}
	b := m.before[src.entry]
	if b == nil {
		b = map[slot]told{}
		m.before[src.entry] = b
	}
	if _, ok := b[sl]; ok {
		return
	}
	was := cd
	was.e.DstAgent, was.e.Value = uint64(uint32(cd.e.Value)), 0
	was.ent, was.entry, was.slot = src.id, src.entry, sl
	if rw := ruleOf(&was.e, k, r.typed); rw.value != nil {
		was.value = rw.value(&was.e)
	}
	b[sl] = told{cd.t, was}
}

// message fills the candidate of a message of the server, and reports
// whether it goes on.
func (m *combiner) message(cd *cand, k evtc.StateChange, rl rule, e *evtc.Event, src, dst resolved) bool {
	t := cd.t
	cd.k = key{kind: k, p: rl.key(e)}
	if rl.keySrc {
		cd.k.src = src.id
	}
	if rl.keyDst {
		cd.k.dst = dst.id
	}
	if rl.keySkill {
		cd.k.skill = e.SkillID
	}
	cd.pairable = true
	// The same message under the instance ids of its agents: an agent
	// written without address is found by its instance id and the
	// instant, and two clients a millisecond apart may find two agents.
	cd.ki, cd.byInst = cd.k, true
	if rl.keySrc {
		cd.ki.src = -int32(e.SrcInstanceID)
	}
	if rl.keyDst {
		cd.ki.dst = -int32(e.DstInstanceID)
	}
	if tracks(k) && src.x != nil {
		if sp := src.x.speaker(t); sp != nil && sp.entry == src.entry {
			cd.speaker = true
		}
	}
	if w, ok := wearer(k, src, dst); ok && w.x != nil {
		// A client knows every buff of its character and how long each
		// lasts; the others are told a part. Every recording looks for
		// its pairs; the client of the character decides afterwards.
		// Where the recording of that client starts or ends, what
		// another one tells alone is kept: the clocks are set within
		// some milliseconds.
		cd.owner = int8(m.owner(t, -shared, w.x))
		cd.ent = w.id
	}
	switch k {
	case evtc.StateBuffChange:
		cd.stack = stackChanges
	case evtc.StateBuffActive:
		cd.stack = stackActive
	case evtc.StateBuffDeactive:
		cd.stack = stackDeactive
	case evtc.StateBuffInitial, evtc.StateBuffApply:
		// The same opening of a stack.
		cd.k.kind = evtc.StateBuffApply
		if dst.x != nil {
			cd.ent, cd.entry, cd.stack = dst.id, dst.entry, stackOpens
			if dst.x.gained(dst.entry, t) {
				// A client that gains an agent writes the stacks it
				// carries: no message to pair with the one that opened
				// the stack.
				cd.gained = true
				cd.k.p[1] = 1
			}
		}
	case evtc.StateBuffRemoveSingle:
		if src.x != nil {
			cd.ent, cd.stack = src.id, stackCloses
		}
	case evtc.StateBuffRemoveAll:
		if src.x != nil {
			cd.ent, cd.stack = src.id, stackClosesBuff
		}
	}
	if rl.slot != slotNone && src.x != nil {
		cd.ent, cd.entry, cd.slot, cd.value = src.id, src.entry, rl.slot, rl.value(e)
		// What a client writes of an agent it has just gained is no
		// message of the server: it counts when its entry speaks.
		if src.x.gained(src.entry, t) {
			cd.gained = true
			m.tell(*cd)
			if !src.x.speaks(src.entry, t) {
				return false
			}
			// Two clients that gain the agent a second apart tell it
			// twice: no pair to look for.
			cd.pairable = false
		}
	}
	return true
}

// pair looks, for every message of a recording, for the same message
// among those gathered: the nearest one of the same key that this
// recording did not claim yet.
func (m *combiner) pair(ri int, cands []cand) {
	late := m.lateness(cands)
	resort := map[key]bool{}

	// The events of an extension go from a client to the others on their
	// own way, and reach them seconds late and several at once: those of
	// a key pair in their order.
	ext := map[key][]int{}
	for i := range cands {
		if c := &cands[i]; c.pairable && c.k.kind == evtc.StateExtensionCombat {
			ext[c.k] = append(ext[c.k], i)
		}
	}
	for k, l := range ext {
		list, j := m.index[k], 0
		for _, i := range l {
			c := &cands[i]
			for j < len(list) && (list[j].claim == ri || list[j].t < c.t-relay) {
				j++
			}
			if j < len(list) && list[j].t <= c.t+relay {
				m.claim(ri, c, list[j], resort)
				j++
			}
		}
	}

	nearest := func(list []*place, at, w time.Duration, skip func(*place) bool) *place {
		lo := sort.Search(len(list), func(j int) bool { return list[j].t >= at-w })
		var best *place
		var bestDist time.Duration
		for j := lo; j < len(list) && list[j].t <= at+w; j++ {
			if list[j].claim == ri || (skip != nil && skip(list[j])) {
				continue
			}
			d := list[j].t - at
			if d < 0 {
				d = -d
			}
			if best == nil || d < bestDist {
				best, bestDist = list[j], d
			}
		}
		return best
	}
	for _, w := range windows {
		for i := range cands {
			c := &cands[i]
			if !c.pairable || c.paired || c.k.kind == evtc.StateExtensionCombat {
				continue
			}
			if best := nearest(m.index[c.k], c.t-late[c.path()], w, nil); best != nil {
				m.claim(ri, c, best, resort)
				continue
			}
			// The stack a client tells at a gain, which another client that
			// held the agent tells as an application at the same instant,
			// is that application; its gain was no more than a quarter
			// second before. Measured: 352 of 352 such pairs stand within
			// 100 ms, 324 within 30 ms.
			if c.stack == stackOpens && w <= 100*time.Millisecond {
				k := c.k
				k.p[1] ^= 1
				if best := nearest(m.index[k], c.t-late[c.path()], w, nil); best != nil {
					m.claim(ri, c, best, resort)
				}
			}
		}
	}
	// A message left alone, that another recording tells at the same
	// instant under the same instance ids.
	gained := func(p *place) bool { return m.kept[p.at].gained }
	for i := range cands {
		c := &cands[i]
		if !c.pairable || c.paired || !c.byInst || c.gained {
			continue
		}
		if best := nearest(m.again[c.ki], c.t-late[c.path()], shared, gained); best != nil {
			m.claim(ri, c, best, resort)
		}
	}
	if m.audit != nil {
		m.lonely(ri, cands)
	}
	for k := range resort {
		list := m.index[k]
		sort.SliceStable(list, func(a, b int) bool { return list[a].t < list[b].t })
	}
}

// lonely keeps, for the audit, how far the messages of a recording that
// found no pair stand from the nearest message of their content.
func (m *combiner) lonely(ri int, cands []cand) {
	for i := range cands {
		c := &cands[i]
		if !c.pairable || c.paired {
			continue
		}
		nearest := forever
		for _, p := range m.index[c.k] {
			if p.claim != ri {
				nearest = min(nearest, (p.t - c.t).Abs())
			}
		}
		if nearest != forever {
			m.audit.alone[c.k.kind] = append(m.audit.alone[c.k.kind], nearest)
		}
	}
}

// lateness tells how late a recording gets the messages of each path, once
// the clocks are set on the strikes. The messages told once by this
// recording and once by those gathered give the delay.
func (m *combiner) lateness(cands []cand) map[path]time.Duration {
	count := map[key]int{}
	for i := range cands {
		if c := &cands[i]; c.pairable {
			count[c.k]++
		}
	}
	gaps := map[path][]time.Duration{}
	for i := range cands {
		c := &cands[i]
		if !c.pairable || c.bound != 0 || c.gained || count[c.k] != 1 || len(m.index[c.k]) != 1 {
			continue
		}
		if d := c.t - m.index[c.k][0].t; d >= -widest && d <= widest {
			gaps[c.path()] = append(gaps[c.path()], d)
		}
	}
	late := map[path]time.Duration{}
	for p, l := range gaps {
		if len(l) >= enough {
			slices.Sort(l)
			late[p] = l[len(l)/2]
		}
	}
	return late
}

// claim pairs a message of a recording with a gathered one, and gives the
// latter what the former tells better.
func (m *combiner) claim(ri int, c *cand, best *place, resort map[key]bool) {
	best.claim = ri
	c.paired = true
	if m.audit != nil {
		m.audit.pairs[c.k]++
		m.audit.gaps[c.k.kind] = append(m.audit.gaps[c.k.kind], (c.t - best.t).Abs())
		m.audit.order = append(m.audit.order, paired{c.k.kind, best.t, c.t})
	}
	k := &m.kept[best.at]
	// A start or an end is that of a log when every recording that tells
	// it wrote it as its own started or ended.
	ofLog := k.ofLog && c.ofLog
	// A recording names the master of a minion another one left out.
	srcMaster, dstMaster := cmp.Or(k.e.SrcMasterInstanceID, c.e.SrcMasterInstanceID), cmp.Or(k.e.DstMasterInstanceID, c.e.DstMasterInstanceID)
	if int(c.owner) == ri && int(k.origin) != ri {
		// The client of the character tells it best.
		at, claims := k.t, k.claims
		*k = *c
		k.paired, k.t, k.claims = false, at, claims
	}
	if c.speaker && !k.speaker && c.owner < 0 {
		// What happens to an agent goes with the samples of the client
		// that speaks for it: a teleport dated by another client falls
		// between two of them.
		claims := k.claims
		*k = *c
		k.paired, k.claims = false, claims
		best.t = c.t
		resort[c.k] = true
	}
	if c.gained && c.stack == stackOpens && c.t < k.t && k.gained {
		// Two clients that gained the agent tell the stacks it carries:
		// the first to see them dates them.
		claims := k.claims
		*k = *c
		k.paired, k.claims = false, claims
	}
	if k.gained && !c.gained && c.stack == stackOpens {
		// The stack told at a gain is the application another client
		// saw: the application stands, under the key it was found by.
		at, claims, key := k.t, k.claims, k.k
		*k = *c
		k.paired, k.t, k.claims, k.k = false, at, claims, key
	}
	if k.src && c.src && k.e.SrcAgent == 0 && c.e.SrcAgent != 0 {
		// A recording names the agent the other one left out: the
		// combined log names it.
		k.e.SrcAgent = c.e.SrcAgent
	}
	if k.dst && c.dst && k.e.DstAgent == 0 && c.e.DstAgent != 0 {
		k.e.DstAgent = c.e.DstAgent
	}
	if c.k.kind == evtc.StateExtensionCombat {
		// The agent that was there is the one.
		if c.srcThere && !k.srcThere {
			k.e.SrcAgent, k.srcThere = c.e.SrcAgent, true
		}
		if c.dstThere && !k.dstThere {
			k.e.DstAgent, k.dstThere = c.e.DstAgent, true
		}
	}
	k.claims.add(ri)
	// What the recordings held together starts at the earliest spawn and
	// ends at the latest leave.
	if c.bound != 0 && c.e.Time != k.e.Time && (c.bound > 0) == (c.e.Time > k.e.Time) && !(c.bound > 0 && k.clamped && k.t < c.t) {
		if c.fight {
			// The clock of the client that wrote it.
			k.e = c.e
		}
		k.e.Time, k.t, k.moved = c.e.Time, c.t, true
	}
	if c.k.kind == evtc.StateExtensionCombat && c.t < k.t {
		// The client that wrote it first is the closest to the instant
		// it tells of.
		k.e.Time, k.t, best.t = c.e.Time, c.t, c.t
		resort[c.k] = true
	}
	k.ofLog = ofLog
	if k.srcOf != 0 && k.e.SrcMasterInstanceID == 0 {
		k.e.SrcMasterInstanceID = srcMaster
	}
	if k.dstOf != 0 && k.e.DstMasterInstanceID == 0 {
		k.e.DstMasterInstanceID = dstMaster
	}
}

// covered reports whether a recording read before the one given was going
// on at t.
func (m *combiner) covered(ri int, t time.Duration) bool {
	for _, r := range m.recs[:ri] {
		if r.span.Contains(t) {
			return true
		}
	}
	return false
}
