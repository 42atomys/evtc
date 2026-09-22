package timeline

import (
	"time"

	"github.com/42atomys/evtc"
)

// Missile is a projectile, from its creation event to its removal event,
// with every launch in between.
//
// The exported fields are read-only after Build.
type Missile struct {
	// ID is the trackable id of the missile.
	ID uint32
	// Skill is the skill that fired the missile.
	Skill *Skill
	// Owner is the agent the missile belongs to; the Unknown sentinel when
	// the log names none.
	Owner *Agent
	// Origin is the point where the missile was created.
	Origin Vec3
	// Skin is the skin id of a player missile, 0 otherwise.
	Skin uint32
	// Interval runs from the creation to the removal of the missile, or to
	// the end of the log.
	Interval Interval
	// Launches are the launches of the missile, in time order.
	Launches []Launch
	// Effects are the effects applied to the missile, in time order.
	Effects []MissileEffect
	// FriendlyFire is the total friendly fire damage the missile dealt,
	// from the removal event.
	FriendlyFire int32
	// HitEnemy is set when the missile hit at least one enemy, from the
	// removal event.
	HitEnemy bool
	// RemovedAt is the point where the missile was removed.
	RemovedAt Vec3
	// Create is the creation event.
	Create *evtc.Event
	// Remove is the removal event, nil when none was logged.
	Remove *evtc.Event
}

// Removed reports whether the log holds the removal of the missile.
func (m *Missile) Removed() bool { return m.Remove != nil }

// Target returns the agent the first launch aimed at, nil when there is
// none.
func (m *Missile) Target() *Agent {
	for i := range m.Launches {
		if m.Launches[i].Target != nil {
			return m.Launches[i].Target
		}
	}
	return nil
}

// Launch is one launch of a missile.
type Launch struct {
	// Time is the time of the launch.
	Time time.Duration
	// Target is the agent aimed at, nil when the launch aims at a point.
	Target *Agent
	// TargetPos is the point aimed at.
	TargetPos Vec3
	// Position is the point launched from.
	Position Vec3
	// Motion is the raw motion type of the client.
	Motion uint8
	// Radius is the motion radius.
	Radius int16
	// Flags are the raw launch flags of the client.
	Flags uint32
	// Speed is the missile speed.
	Speed int16
	// First is set for the first launch of the missile.
	First bool
	// Event is the launch event.
	Event *evtc.Event
}

// MissileEffect is an effect applied to a missile.
type MissileEffect struct {
	// Time is the time the effect was applied.
	Time time.Duration
	// EffectID is the content id of the effect.
	EffectID uint32
	// GUID is the content GUID of the effect, zero when unknown.
	GUID GUID
	// Duration is the duration of the effect.
	Duration time.Duration
	// Event is the missile effect event.
	Event *evtc.Event
}

// Missiles is a query over missiles sorted by creation time.
type Missiles struct{ Query[Missile] }

// Where keeps the missiles accepted by p.
func (q Missiles) Where(p func(*Missile) bool) Missiles { return Missiles{q.Query.Where(p)} }

// Between keeps the missiles alive at some instant of iv.
func (q Missiles) Between(iv Interval) Missiles {
	return Missiles{overlapping(q.Query, missileStart, missileEnd, iv)}
}

// At keeps the missiles alive at t.
func (q Missiles) At(t time.Duration) Missiles { return q.Between(At(t)) }

// By keeps the missiles owned by e.
func (q Missiles) By(e Entity) Missiles {
	w := agentsOf(e)
	if w.none() {
		return q.Where(never[Missile])
	}
	return q.Where(func(m *Missile) bool { return w.is(m.Owner) })
}

// OfSkill keeps the missiles of the skill id.
func (q Missiles) OfSkill(id uint32) Missiles {
	return q.Where(func(m *Missile) bool { return m.Skill.ID == id })
}

// Of keeps the missiles of the skill.
func (q Missiles) Of(s *Skill) Missiles {
	return q.Where(func(m *Missile) bool { return m.Skill == s })
}

// Skip drops the first n missiles of the traversal.
func (q Missiles) Skip(n int) Missiles {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n missiles.
func (q Missiles) Limit(n int) Missiles {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the missiles from the latest to the earliest.
func (q Missiles) Reverse() Missiles {
	q.Query = q.Query.Reverse()
	return q
}

// GroupBy partitions the matching missiles by the key returned by f. Each
// group is in creation order.
func (q Missiles) GroupBy[K comparable](f func(*Missile) K) map[K]Missiles {
	return groupInto(q.Query, f, func(items []*Missile) Missiles { return Missiles{From(items)} })
}

// MissileShare is the share of one skill in a set of missiles.
type MissileShare struct {
	// Skill is the skill of the share.
	Skill *Skill
	// Missiles are the missiles of the skill, in creation order.
	Missiles Missiles
}

// PerSkill partitions the matching missiles by skill and returns the
// shares by decreasing number of missiles; skills with as many missiles
// keep the order in which the traversal met them. It is nil when there
// are no missiles.
func (q Missiles) PerSkill() []MissileShare {
	groups := ranked(q.Query, func(m *Missile) *Skill { return m.Skill }, one[Missile])
	if len(groups) == 0 {
		return nil
	}
	out := make([]MissileShare, len(groups))
	for i, g := range groups {
		out[i] = MissileShare{Skill: g.key, Missiles: Missiles{From(g.items)}}
	}
	return out
}

func missileStart(m *Missile) time.Duration { return m.Interval.Start }
func missileEnd(m *Missile) time.Duration   { return m.Interval.End }
