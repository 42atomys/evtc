package combine

import (
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

// strikesOf returns the strikes of a skill in a log, in log order.
func strikesOf(l *evtc.Log, skill uint32) []evtc.Event {
	var out []evtc.Event
	for _, e := range of(l, evtc.StateCombat) {
		if e.SkillID == skill && e.Buff == 0 {
			out = append(out, e)
		}
	}
	return out
}

// TestPairStrikes checks that a strike is in the combined log once,
// whatever the number of clients that tell it.
func TestPairStrikes(t *testing.T) {
	s := newScene(3)
	a, b, c := s.recs[0], s.recs[1], s.recs[2]
	s.each(func(r *recorder) { r.hit(2500, bravo, boss, skillSlam, 700) })
	a.hit(2600, alpha, boss, skillSlam, 701)
	c.hit(2700, charlie, boss, skillSlam, 702)
	b.hit(2800, bravo, boss, skillSlam, 703)
	c.hit(2803, bravo, boss, skillSlam, 703)
	// Two strikes that nothing tells apart are two strikes.
	s.each(func(r *recorder) {
		r.hit(2900, bravo, boss, skillSlam, 704)
		r.hit(2900, bravo, boss, skillSlam, 704)
	})
	l, tl, m := audited(t, s.logs(5000)...)
	got := strikesOf(l, skillSlam)
	want := []struct {
		at    uint64
		value int32
	}{{2500, 700}, {2600, 701}, {2700, 702}, {2800, 703}, {2900, 704}, {2900, 704}}
	if len(got) != len(want) {
		t.Fatalf("%d strikes, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Time != a.at(w.at) || got[i].Value != w.value || got[i].SrcAgent>>32 != 1 || got[i].DstAgent != a.addr(boss) {
			t.Errorf("strike %d = %+v", i, got[i])
		}
	}
	if n := tl.Hits().On(tl.Boss()).Count(); n != 40+6 {
		t.Errorf("%d hits on the boss", n)
	}
	if m.audit.pairs[key{kind: evtc.StateCombat, src: 2, dst: 4, skill: skillSlam, p: payload{704}}] != 4 {
		t.Errorf("pairs = %v", m.audit.pairs)
	}
}

// TestPairWindows checks how far apart two clients may date a message.
func TestPairWindows(t *testing.T) {
	for _, tt := range []struct {
		apart uint64
		want  int
	}{{0, 1}, {25, 1}, {90, 1}, {400, 1}, {2000, 1}, {2001, 2}} {
		s := newScene(2)
		s.recs[0].hit(2500, bravo, boss, skillSlam, 700)
		s.recs[1].hit(2500+tt.apart, bravo, boss, skillSlam, 700)
		l, _, _ := audited(t, s.logs(6000)...)
		if got := strikesOf(l, skillSlam); len(got) != tt.want || got[0].Time != s.recs[0].at(2500) {
			t.Errorf("%d ms apart: %d strikes, the first at %d", tt.apart, len(got), got[0].Time)
		}
	}
}

// TestPairNearest checks that a message pairs with the nearest one of its
// key, the close ones first.
func TestPairNearest(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	for _, ts := range []uint64{2500, 2800, 3100} {
		a.hit(ts, bravo, boss, skillSlam, 700)
	}
	// The second client missed the first one and dates the others late.
	b.hit(2890, bravo, boss, skillSlam, 700)
	b.hit(3110, bravo, boss, skillSlam, 700)
	l, _, _ := audited(t, s.logs(6000)...)
	got := strikesOf(l, skillSlam)
	if len(got) != 3 || got[0].Time != a.at(2500) || got[1].Time != a.at(2800) || got[2].Time != a.at(3100) {
		t.Errorf("strikes = %+v", got)
	}
}

// TestLateness checks that a client that gets the messages of a path late
// pairs them where they were sent.
func TestLateness(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	const late = 600
	// Messages told once tell the delay.
	for i := range 10 {
		a.hit(uint64(2500+100*i), bravo, boss, skillSlam, int32(800+i))
		b.hit(uint64(2500+100*i+late), bravo, boss, skillSlam, int32(800+i))
	}
	// The same message every 400 ms: late by 600 ms, each is nearer to
	// the next one.
	for i := range 6 {
		a.hit(uint64(4000+400*i), bravo, boss, skillSlam, 700)
		b.hit(uint64(4000+400*i+late), bravo, boss, skillSlam, 700)
	}
	recs := mustRecord(t, s.logs(9000)...)
	m := &combiner{recs: recs, byInst: map[uint16][]*entity{}, tellings: map[int32]map[slot][]told{}, before: map[int32]map[slot]told{}}
	m.ents = entities(recs)
	m.index, m.again = map[key][]*place{}, map[key][]*place{}
	m.defs, m.exts = map[[64]byte]bool{}, map[[56]byte]int{}
	m.keep(0, m.read(0))
	delays := m.lateness(m.read(1))
	id := func(player uint16) int32 { return recs[0].entity[recs[0].tl.Agent(a.addr(player))].id }
	fromAlpha := path{evtc.StateCombat, id(alpha), id(boss)}
	fromBravo := path{evtc.StateCombat, id(bravo), id(boss)}
	if d, ok := delays[fromAlpha]; !ok || d != 0 {
		t.Errorf("the strikes of Alpha are %v late, measured %v", d, ok)
	}
	if d := delays[fromBravo]; d != late*time.Millisecond {
		t.Errorf("the strikes of Bravo are %v late, want %v", d, late*time.Millisecond)
	}
	if len(delays) != 2 {
		t.Errorf("delays = %v", delays)
	}

	l, _, _ := audited(t, s.logs(9000)...)
	if got := strikesOf(l, skillSlam); len(got) != 16 {
		t.Errorf("%d strikes, want 16", len(got))
	}
}

// TestPairNamesAgent checks that a message names the agent a client left
// out when another one names it.
func TestPairNamesAgent(t *testing.T) {
	for _, first := range []int{0, 1} {
		s := newScene(2)
		s.each(func(r *recorder) {
			r.npc(crow, 1083, "Crow")
			r.spawn(1000, crow)
			r.despawn(4000, crow)
		})
		// One client writes the target by its instance id alone.
		r := s.recs[first]
		r.ev(2500, evtc.StateCombat, bravo, 0, evtc.Event{DstInstanceID: crow, SkillID: skillSlam, Value: 700, IFF: evtc.IFFFoe})
		s.recs[1-first].hit(2510, bravo, crow, skillSlam, 700)
		l, tl, _ := audited(t, s.logs(5000)...)
		got := strikesOf(l, skillSlam)
		if len(got) != 1 || got[0].DstAgent != s.recs[0].addr(crow) || got[0].DstInstanceID != crow {
			t.Errorf("first %d: strikes = %+v", first, got)
		}
		if n := tl.Hits().On(agentsNamed(tl, "Crow")[0]).Count(); n != 1 {
			t.Errorf("first %d: %d hits on the crow", first, n)
		}
	}
}

// TestPairByInstance checks the message two clients date on either side
// of the instant an instance id changes hands: each finds another agent
// for it, and the instance id tells it is the same message.
func TestPairByInstance(t *testing.T) {
	s := newScene(2)
	s.each(func(r *recorder) {
		r.npc(crow, 1083, "Crow")
		r.l.Agents = append(r.l.Agents, evtc.Agent{Addr: r.base | 0x100 | crow, Profession: 1084, IsElite: 0xffffffff, Name: "Rat"})
		r.spawn(1000, crow)
		r.position(1100, crow, 1, 1)
		r.despawn(2000, crow)
		rat := r.base | 0x100 | crow
		r.l.Events = append(r.l.Events,
			evtc.Event{Time: r.at(2000), SrcAgent: rat, SrcInstanceID: crow, IsStateChange: evtc.StateSpawn},
			evtc.Event{Time: r.at(2100), SrcAgent: rat, SrcInstanceID: crow, IsStateChange: evtc.StateEnterCombat},
			evtc.Event{Time: r.at(3000), SrcAgent: rat, SrcInstanceID: crow, IsStateChange: evtc.StateDespawn})
	})
	strike := evtc.Event{DstInstanceID: crow, SkillID: skillSlam, Value: 700, IFF: evtc.IFFFoe}
	s.recs[0].ev(1995, evtc.StateCombat, bravo, 0, strike)
	s.recs[1].ev(2005, evtc.StateCombat, bravo, 0, strike)
	l, _, _ := audited(t, s.logs(5000)...)
	if got := strikesOf(l, skillSlam); len(got) != 1 || got[0].Time != s.recs[0].at(1995) {
		t.Errorf("strikes = %+v", got)
	}
}

// TestOwnBuffs checks that the buff events of the character of a
// recording player are those its client tells.
func TestOwnBuffs(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	// Both tell it.
	a.apply(1500, bravo, alpha, skillMight, 9000, 1)
	b.apply(1510, bravo, alpha, skillMight, 9000, 1)
	// The other client alone: of a part of the buffs it is told, it makes
	// stacks the character does not wear.
	b.apply(2500, bravo, alpha, skillMight, 9000, 2)
	a.apply(2600, alpha, bravo, skillMight, 9000, 3)
	// The client of the character alone.
	a.apply(3500, bravo, alpha, skillMight, 9000, 4)
	b.apply(3600, alpha, bravo, skillMight, 9000, 5)
	// A player without a recording: what any client tells.
	a.apply(3700, alpha, charlie, skillMight, 9000, 6)
	b.apply(3800, alpha, charlie, skillMight, 9000, 7)
	l, tl, _ := audited(t, s.logs(5000)...)
	var stacks []uint64
	for _, e := range of(l, evtc.StateBuffApply) {
		stacks = append(stacks, trackable(&e))
	}
	if want := []uint64{1, 4, 5, 6, 7}; len(stacks) != len(want) || stacks[0] != 1 || stacks[1] != 4 || stacks[2] != 5 || stacks[3] != 6 || stacks[4] != 7 {
		t.Errorf("stacks applied = %v, want %v", stacks, want)
	}
	if n := tl.PlayerByName("Alpha").Ref().Stacks().Count(); n != 2 {
		t.Errorf("%d stacks on Alpha", n)
	}
}

// TestOwnBuffsAtBounds checks that where the recording of the client of
// a character starts or ends, what another client tells alone is kept.
func TestOwnBuffsAtBounds(t *testing.T) {
	s := newScene(2)
	s.recs[1].apply(4950, bravo, alpha, skillMight, 9000, 2)
	s.recs[1].apply(4000, bravo, alpha, skillMight, 9000, 3)
	logs := s.logs(5000)
	// The second client records half a second more.
	for i := range logs[1].Events {
		if e := &logs[1].Events[i]; e.Time == s.recs[1].at(5000) {
			e.Time += 500
		}
	}
	l, _, _ := audited(t, logs...)
	got := of(l, evtc.StateBuffApply)
	if len(got) != 1 || trackable(&got[0]) != 2 {
		t.Errorf("stacks applied = %+v", got)
	}
}

// TestOwnTicks checks that a buff damage tick comes from the client of
// the player who deals or takes it, which is told of it; the others
// estimate it.
func TestOwnTicks(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	a.tick(2000, alpha, boss, 737, 100)
	b.tick(2004, alpha, boss, 737, 95)
	b.tick(2500, bravo, boss, 737, 80)
	a.tick(2503, bravo, boss, 737, 85)
	a.tick(3000, boss, charlie, 737, 60)
	b.tick(3002, boss, charlie, 737, 61)
	l, _, _ := audited(t, s.logs(5000)...)
	var got []int32
	for _, e := range of(l, evtc.StateCombat) {
		if e.Buff != 0 {
			got = append(got, e.BuffDamage)
		}
	}
	// The ticks on a player without a recording come from the client
	// that speaks for it.
	if len(got) != 3 || got[0] != 100 || got[1] != 80 || got[2] != 60 {
		t.Errorf("ticks = %v, want [100 80 60]", got)
	}
}

// TestRelayedHeals checks the events of an extension a client got from
// another one, seconds late: they pair in their order, and the earliest
// copy dates them.
func TestRelayedHeals(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	const fromSrc = 0x80
	a.heal(1500, alpha, bravo, skillRegen, 900, fromSrc)
	a.heal(1500, alpha, bravo, skillRegen, 900, fromSrc)
	a.heal(2500, alpha, bravo, skillRegen, 900, fromSrc)
	b.heal(9500, alpha, bravo, skillRegen, 900, fromSrc)
	b.heal(9500, alpha, bravo, skillRegen, 900, fromSrc)
	b.heal(9501, alpha, bravo, skillRegen, 900, fromSrc)
	// Beyond relay they are two heals.
	a.heal(3000, alpha, charlie, skillRegen, 400, fromSrc)
	b.heal(63001, alpha, charlie, skillRegen, 400, fromSrc)
	// The second client wrote some first.
	a.heal(12000, bravo, alpha, skillRegen, 300, fromSrc)
	a.heal(12000, bravo, alpha, skillRegen, 300, fromSrc)
	b.heal(4000, bravo, alpha, skillRegen, 300, fromSrc)
	b.heal(4100, bravo, alpha, skillRegen, 300, fromSrc)
	// One that comes 45 seconds late.
	a.heal(20000, alpha, charlie, skillRegen, 500, fromSrc)
	b.heal(65000, alpha, charlie, skillRegen, 500, fromSrc)
	l, _, _ := audited(t, s.logs(70000)...)
	got := of(l, evtc.StateExtensionCombat)
	want := []struct {
		at    uint64
		value int32
	}{{1500, -900}, {1500, -900}, {2500, -900}, {3000, -400}, {4000, -300}, {4100, -300}, {20000, -500}, {63001, -400}}
	if len(got) != len(want) {
		t.Fatalf("%d heals, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Time != a.at(w.at) || got[i].Value != w.value {
			t.Errorf("heal %d at %d of %d, want at %d of %d", i, got[i].Time-a.at(0), got[i].Value, w.at, w.value)
		}
	}
}

// TestExtensionAgents checks that an event of an extension names the
// agent that was there: an extension keeps its own list of agents, and
// names one that left when its instance id serves again.
func TestExtensionAgents(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) {
		r.npc(crow, 1083, "Crow")
		r.l.Agents = append(r.l.Agents, evtc.Agent{Addr: r.base | 0x100 | crow, Profession: 1084, IsElite: 0xffffffff, Name: "Rat"})
		r.spawn(1000, crow)
		r.despawn(2000, crow)
		rat := r.base | 0x100 | crow
		r.l.Events = append(r.l.Events,
			evtc.Event{Time: r.at(2500), SrcAgent: rat, SrcInstanceID: crow, IsStateChange: evtc.StateSpawn},
			evtc.Event{Time: r.at(4000), SrcAgent: rat, SrcInstanceID: crow, IsStateChange: evtc.StateDespawn})
	})
	// The first client names the crow, long gone, the second one the rat.
	a.heal(3000, alpha, crow, skillRegen, 900, 0x80)
	b.ev(3005, evtc.StateExtensionCombat, alpha, 0, withID(evtc.Event{DstAgent: b.base | 0x100 | crow, DstInstanceID: crow, SkillID: skillRegen, Value: -900, IsOffcycle: 0x80}, healing))
	l, _, _ := audited(t, s.logs(5000)...)
	got := of(l, evtc.StateExtensionCombat)
	if len(got) != 1 || got[0].DstAgent != a.base|0x100|crow || got[0].Time != a.at(3000) {
		t.Errorf("heals = %+v", got)
	}
}

