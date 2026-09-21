package timeline

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

const msec = time.Millisecond

func TestBuildRejectsLegacyLogs(t *testing.T) {
	l := fixture().build(1000)
	l.Header.Build = "20260101"
	if _, err := Build(l); !errors.Is(err, ErrLegacyLog) {
		t.Errorf("err = %v, want ErrLegacyLog", err)
	}
	l.Header.Build = "garbage!"
	if _, err := Build(l); err == nil {
		t.Error("an invalid build date was accepted")
	}
	if _, err := Build(nil); err == nil {
		t.Error("a nil log was accepted")
	}
}

func TestBuildBasics(t *testing.T) {
	b := fixture()
	b.hit(1000, addrP1, addrBoss, skillSlam, 500, evtc.ResultStrikeDamageNormal)
	b.hit(1500, addrBoss, addrP2, skillHeat, 800, evtc.ResultStrikeDamageCrit)
	b.hit(1600, addrAdd, addrP1, skillHeat, 10, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(10000))

	if tl.Build != 20260816 || tl.Duration != 10*time.Second || tl.Interval() != NewInterval(0, 10*time.Second) {
		t.Errorf("build %d duration %v", tl.Build, tl.Duration)
	}
	if tl.Start.Unix() != 1787685623 || tl.LocalStart.Unix() != 1787685624 || tl.WallClock(2*time.Second).Unix() != 1787685625 {
		t.Errorf("start %v local %v", tl.Start, tl.LocalStart)
	}
	if len(tl.players) != 2 || tl.players[0].Account != "Alpha.1234" || tl.players[0].SubgroupAt(0) != 1 || tl.players[1].EliteSpecAt(0) != 65 || tl.characters[1].Profession != 2 {
		t.Errorf("players = %+v", tl.players)
	}
	if tl.POV != tl.characters[0].Player {
		t.Errorf("POV = %v", tl.POV)
	}
	if len(tl.targets) != 2 || !tl.targets[0].Boss || tl.targets[0].SpeciesID != 15375 || tl.targets[1].Name != "Crow" || tl.targets[1].Boss {
		t.Errorf("targets = %v", tl.targets)
	}
	if tl.NPCs().Count() != 3 || tl.Gadgets().Count() != 2 || len(tl.agents) != 7 {
		t.Errorf("npcs %d gadgets %d agents %d", tl.NPCs().Count(), tl.Gadgets().Count(), len(tl.agents))
	}
	boss := tl.targets[0]
	if tl.Agent(addrBoss) != boss.Agent || boss.Target != tl.targets[0] || tl.Agent(0xdead) != nil {
		t.Error("agent lookups are wrong")
	}
	if boss.Kind != KindNPC || !boss.IsNPC() || boss.Toughness != 100 || boss.InstanceID != instBoss {
		t.Errorf("boss = %+v", boss.Agent)
	}
	if boss.Lifetime != NewInterval(200*msec, 1500*msec) {
		t.Errorf("boss lifetime = %v", boss.Lifetime)
	}
	if tl.Hits().Count() != 3 || boss.Hits().Count() != 1 || boss.HitsTaken().Count() != 1 || tl.characters[0].HitsTaken().Count() != 1 {
		t.Error("hit edges are wrong")
	}
	h := tl.characters[0].Hits().First()
	if h.Dst != boss.Agent || h.Src != tl.characters[0].Agent || h.Skill.Name != "Slam" || h.Damage != 500 || !h.IsStrike() || h.IFF != evtc.IFFFoe || h.Time != time.Second {
		t.Errorf("hit = %+v", h)
	}
	if tl.Skill(skillSlam).Hits().First() != h || tl.Skill(999) != nil || tl.Buff(skillSlam) != nil {
		t.Error("skill lookups are wrong")
	}
	if tl.TimeOf(h.Event) != h.Time || tl.TimeOf(&evtc.Event{Time: rawEpoch - 500}) != -500*msec {
		t.Error("TimeOf is wrong")
	}
	if len(tl.Skills) != 4 || tl.Skills[0].ID != skillHeat || tl.Skills[3].ID != skillBuff {
		t.Errorf("skills = %v", tl.Skills)
	}
	if tl.Events().Count() != len(tl.events) || tl.Events().Of(evtc.StateCombat).Count() != 3 || boss.Events().Involving(tl.characters[1]).Count() != 1 {
		t.Error("event queries are wrong")
	}
	if got := tl.characters[0].Events().Between(NewInterval(900*msec, 1100*msec)).Count(); got != 1 {
		t.Errorf("events between = %d", got)
	}
	if tl.characters[0].String() != "Alpha(Player#0)" || boss.String() != "Sabetha(NPC#15375)" || (*Agent)(nil).String() != "<nil>" {
		t.Errorf("String = %q", boss.String())
	}
	if tl.Skill(skillSlam).String() != "Slam (200)" || (*Skill)(nil).String() != "<nil>" || (*Buff)(nil).String() != "<nil>" {
		t.Error("skill String is wrong")
	}
	if tl.Unknown.Kind != KindUnknown || tl.Unknown.Hits().Count() != 0 {
		t.Error("Unknown sentinel is wrong")
	}
}

