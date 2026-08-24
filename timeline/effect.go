package timeline

import (
	"time"

	"github.com/42atomys/evtc"
)

// Effect is a visual effect played on the ground or around an agent, from
// its creation event to its removal event.
//
// The exported fields are read-only after Build.
type Effect struct {
	// ID is the trackable id of the effect instance, 0 for the many
	// effects arcdps does not track: those are never removed by an event
	// and last their announced duration, or are instantaneous.
	ID uint32
	// EffectID is the content id of the effect.
	EffectID uint32
	// GUID is the content GUID of the effect, zero when the log carries no
	// association.
	GUID GUID
	// Agent is the agent the effect is played around, or the agent that
	// placed a ground effect; the Unknown sentinel when the log names
	// none.
	Agent *Agent
	// Ground is set for an effect placed on the ground rather than played
	// around an agent.
	Ground bool
	// Origin is the position of a ground effect.
	Origin Vec3
	// Orientation is the orientation of a ground effect.
	Orientation Vec3
	// Scale is the scale of a ground effect, 1 when the log gives none.
	Scale float32
	// MovingPlatform is set when the effect sits on a moving platform.
	MovingPlatform bool
	// Flags is the raw flags byte of a ground effect.
	Flags uint8
	// Duration is the announced duration of the effect, the default of
	// its content id when the event gives none, 0 when unknown.
	Duration time.Duration
	// Interval runs from the creation to the removal of the effect, to the
	// end of its duration when no removal was logged, or to the end of the
	// log.
	Interval Interval
	// Create is the creation event.
	Create *evtc.Event
	// Remove is the removal event, nil when none was logged.
	Remove *evtc.Event

	// closed is set when a removal or the reuse of the id ended the
	// effect, so that its announced duration no longer applies.
	closed bool
}

// Removed reports whether the log holds the removal of the effect.
func (e *Effect) Removed() bool { return e.Remove != nil }

// Effects is a query over effects sorted by creation time.
type Effects struct{ Query[Effect] }

// Where keeps the effects accepted by p.
func (q Effects) Where(p func(*Effect) bool) Effects { return Effects{q.Query.Where(p)} }

// Between keeps the effects present at some instant of iv.
func (q Effects) Between(iv Interval) Effects {
	return Effects{overlapping(q.Query, effectStart, effectEnd, iv)}
}

// At keeps the effects present at t.
func (q Effects) At(t time.Duration) Effects { return q.Between(At(t)) }

// By keeps the effects played around e or placed by it.
func (q Effects) By(e Entity) Effects {
	a := ref(e)
	if a == nil {
		return q.Where(never[Effect])
	}
	return q.Where(func(f *Effect) bool { return f.Agent == a })
}

// OfID keeps the effects with the given content id.
func (q Effects) OfID(id uint32) Effects {
	return q.Where(func(f *Effect) bool { return f.EffectID == id })
}

// Ground keeps the effects placed on the ground.
func (q Effects) Ground() Effects { return q.Where(func(f *Effect) bool { return f.Ground }) }

// Around keeps the effects played around an agent.
func (q Effects) Around() Effects { return q.Where(func(f *Effect) bool { return !f.Ground }) }

// Skip drops the first n effects of the traversal.
func (q Effects) Skip(n int) Effects {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n effects.
func (q Effects) Limit(n int) Effects {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the effects from the latest to the earliest.
func (q Effects) Reverse() Effects {
	q.Query = q.Query.Reverse()
	return q
}

// GroupBy partitions the matching effects by the key returned by f. Each
// group is in creation order.
func (q Effects) GroupBy[K comparable](f func(*Effect) K) map[K]Effects {
	return groupInto(q.Query, f, func(items []*Effect) Effects { return Effects{From(items)} })
}

// EffectShare is the share of one effect id in a set of effects.
type EffectShare struct {
	// EffectID is the content id of the share.
	EffectID uint32
	// GUID is the content GUID of the effect, zero when unknown.
	GUID GUID
	// Effects are the effects with that id, in creation order.
	Effects Effects
}

// PerID partitions the matching effects by content id and returns the
// shares by decreasing number of effects; ids with as many effects keep
// the order in which the traversal met them. It is nil when there are no
// effects.
func (q Effects) PerID() []EffectShare {
	groups := ranked(q.Query, func(f *Effect) uint32 { return f.EffectID }, one[Effect])
	if len(groups) == 0 {
		return nil
	}
	out := make([]EffectShare, len(groups))
	for i, g := range groups {
		out[i] = EffectShare{EffectID: g.key, GUID: g.items[0].GUID, Effects: Effects{From(g.items)}}
	}
	return out
}

func effectStart(f *Effect) time.Duration { return f.Interval.Start }
func effectEnd(f *Effect) time.Duration   { return f.Interval.End }
