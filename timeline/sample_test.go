package timeline

import (
	"math"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

// samplePath is the real log used by the integration tests; they are
// skipped when it is absent.
const samplePath = "../tests_fixtures/sabetha-05-fd9b6f3a.zevtc"

// Players of the sample log, by their index in Timeline.Players.
const (
	sampleKilled = 3 // killed outright at 39.6s
	sampleDowned = 5 // target of the first heat cast, downed twice, dead at the end
	sampleMoving = 7 // hit by that heat cast, teleported 17 times
)

func loadSample(tb testing.TB) *Timeline {
	tb.Helper()
	if _, err := os.Stat(samplePath); err != nil {
		tb.Skipf("%s not available", samplePath)
	}
	tl, err := ParseFile(samplePath)
	if err != nil {
		tb.Fatal(err)
	}
	return tl
}

func TestSampleOverview(t *testing.T) {
	tl := loadSample(t)

	if tl.Build != 20260816 || tl.Duration != 319082*msec || tl.Start.Unix() != 1787685623 || tl.MapID != 1062 {
		t.Errorf("build %d duration %v start %v map %d", tl.Build, tl.Duration, tl.Start, tl.MapID)
	}
	if len(tl.Players) != 10 || tl.POV == nil || tl.POV != tl.Players[0] || tl.POV.Subgroup != 2 {
		t.Errorf("players %d pov %v", len(tl.Players), tl.POV)
	}
	boss := tl.Targets[0]
	if !boss.Boss || boss.SpeciesID != 15375 || boss.Name != "Sabetha la saboteuse" || boss.Toughness != 1374 {
		t.Errorf("boss = %+v", boss.Agent)
	}
	if len(tl.Targets) < 10 || len(tl.NPCs()) != 260 || len(tl.Gadgets()) != 183 || len(tl.Agents) != 453 {
		t.Errorf("targets %d npcs %d gadgets %d agents %d", len(tl.Targets), len(tl.NPCs()), len(tl.Gadgets()), len(tl.Agents))
	}
	if tl.Hits().Count() != 15442 || tl.Stacks().Count() != 38258+407 || tl.Casts().Count() < 5495 {
		t.Errorf("hits %d stacks %d casts %d", tl.Hits().Count(), tl.Stacks().Count(), tl.Casts().Count())
	}
	if len(tl.Buffs) < 300 || tl.Buff(740) == nil || tl.Buff(740).StackLimit != 25 || tl.Skill(740).Buff != tl.Buff(740) {
		t.Errorf("buffs %d might %+v", len(tl.Buffs), tl.Buff(740))
	}
	if s := tl.Skill(evtc.SkillGenericKill); s == nil || s.Name != "Kill" || !s.Custom {
		t.Errorf("custom skill = %+v", s)
	}
	if v, ok := boss.MaxHealth.At(time.Second); !ok || v != 34015256 {
		t.Errorf("boss max health = %v, %v", v, ok)
	}
	minions, attackTargets := 0, 0
	for _, a := range tl.Agents {
		switch {
		case a.Master == nil:
		case a.Master.Player != nil:
			minions++
			if !slices.Contains(a.Master.Minions, a) {
				t.Errorf("%v is missing from the minions of %v", a, a.Master)
			}
		case a.Master.Kind == KindGadget && a.Gadget == a.Master:
			attackTargets++
		default:
			t.Errorf("%v has an unexpected master %v", a, a.Master)
		}
	}
	if minions < 5 || attackTargets != 4 {
		t.Errorf("minions %d attack targets %d", minions, attackTargets)
	}
	agents906 := 0
	for _, a := range tl.Agents {
		if a.InstanceID == 906 {
			agents906++
		}
	}
	if agents906 != 11 || tl.AgentAt(906, 34700*msec) == nil || tl.AgentAt(906, 34700*msec).Name != "Corneille" {
		t.Errorf("instance 906: %d agents, at 34.7s %v", agents906, tl.AgentAt(906, 34700*msec))
	}
}

func TestSampleHealthCrossings(t *testing.T) {
	tl := loadSample(t)
	boss := tl.Targets[0]

	cs := boss.Health.Crossings(66.6, 33.3)
	var down66, down33 *Crossing[float64]
	for i := range cs {
		c := &cs[i]
		if c.Direction != Falling {
			continue
		}
		switch {
		case c.Level == 66.6 && down66 == nil:
			down66 = c
		case c.Level == 33.3 && down33 == nil:
			down33 = c
		}
	}
	if down66 == nil || down66.Time != 99992*msec || down66.From.Value != 66.68 || down66.To.Value != 66.44 {
		t.Errorf("66.6%% crossing = %+v", down66)
	}
	if down33 == nil || down33.Time != 214892*msec {
		t.Errorf("33.3%% crossing = %+v", down33)
	}
	if v, ok := boss.Health.At(0); !ok || v != 100 {
		t.Errorf("boss health at 0 = %v, %v", v, ok)
	}
	if v, _ := boss.Health.Min(); v != 10.68 {
		t.Errorf("boss health min = %v", v)
	}
}

func TestSampleCastsAndHits(t *testing.T) {
	tl := loadSample(t)
	boss := tl.Targets[0]
	target, other := tl.Players[sampleDowned], tl.Players[sampleMoving]

	casts := boss.Casts().OfSkill(31390).Between(NewInterval(6*time.Second, 7*time.Second))
	if casts.Count() != 1 {
		t.Fatalf("heat casts between 6s and 7s = %d", casts.Count())
	}
	c := casts.First()
	if c.Target != target.Agent || c.Interval.Start != 6129*msec || c.Interval.End != 7548*msec || !c.Completed() || c.Skill.Name != "Chaleur" {
		t.Errorf("cast = %+v", c)
	}
	if casts.Hits().Blocked().Count() != 4 || !casts.Hits().On(target).Blocked().Any() || casts.Hits().On(other).Landed().Damage() != 2620 {
		t.Errorf("cast hits = %v", casts.Hits().All())
	}
	if len(c.hits) < 5 || c.hits[0].Cast != c {
		t.Errorf("cast hit edges = %v", c.hits)
	}
	if boss.Hits().Count() != 571 || boss.HitsTaken().Count() != 8162 {
		t.Errorf("boss hits %d taken %d", boss.Hits().Count(), boss.HitsTaken().Count())
	}
	if boss.HitsTaken().Landed().Damage() <= 0 || boss.HitsTaken().Absorbed().Count() != 217 {
		t.Errorf("boss damage taken = %d absorbed %d", boss.HitsTaken().Landed().Damage(), boss.HitsTaken().Absorbed().Count())
	}
	byPlayer := boss.HitsTaken().Strikes().GroupBy(func(h *Hit) *Agent { return h.Src })
	if len(byPlayer) == 0 {
		t.Error("no strikes on the boss")
	}
	for _, s := range tl.Skills {
		for _, c := range s.casts {
			if c.Skill != s {
				t.Fatalf("skill %v holds a cast of %v", s, c.Skill)
			}
		}
	}
}

func TestSampleDownsAndDeaths(t *testing.T) {
	tl := loadSample(t)
	downed, killed := tl.Players[sampleDowned], tl.Players[sampleKilled]

	if len(downed.Downs) != 2 || len(downed.Deaths) != 1 {
		t.Fatalf("downs %d deaths %d", len(downed.Downs), len(downed.Deaths))
	}
	d := downed.Downs[0]
	if d.Interval != NewInterval(192147*msec, 195038*msec) || !d.Recovered || d.Cause == nil || d.Cause.Src.Name != "Canon nord" || d.Cause.Skill.ID != 31643 || d.Cause.Down != d {
		t.Errorf("first down = %+v cause %+v", d, d.Cause)
	}
	if !downed.IsDownAt(193*time.Second) || !downed.IsAliveAt(200*time.Second) || !downed.IsDeadAt(316*time.Second) {
		t.Error("life states of the downed player are wrong")
	}
	de := downed.Deaths[0]
	if de.Time != 315023*msec || de.Down != downed.Downs[1] || de.Cause == nil || de.Cause.Src != tl.Unknown || de.Cause.Skill.Name != "Kill" {
		t.Errorf("death = %+v cause %+v", de, de.Cause)
	}
	if len(killed.Deaths) != 1 || killed.Deaths[0].Time != 39590*msec || killed.Deaths[0].Down != nil || killed.Deaths[0].Cause.Src != tl.Targets[0].Agent || len(killed.Downs) != 0 {
		t.Errorf("outright death = %+v", killed.Deaths)
	}
	if v, _ := downed.InCombat.ValueAt(100 * time.Second); !v {
		t.Error("the downed player was not in combat at 100s")
	}
}

func TestSamplePositions(t *testing.T) {
	tl := loadSample(t)
	p := tl.Players[sampleMoving]

	teleports := 0
	for s := range p.Position.Seq() {
		if s.Break {
			teleports++
		}
	}
	if p.Position.Len() != 767+17 || teleports != 17 {
		t.Errorf("positions = %d teleports %d", p.Position.Len(), teleports)
	}
	near := func(a, b Vec3) bool { return a.DistTo(b) < 0.5 }
	if pos, ok := p.Position.At(92 * msec); !ok || !near(pos, Vec3{-4354.1, 3957.1, -2464.3}) {
		t.Errorf("position at 92ms = %v, %v", pos, ok)
	}
	want := LerpVec3(Vec3{-4354.1, 3957.1, -2464.3}, Vec3{-4397.2, 3888.1, -2463.9}, 150.0/301)
	if pos, ok := p.Position.At(242 * msec); !ok || !near(pos, want) {
		t.Errorf("position at 242ms = %v, want %v", pos, want)
	}
	boss := tl.Targets[0]
	if boss.Facing.Len() != 229 || boss.Position.Len() != 13+11 {
		t.Errorf("boss facing %d positions %d", boss.Facing.Len(), boss.Position.Len())
	}
	if f, ok := boss.Facing.At(100 * time.Second); !ok || math.Abs(f.Len()-1) > 0.01 {
		t.Errorf("boss facing at 100s = %v, %v", f, ok)
	}
	bp, _ := boss.Position.At(100 * time.Second)
	pp, _ := p.Position.At(100 * time.Second)
	if d := bp.DistTo2D(pp); d <= 0 || d > 5000 {
		t.Errorf("distance to the boss at 100s = %v", d)
	}
}

func TestSampleBuffs(t *testing.T) {
	tl := loadSample(t)
	p := tl.Players[sampleMoving]

	closed := tl.Stacks().Where(func(s *BuffStack) bool {
		return s.Remove != nil && s.Remove.IsStateChange == evtc.StateBuffRemoveSingle
	}).Count()
	if closed != 37295 {
		t.Errorf("stacks closed by a single removal = %d, want 37295", closed)
	}
	might := p.Stacks().OfBuff(740)
	if might.Count() == 0 {
		t.Fatal("no might on the player")
	}
	if n := might.CountAt(100 * time.Second); n < 1 || n > 25 {
		t.Errorf("might stacks at 100s = %d", n)
	}
	if up := might.Uptime(NewInterval(10*time.Second, 300*time.Second)); up <= 0 || up > 290*time.Second {
		t.Errorf("might uptime = %v", up)
	}
	for s := range p.Stacks().Seq() {
		if s.Receiver != p.Agent || s.Buff == nil || s.Interval.End < s.Interval.Start {
			t.Fatalf("bad stack %+v", s)
		}
	}
	initial := tl.Stacks().Where(func(s *BuffStack) bool { return s.Initial }).Count()
	if initial != 407 {
		t.Errorf("initial stacks = %d", initial)
	}
}

func TestSampleBreakbars(t *testing.T) {
	tl := loadSample(t)
	var knuckles *Agent
	for _, a := range tl.NPCs() {
		if a.SpeciesID == 15404 {
			knuckles = a
		}
	}
	if knuckles == nil {
		t.Fatal("no Knuckles in the log")
	}
	if len(knuckles.Breakbars) == 0 {
		t.Fatalf("no breakbar on %v", knuckles)
	}
	bb := knuckles.Breakbars[0]
	if bb.Interval != NewInterval(144093*msec, 147093*msec) || bb.TotalCC() != 9500 || len(bb.hits) != 17 || !bb.Broken() || bb.End != DefianceImmune {
		t.Errorf("breakbar = %+v", bb)
	}
	if len(knuckles.Breakbars) != 2 || knuckles.Breakbars[1].Interval.Start != 162092*msec {
		t.Errorf("breakbars = %v", knuckles.Breakbars)
	}
	if cc := bb.CC(tl.POV); cc < 3000 {
		t.Errorf("CC of the recording player = %d", cc)
	}
	if v, ok := bb.Percent.Min(); !ok || v > 1 {
		t.Errorf("breakbar min percent = %v, %v", v, ok)
	}
}

func BenchmarkBuildSample(b *testing.B) {
	if _, err := os.Stat(samplePath); err != nil {
		b.Skipf("%s not available", samplePath)
	}
	l, err := evtc.ParseFile(samplePath)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Build(l); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQuerySample(b *testing.B) {
	tl := loadSample(b)
	boss := tl.Targets[0]
	target := tl.Players[sampleDowned]
	iv := NewInterval(6*time.Second, 7*time.Second)
	b.ReportAllocs()
	for b.Loop() {
		if !boss.Casts().OfSkill(31390).Between(iv).Hits().On(target).Blocked().Any() {
			b.Fatal("query failed")
		}
	}
}
