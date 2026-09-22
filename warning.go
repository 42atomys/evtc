package evtc

import (
	"cmp"
	"slices"
	"strconv"
)

// WarningCode identifies what a Warning is about, for a program that
// reacts to some of them.
type WarningCode uint8

// The codes, in the order Warnings reports them.
const (
	// WarningBuild when the build date of the header is not a date.
	// Capabilities then rest on what the events prove alone, and
	// CapabilityGUIDs, which wants a build, is never reported.
	WarningBuild WarningCode = iota
	// WarningGenericTarget when the target species id is 1 in a log older
	// than 20221126, the date of the first captured arcdps README that
	// calls 1 a WvW log. It means a generic log before, started by a squad
	// member entering combat: a WvW log or not.
	WarningGenericTarget
	// WarningNoSkills when events name skills and the skill table holds
	// none of them. arcdps 20260701 writes a table of one row, skill 0, no
	// StateBuffInfo, StateBuffFormula or StateSkillTiming event and no
	// skill GUID: its logs name and define no skill but that one.
	WarningNoSkills
	// WarningDurationChangeAgents when the release swaps the source and the
	// destination of buff duration changes.
	WarningDurationChangeAgents
	// WarningCancelTimes when the release writes 0 in the times of cast
	// ends other than resets.
	WarningCancelTimes
	// WarningGUIDs when the log holds StateIDToGUID events of a release
	// whose GUIDs are to be ignored.
	WarningGUIDs
	// WarningTarget when the release is known to leave the target species
	// id wrong or unset.
	WarningTarget
	// WarningLanguage when the release writes a wrong StateLanguage.
	WarningLanguage
	// WarningEffectGUIDs when the release writes no GUID for its effects.
	WarningEffectGUIDs
	// WarningEffectOrientation when the release encodes the orientation of
	// StateEffect2Defunc another way than every later one.
	WarningEffectOrientation
	// WarningMarkerSpam when the release may repeat its StateMarker events.
	WarningMarkerSpam
	// WarningDeadDown when the release swaps StateChangeDead and
	// StateChangeDown.
	WarningDeadDown
	// WarningHitboxWidth when the release leaves the hitbox width of most
	// agents empty.
	WarningHitboxWidth
	// WarningMarkers when the release writes broken StateMarker events.
	WarningMarkers
	// WarningMissileRadius when the release writes a wrong motion radius in
	// StateMissileLaunch.
	WarningMissileRadius

	warningCount
)

var warningNames = [warningCount]string{
	WarningBuild:                "Build",
	WarningGenericTarget:        "GenericTarget",
	WarningNoSkills:             "NoSkills",
	WarningDurationChangeAgents: "DurationChangeAgents",
	WarningCancelTimes:          "CancelTimes",
	WarningGUIDs:                "GUIDs",
	WarningTarget:               "Target",
	WarningLanguage:             "Language",
	WarningEffectGUIDs:          "EffectGUIDs",
	WarningEffectOrientation:    "EffectOrientation",
	WarningMarkerSpam:           "MarkerSpam",
	WarningDeadDown:             "DeadDown",
	WarningHitboxWidth:          "HitboxWidth",
	WarningMarkers:              "Markers",
	WarningMissileRadius:        "MissileRadius",
}

// String returns the name of the code, without its Warning prefix.
func (c WarningCode) String() string { return enumString(warningNames[:], "WarningCode", int(c)) }

// Warning is something the reader of one log should know before trusting
// it: a defect of the arcdps release that wrote it, or a header that
// cannot be read or does not mean what it means today. It is not an
// error, the log decodes.
type Warning struct {
	// Code identifies the warning.
	Code WarningCode
	// Message describes it in one sentence.
	Message string
}

// defect is a known defect of arcdps: the release that introduced it, the
// release that fixed it, and the kinds the log must hold for it to
// matter (none when it concerns every log).
type defect struct {
	code        WarningCode
	from, fixed int
	kinds       []StateChange
	text        string
}

