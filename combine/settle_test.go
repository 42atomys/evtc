package combine

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

// TestStackNumbers checks the stacks two clients number alike: the one
// the other client does not hold gets a number of its own, and the events
// that name it follow.
func TestStackNumbers(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) {
		r.npc(crow, 1083, "Crow")
		r.spawn(1000, crow)
		r.despawn(9000, crow)
	})
	a.apply(1500, alpha, crow, skillMight, 9000, 5)
	b.apply(1600, bravo, crow, skillRegen, 9000, 5)
	b.ev(2000, evtc.StateBuffChange, 0, crow, withID(evtc.Event{SkillID: skillRegen, Value: 500}, 5))
	b.ev(2100, evtc.StateBuffActive, crow, 0, evtc.Event{DstAgent: 5})
	b.remove(3000, crow, skillRegen, 5)
	a.remove(3500, crow, skillMight, 5)
	l, tl, _ := audited(t, s.logs(10000)...)
	var got []uint64
	for _, e := range l.Events {
		switch e.IsStateChange {
		case evtc.StateBuffApply, evtc.StateBuffChange, evtc.StateBuffRemoveSingle:
			got = append(got, trackable(&e))
		case evtc.StateBuffActive:
			got = append(got, e.DstAgent)
		}
	}
	want := []uint64{5, firstOwn, firstOwn, firstOwn, firstOwn, 5}
	if len(got) != len(want) {
		t.Fatalf("stack numbers = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("stack numbers = %v, want %v", got, want)
			break
		}
	}
	c := agentsNamed(tl, "Crow")[0]
	stacks := c.Stacks().All()
	if len(stacks) != 2 || stacks[0].Interval != iv(1500, 3500) || stacks[1].Interval != iv(1600, 3000) {
		t.Errorf("stacks = %v", stacks)
	}
}

// TestStackNumberReused checks that a client that gives the number of a
// stack to the next one ends the first: as within one log.
func TestStackNumberReused(t *testing.T) {
	s := newScene(2)
	a := s.recs[0]
	s.each(func(r *recorder) {
		r.npc(crow, 1083, "Crow")
		r.spawn(1000, crow)
		r.despawn(9000, crow)
	})
	a.apply(1500, alpha, crow, skillMight, 9000, 5)
	a.apply(4500, alpha, crow, skillRegen, 9000, 5)
	l, _, _ := audited(t, s.logs(10000)...)
	for _, e := range of(l, evtc.StateBuffApply) {
		if trackable(&e) != 5 {
			t.Errorf("stack of buff %d numbered %d", e.SkillID, trackable(&e))
		}
	}
}

// TestStackNumberGiven checks the number of a stack two clients told,
// which one of them gives to the next stack: the first one ended for it,
// and the number stays.
func TestStackNumberGiven(t *testing.T) {
	s := newScene(2)
	b := s.recs[1]
	s.each(func(r *recorder) {
		r.npc(crow, 1083, "Crow")
		r.spawn(1000, crow)
		r.apply(1500, alpha, crow, skillMight, 9000, 5)
		r.despawn(9000, crow)
	})
	b.apply(4500, bravo, boss, skillRegen, 9000, 5)
	b.ev(4600, evtc.StateBuffChange, 0, boss, withID(evtc.Event{SkillID: skillRegen, Value: 500}, 5))
	b.remove(5000, boss, skillRegen, 5)
	l, tl, _ := audited(t, s.logs(10000)...)
	for _, e := range l.Events {
		switch e.IsStateChange {
		case evtc.StateBuffApply, evtc.StateBuffChange, evtc.StateBuffRemoveSingle:
			if trackable(&e) != 5 {
				t.Errorf("%v of buff %d numbered %d", e.IsStateChange, e.SkillID, trackable(&e))
			}
		}
	}
	if got := tl.Stacks().Count(); got != 2 {
		t.Errorf("%d stacks", got)
	}
}

// TestStacksOnGain checks the stacks a client tells when it gains an
// agent: those the combined log holds are told again.
func TestStacksOnGain(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) { r.npc(crow, 1083, "Crow") })
	a.spawn(1000, crow)
	a.apply(1500, alpha, crow, skillMight, 9000, 5)
	b.spawn(2500, crow)
	b.initial(2501, alpha, crow, skillMight, 8000, 5)
	// One the first client does not hold.
	b.initial(2501, bravo, crow, skillRegen, 4000, 6)
	s.each(func(r *recorder) { r.despawn(9000, crow) })
	l, tl, _ := audited(t, s.logs(10000)...)
	if n := count(l, evtc.StateBuffApply) + count(l, evtc.StateBuffInitial); n != 2 {
		t.Errorf("%d stacks opened", n)
	}
	stacks := agentsNamed(tl, "Crow")[0].Stacks().All()
	if len(stacks) != 2 || stacks[0].Buff.Skill.ID != skillMight || stacks[0].Interval.Start != 1500*time.Millisecond || stacks[1].Buff.Skill.ID != skillRegen {
		t.Errorf("stacks = %v", stacks)
	}
}

