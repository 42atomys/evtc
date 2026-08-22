package timeline

import (
	"time"

	"github.com/42atomys/evtc"
)

// Cast is one skill animation, from its StateAnimationStart event to its
// StateAnimationStop event. Hits returns the hits attributed to the cast.
//
// The exported fields are read-only after Build.
type Cast struct {
	// Start is the animation start event, nil when the cast began before
	// the log.
	Start *evtc.Event
	// Stop is the animation stop event, nil when the cast never ended
	// within the log.
	Stop *evtc.Event
	// Caster is the casting agent.
	Caster *Agent
	// Target is the agent the cast was aimed at, nil when there is none.
	Target *Agent
	// Skill is the skill cast.
	Skill *Skill
	// Interval runs from the start to the stop of the animation. Without a
	// stop event it ends at the expected end of the cast.
	Interval Interval
	// Expected is the time until the last significant trigger of the
	// skill, from the start event.
	Expected time.Duration
	// Control is the time until control is returned, from the start event.
	Control time.Duration
	// Elapsed is the time spent in the animation scaled for quickness and
	// slow, from the stop event.
	Elapsed time.Duration
	// ElapsedUnscaled is the wall time spent in the animation, from the
	// stop event.
	ElapsedUnscaled time.Duration
	// Activation tells how the animation ended; ActivationNone when it did
	// not end within the log.
	Activation evtc.Activation

	hits []*Hit
}

// Ended reports whether the cast has a stop event.
func (c *Cast) Ended() bool { return c.Stop != nil }

// Completed reports whether the skill went off: the animation reached its
// first trigger point (Minimum, NoData) or played to its end (Reset).
func (c *Cast) Completed() bool {
	switch c.Activation {
	case evtc.ActivationMinimum, evtc.ActivationNoData, evtc.ActivationReset:
		return true
	}
	return false
}

// Full reports whether the animation played to its end. Most casts are
// completed but not full: the skill went off and the next one cut the
// animation short.
func (c *Cast) Full() bool { return c.Activation == evtc.ActivationReset }

// Cancelled reports whether the animation was stopped before its first
// trigger point, so that the skill did not go off.
func (c *Cast) Cancelled() bool { return c.Activation == evtc.ActivationCancel }

// Duration returns the length of the animation.
func (c *Cast) Duration() time.Duration { return c.Interval.Duration() }

// Hits returns the hits attributed to the cast, in time order.
func (c *Cast) Hits() Hits { return Hits{From(c.hits)} }

// Casts is a query over casts sorted by start time.
type Casts struct{ Query[Cast] }

// Where keeps the casts accepted by p.
func (q Casts) Where(p func(*Cast) bool) Casts { return Casts{q.Query.Where(p)} }

// Between keeps the casts overlapping iv.
func (q Casts) Between(iv Interval) Casts {
	return Casts{overlapping(q.Query, castStart, castEnd, iv)}
}

// By keeps the casts of e.
func (q Casts) By(e Entity) Casts {
	a := ref(e)
	if a == nil {
		return q.Where(never[Cast])
	}
	return q.Where(func(c *Cast) bool { return c.Caster == a })
}

// On keeps the casts aimed at e.
func (q Casts) On(e Entity) Casts {
	a := ref(e)
	if a == nil {
		return q.Where(never[Cast])
	}
	return q.Where(func(c *Cast) bool { return c.Target == a })
}

// OfSkill keeps the casts of the skill id.
func (q Casts) OfSkill(id uint32) Casts {
	return q.Where(func(c *Cast) bool { return c.Skill.ID == id })
}

// Of keeps the casts of the skill.
func (q Casts) Of(s *Skill) Casts {
	return q.Where(func(c *Cast) bool { return c.Skill == s })
}

// Completed keeps the casts whose skill went off.
func (q Casts) Completed() Casts { return q.Where((*Cast).Completed) }

// Full keeps the casts whose animation played to its end.
func (q Casts) Full() Casts { return q.Where((*Cast).Full) }

// Cancelled keeps the casts stopped before their first trigger point.
func (q Casts) Cancelled() Casts { return q.Where((*Cast).Cancelled) }

// Hits returns the hits of the matching casts, sorted by time.
func (q Casts) Hits() Hits {
	n := 0
	for c := range q.Seq() {
		n += len(c.hits)
	}
	out := make([]*Hit, 0, n)
	for c := range q.Seq() {
		out = append(out, c.hits...)
	}
	return Hits{From(sortedByTime(out, hitTime))}
}

// GroupBy partitions the matching casts by the key returned by f. Each
// group is in start order.
func (q Casts) GroupBy[K comparable](f func(*Cast) K) map[K]Casts {
	return groupInto(q.Query, f, func(items []*Cast) Casts { return Casts{From(items)} })
}

func castStart(c *Cast) time.Duration { return c.Interval.Start }
func castEnd(c *Cast) time.Duration   { return c.Interval.End }

// Skip drops the first n casts of the traversal.
func (q Casts) Skip(n int) Casts {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n casts.
func (q Casts) Limit(n int) Casts {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the casts from the latest to the earliest.
func (q Casts) Reverse() Casts {
	q.Query = q.Query.Reverse()
	return q
}

// CastShare is the share of one skill in a set of casts.
type CastShare struct {
	// Skill is the skill of the share.
	Skill *Skill
	// Casts are the casts of the skill, in time order.
	Casts Casts
}

// PerSkill partitions the matching casts by skill and returns the shares
// by decreasing number of casts; skills cast as often keep the order in
// which the traversal met them. The casts of a share are in start order.
// It is nil when there are no casts.
func (q Casts) PerSkill() []CastShare {
	groups := ranked(q.Query, func(c *Cast) *Skill { return c.Skill }, one[Cast])
	if len(groups) == 0 {
		return nil
	}
	out := make([]CastShare, len(groups))
	for i, g := range groups {
		out[i] = CastShare{Skill: g.key, Casts: Casts{From(g.items)}}
	}
	return out
}
