package combine

import (
	"errors"
	"path/filepath"
	"testing"

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
