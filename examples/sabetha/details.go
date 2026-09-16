package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/42atomys/evtc/extensions/healingstats"
	"github.com/42atomys/evtc/timeline"
)

// details prints one section per kind of data the fight itself does not
// need, from the session to the extensions.
func details(tl *timeline.Timeline) {
	session(tl)
	agentStates(tl)
	metadata(tl)
	activity(tl)
	effects(tl)
	missiles(tl)
	gadgets(tl)
	extensions(tl)
}

func session(tl *timeline.Timeline) {
	section("Session")
	fmt.Printf("language %v, game build %d, shard %d, ruleset %v, fractal scale %d\n", tl.Language, tl.GameBuild, tl.ShardID, tl.Ruleset, tl.FractalScale)
	fmt.Printf("arcdps %q, instance started %v before the fight, ended by map exit: %v\n", tl.ArcBuild, tl.Start.Sub(tl.InstanceStart).Round(time.Second), tl.EndedByMapExit)
	if c := tl.Commander(); c != nil {
		fmt.Println("commander:", c.Name, "guild", c.Guild)
		for _, m := range c.Markers {
			if m.Commander {
				fmt.Printf("  %v tag (catmander %v) worn %v\n", m.Tag, m.Catmander, m.Interval)
			}
		}
	}
	fmt.Printf("ground markers: %d placements, ping samples: %d\n", len(tl.GroundMarkers), tl.Ping.Len())
	for _, gm := range tl.GroundMarkers {
		fmt.Printf("  %v at %v %v, removed %v\n", gm.Squad, gm.Position, gm.Interval, gm.Removed())
	}
	// A marker on an agent is drawn at the position of the agent.
	for _, a := range tl.Agents {
		for _, m := range a.Markers {
			if m.Squad != timeline.SquadNone {
				fmt.Printf("  %v on %s %v, placed at %v\n", m.Squad, a.Name, m.Interval, a.PositionAt(m.Interval.Start))
			}
		}
	}
	if lo, ok := tl.Ping.MinBetween(tl.Interval()); ok {
		hi, _ := tl.Ping.MaxBetween(tl.Interval())
		fmt.Printf("ping between %d and %d ms\n", lo, hi)
	}
	for p := range tl.Players().Seq() {
		if len(p.Markers) > 0 {
			fmt.Printf("  %s wore %d markers, team %d\n", p.Name, len(p.Markers), p.TeamAt(tl.Duration))
		}
	}
}

func agentStates(tl *timeline.Timeline) {
	section("Agent states")
	w := table()
	fmt.Fprintln(w, "player\tswaps\tsets used\tstealth state\tgliding\ttransformations\tstun breaks")
	span := tl.Interval()
	for p := range tl.Players().Seq() {
		swaps := max(p.WeaponSet.Len()-1, 0)
		sets := map[uint32]bool{}
		for sp := range p.WeaponSet.Seq() {
			sets[sp.Value] = true
		}
		fmt.Fprintf(w, "%s\t%d\t%v\t%d\t%v\t%d\t%d\n", p.Name, swaps, sets, p.StealthAt(tl.Duration),
			p.GlidingTime(span).Round(time.Millisecond), p.Transformation.Len(), len(p.StunBreaks))
	}
	w.Flush()
}

func metadata(tl *timeline.Timeline) {
	section("Skill and buff metadata")
	described, timed, guids := 0, 0, 0
	for _, s := range tl.Skills {
		if s.Info != nil {
			described++
		}
		if len(s.Timings) > 0 {
			timed++
		}
		if !s.GUID.IsZero() {
			guids++
		}
	}
	fmt.Printf("%d skills, %d with cost and range, %d with timings, %d with a GUID\n", len(tl.Skills), described, timed, guids)
	formulas := 0
	for _, b := range tl.Buffs {
		formulas += len(b.Formulas)
	}
	fmt.Printf("%d buffs, %d formulas\n", len(tl.Buffs), formulas)
	if s := tl.Skill(flakShot); s != nil {
		fmt.Printf("%s: cost %.0f, range %.0f to %.0f, tooltip %v, %d timings, guid %v\n", s.Name, s.Cost, s.MinRange, s.MaxRange, s.TooltipTime, len(s.Timings), s.GUID)
	}
	if b := tl.Buff(timeline.BuffMight); b != nil && len(b.Formulas) > 0 {
		f := b.Formulas[0]
		fmt.Printf("might: %v stacking, limit %d, formula type %.0f on attribute %.0f by %.2f\n", b.Stacking, b.StackLimit, f.Type, f.Attribute1, f.Parameter1)
	}
}

