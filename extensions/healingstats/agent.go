package healingstats

import "github.com/42atomys/evtc/timeline"

// Agent is the node of one agent of the timeline in the healing graph. It
// embeds the timeline agent, so that every field and method of the
// timeline is available on it and it is accepted wherever the timeline
// takes an Entity.
//
// The exported fields are read-only once the timeline is built.
type Agent struct {
	*timeline.Agent
	// Stats owns the node.
	Stats *Stats
	// Recorded is set when the client of the agent, or of its master,
	// wrote heals into the log: the agent is the recording player, a squad
	// member sharing its stats with it, or a minion of one of them.
	Recorded bool

	heals, healsTaken, healsCredited []*Heal
	cnt                              struct{ heals, taken int }
}

// Ref returns the timeline agent behind the node, nil for a nil node.
func (a *Agent) Ref() *timeline.Agent {
	if a == nil {
		return nil
	}
	return a.Agent
}

// Heals returns the heals dealt by the agent itself, in time order. The
// heals of its minions are in HealsCredited.
func (a *Agent) Heals() Heals {
	if a == nil {
		return Heals{}
	}
	return Heals{timeline.From(a.heals)}
}

// HealsCredited returns the heals dealt by the agent and by its minions
// (pets, clones, turrets, mechs), in time order: what a healing meter
// credits to a player.
func (a *Agent) HealsCredited() Heals {
	if a == nil {
		return Heals{}
	}
	if a.healsCredited != nil {
		return Heals{timeline.From(a.healsCredited)}
	}
	return Heals{timeline.From(a.heals)}
}

// HealsTaken returns the heals received by the agent, in time order.
func (a *Agent) HealsTaken() Heals {
	if a == nil {
		return Heals{}
	}
	return Heals{timeline.From(a.healsTaken)}
}
