package healingstats

import (
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

// TestRealLogs decodes every log under ../../tests_fixtures, or under the
// folder EVTC_REAL_LOGS names, checks the invariants of the stats and runs
// the API on them. The report, printed with -v, tells how many heals the
// addon logged, how many were written by both clients, and how many heals
// between two recording players were written by one client only, which the
// merge cannot pair. The test runs only with EVTC_REAL_LOGS set, as it
// reads hundreds of megabytes of logs, several at a time: -parallel bounds
// how many.
func TestRealLogs(t *testing.T) {
	dir := os.Getenv("EVTC_REAL_LOGS")
	if dir == "" {
		t.Skip("set EVTC_REAL_LOGS to 1 to decode every log under ../../tests_fixtures, or to a folder of logs")
	}
	named := dir != "1"
	if !named {
		dir = "tests_fixtures"
	}
	// A relative path is read from the repository root.
	if !filepath.IsAbs(dir) {
		dir = filepath.Join("../..", dir)
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
	logs, heals, merged, single := 0, 0, 0, 0
	var total time.Duration
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
				l, err := evtc.ParseFile(path)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				start := time.Now()
				tl, err := timeline.Build(l)
				build := time.Since(start)
				if err != nil && !errors.Is(err, timeline.ErrLegacyLog) {
					t.Fatalf("build: %v", err)
				}
				var s *Stats
				line := name + ": too old, refused"
				oneSided := 0
				if err == nil {
					line = fmt.Sprintf("%-46s no healing stats", name)
					s = Of(tl)
				}
				if s != nil {
					checkInvariants(t, s)
					smoke(t, s)
					for _, a := range s.Recorded {
						for _, b := range s.Recorded {
							if a != b {
								oneSided += s.Heals().CreditedTo(a).On(b).Where(func(h *Heal) bool { return h.PeerEvent == nil && h.SrcRecorded != h.DstRecorded }).Count()
							}
						}
					}
					top := "nobody"
					if shares := s.Heals().Healing().PerAgent(); len(shares) > 0 {
						top = fmt.Sprintf("%s %d", shares[0].Agent.Name, shares[0].Heals.Healed())
					}
					line = fmt.Sprintf("%-46s %-8s rev %d %6d heals %5d merged %2d recorded %4d one-sided  top %s", name, s.Version, s.Revision, s.Heals().Count(), s.Merged, len(s.Recorded), oneSided, top)
				}
				mu.Lock()
				defer mu.Unlock()
				lines = append(lines, line)
				total += build
				if s != nil {
					logs++
					heals += s.Heals().Count()
					merged += s.Merged
					single += oneSided
				}
			})
		}
	})
	slices.Sort(lines)
	t.Logf("\n%s", strings.Join(lines, "\n"))
	t.Logf("%d logs with healing stats: %d heals, %d written by both clients, %d one-sided between recording players, built in %v", logs, heals, merged, single, total.Round(time.Millisecond))
}

// smoke runs the public API the way a report would, to catch panics on
// unusual logs; the values are not checked.
func smoke(t *testing.T, s *Stats) {
	t.Helper()
	tl := s.Timeline
	iv := tl.Interval()
	half := timeline.NewInterval(0, tl.Duration/2)
	for p := range s.Players().Seq() {
		p.Heals().Healed()
		p.HealsCredited().Healing().PerSkill()
		p.HealsTaken().PerAgent()
		p.HealsTaken().Barrier().BPS(iv)
		p.Heals().Others().HPS(half)
		p.Heals().Downed().Count()
		p.Heals().Ticks().Between(half).Amount()
		p.HealsTaken().Reverse().Limit(3).All()
		tl.Hits().By(p).Count()
	}
	s.Heals().PerTarget()
	s.Heals().Barrier().PerAgent()
	s.Heals().Self().GroupBy(func(h *Heal) *timeline.Skill { return h.Skill })
	if h := s.Heals().First(); h != nil {
		h.Credited()
		s.Heals().OfCast(h.Cast).Count()
		s.Heals().Of(h.Skill).Count()
		s.Heals().On(h.Dst).By(h.Src).Count()
	}
	s.Unknown.Heals().Count()
	s.Agent(tl.Boss()).HealsTaken().Count()
}