func activity(tl *timeline.Timeline) {
	section("Buff activity (queued boons keep one stack in effect)")
	w := table()
	fmt.Fprintln(w, "player\tquickness present\tactive\teffective\tmight present\tactive\teffective")
	at := tl.Duration / 2
	for p := range tl.Players().Seq() {
		q := p.Stacks().OfBuff(timeline.BuffQuickness)
		m := p.Stacks().OfBuff(timeline.BuffMight)
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\n", p.Name, q.CountAt(at), q.ActiveAt(at).Count(), q.EffectiveAt(at), m.CountAt(at), m.ActiveAt(at).Count(), m.EffectiveAt(at))
	}
	w.Flush()
}

func effects(tl *timeline.Timeline) {
	section("Effects")
	all := tl.Effects()
	fmt.Printf("%d effects, %d on the ground, %d around agents, %d removed explicitly\n", all.Count(), all.Ground().Count(), all.Around().Count(), all.Where((*timeline.Effect).Removed).Count())
	w := table()
	fmt.Fprintln(w, "effect id\tguid\tcount\tground\tavg duration\tmostly by")
	for _, s := range all.PerID()[:min(8, len(all.PerID()))] {
		var total time.Duration
		for f := range s.Effects.Seq() {
			total += f.Interval.Duration()
		}
		by := "-"
		if owners := s.Effects.GroupBy(func(f *timeline.Effect) *timeline.Agent { return f.Agent }); len(owners) > 0 {
			best, n := "", 0
			for a, fs := range owners {
				if c := fs.Count(); c > n {
					best, n = a.Name, c
				}
			}
			by = best
		}
		fmt.Fprintf(w, "%d\t%v\t%d\t%v\t%v\t%s\n", s.EffectID, s.GUID, s.Effects.Count(), s.Effects.First().Ground, (total / time.Duration(s.Effects.Count())).Round(time.Millisecond), by)
	}
	w.Flush()
	boss := tl.Boss()
	fmt.Printf("effects placed by the boss: %d, around players: %d, unknown source: %d\n", boss.Effects().Count(), all.Around().Where(func(f *timeline.Effect) bool { return f.Agent.IsPlayer() }).Count(), all.By(tl.Unknown).Count())
}

func missiles(tl *timeline.Timeline) {
	section("Missiles")
	all := tl.Missiles()
	launches, targeted, hits := 0, 0, 0
	for m := range all.Seq() {
		launches += len(m.Launches)
		if m.Target() != nil {
			targeted++
		}
		if m.HitEnemy {
			hits++
		}
	}
	fmt.Printf("%d missiles, %d launches, %d aimed at an agent, %d hit an enemy, %d removed\n", all.Count(), launches, targeted, hits, all.Where((*timeline.Missile).Removed).Count())
	w := table()
	fmt.Fprintln(w, "skill\tmissiles\tby\thit enemy\tavg flight")
	for _, s := range all.PerSkill()[:min(8, len(all.PerSkill()))] {
		var total time.Duration
		hit := 0
		owners := map[string]int{}
		for m := range s.Missiles.Seq() {
			total += m.Interval.Duration()
			if m.HitEnemy {
				hit++
			}
			owners[m.Owner.Name]++
		}
		var names []string
		for n := range owners {
			names = append(names, n)
		}
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%v\n", s.Skill.Name, s.Missiles.Count(), strings.Join(names[:min(2, len(names))], ", "), pct(hit, s.Missiles.Count()), (total / time.Duration(s.Missiles.Count())).Round(time.Millisecond))
	}
	w.Flush()
	if m := tl.Boss().Missiles().First(); m != nil {
		fmt.Printf("first boss missile: %s at %v from %v, %d launches, first aimed at %v\n", m.Skill.Name, m.Interval.Start.Round(time.Millisecond), m.Origin, len(m.Launches), m.Target())
	}
}

