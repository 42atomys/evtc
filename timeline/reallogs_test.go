package timeline

import (
	"cmp"
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
)

// scalarKinds are the event kinds the builder reads into scalar fields
// without keeping a pointer to the event: they count as covered.
var scalarKinds = map[evtc.StateChange]bool{
	evtc.StateSquadCombatStart: true, evtc.StateSquadCombatEnd: true, evtc.StatePointOfView: true,
	evtc.StateMapID: true, evtc.StateLogNPCUpdate: true, evtc.StateLanguage: true, evtc.StateGWBuild: true,
	evtc.StateShardID: true, evtc.StateFractalScale: true, evtc.StateRuleset: true, evtc.StateInstanceStart: true,
	evtc.StateArcBuild: true, evtc.StateGuild: true, evtc.StateIDToGUID: true, evtc.StateAttackTarget: true,
	evtc.StateIIDChange: true,
}

// kindTally counts the events of one kind over the logs.
type kindTally struct {
	logs, events, uncovered int
}

// TestRealLogs builds every log under ../tests_fixtures, checks the graph
// invariants, runs the public API on it and measures how much of the log
// the graph consumes: an event is covered when a node keeps a pointer to
// it or when its kind is read into a scalar field. The report is printed
// with -v. The test runs only with EVTC_REAL_LOGS set, as it reads
// hundreds of megabytes of logs.
func TestRealLogs(t *testing.T) {
	if os.Getenv("EVTC_REAL_LOGS") == "" {
		t.Skip("set EVTC_REAL_LOGS=1 to build every log under ../tests_fixtures")
	}
	paths, _ := filepath.Glob("../tests_fixtures/*.zevtc")
	if len(paths) == 0 {
		t.Skip("no log under ../tests_fixtures")
	}
	slices.Sort(paths)
	tallies := map[evtc.StateChange]*kindTally{}
	var lines []string
	totalEvents, totalCovered := 0, 0
	var totalBuild time.Duration
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".zevtc")
		t.Run(name, func(t *testing.T) {
			line, covered, events, build := validateLog(t, path, tallies)
			lines = append(lines, line)
			totalEvents += events
			totalCovered += covered
			totalBuild += build
		})
	}
	t.Logf("\n%s", strings.Join(lines, "\n"))
	kinds := make([]evtc.StateChange, 0, len(tallies))
	for k := range tallies {
		kinds = append(kinds, k)
	}
	slices.SortFunc(kinds, func(x, y evtc.StateChange) int {
		if a, b := tallies[x], tallies[y]; a.uncovered != b.uncovered {
			return cmp.Compare(b.uncovered, a.uncovered)
		}
		return cmp.Compare(x, y)
	})
	var table []string
	for _, k := range kinds {
		ty := tallies[k]
		note := "covered"
		switch {
		case ty.uncovered == ty.events:
			note = "raw only"
		case ty.uncovered > 0:
			note = fmt.Sprintf("%.2f%% dropped", 100*float64(ty.uncovered)/float64(ty.events))
		}
		table = append(table, fmt.Sprintf("%-22s logs %3d events %9d uncovered %8d  %s", k, ty.logs, ty.events, ty.uncovered, note))
	}
	t.Logf("\n%d logs, %d events, %.3f%% covered, built in %v\n%s", len(paths), totalEvents, 100*float64(totalCovered)/float64(max(totalEvents, 1)), totalBuild.Round(time.Millisecond), strings.Join(table, "\n"))
}

