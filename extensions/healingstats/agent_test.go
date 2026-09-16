package healingstats

import (
	"fmt"
	"testing"

	"github.com/42atomys/evtc/timeline"
)

func TestAgentNode(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(1000, addrAlpha, addrBravo, skillHeal, 100, fromSrc)
	b.hit(1500, addrAlpha, addrBoss, skillSlam, 700)
	b.heal(2000, addrBravo, addrAlpha, skillHeal, 200, fromDst)
	s := mustBuild(t, b.build(10000))
	tl := s.Timeline
	alpha := s.Agent(tl.POV)
	if alpha == nil || alpha.Name != "Alpha" || alpha.Stats != s || alpha.Agent != tl.POV.Agent || alpha.Ref() != tl.POV.Agent || !alpha.IsPlayer() || alpha.Player != tl.POV || !alpha.Recorded {
		t.Fatalf("alpha = %+v", alpha)
	}
	// The node is an entity of the timeline.
	if tl.Hits().By(alpha).Count() != 1 || tl.Hits().By(alpha).First().Dst != tl.Boss().Agent || tl.Events().Involving(alpha).Count() == 0 {
		t.Errorf("the node is not accepted as an entity: %d hits", tl.Hits().By(alpha).Count())
	}
	if alpha.Heals().Count() != 1 || alpha.HealsTaken().Count() != 1 || alpha.HealsTaken().First().Src.Name != "Bravo" || alpha.HealsCredited().Count() != 1 || alpha.Heals().First().Dst.Player.Account != "Bravo.5678" {
		t.Errorf("alpha heals %d taken %d credited %d", alpha.Heals().Count(), alpha.HealsTaken().Count(), alpha.HealsCredited().Count())
	}
	if boss := s.Agent(tl.Boss()); boss == nil || boss.Heals().Count() != 0 || boss.HealsTaken().Count() != 0 || boss.HealsCredited().Count() != 0 || boss.Recorded || boss.Target != tl.Boss() {
		t.Errorf("boss = %+v", boss)
	}
	if s.Agent(nil) != nil || s.Agent((*timeline.Player)(nil)) != nil || s.Agent((*Agent)(nil)) != nil || s.Agent(alpha) != alpha || s.Agent(alpha.Agent) != alpha || s.Agents().Count() != tl.Agents().Count() || s.Players().First() != alpha || s.Unknown.Agent != tl.Unknown || s.Agent(tl.Unknown) != s.Unknown {
		t.Error("lookups are inconsistent")
	}
	players := s.Players()
	bravo := s.Agent(tl.PlayerByName("Bravo"))
	if players.Where(func(a *Agent) bool { return a.Recorded }).Count() != len(s.Recorded) || players.Reverse().Limit(1).First() != players.Last() || players.Skip(1).First() != bravo {
		t.Errorf("player query: %v", players.All())
	}
	if groups := players.Reverse().GroupBy(func(a *Agent) bool { return a.Heals().Any() }); groups[true].First() != alpha || groups[true].Count() != 2 {
		t.Errorf("player groups = %v", groups)
	}
	other := mustBuild(t, filterLog())
	if s.Agent(other.Players().First()) != nil || other.Agent(alpha) != nil {
		t.Error("an agent of another timeline has a node")
	}
	if got := fmt.Sprint(alpha); got != "Alpha(Player#0)" {
		t.Errorf("String = %q", got)
	}
	checkInvariants(t, s)
}
