package timeline

import (
	"slices"
	"testing"
)

func TestBuffIDs(t *testing.T) {
	if BuffMight != 740 || BuffQuickness != 1187 || BuffAlacrity != 30328 || BuffBurning != 737 || BuffVulnerability != 738 {
		t.Error("well-known buff ids are wrong")
	}
	if len(Boons) != 12 || len(Conditions) != 14 {
		t.Errorf("boons %d conditions %d", len(Boons), len(Conditions))
	}
	all := slices.Concat(Boons, Conditions, []uint32{BuffSuperspeed, BuffStealth, BuffRevealed})
	sorted := slices.Clone(all)
	slices.Sort(sorted)
	if len(slices.Compact(sorted)) != len(all) {
		t.Error("a buff id is listed twice")
	}
	// The fixture names Might and Burning by these ids.
	b := fixture()
	b.buffApply(1000, addrP2, addrP1, BuffMight, 1000, 1)
	tl := mustBuild(t, b.build(5000))
	if tl.Buff(BuffMight) == nil || tl.Buff(BuffMight).Skill.Name != "Might" || tl.Skill(BuffBurning).Name != "Burning" || tl.players[0].Stacks().OfBuff(BuffMight).Count() != 1 {
		t.Error("the fixture ids do not match the constants")
	}
}
