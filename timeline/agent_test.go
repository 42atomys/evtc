package timeline

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestAgentStates(t *testing.T) {
	b := fixture()
	b.weaponSwap(2000, addrP1, 0, 1)
	b.weaponSwap(5000, addrP1, 1, 0)
	b.stealth(1000, addrP1, 1)
	b.stealth(2000, addrP1, 0)
	b.stealth(3000, addrP1, 2)
	b.glider(1000, addrP2, true)
	b.glider(1500, addrP2, false)
	b.transformation(1000, addrP1, 999, 5000)
	b.transformation(4000, addrP1, 0, 0)
	b.stunBreak(2500, addrP1, 700)
	b.weaponSwap(100, 0, 0, 1)
	b.stealth(100, 0, 1)
	b.stunBreak(100, 0, 10)
	tl := mustBuild(t, b.build(10000))
	p1, p2 := tl.Players[0], tl.Players[1]

	if p1.WeaponSet.Len() != 3 || held(p1.WeaponSet.ValueAt(time.Second)) != 0 || held(p1.WeaponSet.ValueAt(3*time.Second)) != 1 || held(p1.WeaponSet.ValueAt(7*time.Second)) != 0 || p2.WeaponSet.Len() != 0 {
		t.Errorf("weapon sets = %v", p1.WeaponSet.All())
	}
	if first, _ := p1.WeaponSet.First(); first.Start != p1.Lifetime.Start || first.Event != nil {
		t.Errorf("the first weapon set span = %+v", first)
	}
	if p1.Stealth.Len() != 3 || held(p1.Stealth.ValueAt(1500*msec)) != 1 || held(p1.Stealth.ValueAt(2500*msec)) != 0 || held(p1.Stealth.ValueAt(3500*msec)) != 2 {
		t.Errorf("stealth = %v", p1.Stealth.All())
	}
	if p2.Gliding.Len() != 2 || !held(p2.Gliding.ValueAt(1200*msec)) || held(p2.Gliding.ValueAt(2*time.Second)) || p1.Gliding.Len() != 0 {
		t.Errorf("gliding = %v", p2.Gliding.All())
	}
	if p1.Transformation.Len() != 2 || held(p1.Transformation.ValueAt(2*time.Second)) != tl.Skill(999) || held(p1.Transformation.ValueAt(5*time.Second)) != nil || tl.Skill(999).Name != "999" {
		t.Errorf("transformation = %v", p1.Transformation.All())
	}
	if len(p1.StunBreaks) != 1 || p1.StunBreaks[0].Remaining != 700*msec || p1.StunBreaks[0].Agent != p1.Agent || p1.StunBreaks[0].Time != 2500*msec {
		t.Errorf("stun breaks = %+v", p1.StunBreaks)
	}
	if u := tl.Unknown; u.WeaponSet.Len() != 0 || u.Stealth.Len() != 0 || len(u.StunBreaks) != 0 {
		t.Error("the sentinel tracked agent states")
	}
	if p1.WeaponSetAt(3*time.Second) != 1 || p2.WeaponSetAt(3*time.Second) != 0 || p1.StealthAt(1500*msec) != 1 || p2.StealthAt(time.Second) != 0 || p1.TransformationAt(2*time.Second) != tl.Skill(999) || p1.TransformationAt(5*time.Second) != nil || p2.TransformationAt(time.Second) != nil {
		t.Error("point helpers on the agent states are wrong")
	}
	if !p2.IsGlidingAt(1200*msec) || p2.IsGlidingAt(2*time.Second) || p2.GlidingTime(tl.Interval()) != 500*msec || p1.GlidingTime(tl.Interval()) != 0 {
		t.Error("gliding helpers are wrong")
	}
	checkInvariants(t, tl)
}

