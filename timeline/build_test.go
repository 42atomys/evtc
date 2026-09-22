package timeline

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestStateByInstanceID(t *testing.T) {
	b := fixture()
	b.move(1000, addrPet, evtc.StatePosition, 1, 2, 3)
	b.stateByInst(2000, instPet, evtc.StateDespawn)
	// A second add reuses the instance id of the first.
	b.state(1000, addrAdd, evtc.StateSpawn)
	b.stateByInst(1500, instAdd, evtc.StateChangeDead)
	b.stateByInst(2000, instAdd, evtc.StateDespawn)
	b.npc(0x2999, instAdd, 1083, "Crow 2")
	b.state(3000, 0x2999, evtc.StateSpawn)
	b.stateByInst(4000, instAdd, evtc.StateDespawn)
	b.stateByInst(4500, 999, evtc.StateDespawn)                                                                                                   // an instance id never seen
	b.add(evtc.Event{Time: b.at(4600), SrcInstanceID: instPet, SkillID: skillHeat, DstAgent: addrP1, Value: 10, IsStateChange: evtc.StateCombat}) // hits are not resolved this way
	tl := mustBuild(t, b.build(5000))
	pet, add, add2 := tl.Agent(addrPet), tl.Agent(addrAdd), tl.Agent(0x2999)

	if pet.LifeStateAt(1500*msec) != LifeAlive || pet.LifeStateAt(2500*msec) != LifeGone || pet.Lifetime.End != 2*time.Second {
		t.Errorf("pet life = %v, lifetime %v", pet.Life.All(), pet.Lifetime)
	}
	if add.LifeStateAt(1200*msec) != LifeAlive || add.LifeStateAt(1700*msec) != LifeDead || add.LifeStateAt(2500*msec) != LifeGone || len(add.Deaths) != 1 || add.Lifetime.End != 2*time.Second {
		t.Errorf("add life = %v", add.Life.All())
	}
	if add2.LifeStateAt(3500*msec) != LifeAlive || add2.LifeStateAt(4200*msec) != LifeGone || add2.Lifetime != NewInterval(3*time.Second, 4*time.Second) || add.LifeStateAt(4200*msec) != LifeGone {
		t.Errorf("second add life = %v, lifetime %v", add2.Life.All(), add2.Lifetime)
	}
	if tl.Unknown.Life.Len() != 0 || tl.Unknown.Hits().Count() != 1 || pet.Hits().Count() != 0 {
		t.Errorf("unknown: life %v, hits %d, pet hits %d", tl.Unknown.Life.All(), tl.Unknown.Hits().Count(), pet.Hits().Count())
	}
	checkInvariants(t, tl)
}