// TestRegistrations checks that an extension two clients run registers
// once.
func TestRegistrations(t *testing.T) {
	s := newScene(2)
	s.each(func(r *recorder) {
		r.ev(0, evtc.StateExtension, 0, 0, withID(evtc.Event{SrcAgent: 0x07000002_00000000 | healing, DstAgent: 0x32637239312e32}, healing))
	})
	s.recs[1].ev(0, evtc.StateExtension, 0, 0, withID(evtc.Event{SrcAgent: 0x1234}, 0x1234))
	l, tl, _ := audited(t, s.logs(5000)...)
	if count(l, evtc.StateExtension) != 2 || len(tl.Extensions) != 2 {
		t.Errorf("%d registrations, %d extensions", count(l, evtc.StateExtension), len(tl.Extensions))
	}
}

// TestRecordingKinds checks that what a client writes once about its own
// recording comes from the first one.
func TestRecordingKinds(t *testing.T) {
	s := newScene(2)
	for i, r := range s.recs {
		r.ev(0, evtc.StateGWBuild, 0, 0, evtc.Event{SrcAgent: uint64(170000 + i)})
		r.l.Events = append(r.l.Events, evtc.Event{Time: 0x6e6567, IsStateChange: evtc.StateIntegrity})
		r.ev(600, evtc.StateIIDChange, 0, 0, evtc.Event{SrcAgent: r.addr(99), DstAgent: r.addr(98)})
	}
	l, tl, _ := audited(t, s.logs(5000)...)
	for _, k := range []evtc.StateChange{evtc.StatePointOfView, evtc.StateLanguage, evtc.StateGWBuild, evtc.StateIntegrity} {
		if n := count(l, k); n != 1 {
			t.Errorf("%d %v events", n, k)
		}
	}
	if n := count(l, evtc.StateIIDChange); n != 0 {
		t.Errorf("%d address changes", n)
	}
	if tl.POV == nil || tl.POV.Account != "Alpha.1234" || tl.Language != 0 || tl.GameBuild != 170000 {
		t.Errorf("point of view %v, language %v, game build %d", tl.POV, tl.Language, tl.GameBuild)
	}
}