func TestGadgetsAndJumps(t *testing.T) {
	b := fixture()
	b.gadgetName(1000, addrGad, 1)
	b.gadgetName(2000, addrGad, 2) // unsupported reads as hidden
	b.gadgetName(3000, addrGad, 0)
	b.gadgetAnimation(1500, addrGad, 272061484)
	b.gadgetAnimation(2500, addrGad, 463)
	b.gadgetAnimation(2600, addrBoss, 463)
	b.jump(1000, addrP1, true)
	b.jump(1400, addrP1, false)
	b.jump(4000, addrP1, true)
	b.move(4800, addrP1, evtc.StatePosition, 1, 2, 3)
	b.gadgetAnimation(100, 0, 1) // noise from an unknown source
	b.jump(100, 0, true)
	b.gadgetName(100, 0, 1)
	tl := mustBuild(t, b.build(5000))
	gad, p1, p2, boss := tl.Agent(addrGad), tl.Players[0], tl.Players[1], tl.Boss()

	if gad.NameVisible.Len() != 3 || !gad.IsNameVisibleAt(1500*msec) || gad.IsNameVisibleAt(2500*msec) || gad.IsNameVisibleAt(3500*msec) || gad.IsNameVisibleAt(500*msec) || boss.IsNameVisibleAt(1500*msec) {
		t.Errorf("name visibility = %v", gad.NameVisible.All())
	}
	if sp, ok := gad.NameVisible.At(2500 * msec); !ok || sp.Value || sp.Event.DstAgent != 2 {
		t.Errorf("unsupported state = %+v", sp)
	}
	if len(gad.GadgetAnimations) != 2 || gad.GadgetAnimations[0].Token != 272061484 || gad.GadgetAnimations[0].Time != 1500*msec || gad.GadgetAnimations[0].Agent != gad || gad.GadgetAnimations[1].Event == nil || len(boss.GadgetAnimations) != 1 || boss.GadgetAnimations[0].Agent != boss.Agent {
		t.Errorf("animations = %+v / %+v", gad.GadgetAnimations, boss.GadgetAnimations)
	}
	if p1.Airborne.Len() != 3 || !p1.IsAirborneAt(1200*msec) || p1.IsAirborneAt(1400*msec) || !p1.IsAirborneAt(4500*msec) || p1.IsAirborneAt(500*msec) || p1.IsAirborneAt(4900*msec) || p2.IsAirborneAt(1200*msec) {
		t.Errorf("airborne = %v", p1.Airborne.All())
	}
	if p1.AirborneTime(tl.Interval()) != 1200*msec || p1.AirborneTime(NewInterval(0, 1200*msec)) != 200*msec || p2.AirborneTime(tl.Interval()) != 0 {
		t.Errorf("airborne time = %v", p1.AirborneTime(tl.Interval()))
	}
	if gad.NameVisibleTime(tl.Interval()) != time.Second || gad.NameVisibleTime(NewInterval(1500*msec, 5*time.Second)) != 500*msec || p1.NameVisibleTime(tl.Interval()) != 0 {
		t.Errorf("name visible time = %v", gad.NameVisibleTime(tl.Interval()))
	}
	if u := tl.Unknown; u.Airborne.Len() != 0 || u.NameVisible.Len() != 0 || len(u.GadgetAnimations) != 0 {
		t.Errorf("unknown source was tracked: %v %v %v", u.Airborne.All(), u.NameVisible.All(), u.GadgetAnimations)
	}
	checkInvariants(t, tl)
}

func TestStatesEndWithLifetime(t *testing.T) {
	b := fixture()
	b.gadgetName(1000, addrGad, 1)
	b.state(2000, addrGad, evtc.StateDespawn)
	b.glider(1000, addrP1, true)
	b.teamChange(1000, addrP1, 5, 0)
	b.move(3000, addrP1, evtc.StatePosition, 1, 2, 3)
	tl := mustBuild(t, b.build(5000))
	gad, p1 := tl.Agent(addrGad), tl.Players[0]

	if !gad.IsNameVisibleAt(1500*msec) || gad.IsNameVisibleAt(2500*msec) || gad.NameVisibleTime(tl.Interval()) != time.Second || gad.LifeStateAt(2500*msec) != LifeGone {
		t.Errorf("gadget name visibility after despawn: %v, life %v", gad.NameVisible.All(), gad.Life.All())
	}
	if !p1.IsGlidingAt(2500*msec) || p1.IsGlidingAt(3500*msec) || p1.GlidingTime(tl.Interval()) != 2*time.Second || p1.TeamAt(2500*msec) != 5 || p1.TeamAt(3500*msec) != 0 || p1.LifeStateAt(3500*msec) != LifeAlive || p1.IsInCombatAt(3500*msec) {
		t.Errorf("player states after its last event: gliding %v, team %v", p1.Gliding.All(), p1.Team.All())
	}
	if last, ok := p1.Gliding.Last(); !ok || last.End != 3*time.Second || last.Start != time.Second {
		t.Errorf("last gliding span = %+v", last)
	}
	checkInvariants(t, tl)
}

