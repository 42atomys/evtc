package timeline

import (
	"math"
	"testing"
)

func TestProfessions(t *testing.T) {
	if ProfessionGuardian.String() != "Guardian" || ProfessionRevenant.String() != "Revenant" || ProfessionUnknown.String() != "Unknown" {
		t.Error("profession names are wrong")
	}
	if Profession(42).String() != "Profession(42)" || Profession(math.MaxUint32).String() != "Profession(4294967295)" {
		t.Error("unknown profession names are wrong")
	}
	if EliteNone.String() != "None" || EliteFirebrand.String() != "Firebrand" || EliteLuminary.String() != "Luminary" || EliteDruid.String() != "Druid" {
		t.Error("elite specialization names are wrong")
	}
	if EliteSpec(1).String() != "EliteSpec(1)" || EliteSpec(99).String() != "EliteSpec(99)" || EliteSpec(math.MaxUint32).String() != "EliteSpec(4294967295)" {
		t.Error("unknown elite specialization names are wrong")
	}
	if EliteFirebrand.Profession() != ProfessionGuardian || EliteGaleshot.Profession() != ProfessionRanger || EliteNone.Profession() != ProfessionUnknown || EliteSpec(99).Profession() != ProfessionUnknown || EliteSpec(math.MaxUint32).Profession() != ProfessionUnknown {
		t.Error("elite specialization professions are wrong")
	}
	// The table holds four specializations per profession, 36 in total.
	perProfession := map[Profession]int{}
	for id, info := range eliteSpecs {
		if info.name == "" {
			continue
		}
		if EliteSpec(id).String() != info.name || info.prof == ProfessionUnknown {
			t.Errorf("entry %d = %+v is inconsistent", id, info)
		}
		perProfession[info.prof]++
	}
	if len(perProfession) != 9 {
		t.Errorf("professions with specializations = %d", len(perProfession))
	}
	for p := ProfessionGuardian; p <= ProfessionRevenant; p++ {
		if perProfession[p] != 4 {
			t.Errorf("%v has %d specializations", p, perProfession[p])
		}
	}
}

func TestPlayerSpec(t *testing.T) {
	b := fixture()
	b.player(0x1003, 13, "Charlie", ":Charlie.9012", "1", 3, 999)
	b.player(0x1004, 14, "Delta", ":Delta.3456", "1", 42, 0)
	tl := mustBuild(t, b.build(1000))

	for i, want := range []string{"Guardian", "Willbender", "Engineer", "Profession(42)"} {
		if got := tl.players[i].Spec(); got != want {
			t.Errorf("player %d Spec = %q, want %q", i, got, want)
		}
	}
	p2 := tl.players[1]
	if p2.Profession != ProfessionWarrior || p2.EliteSpec != EliteWillbender || p2.EliteSpec.Profession() != ProfessionGuardian {
		t.Errorf("player 2 = %v %v", p2.Profession, p2.EliteSpec)
	}
}
