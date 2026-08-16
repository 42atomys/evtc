package evtc

// StateChange is the event type carried by Event.IsStateChange. It mirrors
// the cbtstatechange enum of arcdps. The meaning of the other Event fields
// depends on this value; the fields used by each type are listed below.
type StateChange uint8

const (
	// StateCombat is a combat event.
	// src_agent: source, dst_agent: target, value: strike damage,
	// buff_dmg: buff damage, overstack_value: shield damage, skillid: skill,
	// iff, buff, result, is_ninety, is_fifty, is_moving, is_flanking,
	// is_shields, is_offcycle: see the matching enums.
	StateCombat StateChange = iota
	// StateEnterCombat when agent entered combat.
	// src_agent: agent, dst_agent: subgroup, value: prof id,
	// buff_dmg: elite spec id.
	StateEnterCombat
	// StateExitCombat when agent left combat.
	// src_agent: agent, dst_agent: subgroup, value: prof id,
	// buff_dmg: elite spec id.
	StateExitCombat
	// StateChangeUp when agent is alive at time of event.
	// src_agent: agent.
	StateChangeUp
	// StateChangeDead when agent is dead at time of event.
	// src_agent: agent.
	StateChangeDead
	// StateChangeDown when agent is down at time of event.
	// src_agent: agent.
	StateChangeDown
	// StateSpawn when agent entered tracking.
	// src_agent: agent.
	StateSpawn
	// StateDespawn when agent left tracking.
	// src_agent: agent.
	StateDespawn
	// StateHealthPctUpdate when agent health percentage changed.
	// src_agent: agent, dst_agent: percent * 10000 (99.5% is 9950).
	StateHealthPctUpdate
	// StateSquadCombatStart when the first player enters combat.
	// arcdps formerly named it log start.
	// value: server unix timestamp (uint32), buff_dmg: local unix timestamp.
	StateSquadCombatStart
	// StateSquadCombatEnd when the last player leaves combat.
	// arcdps formerly named it log end.
	// dst_agent: bit 0 = log ended by pov map exit,
	// value: server unix timestamp (uint32), buff_dmg: local unix timestamp.
	StateSquadCombatEnd
	// StateWeaponSwap when agent weapon set changed.
	// src_agent: agent, dst_agent: new weapon set id, value: old weapon set id.
	StateWeaponSwap
	// StateMaxHealthUpdate when agent maximum health changed.
	// src_agent: agent, dst_agent: new max health. Non-players only.
	StateMaxHealthUpdate
	// StatePointOfView identifies the recording player.
	// src_agent: agent.
	StatePointOfView
	// StateLanguage is the text language id.
	// src_agent: language id of enum Language.
	StateLanguage
	// StateGWBuild is the game build.
	// src_agent: game build number.
	StateGWBuild
	// StateShardID is the server shard id.
	// src_agent: shard id.
	StateShardID
	// StateReward is a wiggly box reward.
	// dst_agent: reward id, value: reward type.
	StateReward
	// StateBuffInitial is a buff application for buffs already existing at
	// time of event. Matches StateBuffApply, except buff_dmg: original ms
	// duration of stack.
	StateBuffInitial
	// StatePosition when agent position changed.
	// src_agent: agent, dst_agent: float[3] x/y/z.
	StatePosition
	// StateVelocity when agent velocity changed.
	// src_agent: agent, dst_agent: float[3] x/y/z.
	StateVelocity
	// StateFacing when agent facing direction changed.
	// src_agent: agent, dst_agent: float[2] x/y.
	StateFacing
	// StateTeamChange when agent team id changed.
	// src_agent: agent, dst_agent: new team id, value: old team id.
	StateTeamChange
	// StateAttackTarget is an attacktarget to gadget association.
	// src_agent: the attacktarget, dst_agent: the gadget.
	StateAttackTarget
	// StateTargetable when agent targetable state changed.
	// src_agent: agent, dst_agent: 0 false, 1 true, 2 unsupported.
	StateTargetable
	// StateMapID is the map info.
	// src_agent: map id, dst_agent: map type.
	StateMapID
	// StateReplInfo is for internal use.
	StateReplInfo
	// StateBuffActive when a buff instance is now active.
	// src_agent: agent, dst_agent: trackable id, value: current buff duration.
	StateBuffActive
	// StateBuffDeactive when a buff is set inactive.
	// src_agent: agent, value: new duration, pad61: uint32 trackable id.
	StateBuffDeactive
	// StateGuild when agent is a member of a guild.
	// src_agent: agent, dst_agent: uint8[16] guid of guild.
	StateGuild
	// StateBuffInfo is buff information.
	// overstack_value: max combined duration, skillid: buff skilldef id,
	// src_master_instid: stacking limit, is_flanking: likely an invuln,
	// is_shields: likely an invert, is_offcycle: category,
	// pad61: stacking type, pad62: likely a resistance,
	// pad63: non-zero if used in buff damage simulation.
	StateBuffInfo
	// StateBuffFormula is one buff formula per event.
	// time: float[9] type attribute1 attribute2 parameter1 parameter2
	// parameter3 trait_condition_source trait_condition_self
	// content_reference, skillid: buff skilldef id,
	// src_instid: float[2] buff_condition_source buff_condition_self.
	StateBuffFormula
	// StateSkillInfo is skill information.
	// time: float[4] cost range0 range1 tooltiptime, skillid: skilldef id.
	StateSkillInfo
	// StateSkillTiming is one skill timing per event.
	// src_agent: timing type, dst_agent: ms since activation,
	// skillid: skilldef id.
	StateSkillTiming
	// StateDefianceBarState when agent defiance bar state changed.
	// src_agent: agent, dst_agent: new breakbar state (0 active, 1 recover,
	// 2 immune, 3 none) per the arcdps README. Logs of build 20260816 carry
	// the state in value while dst_agent stays zero.
	StateDefianceBarState
	// StateDefianceBarPercent when agent defiance bar percentage changed.
	// src_agent: agent, value: float new percentage.
	StateDefianceBarPercent
	// StateIntegrity is one message per event. arcdps formerly named it
	// error.
	// time: char[32] null-terminated message.
	StateIntegrity
	// StateMarker is one marker per event on an agent.
	// src_agent: agent, value: markerdef id (0 removes all markers),
	// buff: marker is a commander tag.
	StateMarker
	// StateBarrierPctUpdate when agent barrier percentage changed.
	// src_agent: agent, dst_agent: percent * 10000.
	StateBarrierPctUpdate
	// StateStatResetDefunc is retired, not used since 260402+.
	StateStatResetDefunc
	// StateExtension is for extension use, not managed by arcdps.
	StateExtension
	// StateAPIDelayedDefunc is retired, not used since 260501+.
	StateAPIDelayedDefunc
	// StateInstanceStart is the map instance start.
	// src_agent: ms ago the instance was started, value: uint32 server socket.
	StateInstanceStart
	// StateRateHealth is retired, not used since 260627+.
	StateRateHealth
	// StateLast90BeforeDownDefunc is retired, not used since 240529+.
	StateLast90BeforeDownDefunc
	// StateEffect1Defunc is retired, not used since 230716+.
	StateEffect1Defunc
	// StateIDToGUID is a content id to guid association for volatile types.
	// src_agent: uint8[16] guid of content,
	// overstack_value: of enum ContentLocal.
	StateIDToGUID
	// StateLogNPCUpdate when the log boss agent changed.
	// src_agent: species id, dst_agent: agent,
	// value: uint32 server unix timestamp.
	StateLogNPCUpdate
	// StateIdleEvent is for internal use.
	StateIdleEvent
	// StateExtensionCombat is for extension use, not managed by arcdps.
	// Assumed to be a cbtevent, skillid is processed for buffinfo/skillinfo.
	StateExtensionCombat
	// StateFractalScale is the fractal scale.
	// src_agent: scale.
	StateFractalScale
	// StateEffect2Defunc is retired, not used since 250526+.
	StateEffect2Defunc
	// StateRuleset is the ruleset for self.
	// src_agent: bit0 pve, bit1 wvw, bit2 pvp.
	StateRuleset
	// StateSquadMarkerGround is a squad ground marker.
	// src_agent: float[3] x/y/z (all zero or infinity means removed),
	// skillid: marker index (0 is arrow).
	StateSquadMarkerGround
	// StateArcBuild is the arc build info.
	// src_agent: null-terminated build string.
	StateArcBuild
	// StateGlider when glider status changed.
	// src_agent: agent, value: 1 deployed, 0 stowed.
	StateGlider
	// StateStunBreak when a disable stopped early.
	// src_agent: agent, value: duration remaining.
	StateStunBreak
	// StateMissileCreate when a missile is created.
	// src_agent: agent, value: int16[3] x/y/z divided by 10,
	// overstack_value: skin id (player only), skillid: missile skill id,
	// pad61: uint32 trackable id.
	StateMissileCreate
	// StateMissileLaunch when a missile is launched.
	// src_agent: agent, dst_agent: at agent if set and in range,
	// value: int16[6] target x/y/z, current x/y/z divided by 10,
	// skillid: missile skill id, iff: launch motion,
	// result: int16 motion radius, is_buffremove: uint32 launch flags,
	// is_flanking: non-zero if first launch, is_shields: int16 speed,
	// pad61: uint32 trackable id.
	StateMissileLaunch
	// StateMissileRemove when a missile is removed.
	// src_agent: agent, value: friendly fire damage total,
	// skillid: missile skill id, buff_dmg: int16[3] x/y/z divided by 10,
	// is_flanking: hit at least one enemy, pad61: uint32 trackable id.
	StateMissileRemove
	// StateEffectGroundCreate plays an effect on the ground.
	// src_agent: agent, dst_agent: int16[6] origin x/y/z divided by 10 and
	// orient x/y/z multiplied by 1000, skillid: effect id,
	// iff: uint32 duration, is_buffremove: flags,
	// is_flanking: on a non-static platform,
	// is_shields: int16 scale multiplied by 1000, pad61: uint32 trackable id.
	StateEffectGroundCreate
	// StateEffectGroundRemove stops an effect on the ground.
	// pad61: uint32 trackable id.
	StateEffectGroundRemove
	// StateEffectAgentCreate plays an effect around an agent.
	// src_agent: agent, skillid: effect id, iff: uint32 duration,
	// pad61: uint32 trackable id.
	StateEffectAgentCreate
	// StateEffectAgentRemove stops an effect around an agent.
	// src_agent: agent, pad61: uint32 trackable id.
	StateEffectAgentRemove
	// StateIIDChange when a player iid (Agent.Addr) changed. Happens after
	// spawn when player historical data is loaded.
	// src_agent: old iid, dst_agent: new iid.
	StateIIDChange
	// StateMapChange when the map changed.
	// src_agent: new map id, dst_agent: old map id, value: new map type.
	StateMapChange
	// StateEarlyExit is for internal use.
	StateEarlyExit
	// StateAnimationStart when an animation starts.
	// src_agent: agent, dst_agent: target if applicable,
	// value: ms until minimum of last trigger point and tooltip time,
	// buff_dmg: ms until control is returned, overstack_value: reference
	// data, skillid: skill id.
	StateAnimationStart
	// StateAnimationStop when an animation stops.
	// src_agent: agent, value: ms spent scaled for speed,
	// buff_dmg: ms spent not scaled, skillid: skill id of the start,
	// is_activation: of enum Activation.
	StateAnimationStop
	// StateBuffApply is a buff stack application.
	// src_agent: applier, dst_agent: receiver, value: ms duration,
	// skillid: buff skill id, iff, is_ninety, is_fifty, is_moving,
	// is_flanking: as combat, is_shields: non-zero if active when applied,
	// pad61: uint32 trackable id.
	StateBuffApply
	// StateBuffChange is a buff stack duration change, active only.
	// dst_agent: agent, value: duration difference,
	// overstack_value: new ms duration, skillid: buff skill id,
	// pad61: uint32 trackable id.
	StateBuffChange
	// StateBuffRemoveSingle when a buff stack is removed.
	// src_agent: agent with buff removed, dst_agent: agent removing it,
	// value: ms duration removed, skillid: buff skill id,
	// is_buffremove: of enum BuffRemove, pad61: uint32 trackable id.
	StateBuffRemoveSingle
	// StateBuffRemoveAll when all buff stacks of skillid are removed.
	// src_agent: agent with buffs removed, dst_agent: agent removing them,
	// value: ms duration removed as duration, buff_dmg: as intensity,
	// skillid: buff skill id, is_buffremove: of enum BuffRemove.
	StateBuffRemoveAll
	// StateTransformation when agent transformation changed.
	// src_agent: agent, skillid: transformation id (0 if untransformed),
	// value: duration.
	StateTransformation
	// StateWvWTeams is the wvw team association.
	// src_agent: uint32[6] redshard, blueshard, greenshard, redteam,
	// blueteam, greenteam.
	StateWvWTeams
	// StateWvWObjectiveStatus is a status update on a wvw objective.
	// value: map id, buff_dmg: team id, skillid: objective id,
	// buff: objective type, pad61: uint32 upgrade progress count.
	StateWvWObjectiveStatus
	// StateStealthChange when agent stealth state changed.
	// src_agent: agent, dst_agent: 0 false, 1 true, 2 unsupported.
	StateStealthChange
	// StateGadgetAnimation plays a model animation.
	// src_agent: agent, dst_agent: token.
	StateGadgetAnimation
	// StateGadgetName when gadget name visibility changed.
	// src_agent: agent, dst_agent: 0 false, 1 true, 2 unsupported.
	StateGadgetName
	// StateMissileEffect applies an effect to a missile.
	// dst_agent: owner of missile, skillid: effect id,
	// value: uint32 duration, pad61: uint32 trackable id.
	StateMissileEffect
	// StateGadgetCaptureOutlineShow shows a capture point outline.
	// src_agent: agent, buff: wrbg colour.
	StateGadgetCaptureOutlineShow
	// StateGadgetCaptureSplitPercent is a capture point percent split.
	// src_agent: agent, value: float percent (1.0 - 0.0),
	// buff: wrbg capping from, result: wrbg capping by.
	StateGadgetCaptureSplitPercent
	// StateGadgetCaptureOutlineHide hides a capture point outline.
	// src_agent: agent.
	StateGadgetCaptureOutlineHide
	// StateGadgetCaptureOutlinePoint is capture point point data.
	// src_agent: agent, dst_agent: point index, value: float x,
	// buff_dmg: float y, overstack_value: point count (1 means a circle of
	// radius x around the agent).
	StateGadgetCaptureOutlinePoint
	// StateTick is emitted every 25 ticks.
	// src_agent: current extrapolated tick, dst_agent: ticks since last
	// real update, value: ping.
	StateTick
	// StateTeleport when agent position changed by teleport.
	// src_agent: agent, dst_agent: float[3] x/y/z of target.
	StateTeleport
	// StateJump when agent jumps.
	// src_agent: agent, dst_agent: 1 if leaving platform, 0 on landing.
	StateJump
	// StateUnknown is any type newer than this list.
	StateUnknown
)

