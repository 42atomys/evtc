package timeline

import (
	"testing"
	"time"
)

func TestMissiles(t *testing.T) {
	b := fixture()
	b.missileCreate(1000, addrP1, skillSlam, 200, Vec3{10, 20, 30}, 77)
	b.missileLaunch(1100, addrP1, addrBoss, 200, Vec3{40, 50, 60}, Vec3{10, 20, 30}, 2, 50, 0x11, true, 900)
	b.missileEffect(1150, addrP1, 200, 5000, 3000)
	b.missileLaunch(1200, addrP1, 0, 200, Vec3{70, 80, 90}, Vec3{20, 20, 20}, 0, 0, 0, false, 900)
	b.missileRemove(1500, addrP1, skillSlam, 200, 12, true, Vec3{40, 50, 60})
	b.missileCreate(2000, addrBoss, skillHeat, 201, Vec3{}, 0)
	b.missileCreate(2500, addrBoss, skillHeat, 201, Vec3{}, 0)
	b.missileLaunch(2600, addrBoss, addrP1, 201, Vec3{}, Vec3{}, 0, 0, 0, true, 0)
	b.missileCreate(3000, 0, skillHeat, 202, Vec3{}, 0)
	b.missileLaunch(9000, addrP1, 0, 999, Vec3{}, Vec3{}, 0, 0, 0, false, 0)
	b.missileEffect(9100, addrP1, 999, 5000, 1)
	b.missileRemove(9200, addrP1, skillSlam, 999, 0, false, Vec3{})
	tl := mustBuild(t, b.build(10000))
	p1, boss := tl.players[0], tl.Targets[0]
	all := tl.Missiles().All()
	if len(all) != 4 {
		t.Fatalf("missiles = %d", len(all))
	}
	m0, m1, m2, m3 := all[0], all[1], all[2], all[3]
	if m0.Owner != p1.Agent || m0.Skill != tl.Skill(skillSlam) || m0.Origin != (Vec3{10, 20, 30}) || m0.Skin != 77 || m0.Interval != NewInterval(time.Second, 1500*msec) || !m0.Removed() || m0.FriendlyFire != 12 || !m0.HitEnemy || m0.RemovedAt != (Vec3{40, 50, 60}) || m0.ID != 200 {
		t.Errorf("missile = %+v", m0)
	}
	if len(m0.Launches) != 2 || m0.Launches[0].Target != boss.Agent || m0.Launches[0].TargetPos != (Vec3{40, 50, 60}) || m0.Launches[0].Position != (Vec3{10, 20, 30}) || m0.Launches[0].Motion != 2 || m0.Launches[0].Radius != 50 || m0.Launches[0].Flags != 0x11 || !m0.Launches[0].First || m0.Launches[0].Speed != 900 || m0.Launches[1].Target != nil || m0.Launches[1].First || m0.Target() != boss.Agent {
		t.Errorf("launches = %+v", m0.Launches)
	}
	if len(m0.Effects) != 1 || m0.Effects[0].EffectID != 5000 || m0.Effects[0].Duration != 3*time.Second || m0.Effects[0].Time != 1150*msec {
		t.Errorf("missile effects = %+v", m0.Effects)
	}
	if m1.Owner != boss.Agent || m1.Interval != NewInterval(2*time.Second, 2500*msec) || m1.Removed() || len(m1.Launches) != 0 || m1.Target() != nil {
		t.Errorf("superseded missile = %+v", m1)
	}
	if m2.Interval != NewInterval(2500*msec, 10*time.Second) || len(m2.Launches) != 1 || m2.Launches[0].Target != p1.Agent || m3.Owner != tl.Unknown {
		t.Errorf("later missiles = %+v %+v", m2, m3)
	}
	q := tl.Missiles()
	if p1.Missiles().Count() != 1 || q.By(boss).Count() != 2 || q.By(nil).Count() != 0 || q.OfSkill(skillHeat).Count() != 3 || q.Of(tl.Skill(skillSlam)).Count() != 1 || q.At(1200*msec).Count() != 1 || q.Between(NewInterval(2400*msec, 2600*msec)).Count() != 2 {
		t.Error("missile filters are wrong")
	}
	if shares := q.PerSkill(); len(shares) != 2 || shares[0].Skill != tl.Skill(skillHeat) || shares[0].Missiles.Count() != 3 || shares[1].Missiles.First() != m0 || q.Reverse().Limit(1).First() != m3 || tl.players[1].Missiles().PerSkill() != nil {
		t.Error("missile rankings are wrong")
	}
	if groups := q.GroupBy(func(m *Missile) *Agent { return m.Owner }); len(groups) != 3 || groups[boss.Agent].Count() != 2 || q.Skip(3).First() != m3 {
		t.Error("missile grouping is wrong")
	}
	if tl.Skill(skillHeat).Missiles().Count() != 3 || tl.Skill(skillSlam).Missiles().First() != m0 || tl.Skill(skillBuff).Missiles().Count() != 0 {
		t.Error("Skill.Missiles is wrong")
	}
	checkInvariants(t, tl)
}