func TestInstanceReuse(t *testing.T) {
	const addrAdd2 = 0x2004
	b := fixture()
	b.npc(addrAdd2, instAdd, 1083, "Crow 2")
	b.hit(1000, addrAdd, addrP1, skillHeat, 1, evtc.ResultStrikeDamageNormal)
	b.state(2000, addrAdd, evtc.StateDespawn)
	b.state(5000, addrAdd2, evtc.StateSpawn)
	b.hit(6000, addrAdd2, addrP1, skillHeat, 1, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(10000))

	add, add2 := tl.Agent(addrAdd), tl.Agent(addrAdd2)
	for _, tt := range []struct {
		at   time.Duration
		want *Agent
	}{{1500 * msec, add}, {5500 * msec, add2}, {2200 * msec, add}, {4800 * msec, add2}, {3500 * msec, nil}, {900 * msec, add}} {
		if got := tl.AgentAt(instAdd, tt.at); got != tt.want {
			t.Errorf("AgentAt(%d, %v) = %v, want %v", instAdd, tt.at, got, tt.want)
		}
	}
	if tl.AgentAt(999, time.Second) != nil {
		t.Error("an unknown instance id resolved")
	}
	if len(tl.targets) != 3 || tl.targets[1] != add.Target || tl.targets[2] != add2.Target {
		t.Errorf("targets = %v", tl.targets)
	}
}

