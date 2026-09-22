package timeline

import (
	"cmp"
	"slices"
	"time"

	"github.com/42atomys/evtc"
)

// Hit is one combat event: a strike, a buff damage tick, a blocked or
// evaded attack, a defiance bar hit, a crowd control or a cast signal. The
// Result tells which.
//
// Src is never nil: an unknown source (environment, out of range) is the
// Unknown sentinel of the timeline. Cast, Down and Death are nil when the
// hit is not attributed to a cast or had no such consequence.
//
// The exported fields are read-only after Build.
type Hit struct {
	// Event is the combat event of the hit.
	Event *evtc.Event
	// Time is the time of the hit.
	Time time.Duration
	// Src is the agent dealing the hit, never nil.
	Src *Agent
	// Dst is the agent receiving the hit, never nil.
	Dst *Agent
	// Skill is the skill or buff that dealt the hit.
	Skill *Skill
	// Cast is the cast the hit belongs to: the most recent cast of the
	// same skill by the same agent started before the hit.
	Cast *Cast
	// Result is the outcome of the hit: normal, critical, blocked, evaded
	// and so on.
	Result evtc.Result
	// Damage is the damage dealt as arcdps logs it, health and barrier
	// parts combined: strike damage for strikes, buff damage for ticks.
	Damage int32
	// Barrier is the part of Damage absorbed by barrier; HealthDamage is
	// the rest.
	Barrier int32
	// IsBuff is set for buff damage ticks.
	IsBuff bool
	// IFF is the friend or foe relation of the source to the target.
	IFF evtc.IFF
	// OverNinety is set when the source was above 90% health.
	OverNinety bool
	// UnderFifty is set when the target was below 50% health.
	UnderFifty bool
	// Moving is set when the source was moving.
	Moving bool
	// TargetMoving is set when the target was moving.
	TargetMoving bool
	// Flanking is set when the source was flanking the target.
	Flanking bool
	// Shielded is set when barrier absorbed part of the hit.
	Shielded bool
	// TargetDowned is set when the target was down.
	TargetDowned bool
	// Down is the down state the hit caused, nil otherwise.
	Down *Down
	// Death is the death the hit caused, nil otherwise.
	Death *Death
}

// HealthDamage returns the damage that reached health: Damage minus the
// part absorbed by barrier.
func (h *Hit) HealthDamage() int32 { return h.Damage - h.Barrier }

// IsStrike reports whether the hit is a landed strike (normal, critical or
// glancing).
func (h *Hit) IsStrike() bool {
	return !h.IsBuff && h.Result <= evtc.ResultStrikeDamageGlance
}

// IsBuffDamage reports whether the hit is a buff damage tick.
func (h *Hit) IsBuffDamage() bool {
	return h.IsBuff && h.Result >= evtc.ResultBuffDamageCycle && h.Result < evtc.ResultUnknown
}

// Landed reports whether the hit connected: a strike or a buff tick.
func (h *Hit) Landed() bool { return h.IsStrike() || h.IsBuffDamage() }

// Crit reports whether the strike was critical.
func (h *Hit) Crit() bool { return h.Result == evtc.ResultStrikeDamageCrit }

// Glance reports whether the strike glanced.
func (h *Hit) Glance() bool { return h.Result == evtc.ResultStrikeDamageGlance }

// Blocked reports whether the attack was blocked.
func (h *Hit) Blocked() bool { return h.Result == evtc.ResultBlock }

// Evaded reports whether the attack was evaded.
func (h *Hit) Evaded() bool { return h.Result == evtc.ResultEvade }

// Absorbed reports whether the attack hit an invulnerable target.
func (h *Hit) Absorbed() bool { return h.Result == evtc.ResultAbsorb }

// Missed reports whether the attack missed because of blindness.
func (h *Hit) Missed() bool { return h.Result == evtc.ResultBlind }

// Interrupted reports whether the hit interrupted the target.
func (h *Hit) Interrupted() bool { return h.Result == evtc.ResultInterrupt }

// Downing reports whether the hit downed the target.
func (h *Hit) Downing() bool { return h.Result == evtc.ResultDowned }

// Killing reports whether the hit killed the target.
func (h *Hit) Killing() bool { return h.Result == evtc.ResultKillingBlow }

// IsDefiance reports whether the hit damaged (or regenerated, with a
// negative Damage) a defiance bar.
func (h *Hit) IsDefiance() bool { return h.Result == evtc.ResultDefianceDamageNormal }

// IsCrowdControl reports whether the hit crowd controlled the target.
func (h *Hit) IsCrowdControl() bool { return h.Result == evtc.ResultCrowdControl }

