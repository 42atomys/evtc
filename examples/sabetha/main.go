// Command sabetha prints a full report of a Sabetha log using only
// the public API of the timeline and healingstats packages.
//
//	go run ./examples/sabetha [path/to/log.zevtc]
package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/42atomys/evtc/timeline"
)

// Species and skill ids of the Sabetha encounter.
const (
	kernan   = 15372
	knuckles = 15404
	karde    = 15430
	flakShot = 31544
)

func main() {
	start := time.Now()
	path := "tests_fixtures/sabetha-05-fd9b6f3a.zevtc"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	tl, err := timeline.ParseFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	section("Parsing")
	fmt.Printf("%s, parsed in %v\n", tl.Boss().Name, time.Since(start))

	overview(tl)
	phases(tl)
	champions(tl)
	downs(tl)
	bossSkills(tl)
	boons(tl)
	positioning(tl)
	damageBreakdown(tl)
	analysis(tl)
	details(tl)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}

func table() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
}

func overview(tl *timeline.Timeline) {
	boss := tl.Boss()
	section("Overview")
	fmt.Printf("%s, %v, started %s (map %d), recorded by %s\n", boss.Name, tl.Duration.Round(time.Millisecond), tl.Start.Format(time.RFC3339), tl.MapID, tl.POV.Name)
	fmt.Printf("boss ends at %.1f%% health, %d events, %d hits, %d casts, %d buff stacks\n", boss.HealthAt(boss.Lifetime.End), tl.Events().Count(), tl.Hits().Count(), tl.Casts().Count(), tl.Stacks().Count())

	w := table()
	fmt.Fprintln(w, "player\taccount\tgroup\tspec\tboss dps\tdowns\tdeaths\tin combat")
	for _, p := range tl.Players {
		// HitsCredited includes the pets, clones and turrets of the player.
		dps := p.HitsCredited().On(boss).Landed().DPS(boss.Lifetime)
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%.0f\t%d\t%d\t%v\n", p.Name, p.Account, p.Subgroup, p.Spec(), dps, len(p.Downs), len(p.Deaths), p.CombatTime(tl.Interval()).Round(time.Second))
	}
	w.Flush()
}

func phases(tl *timeline.Timeline) {
	boss := tl.Boss()
	section("Phases (boss health 75 / 50 / 25)")
	w := table()
	fmt.Fprintln(w, "phase\tinterval\tduration\tboss damage\tsquad dps\ttop players")
	for i, phase := range boss.PhasesByHealth(75, 50, 25) {
		taken := boss.HitsTaken().Landed().Between(phase)
		var top []string
		shares := taken.PerAgent()
		for _, c := range shares[:min(3, len(shares))] {
			top = append(top, fmt.Sprintf("%s %.0f", c.Agent.Name, float64(c.Hits.Damage())/phase.Duration().Seconds()))
		}
		fmt.Fprintf(w, "%d\t%v\t%v\t%d\t%.0f\t%s\n", i+1, phase, phase.Duration().Round(time.Second), taken.Damage(), taken.DPS(phase), strings.Join(top, ", "))
	}
	w.Flush()
}

func champions(tl *timeline.Timeline) {
	section("Champions")
	for _, species := range []uint16{kernan, knuckles, karde} {
		for _, add := range tl.TargetsBySpeciesID(species) {
			// An NPC that dies has a death; one that despawns only has
			// the end of its lifetime.
			end, died := add.DiedAt()
			if !died {
				end = add.Lifetime.End
			}
			alive := timeline.NewInterval(add.Lifetime.Start, end)
			taken := add.HitsTaken().Landed()
			fmt.Printf("%s: alive %v to %v (%v), %d damage taken, %.0f dps\n", add.Name, alive.Start.Round(time.Second), alive.End.Round(time.Second), alive.Duration().Round(time.Second), taken.Damage(), taken.DPS(alive))
			for _, bb := range add.Breakbars {
				fmt.Printf("  breakbar %v broken=%v in %v, cc: ", bb.Interval, bb.Broken(), bb.Duration().Round(time.Millisecond))
				for _, c := range bb.CCHits().PerAgent() {
					fmt.Printf("%s %d  ", c.Agent.Name, c.Hits.Damage())
				}
				fmt.Println()
			}
		}
	}
}

func downs(tl *timeline.Timeline) {
	section("Downs and deaths")
	w := table()
	fmt.Fprintln(w, "time\tplayer\tevent\tcause\tfrom\toutcome")
	for _, p := range tl.Players {
		for _, d := range p.Downs {
			cause, from := "?", "?"
			if d.Cause != nil {
				cause, from = d.Cause.Skill.Name, d.Cause.Src.Name
			}
			outcome := fmt.Sprintf("revived after %v", d.Duration().Round(time.Millisecond))
			if d.Death != nil {
				outcome = fmt.Sprintf("died after %v", d.Duration().Round(time.Millisecond))
			} else if !d.Recovered {
				outcome = "still down at the end"
			}
			fmt.Fprintf(w, "%v\t%s\tdown\t%s\t%s\t%s\n", d.Start.Round(time.Millisecond), p.Name, cause, from, outcome)
		}
		for _, de := range p.Deaths {
			if de.Down != nil {
				continue // already listed with its down
			}
			cause, from := "?", "?"
			if de.Cause != nil {
				cause, from = de.Cause.Skill.Name, de.Cause.Src.Name
			}
			fmt.Fprintf(w, "%v\t%s\tdeath\t%s\t%s\toutright\n", de.Time.Round(time.Millisecond), p.Name, cause, from)
		}
	}
	w.Flush()
}