func TestAddressChangeEdgeCases(t *testing.T) {
	// A change between two addresses absent from the table merges the
	// events under one synthesized agent.
	b := fixture()
	b.hit(1000, 0x9001, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	b.add(evtc.Event{Time: b.at(1500), SrcAgent: 0x9001, DstAgent: 0x9002, IsStateChange: evtc.StateIIDChange})
	b.hit(2000, 0x9002, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	// A change to the same address is ignored.
	b.add(evtc.Event{Time: b.at(2500), SrcAgent: addrP1, DstAgent: addrP1, IsStateChange: evtc.StateIIDChange})
	tl := mustBuild(t, b.build(10000))

	if len(tl.agents) != 8 {
		t.Fatalf("agents = %d", len(tl.agents))
	}
	ghost := tl.agents[7]
	if ghost.Hits().Count() != 2 || tl.Agent(0x9001) != ghost || tl.Agent(0x9002) != ghost || ghost.Addr != 0x9002 {
		t.Errorf("ghost = %+v hits %d", ghost, ghost.Hits().Count())
	}
	if tl.Agent(addrP1) != tl.characters[0].Agent {
		t.Error("a self alias broke the lookup")
	}
	checkInvariants(t, tl)
}

func TestCorruptTimes(t *testing.T) {
	b := fixture()
	b.hit(1000, addrP1, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	b.add(evtc.Event{Time: 0x3030303030303030, SrcAgent: addrP1, IsStateChange: evtc.StateChangeDead})
	b.add(evtc.Event{Time: 1, SrcAgent: addrP1, IsStateChange: evtc.StateChangeDown})
	b.add(evtc.Event{Time: rawEpoch + uint64(maxSpan/time.Millisecond), SrcAgent: addrP1, IsStateChange: evtc.StateChangeDown})
	tl := mustBuild(t, b.build(5000))

	p1 := tl.characters[0]
	if tl.Duration != maxSpan || len(p1.Deaths) != 0 || len(p1.Downs) != 2 || tl.Events().Count() != len(tl.Log.Events)-1 {
		t.Errorf("duration %v deaths %d downs %d events %d of %d", tl.Duration, len(p1.Deaths), len(p1.Downs), tl.Events().Count(), len(tl.Log.Events))
	}
	if p1.Downs[0].Start != -999*msec {
		t.Errorf("early down at %v", p1.Downs[0].Start)
	}
	checkInvariants(t, tl)

	// A log whose origin itself is corrupt keeps nothing but stays
	// consistent.
	b = fixture()
	b.l.Events[0].Time = 0x3030303030303030
	b.hit(1000, addrP1, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	tl = mustBuild(t, b.build(5000))
	if tl.Events().Count() != 1 || tl.Hits().Count() != 0 {
		t.Errorf("events %d hits %d", tl.Events().Count(), tl.Hits().Count())
	}
	checkInvariants(t, tl)
}

func TestEventsAfterCombatEnd(t *testing.T) {
	b := fixture()
	b.hit(1000, addrP1, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	l := b.build(2000)
	b.state(5000, addrP1, evtc.StateChangeDead)
	tl := mustBuild(t, l)
	if tl.Duration != 5*time.Second {
		t.Errorf("duration = %v, want the last event", tl.Duration)
	}
	checkInvariants(t, tl)
}

func TestBuffArenaOverflow(t *testing.T) {
	// The buff arena is sized from the BuffInfo events plus a small
	// headroom; buffs beyond it are allocated one by one.
	b := fixture()
	const n = 40
	for i := range n {
		id := uint32(5000 + i)
		b.skill(int32(id), "Buff")
		b.buffApply(uint64(1000+i), addrP2, addrP1, id, 1000, uint32(1+i))
	}
	tl := mustBuild(t, b.build(10000))
	if len(tl.Buffs) != n || tl.characters[0].Stacks().Count() != n || tl.Buff(5000+n-1) == nil || tl.Buff(5000+n-1).Skill.Buff != tl.Buff(5000+n-1) {
		t.Errorf("buffs = %d stacks = %d", len(tl.Buffs), tl.characters[0].Stacks().Count())
	}
	checkInvariants(t, tl)
}

func TestSentinelHasNoMaster(t *testing.T) {
	// A minion leaving tracking is logged with a zero source and its
	// master instance id; the sentinel must not adopt that master.
	b := fixture()
	b.add(evtc.Event{Time: b.at(700), SrcMasterInstanceID: instP1, IsStateChange: evtc.StateDespawn})
	b.add(evtc.Event{Time: b.at(800), DstAgent: addrP1, SrcMasterInstanceID: instP2, DstMasterInstanceID: instP2, SkillID: skillSlam, Value: 5, IFF: evtc.IFFFoe})
	b.hit(1000, 0, addrBoss, skillSlam, 7, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(10000))

	p1, p2 := tl.characters[0], tl.characters[1]
	if tl.Unknown.Master != nil || len(p1.Minions) != 0 || len(p2.Minions) != 0 || p1.Master != nil {
		t.Errorf("sentinel master = %v, minions of p1 %d, of p2 %d", tl.Unknown.Master, len(p1.Minions), len(p2.Minions))
	}
	if tl.Hits().CreditedTo(p2).Count() != 0 || tl.Unknown.Hits().First().Credited() != tl.Unknown || tl.Hits().PerAgent()[0].Agent != tl.Unknown {
		t.Error("unknown hits are credited to a player")
	}
	checkInvariants(t, tl)
}

func TestAddressChange(t *testing.T) {
	const old = 0x9999
	b := fixture()
	b.hit(1000, old, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	b.add(evtc.Event{Time: b.at(1500), SrcAgent: old, DstAgent: addrP1, IsStateChange: evtc.StateIIDChange})
	b.hit(2000, addrP1, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(10000))

	p1 := tl.characters[0]
	if p1.Hits().Count() != 2 || tl.Agent(old) != p1.Agent || len(tl.agents) != 7 {
		t.Errorf("hits %d agent %v agents %d", p1.Hits().Count(), tl.Agent(old), len(tl.agents))
	}
	if p1.Lifetime != NewInterval(0, 2*time.Second) {
		t.Errorf("lifetime = %v", p1.Lifetime)
	}
	// Point of view, the hit under the old address and the hit under the
	// new one; the address change event names no agent.
	if got, want := tl.Events().Involving(p1).Count(), p1.Events().Count(); got != want || got != 3 {
		t.Errorf("Involving = %d events, want %d", got, want)
	}

	// The table may hold the old address instead.
	b = fixture()
	b.add(evtc.Event{Time: b.at(500), SrcAgent: addrP1, DstAgent: old, IsStateChange: evtc.StateIIDChange})
	b.hit(1000, old, addrBoss, skillSlam, 100, evtc.ResultStrikeDamageNormal)
	tl = mustBuild(t, b.build(10000))
	if tl.characters[0].Hits().Count() != 1 || tl.Agent(old) != tl.characters[0].Agent {
		t.Error("an address change towards an unknown address was not aliased")
	}
}

// enterCombat appends the event a player announces their build with.
func (b *logBuilder) enterCombat(t, agent uint64, inst uint16, subgroup uint64, prof Profession, spec EliteSpec) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, SrcInstanceID: inst, DstAgent: subgroup, Value: int32(prof), BuffDamage: int32(spec), IsStateChange: evtc.StateEnterCombat})
}

func TestCharacterSwap(t *testing.T) {
	// A player who swaps keeps their address and gets a second table
	// entry: the events follow the instance id.
	b := fixture()
	b.player(addrP2, instP2, "Charlie", ":Bravo.5678", "3", 7, 59)
	b.hit(1000, addrP2, addrBoss, skillHeat, 10, evtc.ResultStrikeDamageNormal)
	b.add(evtc.Event{Time: b.at(4000), SrcAgent: addrP2, SrcInstanceID: 77, DstAgent: addrBoss, SkillID: skillHeat, Value: 20, IFF: evtc.IFFFoe})
	b.add(evtc.Event{Time: b.at(5000), SrcAgent: addrBoss, DstAgent: addrP2, DstInstanceID: 77, SkillID: skillSlam, Value: 30, IFF: evtc.IFFFoe})
	b.hit(6000, addrBoss, addrP2, skillSlam, 40, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(8000))

	if n, m := tl.Players().Count(), tl.Characters().Count(); n != 2 || m != 3 {
		t.Fatalf("players = %d, characters = %d, want 2 and 3", n, m)
	}
	p := tl.PlayerByAccount("Bravo.5678")
	bravo, charlie := tl.CharacterByName("Bravo"), tl.CharacterByName("Charlie")
	if tl.PlayerByName("Charlie") != p || len(p.Characters()) != 2 || p.Characters()[0] != bravo || p.Characters()[1] != charlie {
		t.Fatalf("characters of %v = %v", p, p.Characters())
	}
	if bravo.InstanceID != instP2 || charlie.InstanceID != 77 {
		t.Errorf("instance ids = %d, %d", bravo.InstanceID, charlie.InstanceID)
	}
	if bravo.Hits().Count() != 1 || bravo.HitsTaken().Count() != 1 || charlie.Hits().Count() != 1 || charlie.HitsTaken().Count() != 1 {
		t.Errorf("hits = %d/%d and %d/%d, want 1/1 each", bravo.Hits().Count(), bravo.HitsTaken().Count(), charlie.Hits().Count(), charlie.HitsTaken().Count())
	}
	if want := (Interval{Start: 4 * time.Second, End: 5 * time.Second}); charlie.Lifetime != want {
		t.Errorf("Charlie lifetime = %v, want %v", charlie.Lifetime, want)
	}
	if tl.AgentAt(77, 4500*time.Millisecond) != charlie.Agent {
		t.Error("AgentAt did not find the second character by its instance id")
	}
	if tl.AgentOf(addrP2, 77) != charlie.Agent || tl.AgentOf(addrP2, instP2) != bravo.Agent || tl.AgentOf(addrP2, 0) != bravo.Agent || tl.AgentOf(addrP1, 77) != tl.Agent(addrP1) {
		t.Error("AgentOf did not tell the characters apart by instance id")
	}

	// The player covers both characters.
	if p.Hits().Count() != 2 || p.HitsTaken().Count() != 2 || tl.Hits().By(p).Count() != 2 || tl.Hits().On(p).Count() != 2 {
		t.Errorf("player hits = %d/%d, filtered %d/%d, want 2 each", p.Hits().Count(), p.HitsTaken().Count(), tl.Hits().By(p).Count(), tl.Hits().On(p).Count())
	}
	if first := p.Hits().First(); first == nil || first.Src != bravo.Agent {
		t.Error("the merged hits are out of order")
	}
	if p.Main() != bravo || p.CharacterAt(2*time.Second) != bravo || p.CharacterAt(4500*time.Millisecond) != charlie {
		t.Errorf("main = %v, at 2s %v, at 4.5s %v", p.Main(), p.CharacterAt(2*time.Second), p.CharacterAt(4500*time.Millisecond))
	}
	if p.ProfessionAt(0) != 2 || p.ProfessionAt(4500*time.Millisecond) != 7 || p.SubgroupAt(0) != 2 || p.SubgroupAt(7*time.Second) != 3 || p.EliteSpecAt(7*time.Second) != 59 {
		t.Errorf("profession = %v, subgroup = %v, spec = %v", p.Profession.All(), p.Subgroup.All(), p.EliteSpec.All())
	}
	if tl.Players().OfProfession(7).First() != p || tl.Players().InSubgroup(2).Count() != 1 || tl.Players().InSubgroup(3).First() != p {
		t.Error("the player filters miss a build the player held")
	}
	checkInvariants(t, tl)
}

func TestCharacterComesBack(t *testing.T) {
	// The second entry names the character already on the field: it is
	// the same character under a new instance id.
	b := fixture()
	b.player(addrP2, instP2, "Bravo", ":Bravo.5678", "4", 2, 27)
	b.enterCombat(500, addrP2, instP2, 2, 2, 65)
	b.hit(1000, addrP2, addrBoss, skillHeat, 10, evtc.ResultStrikeDamageNormal)
	b.enterCombat(2000, addrP2, instP2, 2, 2, 27)
	b.add(evtc.Event{Time: b.at(4000), SrcAgent: addrP2, SrcInstanceID: 77, DstAgent: addrBoss, SkillID: skillHeat, Value: 20, IFF: evtc.IFFFoe})
	b.enterCombat(5000, addrP2, 77, 4, 2, 27)
	tl := mustBuild(t, b.build(8000))

	if n, m := tl.Players().Count(), tl.Characters().Count(); n != 2 || m != 2 {
		t.Fatalf("players = %d, characters = %d, want 2 and 2", n, m)
	}
	p := tl.PlayerByName("Bravo")
	c := p.Main()
	if len(p.Characters()) != 1 || c.Hits().Count() != 2 || c.InstanceID != instP2 {
		t.Errorf("characters = %v, hits = %d, instance id = %d", p.Characters(), c.Hits().Count(), c.InstanceID)
	}
	if tl.AgentAt(77, 4500*time.Millisecond) != c.Agent || tl.AgentAt(instP2, time.Second) != c.Agent {
		t.Error("AgentAt did not find the character by both instance ids")
	}
	if n := tl.Agents().Where(func(a *Agent) bool { return a.Addr == addrP2 }).Count(); n != 2 {
		t.Errorf("agents at the address = %d, want 2", n)
	}
	// The table holds the last build of each stay; the first one comes
	// from the events.
	if p.EliteSpecAt(0) != 65 || p.EliteSpecAt(3*time.Second) != 27 || p.EliteSpec.Len() != 2 {
		t.Errorf("spec = %v", p.EliteSpec.All())
	}
	if p.SubgroupAt(time.Second) != 2 || p.SubgroupAt(4*time.Second) != 4 || p.Subgroup.Len() != 2 {
		t.Errorf("subgroup = %v", p.Subgroup.All())
	}
	if p.Profession.Len() != 1 || c.Spec() != "Dragonhunter" {
		t.Errorf("profession = %v, spec = %q", p.Profession.All(), c.Spec())
	}
	checkInvariants(t, tl)
}

func TestDuplicateEntryWithoutEvents(t *testing.T) {
	b := fixture()
	b.player(addrP2, instP2, "Bravo", ":Bravo.5678", "2", 2, 65)
	b.player(0x1003, 13, "Delta", ":Bravo.5678", "2", 3, 0)
	b.hit(1000, addrP2, addrBoss, skillHeat, 10, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(8000))

	if n, m := tl.Players().Count(), tl.Characters().Count(); n != 2 || m != 2 {
		t.Errorf("players = %d, characters = %d, want 2 and 2", n, m)
	}
	if c := tl.CharacterByName("Bravo"); c == nil || c.Hits().Count() != 1 {
		t.Error("the live entry lost its events")
	}
	checkInvariants(t, tl)
}