func TestPointHelpers(t *testing.T) {
	b := fixture()
	b.state(0, addrP1, evtc.StateEnterCombat)
	b.move(1000, addrP1, evtc.StatePosition, 0, 0, 0)
	b.move(2000, addrP1, evtc.StatePosition, 100, 0, 0)
	b.move(1000, addrP1, evtc.StateVelocity, 1, 2, 3)
	b.facing(1000, addrP1, 0, 1)
	b.state(4000, addrP1, evtc.StateExitCombat)
	b.move(9000, addrP1, evtc.StatePosition, 0, 500, 0)
	b.move(1000, addrBoss, evtc.StatePosition, 0, 300, 0)
	b.move(2000, addrBoss, evtc.StatePosition, 0, 300, 0)
	b.health(1000, addrBoss, 80)
	b.maxHealth(1000, addrBoss, 1000000)
	b.barrier(1500, addrBoss, 5)
	b.defianceState(1000, addrBoss, DefianceActive)
	b.defiancePercent(1200, addrBoss, 0.5)
	b.targetable(1000, addrBoss, true)
	b.targetable(3000, addrBoss, false)
	tl := mustBuild(t, b.build(10000))
	p1, p2, boss := tl.Players[0], tl.Players[1], tl.Targets[0]
	at := 1500 * msec

	if p1.PositionAt(at) != (Vec3{50, 0, 0}) || p1.PositionAt(3*time.Second) != (Vec3{100, 0, 0}) {
		t.Errorf("PositionAt = %v, %v", p1.PositionAt(at), p1.PositionAt(3*time.Second))
	}
	if p1.PositionAt(500*msec) != (Vec3{}) || p1.PositionAt(9500*msec) != (Vec3{}) || p2.PositionAt(at) != (Vec3{}) {
		t.Error("an unknown position is not zero")
	}
	if p1.VelocityAt(at) != (Vec3{1, 2, 3}) || p1.FacingAt(at) != (Vec2{0, 1}) || p2.VelocityAt(at) != (Vec3{}) || p2.FacingAt(at) != (Vec2{}) {
		t.Errorf("VelocityAt = %v FacingAt = %v", p1.VelocityAt(at), p1.FacingAt(at))
	}
	if boss.HealthAt(at) != 80 || boss.HealthAt(500*msec) != 80 || boss.HealthAt(100*msec) != 0 || p1.HealthAt(at) != 0 {
		t.Errorf("HealthAt = %v, %v, %v", boss.HealthAt(at), boss.HealthAt(500*msec), boss.HealthAt(100*msec))
	}
	if boss.BarrierAt(2*time.Second) != 5 || boss.BarrierAt(-time.Second) != 0 || boss.DefiancePercentAt(at) != 50 || boss.DefiancePercentAt(-time.Second) != 0 {
		t.Errorf("BarrierAt = %v DefiancePercentAt = %v", boss.BarrierAt(2*time.Second), boss.DefiancePercentAt(at))
	}
	if boss.DefianceStateAt(at) != DefianceActive || boss.DefianceStateAt(500*msec) != DefianceNone || p1.DefianceStateAt(at) != DefianceNone {
		t.Errorf("DefianceStateAt = %v, %v", boss.DefianceStateAt(at), boss.DefianceStateAt(500*msec))
	}
	if boss.MaxHealthAt(at) != 1000000 || boss.MaxHealthAt(-time.Second) != 0 || p1.MaxHealthAt(at) != 0 {
		t.Errorf("MaxHealthAt = %v", boss.MaxHealthAt(at))
	}
	// A gap wider than MoveGap means the agent stood still: the position
	// is held, not interpolated and not unknown.
	if p1.PositionAt(6*time.Second) != (Vec3{100, 0, 0}) || p1.PositionAt(8*time.Second) != (Vec3{100, 0, 0}) || p1.PositionAt(9*time.Second) != (Vec3{0, 500, 0}) {
		t.Errorf("PositionAt across a gap = %v, %v", p1.PositionAt(6*time.Second), p1.PositionAt(8*time.Second))
	}

	if d := p1.DistanceTo(boss, at); math.Abs(d-math.Sqrt(92500)) > 1e-9 {
		t.Errorf("DistanceTo = %v", d)
	}
	if d := boss.DistanceTo(p1, at); math.Abs(d-math.Sqrt(92500)) > 1e-9 {
		t.Errorf("reverse DistanceTo = %v", d)
	}
	var none *Player
	for name, d := range map[string]float64{
		"nil":              p1.DistanceTo(nil, at),
		"nil player":       p1.DistanceTo(none, at),
		"unknown self":     p1.DistanceTo(boss, 500*msec),
		"unknown other":    p1.DistanceTo(p2, at),
		"outside all":      p1.DistanceTo(boss, 20*time.Second),
		"unknown sentinel": p1.DistanceTo(tl.Unknown, at),
	} {
		if !math.IsNaN(d) {
			t.Errorf("DistanceTo %s = %v, want NaN", name, d)
		}
	}

	if !p1.IsInCombatAt(2*time.Second) || p1.IsInCombatAt(5*time.Second) || boss.IsInCombatAt(2*time.Second) {
		t.Error("IsInCombatAt is wrong")
	}
	if p1.CombatTime(tl.Interval()) != 4*time.Second || p1.CombatTime(NewInterval(3*time.Second, 6*time.Second)) != time.Second || boss.CombatTime(tl.Interval()) != 0 {
		t.Errorf("CombatTime = %v", p1.CombatTime(tl.Interval()))
	}
	if !boss.IsTargetableAt(2*time.Second) || boss.IsTargetableAt(3500*msec) || p1.IsTargetableAt(2*time.Second) {
		t.Error("IsTargetableAt is wrong")
	}
	checkInvariants(t, tl)
}

