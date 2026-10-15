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
	slotInfo
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

// rules holds the treatment of every kind of the typed format. A kind that
// is missing is one a later arcdps added: the first recording keeps its
// own, as for what a client writes about its recording.
var rules = map[evtc.StateChange]rule{
	evtc.StatePointOfView:   {class: classRecording, src: true},
	evtc.StateLanguage:      {class: classRecording},
	evtc.StateGWBuild:       {class: classRecording},
	evtc.StateShardID:       {class: classRecording},
	evtc.StateMapID:         {class: classRecording},
	evtc.StateInstanceStart: {class: classRecording},
	evtc.StateFractalScale:  {class: classRecording},
	evtc.StateRuleset:       {class: classRecording},
	evtc.StateWvWTeams:      {class: classRecording},
	evtc.StateArcBuild:      {class: classRecording},
	evtc.StateIntegrity:     {class: classRecording},

	evtc.StateTick:                   {class: classLocal},
	evtc.StateMapChange:              {class: classLocal},
	evtc.StateRateHealth:             {class: classLocal},
	evtc.StateStatResetDefunc:        {class: classLocal},
	evtc.StateReplInfo:               {class: classLocal},
	evtc.StateIdleEvent:              {class: classLocal},
	evtc.StateEarlyExit:              {class: classLocal},
	evtc.StateAPIDelayedDefunc:       {class: classLocal},
	evtc.StateEffect1Defunc:          {class: classLocal},
	evtc.StateLast90BeforeDownDefunc: {class: classLocal, src: true},

	// The target of the log changes for every client of the squad.
	evtc.StateLogNPCUpdate:     {class: classFight, dst: true},
	evtc.StateSquadCombatStart: {class: classFight},
	evtc.StateSquadCombatEnd:   {class: classFight},
	evtc.StateIIDChange:        {class: classDropped},

	evtc.StateBuffInfo:    {class: classDefinition},
	evtc.StateBuffFormula: {class: classDefinition},
	evtc.StateSkillInfo:   {class: classDefinition},
	evtc.StateSkillTiming: {class: classDefinition},
	evtc.StateIDToGUID:    {class: classDefinition},

	evtc.StateExtension:       {class: classExtension},
	evtc.StateExtensionCombat: {class: classExtension, src: true, dst: true},

	evtc.StateSpawn:   {class: classBound, src: true, keySrc: true, key: noPayload},
	evtc.StateDespawn: {class: classBound, src: true, keySrc: true, key: noPayload},

	// A strike is named by its agents, its skill and what it did;
	// StateCombat with a buff is a tick, see ruleOf.
	evtc.StateCombat: server(true, true, true, true, true, func(e *evtc.Event) payload {
		return payload{u32(e.Value), u32(e.BuffDamage), uint64(e.OverstackValue)<<8 | uint64(e.Result)}
	}),
	evtc.StateEnterCombat: server(true, false, false, true, false, noPayload).with(slotCombat, constant(1)),
	evtc.StateExitCombat:  server(true, false, false, true, false, noPayload).with(slotCombat, constant(0)),
	evtc.StateChangeUp:    server(true, false, false, true, false, noPayload).with(slotLife, constant(1)),
	evtc.StateChangeDown:  server(true, false, false, true, false, noPayload).with(slotLife, constant(2)),
	evtc.StateChangeDead:  server(true, false, false, true, false, noPayload).with(slotLife, constant(3)),
	evtc.StateWeaponSwap:  server(true, false, false, true, false, byDst).with(slotWeaponSet, valDst),
	// A stack is named by its receiver, its buff and its id. An initial
	// stack and an application are the same opening, see gather.
	evtc.StateBuffInitial:      server(false, true, true, true, true, byTrackable),
	evtc.StateBuffApply:        server(false, true, true, true, true, byTrackable),
	evtc.StateBuffRemoveSingle: server(true, false, true, true, true, byTrackable),
	evtc.StateBuffRemoveAll:    server(true, false, true, true, true, noPayload),
	evtc.StateBuffActive:       server(true, false, false, true, false, byDst),
	evtc.StateBuffDeactive:     server(true, false, false, true, false, byTrackable),
	evtc.StateBuffChange: server(false, true, true, false, true, func(e *evtc.Event) payload {
		return payload{trackable(e), u32(e.Value)}
	}),
	evtc.StateAnimationStart: server(true, true, true, true, true, noPayload),
	evtc.StateAnimationStop:  server(true, false, true, true, false, noPayload),
	evtc.StateMissileCreate:  server(true, false, true, true, false, byTrackable),
	evtc.StateMissileLaunch: server(true, false, true, true, true, func(e *evtc.Event) payload {
		// The target is x, y in value and z in the low half of
		// buff_dmg; the rest is where the client saw the missile.
		return payload{trackable(e), u32(e.Value), u32(e.BuffDamage) & 0xFFFF}
	}),
	evtc.StateMissileRemove: server(true, false, true, true, false, byTrackable),
	evtc.StateMissileEffect: server(false, true, true, false, true, byTrackable),
	evtc.StateEffectGroundCreate: server(true, false, true, true, false, func(e *evtc.Event) payload {
		return payload{trackable(e), e.DstAgent}
	}),
	evtc.StateEffectGroundRemove: server(false, false, false, false, false, byTrackable),
	evtc.StateEffectAgentCreate:  server(true, false, true, true, true, byTrackable),
	evtc.StateEffectAgentRemove:  server(true, false, false, true, false, byTrackable),
	evtc.StateEffect2Defunc: server(true, false, true, true, false, func(e *evtc.Event) payload {
		return payload{legacyTrackable(e)}
	}),
	evtc.StateTeleport:        server(true, false, false, true, false, byDstValue),
	evtc.StateJump:            server(true, false, false, true, false, byDst),
	evtc.StateFlyTo:           server(true, false, false, true, false, byDst),
	evtc.StateTransformation:  server(true, false, true, true, false, noPayload).with(slotTransformation, valSkill),
	evtc.StateGlider:          server(true, false, false, true, false, byValue).with(slotGlider, valValue),
	evtc.StateStunBreak:       server(true, false, false, true, false, noPayload),
	evtc.StateGadgetAnimation: server(true, false, false, true, false, byDst),
	evtc.StateGuild:           server(true, false, false, true, false, byDstValue).with(slotGuild, whole),
	evtc.StateAttackTarget:    server(true, true, false, true, true, noPayload),
	evtc.StateSquadMarkerGround: server(false, false, true, false, false, func(e *evtc.Event) payload {
		return payload{e.SrcAgent, e.DstAgent}
	}),
	evtc.StateWvWObjectiveStatus: server(false, false, true, false, false, func(e *evtc.Event) payload {
		return payload{u32(e.Value), u32(e.BuffDamage), uint64(e.Buff)}
	}),
	// A reward is named by its id and its type. The clients that get it
	// are told at once; a client may get one the others do not.
	evtc.StateReward: server(false, false, false, false, false, byDstValue),

	evtc.StateHealthPctUpdate:  client(slotHealth, whole),
	evtc.StateBarrierPctUpdate: client(slotBarrier, whole),
	evtc.StateMaxHealthUpdate:  client(slotMaxHealth, whole),
	// A movement sample anchors the interpolation: one that repeats the
	// last value is a repetition only when it follows it at once.
	evtc.StatePosition:           client(slotPosition, whole),
	evtc.StateVelocity:           client(slotVelocity, whole),
	evtc.StateFacing:             client(slotFacing, whole),
	evtc.StateTeamChange:         client(slotTeam, valDst),
	evtc.StateTargetable:         client(slotTargetable, whole),
	evtc.StateStealthChange:      client(slotStealth, whole),
	evtc.StateGadgetName:         client(slotNameVisible, whole),
	evtc.StateDefianceBarState:   client(slotDefiance, whole),
	evtc.StateDefianceBarPercent: client(slotDefiancePercent, whole),
	// A marker is its id and whether it is a commander tag; id 0 takes
	// every marker off.
	evtc.StateMarker: client(slotMarker, func(e *evtc.Event) state {
		return state{u32(e.Value), uint64(e.Buff)}
	}),
	evtc.StateGadgetModelInfo:           client(slotModel, whole),
	evtc.StateAgentInfo:                 client(slotInfo, whole),
	evtc.StateGadgetCaptureOutlineShow:  client(slotNone, nil),
	evtc.StateGadgetCaptureSplitPercent: client(slotNone, nil),
	evtc.StateGadgetCaptureOutlineHide:  client(slotNone, nil),
	evtc.StateGadgetCaptureOutlinePoint: client(slotNone, nil),
}

