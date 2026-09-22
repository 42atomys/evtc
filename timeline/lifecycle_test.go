package timeline

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestStatesDownsDeaths(t *testing.T) {
	b := fixture()
	b.state(0, addrP1, evtc.StateEnterCombat)
	b.hit(2990, addrBoss, addrP1, skillHeat, 0, evtc.ResultDowned)
	b.state(3000, addrP1, evtc.StateChangeDown)
	b.state(4000, addrP1, evtc.StateChangeUp)
	b.state(8000, addrP1, evtc.StateChangeDown)
	b.hit(8000, 0, addrP1, evtc.SkillGenericDown, 0, evtc.ResultDowned)
	b.hit(8000, 0, addrP1, evtc.SkillGenericKill, 0, evtc.ResultKillingBlow)
	b.state(8000, addrP1, evtc.StateChangeDead)
	b.state(9000, addrP1, evtc.StateExitCombat)
	b.state(500, addrAdd, evtc.StateSpawn)
	b.state(6000, addrAdd, evtc.StateDespawn)
	b.hit(7000, addrBoss, addrP2, skillHeat, 0, evtc.ResultKillingBlow)
	b.state(7000, addrP2, evtc.StateChangeDead)
	tl := mustBuild(t, b.build(10000))

	p1, p2, boss := tl.characters[0], tl.characters[1], tl.targets[0]
	for _, tt := range []struct {
		at   time.Duration
		want LifeState
	}{{time.Second, LifeAlive}, {3500 * msec, LifeDown}, {5 * time.Second, LifeAlive}, {8 * time.Second, LifeDead}, {9 * time.Second, LifeDead}, {-time.Second, LifeUnknown}} {
		if got := p1.LifeStateAt(tt.at); got != tt.want {
			t.Errorf("LifeStateAt(%v) = %v, want %v", tt.at, got, tt.want)
		}
	}
	if !p1.IsDownAt(3500*msec) || !p1.IsAliveAt(5*time.Second) || !p1.IsDeadAt(9*time.Second) || p1.IsAliveAt(9*time.Second) {
		t.Error("state predicates are wrong")
	}
	if len(p1.Downs) != 2 || len(p1.Deaths) != 1 {
		t.Fatalf("downs %d deaths %d", len(p1.Downs), len(p1.Deaths))
	}
	d0 := p1.Downs[0]
	if d0.Interval != NewInterval(3*time.Second, 4*time.Second) || !d0.Recovered || d0.Death != nil || d0.Agent != p1.Agent || d0.Event.IsStateChange != evtc.StateChangeDown {
		t.Errorf("first down = %+v", d0)
	}
	if d0.Cause == nil || d0.Cause.Skill.ID != skillHeat || d0.Cause.Src != boss.Agent || d0.Cause.Down != d0 || d0.Cause.Death != nil {
		t.Errorf("first down cause = %+v", d0.Cause)
	}
	d1, de := p1.Downs[1], p1.Deaths[0]
	if d1.Interval != At(8*time.Second) || d1.Recovered || d1.Death != de || de.Down != d1 || de.Time != 8*time.Second || de.Agent != p1.Agent {
		t.Errorf("second down = %+v death = %+v", d1, de)
	}
	if d1.Cause == nil || d1.Cause.Skill.Name != "Down" || !d1.Cause.Skill.Custom || d1.Cause.Src != tl.Unknown {
		t.Errorf("second down cause = %+v", d1.Cause)
	}
	if de.Cause == nil || de.Cause.Skill.ID != evtc.SkillGenericKill || de.Cause.Death != de || !de.Cause.Killing() {
		t.Errorf("death cause = %+v", de.Cause)
	}
	if v, _ := p1.InCombat.ValueAt(5 * time.Second); !v {
		t.Error("InCombat at 5s is false")
	}
	if v, _ := p1.InCombat.ValueAt(9500 * msec); v {
		t.Error("InCombat at 9.5s is true")
	}
	inCombat := func(sp Span[bool]) bool { return sp.Value }
	if got := p1.InCombat.Total(tl.Interval(), inCombat); got != 9*time.Second {
		t.Errorf("time in combat = %v", got)
	}
	down := func(sp Span[LifeState]) bool { return sp.Value == LifeDown }
	if got := p1.Life.Total(tl.Interval(), down); got != time.Second {
		t.Errorf("time down = %v", got)
	}
	add := tl.Agent(addrAdd)
	if add.LifeStateAt(300*msec) != LifeUnknown || add.LifeStateAt(3*time.Second) != LifeAlive || add.LifeStateAt(7*time.Second) != LifeGone || add.Lifetime != NewInterval(500*msec, 6*time.Second) {
		t.Errorf("add states = %v lifetime %v", add.Life.All(), add.Lifetime)
	}
	if len(p2.Deaths) != 1 || p2.Deaths[0].Down != nil || p2.Deaths[0].Cause.Src != boss.Agent || len(p2.Downs) != 0 {
		t.Errorf("outright death = %+v", p2.Deaths)
	}
	if boss.LifeStateAt(time.Second) != LifeAlive || len(boss.Life.All()) != 1 {
		t.Errorf("boss states = %v", boss.Life.All())
	}
	if tl.Unknown.Life.Len() != 0 {
		t.Error("the Unknown sentinel has states")
	}
}

