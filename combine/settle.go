package combine

import (
	"maps"
	"slices"
	"time"

	"github.com/42atomys/evtc"
)

// firstOwn is the first stack number given to a stack whose own number
// is taken. arcdps numbers the stacks from 1.
const firstOwn = uint64(1) << 31

// stackKey names a stack of the combined log.
type stackKey struct {
	wearer int32
	skill  uint32
	id     uint64
}

// stackOf is an open stack: the entry that opened it, the recordings that
// told it, the number it carries in the combined log and when it was
// opened.
type stackOf struct {
	entry int32
	told  set
	id    uint64
	at    uint64
}

// stacks follows the stacks of the combined log. A client numbers the
// stacks it holds, and gives the number of a stack that ended to the next
// one. Two clients agree on the stacks both see, but one of them may hold
// a stack the other does not under a number the other reads on another
// stack at that time. Such a stack gets a number of its own.
type stacks struct {
	open map[stackKey]stackOf
	// worn holds the open stacks of each wearer.
	worn map[int32]map[stackKey]bool
	// latest holds the last stack opened under a number, per wearer, and
	// holder the stack that carries a number.
	latest map[wornID]stackKey
	holder map[uint64]stackKey
	next   uint64
}

// newStacks returns the stacks of a log that holds none yet.
func newStacks() *stacks {
	return &stacks{open: map[stackKey]stackOf{}, worn: map[int32]map[stackKey]bool{}, latest: map[wornID]stackKey{}, holder: map[uint64]stackKey{}, next: firstOwn}
}

// wornID is the number of a stack of a wearer, as a client wrote it.
type wornID struct {
	wearer int32
	id     uint64
}

// fresh returns a number no open stack carries.
func (s *stacks) fresh() uint64 {
	for {
		if _, ok := s.holder[s.next]; !ok {
			break
		}
		s.next++
	}
	s.next++
	return s.next - 1
}

// release closes a stack and frees its number.
func (s *stacks) release(k stackKey) {
	if st, ok := s.open[k]; ok {
		if s.holder[st.id] == k {
			delete(s.holder, st.id)
		}
		delete(s.open, k)
		delete(s.worn[k.wearer], k)
	}
}

// number writes the number of a stack in an event.
func number(e *evtc.Event, id uint64) {
	e.Pad61, e.Pad62, e.Pad63, e.Pad64 = uint8(id), uint8(id>>8), uint8(id>>16), uint8(id>>24)
}

// settle gives a buff event the number its stack carries in the combined
// log, and reports whether the event tells again a stack that is open.
func (s *stacks) settle(c *cand) (restated bool) {
	switch c.stack {
	case stackOpens:
		k := stackKey{c.ent, c.e.SkillID, trackable(&c.e)}
		if from, ok := s.open[k]; ok && from.entry != c.entry && (c.gained || c.e.Time-from.at <= uint64(widest/time.Millisecond)) {
			// A client that gains the agent tells the stacks it
			// carries, and two clients tell the same stack a moment
			// apart. Later, a stack that takes the number of one still
			// open is a new one: the other ended without a word, as
			// within one log.
			return true
		}
		s.release(k)
		id := k.id
		if h, ok := s.holder[id]; ok {
			if s.open[h].told.has(int(c.origin)) {
				// The client gave the number to a new stack: the one
				// that held it ended without a word.
				s.release(h)
			} else {
				id = s.fresh()
				number(&c.e, id)
			}
		}
		s.holder[id] = k
		told := c.claims
		told.add(int(c.origin))
		s.open[k] = stackOf{c.entry, told, id, c.e.Time}
		if s.worn[k.wearer] == nil {
			s.worn[k.wearer] = map[stackKey]bool{}
		}
		s.worn[k.wearer][k] = true
		s.latest[wornID{c.ent, k.id}] = k
	case stackCloses:
		k := stackKey{c.ent, c.e.SkillID, trackable(&c.e)}
		if st, ok := s.open[k]; ok {
			number(&c.e, st.id)
			s.release(k)
		} else if h, ok := s.holder[k.id]; ok && !s.open[h].told.has(int(c.origin)) {
			// Another stack holds the number: this one ends nothing
			// the combined log holds.
			number(&c.e, s.fresh())
		}
	case stackClosesBuff, stackClosesAll:
		for k := range s.worn[c.ent] {
			if c.stack == stackClosesAll || k.skill == c.e.SkillID {
				s.release(k)
			}
		}
	case stackChanges, stackActive, stackDeactive:
		id := trackable(&c.e)
		if c.stack == stackActive {
			id = c.e.DstAgent
		}
		k, ok := s.latest[wornID{c.ent, id}]
		st, isOpen := s.open[k]
		h, held := s.holder[id]
		to := id
		switch {
		case ok && isOpen:
			to = st.id
		case held && !s.open[h].told.has(int(c.origin)):
			to = s.fresh()
		}
		if to != id && c.stack == stackActive {
			c.e.DstAgent = to
		} else if to != id {
			number(&c.e, to)
		}
	}
	return false
}

