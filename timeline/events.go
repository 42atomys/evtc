package timeline

import (
	"math"
	"slices"
	"time"

	"github.com/42atomys/evtc"
)

// Events is a query over raw log events sorted by time.
type Events struct {
	Query[evtc.Event]
	tl *Timeline
}

// Where keeps the events accepted by p.
func (q Events) Where(p func(*evtc.Event) bool) Events {
	q.Query = q.Query.Where(p)
	return q
}

// Of keeps the events of the given state change.
func (q Events) Of(kind evtc.StateChange) Events {
	return q.Where(func(e *evtc.Event) bool { return e.IsStateChange == kind })
}

// Between keeps the events that happened within iv.
func (q Events) Between(iv Interval) Events {
	q.items = narrow(q.items, q.tl.TimeOf, iv)
	return q
}

// addresses returns every address that resolves to the agent behind e,
// so that predicates compare integers instead of looking agents up, or nil
// for a nil entity.
func (q Events) addresses(e Entity) []uint64 {
	a := ref(e)
	if a == nil {
		return nil
	}
	if a == q.tl.Unknown {
		return []uint64{0}
	}
	addrs := []uint64{a.Addr}
	for old := range q.tl.alias {
		if old != a.Addr && q.tl.Agent(old) == a {
			addrs = append(addrs, old)
		}
	}
	return addrs
}

// Involving keeps the events where e is the source or the destination,
// following address changes.
func (q Events) Involving(e Entity) Events {
	addrs := q.addresses(e)
	if addrs == nil {
		return q.Where(never[evtc.Event])
	}
	return q.Where(func(ev *evtc.Event) bool {
		k := ev.IsStateChange
		return (srcIsAgent(k) && slices.Contains(addrs, ev.SrcAgent)) || (dstIsAgent(k) && slices.Contains(addrs, ev.DstAgent))
	})
}

// By keeps the events whose source is e, following address changes.
func (q Events) By(e Entity) Events {
	addrs := q.addresses(e)
	if addrs == nil {
		return q.Where(never[evtc.Event])
	}
	return q.Where(func(ev *evtc.Event) bool {
		return srcIsAgent(ev.IsStateChange) && slices.Contains(addrs, ev.SrcAgent)
	})
}

// On keeps the events whose destination is e, following address changes.
func (q Events) On(e Entity) Events {
	addrs := q.addresses(e)
	if addrs == nil {
		return q.Where(never[evtc.Event])
	}
	return q.Where(func(ev *evtc.Event) bool {
		return dstIsAgent(ev.IsStateChange) && slices.Contains(addrs, ev.DstAgent)
	})
}

// hasTime reports whether the Time field of an event of this kind is a
// timestamp rather than payload.
func hasTime(k evtc.StateChange) bool {
	switch k {
	case evtc.StateBuffInfo, evtc.StateBuffFormula, evtc.StateSkillInfo, evtc.StateSkillTiming,
		evtc.StateIntegrity, evtc.StateIDToGUID, evtc.StateArcBuild:
		return false
	}
	return k < evtc.StateUnknown
}

// srcIsAgent reports whether SrcAgent holds an agent address for this
// kind of event.
func srcIsAgent(k evtc.StateChange) bool {
	switch k {
	case evtc.StateSquadCombatStart, evtc.StateSquadCombatEnd, evtc.StateLanguage, evtc.StateGWBuild,
		evtc.StateShardID, evtc.StateMapID, evtc.StateReplInfo, evtc.StateBuffInfo, evtc.StateBuffFormula,
		evtc.StateSkillInfo, evtc.StateSkillTiming, evtc.StateIntegrity, evtc.StateExtension,
		evtc.StateAPIDelayedDefunc, evtc.StateInstanceStart, evtc.StateRateHealth, evtc.StateIDToGUID,
		evtc.StateLogNPCUpdate, evtc.StateIdleEvent, evtc.StateFractalScale, evtc.StateRuleset,
		evtc.StateSquadMarkerGround, evtc.StateArcBuild, evtc.StateIIDChange, evtc.StateMapChange,
		evtc.StateEarlyExit, evtc.StateWvWTeams, evtc.StateWvWObjectiveStatus, evtc.StateTick,
		evtc.StateBuffChange, evtc.StateEffectGroundRemove, evtc.StateMissileEffect,
		evtc.StateReward:
		return false
	}
	return k < evtc.StateUnknown
}

// dstIsAgent reports whether DstAgent holds an agent address for this kind
// of event.
func dstIsAgent(k evtc.StateChange) bool {
	switch k {
	case evtc.StateCombat, evtc.StateExtensionCombat, evtc.StateBuffInitial, evtc.StateBuffApply,
		evtc.StateBuffChange, evtc.StateBuffRemoveSingle, evtc.StateBuffRemoveAll,
		evtc.StateAnimationStart, evtc.StateAttackTarget, evtc.StateLogNPCUpdate,
		evtc.StateMissileLaunch, evtc.StateMissileEffect:
		return true
	}
	return false
}

// vec3 decodes the float[3] packed in DstAgent and Value.
func vec3(e *evtc.Event) Vec3 {
	return Vec3{
		X: math.Float32frombits(uint32(e.DstAgent)),
		Y: math.Float32frombits(uint32(e.DstAgent >> 32)),
		Z: math.Float32frombits(uint32(e.Value)),
	}
}

// vec2 decodes the float[2] packed in DstAgent.
func vec2(e *evtc.Event) Vec2 {
	return Vec2{
		X: math.Float32frombits(uint32(e.DstAgent)),
		Y: math.Float32frombits(uint32(e.DstAgent >> 32)),
	}
}

// percent decodes a DstAgent holding a percentage times 100.
func percent(e *evtc.Event) float64 { return float64(e.DstAgent) / 100 }

// defiancePercent decodes the float fraction carried by Value into a
// percentage.
func defiancePercent(e *evtc.Event) float64 {
	return float64(math.Float32frombits(uint32(e.Value))) * 100
}

// defianceState decodes the state of a defiance bar. The arcdps README
// documents it in DstAgent; logs of build 20260816 carry it in Value with
// DstAgent left at zero, so DstAgent wins whenever it is set.
func defianceState(e *evtc.Event) DefianceState {
	if e.DstAgent != 0 {
		return DefianceState(e.DstAgent)
	}
	return DefianceState(e.Value)
}

// trackableID decodes the uint32 carried by Pad61 to Pad64.
func trackableID(e *evtc.Event) uint32 {
	return uint32(e.Pad61) | uint32(e.Pad62)<<8 | uint32(e.Pad63)<<16 | uint32(e.Pad64)<<24
}

// ms converts a millisecond count carried by an event field.
func ms(v int64) time.Duration { return time.Duration(v) * time.Millisecond }

// Skip drops the first n events of the traversal.
func (q Events) Skip(n int) Events {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n events.
func (q Events) Limit(n int) Events {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the events from the latest to the earliest.
func (q Events) Reverse() Events {
	q.Query = q.Query.Reverse()
	return q
}

// GroupBy partitions the matching events by the key returned by f. Each
// group is in time order.
func (q Events) GroupBy[K comparable](f func(*evtc.Event) K) map[K]Events {
	return groupInto(q.Query, f, func(items []*evtc.Event) Events { return Events{Query: From(items), tl: q.tl} })
}
