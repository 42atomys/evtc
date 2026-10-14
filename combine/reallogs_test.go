package combine

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// recorded is what TestRealLogs keeps of a log to find the recordings of
// one fight.
type recorded struct {
	path, name, build, account string
	mapID                      uint32
	start, end                 time.Time
}

// TestRealLogs runs, on every log under ../tests_fixtures or under the
// folder EVTC_REAL_LOGS names, the two checks a log alone allows: combined
// with itself it must come back as it is, and combined with what a client
// that stopped at three fifths of the fight would hold of it, under other
// addresses and another clock, it must come back as well, whichever of
// the two is read first. The logs of the folder that recorded the same
// fight are then combined with each other. The report, printed with -v,
// tells what each run held. The test runs only with EVTC_REAL_LOGS set,
// as it reads hundreds of megabytes of logs, several at a time: -parallel
// bounds how many.
func TestRealLogs(t *testing.T) {
	dir := os.Getenv("EVTC_REAL_LOGS")
	if dir == "" {
		t.Skip("set EVTC_REAL_LOGS to 1 to combine every log under ../tests_fixtures, or to a folder of logs")
	}
	named := dir != "1"
	if !named {
		dir = "tests_fixtures"
	}
	// A relative path is read from the repository root.
	if !filepath.IsAbs(dir) {
		dir = filepath.Join("..", dir)
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.zevtc"))
	// A folder named on purpose must hold logs; tests_fixtures may be
	// missing.
	if len(paths) == 0 && named {
		t.Fatalf("no log under %s", dir)
	}
	if len(paths) == 0 {
		t.Skipf("no log under %s", dir)
	}
	slices.Sort(paths)
	var mu sync.Mutex
	var lines []string
	var logs []recorded
	checked, refused, short := 0, 0, 0
	t.Run("logs", func(t *testing.T) {
		for _, path := range paths {
			name := strings.TrimSuffix(filepath.Base(path), ".zevtc")
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s: panic: %v\n%s", name, r, debug.Stack())
					}
				}()
				tl, err := timeline.ParseFile(path)
				if err != nil && !errors.Is(err, timeline.ErrLegacyLog) {
					t.Fatalf("parse: %v", err)
				}
				line, rec, ok := "", recorded{path: path, name: name}, false
				switch {
				case err != nil:
					line = fmt.Sprintf("%-46s too old, refused", name)
				default:
					rec.build, rec.mapID, rec.start, rec.end = strings.TrimSpace(tl.Log.Header.Build), tl.MapID, tl.Start, tl.Start.Add(tl.Duration)
					if tl.POV != nil {
						rec.account = tl.POV.Account
					}
					line, ok = alone(t, name, tl)
				}
				mu.Lock()
				defer mu.Unlock()
				lines = append(lines, line)
				switch {
				case err != nil:
					refused++
				case !ok:
					short++
					logs = append(logs, rec)
				default:
					checked++
					logs = append(logs, rec)
				}
			})
		}
	})
	slices.Sort(lines)
	t.Logf("\n%s", strings.Join(lines, "\n"))
	t.Logf("%d logs come back from themselves and from a client that stopped early, %d hold too few strikes, %d are too old", checked, short, refused)

	var fights [][]recorded
	groups := together(logs)
	for _, g := range groups {
		fights = append(fights, sameFight(t, g)...)
	}
	lines = lines[:0]
	for i, f := range fights {
		t.Run(fmt.Sprintf("fight %d", i+1), func(t *testing.T) {
			lines = append(lines, fight(t, f)...)
		})
	}
	if len(fights) > 0 {
		t.Logf("\n%s", strings.Join(lines, "\n"))
	}
	t.Logf("%d groups of logs written on the same map at the same time, %d fights recorded by several players", len(groups), len(fights))
}

// alone runs the checks of one log and returns its line of the report,
// and whether the log holds strikes enough to set a clock.
func alone(t *testing.T, name string, tl *timeline.Timeline) (string, bool) {
	t.Helper()
	start := time.Now()
	want := totalsOf(tl)
	want.events -= addressChanges(tl.Log)
	if _, err := recordings([]*evtc.Log{tl.Log, tl.Log}); errors.Is(err, ErrFightMismatch) {
		return fmt.Sprintf("%-46s too few strikes", name), false
	}
	l, got, m := audited(t, tl.Log, tl.Log)
	checkInvariants(t, l, got, m)
	if have := totalsOf(got); have != want {
		t.Errorf("with itself: totals = %+v, want %+v", have, want)
	}
	compared, differ := states(got, m).total()
	if differ != 0 {
		t.Errorf("with itself: %d states of %d differ", differ, compared)
	}
	_, filled := sameEvents(t, tl, l, holding{ordered: true})

	at := tl.Duration * 3 / 5
	early, other := cut(tl, at), moved(tl, 0x5a5a_0000_5a5a, 7_654_322)
	if _, err := recordings([]*evtc.Log{early, other}); errors.Is(err, ErrFightMismatch) {
		return fmt.Sprintf("%-46s %7d events, too few strikes before it stops", name, len(tl.Log.Events)), false
	}
	for i, logs := range [][]*evtc.Log{{early, other}, {other, early}} {
		l, got, m := audited(t, logs...)
		checkInvariants(t, l, got, m)
		have := totalsOf(got)
		// sameEvents counts the events.
		have.events = want.events
		if have != want {
			t.Errorf("with a client that stopped early: totals = %+v, want %+v", have, want)
		}
		// The log of the whole fight, as the recording read first names
		// and dates it.
		if i == 0 {
			sameEvents(t, tl, l, holding{left: lost(tl, at), first: m.recs[0].tl, mask: 0x5a5a_0000_5a5a})
		} else {
			sameEvents(t, m.recs[0].tl, l, holding{ordered: true})
		}
	}
	return fmt.Sprintf("%-46s %7d events %4d agents %9d states compared %5d agents named  %v", name, len(tl.Log.Events), tl.Agents().Count(), compared, filled, time.Since(start).Round(time.Millisecond)), true
}

