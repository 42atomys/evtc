package timeline

import (
	"fmt"
	"time"

	"github.com/42atomys/evtc"
)

// Skill is a skill, buff or effect id seen in the log. Names come from the
// skill table and are localized in the language of the recording client;
// arcdps custom ids that the table never names get an English name.
//
// The exported fields are read-only after Build.
type Skill struct {
	// ID is the skill id.
	ID uint32
	// Name is the skill name, in the language of the log.
	Name string
	// Custom is set for ids emitted by arcdps itself (evtc.SkillDodge and
	// the following constants).
	Custom bool
	// Buff is set when the skill is a buff.
	Buff *Buff
	// Info is the raw StateSkillInfo event, nil when absent.
	Info *evtc.Event
	// Cost is the resource cost of the skill, 0 without Info.
	Cost float32
	// MinRange is the minimum range of the skill, 0 without Info.
	MinRange float32
	// MaxRange is the maximum range of the skill, 0 without Info.
	MaxRange float32
	// TooltipTime is the cast time shown by the tooltip, 0 without Info.
	TooltipTime time.Duration
	// Timings are the trigger timings of the skill, in log order.
	Timings []SkillTiming
	// GUID is the content GUID of the skill, zero when the log carries
	// none.
	GUID GUID

	casts    []*Cast
	hits     []*Hit
	missiles []*Missile
	cnt      struct{ hits, casts, timings, missiles int }
}

// Casts returns every cast of the skill in the log, in start order.
func (s *Skill) Casts() Casts { return Casts{From(s.casts)} }

// Hits returns every hit of the skill in the log, in time order.
func (s *Skill) Hits() Hits { return Hits{From(s.hits)} }

// Missiles returns every missile of the skill in the log, in creation
// order.
func (s *Skill) Missiles() Missiles { return Missiles{From(s.missiles)} }

// String formats the skill as "Name (id)".
func (s *Skill) String() string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s (%d)", s.Name, s.ID)
}

// Buff is the definition of a buff, decoded from the StateBuffInfo event
// when the log carries one.
//
// The exported fields are read-only after Build.
type Buff struct {
	// Skill is the skill the buff belongs to.
	Skill *Skill
	// MaxDuration is the maximum combined duration, 0 when unknown.
	MaxDuration time.Duration
	// StackLimit is the maximum number of stacks, 0 when unknown.
	StackLimit int
	// Category is the raw arcdps buff category: 0 for a boon, 2 for a
	// condition. IsBoon and IsCondition read it.
	Category uint8
	// Stacking tells how the stacks of the buff combine.
	Stacking Stacking
	// Invuln is the "likely invulnerability" flag of arcdps.
	Invuln bool
	// Invert is the "likely inverted" flag of arcdps.
	Invert bool
	// Resistance is the "likely resistance" flag of arcdps.
	Resistance bool
	// Info is the raw StateBuffInfo event, nil when absent.
	Info *evtc.Event
	// Formulas are the effect formulas of the buff, in log order.
	Formulas []BuffFormula

	stacks    []*BuffStack
	cnt       int
	nFormulas int
}

// Stacks returns every stack of the buff in the log, in apply order.
func (b *Buff) Stacks() Stacks { return Stacks{From(b.stacks)} }

// String formats the buff through its skill.
func (b *Buff) String() string {
	if b == nil {
		return "<nil>"
	}
	return b.Skill.String()
}