// defects lists defects the arcdps changelog admits and describes, for
// the releases Parse accepts; it is not exhaustive. A header can be dated
// the day after its release. A build dated the day of a fix may therefore
// still have the defect and is not reported, and one dated the first day
// of a defect may belong to a sound release of the day before: 20240329
// is in that case, hence its "may". 20240613 is not: every log of that
// date lacks most of its widths, nine NPCs out of ten over the lot.
var defects = []defect{
	{WarningDurationChangeAgents, 20190108, 20190109, nil, "buff duration changes have their source and destination swapped"},
	{WarningCancelTimes, 20210529, 20210530, nil, "the times of cast ends other than resets are 0"},
	{WarningGUIDs, 20220628, 20220709, []StateChange{StateIDToGUID}, "the GUIDs of this release are defective and to be ignored"},
	{WarningTarget, 20230110, 20230111, nil, "the target species id may be the wrong boss"},
	{WarningLanguage, 20230718, 20230719, []StateChange{StateLanguage}, "the language event is wrong"},
	{WarningEffectGUIDs, 20230718, 20230719, []StateChange{StateEffect2Defunc}, "effects have no GUID"},
	{WarningEffectOrientation, 20230718, 20230719, []StateChange{StateEffect2Defunc}, "effect orientations use an encoding no other release shares"},
	{WarningMarkerSpam, 20240329, 20240330, []StateChange{StateMarker}, "marker events may be repeated"},
	{WarningDeadDown, 20240612, 20240613, []StateChange{StateChangeDead, StateChangeDown}, "dead and down events are swapped"},
	{WarningHitboxWidth, 20240613, 20240614, nil, "most agents have no hitbox width"},
	{WarningMarkers, 20241210, 20241211, []StateChange{StateMarker}, "marker events are broken"},
	{WarningMissileRadius, 20250525, 20250603, []StateChange{StateMissileLaunch}, "the motion radius of missile launches is wrong"},
	{WarningTarget, 20250806, 20250809, nil, "the target species id may not be set"},
}

// genericTargetUntil is the date of the first captured arcdps README that
// gives a target species id of 1 its WvW meaning.
const genericTargetUntil = 20221126

// noSkillsFrom and noSkillsFixed bound the release behind WarningNoSkills.
const noSkillsFrom, noSkillsFixed = 20260701, 20260702

// Warnings lists what is known to be wrong with the log, in the order of
// the codes, or nil. arcdps writes its own messages in the log as
// StateIntegrity events; they are not repeated here. Like Has, a parsed
// log reads the kinds of its events once, at the first call of either; the
// skill table check reads the events at each call, up to the first skill
// the table holds.
func (l *Log) Warnings() []Warning {
	if l == nil {
		return nil
	}
	var out []Warning
	build := l.buildDate()
	if build == 0 {
		out = append(out, Warning{WarningBuild, "the build date " + strconv.Quote(l.Header.Build) + " is not a date"})
	}
	if l.Header.TargetSpeciesID == 1 && build != 0 && build < genericTargetUntil {
		out = append(out, Warning{WarningGenericTarget, "a target species id of 1 means a generic log before arcdps " + strconv.Itoa(genericTargetUntil) + ", a WvW log or not"})
	}
	if l.namesNoSkill() {
		text := "the skill table holds none of the skills the events name"
		if build >= noSkillsFrom && build < noSkillsFixed {
			text += ": arcdps " + strconv.Itoa(build) + " writes no buff definition, skill timing or skill GUID either"
		}
		out = append(out, Warning{WarningNoSkills, text})
	}
	var kinds kindSet
	scanned := false
	for _, d := range defects {
		if build < d.from || build >= d.fixed {
			continue
		}
		// The events are only scanned for a defect tied to a kind.
		concerned := len(d.kinds) == 0
		if !concerned && !scanned {
			kinds, _ = l.observed()
			scanned = true
		}
		for _, k := range d.kinds {
			concerned = concerned || kinds.has(k)
		}
		if concerned {
			out = append(out, Warning{d.code, "arcdps " + strconv.Itoa(build) + ": " + d.text})
		}
	}
	slices.SortStableFunc(out, func(a, b Warning) int { return cmp.Compare(a.Code, b.Code) })
	return out
}

// namesNoSkill reports whether strikes, ticks and buff applications name
// skills and the skill table holds none of them. Other kinds may hold
// something else in SkillID. A sound log answers at its first such event,
// or a few later: the table lacks the skills arcdps emits itself.
func (l *Log) namesNoSkill() bool {
	var table map[uint32]struct{}
	for i := range l.Events {
		e := &l.Events[i]
		if e.SkillID == 0 || (e.IsStateChange != StateCombat && e.IsStateChange != StateBuffApply) {
			continue
		}
		if table == nil {
			table = make(map[uint32]struct{}, len(l.Skills))
			for _, s := range l.Skills {
				table[uint32(s.ID)] = struct{}{}
			}
		}
		if _, ok := table[e.SkillID]; ok {
			return false
		}
	}
	return table != nil
}
