package timeline

import (
	"testing"
	"time"
)

// TestQueryAllocations pins the allocation profile of the hot query paths:
// point lookups and lazy terminals must not allocate, filters allocate at
// most one closure, and only the terminals that return a collection
// allocate it.
func TestQueryAllocations(t *testing.T) {
	tl := mustBuild(t, genLog(genOptions{players: 8, adds: 6, duration: 60 * time.Second, seed: 5}))
	boss, p := tl.targets[0], tl.players[0]
	iv := NewInterval(10*time.Second, 20*time.Second)
	at := 15 * time.Second
	var sink int
	var sinkF float64
	var sinkV Vec3

	cases := []struct {
		name string
		max  float64
		fn   func()
	}{
		{"Series.At", 0, func() { v, _ := p.Position.At(at); sinkV = v }},
		{"Numbers.At", 0, func() { v, _ := boss.Health.At(at); sinkF = v }},
		{"Spans.ValueAt", 0, func() { s, _ := p.Life.ValueAt(at); sink = int(s) }},
		{"LifeStateAt", 0, func() { sink = int(p.LifeStateAt(at)) }},
		{"AgentAt", 0, func() { tl.AgentAt(p.InstanceID, at) }},
		{"Count", 0, func() { sink = tl.Hits().Count() }},
		{"Any", 0, func() { tl.Hits().Any() }},
		{"First", 0, func() { tl.Hits().First() }},
		{"Last", 0, func() { tl.Hits().Last() }},
		{"Hits.Between.Count", 0, func() { sink = tl.Hits().Between(iv).Count() }},
		{"Hits.Blocked.Count", 0, func() { sink = boss.Hits().Blocked().Count() }},
		{"Hits.Damage", 0, func() { sink = int(boss.HitsTaken().Damage()) }},
		{"Seq", 0, func() {
			for range tl.Hits().Seq() {
				sink++
			}
		}},
		{"Series.Between", 0, func() { sink = boss.Health.Between(iv).Len() }},
		{"Spans.Between", 0, func() { sink = p.Life.Between(iv).Len() }},
		{"Hits.By.Count", 1, func() { sink = tl.Hits().By(p).Count() }},
		{"Casts.Between.Count", 1, func() { sink = boss.Casts().Between(iv).Count() }},
		{"Stacks.CountAt", 1, func() { sink = p.Stacks().CountAt(at) }},
		{"chained query", 6, func() { boss.Casts().OfSkill(1200).Between(iv).Hits().On(p).Blocked().Any() }},
		{"PositionAt", 0, func() { sinkV = p.PositionAt(at) }},
		{"HealthAt", 0, func() { sinkF = boss.HealthAt(at) }},
		{"DistanceTo", 0, func() { sinkF = p.DistanceTo(boss, at) }},
		{"IsInCombatAt", 0, func() { p.IsInCombatAt(at) }},
		{"Effects.Count", 0, func() { sink = tl.Effects().Count() }},
		{"Missiles.By.Count", 1, func() { sink = tl.Missiles().By(p).Count() }},
		{"Commander", 0, func() { tl.Commander() }},
		{"CommanderAt", 0, func() { tl.CommanderAt(at) }},
		{"IsCommanderAt", 0, func() { p.IsCommanderAt(at) }},
		{"SquadMarkerAt", 0, func() { sink = int(p.SquadMarkerAt(at)) }},
		{"GroundMarkerAt", 0, func() { tl.GroundMarkerAt(SquadArrow, at) }},
		{"Ping.At", 0, func() { tl.Ping.At(at) }},
		{"WeaponSetAt", 0, func() { p.WeaponSetAt(at) }},
		{"PingAt", 0, func() { tl.PingAt(at) }},
		{"Hits.Foes.Count", 0, func() { sink = p.Hits().Foes().Count() }},
		{"Stacks.RemovedBy.Count", 1, func() { sink = tl.Stacks().RemovedBy(p).Count() }},
		{"Stacks.EffectiveAt", 3, func() { sink = p.Stacks().EffectiveAt(at) }},
		{"Numbers.TimeBelow", 0, func() { boss.Health.TimeBelow(50, iv) }},
		{"Numbers.MinBetween", 0, func() { sinkF, _ = boss.Health.MinBetween(iv) }},
		{"HitsCredited.Count", 0, func() { sink = p.HitsCredited().Count() }},
		{"CombatTime", 0, func() { p.CombatTime(iv) }},
		{"AliveTime", 0, func() { p.AliveTime(iv) }},
		{"Stacks.Average", 1, func() { sinkF = p.Stacks().Average(iv) }},
		{"Reverse.First", 0, func() { tl.Hits().Reverse().First() }},
		{"Limit.Count", 0, func() { sink = tl.Hits().Limit(3).Count() }},
		{"Skip.Limit.Any", 0, func() { tl.Hits().Skip(5).Limit(3).Any() }},
		{"Blocked.Reverse.Limit.Count", 0, func() { sink = boss.Hits().Blocked().Reverse().Limit(3).Count() }},
		{"Hits.DPS", 0, func() { sinkF = boss.HitsTaken().DPS(iv) }},
		{"Hits.Barrier", 0, func() { sink = int(boss.HitsTaken().Barrier()) }},
		{"Stacks.Reverse.Uptime", 1, func() { p.Stacks().Reverse().Uptime(iv) }},
		{"DefianceStateAt", 0, func() { sink = int(boss.DefianceStateAt(at)) }},
		{"HealthAbove", 0, func() { boss.HealthAbove(50) }},
		{"HealthBelow", 0, func() { boss.HealthBelow(50) }},
		{"DownedBetween", 0, func() { p.DownedBetween(iv) }},
		{"Boss", 0, func() { tl.Boss() }},
		{"TargetBySpeciesIDAt", 0, func() { tl.TargetBySpeciesIDAt(boss.SpeciesID, at) }},
		{"PlayerByAccount", 0, func() { tl.PlayerByAccount(p.Account) }},
		{"Since", 0, func() { tl.Since(at) }},
		{"IsAirborneAt", 0, func() { p.IsAirborneAt(at) }},
		{"IsNameVisibleAt", 0, func() { tl.Gadgets().First().IsNameVisibleAt(at) }},
		{"ExtensionEvents.Count", 1, func() { sink = tl.ExtensionEvents().Count() }},
		{"Extension.Events.Count", 0, func() { sink = tl.Extensions[0].Events().Count() }},
		{"Extension", 0, func() { tl.Extension(0x9c9b3c99) }},
		{"ExtensionOf", 0, func() { tl.ExtensionOf(tl.Extensions[0].Events().First()) }},
		{"Events.By.Count", 2, func() { sink = tl.Extensions[0].Events().By(p).Count() }},
		{"NameVisibleTime", 0, func() { tl.Gadgets().First().NameVisibleTime(iv) }},
		{"Cast.Hits.Count", 0, func() {
			if c := boss.Casts().First(); c != nil {
				sink = c.Hits().Count()
			}
		}},
		{"All", 1, func() { sink = len(tl.Hits().Between(iv).All()) }},
	}
	for _, c := range cases {
		c.fn()
		got := testing.AllocsPerRun(50, c.fn)
		if got > c.max {
			t.Errorf("%s: %v allocations per run, want at most %v", c.name, got, c.max)
		} else {
			t.Logf("%s: %v allocations", c.name, got)
		}
	}
	_, _, _ = sink, sinkF, sinkV
}