// TestStacksOnGainTwice checks that two clients that gain an agent pair
// the stacks it carries, dated by the first to see them.
func TestStacksOnGainTwice(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) { r.npc(crow, 1083, "Crow") })
	a.spawn(2040, crow)
	a.initial(2041, alpha, crow, skillMight, 8000, 5)
	b.spawn(2000, crow)
	b.initial(2001, alpha, crow, skillMight, 8040, 5)
	s.each(func(r *recorder) { r.despawn(9000, crow) })
	l, _, _ := audited(t, s.logs(10000)...)
	got := of(l, evtc.StateBuffInitial)
	if len(got) != 1 || got[0].Time != rawEpoch+2001 || got[0].Value != 8040 {
		t.Errorf("stacks = %+v", got)
	}
}

// TestStackEndsByDespawn checks that a despawn ends the stacks of an
// agent: the number of one serves again for the agent that follows.
func TestStackEndsByDespawn(t *testing.T) {
	st := newStacks()
	open := func(ent int32, skill uint32, id uint32, origin int8, at uint64) *cand {
		c := &cand{e: withID(evtc.Event{SkillID: skill, Time: at}, id), ent: ent, entry: ent*10 + int32(origin), origin: origin, stack: stackOpens}
		if st.settle(c) {
			t.Errorf("stack %d of %d told again", id, ent)
		}
		return c
	}
	open(1, skillMight, 5, 0, 1000)
	open(1, skillRegen, 6, 0, 1000)
	open(2, skillMight, 7, 0, 1000)
	st.settle(&cand{ent: 1, stack: stackClosesBuff, e: evtc.Event{SkillID: skillRegen}})
	if len(st.open) != 2 || len(st.holder) != 2 {
		t.Errorf("after the buff is removed: %d stacks open, %d numbers held", len(st.open), len(st.holder))
	}
	st.settle(&cand{ent: 1, stack: stackClosesAll})
	if len(st.open) != 1 || len(st.holder) != 1 {
		t.Errorf("after the despawn: %d stacks open, %d numbers held", len(st.open), len(st.holder))
	}
	// The number is free: another client may give it.
	if c := open(3, skillMight, 5, 1, 2000); trackable(&c.e) != 5 {
		t.Errorf("stack numbered %d", trackable(&c.e))
	}
	// A number held by a stack of another client is not.
	if c := open(3, skillRegen, 7, 1, 2000); trackable(&c.e) != firstOwn {
		t.Errorf("stack numbered %d", trackable(&c.e))
	}
	// A removal that names a stack the combined log does not hold, under
	// a number another one carries, ends nothing.
	c := &cand{ent: 4, origin: 1, stack: stackCloses, e: withID(evtc.Event{SkillID: skillMight}, 7)}
	st.settle(c)
	if trackable(&c.e) != firstOwn+1 || len(st.open) != 3 {
		t.Errorf("removal numbered %d, %d stacks open", trackable(&c.e), len(st.open))
	}
}

// TestStatesToldAgain checks the states a client tells of an agent it
// has just gained: the combined log holds them.
func TestStatesToldAgain(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) { r.npc(crow, 1083, "Crow") })
	a.spawn(1000, crow)
	a.health(1500, crow, 80)
	a.despawn(2000, crow)
	// The second client holds the crow the longest and speaks for it
	// from the time it gains it.
	b.spawn(1900, crow)
	b.health(1901, crow, 80)
	b.health(3000, crow, 80)
	b.health(3500, crow, 70)
	b.despawn(9000, crow)
	l, _, _ := audited(t, s.logs(10000)...)
	if got := healths(l); !sameHealths(got, [2]uint64{1500, 80}, [2]uint64{3000, 80}, [2]uint64{3500, 70}) {
		t.Errorf("health updates = %v", got)
	}
}

