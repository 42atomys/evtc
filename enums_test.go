package evtc

import (
	"fmt"
	"testing"
)

func TestEnumValues(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"IFFUnknown", int(IFFUnknown), 2},
		{"ResultKillingBlow", int(ResultKillingBlow), 8},
		{"ResultBuffDamageCycle", int(ResultBuffDamageCycle), 14},
		{"ResultUnknown", int(ResultUnknown), 19},
		{"ActivationCancel", int(ActivationCancel), 4},
		{"ActivationUnknown", int(ActivationUnknown), 7},
		{"BuffRemoveManual", int(BuffRemoveManual), 3},
		{"BuffRemoveUnknown", int(BuffRemoveUnknown), 4},
		{"BuffCycleNotCycleDmgToTargetOnStackRemove", int(BuffCycleNotCycleDmgToTargetOnStackRemove), 5},
		{"LanguageFrench", int(LanguageFrench), 2},
		{"LanguageChinese", int(LanguageChinese), 5},
		{"ContentLocalTeam", int(ContentLocalTeam), 4},
		{"ContentLocalTransformation", int(ContentLocalTransformation), 6},
		{"SkillDodge", SkillDodge, 23275},
		{"SkillWeaponDraw", SkillWeaponDraw, 23284},
		{"SkillEmote", SkillEmote, 23303},
		{"SkillGenericFallDown", SkillGenericFallDown, 23309},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestEnumString(t *testing.T) {
	tests := []struct {
		v    fmt.Stringer
		want string
	}{
		{IFFFoe, "Foe"},
		{IFF(9), "IFF(9)"},
		{ResultStrikeDamageCrit, "StrikeDamageCrit"},
		{Result(200), "Result(200)"},
		{ActivationReset, "Reset"},
		{BuffRemoveAll, "All"},
		{BuffCycleNotCycle, "NotCycle"},
		{LanguageGerman, "German"},
		{Language(1), "Language(1)"},
		{ContentLocalSkill, "Skill"},
		{ContentLocal(42), "ContentLocal(42)"},
	}
	for _, tt := range tests {
		if got := tt.v.String(); got != tt.want {
			t.Errorf("%T(%v).String() = %q, want %q", tt.v, tt.v, got, tt.want)
		}
	}
}

// TestEnumNamesComplete checks every dense enum has a name for each value.
func TestEnumNamesComplete(t *testing.T) {
	tables := map[string]struct {
		names []string
		last  int
	}{
		"iff":          {iffNames, int(IFFUnknown)},
		"result":       {resultNames, int(ResultUnknown)},
		"activation":   {activationNames, int(ActivationUnknown)},
		"buffRemove":   {buffRemoveNames, int(BuffRemoveUnknown)},
		"buffCycle":    {buffCycleNames, int(BuffCycleUnknown)},
		"contentLocal": {contentLocalNames, int(ContentLocalTransformation)},
	}
	for name, tb := range tables {
		if len(tb.names) != tb.last+1 {
			t.Errorf("%s: %d names, want %d", name, len(tb.names), tb.last+1)
		}
		for i, n := range tb.names {
			if n == "" {
				t.Errorf("%s: value %d has no name", name, i)
			}
		}
	}
}