// TestRewards checks that a reward several clients got is there once, and
// that one a client alone got is there as well.
func TestRewards(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	a.ev(4900, evtc.StateReward, 0, 0, evtc.Event{DstAgent: 100, Value: 13})
	b.ev(4910, evtc.StateReward, 0, 0, evtc.Event{DstAgent: 100, Value: 13})
	b.ev(4905, evtc.StateReward, 0, 0, evtc.Event{DstAgent: 101, Value: 13})
	// The same reward given again, long after.
	b.ev(8000, evtc.StateReward, 0, 0, evtc.Event{DstAgent: 100, Value: 13})
	// The recording read first dates what both tell.
	for i, logs := range [][]*evtc.Log{s.logs(9000), {b.l, a.l}} {
		_, tl, _ := audited(t, logs...)
		var got [][2]uint64
		for _, r := range tl.Rewards {
			got = append(got, [2]uint64{uint64(r.Time.Milliseconds()), r.ID})
		}
		want := [][2]uint64{{4900, 100}, {4905, 101}, {8000, 100}}
		if i == 1 {
			want = [][2]uint64{{4905, 101}, {4910, 100}, {8000, 100}}
		}
		if !slices.Equal(got, want) {
			t.Errorf("rewards = %v, want %v", got, want)
		}
	}
}

