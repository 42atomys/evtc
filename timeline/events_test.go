package timeline

import (
	"testing"

	"github.com/42atomys/evtc"
)

func TestUnknownInvolvement(t *testing.T) {
	b := fixture()
	b.hit(1000, 0, addrP1, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	b.hit(2000, addrP1, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	// Tracking events without a source are noise: they must not give the
	// sentinel states, samples or spans.
	b.state(753, 0, evtc.StateDespawn)
	b.move(800, 0, evtc.StatePosition, 1, 2, 3)
	b.health(900, 0, 50)
	b.state(950, 0, evtc.StateEnterCombat)
	b.attackTarget(960, 0, addrGad)
	tl := mustBuild(t, b.build(10000))

	if tl.Events().Involving(tl.Unknown).Count() != 6 || tl.Unknown.Events().Count() != 6 {
		t.Errorf("unknown involvement = %d", tl.Events().Involving(tl.Unknown).Count())
	}
	u := tl.Unknown
	if u.Life.Len() != 0 || u.Position.Len() != 0 || u.Health.Len() != 0 || u.InCombat.Len() != 0 || u.Gadget != nil || len(tl.Agent(addrGad).AttackTargets) != 0 {
		t.Errorf("the sentinel tracked state: states %d positions %d", u.Life.Len(), u.Position.Len())
	}
	checkInvariants(t, tl)
}