// customSkillNames names the ids arcdps emits itself, which the skill table
// leaves unnamed.
var customSkillNames = map[uint32]string{
	evtc.SkillDodge:                       "Dodge",
	evtc.SkillDefianceDamage:              "Defiance Damage",
	evtc.SkillSelfCast1:                   "Self Cast 1",
	evtc.SkillEnemyCast1:                  "Enemy Cast 1",
	evtc.SkillSelfCast2:                   "Self Cast 2",
	evtc.SkillEnemyCast2:                  "Enemy Cast 2",
	evtc.SkillSelfCast3:                   "Self Cast 3",
	evtc.SkillEnemyCast3:                  "Enemy Cast 3",
	evtc.SkillBreakbarDefunc:              "Breakbar",
	evtc.SkillWeaponDraw:                  "Weapon Draw",
	evtc.SkillWeaponStow:                  "Weapon Stow",
	evtc.SkillGenericBlock:                "Block",
	evtc.SkillGenericDamage:               "Damage",
	evtc.SkillGenericKill:                 "Kill",
	evtc.SkillGenericDown:                 "Down",
	evtc.SkillGenericEvade:                "Evade",
	evtc.SkillGenericInterrupt:            "Interrupt",
	evtc.SkillGenericAbsorb:               "Absorb",
	evtc.SkillGenericMiss:                 "Miss",
	evtc.SkillGenericKnockdown:            "Knockdown",
	evtc.SkillGenericKnockbackPull:        "Knockback or Pull",
	evtc.SkillGenericFloatLand:            "Float or Land",
	evtc.SkillGenericLaunch:               "Launch",
	evtc.SkillGenericWaterFloatSinkDefunc: "Water Float or Sink",
	evtc.SkillGenericCCBuff:               "Crowd Control Buff",
	evtc.SkillGenericStagger:              "Stagger",
	evtc.SkillGenericInvalid:              "Invalid",
	evtc.SkillGadgetInteract:              "Gadget Interact",
	evtc.SkillEmote:                       "Emote",
	evtc.SkillGenericFloatWater:           "Float in Water",
	evtc.SkillGenericSink:                 "Sink",
	evtc.SkillGenericLockout:              "Lockout",
	evtc.SkillGenericFear:                 "Fear",
	evtc.SkillPickup:                      "Pickup",
	evtc.SkillGenericFallDown:             "Fall Down",
}

// isCustomSkill reports whether id belongs to the arcdps custom range.
func isCustomSkill(id uint32) bool {
	return id >= evtc.SkillDodge && id <= evtc.SkillGenericFallDown
}

// IsBoon reports whether the buff is a boon: a buff with definition data
// in category 0.
func (b *Buff) IsBoon() bool { return b.Info != nil && b.Category == 0 }

// IsCondition reports whether the buff is a condition (category 2).
func (b *Buff) IsCondition() bool { return b.Category == 2 }

// EffectiveStacks returns the number of stacks in effect when n are
// present: n capped at StackLimit for a buff stacking by intensity, 1
// for the others, whose extra applications only queue up duration.
func (b *Buff) EffectiveStacks(n int) int {
	if n <= 0 {
		return 0
	}
	if !b.Stacking.Intensity() {
		return 1
	}
	if b.StackLimit > 0 {
		return min(n, b.StackLimit)
	}
	return n
}

// Stacking is how the stacks of a buff combine, with the values of the
// arcdps stacking type.
type Stacking uint8

const (
	// StackingConditionalLoss stacks by intensity and loses stacks to a
	// condition, as stability does to crowd control.
	StackingConditionalLoss Stacking = iota
	// StackingQueue queues the applied durations: one stack is active at a
	// time and the others wait, as fury, quickness or alacrity do.
	StackingQueue
	// StackingCappedDuration holds a single stack whose duration is capped.
	StackingCappedDuration
	// StackingRegeneration keeps every stack but only the strongest acts.
	StackingRegeneration
	// StackingIntensity adds every stack to the effect, up to the stack
	// limit, as might, vulnerability and damaging conditions do.
	StackingIntensity
	// StackingForce is the remaining arcdps stacking type.
	StackingForce
)

var stackingNames = []string{"ConditionalLoss", "Queue", "CappedDuration", "Regeneration", "Intensity", "Force"}

// String returns the stacking type name.
func (s Stacking) String() string { return enumString(stackingNames, "Stacking", int(s)) }

// Intensity reports whether every stack adds to the effect of the buff.
func (s Stacking) Intensity() bool { return s == StackingIntensity || s == StackingConditionalLoss }
