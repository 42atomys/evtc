package evtc

import (
	"strconv"
	"strings"
	"sync"
	"weak"
)

// Capability is something a log can carry. arcdps grew the format release
// after release, so what a log holds depends on the build that wrote it:
// Log.Has tells an empty result ("nobody used stealth") from a log that
// cannot say.
type Capability uint8

// The capabilities, ordered by the first build that writes them.
const (
	// CapabilityMapID when the map of the log is written (StateMapID).
	CapabilityMapID Capability = iota
	// CapabilityStackIdentity when buff applications and single removals
	// name their stack with a trackable id in Pad61 to Pad64.
	CapabilityStackIdentity
	// CapabilityGuild when players carry their guild (StateGuild).
	CapabilityGuild
	// CapabilityCastControl when cast events give the time at which
	// control returns to the agent, in BuffDamage.
	CapabilityCastControl
	// CapabilityDefinitions when buffs and skills are described by
	// StateBuffInfo, StateBuffFormula, StateSkillInfo and StateSkillTiming.
	CapabilityDefinitions
	// CapabilityDefiance when defiance bars are tracked
	// (StateDefianceBarState, StateDefianceBarPercent).
	CapabilityDefiance
	// CapabilityIntegrity when arcdps writes its own diagnostics in the log
	// (StateIntegrity).
	CapabilityIntegrity
	// CapabilityMarkers when commander tags are written (StateMarker).
	CapabilityMarkers
	// CapabilityDefianceDamage when defiance damage is logged as a strike
	// of result ResultDefianceDamageNormal.
	CapabilityDefianceDamage
	// CapabilityBarrierPercent when the barrier of agents is tracked
	// (StateBarrierPctUpdate).
	CapabilityBarrierPercent
	// CapabilityExtensions when arcdps extensions can write their own
	// events (StateExtension).
	CapabilityExtensions
	// CapabilityTickCycle when a buff tick tells how it was triggered:
	// IsOffcycle of enum cbtbuffcycle before CapabilityTypedEvents, Result
	// since.
	CapabilityTickCycle
	// CapabilityCastSignals when skill uses are signalled by a strike of
	// result ResultSkillCast.
	CapabilityCastSignals
	// CapabilityTickDowned when a buff tick tells whether its target was
	// downed: Pad61 before CapabilityTypedEvents, IsOffcycle since.
	CapabilityTickDowned
	// CapabilityInstanceStart when the age of the map instance is written
	// (StateInstanceStart).
	CapabilityInstanceStart
	// CapabilityEffects when visual effects are logged, by the retired
	// StateEffect1Defunc and StateEffect2Defunc or by the four kinds from
	// StateEffectGroundCreate to StateEffectAgentRemove.
	CapabilityEffects
	// CapabilityGUIDs when the log maps its volatile content ids to GUIDs
	// (StateIDToGUID). The events written before this build are defective
	// and later logs may hold none: Has wants both this build and a
	// StateIDToGUID event.
	CapabilityGUIDs
	// CapabilityTargetUpdates when a log started without a boss tells the
	// boss it meets (StateLogNPCUpdate).
	CapabilityTargetUpdates
	// CapabilityFlankAngle when IsFlanking of a strike is an angle from 1
	// to 135 degrees instead of a flag.
	CapabilityFlankAngle
	// CapabilityExtensionEvents when extensions can write combat events
	// whose skill joins the skill table (StateExtensionCombat).
	CapabilityExtensionEvents
	// CapabilityFractalScale when the fractal scale is written
	// (StateFractalScale).
	CapabilityFractalScale
	// CapabilityDurationChangeValues when the values of buff duration
	// changes can be trusted. Event.Value possibly from 20231107 only.
	CapabilityDurationChangeValues
	// CapabilityOriginalDuration when StateBuffInitial gives the original
	// duration of the stack in BuffDamage.
	CapabilityOriginalDuration
	// CapabilityRuleset when the ruleset of the recording player is
	// written (StateRuleset).
	CapabilityRuleset
	// CapabilityMarkerChanges when markers are written as they are added
	// and removed, not only at the start.
	CapabilityMarkerChanges
	// CapabilityGroundMarkers when squad ground markers are written
	// (StateSquadMarkerGround).
	CapabilityGroundMarkers
	// CapabilityAllAgentStates when the kinds arcdps limits to its agent
	// table are written for every agent of that table. Positions were kept
	// for players, the boss and its listed adds before.
	CapabilityAllAgentStates
	// CapabilityPreviousTeam when StateTeamChange gives the old team in
	// Value.
	CapabilityPreviousTeam
	// CapabilitySpecChanges when StateEnterCombat gives the profession
	// and elite specialization of the moment, in Value and BuffDamage.
	CapabilitySpecChanges
	// CapabilityArcBuild when the full arcdps version string is written
	// (StateArcBuild).
	CapabilityArcBuild
	// CapabilityGliding when glider use is written (StateGlider).
	CapabilityGliding
	// CapabilityPreviousWeaponSet when StateWeaponSwap gives the old set
	// in Value.
	CapabilityPreviousWeaponSet
	// CapabilityCrowdControl when crowd control is logged as a strike of
	// result ResultCrowdControl whose Value is a duration.
	CapabilityCrowdControl
	// CapabilityStunBreaks when stun breaks are written (StateStunBreak).
	CapabilityStunBreaks
	// CapabilityTargetMoving when bit 1 of IsMoving tells that the target
	// was moving.
	CapabilityTargetMoving
	// CapabilityAgentEffectEnds when an effect played on an agent has a
	// trackable id and an end event.
	CapabilityAgentEffectEnds
	// CapabilityMissiles when missiles are logged (StateMissileCreate,
	// StateMissileLaunch, StateMissileRemove).
	CapabilityMissiles
	// CapabilityAddressChanges when a change of the address of a player
	// is written (StateIIDChange).
	CapabilityAddressChanges
	// CapabilityMapChanges when map changes are written (StateMapChange).
	CapabilityMapChanges
	// CapabilityMapExit when StateSquadCombatEnd tells that the log ended
	// because the recording player left the map.
	CapabilityMapExit
	// CapabilityTypedEvents when every event is typed by IsStateChange:
	// casts are StateAnimationStart and StateAnimationStop, buff events
	// StateBuffApply to StateBuffRemoveAll, and a cast start names its
	// target agent. Older logs write them all as StateCombat and tell them
	// apart by IsActivation, IsBuffRemove and Buff; package timeline reads
	// both into the same graph.
	CapabilityTypedEvents
	// CapabilityTransformation when transformations are written
	// (StateTransformation).
	CapabilityTransformation
	// CapabilityWvW when WvW teams and objectives are written
	// (StateWvWTeams, StateWvWObjectiveStatus).
	CapabilityWvW
	// CapabilityStealth when stealth is written (StateStealthChange).
	CapabilityStealth
	// CapabilityGadgetAnimations when gadget animations are written
	// (StateGadgetAnimation).
	CapabilityGadgetAnimations
	// CapabilityGadgetNames when the visibility of gadget names is written
	// (StateGadgetName).
	CapabilityGadgetNames
	// CapabilityMissileEffects when effects applied to missiles are
	// written (StateMissileEffect).
	CapabilityMissileEffects
	// CapabilityCapturePoints when capture points are written, from
	// StateGadgetCaptureOutlineShow to StateGadgetCaptureOutlinePoint.
	CapabilityCapturePoints
	// CapabilityTicks when the server tick is written (StateTick).
	CapabilityTicks
	// CapabilityTeleports when teleports are written (StateTeleport).
	CapabilityTeleports
	// CapabilityJumps when jumps are written (StateJump).
	CapabilityJumps
	// CapabilityPing when StateTick carries the ping in Value.
	CapabilityPing
	// CapabilityGadgetModels when gadget models are written
	// (StateGadgetModelInfo).
	CapabilityGadgetModels

	capabilityCount
)