// together groups the logs that may have recorded the same fight: written
// by the same arcdps build on the same map, by other players, over times
// that overlap. Two squads fight the same boss at the same time, each in
// its own instance of the map: Logs tells them apart.
func together(logs []recorded) [][]recorded {
	slices.SortFunc(logs, func(a, b recorded) int { return cmp.Or(a.start.Compare(b.start), cmp.Compare(a.name, b.name)) })
	var out [][]recorded
	grouped := make([]bool, len(logs))
	for i, a := range logs {
		if grouped[i] {
			continue
		}
		group, end, accounts := []recorded{a}, a.end, map[string]bool{a.account: true}
		for j := i + 1; j < len(logs) && logs[j].start.Before(end); j++ {
			b := logs[j]
			if grouped[j] || b.build != a.build || b.mapID != a.mapID || accounts[b.account] {
				continue
			}
			grouped[j], accounts[b.account] = true, true
			group = append(group, b)
			if b.end.After(end) {
				end = b.end
			}
		}
		if len(group) > 1 {
			out = append(out, group)
		}
	}
	return out
}

// sameFight splits a group into the fights its logs recorded: two logs
// recorded the same one when their clocks can be set on each other.
func sameFight(t *testing.T, group []recorded) [][]recorded {
	t.Helper()
	recs := make([]*recording, len(group))
	shared := make([]map[strike][]time.Duration, len(group))
	for i, g := range group {
		tl, err := timeline.ParseFile(g.path)
		if err != nil {
			t.Fatal(err)
		}
		recs[i] = &recording{tl: tl, typed: tl.Has(evtc.CapabilityTypedEvents)}
		shared[i] = strikes(recs[i])
	}
	fight := make([]int, len(group))
	for i := range fight {
		fight[i] = i
	}
	for i := range group {
		for j := i + 1; j < len(group); j++ {
			if off, n := offsetOf(shared[i], shared[j]); n >= MinStrikes && sameStart(recs[i].tl, recs[j].tl, off) {
				from := fight[j]
				for k := range fight {
					if fight[k] == from {
						fight[k] = fight[i]
					}
				}
			}
		}
	}
	var out [][]recorded
	for i := range group {
		var logs []recorded
		for j := range group {
			if fight[j] == i {
				logs = append(logs, group[j])
			}
		}
		if len(logs) > 1 {
			out = append(out, logs)
		}
	}
	return out
}

// named counts the casts of a timeline whose caster the log names. A log
// that writes the end of a cast without address holds that end as the
// cast of nobody.
func named(tl *timeline.Timeline) int {
	return tl.Casts().Where(func(c *timeline.Cast) bool { return c.Caster != tl.Unknown }).Count()
}

// fight combines the recordings of one fight, all together and two by
// two, and returns the lines of the report.
func fight(t *testing.T, group []recorded) []string {
	t.Helper()
	var logs []*evtc.Log
	var alone []totals
	var lines []string
	for _, r := range group {
		tl, err := timeline.ParseFile(r.path)
		if err != nil {
			t.Fatal(err)
		}
		logs = append(logs, tl.Log)
		one := totalsOf(tl)
		one.casts = named(tl)
		alone = append(alone, one)
		lines = append(lines, fmt.Sprintf("   %-43s %s", r.name, line(one)))
	}
	all, err := Logs(logs...)
	if err != nil {
		t.Fatal(err)
	}
	tl, err := timeline.Build(all)
	if err != nil {
		t.Fatalf("the combined log does not build: %v", err)
	}
	got := totalsOf(tl)
	got.casts = named(tl)
	for i, one := range alone {
		if got.strikes < one.strikes || got.casts < one.casts || got.effects < one.effects || got.missiles < one.missiles || got.players < one.players {
			t.Errorf("the combined log holds less than %s: %+v and %+v", group[i].name, got, one)
		}
	}
	back := slices.Clone(logs)
	slices.Reverse(back)
	if again, err := Logs(back...); err != nil || !sameLog(all, again) {
		t.Errorf("the order given matters: %v", err)
	}
	lines = append(lines, fmt.Sprintf("   %-43s %s", "combined", line(got)))

	twice, compared, differ := 0, 0, 0
	var far margins
	for i := range logs {
		for j := range logs {
			if i == j {
				continue
			}
			l, tl, m, err := tried(logs[i], logs[j])
			if errors.Is(err, ErrFightMismatch) {
				// Two recordings that share too little, set on each
				// other through a third one.
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			checkInvariants(t, l, tl, m)
			twice += toldTwice(l, m, 100*time.Millisecond)
			c, d := states(tl, m).total()
			compared, differ = compared+c, differ+d
			far.add(m)
		}
	}
	return append(lines,
		fmt.Sprintf("   two by two: %d events told twice, %d states of %d differ", twice, differ, compared),
		fmt.Sprintf("   two by two: %v", far))
}

// tried is audited for logs that may not be of the same fight.
func tried(logs ...*evtc.Log) (*evtc.Log, *timeline.Timeline, *combiner, error) {
	l, m, err := combine(logs, true)
	if err != nil {
		return nil, nil, nil, err
	}
	tl, err := timeline.Build(l)
	return l, tl, m, err
}

// line is what a log holds, for the report.
func line(t totals) string {
	return fmt.Sprintf("%7d events %4d npcs %5d strikes %5d casts %6d stacks %5d effects %4d missiles %6d positions",
		t.events, t.npcs, t.strikes, t.casts, t.stacks, t.effects, t.missiles, t.positions)
}
