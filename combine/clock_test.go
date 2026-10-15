package combine

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

// mustRecord builds the recordings of the logs, their clocks set.
func mustRecord(tb testing.TB, logs ...*evtc.Log) []*recording {
	tb.Helper()
	recs, err := recordings(logs)
	if err != nil {
		tb.Fatal(err)
	}
	return recs
}

func TestOffsetOf(t *testing.T) {
	a := map[strike][]time.Duration{
		{skill: 1}: ms(100), {skill: 2}: ms(200), {skill: 3}: ms(300),
		{skill: 4}: ms(400, 450), // told twice by a: no gap
		{skill: 5}: ms(500),      // told twice by b
		{skill: 6}: ms(600),      // missing from b
	}
	b := map[strike][]time.Duration{
		{skill: 1}: ms(110), {skill: 2}: ms(212), {skill: 3}: ms(340),
		{skill: 4}: ms(410), {skill: 5}: ms(510, 520),
	}
	if off, n := offsetOf(a, b); off != 12*time.Millisecond || n != 3 {
		t.Errorf("offset = %v over %d strikes, want the median 12ms over 3", off, n)
	}
	if off, n := offsetOf(a, nil); off != 0 || n != 0 {
		t.Errorf("offset = %v over %d strikes, want none", off, n)
	}

	// Strikes two fights share by chance tell gaps far apart: those that
	// stand together count.
	a, b = map[strike][]time.Duration{}, map[strike][]time.Duration{}
	for i, gap := range []int{-200_000, -90_000, -40_000, 1480, 1500, 1500, 1510, 3400, 60_000, 61_000, 150_000, 400_000} {
		a[strike{skill: uint32(i)}] = ms(1000 * i)
		b[strike{skill: uint32(i)}] = ms(1000*i + gap)
	}
	if off, n := offsetOf(a, b); off != 1500*time.Millisecond || n != 5 {
		t.Errorf("offset = %v over %d strikes, want 1.5s over 5", off, n)
	}
}

// TestSameStart checks that two attempts of a fight are told apart by the
// clock of the server: the same strikes a few seconds into each, minutes
// apart.
func TestSameStart(t *testing.T) {
	first := newScene(1).logs(5000)[0]
	for _, tt := range []struct {
		later int32
		same  bool
	}{{0, true}, {4, true}, {-5, true}, {6, false}, {600, false}} {
		second := newScene(1).logs(5000)[0]
		for i := range second.Events {
			if e := &second.Events[i]; e.IsStateChange == evtc.StateSquadCombatStart || e.IsStateChange == evtc.StateSquadCombatEnd {
				e.Value += tt.later
			}
		}
		_, err := Logs(first, second)
		if got := err == nil; got != tt.same || (err != nil && !errors.Is(err, ErrFightMismatch)) {
			t.Errorf("started %d s later: err = %v", tt.later, err)
		}
	}
	// A log without the time of the server is set on its strikes alone.
	blind := newScene(1).logs(5000)[0]
	blind.Events = blind.Events[1:]
	recs := mustRecord(t, first, blind)
	if !recs[1].tl.Start.IsZero() || recs[1].off != 0 {
		t.Errorf("start %v, offset %v", recs[1].tl.Start, recs[1].off)
	}
}

// TestSetClocks checks the offset of a recording that started later than
// the first one, whatever the clock of its client.
func TestSetClocks(t *testing.T) {
	logs := newScene(2).logs(5000)
	// The second client starts its squad combat 500 ms into the fight.
	logs[1].Events[0].Time += 500
	recs := mustRecord(t, logs...)
	if recs[0].off != 0 || recs[1].off != -500*time.Millisecond {
		t.Errorf("offsets = %v and %v, want 0s and -500ms", recs[0].off, recs[1].off)
	}
	if want := -recs[1].off; recs[1].span.Start != want || recs[1].span.End != recs[1].tl.Duration+want {
		t.Errorf("span = %v", recs[1].span)
	}
	// The same instant of the fight, read from both logs.
	a, b := of(logs[0], evtc.StateCombat)[7], of(logs[1], evtc.StateCombat)[7]
	if ta, tb := recs[0].at(&a), recs[1].at(&b); ta != tb || ta != 450*time.Millisecond {
		t.Errorf("the same strike at %v and %v", ta, tb)
	}
}