// validateLog builds one log and returns its report line, its covered and
// total event counts and its build time.
func validateLog(t *testing.T, path string, tallies map[evtc.StateChange]*kindTally) (line string, covered, events int, build time.Duration) {
	name := strings.TrimSuffix(filepath.Base(path), ".zevtc")
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: panic: %v\n%s", name, r, debug.Stack())
			line = name + ": panic"
		}
	}()
	l, err := evtc.ParseFile(path)
	if err != nil {
		t.Errorf("%s: parse: %v", name, err)
		return name + ": parse error", 0, 0, 0
	}
	start := time.Now()
	tl, err := Build(l)
	build = time.Since(start)
	if errors.Is(err, ErrLegacyLog) {
		return fmt.Sprintf("%s: legacy log (arcdps %s), refused", name, l.Header.Build), 0, len(l.Events), build
	}
	if err != nil {
		t.Errorf("%s: build: %v", name, err)
		return name + ": build error", 0, len(l.Events), build
	}
	checkInvariants(t, tl)
	smoke(t, name, tl)
	anomalies := sanity(tl)

	refs := referencedEvents(tl)
	perKind := map[evtc.StateChange]*kindTally{}
	for i := range l.Events {
		e := &l.Events[i]
		k := e.IsStateChange
		ty := perKind[k]
		if ty == nil {
			ty = &kindTally{}
			perKind[k] = ty
		}
		ty.events++
		if refs[e] || scalarKinds[k] {
			covered++
		} else {
			ty.uncovered++
		}
	}
	events = len(l.Events)
	var dropped []string
	for k, ty := range perKind {
		g := tallies[k]
		if g == nil {
			g = &kindTally{}
			tallies[k] = g
		}
		g.logs++
		g.events += ty.events
		g.uncovered += ty.uncovered
		if ty.uncovered > 0 {
			dropped = append(dropped, fmt.Sprintf("%v %d/%d", k, ty.uncovered, ty.events))
		}
	}
	slices.Sort(dropped)
	boss := "no boss"
	if b := tl.Boss(); b != nil {
		boss = b.Name
	}
	line = fmt.Sprintf("%-46s %8d events %6.2f%% covered %5dms %2d players %3d targets %-28s %s", name, events, 100*float64(covered)/float64(max(events, 1)), build.Milliseconds(), len(tl.players), len(tl.Targets), boss, strings.Join(dropped, ", "))
	if len(anomalies) > 0 {
		line += "\n    anomalies: " + strings.Join(anomalies, "; ")
	}
	return line, covered, events, build
}

// referencedEvents collects every raw event a node of the graph points to.
func referencedEvents(tl *Timeline) map[*evtc.Event]bool {
	refs := make(map[*evtc.Event]bool, len(tl.events))
	mark := func(e *evtc.Event) {
		if e != nil {
			refs[e] = true
		}
	}
	for _, h := range tl.hits {
		mark(h.Event)
	}
	for _, c := range tl.casts {
		mark(c.Start)
		mark(c.Stop)
	}
	for _, s := range tl.stacks {
		mark(s.Apply)
		mark(s.Remove)
		for _, c := range s.Changes {
			mark(c)
		}
		markSpans(refs, s.Active)
	}
	for _, f := range tl.effects {
		mark(f.Create)
		mark(f.Remove)
	}
	for _, m := range tl.missiles {
		mark(m.Create)
		mark(m.Remove)
		for _, l := range m.Launches {
			mark(l.Event)
		}
		for _, me := range m.Effects {
			mark(me.Event)
		}
	}
	for _, a := range append(slices.Clone(tl.Agents), tl.Unknown) {
		for _, d := range a.Downs {
			mark(d.Event)
		}
		for _, d := range a.Deaths {
			mark(d.Event)
		}
		for _, m := range a.Markers {
			mark(m.Event)
			mark(m.Remove)
		}
		for _, s := range a.StunBreaks {
			mark(s.Event)
		}
		for _, g := range a.GadgetAnimations {
			mark(g.Event)
		}
		markSamples(refs, a.Position)
		markSamples(refs, a.Velocity)
		markSamples(refs, a.Facing)
		markSamples(refs, a.Health.Series)
		markSamples(refs, a.Barrier.Series)
		markSamples(refs, a.MaxHealth.Series)
		markSamples(refs, a.DefiancePercent.Series)
		markSpans(refs, a.Life)
		markSpans(refs, a.Defiance)
		markSpans(refs, a.InCombat)
		markSpans(refs, a.Targetable)
		markSpans(refs, a.Team)
		markSpans(refs, a.WeaponSet)
		markSpans(refs, a.Stealth)
		markSpans(refs, a.Gliding)
		markSpans(refs, a.Transformation)
		markSpans(refs, a.Airborne)
		markSpans(refs, a.NameVisible)
	}
	for _, gm := range tl.GroundMarkers {
		mark(gm.Event)
		mark(gm.Remove)
	}
	for _, r := range tl.Rewards {
		mark(r.Event)
	}
	for _, mc := range tl.MapChanges {
		mark(mc.Event)
	}
	for _, m := range tl.Integrity {
		mark(m.Event)
	}
	for _, x := range tl.Extensions {
		mark(x.Event)
		for _, e := range x.events {
			mark(e)
		}
	}
	markSamples(refs, tl.Ping.Series)
	for _, s := range tl.Skills {
		mark(s.Info)
		for _, tm := range s.Timings {
			mark(tm.Event)
		}
	}
	for _, b := range tl.Buffs {
		mark(b.Info)
		for _, f := range b.Formulas {
			mark(f.Event)
		}
	}
	return refs
}

