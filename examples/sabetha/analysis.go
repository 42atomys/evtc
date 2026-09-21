package main

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// resurrect is the skill id of the resurrect action, which the skill table
// does not name.
const resurrect = 1066

// analysis prints the sections that combine several parts of the graph.
func analysis(tl *timeline.Timeline) {
	offense(tl)
	defense(tl)
	boonGeneration(tl)
	cleanses(tl)
	deathAnalysis(tl)
	addWaves(tl)
	cannons(tl)
	mechanics(tl)
	rotation(tl)
	spread(tl)
	closeCalls(tl)
}

func pct(part, total int) string {
	if total == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(part)/float64(total))
}

func offense(tl *timeline.Timeline) {
	boss := tl.Boss()
	section("Offense (credited hits, minions included)")
	w := table()
	fmt.Fprintln(w, "player\ttotal\tboss\tcleave\tpower\tcondi\tcrit\tflank\tscholar\tmoving\tdodges\tswaps\tresurrects\tinterrupts")
	for p := range tl.Characters().Seq() {
		foes := p.HitsCredited().Landed().Foes()
		strikes := foes.Strikes()
		swaps := tl.Events().Of(evtc.StateWeaponSwap).Involving(p).Count()
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\n",
			p.Name, foes.Damage(), foes.On(boss).Damage(), foes.Damage()-foes.On(boss).Damage(),
			strikes.Damage(), foes.BuffDamage().Damage(),
			pct(strikes.Crits().Count(), strikes.Count()),
			pct(strikes.Where(func(h *timeline.Hit) bool { return h.Flanking }).Count(), strikes.Count()),
			pct(strikes.Where(func(h *timeline.Hit) bool { return h.OverNinety }).Count(), strikes.Count()),
			pct(strikes.Where(func(h *timeline.Hit) bool { return h.Moving }).Count(), strikes.Count()),
			p.Casts().OfSkill(evtc.SkillDodge).Count(), swaps, p.Casts().OfSkill(resurrect).Count(),
			p.Hits().Where((*timeline.Hit).Interrupted).Count())
	}
	w.Flush()
}

func defense(tl *timeline.Timeline) {
	section("Defense")
	w := table()
	fmt.Fprintln(w, "player\thits taken\thealth dmg\tbarrier\tblocked\tevaded\tabsorbed\tmissed\tdown time\tworst skill")
	for p := range tl.Characters().Seq() {
		taken := p.HitsTaken()
		worst := "-"
		if shares := taken.Landed().PerSkill(); len(shares) > 0 {
			worst = fmt.Sprintf("%s %d", shares[0].Skill.Name, shares[0].Hits.HealthDamage())
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%v\t%s\n", p.Name, taken.Count(), taken.HealthDamage(), taken.Barrier(),
			taken.Blocked().Count(), taken.Evaded().Count(), taken.Absorbed().Count(), taken.Missed().Count(),
			p.DownTime(tl.Interval()).Round(time.Millisecond), worst)
	}
	w.Flush()
}

