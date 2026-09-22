package timeline

import (
	"cmp"
	"slices"
	"time"

	"github.com/42atomys/evtc"
)

// BuffStack is one stack of a buff on an agent, from its application to
// its removal. Stacks are matched exactly through the trackable id arcdps
// assigns to each of them.
//
// The exported fields are read-only after Build.
type BuffStack struct {
	// ID is the trackable id of the stack.
	ID uint32
	// Buff is the definition of the buff.
	Buff *Buff
	// Applier is the agent that applied the stack, the Unknown sentinel
	// when the source is unknown.
	Applier *Agent
	// Receiver is the agent carrying the stack.
	Receiver *Agent
	// Interval runs from the application to the removal of the stack, or
	// to the end of the log while the stack is still present.
	Interval Interval
	// Applied is the duration applied.
	Applied time.Duration
	// Original is the full duration of a stack that pre-existed the log.
	Original time.Duration
	// Extended is the total of the duration changes received.
	Extended time.Duration
	// Remaining is the duration left when the stack was removed.
	Remaining time.Duration
	// Initial is set for stacks already present when the log started.
	Initial bool
	// ActiveOnApply is set when the stack was ticking as soon as applied.
	ActiveOnApply bool
	// Superseded is set when another stack reused the trackable id before
	// any removal was seen; the stack then ends at that application and
	// has no Remove event.
	Superseded bool
	// EndedByDespawn is set when the receiver left tracking while the
	// stack was present: arcdps logs no removal for it, so the stack ends
	// at the despawn and has no Remove event.
	EndedByDespawn bool
	// Active is true over the spans where the stack was in effect: for a
	// buff that queues its durations, one stack is active at a time. It
	// starts with ActiveOnApply and follows the buff active and deactive
	// events.
	Active Spans[bool]
	// Apply is the application event.
	Apply *evtc.Event
	// Remove is the removal event, nil while the stack is present or when
	// it ended without one.
	Remove *evtc.Event
	// Changes are the duration change events of the stack, in time order;
	// Extended sums them.
	Changes []*evtc.Event
	// Removal tells how the stack ended, BuffRemoveNone while present.
	Removal evtc.BuffRemove
	// RemovedBy is the agent that removed the stack, nil when the stack
	// expired or the remover is unknown.
	RemovedBy *Agent
	// IFF is the friend or foe relation of the applier to the receiver.
	IFF evtc.IFF

	// openIdx is the position of the stack in the open list of its
	// receiver and buff while it is present, used by the builder.
	openIdx int
}

// Open reports whether the stack was still present at the end of the log.
func (s *BuffStack) Open() bool { return s.Remove == nil && !s.Superseded && !s.EndedByDespawn }

// Expired reports whether the stack has a removal that is manual or names
// no remover.
func (s *BuffStack) Expired() bool {
	return s.Remove != nil && (s.Removal == evtc.BuffRemoveManual || s.RemovedBy == nil)
}

// IsActiveAt reports whether the stack was in effect at t.
func (s *BuffStack) IsActiveAt(t time.Duration) bool {
	v, ok := s.Active.ValueAt(t)
	return ok && v
}

// Duration returns how long the stack was present within the log.
func (s *BuffStack) Duration() time.Duration { return s.Interval.Duration() }

// Stacks is a query over buff stacks sorted by application time.
type Stacks struct{ Query[BuffStack] }

// Where keeps the stacks accepted by p.
func (q Stacks) Where(p func(*BuffStack) bool) Stacks { return Stacks{q.Query.Where(p)} }

// Between keeps the stacks present at some point within iv.
func (q Stacks) Between(iv Interval) Stacks {
	return Stacks{overlapping(q.Query, stackStart, stackEnd, iv)}
}

// At keeps the stacks present at t.
func (q Stacks) At(t time.Duration) Stacks { return q.Between(At(t)) }