func TestHealthHelpers(t *testing.T) {
	b := fixture()
	b.health(1000, addrBoss, 100)
	b.health(2000, addrBoss, 70)
	b.health(3000, addrBoss, 50)
	b.health(4000, addrBoss, 80)
	b.health(5000, addrBoss, 20)
	b.health(2000, addrP1, 50)
	tl := mustBuild(t, b.build(10000))
	boss, p1, p2 := tl.Targets[0], tl.Players[0], tl.Players[1]

	got := boss.HealthCrossings(66.6, 33.3)
	want := []struct {
		at    time.Duration
		level float64
		dir   Direction
	}{{3 * time.Second, 66.6, Falling}, {4 * time.Second, 66.6, Rising}, {5 * time.Second, 66.6, Falling}, {5 * time.Second, 33.3, Falling}}
	if len(got) != len(want) {
		t.Fatalf("crossings = %v", got)
	}
	for i, w := range want {
		if c := got[i]; c.Time != w.at || c.Level != w.level || c.Direction != w.dir {
			t.Errorf("crossing %d = %+v, want %+v", i, c, w)
		}
	}
	if at, ok := boss.HealthBelow(66.6); !ok || at != 3*time.Second {
		t.Errorf("HealthBelow(66.6) = %v, %v", at, ok)
	}
	if at, ok := boss.HealthBelow(33.3); !ok || at != 5*time.Second {
		t.Errorf("HealthBelow(33.3) = %v, %v", at, ok)
	}
	if _, ok := boss.HealthBelow(10); ok {
		t.Error("HealthBelow found a level never reached")
	}
	if at, ok := boss.HealthAbove(75); !ok || at != boss.Lifetime.Start {
		t.Errorf("HealthAbove(75) = %v, %v, want the held first sample", at, ok)
	}
	if at, ok := boss.Health.Between(NewInterval(2500*msec, 6*time.Second)).FirstAbove(75); !ok || at != 4*time.Second {
		t.Errorf("FirstAbove(75) on the sub-series = %v, %v", at, ok)
	}
	if _, ok := boss.HealthAbove(101); ok {
		t.Error("HealthAbove found a level never reached")
	}
	// A first sample already below the level: the value is held from the
	// start of the lifetime.
	if at, ok := p1.HealthBelow(60); !ok || at != p1.Lifetime.Start {
		t.Errorf("HealthBelow on a held first sample = %v, %v, lifetime %v", at, ok, p1.Lifetime)
	}
	if _, ok := p2.HealthBelow(60); ok || len(p2.HealthCrossings(50)) != 0 {
		t.Error("HealthBelow without samples succeeded")
	}
	// Without a backwards hold the first sample keeps its own time.
	n := NewNumbers([]Sample[int]{{Time: time.Second, Value: 50}, {Time: 2 * time.Second, Value: 10}})
	if at, ok := n.FirstBelow(60); !ok || at != time.Second {
		t.Errorf("FirstBelow = %v, %v", at, ok)
	}
	if at, ok := n.FirstBelow(20); !ok || at != 2*time.Second {
		t.Errorf("FirstBelow = %v, %v", at, ok)
	}
	if at, ok := n.FirstAbove(50); !ok || at != time.Second {
		t.Errorf("FirstAbove = %v, %v", at, ok)
	}
	if _, ok := n.FirstAbove(51); ok {
		t.Error("FirstAbove found a level never reached")
	}
	checkInvariants(t, tl)
}