func TestBossFallbackAndNoCombatStart(t *testing.T) {
	b := newLog()
	b.l.Events = nil // drop the squad combat start
	b.npc(addrBoss, instBoss, 15375, "Sabetha")
	b.player(addrP1, instP1, "Alpha", ":Alpha.1", "1", 1, 0)
	b.hit(5000, addrP1, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	b.hit(6000, addrP1, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.l)

	if tl.epoch != rawEpoch+5000 || tl.Duration != time.Second || !tl.Start.IsZero() {
		t.Errorf("epoch %d duration %v start %v", tl.epoch, tl.Duration, tl.Start)
	}
	if len(tl.targets) != 1 || !tl.targets[0].Boss || tl.targets[0].Name != "Sabetha" {
		t.Errorf("targets = %v", tl.targets)
	}
	if tl.Skill(skillSlam).Name != "200" || tl.Skill(skillSlam).Custom {
		t.Errorf("unnamed skill = %+v", tl.Skill(skillSlam))
	}
	if h := tl.Hits().First(); h.Time != 0 || tl.Hits().Last().Time != time.Second {
		t.Errorf("hit times = %v", tl.Hits().Map(hitTime))
	}

	empty := mustBuild(t, &evtc.Log{Header: evtc.Header{Build: "20260816", Revision: 1}})
	if len(empty.agents) != 0 || empty.Duration != 0 || empty.Hits().Count() != 0 || len(empty.targets) != 0 {
		t.Error("an empty log built a non-empty timeline")
	}
}

func TestNilEntities(t *testing.T) {
	b := fixture()
	b.hit(1000, addrP1, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	b.castStart(1000, addrP1, addrBoss, skillSlam, 100, 100)
	b.castStart(2000, addrP1, 0, skillHeat, 100, 100)
	b.buffApply(1000, addrP2, addrP1, skillBuff, 1000, 1)
	tl := mustBuild(t, b.build(10000))

	var player *Player
	var target *Target
	if tl.Hits().By(player).Any() || tl.Hits().On(target).Any() || tl.Hits().CreditedTo(player).Any() || tl.Hits().By(nil).Any() {
		t.Error("nil entities matched hits")
	}
	if tl.Casts().By(target).Any() || tl.Casts().On(player).Any() || tl.Casts().On(nil).Any() || tl.Stacks().On(player).Any() || tl.Stacks().By(target).Any() {
		t.Error("nil entities matched casts or stacks")
	}
	if tl.Events().Involving(player).Any() || player.Ref() != nil || target.Ref() != nil {
		t.Error("nil entities matched events")
	}
	if len(tl.targets[0].Breakbars) != 0 {
		t.Error("unexpected breakbars")
	}
	if tl.Hits().By(tl.characters[0]).Count() != 1 || tl.Hits().On(tl.targets[0]).Count() != 1 {
		t.Error("non-nil entities did not match")
	}
}

func TestEdgeQueries(t *testing.T) {
	b := fixture()
	b.castStart(1000, addrP1, addrBoss, skillSlam, 500, 500)
	b.hit(1200, addrP1, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageCrit)
	b.hit(1400, addrP1, addrBoss, skillSlam, 50, evtc.ResultStrikeDamageNormal)
	b.castStop(1500, addrP1, skillSlam, 500, evtc.ActivationReset)
	b.defianceState(2000, addrBoss, DefianceActive)
	b.hit(2200, addrP1, addrBoss, skillHeat, 100, evtc.ResultDefianceDamageNormal)
	b.hit(2400, 0, addrBoss, evtc.SkillDefianceDamage, -10, evtc.ResultDefianceDamageNormal)
	b.hit(2600, addrP2, addrBoss, skillHeat, 50, evtc.ResultDefianceDamageNormal)
	b.defianceState(3000, addrBoss, DefianceRecover)
	tl := mustBuild(t, b.build(10000))
	p1, boss := tl.characters[0], tl.targets[0]

	c := p1.Casts().First()
	if c == nil || c.Hits().Count() != 2 || c.Hits().Crits().Count() != 1 || c.Hits().Damage() != 150 || c.Hits().First().Cast != c {
		t.Errorf("cast hits = %v", c.Hits().All())
	}
	if p1.Casts().Hits().Count() != 2 || p1.Casts().Hits().Between(NewInterval(1300*msec, 2*time.Second)).Count() != 1 {
		t.Error("Casts.Hits is wrong")
	}
	if len(boss.Breakbars) != 1 {
		t.Fatalf("breakbars = %v", boss.Breakbars)
	}
	bb := boss.Breakbars[0]
	if bb.Hits().Count() != 3 || bb.Hits().By(p1).Damage() != 100 || bb.Hits().By(tl.Unknown).Damage() != -10 || bb.Hits().Damage() != 140 {
		t.Errorf("breakbar hits = %v", bb.Hits().All())
	}
	if bb.TotalCC() != 150 || bb.CC(tl.characters[1]) != 50 || bb.Hits().Between(NewInterval(2500*msec, 3*time.Second)).Count() != 1 {
		t.Errorf("CC = %d, p2 %d", bb.TotalCC(), bb.CC(tl.characters[1]))
	}
	if cc := bb.CCHits(); cc.Count() != 2 || cc.Damage() != 150 || cc.PerAgent()[0].Agent != p1.Agent || cc.PerAgent()[1].Hits.Damage() != 50 {
		t.Errorf("CCHits = %v", bb.CCHits().All())
	}
	checkInvariants(t, tl)
}

func TestTimelineLookups(t *testing.T) {
	const addrAdd2 = 0x2004
	b := fixture()
	b.npc(addrAdd2, 24, 1083, "Crow")
	b.hit(1000, addrP1, addrAdd, skillSlam, 10, evtc.ResultStrikeDamageNormal)
	b.hit(1500, addrP1, addrBoss, skillSlam, 10, evtc.ResultStrikeDamageNormal)
	b.hit(2000, addrP1, addrAdd, skillSlam, 10, evtc.ResultStrikeDamageNormal)
	b.hit(5000, addrP1, addrAdd2, skillSlam, 10, evtc.ResultStrikeDamageNormal)
	b.hit(6000, addrP1, addrAdd2, skillSlam, 10, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(10000))
	p1, p2 := tl.characters[0], tl.characters[1]
	add1, add2 := tl.Agent(addrAdd).Target, tl.Agent(addrAdd2).Target

	if boss := tl.Boss(); boss == nil || boss != tl.targets[0] || !boss.Boss || boss.SpeciesID != 15375 {
		t.Errorf("Boss = %v", tl.Boss())
	}
	if tl.TargetBySpeciesID(1083) != add1 || tl.TargetBySpeciesID(15375) != tl.Boss() || tl.TargetBySpeciesID(4242) != nil {
		t.Errorf("TargetBySpeciesID = %v", tl.TargetBySpeciesID(1083))
	}
	if all := tl.Targets().OfSpecies(1083).All(); len(all) != 2 || all[0] != add1 || all[1] != add2 || tl.Targets().OfSpecies(4242).Any() {
		t.Errorf("OfSpecies = %v", all)
	}
	for _, tt := range []struct {
		at   time.Duration
		want *Target
	}{
		{1500 * msec, add1}, {2 * time.Second, add1}, {2200 * msec, add1}, {800 * msec, add1},
		{3500 * msec, nil}, {4800 * msec, add2}, {5500 * msec, add2}, {6300 * msec, add2}, {7 * time.Second, nil},
	} {
		if got := tl.TargetBySpeciesIDAt(1083, tt.at); got != tt.want {
			t.Errorf("TargetBySpeciesIDAt(1083, %v) = %v, want %v", tt.at, got, tt.want)
		}
	}
	if tl.TargetBySpeciesIDAt(4242, time.Second) != nil {
		t.Error("TargetBySpeciesIDAt found an unknown species")
	}

	if tl.PlayerByAccount("Alpha.1234") != p1.Player || tl.PlayerByAccount(":Alpha.1234") != p1.Player || tl.PlayerByAccount("Bravo.5678") != p2.Player || tl.PlayerByAccount("Nobody.0000") != nil {
		t.Error("PlayerByAccount is wrong")
	}
	if tl.PlayerByName("Bravo") != p2.Player || tl.PlayerByName("Alpha") != p1.Player || tl.PlayerByName("Charlie") != nil {
		t.Error("PlayerByName is wrong")
	}
	if g := tl.Players().InSubgroup(1); g.Count() != 1 || g.First() != p1.Player || tl.Players().InSubgroup(2).Count() != 1 || tl.Players().InSubgroup(3).Any() {
		t.Errorf("InSubgroup(1) = %v", g.All())
	}
	if named := tl.Agents().Named("Crow").All(); len(named) != 2 || named[0] != add1.Agent || named[1] != add2.Agent || tl.Agents().Named("Nobody").Any() {
		t.Errorf("Named = %v", named)
	}
	agents := tl.Agents()
	if agents.OfKind(KindPlayer).Count() != 2 || agents.OfKind(KindGadget).Count() != 2 || agents.OfSpecies(1083).Count() != 2 || agents.OfSpecies(0).Count() != 1 || agents.OfSpecies(0).First().Name != "at3001-7307" {
		t.Errorf("agent filters: %d players, %d gadgets", agents.OfKind(KindPlayer).Count(), agents.OfKind(KindGadget).Count())
	}
	if agents.Reverse().First() != add2.Agent || agents.Skip(2).First() != tl.Boss().Agent || agents.Limit(3).Count() != 3 || agents.GroupBy((*Agent).IsNPC)[true].Count() != 4 {
		t.Errorf("agent traversal: last %v", agents.Reverse().First())
	}
	if agents.AliveAt(5500*msec).Where(func(a *Agent) bool { return a == add2.Agent }).Count() != 1 || agents.AliveAt(-time.Second).Any() {
		t.Error("Agents.AliveAt is wrong")
	}
	players := tl.Players()
	if players.OfProfession(ProfessionWarrior).First() != p2.Player || players.OfEliteSpec(65).First() != p2.Player || players.OfEliteSpec(EliteNone).First() != p1.Player || players.AliveAt(1500*msec).First() != p1.Player || players.AliveAt(-time.Second).Any() {
		t.Error("player filters are wrong")
	}
	if players.Reverse().First() != p2.Player || players.Skip(1).First() != p2.Player || players.Limit(1).Count() != 1 || players.GroupBy(func(p *Player) int { return p.SubgroupAt(0) })[2].First() != p2.Player {
		t.Error("player traversal is wrong")
	}
	targets := tl.Targets()
	if targets.Reverse().First() != add2 || targets.Skip(1).Limit(1).First() != add1 || targets.Reverse().GroupBy(func(tg *Target) uint16 { return tg.SpeciesID })[1083].First() != add1 {
		t.Error("target traversal is wrong")
	}
	if targets.AliveAt(5500*msec).Where(func(tg *Target) bool { return tg == add2 }).Count() != 1 || targets.AliveAt(-time.Second).Any() {
		t.Error("Targets.AliveAt is wrong")
	}

	for _, tt := range []struct{ got, want Interval }{
		{tl.Since(3 * time.Second), NewInterval(3*time.Second, 10*time.Second)},
		{tl.Since(-time.Second), NewInterval(0, 10*time.Second)},
		{tl.Since(12 * time.Second), At(10 * time.Second)},
		{tl.Until(3 * time.Second), NewInterval(0, 3*time.Second)},
		{tl.Until(-time.Second), At(0)},
		{tl.Until(12 * time.Second), NewInterval(0, 10*time.Second)},
	} {
		if tt.got != tt.want {
			t.Errorf("got %v, want %v", tt.got, tt.want)
		}
	}
	checkInvariants(t, tl)
}

func TestNoBoss(t *testing.T) {
	b := newLog()
	b.l.Header.TargetSpeciesID = 0
	b.player(addrP1, instP1, "Alpha", ":Alpha.1234", "1", 1, 0)
	b.npc(addrAdd, instAdd, 1083, "Crow")
	b.skill(skillSlam, "Slam")
	b.hit(1000, addrP1, addrAdd, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(2000))

	if tl.Boss() != nil || len(tl.targets) != 1 || tl.targets[0].Boss || tl.TargetBySpeciesID(1083) != tl.targets[0] {
		t.Errorf("Boss = %v targets %v", tl.Boss(), tl.targets)
	}
	checkInvariants(t, tl)
}

func TestParseFileErrors(t *testing.T) {
	if _, err := ParseFile(filepath.Join(t.TempDir(), "missing.zevtc")); err == nil {
		t.Error("a missing file was parsed")
	}
}