func markSamples[T any](refs map[*evtc.Event]bool, s Series[T]) {
	for _, smp := range s.Samples() {
		if smp.Event != nil {
			refs[smp.Event] = true
		}
	}
}

func markSpans[T any](refs map[*evtc.Event]bool, s Spans[T]) {
	for _, sp := range s.All() {
		if sp.Event != nil {
			refs[sp.Event] = true
		}
	}
}

// smoke runs the public API the way a report would, to catch panics on
// unusual logs; the values are not checked.
func smoke(t *testing.T, name string, tl *Timeline) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: API panic: %v\n%s", name, r, debug.Stack())
		}
	}()
	iv := tl.Interval()
	instants := []time.Duration{0, tl.Duration / 3, tl.Duration / 2, tl.Duration, tl.Duration + time.Second, -time.Second}
	for _, p := range tl.players {
		for _, at := range instants {
			p.PositionAt(at)
			p.HealthAt(at)
			p.LifeStateAt(at)
			p.IsInCombatAt(at)
			p.TeamAt(at)
			p.WeaponSetAt(at)
			p.IsGlidingAt(at)
			p.IsAirborneAt(at)
			p.SquadMarkerAt(at)
			p.IsCommanderAt(at)
			p.Stacks().CountAt(at)
			p.Stacks().EffectiveAt(at)
		}
		p.HitsCredited().Landed().DPS(iv)
		p.Hits().PerSkill()
		p.HitsTaken().PerAgent()
		p.Casts().Completed().PerSkill()
		p.Stacks().OfBuff(BuffMight).Uptime(iv)
		p.Stacks().OfBuff(BuffQuickness).EffectiveAverage(iv)
		p.StacksApplied().PerBuff()
		p.Stacks().RemovedBy(p).Count()
		p.CombatTime(iv)
		p.AliveTime(iv)
		p.DownTime(iv)
		p.DiedAt()
		p.DownedBetween(iv)
		p.Effects().PerID()
		p.Missiles().PerSkill()
		p.Events().Involving(p).Count()
		p.Events().By(p).Limit(3).All()
		p.Spec()
		p.GlidingTime(iv)
		p.AirborneTime(iv)
		if len(tl.players) > 1 {
			p.DistanceTo(tl.players[0], tl.Duration/2)
		}
	}
	if boss := tl.Boss(); boss != nil {
		boss.PhasesByHealth(75, 50, 25)
		boss.HealthCrossings(66.6, 33.3)
		boss.HealthBelow(50)
		boss.HitsTaken().Landed().PerAgent()
		boss.Casts().PerSkill()
		for _, bb := range boss.Breakbars {
			bb.CCHits().PerAgent()
			bb.TotalCC()
		}
		boss.Effects().Ground().Around().Count()
		boss.Missiles().Count()
		boss.DeathsBy(tl.Unknown)
		tl.TargetBySpeciesIDAt(boss.SpeciesID, tl.Duration/2)
	}
	for _, target := range tl.Targets {
		target.HitsTaken().Damage()
		target.Life.Len()
		for _, d := range target.Downs {
			d.Interval.Duration()
		}
	}
	for _, g := range tl.Gadgets() {
		g.IsNameVisibleAt(g.Lifetime.End)
		g.NameVisibleTime(iv)
	}
	tl.Hits().Foes().Blocked().Reverse().Limit(5).All()
	tl.Hits().PerTarget()
	tl.Casts().Full().Count()
	tl.Stacks().PerReceiver()
	tl.Effects().At(tl.Duration / 2).Count()
	tl.Missiles().Where((*Missile).Removed).Count()
	tl.Commander()
	tl.CommanderAt(tl.Duration / 2)
	tl.GroundMarkerAt(SquadArrow, tl.Duration/2)
	tl.PingAt(tl.Duration / 2)
	tl.Subgroup(1)
	tl.AgentsNamed("")
	tl.Since(tl.Duration / 2)
	tl.ExtensionEvents().Count()
	for _, x := range tl.Extensions {
		x.Events().Count()
		if e := x.Events().First(); e != nil {
			tl.ExtensionOf(e)
			tl.Agent(e.SrcAgent)
			tl.Agent(e.DstAgent)
		}
	}
	tl.Extension(ExtensionHealingStats)
	for _, s := range tl.Skills[:min(20, len(tl.Skills))] {
		s.Casts().Count()
		s.Hits().Count()
		s.Missiles().Count()
	}
	for _, b := range tl.Buffs[:min(20, len(tl.Buffs))] {
		b.Stacks().Count()
		b.IsBoon()
	}
}