// telling is what an entry tells of the markers of an agent at one
// instant: an event that takes them all off, then one per marker.
type telling struct {
	ent, entry int32
	at         uint64
	carried    bool
}

// worn holds the markers an agent wears and the entry that told them.
type worn struct {
	set   map[state]bool
	entry int32
}

// markers follows the markers of the agents of the combined log.
type markers struct {
	tellings map[telling][]int
	worn     map[int32]*worn
	judged   map[telling]bool
	// again holds the events that tell what is worn already.
	again map[int]bool
}

// judge decides, for the events of one telling, which ones tell again
// what the agent wears.
func (m *markers) judge(kept []cand, k telling) {
	m.judged[k] = true
	w := m.worn[k.ent]
	if w == nil {
		w = &worn{set: map[state]bool{}}
		m.worn[k.ent] = w
	}
	said := map[state]bool{}
	whole := false
	for _, i := range m.tellings[k] {
		if v := kept[i].value; v[0] == 0 {
			whole = true
		} else {
			said[v] = true
		}
	}
	if !whole {
		// Markers put on top of those worn.
		for _, i := range m.tellings[k] {
			v := kept[i].value
			if w.set[v] && (w.entry != k.entry || k.carried) {
				m.again[i] = true
			} else {
				w.set[v] = true
			}
		}
		return
	}
	same := len(said) == len(w.set) && (w.entry != 0 || k.carried)
	for v := range said {
		if !w.set[v] {
			same = false
		}
	}
	if same && (w.entry != k.entry || k.carried) {
		for _, i := range m.tellings[k] {
			m.again[i] = true
		}
		if !k.carried {
			w.entry = k.entry
		}
		return
	}
	w.set, w.entry = said, k.entry
}

// slotKey names a state of an agent.
type slotKey struct {
	ent  int32
	slot slot
}

// held is the value of a state, the entry that told it and when.
type held struct {
	value state
	entry int32
	at    uint64
}

// anew reports whether an entry that takes a state over tells it anew,
// within sameBound of the last telling of the entry that spoke before. An
// entry that told the value as many times as the other one did, up to that
// telling, tells another event; one that told it fewer times tells that
// telling, dated by another client.
func (m *combiner) anew(c *cand, h held) bool {
	if c.gained || c.slot.moving() {
		return false
	}
	from := h.at - min(h.at, uint64(sameBound/time.Millisecond))
	told := func(entry int32, until uint64) int {
		n := 0
		for _, p := range m.tellings[entry][c.slot] {
			if at := m.raw(p.t); at >= from && at < until && p.c.value == c.value {
				n++
			}
		}
		return n
	}
	return told(c.entry, c.e.Time) >= told(h.entry, h.at+1)
}

// teamOf returns the team an agent had at t, as the entries that held it
// told it: the last change of team one of them told before t, or the team
// one held before its first change. 0 when none told it.
func (m *combiner) teamOf(ent int32, t time.Duration) uint32 {
	team, at := uint32(0), time.Duration(-1)
	for _, h := range m.ents[ent-1].holds {
		if b, ok := m.before[h.entry][slotTeam]; ok && b.t <= t && b.t > at {
			team, at = uint32(b.c.e.DstAgent), b.t
		}
		for _, told := range m.tellings[h.entry][slotTeam] {
			if told.t <= t && told.t >= at && told.c.e.DstAgent != 0 {
				team, at = uint32(told.c.e.DstAgent), told.t
			}
		}
	}
	return team
}