// On keeps the stacks carried by e.
func (q Stacks) On(e Entity) Stacks {
	w := agentsOf(e)
	if w.none() {
		return q.Where(never[BuffStack])
	}
	return q.Where(func(s *BuffStack) bool { return w.is(s.Receiver) })
}

// By keeps the stacks applied by e.
func (q Stacks) By(e Entity) Stacks {
	w := agentsOf(e)
	if w.none() {
		return q.Where(never[BuffStack])
	}
	return q.Where(func(s *BuffStack) bool { return w.is(s.Applier) })
}

// OfBuff keeps the stacks of the buff id.
func (q Stacks) OfBuff(id uint32) Stacks {
	return q.Where(func(s *BuffStack) bool { return s.Buff.Skill.ID == id })
}

// Of keeps the stacks of the buff.
func (q Stacks) Of(b *Buff) Stacks {
	return q.Where(func(s *BuffStack) bool { return s.Buff == b })
}

// Open keeps the stacks still present at the end of the log.
func (q Stacks) Open() Stacks { return q.Where((*BuffStack).Open) }

// CountAt returns the number of matching stacks present at t.
func (q Stacks) CountAt(t time.Duration) int { return q.At(t).Count() }

// Uptime returns the total time within iv during which at least one of
// the matching stacks was present.
func (q Stacks) Uptime(iv Interval) time.Duration {
	q = q.Between(iv)
	// The sweep needs the stacks in start order. A reversed traversal
	// yields the same stacks backwards unless Skip or Limit selected them
	// from the other end, in which case they are collected and re-sorted.
	if q.desc {
		if q.windowed() {
			q = Stacks{From(sortedByTime(q.All(), stackStart))}
		} else {
			q.desc = false
		}
	}
	var total time.Duration
	cursor := iv.Start
	for s := range q.Seq() {
		clipped, ok := s.Interval.Intersect(iv)
		if !ok || clipped.End <= cursor {
			continue
		}
		total += clipped.End - max(clipped.Start, cursor)
		cursor = clipped.End
	}
	return total
}

// GroupBy partitions the matching stacks by the key returned by f. Each
// group is in start order.
func (q Stacks) GroupBy[K comparable](f func(*BuffStack) K) map[K]Stacks {
	return groupInto(q.Query, f, func(items []*BuffStack) Stacks { return Stacks{From(items)} })
}

func stackStart(s *BuffStack) time.Duration { return s.Interval.Start }
func stackEnd(s *BuffStack) time.Duration   { return s.Interval.End }

