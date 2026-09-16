package healingstats

import (
	"testing"
	"time"

	"github.com/42atomys/evtc/timeline"
)

// TestQueryAllocations pins the allocation profile of the query paths:
// point lookups and lazy terminals must not allocate, filters allocate at
// most one closure, and only the terminals that return a collection
// allocate it.
func TestQueryAllocations(t *testing.T) {
	s := mustBuild(t, genLog(5, 120))
	tl := s.Timeline
	p := tl.Players().First()
	a := s.Agent(p)
	iv := timeline.NewInterval(10*time.Second, 20*time.Second)
	var sink int
	var sinkF float64

	cases := []struct {
		name string
		max  float64
		fn   func()
	}{
		{"Count", 0, func() { sink = s.Heals().Count() }},
		{"Any", 0, func() { s.Heals().Any() }},
		{"First", 0, func() { s.Heals().First() }},
		{"Last", 0, func() { s.Heals().Last() }},
		{"Reverse.First", 0, func() { s.Heals().Reverse().First() }},
		{"Between.Count", 0, func() { sink = s.Heals().Between(iv).Count() }},
		{"Healing.Count", 0, func() { sink = s.Heals().Healing().Count() }},
		{"Barrier.Count", 0, func() { sink = s.Heals().Barrier().Count() }},
		{"Ticks.Downed.Count", 1, func() { sink = s.Heals().Ticks().Downed().Count() }},
		{"Amount", 0, func() { sink = int(s.Heals().Amount()) }},
		{"Healed", 0, func() { sink = int(s.Heals().Healed()) }},
		{"BarrierGiven", 0, func() { sink = int(s.Heals().BarrierGiven()) }},
		{"HPS", 0, func() { sinkF = s.Heals().HPS(iv) }},
		{"BPS", 0, func() { sinkF = s.Heals().BPS(iv) }},
		{"Agent", 0, func() { s.Agent(p) }},
		{"Agent.Heals.Count", 0, func() { sink = a.Heals().Count() }},
		{"Agent.HealsTaken.Healed", 0, func() { sink = int(a.HealsTaken().Healed()) }},
		{"Agent.HealsCredited.Count", 0, func() { sink = a.HealsCredited().Count() }},
		{"By.Count", 1, func() { sink = s.Heals().By(p).Count() }},
		{"CreditedTo.Count", 1, func() { sink = s.Heals().CreditedTo(a).Count() }},
		{"On.Count", 1, func() { sink = s.Heals().On(p).Count() }},
		{"OfSkill.Count", 1, func() { sink = s.Heals().OfSkill(skillHeal).Count() }},
		{"Limit.Count", 0, func() { sink = s.Heals().Limit(3).Count() }},
		{"Skip.Limit.Any", 0, func() { s.Heals().Skip(5).Limit(3).Any() }},
		{"Seq", 0, func() {
			for range s.Heals().Seq() {
				sink++
			}
		}},
		{"chained", 4, func() { s.Heals().Between(iv).On(p).Healing().Others().Amount() }},
		{"All", 1, func() { sink = len(s.Heals().Between(iv).All()) }},
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
	_, _ = sink, sinkF
}
