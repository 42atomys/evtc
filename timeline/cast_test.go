package timeline

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestCastCompletion(t *testing.T) {
	b := fixture()
	for i, act := range []evtc.Activation{evtc.ActivationReset, evtc.ActivationMinimum, evtc.ActivationNoData, evtc.ActivationCancel} {
		start := uint64(1000 + i*1000)
		b.castStart(start, addrP1, addrBoss, skillSlam, 500, 500)
		b.castStop(start+400, addrP1, skillSlam, 400, act)
	}
	b.castStart(9000, addrP1, addrBoss, skillSlam, 500, 500)
	tl := mustBuild(t, b.build(10000))
	casts := tl.players[0].Casts().All()
	if len(casts) != 5 {
		t.Fatalf("casts = %d", len(casts))
	}
	for i, want := range []struct{ completed, full, cancelled bool }{{true, true, false}, {true, false, false}, {true, false, false}, {false, false, true}, {false, false, false}} {
		c := casts[i]
		if c.Completed() != want.completed || c.Full() != want.full || c.Cancelled() != want.cancelled {
			t.Errorf("cast %d (%v): completed %v full %v cancelled %v", i, c.Activation, c.Completed(), c.Full(), c.Cancelled())
		}
	}
	q := tl.players[0].Casts()
	if q.Completed().Count() != 3 || q.Full().Count() != 1 || q.Cancelled().Count() != 1 || q.Where((*Cast).Ended).Count() != 4 {
		t.Errorf("completed %d full %d cancelled %d", q.Completed().Count(), q.Full().Count(), q.Cancelled().Count())
	}
	checkInvariants(t, tl)
}

func TestCastsAndHits(t *testing.T) {
	b := fixture()
	b.castStart(2000, addrBoss, addrP1, skillHeat, 484, 716)
	b.hit(2500, addrBoss, addrP1, skillHeat, 0, evtc.ResultBlock)
	b.hit(2500, addrBoss, addrP2, skillHeat, 900, evtc.ResultStrikeDamageNormal)
	b.castStop(3400, addrBoss, skillHeat, 1400, evtc.ActivationReset)
	b.hit(5000, addrBoss, addrP2, skillHeat, 900, evtc.ResultStrikeDamageNormal)
	b.hit(5100, addrBoss, addrP2, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	b.castStart(6000, addrBoss, 0, skillHeat, 484, 716)
	b.hit(6300, addrBoss, addrP1, skillHeat, 0, evtc.ResultEvade)
	b.castStop(6500, addrBoss, skillHeat, 500, evtc.ActivationCancel)
	b.castStop(8000, addrP1, skillSlam, 300, evtc.ActivationMinimum)
	b.castStart(9000, addrP2, addrBoss, skillSlam, 100, 100)
	tl := mustBuild(t, b.build(10000))

	boss, p1, p2 := tl.targets[0], tl.players[0], tl.players[1]
	casts := boss.Casts()
	if casts.Count() != 2 || tl.Casts().Count() != 4 || tl.Skill(skillHeat).Casts().Count() != 2 {
		t.Fatalf("casts = %d, total %d", casts.Count(), tl.Casts().Count())
	}
	c := casts.First()
	if c.Target != p1.Agent || c.Caster != boss.Agent || c.Interval != NewInterval(2*time.Second, 3400*msec) || !c.Completed() || !c.Ended() {
		t.Errorf("cast = %+v", c)
	}
	if c.Expected != 484*msec || c.Control != 716*msec || c.Elapsed != 1400*msec || c.ElapsedUnscaled != 1400*msec || c.Duration() != 1400*msec {
		t.Errorf("cast timings = %+v", c)
	}
	if len(c.hits) != 3 || c.hits[0].Cast != c || c.hits[2].Time != 5*time.Second {
		t.Errorf("cast hits = %v", c.hits)
	}
	if !casts.OfSkill(skillHeat).Between(NewInterval(2*time.Second, 2600*msec)).Hits().On(p1).Blocked().Any() {
		t.Error("the blocked hit of the heat cast was not found")
	}
	if casts.OfSkill(skillHeat).Hits().Blocked().Count() != 1 || casts.Hits().Evaded().Count() != 1 || casts.Hits().Landed().Damage() != 1800 {
		t.Error("cast hit filters are wrong")
	}
	if boss.Hits().OfSkill(skillSlam).First().Cast != nil {
		t.Error("a hit without cast was attributed")
	}
	second := casts.Last()
	if second.Target != nil || !second.Cancelled() || len(second.hits) != 1 || second.hits[0].Result != evtc.ResultEvade {
		t.Errorf("second cast = %+v", second)
	}
	if casts.Between(NewInterval(3*time.Second, 5*time.Second)).Count() != 1 || casts.Between(NewInterval(4*time.Second, 5*time.Second)).Count() != 0 {
		t.Error("Between overlap semantics are wrong")
	}
	if casts.Completed().Count() != 1 || casts.Full().Count() != 1 || casts.Cancelled().Count() != 1 || casts.On(p1).Count() != 1 || casts.By(p1).Count() != 0 || casts.Of(tl.Skill(skillHeat)).Count() != 2 {
		t.Error("cast filters are wrong")
	}
	stopOnly := p1.Casts().First()
	if p1.Casts().Count() != 1 || stopOnly.Start != nil || stopOnly.Stop == nil || stopOnly.Interval != NewInterval(7700*msec, 8*time.Second) || !stopOnly.Completed() || stopOnly.Full() {
		t.Errorf("stop-only cast = %+v", stopOnly)
	}
	if all := tl.Casts().All(); all[2] != stopOnly || all[3].Caster != p2.Agent {
		t.Errorf("casts are not sorted by start: %v", all)
	}
	open := p2.Casts().First()
	if open.Ended() || open.Activation != evtc.ActivationNone || open.Interval != NewInterval(9*time.Second, 9100*msec) || open.Target != boss.Agent {
		t.Errorf("open cast = %+v", open)
	}
	if hits := tl.Casts().Hits().All(); len(hits) != 4 || hits[0].Time > hits[3].Time {
		t.Errorf("flattened hits = %v", hits)
	}
	groups := boss.Hits().GroupBy(func(h *Hit) *Agent { return h.Dst })
	if len(groups) != 2 || groups[p2.Agent].Count() != 3 {
		t.Errorf("GroupBy = %v", groups)
	}
	if cg := casts.GroupBy(func(c *Cast) bool { return c.Completed() }); len(cg) != 2 || cg[true].Count() != 1 {
		t.Errorf("cast GroupBy = %v", cg)
	}
	h := boss.Hits().First()
	if !h.Blocked() || h.Crit() || h.Glance() || h.Evaded() || h.Absorbed() || h.Missed() || h.Interrupted() || h.Downing() || h.Killing() || h.IsDefiance() || h.IsCrowdControl() || h.IsSignal() || h.IsBuffDamage() {
		t.Error("hit predicates are wrong")
	}
	if boss.Hits().WithResult(evtc.ResultBlock).Count() != 1 || boss.Hits().Strikes().Count() != 3 || boss.Hits().Crits().Count() != 0 || boss.Hits().Absorbed().Count() != 0 || boss.Hits().Missed().Count() != 0 {
		t.Error("hit query filters are wrong")
	}
}