func boonGeneration(tl *timeline.Timeline) {
	section("Boon generation (average stacks given to the other players)")
	w := table()
	fmt.Fprintln(w, "boon\tgenerators")
	span := tl.Interval()
	for _, id := range []uint32{timeline.BuffMight, timeline.BuffFury, timeline.BuffQuickness, timeline.BuffAlacrity, timeline.BuffProtection, timeline.BuffStability} {
		buff := tl.Buff(id)
		if buff == nil {
			continue
		}
		// Might or stability stack by intensity: what a player gives is a
		// number of stacks. Fury or quickness queue up durations: what a
		// player gives is uptime.
		intensity := buff.Stacking.Intensity()
		type gen struct {
			name  string
			given float64
		}
		var gens []gen
		for p := range tl.Characters().Seq() {
			applied := p.StacksApplied().OfBuff(id)
			var sum float64
			for r := range tl.Characters().Seq() {
				if r == p {
					continue
				}
				if intensity {
					sum += applied.On(r).EffectiveAverage(span)
				} else {
					sum += 100 * applied.On(r).Uptime(span).Seconds() / span.Duration().Seconds()
				}
			}
			if sum > 0 {
				gens = append(gens, gen{p.Name, sum / float64(tl.Characters().Count()-1)})
			}
		}
		slices.SortFunc(gens, func(a, b gen) int { return cmp.Compare(b.given, a.given) })
		var parts []string
		unit := " stacks"
		if !intensity {
			unit = "%"
		}
		for _, g := range gens[:min(3, len(gens))] {
			parts = append(parts, fmt.Sprintf("%s %.1f%s", g.name, g.given, unit))
		}
		fmt.Fprintf(w, "%s\t%s\n", buff.Skill.Name, strings.Join(parts, ", "))
	}
	w.Flush()
}

