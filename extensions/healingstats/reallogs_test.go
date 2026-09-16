package healingstats

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// TestRealLogs decodes every log under ../../tests_fixtures, checks the
// invariants of the stats and runs the API on them. The report, printed
// with -v, tells how many heals the addon logged, how many were written
// by both clients, and how many heals between two recording players were
// written by one client only, which the merge cannot pair. The test runs
// only with EVTC_REAL_LOGS set, as it reads hundreds of megabytes of logs.
func TestRealLogs(t *testing.T) {
	if os.Getenv("EVTC_REAL_LOGS") == "" {
		t.Skip("set EVTC_REAL_LOGS=1 to decode every log under ../../tests_fixtures")
	}
	paths, _ := filepath.Glob("../../tests_fixtures/*.zevtc")
	if len(paths) == 0 {
		t.Skip("no log under ../../tests_fixtures")
	}
	slices.Sort(paths)
	var lines []string
	logs, heals, merged, single := 0, 0, 0, 0
	var total time.Duration
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".zevtc")
		t.Run(name, func(t *testing.T) {
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
			total += time.Since(start)
			if errors.Is(err, timeline.ErrLegacyLog) {
				lines = append(lines, name+": legacy log, refused")
				return
			}
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			s := Of(tl)
			if s == nil {
				lines = append(lines, fmt.Sprintf("%-46s no healing stats", name))
				return
			}
			checkInvariants(t, s)
			smoke(t, s)
			logs++
			heals += s.Heals().Count()
			merged += s.Merged
			oneSided := 0
			for _, a := range s.Recorded {
				for _, b := range s.Recorded {
					if a != b {
						oneSided += s.Heals().CreditedTo(a).On(b).Where(func(h *Heal) bool { return h.PeerEvent == nil && h.SrcRecorded != h.DstRecorded }).Count()
					}
				}
			}
			single += oneSided
			top := "nobody"
			if shares := s.Heals().Healing().PerAgent(); len(shares) > 0 {
				top = fmt.Sprintf("%s %d", shares[0].Agent.Name, shares[0].Heals.Healed())
			}
			lines = append(lines, fmt.Sprintf("%-46s %-8s rev %d %6d heals %5d merged %2d recorded %4d one-sided  top %s", name, s.Version, s.Revision, s.Heals().Count(), s.Merged, len(s.Recorded), oneSided, top))
		})
	}
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
