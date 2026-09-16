package timeline

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestEffects(t *testing.T) {
	b := fixture()
	g := GUID{0xEF, 0xFE}
	b.idToGUID(ContentEffect, 5000, g, 2000)
	b.groundEffect(1000, addrP1, 5000, 100, Vec3{100, 200, -300}, Vec3{0.5, -0.25, 1}, 0, 1500, true, 3)
	b.groundEffect(1500, addrBoss, 5001, 101, Vec3{10, 20, 30}, Vec3{}, 4000, 0, false, 0)
	b.effectRemove(2000, 0, evtc.StateEffectGroundRemove, 101)
	b.agentEffect(2500, addrP2, 5002, 102, 1000)
	b.agentEffect(3000, addrP2, 5002, 102, 0)
	b.effectRemove(4000, addrP2, evtc.StateEffectAgentRemove, 102)
	b.agentEffect(5000, 0, 5003, 103, 500)
	b.effectRemove(9000, 0, evtc.StateEffectGroundRemove, 999)
	tl := mustBuild(t, b.build(10000))
	p1, p2, boss := tl.players[0], tl.players[1], tl.Targets[0]
	all := tl.Effects().All()
	if len(all) != 5 {
		t.Fatalf("effects = %d", len(all))
	}
	f0, f1, f2, f3, f4 := all[0], all[1], all[2], all[3], all[4]
	if !f0.Ground || f0.Agent != p1.Agent || f0.EffectID != 5000 || f0.GUID != g || f0.Origin != (Vec3{100, 200, -300}) || f0.Orientation != (Vec3{0.5, -0.25, 1}) || f0.Scale != 1.5 || !f0.MovingPlatform || f0.Flags != 3 || f0.Duration != 2*time.Second || f0.Interval != NewInterval(time.Second, 3*time.Second) || f0.Removed() || f0.ID != 100 {
		t.Errorf("ground effect = %+v", f0)
	}
	if f1.Agent != boss.Agent || f1.Duration != 4*time.Second || f1.Interval != NewInterval(1500*msec, 2*time.Second) || !f1.Removed() || f1.Remove == nil || f1.Scale != 1 {
		t.Errorf("removed ground effect = %+v", f1)
	}
	if f2.Ground || f2.Agent != p2.Agent || f2.Duration != time.Second || f2.Interval != NewInterval(2500*msec, 3*time.Second) || f2.Removed() {
		t.Errorf("superseded agent effect = %+v", f2)
	}
	if f3.Interval != NewInterval(3*time.Second, 4*time.Second) || !f3.Removed() || f3.Duration != 0 {
		t.Errorf("removed agent effect = %+v", f3)
	}
	if f4.Agent != tl.Unknown || f4.Interval != NewInterval(5*time.Second, 5500*msec) || f4.GUID != (GUID{}) {
		t.Errorf("effect of an unknown source = %+v", f4)
	}
	q := tl.Effects()
	if p1.Effects().Count() != 1 || q.Ground().Count() != 2 || q.Around().Count() != 3 || q.OfID(5002).Count() != 2 || q.By(p2).Count() != 2 || q.By(nil).Count() != 0 || q.By(tl.Unknown).Count() != 1 {
		t.Error("effect filters are wrong")
	}
	if q.At(2900*msec).Count() != 2 || q.Between(NewInterval(3200*msec, 3800*msec)).Count() != 1 || q.Reverse().First() != f4 || q.Limit(2).Count() != 2 {
		t.Error("effect windows are wrong")
	}
	shares := q.PerID()
	if len(shares) != 4 || shares[0].EffectID != 5002 || shares[0].Effects.Count() != 2 || shares[1].EffectID != 5000 || shares[1].GUID != g || shares[3].EffectID != 5003 {
		t.Errorf("PerID = %v", shares)
	}
	if groups := q.GroupBy(func(f *Effect) bool { return f.Ground }); groups[true].Count() != 2 || groups[false].First() != f2 || p2.Effects().PerID() == nil || tl.players[1].Effects().Reverse().PerID()[0].Effects.First() != f2 {
		t.Error("effect grouping is wrong")
	}
	checkInvariants(t, tl)
}

func TestEffectWindows(t *testing.T) {
	b := fixture()
	b.agentEffect(1000, addrP1, 5000, 1, 0)
	b.agentEffect(2000, addrP1, 5001, 2, 0)
	tl := mustBuild(t, b.build(5000))
	if tl.Effects().Skip(1).First().EffectID != 5001 || tl.Effects().Skip(2).PerID() != nil || tl.players[1].Effects().PerID() != nil {
		t.Error("effect windows and empty rankings are wrong")
	}
}

func TestUntrackedEffects(t *testing.T) {
	b := fixture()
	b.idToGUID(ContentEffect, 5010, GUID{1}, 400)
	b.agentEffect(1000, addrP1, 5009, 0, 300)
	b.agentEffect(2000, addrP1, 5009, 0, 0)
	b.agentEffect(3000, addrP1, 5010, 0, 0)
	b.groundEffect(4000, addrP1, 5011, 0, Vec3{}, Vec3{}, 0xFFFFFFFF, 0, false, 0)
	b.effectRemove(4500, addrP1, evtc.StateEffectAgentRemove, 0)
	b.agentEffect(9900, addrP1, 5009, 0, 5000)
	tl := mustBuild(t, b.build(10000))
	all := tl.players[0].Effects().All()
	if len(all) != 5 {
		t.Fatalf("effects = %d", len(all))
	}
	for i, want := range []Interval{NewInterval(time.Second, 1300*msec), At(2 * time.Second), NewInterval(3*time.Second, 3400*msec), At(4 * time.Second), NewInterval(9900*msec, 10*time.Second)} {
		if f := all[i]; f.Interval != want || f.Removed() || f.ID != 0 {
			t.Errorf("untracked effect %d = %v removed %v, want %v", i, f.Interval, f.Removed(), want)
		}
	}
	if all[3].Duration != 0 || all[2].Duration != 400*msec {
		t.Errorf("durations = %v %v", all[3].Duration, all[2].Duration)
	}
	checkInvariants(t, tl)
}