// capabilityInfo describes one capability: the first arcdps build that
// writes it and the kinds whose presence proves it whatever the build.
// The capabilities proved by a field value are in observe.
type capabilityInfo struct {
	name  string
	build int
	kinds []StateChange
	// both is set when neither test is enough alone.
	both bool
}

var capabilityTable = [capabilityCount]capabilityInfo{
	CapabilityMapID:                {name: "MapID", build: 20181008, kinds: []StateChange{StateMapID}},
	CapabilityStackIdentity:        {name: "StackIdentity", build: 20181214},
	CapabilityGuild:                {name: "Guild", build: 20190329, kinds: []StateChange{StateGuild}},
	CapabilityCastControl:          {name: "CastControl", build: 20191107},
	CapabilityDefinitions:          {name: "Definitions", build: 20191225, kinds: []StateChange{StateBuffInfo, StateBuffFormula, StateSkillInfo, StateSkillTiming}},
	CapabilityDefiance:             {name: "Defiance", build: 20200506, kinds: []StateChange{StateDefianceBarState, StateDefianceBarPercent}},
	CapabilityIntegrity:            {name: "Integrity", build: 20200513, kinds: []StateChange{StateIntegrity}},
	CapabilityMarkers:              {name: "Markers", build: 20200609, kinds: []StateChange{StateMarker}},
	CapabilityDefianceDamage:       {name: "DefianceDamage", build: 20201117},
	CapabilityBarrierPercent:       {name: "BarrierPercent", build: 20201201, kinds: []StateChange{StateBarrierPctUpdate}},
	CapabilityExtensions:           {name: "Extensions", build: 20210429, kinds: []StateChange{StateExtension}},
	CapabilityTickCycle:            {name: "TickCycle", build: 20210511},
	CapabilityCastSignals:          {name: "CastSignals", build: 20210525},
	CapabilityTickDowned:           {name: "TickDowned", build: 20210828},
	CapabilityInstanceStart:        {name: "InstanceStart", build: 20220228, kinds: []StateChange{StateInstanceStart}},
	CapabilityEffects:              {name: "Effects", build: 20220628, kinds: []StateChange{StateEffect1Defunc, StateEffect2Defunc, StateEffectGroundCreate, StateEffectGroundRemove, StateEffectAgentCreate, StateEffectAgentRemove}},
	CapabilityGUIDs:                {name: "GUIDs", build: 20220709, kinds: []StateChange{StateIDToGUID}, both: true},
	CapabilityTargetUpdates:        {name: "TargetUpdates", build: 20221111, kinds: []StateChange{StateLogNPCUpdate}},
	CapabilityFlankAngle:           {name: "FlankAngle", build: 20221213},
	CapabilityExtensionEvents:      {name: "ExtensionEvents", build: 20230301, kinds: []StateChange{StateExtensionCombat}},
	CapabilityFractalScale:         {name: "FractalScale", build: 20230718, kinds: []StateChange{StateFractalScale}},
	CapabilityDurationChangeValues: {name: "DurationChangeValues", build: 20230902},
	CapabilityOriginalDuration:     {name: "OriginalDuration", build: 20231107},
	CapabilityRuleset:              {name: "Ruleset", build: 20240205, kinds: []StateChange{StateRuleset}},
	CapabilityMarkerChanges:        {name: "MarkerChanges", build: 20240328},
	CapabilityGroundMarkers:        {name: "GroundMarkers", build: 20240328, kinds: []StateChange{StateSquadMarkerGround}},
	CapabilityAllAgentStates:       {name: "AllAgentStates", build: 20240612},
	CapabilityPreviousTeam:         {name: "PreviousTeam", build: 20240612},
	CapabilitySpecChanges:          {name: "SpecChanges", build: 20240612},
	CapabilityArcBuild:             {name: "ArcBuild", build: 20240614, kinds: []StateChange{StateArcBuild}},
	CapabilityGliding:              {name: "Gliding", build: 20240627, kinds: []StateChange{StateGlider}},
	CapabilityPreviousWeaponSet:    {name: "PreviousWeaponSet", build: 20240627},
	CapabilityCrowdControl:         {name: "CrowdControl", build: 20240709},
	CapabilityStunBreaks:           {name: "StunBreaks", build: 20240709, kinds: []StateChange{StateStunBreak}},
	CapabilityTargetMoving:         {name: "TargetMoving", build: 20240716},
	CapabilityAgentEffectEnds:      {name: "AgentEffectEnds", build: 20241030, kinds: []StateChange{StateEffectAgentRemove}},
	CapabilityMissiles:             {name: "Missiles", build: 20250525, kinds: []StateChange{StateMissileCreate, StateMissileLaunch, StateMissileRemove}},
	CapabilityAddressChanges:       {name: "AddressChanges", build: 20250819, kinds: []StateChange{StateIIDChange}},
	CapabilityMapChanges:           {name: "MapChanges", build: 20250827, kinds: []StateChange{StateMapChange}},
	CapabilityMapExit:              {name: "MapExit", build: 20250829},
	CapabilityTypedEvents:          {name: "TypedEvents", build: 20260501, kinds: []StateChange{StateAnimationStart, StateAnimationStop, StateBuffApply, StateBuffChange, StateBuffRemoveSingle, StateBuffRemoveAll}},
	CapabilityTransformation:       {name: "Transformation", build: 20260507, kinds: []StateChange{StateTransformation}},
	CapabilityWvW:                  {name: "WvW", build: 20260507, kinds: []StateChange{StateWvWTeams, StateWvWObjectiveStatus}},
	CapabilityStealth:              {name: "Stealth", build: 20260602, kinds: []StateChange{StateStealthChange}},
	CapabilityGadgetAnimations:     {name: "GadgetAnimations", build: 20260602, kinds: []StateChange{StateGadgetAnimation}},
	CapabilityGadgetNames:          {name: "GadgetNames", build: 20260602, kinds: []StateChange{StateGadgetName}},
	CapabilityMissileEffects:       {name: "MissileEffects", build: 20260701, kinds: []StateChange{StateMissileEffect}},
	CapabilityCapturePoints:        {name: "CapturePoints", build: 20260701, kinds: []StateChange{StateGadgetCaptureOutlineShow, StateGadgetCaptureSplitPercent, StateGadgetCaptureOutlineHide, StateGadgetCaptureOutlinePoint}},
	CapabilityTicks:                {name: "Ticks", build: 20260701, kinds: []StateChange{StateTick}},
	CapabilityTeleports:            {name: "Teleports", build: 20260811, kinds: []StateChange{StateTeleport}},
	CapabilityJumps:                {name: "Jumps", build: 20260915, kinds: []StateChange{StateJump}},
	CapabilityPing:                 {name: "Ping", build: 20260915},
	CapabilityGadgetModels:         {name: "GadgetModels", build: 20260915, kinds: []StateChange{StateGadgetModelInfo}},
}