// TestStatesToldTwice checks the state a client tells twice with the same
// value, a moment apart, around the time it starts to speak for the agent:
// the first telling is the one the client that spoke before dated, the
// second one is another event.
func TestStatesToldTwice(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) {
		r.npc(crow, 1083, "Crow")
		r.spawn(1000, crow)
		r.health(1500, crow, 90)
		r.health(2900, crow, 80)
	})
	a.despawn(3000, crow)
	b.health(3200, crow, 80)
	b.health(3300, crow, 70)
	b.despawn(3400, crow)
	l, _, m := audited(t, s.logs(5000)...)
	if x := m.recs[0].entity[m.recs[0].tl.Agent(a.addr(crow))]; len(x.speak) != 2 || x.speak[0].rec != 0 || x.speak[1].rec != 1 {
		t.Fatalf("speakers = %+v", x.speak)
	}
	if got := healths(l); !sameHealths(got, [2]uint64{1500, 90}, [2]uint64{2900, 80}, [2]uint64{3200, 80}, [2]uint64{3300, 70}) {
		t.Errorf("health updates = %v", got)
	}

	// The second client dates the last telling of the first one after the
	// first one left: the same telling, once.
	s = newScene(2)
	a, b = s.recs[0], s.recs[1]
	s.each(func(r *recorder) {
		r.npc(crow, 1083, "Crow")
		r.spawn(1000, crow)
		r.health(1500, crow, 90)
	})
	a.health(2990, crow, 80)
	a.despawn(3000, crow)
	b.health(3005, crow, 80)
	b.health(3300, crow, 70)
	b.despawn(3400, crow)
	l, _, _ = audited(t, s.logs(5000)...)
	if got := healths(l); !sameHealths(got, [2]uint64{1500, 90}, [2]uint64{2990, 80}, [2]uint64{3300, 70}) {
		t.Errorf("health updates = %v", got)
	}
}

// TestStatesAfterLeave checks that what an agent was is told again when
// it comes back.
func TestStatesAfterLeave(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) { r.npc(crow, 1083, "Crow") })
	a.spawn(1000, crow)
	a.health(1500, crow, 80)
	a.despawn(2000, crow)
	b.spawn(3000, crow)
	b.health(3001, crow, 80)
	b.despawn(9000, crow)
	l, _, _ := audited(t, s.logs(10000)...)
	if got := healths(l); !sameHealths(got, [2]uint64{1500, 80}, [2]uint64{3001, 80}) {
		t.Errorf("health updates = %v", got)
	}
}

// TestMarkersToldAgain checks the markers a client tells of an agent
// another one spoke for: the same ones are told again, other ones
// replace them.
func TestMarkersToldAgain(t *testing.T) {
	for _, tt := range []struct {
		name   string
		second []int32
		want   []int32
	}{
		{"the same markers", []int32{0, 3, 4}, []int32{0, 3, 4}},
		{"another marker", []int32{0, 5}, []int32{0, 3, 4, 0, 5}},
		{"a marker on top", []int32{3, 6}, []int32{0, 3, 4, 6}},
	} {
		s := newScene(2)
		a, b := s.recs[0], s.recs[1]
		s.each(func(r *recorder) { r.npc(crow, 1083, "Crow") })
		a.spawn(1000, crow)
		for _, v := range []int32{0, 3, 4} {
			a.marker(1500, crow, v)
		}
		a.despawn(2000, crow)
		b.spawn(1900, crow)
		for _, v := range tt.second {
			b.marker(1901, crow, v)
		}
		b.despawn(9000, crow)
		l, _, _ := audited(t, s.logs(10000)...)
		var got []int32
		for _, e := range of(l, evtc.StateMarker) {
			got = append(got, e.Value)
		}
		if len(got) != len(tt.want) {
			t.Errorf("%s: markers = %v, want %v", tt.name, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: markers = %v, want %v", tt.name, got, tt.want)
				break
			}
		}
	}
}

// TestOrder checks the order of the events of one instant: those of a
// recording keep theirs, a spawn the recordings date otherwise comes
// first and a leave last.
func TestOrder(t *testing.T) {
	at := func(time uint64, origin int8, seq int, c cand) cand {
		c.e.Time, c.e.IsStateChange, c.origin, c.seq = time, evtc.StateSpawn, origin, seq
		return c
	}
	sorted := []cand{
		{e: evtc.Event{Time: 9999, IsStateChange: evtc.StateBuffInfo}},
		at(1000, 0, 7, cand{}),
		at(2000, 1, 9, cand{bound: -1, moved: true}),
		at(2000, 1, 3, cand{carried: true}),
		at(2000, 0, 4, cand{bound: -1}),
		at(2000, 0, 5, cand{bound: 1}),
		at(2000, 1, 2, cand{}),
		at(2000, 0, 1, cand{bound: 1, moved: true}),
		at(3000, 1, 0, cand{}),
	}
	for i := 1; i < len(sorted); i++ {
		if before(sorted[i-1], sorted[i]) >= 0 || before(sorted[i], sorted[i-1]) <= 0 {
			t.Errorf("events %d and %d are not in order", i-1, i)
		}
	}
	if before(sorted[3], sorted[3]) != 0 {
		t.Error("an event comes before itself")
	}
}