// TestDeathWrittenTwice checks a death written twice in the same
// millisecond, as arcdps does at times: each death gets its own killing
// blow, and a death left without one has no cause.
func TestDeathWrittenTwice(t *testing.T) {
	b := fixture()
	b.hit(5000, addrBoss, addrP1, skillSlam, 0, evtc.ResultKillingBlow)
	b.hit(5000, addrBoss, addrP1, skillHeat, 0, evtc.ResultKillingBlow)
	b.state(5000, addrP1, evtc.StateChangeDead)
	b.state(5000, addrP1, evtc.StateChangeDead)
	b.hit(7000, addrBoss, addrP2, skillSlam, 0, evtc.ResultKillingBlow)
	b.state(7000, addrP2, evtc.StateChangeDead)
	b.state(7000, addrP2, evtc.StateChangeDead)
	tl := mustBuild(t, b.build(10000))

	p1, p2 := tl.characters[0], tl.characters[1]
	if len(p1.Deaths) != 2 || len(p2.Deaths) != 2 {
		t.Fatalf("deaths = %d and %d, want 2 and 2", len(p1.Deaths), len(p2.Deaths))
	}
	first, second := p1.Deaths[0], p1.Deaths[1]
	if first.Cause == nil || second.Cause == nil || first.Cause == second.Cause || first.Cause.Death != first || second.Cause.Death != second {
		t.Errorf("causes of the two deaths = %+v and %+v", first.Cause, second.Cause)
	}
	if p2.Deaths[0].Cause == nil || p2.Deaths[0].Cause.Death != p2.Deaths[0] || p2.Deaths[1].Cause != nil {
		t.Errorf("causes with one killing blow = %+v and %+v", p2.Deaths[0].Cause, p2.Deaths[1].Cause)
	}
	checkInvariants(t, tl)
}

func TestBreakbar(t *testing.T) {
	b := fixture()
	b.defianceState(500, addrAdd, DefianceNone)
	b.defianceState(1000, addrAdd, DefianceActive)
	b.defiancePercent(1000, addrAdd, 1)
	b.defiancePercent(1500, addrAdd, 0.5)
	b.defiancePercent(2900, addrAdd, 0)
	b.hit(1500, addrP1, addrAdd, skillSlam, 100, evtc.ResultDefianceDamageNormal)
	b.hit(2000, 0, addrAdd, evtc.SkillDefianceDamage, -10, evtc.ResultDefianceDamageNormal)
	b.hit(2500, addrP2, addrAdd, skillHeat, 50, evtc.ResultDefianceDamageNormal)
	b.hit(2600, addrP2, addrAdd, skillHeat, 5, evtc.ResultCrowdControl)
	b.defianceState(3000, addrAdd, DefianceRecover)
	b.defianceState(6000, addrAdd, DefianceActive)
	// The arcdps README places the state in dst_agent instead.
	b.add(evtc.Event{Time: b.at(8000), SrcAgent: addrAdd, DstAgent: uint64(DefianceImmune), IsStateChange: evtc.StateDefianceBarState})
	b.move(9500, addrAdd, evtc.StatePosition, 0, 0, 0) // keeps the states of the add known until 9.5 s
	tl := mustBuild(t, b.build(10000))

	add := tl.Agent(addrAdd)
	if add.Target == nil || len(add.Breakbars) != 2 {
		t.Fatalf("breakbars = %v target %v", add.Breakbars, add.Target)
	}
	bb := add.Breakbars[0]
	if bb.Interval != NewInterval(time.Second, 3*time.Second) || !bb.Broken() || bb.Agent != add || len(bb.hits) != 3 {
		t.Errorf("breakbar = %+v", bb)
	}
	if bb.TotalCC() != 150 || bb.CC(tl.characters[0]) != 100 || bb.CC(tl.characters[1]) != 50 || bb.CC(tl.Unknown) != 0 {
		t.Errorf("CC = %d, p1 %d", bb.TotalCC(), bb.CC(tl.characters[0]))
	}
	if bb.Percent.Len() != 3 {
		t.Errorf("percent samples = %d", bb.Percent.Len())
	}
	if v, ok := bb.Percent.At(1200 * msec); !ok || v != 100 {
		t.Errorf("Percent.At = %v, %v", v, ok)
	}
	if v, _ := bb.Percent.Min(); v != 0 {
		t.Errorf("Percent.Min = %v", v)
	}
	if bb.hits[1].Src != tl.Unknown || bb.hits[1].Damage != -10 || !bb.hits[1].IsDefiance() || bb.hits[1].Skill.Name != "Defiance Damage" {
		t.Errorf("regeneration tick = %+v", bb.hits[1])
	}
	if add.Breakbars[1].End != DefianceImmune || add.Breakbars[1].Interval != NewInterval(6*time.Second, 8*time.Second) || add.Breakbars[1].Broken() {
		t.Errorf("second breakbar = %+v", add.Breakbars[1])
	}
	for _, tt := range []struct {
		at   time.Duration
		want DefianceState
	}{{700 * msec, DefianceNone}, {2 * time.Second, DefianceActive}, {4 * time.Second, DefianceRecover}, {7 * time.Second, DefianceActive}, {9 * time.Second, DefianceImmune}} {
		if got, ok := add.Defiance.ValueAt(tt.at); !ok || got != tt.want {
			t.Errorf("Defiance.ValueAt(%v) = %v, %v", tt.at, got, ok)
		}
	}
	if add.HitsTaken().Defiance().Count() != 3 || add.HitsTaken().Where((*Hit).IsCrowdControl).Count() != 1 {
		t.Error("defiance filters are wrong")
	}
}