func gadgets(tl *timeline.Timeline) {
	section("Gadget animations and names")
	// Every agent may carry animations and a name state, gadgets and the
	// boss included; the busiest ones come first.
	var agents []*timeline.Agent
	for _, a := range tl.Agents {
		if len(a.GadgetAnimations) > 0 || a.NameVisible.Len() > 0 {
			agents = append(agents, a)
		}
	}
	slices.SortStableFunc(agents, func(a, b *timeline.Agent) int {
		return cmp.Compare(len(b.GadgetAnimations), len(a.GadgetAnimations))
	})
	w := table()
	fmt.Fprintln(w, "agent\tanimations\ttokens\tname shown for\tshown when last seen")
	for _, a := range agents[:min(8, len(agents))] {
		tokens := map[uint64]bool{}
		for _, ga := range a.GadgetAnimations {
			tokens[ga.Token] = true
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%v\t%v\n", a.Name, len(a.GadgetAnimations), len(tokens), a.NameVisibleTime(tl.Interval()).Round(time.Second), a.IsNameVisibleAt(a.Lifetime.End))
	}
	w.Flush()
	jumps := 0
	for p := range tl.Players().Seq() {
		jumps += p.Airborne.Len()
	}
	fmt.Printf("%d agents with animations or a name state, %d jump events\n", len(agents), jumps)
}

func extensions(tl *timeline.Timeline) {
	section("Extensions")
	fmt.Printf("%d extension events, %d rewards, %d map changes, %d integrity messages\n", tl.ExtensionEvents().Count(), len(tl.Rewards), len(tl.MapChanges), len(tl.Integrity))
	for _, x := range tl.Extensions {
		fmt.Printf("extension %#x version %q registered at %v wrote %d events, decoded: %v\n", x.Signature, x.Version, x.Time.Round(time.Millisecond), x.Events().Count(), x.Decoded != nil)
	}
	h := healingstats.Of(tl)
	if h == nil {
		fmt.Println("no healing stats in this log")
		return
	}
	var recorded []string
	for _, p := range h.Recorded {
		recorded = append(recorded, p.Name)
	}
	fmt.Printf("healing stats %s (format revision %d): %d heals, %d written by both clients\n", h.Version, h.Revision, h.Heals().Count(), h.Merged)
	fmt.Printf("complete for %s; of the other players, only the heals exchanged with them are known\n", strings.Join(recorded, ", "))
	w := table()
	fmt.Fprintln(w, "player\theals\thealing\thps\tbarrier\tself\ton downed\treceived\trecorded")
	for _, p := range h.Players {
		// HealsCredited includes the mech, pets and clones of the player.
		heals := p.HealsCredited()
		fmt.Fprintf(w, "%s\t%d\t%d\t%.0f\t%d\t%d\t%d\t%d\t%v\n", p.Name, heals.Count(), heals.Healed(), heals.HPS(tl.Interval()), heals.BarrierGiven(), heals.Self().Amount(), heals.Downed().Healed(), p.HealsTaken().Amount(), p.Recorded)
	}
	w.Flush()
	skills := h.Heals().Healing().PerSkill()
	for _, s := range skills[:min(5, len(skills))] {
		fmt.Printf("  %s: %d heals, %d healed, mostly by %s\n", s.Skill.Name, s.Heals.Count(), s.Heals.Healed(), s.Heals.PerAgent()[0].Agent.Name)
	}
	// From a cast to what it healed: the cast that healed the most.
	var best *timeline.Cast
	var bestHeals healingstats.Heals
	for c, heals := range h.Heals().Healing().Where(func(x *healingstats.Heal) bool { return x.Cast != nil }).GroupBy(func(x *healingstats.Heal) *timeline.Cast { return x.Cast }) {
		if best == nil || heals.Healed() > bestHeals.Healed() || (heals.Healed() == bestHeals.Healed() && c.Interval.Start < best.Interval.Start) {
			best, bestHeals = c, heals
		}
	}
	if best != nil {
		fmt.Printf("best cast: %s by %s at %v healed %d over %d heals\n", best.Skill.Name, best.Caster.Name, best.Interval.Start.Round(time.Millisecond), bestHeals.Healed(), bestHeals.Count())
	}
}
