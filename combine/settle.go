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
