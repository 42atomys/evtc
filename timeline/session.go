package timeline

import (
	"encoding/hex"
	"strings"
	"time"

	"github.com/42atomys/evtc"
)

// Language is the text language of the recording client, with the ids
// of arcdps.
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

// Ruleset is the game mode of the recording client, as bits.
type Ruleset uint8

const (
	RulesetPvE Ruleset = 1 << iota // Player versus environment.
	RulesetWvW                     // World versus world.
	RulesetPvP                     // Player versus player.
)

// IsPvE reports whether the PvE bit is set.
func (r Ruleset) IsPvE() bool { return r&RulesetPvE != 0 }

// IsWvW reports whether the WvW bit is set.
func (r Ruleset) IsWvW() bool { return r&RulesetWvW != 0 }

// IsPvP reports whether the PvP bit is set.
func (r Ruleset) IsPvP() bool { return r&RulesetPvP != 0 }

// String lists the modes set.
func (r Ruleset) String() string {
	var modes []string
	if r.IsPvE() {
		modes = append(modes, "PvE")
	}
	if r.IsWvW() {
		modes = append(modes, "WvW")
	}
	if r.IsPvP() {
		modes = append(modes, "PvP")
	}
	if len(modes) == 0 {
		return "Ruleset(0)"
	}
	return strings.Join(modes, "|")
}

// GUID is a content GUID of the game, as arcdps logs it.
type GUID [16]byte

// IsZero reports whether the GUID is unset.
func (g GUID) IsZero() bool { return g == GUID{} }

// String returns the GUID as 32 uppercase hexadecimal digits, in the byte
// order of the log.
func (g GUID) String() string { return strings.ToUpper(hex.EncodeToString(g[:])) }

// ContentKind is the kind of content an id to GUID association describes,
// with the values arcdps writes. Its README leaves the team out and gives
// emotes 4 and transformations 5; in the logs a transformation is 6. Teams
// are in the logs of arcdps 20260226 to 20260604, emotes in those of
// 20260414 to 20260604.
type ContentKind uint8

const (
	// ContentEffect is an effect id.
	ContentEffect ContentKind = iota
	// ContentMarker is a marker id.
	ContentMarker
	// ContentSkill is a skill id.
	ContentSkill
	// ContentSpecies is a species id.
	ContentSpecies
	// ContentTeam is a team id.
	//
	// Deprecated: arcdps 20260701 stopped writing it.
	ContentTeam
	// ContentEmote is an emote id.
	//
	// Deprecated: arcdps 20260701 stopped writing it.
	ContentEmote
	// ContentTransformation is a transformation id.
	ContentTransformation
)

var contentNames = []string{"Effect", "Marker", "Skill", "Species", "Team", "Emote", "Transformation"}

// String returns the kind name.
func (k ContentKind) String() string { return enumString(contentNames, "ContentKind", int(k)) }

// contentKey identifies one id to GUID association.
type contentKey struct {
	kind ContentKind
	id   uint32
}

// StunBreak is a disable an agent broke early.
type StunBreak struct {
	// Time is the time of the stun break.
	Time time.Duration
	// Remaining is the duration the disable had left.
	Remaining time.Duration
	// Agent is the agent that broke the disable.
	Agent *Agent
	// Event is the stun break event.
	Event *evtc.Event
}

// SkillTiming is one trigger timing of a skill: what happens at what time
// after the activation.
type SkillTiming struct {
	// Kind is the raw arcdps timing type.
	Kind uint32
	// At is the time since the activation.
	At time.Duration
	// Event is the skill timing event.
	Event *evtc.Event
}

// BuffFormula is one effect formula of a buff, with the raw arcdps
// values: the formula type, the attributes it acts on, its parameters and
// the trait or buff conditions gating it.
type BuffFormula struct {
	// Type is the raw formula type.
	Type float32
	// Attribute1 and Attribute2 are the attributes the formula acts on.
	Attribute1, Attribute2 float32
	// Parameter1 to Parameter3 are the parameters of the formula.
	Parameter1, Parameter2, Parameter3 float32
	// TraitConditionSource is the trait the source must have, 0 for none.
	TraitConditionSource float32
	// TraitConditionSelf is the trait the receiver must have, 0 for none.
	TraitConditionSelf float32
	// ContentReference is the raw content reference of the formula.
	ContentReference float32
	// BuffConditionSource is the buff the source must have, 0 for none.
	BuffConditionSource float32
	// BuffConditionSelf is the buff the receiver must have, 0 for none.
	BuffConditionSelf float32
	// Event is the buff formula event.
	Event *evtc.Event
}

// GadgetAnimation is a model animation played by a gadget.
type GadgetAnimation struct {
	// Time is the time of the animation.
	Time time.Duration
	// Token is the animation token of the game.
	Token uint64
	// Agent is the gadget or NPC playing the animation.
	Agent *Agent
	// Event is the animation event.
	Event *evtc.Event
}

// Reward is a reward received during the log.
type Reward struct {
	// Time is the time of the reward.
	Time time.Duration
	// ID is the reward id, as logged.
	ID uint64
	// Kind is the reward type, as logged.
	Kind int32
	// Event is the reward event.
	Event *evtc.Event
}

// MapChange is a change of map of the recording client.
type MapChange struct {
	// Time is the time of the change.
	Time time.Duration
	// From is the old map id.
	From uint32
	// To is the new map id.
	To uint32
	// Kind is the type of the new map, as logged.
	Kind int32
	// Event is the map change event.
	Event *evtc.Event
}

// IntegrityMessage is a diagnostic message arcdps wrote into the log.
type IntegrityMessage struct {
	// Message is the text of the event, empty when it is not printable.
	Message string
	// Event is the integrity event.
	Event *evtc.Event
}
