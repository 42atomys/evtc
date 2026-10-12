package combine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/extensions/healingstats"
	"github.com/42atomys/evtc/timeline"
)

// samplePath is the real log used by the integration tests; they are
// skipped when it is absent.
const samplePath = "../tests_fixtures/sabetha-05-fd9b6f3a.zevtc"

// recordingsPath holds four recordings of one Deimos fight of five
// players, by four of them; the tests that read it are skipped when the
// folder is absent.
const recordingsPath = "../tests_fixtures/deimos-multi-pov"

func loadSample(tb testing.TB) *timeline.Timeline {
	tb.Helper()
	if _, err := os.Stat(samplePath); err != nil {
		tb.Skipf("%s not available", samplePath)
	}
	tl, err := timeline.ParseFile(samplePath)
	if err != nil {
		tb.Fatal(err)
	}
	return tl
}

// loadRecordings reads the logs of recordingsPath, in the order of their
// names.
func loadRecordings(tb testing.TB) []*evtc.Log {
	tb.Helper()
	paths, _ := filepath.Glob(filepath.Join(recordingsPath, "*.zevtc"))
	if len(paths) < 2 {
		tb.Skipf("%s holds no recordings", recordingsPath)
	}
	slices.Sort(paths)
	logs := make([]*evtc.Log, len(paths))
	for i, p := range paths {
		l, err := evtc.ParseFile(p)
		if err != nil {
			tb.Fatal(err)
		}
		logs[i] = l
	}
	return logs
}

// addressChanges counts the address changes of a log, which a combined
// log does not hold.
func addressChanges(l *evtc.Log) int { return count(l, evtc.StateIIDChange) }

// TestSampleItself checks that the sample combined with itself comes
// back as it is.
func TestSampleItself(t *testing.T) {
	tl := loadSample(t)
	l, got, m := audited(t, tl.Log, tl.Log)
	checkInvariants(t, l, got, m)
	want := totalsOf(tl)
	want.events -= addressChanges(tl.Log)
	if have := totalsOf(got); have != want {
		t.Errorf("totals = %+v, want %+v", have, want)
	}
	if len(l.Agents) != len(tl.Log.Agents) || len(l.Skills) != len(tl.Log.Skills) || l.Header != tl.Log.Header {
		t.Errorf("%d agents and %d skills, want %d and %d", len(l.Agents), len(l.Skills), len(tl.Log.Agents), len(tl.Log.Skills))
	}
	if c := states(got, m); len(c.notes) != 0 {
		compared, differ := c.total()
		t.Errorf("%d states of %d differ: %v", differ, compared, c.notes)
	}
	if compared, _ := sameEvents(t, tl, l, holding{ordered: true}); compared != len(l.Events) {
		t.Errorf("%d events compared of %d", compared, len(l.Events))
	}
}

// TestSampleStoppedEarly combines the sample, written as another client
// would, with what a client that stopped at three fifths of the fight
// holds of it: the whole fight comes back, whichever of the two is read
// first.
func TestSampleStoppedEarly(t *testing.T) {
	tl := loadSample(t)
	at := tl.Duration * 3 / 5
	short := cut(tl, at)
	other := moved(tl, 0x5a5a_0000_5a5a, 7_654_322)
	want := totalsOf(tl)
	for i, logs := range [][]*evtc.Log{{short, other}, {other, short}} {
		l, got, m := audited(t, logs...)
		checkInvariants(t, l, got, m)
		have := totalsOf(got)
		// sameEvents counts the events.
		have.events = want.events
		if have != want {
			t.Errorf("totals = %+v, want %+v", have, want)
		}
		if n := toldTwice(l, m, 100*time.Millisecond); n != 0 {
			t.Errorf("%d events told twice", n)
		}
		// The log of the whole fight, as the recording read first names
		// and dates it.
		if i == 0 {
			sameEvents(t, tl, l, holding{left: lost(tl, at), first: m.recs[0].tl, mask: 0x5a5a_0000_5a5a})
		} else {
			sameEvents(t, m.recs[0].tl, l, holding{ordered: true})
		}
	}
	got, err := Logs(short, other)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != len(tl.Log.Events)-addressChanges(tl.Log) || got.Agents[0].Addr != other.Agents[0].Addr {
		t.Errorf("%d events, read from %#x", len(got.Events), got.Agents[0].Addr)
	}
}

// heals are the counted heals of a timeline, as the healing stats addon
// wrote them: twice counts those the clients of both parties wrote.
type heals struct {
	heals, twice    int
	healed, barrier int64
}

func healsOf(tl *timeline.Timeline) heals {
	h := healingstats.Of(tl)
	if h == nil {
		return heals{}
	}
	all := h.Heals()
	out := heals{heals: all.Count(), healed: all.Healed(), barrier: all.BarrierGiven()}
	for x := range all.Seq() {
		if x.PeerEvent != nil {
			out.twice++
		}
	}
	return out
}

// TestSampleHeals checks that the events of an extension come back as
// they were written: the heals of the sample combined with itself, or with
// what a client that stopped early holds of it, are those of the sample.
func TestSampleHeals(t *testing.T) {
	tl := loadSample(t)
	want := healsOf(tl)
	if want.heals == 0 {
		t.Fatal("the sample holds no heals")
	}
	short := cut(tl, tl.Duration*3/5)
	other := moved(tl, 0x5a5a_0000_5a5a, 7_654_322)
	for _, logs := range [][]*evtc.Log{{tl.Log, tl.Log}, {short, other}, {other, short}} {
		_, got, _ := audited(t, logs...)
		if have := healsOf(got); have != want {
			t.Errorf("heals = %+v, want %+v", have, want)
		}
	}
}