// TestLocalKinds checks what a client writes about its own recording over
// time: one recording tells it at a time, and the others where the first
// one started late or stopped early.
func TestLocalKinds(t *testing.T) {
	s := newScene(2)
	for i, r := range s.recs {
		for at := uint64(500); at < 5000; at += 1000 {
			r.ev(at+uint64(i), evtc.StateTick, 0, 0, evtc.Event{SrcAgent: 25, Value: int32(40 + i)})
		}
	}
	b := s.recs[1]
	b.ev(4000, evtc.StateMapChange, 0, 0, evtc.Event{SrcAgent: 1155, DstAgent: 1062})
	logs := s.logs(5000)
	// The first client stopped to record at three seconds.
	logs[0].Events = slices.DeleteFunc(logs[0].Events, func(e evtc.Event) bool { return e.Time > s.recs[0].at(3000) && e.Time < s.recs[0].at(5000) })
	for i := range logs[0].Events {
		if e := &logs[0].Events[i]; e.Time == s.recs[0].at(5000) {
			e.Time -= 2000
		}
	}
	l, tl, _ := audited(t, logs...)
	var got [][2]uint64
	for _, e := range of(l, evtc.StateTick) {
		got = append(got, [2]uint64{e.Time - rawEpoch, uint64(e.Value)})
	}
	if want := [][2]uint64{{500, 40}, {1500, 40}, {2500, 40}, {3501, 41}, {4501, 41}}; !slices.Equal(got, want) {
		t.Errorf("ticks = %v, want %v", got, want)
	}
	if tl.PingAt(2*time.Second) != 40 || tl.PingAt(4*time.Second) != 41 || len(tl.MapChanges) != 1 || tl.Duration != 5*time.Second {
		t.Errorf("ping %d then %d, %d map changes over %v", tl.PingAt(2*time.Second), tl.PingAt(4*time.Second), len(tl.MapChanges), tl.Duration)
	}
}

