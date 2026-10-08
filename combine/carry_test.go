package combine

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

// handover returns a scene where the first client holds a crow from 1 s
// to 3 s and the second one from 2 s to 4 s: the first one speaks for it
// until 3 s, the second one afterwards.
func handover() (s *scene, a, b *recorder) {
	s = newScene(2)
	a, b = s.recs[0], s.recs[1]
	s.each(func(r *recorder) { r.npc(crow, 1083, "Crow") })
	a.spawn(1000, crow)
	a.despawn(3000, crow)
	b.spawn(2000, crow)
	b.despawn(4000, crow)
	return s, a, b
}

// healths returns the health updates of a log as times of the scene and
// percentages.
func healths(l *evtc.Log) [][2]uint64 {
	var out [][2]uint64
	for _, e := range of(l, evtc.StateHealthPctUpdate) {
		out = append(out, [2]uint64{e.Time - rawEpoch, e.DstAgent / 100})
	}
	return out
}

func sameHealths(got [][2]uint64, want ...[2]uint64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestCarryState checks what the combined log gets when the speaker of
// an agent changes: what the new one held, when it is newer than what is
// told.
func TestCarryState(t *testing.T) {
	s, a, b := handover()
	a.health(1500, crow, 80)
	a.health(2800, crow, 60)
	// What the second client tells while the first one speaks.
	b.health(2001, crow, 80)
	b.health(2901, crow, 55)
	b.health(3500, crow, 40)
	l, tl, _ := audited(t, s.logs(5000)...)
	if got := healths(l); !sameHealths(got, [2]uint64{1500, 80}, [2]uint64{2800, 60}, [2]uint64{3001, 55}, [2]uint64{3500, 40}) {
		t.Errorf("health updates = %v", got)
	}
	c := agentsNamed(tl, "Crow")[0]
	for _, tt := range []struct {
		at   int
		want float64
	}{{2000, 80}, {2950, 60}, {3200, 55}, {3800, 40}} {
		if got := c.HealthAt(time.Duration(tt.at) * time.Millisecond); got != tt.want {
			t.Errorf("health at %d ms = %v, want %v", tt.at, got, tt.want)
		}
	}
}

// TestCarryStale checks that what the new speaker held is left out when
// the combined log tells something newer, or the same.
func TestCarryStale(t *testing.T) {
	s, a, b := handover()
	a.health(1500, crow, 80)
	a.health(2800, crow, 60)
	b.health(2500, crow, 70)
	l, _, _ := audited(t, s.logs(5000)...)
	if got := healths(l); !sameHealths(got, [2]uint64{1500, 80}, [2]uint64{2800, 60}) {
		t.Errorf("health updates = %v", got)
	}

	s, a, b = handover()
	a.health(2800, crow, 60)
	b.health(2810, crow, 60)
	l, _, _ = audited(t, s.logs(5000)...)
	if got := healths(l); !sameHealths(got, [2]uint64{2800, 60}) {
		t.Errorf("health updates = %v", got)
	}
}

// TestCarryParity checks that a state carried over keeps the parity of
// the time it was told at.
func TestCarryParity(t *testing.T) {
	s, _, b := handover()
	b.health(2900, crow, 55)
	l, _, _ := audited(t, s.logs(5000)...)
	if got := healths(l); !sameHealths(got, [2]uint64{3002, 55}) {
		t.Errorf("health updates = %v", got)
	}
}

// TestCarrySliver checks that nothing is carried over into what an entry
// holds alone for an instant after the others: the time its client took
// to learn that the agent left.
func TestCarrySliver(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) { r.npc(crow, 1083, "Crow") })
	a.spawn(1000, crow)
	a.despawn(3000, crow)
	b.spawn(1600, crow)
	b.health(2900, crow, 55)
	b.despawn(3060, crow)
	l, _, m := audited(t, s.logs(5000)...)
	if got := healths(l); len(got) != 0 {
		t.Errorf("health updates = %v", got)
	}
	x := m.recs[0].entity[m.recs[0].tl.Agent(a.addr(crow))]
	if len(x.speak) != 2 || x.speak[1].iv != iv(3001, 3060) {
		t.Errorf("speakers = %+v", x.speak)
	}
}

