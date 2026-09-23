package combine

import "github.com/42atomys/evtc"

// class tells how an event kind is combined.
type class uint8

const (
	// classRecording is what a client writes once about its own
	// recording: the player who recorded, the language, the builds, the
	// map. The first recording keeps its own.
	classRecording class = iota
	// classDefinition is a definition without a time: united.
	classDefinition
	// classFight is what every client writes about the fight as a whole:
	// the start and the end of the squad combat, the change of the target
	// of the log. Paired, the earliest start and the latest end kept; what
	// a recording tells alone is kept outside what the first one covers.
	classFight
	// classDropped is an address change: addresses are rewritten.
	classDropped
	// classServer is a message every client that holds the agents
	// receives: paired.
	classServer
	// classClient is a sample or a state a client makes on its own: one
	// entry speaks for an agent at a time.
	classClient
	// classBound is a spawn or a despawn: kept at the bounds of what the
	// recordings held together.
	classBound
	// classExtension is an event of an extension: paired on all of its
	// payload, which the package cannot read.
	classExtension
	// classLocal is what a client writes about its own recording over
	// time: its ping, the map it changes to. One recording tells it at a
	// time, the first one that was going on.
	classLocal
)

// ofRecording reports whether the class is what a client writes about its
// own recording, once or over time.
func (c class) ofRecording() bool { return c == classRecording || c == classLocal }

// slot names a state of an agent that events tell. An event that tells
// again the value a slot holds, from another entry than the one that set
// it, is what a client writes when it gains the agent: it is dropped.
type slot uint8

const (
	slotNone slot = iota
	slotTeam
	slotTargetable
	slotStealth
	slotNameVisible
	slotDefiance
	slotDefiancePercent
	slotMaxHealth
	slotHealth
	slotBarrier
	slotWeaponSet
	slotTransformation
	slotGlider
	slotCombat
	slotLife
	slotPosition
	slotVelocity
	slotFacing
	slotGuild
	slotAttackTarget
	slotModel
	// slotMarker is a set: one marker per event, 0 empties the set.
	slotMarker
)

// moving reports whether the slot is a movement sample.
func (s slot) moving() bool { return s == slotPosition || s == slotVelocity || s == slotFacing }

// payload is what names a message beyond its kind, agents and skill.
type payload [3]uint64

// state is the value an event gives a slot.
type state [4]uint64

// rule is the treatment of one kind.
type rule struct {
	class class
	// src and dst tell which fields hold an address.
	src, dst bool
	// keySrc, keyDst and keySkill tell what names the message.
	keySrc, keyDst, keySkill bool
	// key returns the payload that names the message.
	key func(e *evtc.Event) payload
	// slot and value tell the state the event sets.
	slot  slot
	value func(e *evtc.Event) state
}

// trackable decodes the id carried by Pad61 to Pad64.
func trackable(e *evtc.Event) uint64 {
	return uint64(e.Pad61) | uint64(e.Pad62)<<8 | uint64(e.Pad63)<<16 | uint64(e.Pad64)<<24
}

// legacyTrackable decodes the trackable id of an effect of the retired
// StateEffect2Defunc, stored from is_buffremove onwards.
func legacyTrackable(e *evtc.Event) uint64 {
	return uint64(e.IsBuffRemove) | uint64(e.IsNinety)<<8 | uint64(e.IsFifty)<<16 | uint64(e.IsMoving)<<24
}

func u32(v int32) uint64 { return uint64(uint32(v)) }

func byTrackable(e *evtc.Event) payload { return payload{trackable(e)} }
func byDst(e *evtc.Event) payload       { return payload{e.DstAgent} }
func byDstValue(e *evtc.Event) payload  { return payload{e.DstAgent, u32(e.Value)} }
func byValue(e *evtc.Event) payload     { return payload{u32(e.Value)} }
func noPayload(*evtc.Event) payload     { return payload{} }

// whole is everything an event carries beyond its time and its agent: a
// state told again is an event that carries the same. arcdps does not
// always keep a state in the field its README names, the defiance bar
// state for one.
func whole(e *evtc.Event) state {
	return state{
		e.DstAgent,
		u32(e.Value)<<32 | u32(e.BuffDamage),
		uint64(e.OverstackValue)<<32 | uint64(e.SkillID),
		uint64(e.IFF) | uint64(e.Buff)<<8 | uint64(e.Result)<<16 | uint64(e.IsActivation)<<24 | uint64(e.IsBuffRemove)<<32 |
			uint64(e.IsFlanking)<<40 | uint64(e.IsShields)<<48 | uint64(e.IsOffcycle)<<56,
	}
}

func valDst(e *evtc.Event) state   { return state{e.DstAgent} }
func valSkill(e *evtc.Event) state { return state{uint64(e.SkillID)} }
func valValue(e *evtc.Event) state { return state{u32(e.Value)} }

func constant(v uint64) func(*evtc.Event) state {
	return func(*evtc.Event) state { return state{v} }
}

// server is the rule of a message of the server.
func server(keySrc, keyDst, keySkill bool, src, dst bool, key func(*evtc.Event) payload) rule {
	return rule{class: classServer, src: src, dst: dst, keySrc: keySrc, keyDst: keyDst, keySkill: keySkill, key: key}
}

// client is the rule of a state the client of a recording makes.
func client(s slot, value func(*evtc.Event) state) rule {
	return rule{class: classClient, src: true, slot: s, value: value}
}

// with returns the rule with the state its events set.
func (r rule) with(s slot, value func(*evtc.Event) state) rule {
	r.slot, r.value = s, value
	return r
}
