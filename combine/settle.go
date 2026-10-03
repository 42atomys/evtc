package combine

import (
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