func TestLifeHelpers(t *testing.T) {
	b := fixture()
	b.hit(2990, addrBoss, addrP1, skillHeat, 0, evtc.ResultDowned)
	b.state(3000, addrP1, evtc.StateChangeDown)
	b.state(4000, addrP1, evtc.StateChangeUp)
	b.hit(8000, addrBoss, addrP1, skillSlam, 0, evtc.ResultDowned)
	b.state(8000, addrP1, evtc.StateChangeDown)
	b.hit(8000, addrBoss, addrP1, skillSlam, 0, evtc.ResultKillingBlow)
	b.state(8000, addrP1, evtc.StateChangeDead)
	tl := mustBuild(t, b.build(10000))
	p1, p2 := tl.Players[0], tl.Players[1]
	heat, slam := tl.Skill(skillHeat), tl.Skill(skillSlam)
	if len(p1.Downs) != 2 || len(p1.Deaths) != 1 {
		t.Fatalf("downs %d deaths %d", len(p1.Downs), len(p1.Deaths))
	}

	if p1.DiedBefore(7999*msec) || !p1.DiedBefore(8*time.Second) || !p1.DiedBefore(9*time.Second) || p2.DiedBefore(10*time.Second) {
		t.Error("DiedBefore is wrong")
	}
	if !p1.DiedBetween(NewInterval(7*time.Second, 9*time.Second)) || p1.DiedBetween(NewInterval(0, 7*time.Second)) || p2.DiedBetween(tl.Interval()) {
		t.Error("DiedBetween is wrong")
	}
	if p1.AliveTime(tl.Interval()) != 7*time.Second || p1.DownTime(tl.Interval()) != time.Second || p1.AliveTime(NewInterval(3500*msec, 4500*msec)) != 500*msec || p2.AliveTime(tl.Interval()) != 0 || p2.DownTime(tl.Interval()) != 0 {
		t.Errorf("AliveTime = %v DownTime = %v", p1.AliveTime(tl.Interval()), p1.DownTime(tl.Interval()))
	}
	if at, ok := p1.DiedAt(); !ok || at != 8*time.Second {
		t.Errorf("DiedAt = %v, %v", at, ok)
	}
	if _, ok := p2.DiedAt(); ok {
		t.Error("DiedAt of a survivor succeeded")
	}
	if !p1.DownedBetween(NewInterval(3500*msec, 3600*msec)) || !p1.DownedBetween(NewInterval(0, 3*time.Second)) || p1.DownedBetween(NewInterval(4001*msec, 7999*msec)) || p2.DownedBetween(tl.Interval()) {
		t.Error("DownedBetween is wrong")
	}
	if d := p1.DownsOf(heat); len(d) != 1 || d[0] != p1.Downs[0] {
		t.Errorf("DownsOf(heat) = %v", d)
	}
	if d := p1.DownsOfSkill(skillSlam); len(d) != 1 || d[0] != p1.Downs[1] {
		t.Errorf("DownsOfSkill(slam) = %v", d)
	}
	if p1.DownsOf(nil) != nil || p1.DownsOfSkill(999) != nil || p2.DownsOf(heat) != nil {
		t.Error("DownsOf matched nothing but returned a slice")
	}
	if d := p1.DeathsOf(slam); len(d) != 1 || d[0] != p1.Deaths[0] {
		t.Errorf("DeathsOf(slam) = %v", d)
	}
	if d := p1.DeathsOfSkill(skillSlam); len(d) != 1 || d[0] != p1.Deaths[0] {
		t.Errorf("DeathsOfSkill(slam) = %v", d)
	}
	if p1.DeathsOf(nil) != nil || p1.DeathsOf(heat) != nil || p1.DeathsOfSkill(999) != nil {
		t.Error("DeathsOf matched nothing but returned a slice")
	}
	boss := tl.Targets[0]
	var none *Target
	if d := p1.DownsBy(boss); len(d) != 2 || d[0] != p1.Downs[0] || d[1] != p1.Downs[1] {
		t.Errorf("DownsBy(boss) = %v", d)
	}
	if p1.DownsBy(p2) != nil || p1.DownsBy(nil) != nil || p1.DownsBy(none) != nil || p2.DownsBy(boss) != nil {
		t.Error("DownsBy matched nothing but returned a slice")
	}
	if d := p1.DeathsBy(boss); len(d) != 1 || d[0] != p1.Deaths[0] || p1.DeathsBy(p2) != nil || p1.DeathsBy(nil) != nil {
		t.Errorf("DeathsBy(boss) = %v", d)
	}
	checkInvariants(t, tl)
}