func bossSkills(tl *timeline.Timeline) {
	boss := tl.Boss()
	section("Boss skills")
	w := table()
	fmt.Fprintln(w, "skill\tcasts\thits\tlanded\tblocked\tevaded\tdamage\tmost hit")
	for _, s := range boss.Casts().PerSkill() {
		hits := boss.Hits().Of(s.Skill)
		mostHit := "-"
		if targets := hits.Landed().PerTarget(); len(targets) > 0 {
			mostHit = fmt.Sprintf("%s (%d)", targets[0].Agent.Name, targets[0].Hits.Count())
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n", s.Skill.Name, s.Casts.Count(), hits.Count(), hits.Landed().Count(), hits.Blocked().Count(), hits.Evaded().Count(), hits.Landed().HealthDamage(), mostHit)
	}
	w.Flush()

	// Who was the target of each Flak Shot cast?
	targets := map[string]int{}
	for c := range boss.Casts().OfSkill(flakShot).Seq() {
		if c.Target != nil {
			targets[c.Target.Name]++
		}
	}
	fmt.Println("flak shot targets:", targets)
}

func boons(tl *timeline.Timeline) {
	section("Boons (over the time alive)")
	w := table()
	fmt.Fprintln(w, "player\talive\tmight avg\tfury\tquickness\talacrity")
	for _, p := range tl.Players {
		// Uptimes are measured while the player was alive: a player who
		// died early would otherwise look unbuffed.
		alive := tl.Interval()
		if died, ok := p.DiedAt(); ok {
			alive = tl.Until(died)
		}
		uptime := func(id uint32) string {
			return fmt.Sprintf("%.0f%%", 100*p.Stacks().OfBuff(id).Uptime(alive).Seconds()/alive.Duration().Seconds())
		}
		fmt.Fprintf(w, "%s\t%v\t%.1f\t%s\t%s\t%s\n", p.Name, alive.Duration().Round(time.Second), p.Stacks().OfBuff(timeline.BuffMight).EffectiveAverage(alive), uptime(timeline.BuffFury), uptime(timeline.BuffQuickness), uptime(timeline.BuffAlacrity))
	}
	w.Flush()
}

func positioning(tl *timeline.Timeline) {
	boss := tl.Boss()
	section("Positioning (distance to the boss while she is on the platform, sampled every second)")
	// Sabetha leaves the main platform during the champion phases; her
	// position ten seconds in is the platform.
	home := boss.PositionAt(10 * time.Second)
	w := table()
	fmt.Fprintln(w, "player\tavg\tmax\tcannon trips\taway")
	for _, p := range tl.Players {
		var sum, n, farthest float64
		var trips int
		var away, tripStart time.Duration
		onTrip := false
		for t := time.Duration(0); t <= tl.Duration; t += time.Second {
			d := p.DistanceTo(boss, t)
			if math.IsNaN(d) || boss.PositionAt(t).DistTo(home) > 500 {
				continue
			}
			sum, n = sum+d, n+1
			if d > farthest {
				farthest = d
			}
			// A cannon platform is far from the arena: count the trips.
			far := d > 1500
			switch {
			case far && !onTrip:
				onTrip, tripStart = true, t
			case !far && onTrip:
				onTrip = false
				if t-tripStart >= 3*time.Second {
					trips++
					away += t - tripStart
				}
			}
		}
		fmt.Fprintf(w, "%s\t%.0f\t%.0f\t%d\t%v\n", p.Name, sum/n, farthest, trips, away)
	}
	w.Flush()
}

func damageBreakdown(tl *timeline.Timeline) {
	boss := tl.Boss()
	section("Top skills against the boss")
	w := table()
	fmt.Fprintln(w, "skill\tkind\thits\tdamage\tcrit rate\tby")
	shares := boss.HitsTaken().Landed().PerSkill()
	for _, s := range shares[:min(8, len(shares))] {
		kind := "strike"
		if s.Hits.First().IsBuffDamage() {
			kind = "condition"
		}
		var by []string
		for _, c := range s.Hits.PerAgent()[:min(2, len(s.Hits.PerAgent()))] {
			by = append(by, c.Agent.Name)
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%.0f%%\t%s\n", s.Skill.Name, kind, s.Hits.Count(), s.Hits.Damage(), 100*float64(s.Hits.Crits().Count())/float64(s.Hits.Count()), strings.Join(by, ", "))
	}
	w.Flush()
}