// settle walks the events in time order and drops what tells again what
// the combined log holds. A client that gains an agent is told what the
// agent carries and writes its states anew: within one entry of a log
// these are events as a log alone tells them; once entries are joined
// into one agent they repeat what is known. The states an entry that does
// not speak for the agent tells are dropped as well. An agent whose
// master no event kept names while the master is there keeps the first
// event dropped that does.
func (m *combiner) settle(kept []cand) []cand {
	st := newStacks()
	mk := markers{tellings: map[telling][]int{}, worn: map[int32]*worn{}, judged: map[telling]bool{}, again: map[int]bool{}}
	for i := range kept {
		if c := &kept[i]; c.slot == slotMarker {
			k := telling{c.ent, c.entry, c.e.Time, c.carried}
			mk.tellings[k] = append(mk.tellings[k], i)
		}
	}
	states := map[slotKey]held{}
	// gone holds the agents that left, mastered those an event kept
	// names the master of while the master is there, and fallen the
	// first event dropped that does, per agent.
	gone := map[int32]bool{}
	mastered := map[int32]bool{}
	fallen := map[int32]cand{}
	naming := func(c *cand) bool {
		if c.srcOf <= 0 || c.e.SrcMasterInstanceID == 0 {
			return false
		}
		x := m.ents[c.srcOf-1]
		return x.master != nil && x.master.present(c.t)
	}

	// What is kept takes the place of what is dropped before it.
	out := kept[:0]
	for i := range kept {
		c := &kept[i]
		restated := st.settle(c)
		if c.bound > 0 && c.ent != 0 {
			// What the agent was is to tell again if it comes back.
			for sl := slotNone; sl <= slotMarker; sl++ {
				delete(states, slotKey{c.ent, sl})
			}
			delete(mk.worn, c.ent)
			gone[c.ent] = true
			if c.k.kind == evtc.StateTeamChange && c.e.Value == 0 {
				// A leave tells the team the agent had. The client that
				// held it last may not have learned it: the ones that
				// did tell it.
				c.e.Value = int32(m.teamOf(c.ent, c.t))
			}
		}
		if c.was && !gone[c.ent] {
			// The first change the combined log holds tells it.
			continue
		}
		switch {
		case c.names:
			restated = true
		case c.slot == slotMarker:
			if k := (telling{c.ent, c.entry, c.e.Time, c.carried}); !mk.judged[k] {
				mk.judge(kept, k)
			}
			restated = mk.again[i]
		case c.slot != slotNone:
			k := slotKey{c.ent, c.slot}
			h, ok := states[k]
			switch {
			case ok && c.carried && !c.was && c.told <= h.at:
				// The entry that spoke before told the state later
				// than the one that takes over: what the latter held
				// is older.
				restated = true
			case !ok || h.value != c.value:
				states[k] = held{c.value, c.entry, c.e.Time}
			case c.carried:
				// Nothing to carry over: the combined log holds it.
				restated = true
			case h.entry == c.entry:
				states[k] = held{c.value, c.entry, c.e.Time}
			case (!c.gained || c.slot.moving()) && c.e.Time-h.at > uint64(sameBound/time.Millisecond):
				// What a client tells of an agent it holds is an event
				// of its log, the value new or not. Within sameBound
				// of the same telling by the entry that spoke before,
				// it is that telling, dated by another client.
				states[k] = held{c.value, c.entry, c.e.Time}
			case m.anew(c, h):
				states[k] = held{c.value, c.entry, c.e.Time}
			default:
				// The entry takes the state over: what it tells next
				// is its own, as in its log.
				restated = true
				states[k] = held{h.value, c.entry, h.at}
			}
		}
		if restated {
			if _, ok := fallen[c.srcOf]; !ok && c.stack == stackNone && c.slot != slotMarker && naming(c) {
				fallen[c.srcOf] = *c
			}
			continue
		}
		if naming(c) {
			mastered[c.srcOf] = true
		}
		out = append(out, *c)
	}
	for _, id := range slices.Sorted(maps.Keys(fallen)) {
		if !mastered[id] {
			c := fallen[id]
			j, _ := slices.BinarySearchFunc(out, c, before)
			out = slices.Insert(out, j, c)
		}
	}
	return out
}
