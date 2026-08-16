package evtc

import "strconv"

// enumString returns names[v] when it exists, otherwise typ(v).
func enumString(names []string, typ string, v int) string {
	if v >= 0 && v < len(names) && names[v] != "" {
		return names[v]
	}
	return typ + "(" + strconv.Itoa(v) + ")"
}

// IFF is the friend or foe relation carried by Event.IFF. It mirrors the
// iff enum of arcdps.
type IFF uint8

const (
	IFFFriend  IFF = iota // An ally.
	IFFFoe                // An enemy.
	IFFUnknown            // Unknown relation.
)

var iffNames = []string{
	IFFFriend:  "Friend",
	IFFFoe:     "Foe",
	IFFUnknown: "Unknown",
}

// String returns the arcdps name of the relation, without its IFF_ prefix.
func (i IFF) String() string { return enumString(iffNames, "IFF", int(i)) }

// Result is the combat result carried by Event.Result. It mirrors the
// cbtresult enum of arcdps.
type Result uint8

const (
	// ResultStrikeDamageNormal when damage is a strike.
	ResultStrikeDamageNormal Result = iota
	// ResultStrikeDamageCrit when the strike was a crit.
	ResultStrikeDamageCrit
	// ResultStrikeDamageGlance when the strike was a glance.
	ResultStrikeDamageGlance
	// ResultBlock when the strike was blocked.
	ResultBlock
	// ResultEvade when the strike was evaded.
	ResultEvade
	// ResultInterrupt when the action was interrupted.
	ResultInterrupt
	// ResultAbsorb when the target was invulnerable or absorbed the strike.
	ResultAbsorb
	// ResultBlind when the action missed.
	ResultBlind
	// ResultKillingBlow when the target was killed by the skill.
	ResultKillingBlow
	// ResultDowned when the target was downed by the skill.
	ResultDowned
	// ResultDefianceDamageNormal when damage is to defiance.
	ResultDefianceDamageNormal
	// ResultSkillCast is an on-skill-use signal event.
	ResultSkillCast
	// ResultCrowdControl when the target was crowd controlled.
	ResultCrowdControl
	// ResultInvert when damage was inverted.
	ResultInvert
	// ResultBuffDamageCycle when buff damage happened on the tick timer.
	ResultBuffDamageCycle
	// ResultBuffDamageNotCycle when buff damage happened outside the tick
	// timer.
	ResultBuffDamageNotCycle
	// ResultBuffDamageNotCycleDmgToTargetOnHit when buff damage happened to
	// the target on hitting the target.
	ResultBuffDamageNotCycleDmgToTargetOnHit
	// ResultBuffDamageNotCycleDmgToSourceOnHit when buff damage happened to
	// the source on hitting the target.
	ResultBuffDamageNotCycleDmgToSourceOnHit
	// ResultBuffDamageNotCycleDmgToTargetOnStackRemove when buff damage
	// happened to the target on buff removal.
	ResultBuffDamageNotCycleDmgToTargetOnStackRemove
	// ResultUnknown is any result newer than this list.
	ResultUnknown
)

var resultNames = []string{
	ResultStrikeDamageNormal:                 "StrikeDamageNormal",
	ResultStrikeDamageCrit:                   "StrikeDamageCrit",
	ResultStrikeDamageGlance:                 "StrikeDamageGlance",
	ResultBlock:                              "Block",
	ResultEvade:                              "Evade",
	ResultInterrupt:                          "Interrupt",
	ResultAbsorb:                             "Absorb",
	ResultBlind:                              "Blind",
	ResultKillingBlow:                        "KillingBlow",
	ResultDowned:                             "Downed",
	ResultDefianceDamageNormal:               "DefianceDamageNormal",
	ResultSkillCast:                          "SkillCast",
	ResultCrowdControl:                       "CrowdControl",
	ResultInvert:                             "Invert",
	ResultBuffDamageCycle:                    "BuffDamageCycle",
	ResultBuffDamageNotCycle:                 "BuffDamageNotCycle",
	ResultBuffDamageNotCycleDmgToTargetOnHit: "BuffDamageNotCycleDmgToTargetOnHit",
	ResultBuffDamageNotCycleDmgToSourceOnHit: "BuffDamageNotCycleDmgToSourceOnHit",
	ResultBuffDamageNotCycleDmgToTargetOnStackRemove: "BuffDamageNotCycleDmgToTargetOnStackRemove",
	ResultUnknown: "Unknown",
}

// String returns the arcdps name of the result, without its CBTR_ prefix.
func (r Result) String() string { return enumString(resultNames, "Result", int(r)) }

// Activation is the animation progress carried by Event.IsActivation on
// StateAnimationStop events. It mirrors the cbtanimation enum of arcdps.
type Activation uint8

const (
	// ActivationNone when not an animation event.
	ActivationNone Activation = iota
	// ActivationStartDefunc is retired.
	ActivationStartDefunc
	// ActivationQuicknessDefunc is retired.
	ActivationQuicknessDefunc
	// ActivationMinimum when the animation stopped after reaching the
	// minimum of the first trigger point or tooltip time.
	ActivationMinimum
	// ActivationCancel when the animation stopped before reaching the
	// minimum of the first trigger point or tooltip time.
	ActivationCancel
	// ActivationReset when the animation completed fully.
	ActivationReset
	// ActivationNoData is the same as ActivationMinimum but on an uncertain
	// expected duration.
	ActivationNoData
	// ActivationUnknown is any value newer than this list.
	ActivationUnknown
)

