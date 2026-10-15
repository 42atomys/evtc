package combine

import (
	"sort"
	"time"
)

// tell records what an entry told of a slot.
func (m *combiner) tell(c cand) {
	if c.entry == 0 || c.slot.moving() {
		return
	}
	s := m.tellings[c.entry]
	if s == nil {
		s = map[slot][]told{}
		m.tellings[c.entry] = s
	}
	s[c.slot] = append(s[c.slot], told{c.t, c})
}

// carry returns, for every agent whose speaker changes, what the new
// speaker held of each state when it started to speak: the last it told
// before. settle drops what the combined log holds already.
func (m *combiner) carry() []cand {
	var out []cand
	for _, x := range m.ents {
		for i := 1; i < len(x.speak); i++ {
			seg := x.speak[i]
			if seg.entry == x.speak[i-1].entry {
				continue
			}
			// What an entry holds alone after the others for less than
			// shared is the time its client took to learn that the
			// agent left: nothing to tell anew.
			if last := i == len(x.speak)-1 || x.speak[i+1].iv.Start-seg.iv.End > time.Millisecond; last && seg.iv.Duration() < shared {
				continue
			}
			before, tellings := m.before[seg.entry], m.tellings[seg.entry]
			for sl := slotNone; sl <= slotMarker; sl++ {
				l := tellings[sl]
				// The last telling before the entry speaks.
				j := sort.Search(len(l), func(j int) bool { return l[j].t >= seg.iv.Start }) - 1
				if b, ok := before[sl]; ok && j < 0 && b.t >= seg.iv.Start {
					// The timeline reads what a state was from the first
					// change of it: that of the agent, not of the entry.
					c := b.c
					c.was = true
					c.carried, c.pairable, c.paired, c.bound, c.stack = true, false, false, 0, stackNone
					c.origin = int8(seg.rec)
					c.t = seg.iv.Start
					c.e.Time = m.raw(c.t)
					out = append(out, c)
				}
				if j < 0 {
					continue
				}
				// The markers of an agent are told as a whole.
				from := j
				if sl == slotMarker {
					for from > 0 && l[from-1].t == l[j].t {
						from--
					}
				}
				for _, t := range l[from : j+1] {
					c := t.c
					c.carried, c.pairable, c.paired = true, false, false
					c.told = m.raw(t.t)
					c.t = seg.iv.Start
					// A time keeps its parity, inside the span of the
					// speaker.
					if (m.raw(c.t)^c.told)&1 != 0 {
						if c.t < seg.iv.End {
							c.t += time.Millisecond
						} else {
							c.t -= time.Millisecond
						}
					}
					c.e.Time = m.raw(c.t)
					out = append(out, c)
				}
			}
		}
	}
	return out
}
