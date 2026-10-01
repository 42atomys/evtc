package combine

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