// TestCarryWas checks what a state was before the first change a client
// tells of an agent that came back: the timeline reads it from that
// change.
func TestCarryWas(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	// A cannon that never moves, met by one client then by the other.
	s.each(func(r *recorder) { r.gadget(cannon, 7307, "Cannon") })
	a.spawn(1000, cannon)
	a.position(1100, cannon, 50, 50)
	a.team(1200, cannon, 7, 0)
	a.despawn(2000, cannon)
	b.spawn(3000, cannon)
	b.position(3100, cannon, 50, 50)
	b.team(3500, cannon, 5, 3)
	b.despawn(4500, cannon)
	l, tl, _ := audited(t, s.logs(5000)...)
	var got [][3]uint64
	for _, e := range of(l, evtc.StateTeamChange) {
		if e.SrcAgent == a.addr(cannon) && e.DstAgent != 0 {
			got = append(got, [3]uint64{e.Time - rawEpoch, e.DstAgent, uint64(e.Value)})
		}
	}
	if len(got) != 3 || got[0] != [3]uint64{1200, 7, 0} || got[1] != [3]uint64{3000, 3, 0} || got[2] != [3]uint64{3500, 5, 3} {
		t.Errorf("team changes = %v", got)
	}
	c := agentsNamed(tl, "Cannon")
	if len(c) != 1 {
		t.Fatalf("%d cannons", len(c))
	}
	for _, tt := range []struct {
		at   int
		want uint32
	}{{1500, 7}, {3200, 3}, {3600, 5}} {
		if got := c[0].TeamAt(time.Duration(tt.at) * time.Millisecond); got != tt.want {
			t.Errorf("team at %d ms = %d, want %d", tt.at, got, tt.want)
		}
	}
}

// TestCarryMarkers checks that the markers of an agent are carried over
// as a whole.
func TestCarryMarkers(t *testing.T) {
	s, _, b := handover()
	// Two markers put at once, the first event taking off what was worn.
	b.marker(2500, crow, 0)
	b.marker(2500, crow, 3)
	b.marker(2500, crow, 4)
	l, _, _ := audited(t, s.logs(5000)...)
	var got []int32
	for _, e := range of(l, evtc.StateMarker) {
		if e.Time != rawEpoch+3002 {
			t.Errorf("marker %d at %d", e.Value, e.Time-rawEpoch)
		}
		got = append(got, e.Value)
	}
	if len(got) != 3 || got[0] != 0 || got[1] != 3 || got[2] != 4 {
		t.Errorf("markers = %v", got)
	}
}

func TestTell(t *testing.T) {
	m := &combiner{tellings: map[int32]map[slot][]told{}}
	m.tell(cand{entry: 3, slot: slotHealth, t: time.Second})
	m.tell(cand{entry: 3, slot: slotHealth, t: 2 * time.Second})
	m.tell(cand{entry: 3, slot: slotPosition, t: time.Second})
	m.tell(cand{entry: 0, slot: slotHealth, t: time.Second})
	if len(m.tellings) != 1 || len(m.tellings[3]) != 1 || len(m.tellings[3][slotHealth]) != 2 {
		t.Errorf("tellings = %v", m.tellings)
	}
}

// TestCarrySegments checks the segments that carry nothing over, and the
// parity of a time at the end of a segment of one instant.
func TestCarrySegments(t *testing.T) {
	health := func(entry int32, at int) told {
		t := time.Duration(at) * time.Millisecond
		return told{t, cand{entry: entry, slot: slotHealth, t: t, e: evtc.Event{Time: uint64(at)}}}
	}
	m := &combiner{
		recs: []*recording{{}},
		tellings: map[int32]map[slot][]told{
			1: {slotHealth: {health(1, 100)}},
			2: {slotHealth: {health(2, 100)}},
		},
		ents: []*entity{{speak: []segment{
			{iv(0, 999), 0, 1},
			// The same entry goes on: nothing changes hands.
			{iv(1000, 1999), 0, 1},
			// A segment of one instant: the time moves back to keep its
			// parity.
			{iv(2001, 2001), 1, 2},
			{iv(2002, 5000), 0, 1},
		}}},
	}
	got := m.carry()
	if len(got) != 2 {
		t.Fatalf("%d states carried over", len(got))
	}
	if got[0].entry != 2 || got[0].t != 2*time.Second || got[0].e.Time != 2000 || !got[0].carried || got[0].told != 100 {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].entry != 1 || got[1].t != 2002*time.Millisecond {
		t.Errorf("second = %+v", got[1])
	}
}