// TestFightKinds checks the events every client writes about the fight:
// the earliest start and the latest end.
func TestFightKinds(t *testing.T) {
	s := newScene(3)
	logs := s.logs(5000)
	// The second client started 300 ms before the first one and stopped
	// 400 ms after it, the third one inside what the first one recorded.
	move := func(l *evtc.Log, kind evtc.StateChange, by int64) {
		for i := range l.Events {
			if e := &l.Events[i]; e.IsStateChange == kind {
				e.Time = uint64(int64(e.Time) + by)
			}
		}
	}
	move(logs[1], evtc.StateSquadCombatStart, -300)
	move(logs[1], evtc.StateSquadCombatEnd, 400)
	move(logs[2], evtc.StateSquadCombatStart, 200)
	move(logs[2], evtc.StateSquadCombatEnd, -200)
	l, tl, _ := audited(t, logs...)
	starts, ends, targets := of(l, evtc.StateSquadCombatStart), of(l, evtc.StateSquadCombatEnd), of(l, evtc.StateLogNPCUpdate)
	if len(starts) != 1 || starts[0].Time != rawEpoch-300 {
		t.Errorf("starts = %+v", starts)
	}
	if len(ends) != 1 || ends[0].Time != rawEpoch+5400 {
		t.Errorf("ends = %+v", ends)
	}
	if len(targets) != 1 || targets[0].DstAgent != s.recs[0].addr(boss) {
		t.Errorf("targets = %+v", targets)
	}
	if tl.Duration != 5700*time.Millisecond || tl.Boss() == nil {
		t.Errorf("the fight lasts %v", tl.Duration)
	}
}

