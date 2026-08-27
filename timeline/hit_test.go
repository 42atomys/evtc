package timeline

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestHitFlags(t *testing.T) {
	b := fixture()
	for i, moving := range []uint8{0, 1, 2, 3} {
		b.add(evtc.Event{Time: b.at(uint64(1000 + i)), SrcAgent: addrP1, DstAgent: addrBoss, SkillID: skillSlam, Value: 1, IsMoving: moving, IsNinety: 1, IsOffcycle: uint8(i % 2), IFF: evtc.IFFFoe})
	}
	b.add(evtc.Event{Time: b.at(2000), SrcAgent: addrP1, DstAgent: addrP2, SkillID: skillHeat, Value: 1, IFF: evtc.IFFFriend})
	tl := mustBuild(t, b.build(10000))
	hits := tl.Players[0].Hits().All()
	for i, want := range []struct{ src, dst bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		if h := hits[i]; h.Moving != want.src || h.TargetMoving != want.dst || !h.OverNinety || h.TargetDowned != (i%2 == 1) {
			t.Errorf("hit %d flags = %+v", i, h)
		}
	}
	q := tl.Players[0].Hits()
	if q.Foes().Count() != 4 || q.Friends().Count() != 1 || q.Friends().First().Dst != tl.Players[1].Agent {
		t.Errorf("Foes = %d Friends = %d", q.Foes().Count(), q.Friends().Count())
	}
}

