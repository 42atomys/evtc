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