// TestDefinitions checks that the definitions of the recordings are
// united.
func TestDefinitions(t *testing.T) {
	s := newScene(2)
	def := func(kind evtc.StateChange, skill uint32) evtc.Event {
		return evtc.Event{Time: 0x1234, SkillID: skill, IsStateChange: kind}
	}
	a, b := s.recs[0], s.recs[1]
	a.l.Events = append(a.l.Events, def(evtc.StateBuffInfo, skillMight), def(evtc.StateSkillInfo, skillHeat), def(evtc.StateSkillInfo, skillHeat))
	b.l.Events = append(b.l.Events, def(evtc.StateBuffInfo, skillMight), def(evtc.StateBuffInfo, skillRegen), def(evtc.StateBuffInfo, skillRegen))
	b.l.Skills = append(b.l.Skills, evtc.Skill{ID: 999, Name: "Blink"})
	l, _, _ := audited(t, s.logs(5000)...)
	// The first recording keeps what it wrote, the others add what is new.
	if count(l, evtc.StateBuffInfo) != 2 || count(l, evtc.StateSkillInfo) != 2 {
		t.Errorf("%d buff and %d skill definitions", count(l, evtc.StateBuffInfo), count(l, evtc.StateSkillInfo))
	}
	for i := range 4 {
		if timed(l.Events[i].IsStateChange) {
			t.Errorf("event %d is %v: the definitions come first", i, l.Events[i].IsStateChange)
		}
	}
	if len(l.Skills) != 5 || l.Skills[4].Name != "Blink" {
		t.Errorf("skills = %+v", l.Skills)
	}
}
