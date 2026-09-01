package timeline

import (
	"testing"

	"github.com/42atomys/evtc"
)

func TestSkillMetadata(t *testing.T) {
	b := fixture()
	b.skillInfo(skillSlam, 5, 130, 600, 0.75)
	b.skillTiming(skillSlam, 1, 300)
	b.skillTiming(skillSlam, 2, 900)
	b.skillTiming(skillHeat, 1, 100)
	b.buffFormula(skillBuff, [11]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	b.buffFormula(skillBuff, [11]float32{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	slam, effect, species := GUID{0xAB, 1}, GUID{0xEF}, GUID{0x5F}
	b.idToGUID(ContentSkill, skillSlam, slam, 0)
	b.idToGUID(ContentEffect, 5000, effect, 1500)
	b.idToGUID(ContentSpecies, 15375, species, 0)
	tl := mustBuild(t, b.build(5000))

	s := tl.Skill(skillSlam)
	if s.Cost != 5 || s.MinRange != 130 || s.MaxRange != 600 || s.TooltipTime != 750*msec || s.Info == nil || s.GUID != slam {
		t.Errorf("skill info = %+v", s)
	}
	if len(s.Timings) != 2 || s.Timings[0].Kind != 1 || s.Timings[0].At != 300*msec || s.Timings[1].At != 900*msec || s.Timings[0].Event == nil {
		t.Errorf("timings = %+v", s.Timings)
	}
	if len(tl.Skill(skillHeat).Timings) != 1 || len(tl.Skill(skillBurn).Timings) != 0 || tl.Skill(skillHeat).Cost != 0 || !tl.Skill(skillHeat).GUID.IsZero() {
		t.Error("timings of the other skills are wrong")
	}
	might := tl.Buff(skillBuff)
	if len(might.Formulas) != 2 || might.Formulas[0].Type != 1 || might.Formulas[0].Attribute1 != 2 || might.Formulas[0].Parameter3 != 6 || might.Formulas[0].ContentReference != 9 || might.Formulas[0].BuffConditionSource != 10 || might.Formulas[0].BuffConditionSelf != 11 || might.Formulas[1].Type != 2 || might.Formulas[0].Event == nil {
		t.Errorf("formulas = %+v", might.Formulas)
	}
	if tl.GUID(ContentEffect, 5000) != effect || tl.effectDefaults[5000] != 1500*msec {
		t.Error("effect GUID or default duration is wrong")
	}
	if tl.GUID(ContentSpecies, 15375) != species {
		t.Error("species GUID is wrong")
	}
	if !tl.GUID(ContentSkill, 999).IsZero() || slam.String() != "AB010000000000000000000000000000" {
		t.Error("GUID edge cases are wrong")
	}
	checkInvariants(t, tl)
}

func TestSkillTableEdgeCases(t *testing.T) {
	b := fixture()
	b.skill(skillSlam, "Duplicate")
	b.skill(4242, "")
	b.hit(1000, addrP1, addrBoss, 4242, 1, evtc.ResultStrikeDamageNormal)
	b.buffApply(1000, addrP2, addrP1, skillBuff, 1000, 1)
	b.buffTick(1500, addrP2, addrBoss, skillBuff, 10)
	tl := mustBuild(t, b.build(10000))

	if tl.Skill(skillSlam).Name != "Slam" || tl.Skill(4242).Name != "4242" || len(tl.Skills) != 5 {
		t.Errorf("skills = %v", tl.Skills)
	}
	if got := tl.Buff(skillBuff).String(); got != "Might (740)" {
		t.Errorf("Buff.String = %q", got)
	}
	if tl.Buff(skillBuff).StackLimit != 0 || tl.Buff(skillBuff).Info != nil {
		t.Error("a buff without BuffInfo carries definition data")
	}
	checkInvariants(t, tl)
}