// Skip drops the first n stacks of the traversal.
func (q Stacks) Skip(n int) Stacks {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n stacks.
func (q Stacks) Limit(n int) Stacks {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the stacks from the latest to the earliest.
func (q Stacks) Reverse() Stacks {
	q.Query = q.Query.Reverse()
	return q
}

// Average returns the mean number of matching stacks present over iv: the
// time every stack spent within iv, summed, divided by the length of iv.
// It is 0 when iv is empty.
func (q Stacks) Average(iv Interval) float64 {
	if iv.Duration() <= 0 {
		return 0
	}
	var total time.Duration
	for s := range q.Between(iv).Seq() {
		if clipped, ok := s.Interval.Intersect(iv); ok {
			total += clipped.Duration()
		}
	}
	return float64(total) / float64(iv.Duration())
}

// RemovedBy keeps the stacks whose removal names e: the stacks e cleansed
// or stripped, and those it removed from itself.
func (q Stacks) RemovedBy(e Entity) Stacks {
	w := agentsOf(e)
	if w.none() {
		return q.Where(never[BuffStack])
	}
	return q.Where(func(s *BuffStack) bool { return w.is(s.RemovedBy) })
}

// EffectiveAt returns the number of stacks in effect at t as the game
// counts them: for every buff present, its stacks through
// Buff.EffectiveStacks. CountAt counts every application present, queued
// ones included.
func (q Stacks) EffectiveAt(t time.Duration) int {
	var buffs []*Buff
	var counts []int
	for s := range q.At(t).Seq() {
		i := slices.Index(buffs, s.Buff)
		if i < 0 {
			buffs = append(buffs, s.Buff)
			counts = append(counts, 0)
			i = len(buffs) - 1
		}
		counts[i]++
	}
	total := 0
	for i, b := range buffs {
		total += b.EffectiveStacks(counts[i])
	}
	return total
}

// EffectiveAverage returns the mean number of stacks in effect over iv,
// with the rule of EffectiveAt. It is 0 when iv is empty.
func (q Stacks) EffectiveAverage(iv Interval) float64 {
	if iv.Duration() <= 0 {
		return 0
	}
	type edge struct {
		at    time.Duration
		buff  *Buff
		delta int
	}
	var edges []edge
	for s := range q.Between(iv).Seq() {
		if c, ok := s.Interval.Intersect(iv); ok && c.Duration() > 0 {
			edges = append(edges, edge{c.Start, s.Buff, 1}, edge{c.End, s.Buff, -1})
		}
	}
	slices.SortFunc(edges, func(a, b edge) int { return cmp.Compare(a.at, b.at) })
	counts := map[*Buff]int{}
	var total float64
	current, prev := 0, iv.Start
	for _, e := range edges {
		total += float64(current) * float64(e.at-prev)
		prev = e.at
		n := counts[e.buff]
		current += e.buff.EffectiveStacks(n+e.delta) - e.buff.EffectiveStacks(n)
		counts[e.buff] = n + e.delta
	}
	return total / float64(iv.Duration())
}

// BuffShare is the share of one buff in a set of stacks.
type BuffShare struct {
	// Buff is the buff of the share.
	Buff *Buff
	// Stacks are the stacks of the buff, in time order.
	Stacks Stacks
}

// StackShare is the share of one agent in a set of stacks.
type StackShare struct {
	// Agent is the agent of the share.
	Agent *Agent
	// Stacks are the stacks of the agent, in time order.
	Stacks Stacks
}

// PerBuff partitions the matching stacks by buff and returns the shares
// by decreasing number of stacks; buffs with as many stacks keep the order
// in which the traversal met them. The stacks of a share are in start
// order. It is nil when there are no stacks.
func (q Stacks) PerBuff() []BuffShare {
	groups := ranked(q.Query, func(s *BuffStack) *Buff { return s.Buff }, one[BuffStack])
	if len(groups) == 0 {
		return nil
	}
	out := make([]BuffShare, len(groups))
	for i, g := range groups {
		out[i] = BuffShare{Buff: g.key, Stacks: Stacks{From(g.items)}}
	}
	return out
}

// PerReceiver partitions the matching stacks by the agent carrying them,
// with the ordering rules of PerBuff.
func (q Stacks) PerReceiver() []StackShare {
	return stackShares(q.Query, func(s *BuffStack) *Agent { return s.Receiver })
}

// PerApplier partitions the matching stacks by the agent that applied
// them, with the ordering rules of PerBuff.
func (q Stacks) PerApplier() []StackShare {
	return stackShares(q.Query, func(s *BuffStack) *Agent { return s.Applier })
}

// stackShares ranks the stacks of q by the agent returned by key.
func stackShares(q Query[BuffStack], key func(*BuffStack) *Agent) []StackShare {
	groups := ranked(q, key, one[BuffStack])
	if len(groups) == 0 {
		return nil
	}
	out := make([]StackShare, len(groups))
	for i, g := range groups {
		out[i] = StackShare{Agent: g.key, Stacks: Stacks{From(g.items)}}
	}
	return out
}

// ActiveAt keeps the stacks in effect at t: present and active.
func (q Stacks) ActiveAt(t time.Duration) Stacks {
	return q.At(t).Where(func(s *BuffStack) bool { return s.IsActiveAt(t) })
}