func TestHitsAggregates(t *testing.T) {
	b := fixture()
	b.hit(1000, addrP1, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	b.add(evtc.Event{Time: b.at(2000), SrcAgent: addrP1, DstAgent: addrBoss, SkillID: skillSlam, Value: 50, OverstackValue: 30, IsShields: 1, IFF: evtc.IFFFoe})
	// A buff tick fully absorbed by barrier, and a strike whose overstack
	// is meaningless because is_shields is clear.
	b.add(evtc.Event{Time: b.at(2500), SrcAgent: addrP1, DstAgent: addrBoss, SkillID: skillBurn, BuffDamage: 20, OverstackValue: 20, IsShields: 1, Buff: 1, Result: evtc.ResultBuffDamageCycle})
	b.add(evtc.Event{Time: b.at(2600), SrcAgent: addrP1, DstAgent: addrBoss, SkillID: skillSlam, Value: 10, OverstackValue: 99, IFF: evtc.IFFFoe})
	tl := mustBuild(t, b.build(10000))
	hits := tl.Players[0].Hits()

	if hits.Damage() != 180 || hits.Barrier() != 50 || hits.HealthDamage() != 130 || hits.Between(At(time.Second)).Barrier() != 0 {
		t.Errorf("Damage = %d Barrier = %d HealthDamage = %d", hits.Damage(), hits.Barrier(), hits.HealthDamage())
	}
	if tick := hits.BuffDamage().First(); tick.Damage != 20 || tick.Barrier != 20 || tick.HealthDamage() != 0 || !tick.Shielded {
		t.Errorf("absorbed tick = %+v", tick)
	}
	if last := hits.Last(); last.Barrier != 0 || last.Shielded || last.HealthDamage() != 10 {
		t.Errorf("unshielded strike = %+v", last)
	}
	for _, tt := range []struct {
		iv   Interval
		want float64
	}{
		{NewInterval(0, 3*time.Second), 60},
		{NewInterval(1500*msec, 2500*msec), 70},
		{NewInterval(0, 1500*msec), 100.0 / 1.5},
		{At(time.Second), 0},
		{NewInterval(5*time.Second, 6*time.Second), 0},
	} {
		if got := hits.DPS(tt.iv); got != tt.want {
			t.Errorf("DPS(%v) = %v, want %v", tt.iv, got, tt.want)
		}
	}
	checkInvariants(t, tl)
}

func TestPerAgent(t *testing.T) {
	b := fixture()
	b.hit(1000, addrP1, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	b.hit(1100, addrP2, addrBoss, skillHeat, 300, evtc.ResultStrikeDamageNormal)
	b.minionHit(1200, addrPet, addrBoss, instP1, skillHeat, 250)
	b.hit(1300, 0, addrBoss, skillSlam, 300, evtc.ResultStrikeDamageNormal)
	b.hit(1400, addrBoss, addrP1, skillSlam, 500, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(10000))
	p1, p2, boss, pet := tl.Players[0], tl.Players[1], tl.Targets[0], tl.Agent(addrPet)

	shares := boss.HitsTaken().PerAgent()
	want := []struct {
		agent  *Agent
		hits   int
		damage int64
	}{{p1.Agent, 2, 350}, {p2.Agent, 1, 300}, {tl.Unknown, 1, 300}}
	if len(shares) != len(want) {
		t.Fatalf("PerAgent = %v", shares)
	}
	for i, w := range want {
		if c := shares[i]; c.Agent != w.agent || c.Hits.Count() != w.hits || c.Hits.Damage() != w.damage {
			t.Errorf("share %d = %v with %d hits for %d, want %v %d %d", i, c.Agent, c.Hits.Count(), c.Hits.Damage(), w.agent, w.hits, w.damage)
		}
	}
	if shares[0].Hits.First().Src != p1.Agent || shares[0].Hits.Last().Src != pet {
		t.Error("the hits of a share are not in time order")
	}
	if all := tl.Hits().PerAgent(); len(all) != 4 || all[0].Agent != boss.Agent || all[0].Hits.Damage() != 500 {
		t.Errorf("PerAgent over the log = %v", all)
	}
	if p2.HitsTaken().PerAgent() != nil {
		t.Error("PerAgent of no hits is not nil")
	}
	if got := boss.HitsTaken().Limit(2).PerAgent(); len(got) != 2 || got[0].Agent != p2.Agent || got[1].Hits.Damage() != 100 {
		t.Errorf("PerAgent with a window = %v", got)
	}
	// The hits of a share stay in time order after a reversed traversal;
	// ties are then met from the latest hit.
	reversed := boss.HitsTaken().Reverse().PerAgent()
	if len(reversed) != 3 || reversed[0].Agent != p1.Agent || reversed[1].Agent != tl.Unknown || reversed[2].Agent != p2.Agent {
		t.Errorf("reversed PerAgent = %v", reversed)
	}
	if h := reversed[0].Hits; h.First().Src != p1.Agent || h.Last().Src != pet || h.Between(At(1200*msec)).Count() != 1 {
		t.Errorf("reversed share hits = %v", h.All())
	}
	if pet.Hits().First().Credited() != p1.Agent || p1.Hits().First().Credited() != p1.Agent || tl.Unknown.Hits().First().Credited() != tl.Unknown {
		t.Error("Credited is wrong")
	}
	checkInvariants(t, tl)
}

func TestRankingsAndCredited(t *testing.T) {
	b := fixture()
	b.castStart(900, addrP1, addrBoss, skillSlam, 500, 500)
	b.castStop(1000, addrP1, skillSlam, 100, evtc.ActivationReset)
	b.castStart(1500, addrP1, addrBoss, skillHeat, 500, 500)
	b.castStop(1600, addrP1, skillHeat, 100, evtc.ActivationReset)
	b.castStart(2000, addrP1, addrBoss, skillSlam, 500, 500)
	b.castStop(2100, addrP1, skillSlam, 100, evtc.ActivationReset)
	b.hit(1000, addrP1, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	b.hit(1100, addrP2, addrBoss, skillHeat, 300, evtc.ResultStrikeDamageNormal)
	b.minionHit(1200, addrPet, addrBoss, instP1, skillHeat, 250)
	b.hit(1300, 0, addrBoss, skillSlam, 300, evtc.ResultStrikeDamageNormal)
	b.hit(1400, addrBoss, addrP1, skillSlam, 500, evtc.ResultStrikeDamageNormal)
	b.minionHit(1500, addrPet, addrAdd, instP1, skillHeat, 50)
	tl := mustBuild(t, b.build(10000))
	p1, p2, boss, pet, add := tl.Players[0], tl.Players[1], tl.Targets[0], tl.Agent(addrPet), tl.Agent(addrAdd)

	credited := p1.HitsCredited()
	if credited.Count() != 3 || credited.First().Time != time.Second || credited.Last().Time != 1500*msec || credited.On(boss).Damage() != 350 || p1.Hits().Count() != 1 {
		t.Errorf("HitsCredited = %v", credited.All())
	}
	if p2.HitsCredited().Count() != 1 || pet.HitsCredited().Count() != 2 || tl.Unknown.HitsCredited().Count() != 1 || boss.HitsCredited().Count() != 1 {
		t.Error("HitsCredited of agents without minions differs from Hits")
	}

	skills := boss.HitsTaken().PerSkill()
	if len(skills) != 2 || skills[0].Skill != tl.Skill(skillHeat) || skills[0].Hits.Count() != 2 || skills[0].Hits.Damage() != 550 || skills[1].Skill != tl.Skill(skillSlam) || skills[1].Hits.Damage() != 400 {
		t.Errorf("PerSkill = %v", skills)
	}
	targets := tl.Hits().PerTarget()
	if len(targets) != 3 || targets[0].Agent != boss.Agent || targets[0].Hits.Damage() != 950 || targets[1].Agent != p1.Agent || targets[2].Agent != add || targets[2].Hits.Damage() != 50 {
		t.Errorf("PerTarget = %v", targets)
	}
	if r := tl.Hits().Reverse().PerTarget(); r[0].Hits.First().Time != time.Second || r[0].Hits.Between(At(1200*msec)).Count() != 1 {
		t.Errorf("reversed PerTarget = %v", r[0].Hits.All())
	}
	casts := p1.Casts().PerSkill()
	if len(casts) != 2 || casts[0].Skill != tl.Skill(skillSlam) || casts[0].Casts.Count() != 2 || casts[0].Casts.First().Interval.Start != 900*msec || casts[1].Skill != tl.Skill(skillHeat) || casts[1].Casts.Count() != 1 {
		t.Errorf("Casts.PerSkill = %v", casts)
	}
	if r := p1.Casts().Reverse().PerSkill(); r[0].Casts.First().Interval.Start != 900*msec {
		t.Error("reversed Casts.PerSkill is not in start order")
	}
	if p2.Casts().PerSkill() != nil || p2.HitsTaken().PerSkill() != nil || p2.HitsTaken().PerTarget() != nil {
		t.Error("rankings of nothing are not nil")
	}
	checkInvariants(t, tl)
}

func TestHitFilters(t *testing.T) {
	b := fixture()
	b.hit(1000, addrP1, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	b.hit(2000, addrP1, addrBoss, skillHeat, 100, evtc.ResultStrikeDamageNormal)
	b.hit(3000, addrP1, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(10000))

	p1 := tl.Players[0]
	if !p1.IsPlayer() || p1.IsNPC() || p1.IsGadget() {
		t.Error("kind predicates are wrong")
	}
	if p1.Hits().Between(NewInterval(1500*msec, 2500*msec)).Count() != 1 || p1.Hits().Between(NewInterval(0, 500*msec)).Count() != 0 {
		t.Error("Hits.Between is wrong")
	}
	if p1.Hits().Of(tl.Skill(skillSlam)).Count() != 2 || p1.Hits().Of(nil).Count() != 0 {
		t.Error("Hits.Of is wrong")
	}
	if tl.Buff(skillBuff) != nil {
		t.Error("an unused buff exists")
	}
}