var activationNames = []string{
	ActivationNone:            "None",
	ActivationStartDefunc:     "StartDefunc",
	ActivationQuicknessDefunc: "QuicknessDefunc",
	ActivationMinimum:         "Minimum",
	ActivationCancel:          "Cancel",
	ActivationReset:           "Reset",
	ActivationNoData:          "NoData",
	ActivationUnknown:         "Unknown",
}

// String returns the arcdps name of the activation, without its ACTV_
// prefix.
func (a Activation) String() string { return enumString(activationNames, "Activation", int(a)) }

// BuffRemove is the buff removal kind carried by Event.IsBuffRemove on
// StateBuffRemoveSingle and StateBuffRemoveAll events. It mirrors the
// cbtbuffremove enum of arcdps.
type BuffRemove uint8

const (
	// BuffRemoveNone when not a buff removal event.
	BuffRemoveNone BuffRemove = iota
	// BuffRemoveAll when the last or all stacks were removed (sent by
	// server).
	BuffRemoveAll
	// BuffRemoveSingle when a single stack was removed (sent by server).
	BuffRemoveSingle
	// BuffRemoveManual when a single stack was removed (created by arcdps
	// on all stack remove).
	BuffRemoveManual
	// BuffRemoveUnknown is any value newer than this list.
	BuffRemoveUnknown
)

var buffRemoveNames = []string{
	BuffRemoveNone:    "None",
	BuffRemoveAll:     "All",
	BuffRemoveSingle:  "Single",
	BuffRemoveManual:  "Manual",
	BuffRemoveUnknown: "Unknown",
}

// String returns the arcdps name of the removal, without its CBTB_ prefix.
func (b BuffRemove) String() string { return enumString(buffRemoveNames, "BuffRemove", int(b)) }

// Language is the text language id carried by Event.SrcAgent on
// StateLanguage events. It mirrors the gwlanguage enum of arcdps.
type Language uint8

const (
	LanguageEnglish Language = 0 // English client.
	LanguageFrench  Language = 2 // French client.
	LanguageGerman  Language = 3 // German client.
	LanguageSpanish Language = 4 // Spanish client.
	LanguageChinese Language = 5 // Chinese client.
)

var languageNames = []string{
	LanguageEnglish: "English",
	LanguageFrench:  "French",
	LanguageGerman:  "German",
	LanguageSpanish: "Spanish",
	LanguageChinese: "Chinese",
}

// String returns the language name.
func (l Language) String() string { return enumString(languageNames, "Language", int(l)) }

// ContentLocal is the content type carried by Event.OverstackValue on
// StateIDToGUID events. It mirrors the n_contentlocal enum of arcdps.
type ContentLocal uint32

const (
	// ContentLocalEffect is an effect. src_instid: content type,
	// buff_dmg: float default duration if available.
	ContentLocalEffect ContentLocal = iota
	// ContentLocalMarker is a marker. src_instid: is in commander tag defs.
	ContentLocalMarker
	// ContentLocalSkill is a skill, see StateSkillInfo and StateBuffInfo.
	ContentLocalSkill
	// ContentLocalSpeciesNotGadget is a non-gadget species.
	ContentLocalSpeciesNotGadget
	// ContentLocalEmote is an emote.
	ContentLocalEmote
	// ContentLocalTransformation is a transformation.
	ContentLocalTransformation
)

var contentLocalNames = []string{
	ContentLocalEffect:           "Effect",
	ContentLocalMarker:           "Marker",
	ContentLocalSkill:            "Skill",
	ContentLocalSpeciesNotGadget: "SpeciesNotGadget",
	ContentLocalEmote:            "Emote",
	ContentLocalTransformation:   "Transformation",
}

// String returns the arcdps name of the content type, without its
// CONTENTLOCAL_ prefix.
func (c ContentLocal) String() string { return enumString(contentLocalNames, "ContentLocal", int(c)) }

// Custom skill ids emitted by arcdps itself rather than the game. They
// mirror the n_customskill enum and can be compared with Event.SkillID.
const (
	SkillDodge = 23275 + iota
	SkillDefianceDamage
	SkillSelfCast1
	SkillEnemyCast1
	SkillSelfCast2
	SkillEnemyCast2
	SkillSelfCast3
	SkillEnemyCast3
	SkillBreakbarDefunc
	SkillWeaponDraw
	SkillWeaponStow
	SkillGenericBlock
	SkillGenericDamage
	SkillGenericKill
	SkillGenericDown
	SkillGenericEvade
	SkillGenericInterrupt
	SkillGenericAbsorb
	SkillGenericMiss
	SkillGenericKnockdown
	SkillGenericKnockbackPull
	SkillGenericFloatLand
	SkillGenericLaunch
	SkillGenericWaterFloatSinkDefunc
	SkillGenericCCBuff
	SkillGenericStagger
	SkillGenericInvalid
	SkillGadgetInteract
	// SkillEmote carries the emote id in StateAnimationStart.
	SkillEmote
	SkillGenericFloatWater
	SkillGenericSink
	SkillGenericLockout
	SkillGenericFear
	// SkillPickup carries the item id in StateAnimationStart.
	SkillPickup
	SkillGenericFallDown
)
