package combine

import (
	"github.com/42atomys/evtc"
)

// strikesOf returns the strikes of a skill in a log, in log order.
func strikesOf(l *evtc.Log, skill uint32) []evtc.Event {
	var out []evtc.Event
	for _, e := range of(l, evtc.StateCombat) {
		if e.SkillID == skill && e.Buff == 0 {
			out = append(out, e)
		}
	}
	return out
}
