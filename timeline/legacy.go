package timeline

import (
	"time"

	"github.com/42atomys/evtc"
)

// Before arcdps 20260501 a log has no kind for a cast, a buff application,
// a duration change or a buff removal: they are StateCombat events told
// apart by their other fields, and a buff tick keeps some of its fields
// elsewhere. This file gives such a log the reading of the typed format,
// so that the builder has one path for both. The raw events are left as
// they are: a node built from one still points to a StateCombat event.

// kindSkipped is the kind given to an event the builder must not read.
const kindSkipped = evtc.StateUnknown

// legacyKinds returns, for each timed event of a log written before the
// typed format, the kind that format gives the same fact.
func legacyKinds(events []*evtc.Event) []evtc.StateChange {
	kinds := make([]evtc.StateChange, len(events))
	for i, e := range events {
		kinds[i] = legacyKind(e)
	}
	return kinds
}

// legacyKind types one event with the tests of the arcdps README of that
// format, in its order, after setting aside the empty initial buffs of a
// few builds.
func legacyKind(e *evtc.Event) evtc.StateChange {
	switch {
	case e.IsStateChange == evtc.StateBuffInitial && e.SkillID == 0:
		// arcdps 20250708 to 20250913 opens a log with thousands of
		// initial buffs that name no buff.
		return kindSkipped
	case e.IsStateChange != evtc.StateCombat:
		return e.IsStateChange
	case e.IsActivation == evtc.ActivationStartDefunc || e.IsActivation == evtc.ActivationQuicknessDefunc:
		return evtc.StateAnimationStart
	case e.IsActivation != evtc.ActivationNone:
		return evtc.StateAnimationStop
	case e.IsBuffRemove == evtc.BuffRemoveAll:
		return evtc.StateBuffRemoveAll
	case e.IsBuffRemove != evtc.BuffRemoveNone:
		return evtc.StateBuffRemoveSingle
	case e.Buff != 0 && e.BuffDamage == 0 && e.Value != 0 && e.IsOffcycle != 0:
		return evtc.StateBuffChange
	case e.Buff != 0 && e.BuffDamage == 0 && e.Value != 0:
		return evtc.StateBuffApply
	}
	return evtc.StateCombat
}

// dstAddr returns the address DstAgent holds when it names an agent. A
// cast start written before the typed format keeps something else there,
// and reads as one without a target; an effect of StateEffect2Defunc
// names an agent there or nothing.
func (b *builder) dstAddr(k evtc.StateChange, e *evtc.Event) (addr uint64, ok bool) {
	switch {
	case k == evtc.StateEffect2Defunc:
		return e.DstAgent, legacyEffectOnAgent(e)
	case k == evtc.StateAnimationStart && b.kinds != nil:
		return 0, true
	}
	return e.DstAgent, dstIsAgent(k)
}

// legacyTick reads the result and the downed flag of a buff tick written
// before the typed format. Its result only tells a tick the target was
// invulnerable to, which the typed format writes as absorbed; the cycle
// sits in is_offcycle and the downed flag in pad61.
func legacyTick(e *evtc.Event) (result evtc.Result, targetDowned bool) {
	targetDowned = e.Pad61 != 0
	if e.Result != evtc.ResultStrikeDamageNormal {
		// 1 to 4 are the ways the target was invulnerable.
		if e.Result <= 4 {
			return evtc.ResultAbsorb, targetDowned
		}
		return e.Result, targetDowned
	}
	switch evtc.BuffCycle(e.IsOffcycle) {
	case evtc.BuffCycleCycle:
		return evtc.ResultBuffDamageCycle, targetDowned
	case evtc.BuffCycleNotCycleDmgToTargetOnHit:
		return evtc.ResultBuffDamageNotCycleDmgToTargetOnHit, targetDowned
	case evtc.BuffCycleNotCycleDmgToSourceOnHit:
		return evtc.ResultBuffDamageNotCycleDmgToSourceOnHit, targetDowned
	case evtc.BuffCycleNotCycleDmgToTargetOnStackRemove:
		return evtc.ResultBuffDamageNotCycleDmgToTargetOnStackRemove, targetDowned
	}
	return evtc.ResultBuffDamageNotCycle, targetDowned
}

// An effect of StateEffect2Defunc, the kind arcdps wrote effects with
// until 20250603, plays at an agent when dst_agent names one and on the
// ground otherwise. With neither an agent nor a place it stops the effect
// of its trackable id.

// legacyEffectOnAgent reports whether the effect plays at the agent of
// dst_agent.
func legacyEffectOnAgent(e *evtc.Event) bool { return e.DstAgent != 0 }

// legacyEffectAgent returns the agent the effect belongs to: the one it
// plays at, or the one that placed it on the ground.
func legacyEffectAgent(e *evtc.Event, src, dst *Agent) *Agent {
	if legacyEffectOnAgent(e) {
		return dst
	}
	return src
}

// legacyEffectStops reports whether the event ends an effect.
func legacyEffectStops(e *evtc.Event) bool {
	return e.DstAgent == 0 && e.Value == 0 && e.BuffDamage == 0 && e.OverstackValue == 0
}

// legacyEffectID decodes the trackable id, stored from is_buffremove
// onwards.
func legacyEffectID(e *evtc.Event) uint32 {
	b := e.Bytes()
	return u32At(&b, offBuffRemove)
}

// legacyEffectPlace decodes the origin of a ground effect, three floats
// from value onwards, and its orientation, from is_shields onwards.
func legacyEffectPlace(e *evtc.Event) (origin, orientation Vec3) {
	b := e.Bytes()
	origin = Vec3{f32At(&b, offValue), f32At(&b, offValue+4), f32At(&b, offValue+8)}
	return origin, angles(&b, offShields)
}

// legacyEffect creates the effect of a StateEffect2Defunc event, or ends
// the one it stops.
func (b *builder) legacyEffect(e *evtc.Event, t time.Duration, src, dst *Agent) {
	id := legacyEffectID(e)
	if legacyEffectStops(e) {
		if f := b.openEffects[id]; f != nil {
			b.closeEffect(f, e, t)
		}
		return
	}
	f := &b.effectArena[b.effectIdx]
	b.effectIdx++
	*f = Effect{
		ID:       id,
		EffectID: e.SkillID,
		Agent:    legacyEffectAgent(e, src, dst),
		Ground:   !legacyEffectOnAgent(e),
		Scale:    1,
		Duration: effectDuration(e),
		Interval: Interval{Start: t, End: b.tl.Duration},
		Create:   e,
	}
	if f.Ground {
		f.Origin, f.Orientation = legacyEffectPlace(e)
		f.MovingPlatform = e.IsFlanking != 0
	}
	b.addEffect(f)
}