func TestEnumNames(t *testing.T) {
	if KindPlayer.String() != "Player" || Kind(9).String() != "Kind(9)" {
		t.Error("Kind names are wrong")
	}
	if LifeGone.String() != "Gone" || LifeState(9).String() != "LifeState(9)" {
		t.Error("LifeState names are wrong")
	}
	if DefianceRecover.String() != "Recover" || DefianceState(9).String() != "DefianceState(9)" {
		t.Error("DefianceState names are wrong")
	}
}

func TestPhasesByHealth(t *testing.T) {
	b := fixture()
	b.health(1000, addrBoss, 100)
	b.health(2000, addrBoss, 70)
	b.health(3000, addrBoss, 50)
	b.health(4000, addrBoss, 80)
	b.health(5000, addrBoss, 20)
	b.hit(9000, addrP1, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	b.health(2000, addrP1, 50)
	tl := mustBuild(t, b.build(10000))
	boss, p1 := tl.Targets[0], tl.Players[0]
	life := boss.Lifetime
	if life != NewInterval(200*msec, 9*time.Second) {
		t.Fatalf("lifetime = %v", life)
	}

	want := []Interval{NewInterval(life.Start, 3*time.Second), NewInterval(3*time.Second, 5*time.Second), NewInterval(5*time.Second, life.End)}
	if got := boss.PhasesByHealth(66.6, 33.3); !slices.Equal(got, want) {
		t.Errorf("PhasesByHealth = %v, want %v", got, want)
	}
	if got := boss.PhasesByHealth(33.3, 66.6, 10); !slices.Equal(got, want) {
		t.Errorf("PhasesByHealth with unordered and unreached levels = %v", got)
	}
	if got := boss.PhasesByHealth(); len(got) != 1 || got[0] != life {
		t.Errorf("PhasesByHealth() = %v", got)
	}
	// A level the health was already below from the start adds no cut.
	if got := p1.PhasesByHealth(60); len(got) != 1 || got[0] != p1.Lifetime {
		t.Errorf("PhasesByHealth on a held first sample = %v", got)
	}
	checkInvariants(t, tl)
}

func TestPhasesByBuff(t *testing.T) {
	b := fixture()
	b.hit(500, addrP1, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	b.buffApply(1000, addrP1, addrBoss, skillBuff, 5000, 1)
	b.buffApply(2000, addrP1, addrBoss, skillBuff, 5000, 2)
	b.buffRemoveSingle(3000, addrBoss, 0, skillBuff, 0, 1, evtc.BuffRemoveSingle)
	b.buffRemoveSingle(4000, addrBoss, 0, skillBuff, 0, 2, evtc.BuffRemoveSingle)
	b.buffApply(6000, addrP1, addrBoss, skillBuff, 1000, 3)
	b.buffRemoveSingle(7000, addrBoss, 0, skillBuff, 0, 3, evtc.BuffRemoveSingle)
	b.hit(9000, addrP1, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	// Player one carries a never removed buff from its first event, and
	// another one that wears off.
	b.buffApply(0, addrP2, addrP1, skillBurn, 100000, 4)
	b.buffApply(0, addrP2, addrP1, skillBuff, 2000, 5)
	b.buffRemoveSingle(2000, addrP1, 0, skillBuff, 0, 5, evtc.BuffRemoveSingle)
	tl := mustBuild(t, b.build(10000))
	boss, p1 := tl.Targets[0], tl.Players[0]

	want := []Interval{NewInterval(200*msec, time.Second), NewInterval(4*time.Second, 6*time.Second), NewInterval(7*time.Second, 9*time.Second)}
	if got := boss.PhasesByBuff(skillBuff); !slices.Equal(got, want) {
		t.Errorf("PhasesByBuff = %v, want %v", got, want)
	}
	if got := boss.PhasesByBuff(skillBurn); len(got) != 1 || got[0] != boss.Lifetime {
		t.Errorf("PhasesByBuff of an absent buff = %v", got)
	}
	if got := p1.PhasesByBuff(skillBurn); got != nil {
		t.Errorf("PhasesByBuff of a permanent buff = %v", got)
	}
	if got := p1.PhasesByBuff(skillBuff); len(got) != 1 || got[0] != NewInterval(2*time.Second, p1.Lifetime.End) {
		t.Errorf("PhasesByBuff of a buff present from the start = %v", got)
	}
	checkInvariants(t, tl)
}

func TestMinions(t *testing.T) {
	b := fixture()
	b.hit(1000, addrP1, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	b.minionHit(1500, addrPet, addrBoss, instP1, skillHeat, 50)
	b.hit(2000, addrP1, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	b.minionHit(2100, addrPet, addrBoss, instP1, skillHeat, 25)
	tl := mustBuild(t, b.build(10000))

	p1, pet := tl.Players[0], tl.Agent(addrPet)
	if pet.Master != p1.Agent || len(p1.Minions) != 1 || p1.Minions[0] != pet || len(pet.Minions) != 0 {
		t.Errorf("pet master = %v minions = %v", pet.Master, p1.Minions)
	}
	if tl.Hits().By(p1).Count() != 2 || tl.Hits().CreditedTo(p1).Count() != 4 || tl.Hits().CreditedTo(p1).Damage() != 275 {
		t.Error("credited hits are wrong")
	}
	if tl.AgentAt(instP1, 1500*msec) != p1.Agent || tl.AgentAt(instPet, 1500*msec) != pet {
		t.Error("AgentAt is wrong")
	}
}

func TestUnknownAgents(t *testing.T) {
	b := fixture()
	b.hit(1000, 0xabcd, addrP1, skillSlam, 5, evtc.ResultStrikeDamageNormal)
	b.hit(1100, 0, addrP1, skillSlam, 7, evtc.ResultStrikeDamageNormal)
	b.buffApply(1200, 0, addrP1, skillBuff, 1000, 1)
	tl := mustBuild(t, b.build(10000))

	if len(tl.Agents) != 8 {
		t.Fatalf("agents = %d", len(tl.Agents))
	}
	ghost := tl.Agents[7]
	if ghost.Kind != KindUnknown || ghost.Addr != 0xabcd || ghost.Name != "Unknown abcd" || ghost.Raw != nil || tl.Agent(0xabcd) != ghost {
		t.Errorf("ghost = %+v", ghost)
	}
	p1 := tl.Players[0]
	if p1.HitsTaken().First().Src != ghost || p1.HitsTaken().Last().Src != tl.Unknown || tl.Unknown.Hits().Count() != 1 || tl.Unknown.StacksApplied().Count() != 1 {
		t.Error("unknown sources are wrong")
	}
	if ghost.Lifetime != At(time.Second) || tl.Unknown.Lifetime != (Interval{}) {
		t.Errorf("lifetimes ghost %v unknown %v", ghost.Lifetime, tl.Unknown.Lifetime)
	}
}

func TestAttackTargets(t *testing.T) {
	b := fixture()
	b.attackTarget(300, addrAT, addrGad)
	b.targetable(300, addrAT, true)
	b.targetable(2000, addrAT, false)
	b.health(3500, addrAT, 100) // keeps the states of the attack target known until 3.5 s
	b.hit(1000, addrP1, addrAT, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(10000))

	at, gad := tl.Agent(addrAT), tl.Agent(addrGad)
	if at.Gadget != gad || len(gad.AttackTargets) != 1 || gad.AttackTargets[0] != at || at.Kind != KindGadget || !at.IsGadget() {
		t.Errorf("attack target = %+v", at)
	}
	if v, ok := at.Targetable.ValueAt(time.Second); !ok || !v {
		t.Error("Targetable at 1s is false")
	}
	if v, ok := at.Targetable.ValueAt(3 * time.Second); !ok || v {
		t.Error("Targetable at 3s is true")
	}
	if len(tl.Targets) != 2 || tl.Targets[1].Agent != at {
		t.Errorf("targets = %v", tl.Targets)
	}
}
