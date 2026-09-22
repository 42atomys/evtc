package healingstats

import (
	"cmp"
	"slices"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// Heal is one heal or barrier application the addon logged: the direct
// effect of a skill, the tick of a buff such as regeneration, or the
// barrier a skill or buff gave.
//
// Src is never nil: an unknown source (environment, out of range) is the
// Unknown node of the stats. Cast is nil when the heal is not attributed
// to a cast.
//
// The exported fields are read-only once the timeline is built.
type Heal struct {
	// Event is the record of the heal. When both the client of the source
	// and the client of the destination wrote it, Event is the record of
	// the recording player's own client and PeerEvent the other one.
	Event *evtc.Event
	// PeerEvent is the second record of the heal, nil when it was written
	// once. Both records describe the same heal; only the client that
	// wrote them differs.
	PeerEvent *evtc.Event
	// Time is the time of the heal.
	Time time.Duration
	// Src is the healing agent, never nil.
	Src *Agent
	// Dst is the healed agent, never nil.
	Dst *Agent
	// Skill is the skill or buff that healed, never nil.
	Skill *timeline.Skill
	// Cast is the cast the heal belongs to: the most recent cast of the
	// same skill by the same agent started before the heal, nil when there
	// is none.
	Cast *timeline.Cast
	// Amount is the health restored or the barrier given, as the addon
	// logs it.
	Amount int32
	// IsBarrier is set when the skill gave barrier rather than health.
	IsBarrier bool
	// IsBuff is set for the tick of a buff, such as regeneration, rather
	// than the direct effect of a skill.
	IsBuff bool
	// TargetDowned is set when the target was downed.
	TargetDowned bool
	// SrcRecorded is set when the client of the source, or of its master,
	// wrote the heal.
	SrcRecorded bool
	// DstRecorded is set when the client of the destination, or of its
	// master, wrote the heal.
	DstRecorded bool
	// IFF is the friend or foe relation of the source to the target, as
	// logged.
	IFF evtc.IFF
	// OverNinety is set when the source was above 90% health.
	OverNinety bool
	// UnderFifty is set when the target was below 50% health.
	UnderFifty bool
	// Moving is set when the source was moving.
	Moving bool
	// TargetMoving is set when the target was moving.
	TargetMoving bool
}

// Credited returns the agent credited for the heal: the master of a
// minion source, otherwise the source itself.
func (h *Heal) Credited() *Agent {
	if m := h.Src.Master; m != nil {
		return h.Src.Stats.byAgent[m]
	}
	return h.Src
}

// creditedTo reports whether the source of the heal or its master is one
// of the agents of w.
func (h *Heal) creditedTo(w who) bool { return w.is(h.Src.Agent) || w.is(h.Src.Master) }

// Self reports whether the agent healed itself.
func (h *Heal) Self() bool { return h.Src == h.Dst }

// IsHealing reports whether the heal restored health rather than gave
// barrier.
func (h *Heal) IsHealing() bool { return !h.IsBarrier }

// IsDirect reports whether the heal is the direct effect of a skill rather
// than the tick of a buff.
func (h *Heal) IsDirect() bool { return !h.IsBuff }

// Healed returns the health the heal restored: Amount for a heal, 0 for
// barrier.
func (h *Heal) Healed() int32 {
	if h.IsBarrier {
		return 0
	}
	return h.Amount
}

// BarrierGiven returns the barrier the heal gave: Amount for barrier, 0
// for a heal.
func (h *Heal) BarrierGiven() int32 {
	if h.IsBarrier {
		return h.Amount
	}
	return 0
}

// Heals is a query over heals sorted by time, with the lazy semantics of
// timeline.Query: filters compose a predicate, Skip, Limit and Reverse
// describe the traversal, and nothing is copied until a terminal is
// called.
type Heals struct{ timeline.Query[Heal] }

// Where keeps the heals accepted by p.
func (q Heals) Where(p func(*Heal) bool) Heals { return Heals{q.Query.Where(p)} }

// Between keeps the heals that happened within iv.
func (q Heals) Between(iv timeline.Interval) Heals { return Heals{q.Query.Narrow(healTime, iv)} }

// never is the predicate of a filter given a nil entity.
func never(*Heal) bool { return false }

// By keeps the heals dealt by e.
func (q Heals) By(e timeline.Entity) Heals {
	w := agentsOf(e)
	if w.one == nil {
		return q.Where(never)
	}
	return q.Where(func(h *Heal) bool { return w.is(h.Src.Agent) })
}

// CreditedTo keeps the heals dealt by e or by one of its minions.
func (q Heals) CreditedTo(e timeline.Entity) Heals {
	w := agentsOf(e)
	if w.one == nil {
		return q.Where(never)
	}
	return q.Where(func(h *Heal) bool { return h.creditedTo(w) })
}

// On keeps the heals received by e.
func (q Heals) On(e timeline.Entity) Heals {
	w := agentsOf(e)
	if w.one == nil {
		return q.Where(never)
	}
	return q.Where(func(h *Heal) bool { return w.is(h.Dst.Agent) })
}

// OfSkill keeps the heals of the skill id.
func (q Heals) OfSkill(id uint32) Heals {
	return q.Where(func(h *Heal) bool { return h.Skill.ID == id })
}

// Of keeps the heals of the skill.
func (q Heals) Of(s *timeline.Skill) Heals {
	return q.Where(func(h *Heal) bool { return h.Skill == s })
}

// OfCast keeps the heals attributed to the cast.
func (q Heals) OfCast(c *timeline.Cast) Heals {
	if c == nil {
		return q.Where(never)
	}
	return q.Where(func(h *Heal) bool { return h.Cast == c })
}

// Healing keeps the heals that restored health.
func (q Heals) Healing() Heals { return q.Where((*Heal).IsHealing) }

// Barrier keeps the barrier applications.
func (q Heals) Barrier() Heals { return q.Where(func(h *Heal) bool { return h.IsBarrier }) }

// Direct keeps the direct effects of skills.
func (q Heals) Direct() Heals { return q.Where((*Heal).IsDirect) }

// Ticks keeps the ticks of buffs, such as regeneration.
func (q Heals) Ticks() Heals { return q.Where(func(h *Heal) bool { return h.IsBuff }) }

// Downed keeps the heals received by a downed target.
func (q Heals) Downed() Heals { return q.Where(func(h *Heal) bool { return h.TargetDowned }) }

// Self keeps the heals an agent dealt to itself.
func (q Heals) Self() Heals { return q.Where((*Heal).Self) }

// Others keeps the heals dealt to someone else.
func (q Heals) Others() Heals { return q.Where(func(h *Heal) bool { return !h.Self() }) }

// Skip drops the first n heals of the traversal.
func (q Heals) Skip(n int) Heals {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n heals.
func (q Heals) Limit(n int) Heals {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the heals from the latest to the earliest.
func (q Heals) Reverse() Heals {
	q.Query = q.Query.Reverse()
	return q
}

// Amount sums the amounts of the matching heals, health and barrier alike.
func (q Heals) Amount() int64 {
	return q.Sum(func(h *Heal) int64 { return int64(h.Amount) })
}

// Healed sums the health restored by the matching heals.
func (q Heals) Healed() int64 {
	return q.Sum(func(h *Heal) int64 { return int64(h.Healed()) })
}

// BarrierGiven sums the barrier given by the matching heals.
func (q Heals) BarrierGiven() int64 {
	return q.Sum(func(h *Heal) int64 { return int64(h.BarrierGiven()) })
}

// HPS returns the health restored by the matching heals within iv per
// second of iv, 0 when iv is empty.
func (q Heals) HPS(iv timeline.Interval) float64 {
	if iv.Duration() <= 0 {
		return 0
	}
	return float64(q.Between(iv).Healed()) / iv.Duration().Seconds()
}

// BPS returns the barrier given by the matching heals within iv per
// second of iv, 0 when iv is empty.
func (q Heals) BPS(iv timeline.Interval) float64 {
	if iv.Duration() <= 0 {
		return 0
	}
	return float64(q.Between(iv).BarrierGiven()) / iv.Duration().Seconds()
}

// GroupBy partitions the matching heals by the key returned by f. Each
// group is in time order.
func (q Heals) GroupBy[K comparable](f func(*Heal) K) map[K]Heals {
	groups := q.Query.GroupBy(f)
	out := make(map[K]Heals, len(groups))
	for k, items := range groups {
		if q.Reversed() {
			slices.Reverse(items)
		}
		out[k] = Heals{timeline.From(items)}
	}
	return out
}

// Contribution is the share of one agent in a set of heals: the heals
// credited to the agent, minions included.
type Contribution struct {
	// Agent is the credited agent.
	Agent *Agent
	// Heals are the heals of the share, in time order.
	Heals Heals
}

// SkillShare is the share of one skill in a set of heals.
type SkillShare struct {
	// Skill is the skill of the share.
	Skill *timeline.Skill
	// Heals are the heals of the skill, in time order.
	Heals Heals
}

// PerAgent partitions the matching heals by credited agent (see
// Heal.Credited) and returns the shares by decreasing Amount; agents with
// an equal amount keep the order in which the traversal met them. The
// heals of a share are in time order. It is nil when there are no heals.
func (q Heals) PerAgent() []Contribution {
	return contributions(q, (*Heal).Credited)
}

// PerTarget partitions the matching heals by the agent that received them,
// with the ordering rules of PerAgent.
func (q Heals) PerTarget() []Contribution {
	return contributions(q, func(h *Heal) *Agent { return h.Dst })
}

// contributions ranks the heals of q by the agent returned by key.
func contributions(q Heals, key func(*Heal) *Agent) []Contribution {
	groups := ranked(q, key)
	if len(groups) == 0 {
		return nil
	}
	out := make([]Contribution, len(groups))
	for i, g := range groups {
		out[i] = Contribution{Agent: g.key, Heals: Heals{timeline.From(g.items)}}
	}
	return out
}

// PerSkill partitions the matching heals by skill and returns the shares
// by decreasing Amount, with the ordering rules of PerAgent. It is nil
// when there are no heals.
func (q Heals) PerSkill() []SkillShare {
	groups := ranked(q, func(h *Heal) *timeline.Skill { return h.Skill })
	if len(groups) == 0 {
		return nil
	}
	out := make([]SkillShare, len(groups))
	for i, g := range groups {
		out[i] = SkillShare{Skill: g.key, Heals: Heals{timeline.From(g.items)}}
	}
	return out
}

// group is one partition of a traversal: its key, its heals and the
// amount the partitions are ranked by.
type group[K comparable] struct {
	key    K
	items  []*Heal
	amount int64
}

// ranked partitions the traversal by key and sorts the groups by
// decreasing amount. Groups of an equal amount keep the order in which the
// traversal met them, and every group is put back in time order when the
// traversal ran backwards.
func ranked[K comparable](q Heals, key func(*Heal) K) []group[K] {
	var groups []group[K]
	index := map[K]int{}
	for h := range q.Seq() {
		k := key(h)
		i, ok := index[k]
		if !ok {
			i = len(groups)
			index[k] = i
			groups = append(groups, group[K]{key: k})
		}
		groups[i].items = append(groups[i].items, h)
		groups[i].amount += int64(h.Amount)
	}
	if q.Reversed() {
		for i := range groups {
			slices.Reverse(groups[i].items)
		}
	}
	slices.SortStableFunc(groups, func(a, b group[K]) int { return cmp.Compare(b.amount, a.amount) })
	return groups
}

func healTime(h *Heal) time.Duration { return h.Time }
