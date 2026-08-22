package timeline

import (
	"time"

	"github.com/42atomys/evtc"
)

// Down is one downed state of an agent.
//
// The exported fields are read-only after Build.
type Down struct {
	// Interval runs from the down to the revive or the death, or to the
	// end of the log.
	Interval
	// Agent is the downed agent.
	Agent *Agent
	// Cause is the hit that downed the agent, nil when none was found.
	Cause *Hit
	// Recovered is set when the agent got back up.
	Recovered bool
	// Death is set when the down ended with the agent dying.
	Death *Death
	// Event is the StateChangeDown event.
	Event *evtc.Event
}

// Death is one death of an agent.
//
// The exported fields are read-only after Build.
type Death struct {
	// Time is the time of the death.
	Time time.Duration
	// Agent is the dead agent.
	Agent *Agent
	// Cause is the killing blow, nil when none was found.
	Cause *Hit
	// Down is the downed state the death ended, nil when the agent died
	// outright.
	Down *Down
	// Event is the StateChangeDead event.
	Event *evtc.Event
}

// Breakbar is one period during which the defiance bar of an agent was
// active. Hits returns the defiance hits received meanwhile: crowd control
// damage, and regeneration ticks with a negative Damage.
//
// The exported fields are read-only after Build.
type Breakbar struct {
	// Agent is the agent whose bar was active.
	Agent *Agent
	// Interval is the period during which the bar was active.
	Interval
	// Percent is the bar percentage over the period.
	Percent Numbers[float64]
	// End is the state the bar went to when the period ended: Recover or
	// Immune when it was broken, None when the log ended first.
	End DefianceState

	hits []*Hit
}

// Hits returns the defiance hits received while the bar was active, in
// time order: crowd control damage, and regeneration ticks with a negative
// Damage.
func (b *Breakbar) Hits() Hits { return Hits{From(b.hits)} }

// brokenPercent is the bar percentage under which a bar counts as broken:
// arcdps rarely logs exactly zero.
const brokenPercent = 1.0

// Broken reports whether the bar was broken during the period: it went to
// the recovering state, or its percentage fell under brokenPercent.
func (b *Breakbar) Broken() bool {
	if b.End == DefianceRecover {
		return true
	}
	v, ok := b.Percent.Min()
	return ok && v < brokenPercent
}

// CCHits returns the crowd control hits received while the bar was
// active: the defiance hits with a positive Damage, leaving out the
// regeneration ticks of the bar.
func (b *Breakbar) CCHits() Hits { return b.Hits().Where(positiveDamage) }

func positiveDamage(h *Hit) bool { return h.Damage > 0 }

// CC sums the crowd control damage dealt to the bar by e or its minions.
func (b *Breakbar) CC(e Entity) int64 { return b.CCHits().CreditedTo(e).Damage() }

// TotalCC sums the crowd control damage dealt to the bar by everyone.
func (b *Breakbar) TotalCC() int64 { return b.CCHits().Damage() }
