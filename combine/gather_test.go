package combine

import (
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
