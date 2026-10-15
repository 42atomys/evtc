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

// TestRecordings combines the recordings of recordingsPath: all together,
// in any order, and two by two.
func TestRecordings(t *testing.T) {
	logs := loadRecordings(t)
	all, err := Logs(logs...)
	if err != nil {
		t.Fatal(err)
	}
	back := slices.Clone(logs)
	slices.Reverse(back)
	if again, err := Logs(back...); err != nil || !sameLog(all, again) {
		t.Errorf("the order given matters: %v", err)
	}
	tl, err := timeline.Build(all)
	if err != nil {
		t.Fatal(err)
	}
	got := totalsOf(tl)
	want := totals{
		players: 5, npcs: 549, gadgets: 275, strikes: 3289, strikeDamage: 8442761, casts: 1696, stacks: 14270,
		effects: 8808, missiles: 912, deaths: 247, markers: 4, groundMarkers: 1,
	}
	got.events, got.ticks, got.positions, got.rewards = 0, 0, 0, 0
	if got != want {
		t.Errorf("totals = %+v, want %+v", got, want)
	}
	if boss := tl.Boss(); boss == nil || tl.Hits().On(boss).Strikes().Count() != 1331 {
		t.Errorf("boss = %v", boss)
	}
	// Three of the four clients ran the healing stats addon. A heal two
	// of them wrote is one heal, whatever the log that holds each record.
	if got, want := healsOf(tl), (heals{heals: 1106, twice: 101, healed: 236375, barrier: 771436}); got != want {
		t.Errorf("heals = %+v, want %+v", got, want)
	}

	outOfOrder := 0
	for i := range logs {
		for j := range logs {
			if i == j {
				continue
			}
			l, tl, m := audited(t, logs[i], logs[j])
			checkInvariants(t, l, tl, m)
			outOfOrder += inversions(m)
			if n := toldTwice(l, m, 100*time.Millisecond); n > 2 {
				t.Errorf("recordings %d and %d: %d events told twice", i, j, n)
			}
			c := states(tl, m)
			if compared, differ := c.total(); differ > 2 || compared < 1_500_000 {
				t.Errorf("recordings %d and %d: %d states of %d differ: %v", i, j, differ, compared, c.notes)
			}
			for _, single := range []*evtc.Log{logs[i], logs[j]} {
				one, err := timeline.Build(single)
				if err != nil {
					t.Fatal(err)
				}
				if a, b := totalsOf(one), totalsOf(tl); b.strikes < a.strikes || named(tl) < named(one) || b.effects < a.effects || b.missiles < a.missiles || b.players != a.players {
					t.Errorf("recordings %d and %d hold less than one of them: %+v and %+v", i, j, b, a)
				}
			}
		}
	}
	// The pairs of messages of the server the two recordings tell in
	// another order: two messages within 20 ms of each other for most, a
	// pairing that moves changes the count.
	if outOfOrder != 198 {
		t.Errorf("%d pairs of messages of the server out of order, want 198", outOfOrder)
	}
}

// TestRecordingsByTicks checks the clock set by the ticks of the server on
// real recordings: the first Deimos recording with another one whose
// strikes are dropped, as if its recorder had stood too far to see them,
// gets the offset the strikes give within the jitter of a client.
func TestRecordingsByTicks(t *testing.T) {
	logs := loadRecordings(t)
	for i := 1; i < len(logs); i++ {
		byStrikes := mustRecord(t, logs[0], logs[i])
		far := *logs[i]
		far.Events = nil
		for _, e := range logs[i].Events {
			if e.IsStateChange != evtc.StateCombat || e.Buff != 0 {
				far.Events = append(far.Events, e)
			}
		}
		byTicks := mustRecord(t, logs[0], &far)
		if d := (byTicks[1].off - byStrikes[1].off).Abs(); d > 20*time.Millisecond {
			t.Errorf("recording %d: offset %v by the ticks, %v by the strikes", i, byTicks[1].off, byStrikes[1].off)
		}
		if _, n := offsetOf(strikes(byTicks[0]), strikes(byTicks[1])); n != 0 {
			t.Errorf("recording %d still shares %d strikes", i, n)
		}
	}
}