// TestSetClocksParity checks that a time keeps its parity: arcdps 20260920
// tells what comes from the server by an even time.
func TestSetClocksParity(t *testing.T) {
	for _, shift := range []uint64{0, 1, 2, 7_654_321, 7_654_322} {
		s := newScene(2)
		s.recs[1].shift = shift
		// Written again on the clock given.
		r := s.recs[1]
		for i := range r.l.Events {
			r.l.Events[i].Time += shift - 7_654_322
		}
		r.position(2001, bravo, 1, 2)
		logs := s.logs(5000)
		recs := mustRecord(t, logs...)
		if d := recs[0].epoch - recs[1].epoch - recs[1].off.Milliseconds(); d%2 != 0 {
			t.Errorf("shift %d: times change parity", shift)
		}
		l, _, m := audited(t, logs...)
		for i, e := range l.Events {
			if e.IsStateChange == evtc.StatePosition && m.audit.origin[i] == 1 && e.Time%2 != (rawEpoch+shift+2001)%2 {
				t.Errorf("shift %d: a position written at an odd time is at %d", shift, e.Time)
			}
		}
	}
}

// TestSetClocksThrough checks that a recording that shares nothing with
// the first one is set on one that does.
func TestSetClocksThrough(t *testing.T) {
	s := newScene(3)
	// The third client tells none of the first strikes.
	c := s.recs[2]
	kept := c.l.Events[:0]
	for _, e := range c.l.Events {
		if e.IsStateChange != evtc.StateCombat {
			kept = append(kept, e)
		}
	}
	c.l.Events = kept
	for i := range 30 {
		s.recs[1].hit(uint64(3000+40*i), bravo, boss, skillSlam, int32(2000+i))
		c.hit(uint64(3000+40*i), bravo, boss, skillSlam, int32(2000+i))
	}
	logs := s.logs(5000)
	logs[2].Events[0].Time += 300
	recs := mustRecord(t, logs...)
	if recs[1].off != 0 || recs[2].off != -300*time.Millisecond {
		t.Errorf("offsets = %v and %v, want 0s and -300ms", recs[1].off, recs[2].off)
	}

	// Without the second one, nothing sets the clock of the third.
	if _, err := recordings([]*evtc.Log{logs[0], logs[2]}); !errors.Is(err, ErrFightMismatch) {
		t.Errorf("err = %v, want %v", err, ErrFightMismatch)
	}
}

// TestMinStrikes checks the number of strikes two recordings must share.
func TestMinStrikes(t *testing.T) {
	for _, n := range []int{MinStrikes - 1, MinStrikes} {
		s := newScene(2)
		b := s.recs[1]
		kept, strikes := b.l.Events[:0], 0
		for _, e := range b.l.Events {
			if e.IsStateChange == evtc.StateCombat {
				if strikes++; strikes > n {
					continue
				}
			}
			kept = append(kept, e)
		}
		b.l.Events = kept
		_, err := recordings(s.logs(5000))
		if got := err == nil; got != (n >= MinStrikes) {
			t.Errorf("%d strikes shared: err = %v", n, err)
		}
	}
}

func TestEpoch(t *testing.T) {
	logs := newScene(2).logs(5000)
	recs := mustRecord(t, logs...)
	if recs[0].epoch != rawEpoch || recs[1].epoch != rawEpoch+7_654_322 {
		t.Errorf("epochs = %d and %d", recs[0].epoch, recs[1].epoch)
	}
	empty := mustRecord(t, &evtc.Log{Header: logs[0].Header})
	if empty[0].epoch != 0 {
		t.Errorf("epoch of an empty log = %d", empty[0].epoch)
	}
}

// ticked writes the start of the instance and one tick a second in every
// log of the scene: the instance started a minute before the fight, the
// second client names each tick one later than the first, 40 ms after it.
func ticked(s *scene, end uint64, socket uint32) {
	for i, r := range s.recs {
		r.instance(0, 60_000, socket)
		for t := uint64(500); t < end; t += 1000 {
			r.serverTick(t+uint64(40*i), 1500+t/40+uint64(i), 30)
		}
	}
}