func cleanses(tl *timeline.Timeline) {
	section("Cleanses and strips")
	w := table()
	fmt.Fprintln(w, "player\tconditions cleansed\tboons stripped\tmost cleansed")
	for p := range tl.Characters().Seq() {
		// When a skill cleanses or strips, arcdps writes one manual removal
		// per stack naming the agent that did it. Natural expiries are
		// single removals.
		removed := tl.Stacks().RemovedBy(p).Where(func(s *timeline.BuffStack) bool {
			return s.Receiver != p.Agent && s.Removal == evtc.BuffRemoveManual
		})
		cleansed := removed.Where(func(s *timeline.BuffStack) bool { return s.Receiver.IsPlayer() && s.Buff.IsCondition() })
		stripped := removed.Where(func(s *timeline.BuffStack) bool { return !s.Receiver.IsPlayer() && s.Buff.IsBoon() })
		most := "-"
		if shares := cleansed.PerBuff(); len(shares) > 0 {
			most = fmt.Sprintf("%s (%d)", shares[0].Buff.Skill.Name, shares[0].Stacks.Count())
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\n", p.Name, cleansed.Count(), stripped.Count(), most)
	}
	w.Flush()
}

func deathAnalysis(tl *timeline.Timeline) {
	section("What downed them (damage taken in the five seconds before)")
	for p := range tl.Characters().Seq() {
		for _, d := range p.Downs {
			before := timeline.NewInterval(d.Start-5*time.Second, d.Start)
			taken := p.HitsTaken().Landed().Between(before)
			var parts []string
			for _, s := range taken.PerSkill()[:min(3, len(taken.PerSkill()))] {
				parts = append(parts, fmt.Sprintf("%s %d by %s", s.Skill.Name, s.Hits.HealthDamage(), s.Hits.First().Src.Name))
			}
			fmt.Printf("%v %s: %d health damage over %d hits, health %.0f%% five seconds before: %s\n", d.Start.Round(time.Millisecond), p.Name, taken.HealthDamage(), taken.Count(), p.HealthAt(before.Start), strings.Join(parts, "; "))
		}
	}
}

func addWaves(tl *timeline.Timeline) {
	section("Adds by species")
	type wave struct {
		name          string
		count, killed int
		lifetime      time.Duration
		damage        int64
	}
	waves := map[uint16]*wave{}
	var order []uint16
	adds := tl.Targets().Where(func(t *timeline.Target) bool { return !t.Boss && t.IsNPC() })
	for t := range adds.Seq() {
		wv := waves[t.SpeciesID]
		if wv == nil {
			wv = &wave{name: t.Name}
			waves[t.SpeciesID] = wv
			order = append(order, t.SpeciesID)
		}
		wv.count++
		end := t.Lifetime.End
		if died, ok := t.DiedAt(); ok {
			wv.killed++
			end = died
		}
		wv.lifetime += end - t.Lifetime.Start
		wv.damage += t.HitsTaken().Landed().Damage()
	}
	w := table()
	fmt.Fprintln(w, "species\tname\tcount\tkilled\tavg time alive\tdamage taken")
	for _, id := range order {
		wv := waves[id]
		fmt.Fprintf(w, "%d\t%s\t%d\t%d\t%v\t%d\n", id, wv.name, wv.count, wv.killed, (wv.lifetime / time.Duration(wv.count)).Round(time.Second), wv.damage)
	}
	w.Flush()
}

func cannons(tl *timeline.Timeline) {
	section("Cannons")
	for g := range tl.Gadgets().Seq() {
		// Gadget ids are volatile, so cannons are found by name, as a
		// French or an English client writes it.
		name := strings.ToLower(g.Name)
		if !strings.Contains(name, "canon") && !strings.Contains(name, "cannon") || g.HitsTaken().Count() == 0 {
			continue
		}
		taken := g.HitsTaken().Landed()
		// A cannon is destroyed when its health collapses; arcdps stops
		// updating it before it reaches zero and it comes back at 100%
		// when re-armed, so every fall under 25% is one destruction.
		var kills []string
		for _, c := range g.Health.Crossings(25) {
			if c.Direction != timeline.Falling {
				continue
			}
			by := "?"
			if top := taken.Between(timeline.NewInterval(c.Time-10*time.Second, c.Time)).PerAgent(); len(top) > 0 {
				by = top[0].Agent.Name
			}
			kills = append(kills, fmt.Sprintf("%v by %s", c.Time.Round(time.Second), by))
		}
		when := "never destroyed"
		if len(kills) > 0 {
			when = "destroyed " + strings.Join(kills, ", ")
		}
		var by []string
		for _, c := range taken.PerAgent()[:min(3, len(taken.PerAgent()))] {
			by = append(by, fmt.Sprintf("%s %d", c.Agent.Name, c.Hits.Damage()))
		}
		pos, _ := g.Position.First()
		var nearby []string
		for p := range tl.Characters().Seq() {
			if d := p.DistanceTo(g, taken.Last().Time); !math.IsNaN(d) && d < 400 {
				nearby = append(nearby, p.Name)
			}
		}
		fmt.Printf("%s at %v: %d hits, %s, by %s; near the last hit: %s\n", g.Name, pos.Value, taken.Count(), when, strings.Join(by, ", "), strings.Join(nearby, ", "))
		fmt.Printf("  fired %d shots at players, %d landed for %d damage\n", g.Hits().Count(), g.Hits().Landed().Count(), g.Hits().Landed().HealthDamage())
	}
}

func mechanics(tl *timeline.Timeline) {
	boss := tl.Boss()
	section("Mechanics")
	// Buffs the boss puts on players: the sapper bomb and the like.
	fmt.Println("buffs applied by the boss on players:")
	for _, share := range tl.Stacks().By(boss).PerBuff() {
		var names []string
		for _, r := range share.Stacks.PerReceiver() {
			names = append(names, fmt.Sprintf("%s x%d", r.Agent.Name, r.Stacks.Count()))
		}
		avg := share.Stacks.Sum(func(s *timeline.BuffStack) time.Duration { return s.Duration() }) / time.Duration(share.Stacks.Count())
		fmt.Printf("  %s: %d stacks, %v average duration, on %s\n", share.Buff.Skill.Name, share.Stacks.Count(), avg.Round(time.Millisecond), strings.Join(names, ", "))
	}
	// Casts whose hits carry another skill id: match hits by time instead.
	fmt.Println("hits on players during each boss cast:")
	for _, s := range boss.Casts().PerSkill() {
		hit := map[string]int{}
		for c := range s.Casts.Seq() {
			for h := range boss.Hits().Landed().Between(c.Interval).Seq() {
				hit[h.Dst.Name]++
			}
		}
		fmt.Printf("  %s (%d casts): %v\n", s.Skill.Name, s.Casts.Count(), hit)
	}
	fmt.Println("interrupts:")
	for h := range tl.Hits().Where((*timeline.Hit).Interrupted).Seq() {
		fmt.Printf("  %v %s interrupted %s with %s\n", h.Time.Round(time.Millisecond), h.Src.Name, h.Dst.Name, h.Skill.Name)
	}
}

func rotation(tl *timeline.Timeline) {
	p := tl.POV.Main()
	section("Rotation of " + p.Name)
	casts := p.Casts()
	var quickness, scaled float64
	for c := range casts.Completed().Seq() {
		if c.Elapsed > 0 {
			quickness += float64(c.ElapsedUnscaled) / float64(c.Elapsed)
			scaled++
		}
	}
	fmt.Printf("%d casts, %d cancelled, %d played in full, average speed factor %.2f\n", casts.Count(), casts.Cancelled().Count(), casts.Full().Count(), quickness/scaled)
	w := table()
	fmt.Fprintln(w, "skill\tcasts\tcancelled\tavg duration\thits per cast")
	for _, s := range casts.PerSkill()[:min(10, len(casts.PerSkill()))] {
		avg := s.Casts.Sum(func(c *timeline.Cast) time.Duration { return c.Duration() }) / time.Duration(s.Casts.Count())
		fmt.Fprintf(w, "%s\t%d\t%d\t%v\t%.1f\n", s.Skill.Name, s.Casts.Count(), s.Casts.Cancelled().Count(), avg.Round(time.Millisecond), float64(s.Casts.Hits().Count())/float64(s.Casts.Count()))
	}
	w.Flush()
	fmt.Println("weapon swaps:")
	for e := range tl.Events().Of(evtc.StateWeaponSwap).Involving(p).Seq() {
		fmt.Printf("  %v to set %d\n", tl.TimeOf(e).Round(time.Millisecond), e.DstAgent)
	}
	fmt.Println("first casts:")
	for c := range casts.Limit(8).Seq() {
		fmt.Printf("  %v %s %v %v\n", c.Interval.Start.Round(time.Millisecond), c.Skill.Name, c.Duration().Round(time.Millisecond), c.Activation)
	}
}

// spread prints the mean distance between alive players, sampled every two
// seconds, per phase.
func spread(tl *timeline.Timeline) {
	section("Spread")
	boss := tl.Boss()
	for i, phase := range boss.PhasesByHealth(75, 50, 25) {
		var sum, n float64
		for t := phase.Start; t <= phase.End; t += 2 * time.Second {
			alive := tl.Characters().AliveAt(t).All()
			for a := range alive {
				for b := a + 1; b < len(alive); b++ {
					if d := alive[a].DistanceTo(alive[b], t); !math.IsNaN(d) {
						sum, n = sum+d, n+1
					}
				}
			}
		}
		fmt.Printf("phase %d: players %.0f units apart on average\n", i+1, sum/n)
	}
}

func closeCalls(tl *timeline.Timeline) {
	section("Close calls")
	w := table()
	fmt.Fprintln(w, "player\tlowest health\tat\ttime under 50%\tbarrier peak")
	for p := range tl.Characters().Seq() {
		lowest, at := 100.0, time.Duration(0)
		for s := range p.Health.Seq() {
			if s.Value < lowest {
				lowest, at = s.Value, s.Time
			}
		}
		// A dead player sits at 0%: measure over the time alive.
		alive := tl.Interval()
		if died, ok := p.DiedAt(); ok {
			alive = tl.Until(died)
		}
		peak, _ := p.Barrier.MaxBetween(alive)
		fmt.Fprintf(w, "%s\t%.0f%%\t%v\t%v\t%.0f%%\n", p.Name, lowest, at.Round(time.Millisecond), p.Health.TimeBelow(50, alive).Round(time.Millisecond), peak)
	}
	w.Flush()
}