// String returns the name of the capability, without its Capability
// prefix.
func (c Capability) String() string {
	if c < capabilityCount {
		return capabilityTable[c].name
	}
	return "Capability(" + strconv.Itoa(int(c)) + ")"
}

// FirstBuild returns the first arcdps build that writes the capability, as
// yyyymmdd, or 0 for an unknown capability. It is the release that made
// the data usable: StateJump was released on 20260811 and written from
// 20260915 only.
func (c Capability) FirstBuild() int {
	if c < capabilityCount {
		return capabilityTable[c].build
	}
	return 0
}

// capSet is a set of capabilities, one bit each.
type capSet [2]uint64

func (s *capSet) add(c Capability) { s[c>>6] |= 1 << (c & 63) }
func (s *capSet) has(c Capability) bool {
	return c < capabilityCount && s[c>>6]&(1<<(c&63)) != 0
}

// kindSet is the set of event kinds seen in a log.
type kindSet [4]uint64

func (s *kindSet) add(k StateChange)      { s[k>>6] |= 1 << (k & 63) }
func (s *kindSet) has(k StateChange) bool { return s[k>>6]&(1<<(k&63)) != 0 }

// observe scans the events once: the kinds they use, and the capabilities
// a field value proves. A proof reads a field that every revision 1 log
// defines for the kind, or one the kind leaves at 0 until the capability,
// never a payload.
func observe(events []Event) (kinds kindSet, proved capSet) {
	for i := range events {
		e := &events[i]
		kinds.add(e.IsStateChange)
		switch e.IsStateChange {
		case StateCombat:
			// Before CapabilityTypedEvents this kind also holds casts, buff
			// applications and buff removals; the Result of a removal
			// counts stacks.
			if e.IsActivation != ActivationNone || e.IsBuffRemove != BuffRemoveNone {
				continue
			}
			if e.IsMoving&2 != 0 {
				proved.add(CapabilityTargetMoving)
			}
			if e.Buff != 0 {
				continue
			}
			switch e.Result {
			case ResultDefianceDamageNormal:
				proved.add(CapabilityDefianceDamage)
			case ResultSkillCast:
				proved.add(CapabilityCastSignals)
			}
			if e.IsFlanking > 1 {
				proved.add(CapabilityFlankAngle)
			}
		case StateBuffApply, StateBuffInitial:
			if e.Pad61|e.Pad62|e.Pad63|e.Pad64 != 0 {
				proved.add(CapabilityStackIdentity)
			}
			if e.IsStateChange == StateBuffInitial && e.BuffDamage != 0 {
				proved.add(CapabilityOriginalDuration)
			}
		case StateEnterCombat:
			if e.Value != 0 {
				proved.add(CapabilitySpecChanges)
			}
		case StateTeamChange:
			if e.Value != 0 {
				proved.add(CapabilityPreviousTeam)
			}
		case StateWeaponSwap:
			if e.Value != 0 {
				proved.add(CapabilityPreviousWeaponSet)
			}
		case StateMarker:
			// A marker id of 0 removes the markers of the agent.
			if e.Value == 0 {
				proved.add(CapabilityMarkerChanges)
			}
		case StateSquadCombatEnd:
			if e.DstAgent&1 != 0 {
				proved.add(CapabilityMapExit)
			}
		case StateTick:
			if e.Value != 0 {
				proved.add(CapabilityPing)
			}
		}
	}
	return kinds, proved
}