// Credited returns the agent credited for the hit: the master of a minion
// source, otherwise the source itself.
func (h *Hit) Credited() *Agent {
	if h.Src.Master != nil {
		return h.Src.Master
	}
	return h.Src
}

// creditedTo reports whether the source of the hit or its master is one
// of the agents of w.
func (h *Hit) creditedTo(w who) bool { return w.is(h.Src) || w.is(h.Src.Master) }

// IsSignal reports whether the event is an on-skill-use signal rather
// than a hit.
func (h *Hit) IsSignal() bool { return h.Result == evtc.ResultSkillCast }

// Hits is a query over hits sorted by time.
type Hits struct{ Query[Hit] }

// Where keeps the hits accepted by p.
func (q Hits) Where(p func(*Hit) bool) Hits { return Hits{q.Query.Where(p)} }

// Between keeps the hits that happened within iv.
func (q Hits) Between(iv Interval) Hits {
	q.items = narrow(q.items, hitTime, iv)
	return q
}

// By keeps the hits dealt by e.
func (q Hits) By(e Entity) Hits {
	w := agentsOf(e)
	if w.none() {
		return q.Where(never[Hit])
	}
	return q.Where(func(h *Hit) bool { return w.is(h.Src) })
}

// CreditedTo keeps the hits dealt by e or by one of its minions.
func (q Hits) CreditedTo(e Entity) Hits {
	w := agentsOf(e)
	if w.none() {
		return q.Where(never[Hit])
	}
	return q.Where(func(h *Hit) bool { return h.creditedTo(w) })
}

// On keeps the hits received by e.
func (q Hits) On(e Entity) Hits {
	w := agentsOf(e)
	if w.none() {
		return q.Where(never[Hit])
	}
	return q.Where(func(h *Hit) bool { return w.is(h.Dst) })
}

// OfSkill keeps the hits of the skill id.
func (q Hits) OfSkill(id uint32) Hits {
	return q.Where(func(h *Hit) bool { return h.Skill.ID == id })
}

// Of keeps the hits of the skill.
func (q Hits) Of(s *Skill) Hits {
	return q.Where(func(h *Hit) bool { return h.Skill == s })
}

// WithResult keeps the hits with the given result.
func (q Hits) WithResult(r evtc.Result) Hits {
	return q.Where(func(h *Hit) bool { return h.Result == r })
}

// Strikes keeps the landed strikes.
func (q Hits) Strikes() Hits { return q.Where((*Hit).IsStrike) }

// BuffDamage keeps the buff damage ticks.
func (q Hits) BuffDamage() Hits { return q.Where((*Hit).IsBuffDamage) }

// Landed keeps the hits that connected.
func (q Hits) Landed() Hits { return q.Where((*Hit).Landed) }

// Blocked keeps the blocked attacks.
func (q Hits) Blocked() Hits { return q.Where((*Hit).Blocked) }

// Evaded keeps the evaded attacks.
func (q Hits) Evaded() Hits { return q.Where((*Hit).Evaded) }

// Absorbed keeps the attacks absorbed by invulnerability.
func (q Hits) Absorbed() Hits { return q.Where((*Hit).Absorbed) }

// Missed keeps the attacks that missed.
func (q Hits) Missed() Hits { return q.Where((*Hit).Missed) }

// Crits keeps the critical strikes.
func (q Hits) Crits() Hits { return q.Where((*Hit).Crit) }

// Defiance keeps the defiance bar hits.
func (q Hits) Defiance() Hits { return q.Where((*Hit).IsDefiance) }