var stateChangeNames = []string{
	StateCombat:                    "Combat",
	StateEnterCombat:               "EnterCombat",
	StateExitCombat:                "ExitCombat",
	StateChangeUp:                  "ChangeUp",
	StateChangeDead:                "ChangeDead",
	StateChangeDown:                "ChangeDown",
	StateSpawn:                     "Spawn",
	StateDespawn:                   "Despawn",
	StateHealthPctUpdate:           "HealthPctUpdate",
	StateSquadCombatStart:          "SquadCombatStart",
	StateSquadCombatEnd:            "SquadCombatEnd",
	StateWeaponSwap:                "WeaponSwap",
	StateMaxHealthUpdate:           "MaxHealthUpdate",
	StatePointOfView:               "PointOfView",
	StateLanguage:                  "Language",
	StateGWBuild:                   "GWBuild",
	StateShardID:                   "ShardID",
	StateReward:                    "Reward",
	StateBuffInitial:               "BuffInitial",
	StatePosition:                  "Position",
	StateVelocity:                  "Velocity",
	StateFacing:                    "Facing",
	StateTeamChange:                "TeamChange",
	StateAttackTarget:              "AttackTarget",
	StateTargetable:                "Targetable",
	StateMapID:                     "MapID",
	StateReplInfo:                  "ReplInfo",
	StateBuffActive:                "BuffActive",
	StateBuffDeactive:              "BuffDeactive",
	StateGuild:                     "Guild",
	StateBuffInfo:                  "BuffInfo",
	StateBuffFormula:               "BuffFormula",
	StateSkillInfo:                 "SkillInfo",
	StateSkillTiming:               "SkillTiming",
	StateDefianceBarState:          "DefianceBarState",
	StateDefianceBarPercent:        "DefianceBarPercent",
	StateIntegrity:                 "Integrity",
	StateMarker:                    "Marker",
	StateBarrierPctUpdate:          "BarrierPctUpdate",
	StateStatResetDefunc:           "StatResetDefunc",
	StateExtension:                 "Extension",
	StateAPIDelayedDefunc:          "APIDelayedDefunc",
	StateInstanceStart:             "InstanceStart",
	StateRateHealth:                "RateHealth",
	StateLast90BeforeDownDefunc:    "Last90BeforeDownDefunc",
	StateEffect1Defunc:             "Effect1Defunc",
	StateIDToGUID:                  "IDToGUID",
	StateLogNPCUpdate:              "LogNPCUpdate",
	StateIdleEvent:                 "IdleEvent",
	StateExtensionCombat:           "ExtensionCombat",
	StateFractalScale:              "FractalScale",
	StateEffect2Defunc:             "Effect2Defunc",
	StateRuleset:                   "Ruleset",
	StateSquadMarkerGround:         "SquadMarkerGround",
	StateArcBuild:                  "ArcBuild",
	StateGlider:                    "Glider",
	StateStunBreak:                 "StunBreak",
	StateMissileCreate:             "MissileCreate",
	StateMissileLaunch:             "MissileLaunch",
	StateMissileRemove:             "MissileRemove",
	StateEffectGroundCreate:        "EffectGroundCreate",
	StateEffectGroundRemove:        "EffectGroundRemove",
	StateEffectAgentCreate:         "EffectAgentCreate",
	StateEffectAgentRemove:         "EffectAgentRemove",
	StateIIDChange:                 "IIDChange",
	StateMapChange:                 "MapChange",
	StateEarlyExit:                 "EarlyExit",
	StateAnimationStart:            "AnimationStart",
	StateAnimationStop:             "AnimationStop",
	StateBuffApply:                 "BuffApply",
	StateBuffChange:                "BuffChange",
	StateBuffRemoveSingle:          "BuffRemoveSingle",
	StateBuffRemoveAll:             "BuffRemoveAll",
	StateTransformation:            "Transformation",
	StateWvWTeams:                  "WvWTeams",
	StateWvWObjectiveStatus:        "WvWObjectiveStatus",
	StateStealthChange:             "StealthChange",
	StateGadgetAnimation:           "GadgetAnimation",
	StateGadgetName:                "GadgetName",
	StateMissileEffect:             "MissileEffect",
	StateGadgetCaptureOutlineShow:  "GadgetCaptureOutlineShow",
	StateGadgetCaptureSplitPercent: "GadgetCaptureSplitPercent",
	StateGadgetCaptureOutlineHide:  "GadgetCaptureOutlineHide",
	StateGadgetCaptureOutlinePoint: "GadgetCaptureOutlinePoint",
	StateTick:                      "Tick",
	StateTeleport:                  "Teleport",
	StateJump:                      "Jump",
	StateUnknown:                   "Unknown",
}

// String returns the arcdps name of the state change, without its CBTS_
// prefix. Values newer than this package are formatted as StateChange(n).
func (s StateChange) String() string { return enumString(stateChangeNames, "StateChange", int(s)) }