// buildDate returns the build date of the header as a number, 0 when it
// is not a date of eight digits from 2016 on.
func (l *Log) buildDate() int {
	s := strings.TrimSpace(l.Header.Build)
	if len(s) != 8 {
		return 0
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	if year, month, day := n/10000, n/100%100, n%100; year < 2016 || month < 1 || month > 12 || day < 1 || day > 31 {
		return 0
	}
	return n
}

// capabilitiesOf decides every capability of a log from its build date
// and from what its events prove.
func capabilitiesOf(build int, kinds kindSet, proved capSet) capSet {
	var set capSet
	for c := range capabilityCount {
		info := &capabilityTable[c]
		byBuild := build >= info.build
		byProof := proved.has(c)
		for _, k := range info.kinds {
			byProof = byProof || kinds.has(k)
		}
		if info.both && byBuild && byProof || !info.both && (byBuild || byProof) {
			set.add(c)
		}
	}
	return set
}

// observation is what one scan of the events tells: the kinds they use
// and what they prove. Parse gives the log an empty one, which the first
// call that needs it fills. It answers for its owner alone, so that a copy
// of the log does not share it, and points to it weakly, so that a copy
// does not keep the tables of the original either.
type observation struct {
	owner  weak.Pointer[Log]
	once   sync.Once
	kinds  kindSet
	proved capSet
	// caps is the answer for build, the build date at the first call.
	build int
	caps  capSet
}

// observed returns the kinds and the capabilities of the log. A log
// assembled by hand, or copied, has nowhere to keep its scan and pays it
// at each call.
func (l *Log) observed() (kindSet, capSet) {
	o := l.seen
	if o == nil || o.owner.Value() != l {
		kinds, proved := observe(l.Events)
		return kinds, capabilitiesOf(l.buildDate(), kinds, proved)
	}
	o.once.Do(func() {
		o.kinds, o.proved = observe(l.Events)
		o.build = l.buildDate()
		o.caps = capabilitiesOf(o.build, o.kinds, o.proved)
	})
	if build := l.buildDate(); build != o.build {
		return o.kinds, capabilitiesOf(build, o.kinds, o.proved)
	}
	return o.kinds, o.caps
}

// Has reports whether the log can carry c: its build is at least
// c.FirstBuild, or its events prove it, since a build slightly older than
// a release can already write its kinds. CapabilityGUIDs alone wants
// both. When Has reports false, an empty result proves nothing: the log
// cannot say.
//
// A parsed log scans its events at the first call and does not read them
// again. A log assembled by hand, or copied, scans them at each call.
func (l *Log) Has(c Capability) bool {
	if l == nil {
		return false
	}
	_, caps := l.observed()
	return caps.has(c)
}

// Capabilities lists what the log can carry, in the order of the
// constants.
func (l *Log) Capabilities() []Capability { return l.list(true) }

// Missing lists what the log cannot carry, in the order of the constants.
func (l *Log) Missing() []Capability { return l.list(false) }

func (l *Log) list(has bool) []Capability {
	if l == nil {
		return nil
	}
	_, set := l.observed()
	var out []Capability
	for c := range capabilityCount {
		if set.has(c) == has {
			out = append(out, c)
		}
	}
	return out
}