// Skip drops the first n hits of the traversal.
func (q Hits) Skip(n int) Hits {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n hits.
func (q Hits) Limit(n int) Hits {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the hits from the latest to the earliest.
func (q Hits) Reverse() Hits {
	q.Query = q.Query.Reverse()
	return q
}

// Damage sums the damage of the matching hits, barrier part included.
func (q Hits) Damage() int64 {
	return q.Sum(func(h *Hit) int64 { return int64(h.Damage) })
}

// HealthDamage sums the damage of the matching hits that reached health.
func (q Hits) HealthDamage() int64 {
	return q.Sum(func(h *Hit) int64 { return int64(h.HealthDamage()) })
}

// Barrier sums the damage absorbed by barrier over the matching hits.
func (q Hits) Barrier() int64 {
	return q.Sum(func(h *Hit) int64 { return int64(h.Barrier) })
}

// DPS returns the damage of the matching hits within iv per second of iv,
// barrier part included, 0 when iv is empty.
func (q Hits) DPS(iv Interval) float64 {
	if iv.Duration() <= 0 {
		return 0
	}
	return float64(q.Between(iv).Damage()) / iv.Duration().Seconds()
}

// Contribution is the share of one agent in a set of hits: the hits
// credited to the agent, minions included.
type Contribution struct {
	// Agent is the credited agent.
	Agent *Agent
	// Hits are the hits credited to the agent, in time order.
	Hits Hits
}

// PerAgent partitions the matching hits by credited agent (see
// Hit.Credited) and returns the shares by decreasing Damage; agents
// with equal damage keep the order in which the traversal met them. The
// hits of a share are in time order. It is nil when there are no hits.
func (q Hits) PerAgent() []Contribution {
	return contributions(q.Query, (*Hit).Credited)
}

// PerTarget partitions the matching hits by the agent that received them
// and returns the shares by decreasing Damage; agents with equal damage
// keep the order in which the traversal met them. The hits of a share are
// in time order. It is nil when there are no hits.
func (q Hits) PerTarget() []Contribution {
	return contributions(q.Query, func(h *Hit) *Agent { return h.Dst })
}

// contributions ranks the hits of q by the agent returned by key.
func contributions(q Query[Hit], key func(*Hit) *Agent) []Contribution {
	groups := ranked(q, key, hitDamage)
	if len(groups) == 0 {
		return nil
	}
	out := make([]Contribution, len(groups))
	for i, g := range groups {
		out[i] = Contribution{Agent: g.key, Hits: Hits{From(g.items)}}
	}
	return out
}

// SkillShare is the share of one skill in a set of hits.
type SkillShare struct {
	// Skill is the skill of the share.
	Skill *Skill
	// Hits are the hits of the skill, in time order.
	Hits Hits
}

// PerSkill partitions the matching hits by skill and returns the shares by
// decreasing Damage; skills with equal damage keep the order in which the
// traversal met them. The hits of a share are in time order. It is nil
// when there are no hits.
func (q Hits) PerSkill() []SkillShare {
	groups := ranked(q.Query, func(h *Hit) *Skill { return h.Skill }, hitDamage)
	if len(groups) == 0 {
		return nil
	}
	out := make([]SkillShare, len(groups))
	for i, g := range groups {
		out[i] = SkillShare{Skill: g.key, Hits: Hits{From(g.items)}}
	}
	return out
}

func hitDamage(h *Hit) int64 { return int64(h.Damage) }

// group is one partition of a traversal: its key, its elements and the
// weight the partitions are ranked by.
type group[E any, K comparable] struct {
	key    K
	items  []*E
	weight int64
}

// ranked partitions the traversal by key and sorts the groups by
// decreasing weight. Groups of equal weight keep the order in which the
// traversal met them, and every group is put back in time order when the
// traversal ran backwards.
func ranked[E any, K comparable](q Query[E], key func(*E) K, weight func(*E) int64) []group[E, K] {
	var groups []group[E, K]
	index := map[K]int{}
	for e := range q.Seq() {
		k := key(e)
		i, ok := index[k]
		if !ok {
			i = len(groups)
			index[k] = i
			groups = append(groups, group[E, K]{key: k})
		}
		groups[i].items = append(groups[i].items, e)
		groups[i].weight += weight(e)
	}
	if q.desc {
		for i := range groups {
			slices.Reverse(groups[i].items)
		}
	}
	slices.SortStableFunc(groups, func(a, b group[E, K]) int { return cmp.Compare(b.weight, a.weight) })
	return groups
}

// GroupBy partitions the matching hits by the key returned by f. Each
// group is in time order.
func (q Hits) GroupBy[K comparable](f func(*Hit) K) map[K]Hits {
	return groupInto(q.Query, f, func(items []*Hit) Hits { return Hits{From(items)} })
}

func hitTime(h *Hit) time.Duration { return h.Time }

// groupInto is GroupBy with the groups wrapped by wrap. The wrapped types
// expect their items in time order, which a reversed traversal breaks, so
// the groups of such a traversal are turned back around.
func groupInto[E any, K comparable, W any](q Query[E], f func(*E) K, wrap func([]*E) W) map[K]W {
	groups := q.GroupBy(f)
	out := make(map[K]W, len(groups))
	for k, items := range groups {
		if q.desc {
			slices.Reverse(items)
		}
		out[k] = wrap(items)
	}
	return out
}

// one weights every element the same, for rankings by count.
func one[E any](*E) int64 { return 1 }

// Foes keeps the hits dealt to a foe of the source.
func (q Hits) Foes() Hits {
	return q.Where(func(h *Hit) bool { return h.IFF == evtc.IFFFoe })
}

// Friends keeps the hits dealt to a friend of the source.
func (q Hits) Friends() Hits {
	return q.Where(func(h *Hit) bool { return h.IFF == evtc.IFFFriend })
}