// sanity reports semantic oddities of a log that the invariants do not
// cover: they are hints for a closer look, not failures.
func sanity(tl *Timeline) []string {
	var out []string
	if tl.POV == nil {
		out = append(out, "no point of view")
	}
	if len(tl.players) == 0 {
		out = append(out, "no players")
	}
	if tl.Duration <= 0 {
		out = append(out, fmt.Sprintf("duration %v", tl.Duration))
	}
	openStacks, superseded := 0, 0
	for _, s := range tl.stacks {
		if s.Remove == nil && !s.Superseded {
			openStacks++
		}
		if s.Superseded {
			superseded++
		}
	}
	if n := len(tl.stacks); n > 0 && superseded > n/20 {
		out = append(out, fmt.Sprintf("%d of %d stacks superseded", superseded, n))
	}
	openCasts := 0
	for _, c := range tl.casts {
		if c.Stop == nil {
			openCasts++
		}
	}
	if n := len(tl.casts); n > 0 && openCasts > n/10 {
		out = append(out, fmt.Sprintf("%d of %d casts without a stop", openCasts, n))
	}
	unknownHits := tl.Unknown.Hits().Count()
	if n := tl.Hits().Count(); n > 0 && unknownHits > n/5 {
		out = append(out, fmt.Sprintf("%d of %d hits from an unknown source", unknownHits, n))
	}
	badHealth := 0
	for _, a := range tl.Agents {
		for _, smp := range a.Health.Samples() {
			if smp.Value < 0 || smp.Value > 100 {
				badHealth++
			}
		}
		if a.Lifetime.Start > a.Lifetime.End {
			out = append(out, fmt.Sprintf("%v has an inverted lifetime %v", a, a.Lifetime))
		}
	}
	if badHealth > 0 {
		out = append(out, fmt.Sprintf("%d health samples outside 0..100", badHealth))
	}
	noCause := 0
	for _, p := range tl.players {
		for _, d := range p.Deaths {
			if d.Cause == nil {
				noCause++
			}
		}
	}
	if noCause > 0 {
		out = append(out, fmt.Sprintf("%d player deaths without a cause", noCause))
	}
	attached := 0
	for _, x := range tl.Extensions {
		attached += len(x.events)
	}
	if n := tl.ExtensionEvents().Count(); n != attached {
		out = append(out, fmt.Sprintf("%d of %d extension events without a registered extension", n-attached, n))
	}
	if n := len(tl.MapChanges) + len(tl.Rewards) + len(tl.Integrity); n > 0 {
		out = append(out, fmt.Sprintf("%d map changes, %d rewards, %d integrity messages", len(tl.MapChanges), len(tl.Rewards), len(tl.Integrity)))
	}
	jumps := 0
	for _, p := range tl.players {
		jumps += p.Airborne.Len()
	}
	if jumps > 0 {
		out = append(out, fmt.Sprintf("%d jump states", jumps))
	}
	return out
}