// tick is the rule of a buff damage tick: arcdps estimates most of them,
// so they follow the recording that holds the target.
var tick = rule{class: classClient, src: true, dst: true}

// The rules of the kinds the older format writes another way.
var (
	// A cast start keeps something else than its target in dst_agent.
	legacyAnimationStart = server(true, false, true, true, false, noPayload)
	// A duration change names the agent that extended the stack.
	legacyBuffChange = server(false, true, true, true, true, func(e *evtc.Event) payload {
		return payload{trackable(e), u32(e.Value)}
	})
	// An effect plays at the agent of dst_agent when it names one.
	legacyEffectOnAgent = server(true, true, true, true, true, func(e *evtc.Event) payload {
		return payload{legacyTrackable(e)}
	})
)

// leave is the rule of a team change to no team: what a client writes when
// it stops holding an agent, as a despawn is, and for every agent it holds
// when the log ends.
var leave = rule{class: classBound, src: true, keySrc: true, key: noPayload}

// kindOf returns the kind of an event in the typed format. Before arcdps
// 20260501 a cast and a buff event are StateCombat events told apart by
// their other fields, as the timeline reads them.
func kindOf(e *evtc.Event, typed bool) evtc.StateChange {
	if typed || e.IsStateChange != evtc.StateCombat {
		return e.IsStateChange
	}
	switch {
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

// isLeave reports whether the event tells that a client stops holding the
// agent.
func isLeave(e *evtc.Event, k evtc.StateChange) bool {
	return k == evtc.StateDespawn || (k == evtc.StateTeamChange && e.DstAgent == 0)
}

// ruleOf returns the rule of an event of kind k, the one of classRecording
// for a kind the list does not know.
func ruleOf(e *evtc.Event, k evtc.StateChange, typed bool) rule {
	switch {
	case k == evtc.StateTeamChange && e.DstAgent == 0:
		return leave
	case k == evtc.StateCombat && e.Buff != 0:
		return tick
	case k == evtc.StateEffect2Defunc && e.DstAgent != 0:
		return legacyEffectOnAgent
	case !typed && k == evtc.StateAnimationStart:
		return legacyAnimationStart
	case !typed && k == evtc.StateBuffChange:
		return legacyBuffChange
	}
	return rules[k]
}

// timed reports whether the time field of a kind holds a time.
func timed(k evtc.StateChange) bool {
	switch k {
	case evtc.StateBuffInfo, evtc.StateBuffFormula, evtc.StateSkillInfo, evtc.StateSkillTiming,
		evtc.StateIntegrity, evtc.StateIDToGUID, evtc.StateArcBuild:
		return false
	}
	return true
}

// tracks reports whether a kind tells the state of its source agent. The
// timeline gives such an event written without address to the last agent
// seen with its instance id, and to nobody when it saw none.
func tracks(k evtc.StateChange) bool {
	switch k {
	case evtc.StatePosition, evtc.StateTeleport, evtc.StateVelocity, evtc.StateFacing,
		evtc.StateHealthPctUpdate, evtc.StateBarrierPctUpdate, evtc.StateMaxHealthUpdate,
		evtc.StateDefianceBarPercent, evtc.StateDefianceBarState,
		evtc.StateChangeUp, evtc.StateChangeDown, evtc.StateChangeDead, evtc.StateSpawn, evtc.StateDespawn,
		evtc.StateEnterCombat, evtc.StateExitCombat, evtc.StateTargetable,
		evtc.StateWeaponSwap, evtc.StateTeamChange, evtc.StateStealthChange, evtc.StateGlider,
		evtc.StateTransformation, evtc.StateStunBreak, evtc.StateMarker, evtc.StateGuild,
		evtc.StateJump, evtc.StateGadgetAnimation, evtc.StateGadgetName,
		evtc.StateGadgetModelInfo, evtc.StateAgentInfo:
		return true
	}
	return false
}