// unseen drops the strikes of the second client of a scene: it stood too
// far from the boss to see them.
func unseen(s *scene) {
	b := s.recs[1]
	kept := b.l.Events[:0]
	for _, e := range b.l.Events {
		if e.IsStateChange != evtc.StateCombat {
			kept = append(kept, e)
		}
	}
	b.l.Events = kept
}

// TestSetClocksByTicks checks the clock of a recording that shares no
// strike with the first one: set by the ticks of the server when both were
// made in the same instance.
func TestSetClocksByTicks(t *testing.T) {
	s := newScene(2)
	unseen(s)
	ticked(s, 30_000, 7)
	logs := s.logs(30_000)
	// The second client starts its squad combat 500 ms into the fight.
	logs[1].Events[0].Time += 500
	recs := mustRecord(t, logs...)
	if recs[0].off != 0 || recs[1].off != -500*time.Millisecond {
		t.Errorf("offsets = %v and %v, want 0s and -500ms", recs[0].off, recs[1].off)
	}
	l, tl, m := audited(t, logs...)
	checkInvariants(t, l, tl, m)
	if n := of(l, evtc.StateTick); len(n) != 30 || of(l, evtc.StateInstanceStart)[0].Value != 7 {
		t.Errorf("%d ticks in the combined log", len(n))
	}

	for _, tt := range []struct {
		name   string
		change func(s *scene)
		want   string
	}{
		{"another socket", func(s *scene) { s.recs[1].instance(0, 60_000, 8) }, "0 strikes shared, 20 wanted"},
		{"another instance", func(s *scene) { s.recs[1].instance(0, 20_000, 7) }, "0 strikes shared, 20 wanted"},
		{"too few ticks", func(s *scene) {
			b := s.recs[1]
			kept, n := b.l.Events[:0], 0
			for _, e := range b.l.Events {
				if e.IsStateChange == evtc.StateTick {
					if n++; n > MinTicks-1 {
						continue
					}
				}
				kept = append(kept, e)
			}
			b.l.Events = kept
		}, "0 strikes and 18 ticks shared, 20 and 20 wanted"},
	} {
		s := newScene(2)
		unseen(s)
		// The instance start of the first client, then the change.
		s.recs[0].instance(0, 60_000, 7)
		if tt.name != "another socket" && tt.name != "another instance" {
			s.recs[1].instance(0, 60_000, 7)
		}
		for i, r := range s.recs {
			for u := uint64(500); u < 30_000; u += 1000 {
				r.serverTick(u+uint64(40*i), 1500+u/40+uint64(i), 30)
			}
		}
		tt.change(s)
		_, err := recordings(s.logs(30_000))
		if !errors.Is(err, ErrFightMismatch) || !strings.HasSuffix(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.want)
		}
	}
}

func TestTickOffset(t *testing.T) {
	ms := func(v int) time.Duration { return time.Duration(v) * time.Millisecond }
	a := ticks{tick: []int64{100, 125, 150, 175}, at: []time.Duration{ms(1000), ms(2000), ms(3000), ms(4000)}}
	// The second recording names the ticks one later, 40 ms after, and
	// runs 12 ms behind.
	b := ticks{tick: []int64{101, 126, 151, 176}, at: []time.Duration{ms(1052), ms(2052), ms(3052), ms(4052)}}
	if off, n := tickOffset(a, b); off != 12*time.Millisecond || n != 3 {
		t.Errorf("offset = %v over %d ticks, want 12ms over 3", off, n)
	}
	if off, n := tickOffset(a, ticks{}); off != 0 || n != 0 {
		t.Errorf("offset = %v over %d ticks, want none", off, n)
	}
	// A hole wider than two events is not crossed.
	c := ticks{tick: []int64{100, 200}, at: []time.Duration{ms(1000), ms(5000)}}
	if _, ok := c.timeAt(150); ok {
		t.Error("a tick is placed across a hole of 100 ticks")
	}
	if at, ok := c.timeAt(200); !ok || at != ms(5000) {
		t.Errorf("the time of a tick named = %v, %v", at, ok)
	}
}
