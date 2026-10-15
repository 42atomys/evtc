package combine

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

func TestLogsErrors(t *testing.T) {
	logs := newScene(2).logs(5000)
	other := newScene(1)
	other.recs[0].l.Header.Build = "20260816"
	old := newScene(1)
	old.recs[0].l.Header.Build = "20230114"
	// A fight of which nothing is shared: other strikes.
	apart := newScene(1)
	for i := range apart.recs[0].l.Events {
		if e := &apart.recs[0].l.Events[i]; e.IsStateChange == evtc.StateCombat {
			e.Value += 5000
		}
	}
	tests := []struct {
		name string
		logs []*evtc.Log
		want error
	}{
		{"no log", nil, ErrNoLog},
		{"a nil log", []*evtc.Log{logs[0], nil}, ErrNoLog},
		{"two builds", []*evtc.Log{logs[0], other.logs(5000)[0]}, ErrBuildMismatch},
		{"a build the timeline refuses", []*evtc.Log{old.logs(5000)[0], old.logs(5000)[0]}, timeline.ErrLegacyLog},
		{"two fights", []*evtc.Log{logs[0], apart.logs(5000)[0]}, ErrFightMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := Logs(tt.logs...)
			if !errors.Is(err, tt.want) || l != nil {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}

	many := make([]*evtc.Log, MaxLogs+1)
	for i := range many {
		many[i] = logs[0]
	}
	if _, err := Logs(many...); !errors.Is(err, ErrTooManyLogs) {
		t.Errorf("err = %v, want %v", err, ErrTooManyLogs)
	}
}

func TestLogsSingle(t *testing.T) {
	l := newScene(1).logs(5000)[0]
	got, err := Logs(l)
	if err != nil || got != l {
		t.Errorf("a single log = %p, %v, want the log itself", got, err)
	}
}

// TestLogsOrder checks that the order given does not matter: the log that
// holds the most events is read first.
func TestLogsOrder(t *testing.T) {
	s := newScene(3)
	s.recs[1].hit(2500, bravo, boss, skillSlam, 77)
	s.recs[2].hit(2600, charlie, boss, skillSlam, 78)
	s.recs[2].hit(2700, charlie, boss, skillSlam, 79)
	logs := s.logs(5000)
	want, err := Logs(logs[0], logs[1], logs[2])
	if err != nil {
		t.Fatal(err)
	}
	for _, order := range [][]int{{0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		got, err := Logs(logs[order[0]], logs[order[1]], logs[order[2]])
		if err != nil {
			t.Fatal(err)
		}
		if !sameLog(got, want) {
			t.Errorf("order %v gives another log", order)
		}
	}
	// The third client holds the most events: its point of view is kept.
	tl, err := timeline.Build(want)
	if err != nil {
		t.Fatal(err)
	}
	if tl.POV == nil || tl.POV.Account != "Charlie.9012" {
		t.Errorf("point of view = %v", tl.POV)
	}
	if got := ranked(logs); got[0] != logs[2] || got[1] != logs[1] || got[2] != logs[0] {
		t.Error("the logs are not read by decreasing number of events")
	}
}

// TestRankedTies checks that logs holding as many events are told apart
// by what they hold.
func TestRankedTies(t *testing.T) {
	s := newScene(2)
	s.recs[0].hit(2500, alpha, boss, skillSlam, 77)
	s.recs[1].hit(2500, bravo, boss, skillSlam, 78)
	logs := s.logs(5000)
	if len(logs[0].Events) != len(logs[1].Events) {
		t.Fatalf("events = %d and %d", len(logs[0].Events), len(logs[1].Events))
	}
	a, b := ranked([]*evtc.Log{logs[0], logs[1]}), ranked([]*evtc.Log{logs[1], logs[0]})
	if a[0] != b[0] || a[1] != b[1] {
		t.Error("the order given decides between two logs of the same size")
	}
	// The one that lists the most agents comes first.
	s.recs[0].npc(crow, 1083, "Crow")
	if got := ranked([]*evtc.Log{logs[1], logs[0]}); got[0] != logs[0] {
		t.Error("the log that lists the most agents does not come first")
	}
	if got := ranked([]*evtc.Log{logs[0], logs[0]}); got[0] != logs[0] || got[1] != logs[0] {
		t.Error("a log given twice")
	}
}

// TestLogsLeaveInputs checks that the logs given are not modified.
func TestLogsLeaveInputs(t *testing.T) {
	logs := newScene(2).logs(5000)
	before := []*evtc.Log{clone(logs[0]), clone(logs[1])}
	if _, err := Logs(logs...); err != nil {
		t.Fatal(err)
	}
	for i := range logs {
		if !sameLog(logs[i], before[i]) {
			t.Errorf("log %d was modified", i)
		}
	}
}

// TestLogsItself checks that a log combined with itself comes back as it
// is.
func TestLogsItself(t *testing.T) {
	s := newScene(1)
	r := s.recs[0]
	r.npc(crow, 1083, "Crow")
	r.spawn(1000, crow)
	r.position(1100, crow, 10, 20)
	r.hit(1200, alpha, crow, skillSlam, 500)
	r.apply(1300, alpha, bravo, skillMight, 10000, 1)
	r.remove(2300, bravo, skillMight, 1)
	r.despawn(3000, crow)
	l := s.logs(5000)[0]
	got, err := Logs(l, l)
	if err != nil {
		t.Fatal(err)
	}
	want, err := timeline.Build(l)
	if err != nil {
		t.Fatal(err)
	}
	tl, err := timeline.Build(got)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := totalsOf(want), totalsOf(tl); a != b {
		t.Errorf("totals = %+v, want %+v", b, a)
	}
	if !sameLog(got, mustSorted(l)) {
		t.Error("the log does not come back as it is")
	}
}

// mustSorted returns a log with its events in time order, the
// definitions first, as a combined log holds them.
func mustSorted(l *evtc.Log) *evtc.Log {
	out := clone(l)
	sortEvents(out.Events)
	return out
}

func TestFiles(t *testing.T) {
	logs := newScene(2).logs(5000)
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "alpha.zevtc"), filepath.Join(dir, "bravo.evtc")}
	for i, p := range paths {
		if err := logs[i].WriteFile(p); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Files(paths...)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Logs(logs...)
	if err != nil {
		t.Fatal(err)
	}
	if !sameLog(got, want) {
		t.Error("the files give another log than the logs they hold")
	}
	if _, err := Files(paths[0], filepath.Join(dir, "missing.zevtc")); err == nil {
		t.Error("a missing file was combined")
	}
}

// TestGenerated combines the recordings of generated fights, spread over
// more than a client sees, and checks the combined log against what
// happened: every strike, every death and every buff stack a client at
// least was told of is in it once, every creature once per time the
// clients met it, and the states are those of the recording that speaks.
func TestGenerated(t *testing.T) {
	for seed := range uint64(40) {
		o := genOptions{recorders: 2 + int(seed%3), players: 5, adds: 40, duration: time.Minute, seed: seed}
		logs, truth := genFight(o)
		l, tl, m := audited(t, ranked(logs)...)
		checkInvariants(t, l, tl, m)
		got := totalsOf(tl)
		if got.strikes != truth.strikes || got.stacks != truth.stacks || got.deaths != truth.deaths || got.npcs != truth.creatures+truth.apart || got.players != o.players {
			t.Errorf("seed %d: %d strikes, %d stacks, %d deaths, %d creatures, want %d, %d, %d and %d", seed,
				got.strikes, got.stacks, got.deaths, got.npcs, truth.strikes, truth.stacks, truth.deaths, truth.creatures+truth.apart)
		}
		var held time.Duration
		for a := range tl.NPCs().Seq() {
			held += a.Lifetime.Duration()
		}
		if want := time.Duration(truth.held) * time.Millisecond; held < want-time.Second || held > want+time.Second {
			t.Errorf("seed %d: creatures held for %v, want %v", seed, held, want)
		}
		if n := toldTwice(l, m, 100*time.Millisecond); n != 0 {
			t.Errorf("seed %d: %d events told twice", seed, n)
		}
		if c := states(tl, m); len(c.notes) != 0 {
			t.Errorf("seed %d: states differ: %v", seed, c.notes)
		}
		// A client alone holds less than the clients together.
		alone, err := timeline.Build(logs[0])
		if err != nil {
			t.Fatal(err)
		}
		if one := totalsOf(alone); one.strikes >= got.strikes || one.npcs >= got.npcs {
			t.Errorf("seed %d: a client holds %d strikes and %d creatures of %d and %d", seed, one.strikes, one.npcs, got.strikes, got.npcs)
		}
	}
}

// TestLaterBuild checks the logs of an arcdps build later than the
// package knows: they are combined as the others, and the events of a
// kind it has no rule for come from the first recording.
func TestLaterBuild(t *testing.T) {
	s := newScene(2)
	later := evtc.StateUnknown + 3
	s.each(func(r *recorder) {
		r.l.Header.Build = "20301231"
		r.ev(2000, later, alpha, 0, evtc.Event{Value: 7})
		r.hit(2500, bravo, boss, skillSlam, 700)
	})
	s.recs[1].ev(2100, later, bravo, 0, evtc.Event{Value: 8})
	logs := s.logs(5000)
	l, tl, _ := audited(t, logs...)
	got := of(l, later)
	if len(got) != 1 || got[0] != of(logs[0], later)[0] {
		t.Errorf("events of the later kind = %+v", got)
	}
	if n := tl.Hits().On(tl.Boss()).Count(); n != 41 || l.Header.Build != "20301231" {
		t.Errorf("%d hits on the boss, build %s", n, l.Header.Build)
	}
}

// TestOlderFormat checks the logs written before arcdps 20260501, whose
// casts and buff events are StateCombat events: they are paired as those
// of the typed format.
func TestOlderFormat(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) {
		r.ev(1000, evtc.StateAnimationStart, alpha, boss, evtc.Event{SkillID: skillSlam, Value: 500})
		r.ev(1500, evtc.StateAnimationStop, alpha, 0, evtc.Event{SkillID: skillSlam, Value: 500})
		r.apply(2000, alpha, charlie, skillMight, 9000, 5)
		r.remove(3000, charlie, skillMight, 5)
	})
	a.ev(3500, evtc.StateAnimationStart, alpha, boss, evtc.Event{SkillID: skillHeat, Value: 300})
	b.apply(3600, bravo, charlie, skillRegen, 4000, 6)
	b.ev(4000, evtc.StateBuffRemoveAll, charlie, 0, evtc.Event{SkillID: skillRegen, IsBuffRemove: evtc.BuffRemoveAll})
	logs := s.logs(5000)
	typed, tl, _ := audited(t, logs...)
	l, old, m := audited(t, older(logs[0]), older(logs[1]))
	if m.recs[0].typed || m.recs[1].typed || l.Header.Build != "20250913" {
		t.Fatalf("typed = %v and %v, build %s", m.recs[0].typed, m.recs[1].typed, l.Header.Build)
	}
	if len(l.Events) != len(typed.Events) || count(l, evtc.StateCombat) != count(typed, evtc.StateCombat)+7 {
		t.Errorf("%d events, %d of them combat events; %d in the typed format", len(l.Events), count(l, evtc.StateCombat), len(typed.Events))
	}
	if a, b := totalsOf(old), totalsOf(tl); a != b || a.casts != 2 || a.stacks != 2 {
		t.Errorf("totals = %+v, in the typed format %+v", a, b)
	}
}
