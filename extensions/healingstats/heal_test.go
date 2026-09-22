package healingstats

import (
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// filterLog is the log of the filter tests: five heals of every kind.
func filterLog() *evtc.Log {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(1000, addrAlpha, addrBravo, skillHeal, 100, fromSrc)
	b.tick(2000, addrAlpha, addrAlpha, skillRegen, 50, both)
	b.barrier(3000, addrBravo, addrAlpha, skillSand, 300, fromDst)
	b.heal(4000, addrCharlie, addrAlpha, skillHeal, 70, fromDst|flagArcDowned)
	b.minionHeal(5000, addrPet, instAlpha, addrCharlie, skillHeal, 20, fromSrc)
	return b.build(10000)
}

func amounts(q Heals) []int32 { return q.Map(func(h *Heal) int32 { return h.Amount }) }

func TestHealsFilters(t *testing.T) {
	s := mustBuild(t, filterLog())
	tl := s.Timeline
	alpha := player(t, s, addrAlpha)
	q := s.Heals()
	for _, tt := range []struct {
		name string
		q    Heals
		want []int32
	}{
		{"all", q, []int32{100, 50, 300, 70, 20}},
		{"By node", q.By(alpha), []int32{100, 50}},
		{"By player", q.By(tl.Players().First()), []int32{100, 50}},
		{"By agent", q.By(alpha.Agent), []int32{100, 50}},
		{"By nil", q.By(nil), nil},
		{"By typed nil", q.By((*timeline.Player)(nil)), nil},
		{"By nil node", q.By((*Agent)(nil)), nil},
		{"CreditedTo", q.CreditedTo(alpha), []int32{100, 50, 20}},
		{"CreditedTo nil", q.CreditedTo(nil), nil},
		{"On", q.On(alpha), []int32{50, 300, 70}},
		{"On nil", q.On(nil), nil},
		{"OfSkill", q.OfSkill(skillHeal), []int32{100, 70, 20}},
		{"Of", q.Of(tl.Skill(skillRegen)), []int32{50}},
		{"Of nil", q.Of(nil), nil},
		{"Healing", q.Healing(), []int32{100, 50, 70, 20}},
		{"Barrier", q.Barrier(), []int32{300}},
		{"Direct", q.Direct(), []int32{100, 300, 70, 20}},
		{"Ticks", q.Ticks(), []int32{50}},
		{"Downed", q.Downed(), []int32{70}},
		{"Self", q.Self(), []int32{50}},
		{"Others", q.Others(), []int32{100, 300, 70, 20}},
		{"Between", q.Between(timeline.NewInterval(2*time.Second, 4*time.Second)), []int32{50, 300, 70}},
		{"Between outside", q.Between(timeline.NewInterval(6*time.Second, 7*time.Second)), nil},
		{"Skip Limit", q.Skip(1).Limit(2), []int32{50, 300}},
		{"Reverse Limit", q.Reverse().Limit(2), []int32{20, 70}},
		{"Where", q.Where(func(h *Heal) bool { return h.Amount > 60 }), []int32{100, 300, 70}},
		{"chained", q.On(alpha).Healing().Others(), []int32{70}},
	} {
		got := amounts(tt.q)
		if len(got) != len(tt.want) || (len(got) > 0 && !slices.Equal(got, tt.want)) || tt.q.Count() != len(tt.want) || tt.q.Any() != (len(tt.want) > 0) {
			t.Errorf("%s = %v (count %d), want %v", tt.name, got, tt.q.Count(), tt.want)
		}
	}
	if q.Amount() != 540 || q.Healed() != 240 || q.BarrierGiven() != 300 || q.Healing().Amount() != 240 || q.Barrier().Amount() != 300 {
		t.Errorf("sums: amount %d healed %d barrier %d", q.Amount(), q.Healed(), q.BarrierGiven())
	}
	if hps, bps := q.HPS(tl.Interval()), q.BPS(tl.Interval()); hps != 24 || bps != 30 {
		t.Errorf("HPS = %v BPS = %v over %v", hps, bps, tl.Interval())
	}
	if q.HPS(timeline.NewInterval(0, 2*time.Second)) != 75 || q.HPS(timeline.At(time.Second)) != 0 || q.BPS(timeline.NewInterval(5*time.Second, 6*time.Second)) != 0 {
		t.Errorf("HPS over two seconds = %v", q.HPS(timeline.NewInterval(0, 2*time.Second)))
	}
	if h := q.First(); h.Time != time.Second || q.Last().Time != 5*time.Second || q.Reverse().First().Amount != 20 {
		t.Errorf("first %+v last %+v", q.First(), q.Last())
	}
}

func TestHealsRankings(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(1000, addrAlpha, addrBravo, skillHeal, 100, fromSrc)
	b.heal(2000, addrBravo, addrAlpha, skillHeal, 300, fromDst)
	b.minionHeal(3000, addrPet, instAlpha, addrCharlie, skillHeal, 250, fromSrc)
	b.tick(4000, 0, addrAlpha, skillRegen, 300, fromDst) // as much as Bravo, met later
	s := mustBuild(t, b.build(10000))
	alpha, bravo, charlie := player(t, s, addrAlpha), player(t, s, addrBravo), player(t, s, addrCharlie)
	q := s.Heals()

	shares := q.PerAgent()
	if len(shares) != 3 || shares[0].Agent != alpha || shares[0].Heals.Amount() != 350 || shares[1].Agent != bravo || shares[2].Agent != s.Unknown || shares[0].Heals.First().Time != time.Second {
		t.Errorf("PerAgent = %v", shares)
	}
	if rev := q.Reverse().PerAgent(); len(rev) != 3 || rev[0].Agent != alpha || rev[0].Heals.First().Time != time.Second || rev[0].Heals.Last().Time != 3*time.Second {
		t.Errorf("reversed PerAgent = %v", rev)
	}
	targets := q.PerTarget()
	if len(targets) != 3 || targets[0].Agent != alpha || targets[0].Heals.Amount() != 600 || targets[1].Agent != charlie || targets[2].Agent != bravo {
		t.Errorf("PerTarget = %v", targets)
	}
	skills := q.PerSkill()
	if len(skills) != 2 || skills[0].Skill.ID != skillHeal || skills[0].Heals.Amount() != 650 || skills[1].Skill.ID != skillRegen || skills[1].Heals.Count() != 1 {
		t.Errorf("PerSkill = %v", skills)
	}
	if q.Barrier().PerAgent() != nil || q.Barrier().PerTarget() != nil || q.Barrier().PerSkill() != nil {
		t.Error("rankings of an empty query are not nil")
	}
	groups := q.GroupBy(func(h *Heal) *Agent { return h.Dst })
	if len(groups) != 3 || groups[alpha].Count() != 2 || groups[alpha].Amount() != 600 || groups[bravo].First().Amount != 100 {
		t.Errorf("GroupBy = %v", groups)
	}
	if rev := q.Reverse().GroupBy(func(h *Heal) *Agent { return h.Dst }); rev[alpha].First().Time != 2*time.Second || rev[alpha].Last().Time != 4*time.Second {
		t.Errorf("reversed GroupBy is not in time order: %v", rev[alpha].All())
	}
	checkInvariants(t, s)
}

func TestHealsOfPlayerWithTwoCharacters(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	b.player(addrBravo, instBravo, "Echo", ":Bravo.5678", "1", 5, 0)
	b.heal(1000, addrAlpha, addrBravo, skillHeal, 100, fromSrc)
	b.heal(2000, addrBravo, addrAlpha, skillHeal, 40, fromDst)
	b.inst[addrBravo] = 88
	b.heal(3000, addrAlpha, addrBravo, skillHeal, 200, fromSrc)
	b.heal(4000, addrBravo, addrAlpha, skillHeal, 60, fromDst)
	s := mustBuild(t, b.build(10000))
	tl := s.Timeline

	p := tl.PlayerByAccount("Bravo.5678")
	bravo, echo := tl.CharacterByName("Bravo"), tl.CharacterByName("Echo")
	if len(p.Characters()) != 2 || echo == nil {
		t.Fatalf("characters = %v", p.Characters())
	}
	q := s.Heals()
	for _, tt := range []struct {
		name string
		q    Heals
		want []int32
	}{
		{"On player", q.On(p), []int32{100, 200}},
		{"On first character", q.On(bravo), []int32{100}},
		{"On second character", q.On(echo), []int32{200}},
		{"By player", q.By(p), []int32{40, 60}},
		{"CreditedTo player", q.CreditedTo(p), []int32{40, 60}},
		{"By second character", q.By(echo), []int32{60}},
	} {
		if got := amounts(tt.q); !slices.Equal(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
		}
	}
}
