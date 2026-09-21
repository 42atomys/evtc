package timeline

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

// checkInvariants verifies the structural properties every timeline must
// satisfy: exact arena sizing, symmetric edges, sorted lists, contiguous
// state spans and consistent node fields. It is run on synthetic logs, on
// the real log and by the fuzzers.
func checkInvariants(tb testing.TB, tl *Timeline) {
	tb.Helper()
	fail := func(format string, args ...any) {
		tb.Helper()
		tb.Errorf(format, args...)
	}
	// exact reports a carved slice that grew beyond its scan count.
	exact := func(what string, n, c int) {
		if n != c {
			fail("%s: len %d, cap %d (scan count mismatch)", what, n, c)
		}
	}
	agents := append(slices.Clone(tl.agents), tl.Unknown)
	seenAgents := map[*Agent]bool{}
	for _, a := range agents {
		if seenAgents[a] {
			fail("%v listed twice", a)
		}
		seenAgents[a] = true
	}

	hitsBySrc, hitsByDst, castsByCaster, stacksByReceiver, stacksByApplier := 0, 0, 0, 0, 0
	effectsByAgent, missilesByAgent := 0, 0
	for _, a := range agents {
		if a.Timeline != tl {
			fail("%v does not point to its timeline", a)
		}
		exact("hits of "+a.String(), len(a.hits), cap(a.hits))
		exact("hits taken by "+a.String(), len(a.hitsTaken), cap(a.hitsTaken))
		exact("credited hits of "+a.String(), len(a.hitsCredited), cap(a.hitsCredited))
		if want := len(a.hits) + minionHits(a); (a.hitsCredited == nil) != (minionHits(a) == 0) || (a.hitsCredited != nil && len(a.hitsCredited) != want) {
			fail("credited hits of %v: %d, want %d", a, len(a.hitsCredited), want)
		}
		for i, h := range a.hitsCredited {
			if !h.creditedTo(who{one: a}) {
				fail("credited hit %d of %v was dealt by %v", i, a, h.Src)
			}
			if i > 0 && h.Time < a.hitsCredited[i-1].Time {
				fail("credited hits of %v are not sorted", a)
			}
		}
		exact("casts of "+a.String(), len(a.casts), cap(a.casts))
		exact("stacks on "+a.String(), len(a.stacks), cap(a.stacks))
		exact("stacks applied by "+a.String(), len(a.stacksApplied), cap(a.stacksApplied))
		exact("events of "+a.String(), len(a.events), cap(a.events))
		exact("positions of "+a.String(), a.Position.Len(), cap(a.Position.samples))
		exact("velocities of "+a.String(), a.Velocity.Len(), cap(a.Velocity.samples))
		exact("facings of "+a.String(), a.Facing.Len(), cap(a.Facing.samples))
		exact("health of "+a.String(), a.Health.Len(), cap(a.Health.samples))
		exact("barrier of "+a.String(), a.Barrier.Len(), cap(a.Barrier.samples))
		exact("max health of "+a.String(), a.MaxHealth.Len(), cap(a.MaxHealth.samples))
		exact("defiance percent of "+a.String(), a.DefiancePercent.Len(), cap(a.DefiancePercent.samples))
		exact("defiance of "+a.String(), a.Defiance.Len(), cap(a.Defiance.spans))
		exact("targetable of "+a.String(), a.Targetable.Len(), cap(a.Targetable.spans))
		exact("downs of "+a.String(), len(a.Downs), cap(a.Downs))
		exact("deaths of "+a.String(), len(a.Deaths), cap(a.Deaths))
		exact("minions of "+a.String(), len(a.Minions), cap(a.Minions))
		exact("breakbars of "+a.String(), len(a.Breakbars), cap(a.Breakbars))
		exact("markers of "+a.String(), len(a.Markers), cap(a.Markers))
		exact("stun breaks of "+a.String(), len(a.StunBreaks), cap(a.StunBreaks))
		exact("effects of "+a.String(), len(a.effects), cap(a.effects))
		exact("missiles of "+a.String(), len(a.missiles), cap(a.missiles))
		exact("team of "+a.String(), a.Team.Len(), cap(a.Team.spans))
		exact("weapon sets of "+a.String(), a.WeaponSet.Len(), cap(a.WeaponSet.spans))
		exact("stealth of "+a.String(), a.Stealth.Len(), cap(a.Stealth.spans))
		exact("gliding of "+a.String(), a.Gliding.Len(), cap(a.Gliding.spans))
		exact("transformation of "+a.String(), a.Transformation.Len(), cap(a.Transformation.spans))
		exact("airborne of "+a.String(), a.Airborne.Len(), cap(a.Airborne.spans))
		exact("name visibility of "+a.String(), a.NameVisible.Len(), cap(a.NameVisible.spans))
		exact("gadget animations of "+a.String(), len(a.GadgetAnimations), cap(a.GadgetAnimations))
		// The seeded initial span is only added to agents seen in events.
		if c := cap(a.Life.spans) - a.Life.Len(); c < 0 || c > 1 {
			fail("states of %v: len %d, cap %d", a, a.Life.Len(), cap(a.Life.spans))
		}
		if c := cap(a.InCombat.spans) - a.InCombat.Len(); c < 0 || c > 1 {
			fail("combat of %v: len %d, cap %d", a, a.InCombat.Len(), cap(a.InCombat.spans))
		}

		hitsBySrc += len(a.hits)
		hitsByDst += len(a.hitsTaken)
		castsByCaster += len(a.casts)
		stacksByReceiver += len(a.stacks)
		stacksByApplier += len(a.stacksApplied)

		for i, h := range a.hits {
			if h.Src != a {
				fail("hit %d of %v has source %v", i, a, h.Src)
			}
			if i > 0 && h.Time < a.hits[i-1].Time {
				fail("hits of %v are not sorted", a)
			}
			if !a.Lifetime.Contains(h.Time) && a != tl.Unknown {
				fail("hit at %v outside the lifetime %v of %v", h.Time, a.Lifetime, a)
			}
		}
		for i, h := range a.hitsTaken {
			if h.Dst != a {
				fail("hit taken %d by %v has target %v", i, a, h.Dst)
			}
			if i > 0 && h.Time < a.hitsTaken[i-1].Time {
				fail("hits taken by %v are not sorted", a)
			}
		}
		for i, c := range a.casts {
			if c.Caster != a {
				fail("cast %d of %v has caster %v", i, a, c.Caster)
			}
			if i > 0 && c.Interval.Start < a.casts[i-1].Interval.Start {
				fail("casts of %v are not sorted by start", a)
			}
		}
		for i, s := range a.stacks {
			if s.Receiver != a {
				fail("stack %d on %v has receiver %v", i, a, s.Receiver)
			}
			if i > 0 && s.Interval.Start < a.stacks[i-1].Interval.Start {
				fail("stacks on %v are not sorted by start", a)
			}
		}
		for _, s := range a.stacksApplied {
			if s.Applier != a {
				fail("stack applied by %v has applier %v", a, s.Applier)
			}
		}
		for i, e := range a.events {
			if i > 0 && e.Time < a.events[i-1].Time {
				fail("events of %v are not sorted", a)
			}
		}
		checkSeries(tb, a.String()+" position", a.Position)
		checkSeries(tb, a.String()+" velocity", a.Velocity)
		checkSeries(tb, a.String()+" facing", a.Facing)
		checkSeries(tb, a.String()+" health", a.Health.Series)
		checkSeries(tb, a.String()+" barrier", a.Barrier.Series)
		checkSeries(tb, a.String()+" max health", a.MaxHealth.Series)
		checkSeries(tb, a.String()+" defiance percent", a.DefiancePercent.Series)
		checkSpans(tb, a.String()+" states", a.Life, true)
		checkSpans(tb, a.String()+" defiance", a.Defiance, true)
		checkSpans(tb, a.String()+" combat", a.InCombat, true)
		checkSpans(tb, a.String()+" targetable", a.Targetable, true)
		checkSpans(tb, a.String()+" team", a.Team, true)
		checkSpans(tb, a.String()+" weapon set", a.WeaponSet, true)
		checkSpans(tb, a.String()+" stealth", a.Stealth, true)
		checkSpans(tb, a.String()+" gliding", a.Gliding, true)
		checkSpans(tb, a.String()+" transformation", a.Transformation, true)
		checkSpans(tb, a.String()+" airborne", a.Airborne, true)
		checkSpans(tb, a.String()+" name visibility", a.NameVisible, true)
		for i, ga := range a.GadgetAnimations {
			if ga.Agent != a || ga.Event == nil || (i > 0 && ga.Time < a.GadgetAnimations[i-1].Time) {
				fail("gadget animation %d of %v is inconsistent: %+v", i, a, ga)
			}
		}
		if first, ok := a.WeaponSet.First(); ok && (first.Start != a.Lifetime.Start || first.Event != nil) {
			fail("weapon sets of %v do not start with the set before the first swap: %+v", a, first)
		}
		for i, m := range a.Markers {
			if m.Agent != a || m.Event == nil || m.ID == 0 || m.Interval.Start != tl.rel(m.Event.Time) || m.Interval.Start > m.Interval.End || (i > 0 && m.Interval.Start < a.Markers[i-1].Interval.Start) {
				fail("marker %d of %v is inconsistent: %+v", i, a, m)
				continue
			}
			if m.Removed() != (m.Remove != nil) || (m.Remove != nil && (m.Remove.Value != 0 || m.Interval.End != tl.rel(m.Remove.Time))) {
				fail("marker %d of %v has an inconsistent removal: %+v", i, a, m)
			}
			if a != tl.Unknown && m.Interval.End > a.Lifetime.End {
				fail("marker %d of %v ends at %v, after its lifetime %v", i, a, m.Interval.End, a.Lifetime)
			}
			// The commander flag may come from a later write of the marker.
			if tag, cat := tagOf(m.GUID); m.Tag != tag || m.Catmander != cat || m.Squad != squadOf(m.GUID) || ((m.Event.Buff != 0 || tag != TagNone) && !m.Commander) {
				fail("marker %d of %v is misnamed: %+v", i, a, m)
			}
			// The same marker id is worn once at a time.
			for _, o := range a.Markers[i+1:] {
				if o.Interval.Start >= m.Interval.End {
					break
				}
				if o.ID == m.ID {
					fail("markers of %v overlap for id %d: %v and %v", a, m.ID, m.Interval, o.Interval)
				}
			}
		}
		for _, s := range a.StunBreaks {
			if s.Agent != a || s.Event == nil {
				fail("stun break of %v is inconsistent: %+v", a, s)
			}
		}
		effectsByAgent += len(a.effects)
		for i, f := range a.effects {
			if f.Agent != a || f.Create == nil || f.Interval.Start > f.Interval.End || f.Ground != (f.Create.IsStateChange == evtc.StateEffectGroundCreate) || f.Scale == 0 {
				fail("effect %d of %v is inconsistent: %+v", i, a, f)
			}
			if i > 0 && f.Interval.Start < a.effects[i-1].Interval.Start {
				fail("effects of %v are not sorted", a)
			}
			if f.Removed() != (f.Remove != nil) || (f.Remove != nil && !f.closed) {
				fail("effect %d of %v has an inconsistent removal", i, a)
			}
		}
		missilesByAgent += len(a.missiles)
		for i, m := range a.missiles {
			if m.Owner != a || m.Create == nil || m.Skill == nil || m.Interval.Start > m.Interval.End {
				fail("missile %d of %v is inconsistent: %+v", i, a, m)
			}
			exact("launches of a missile of "+a.String(), len(m.Launches), cap(m.Launches))
			exact("effects of a missile of "+a.String(), len(m.Effects), cap(m.Effects))
			for j, l := range m.Launches {
				if !m.Interval.Contains(l.Time) || l.Event == nil || (j > 0 && l.Time < m.Launches[j-1].Time) {
					fail("launch %d of a missile of %v is inconsistent: %+v", j, a, l)
				}
			}
			for _, me := range m.Effects {
				if !m.Interval.Contains(me.Time) || me.Event == nil {
					fail("effect of a missile of %v is inconsistent: %+v", a, me)
				}
			}
			if i > 0 && m.Interval.Start < a.missiles[i-1].Interval.Start {
				fail("missiles of %v are not sorted", a)
			}
		}
		if a.Life.Len() > 0 {
			first, _ := a.Life.First()
			last, _ := a.Life.Last()
			if first.Start != a.Lifetime.Start || first.Value != LifeAlive || last.End != tl.Duration {
				fail("states of %v run %v to %v, lifetime %v, first %v", a, first.Start, last.End, a.Lifetime, first.Value)
			}
		} else if a != tl.Unknown && len(a.events) > 0 {
			fail("%v has events but no states", a)
		}
		for _, d := range a.Downs {
			if d.Agent != a || d.Start > d.End || (d.Cause != nil && (d.Cause.Down != d || d.Cause.Dst != a || !d.Cause.Downing())) {
				fail("down %+v of %v is inconsistent", d, a)
			}
			if d.Death != nil && (d.Death.Down != d || d.Death.Time != d.End) {
				fail("down %+v of %v disagrees with its death", d, a)
			}
			if d.Recovered && d.Death != nil {
				fail("down %+v of %v both recovered and died", d, a)
			}
		}
		for _, de := range a.Deaths {
			if de.Agent != a || (de.Cause != nil && (de.Cause.Death != de || de.Cause.Dst != a || !de.Cause.Killing())) {
				fail("death %+v of %v is inconsistent", de, a)
			}
		}
		for _, m := range a.Minions {
			if m.Master != a {
				fail("minion %v of %v has master %v", m, a, m.Master)
			}
		}
		if a.Master != nil && !slices.Contains(a.Master.Minions, a) {
			fail("%v is missing from the minions of %v", a, a.Master)
		}
		for _, at := range a.AttackTargets {
			if at.Gadget != a {
				fail("attack target %v of %v has gadget %v", at, a, at.Gadget)
			}
		}
		if a.Gadget != nil && !slices.Contains(a.Gadget.AttackTargets, a) {
			fail("%v is missing from the attack targets of %v", a, a.Gadget)
		}
		for _, bb := range a.Breakbars {
			if bb.Agent != a {
				fail("breakbar of %v has agent %v", a, bb.Agent)
			}
			if sp, ok := a.Defiance.At(bb.Start); !ok || sp.Value != DefianceActive || sp.Interval != bb.Interval {
				fail("breakbar %v of %v has no matching active span", bb.Interval, a)
			}
			exact("breakbar hits of "+a.String(), len(bb.hits), cap(bb.hits))
			for _, h := range bb.hits {
				if h.Dst != a || !h.IsDefiance() || !bb.Contains(h.Time) {
					fail("breakbar hit %+v of %v does not belong", h, a)
				}
			}
			for s := range bb.Percent.Seq() {
				if !bb.Contains(s.Time) {
					fail("breakbar sample at %v of %v outside %v", s.Time, a, bb.Interval)
				}
			}
		}
		if a.Player != nil && a.Kind != KindPlayer {
			fail("player link of %v is broken", a)
		}
		if c := a.Character; c != nil && (c.Agent != a || !slices.Contains(a.Player.characters, c)) {
			fail("character link of %v is broken", a)
		}
		if a.Target != nil && a.Target.Agent != a {
			fail("target link of %v is broken", a)
		}
		if a == tl.Unknown && (a.Lifetime != (Interval{}) || a.Master != nil || len(a.Minions) != 0) {
			fail("the Unknown sentinel has a lifetime, a master or minions: %v %v %d", a.Lifetime, a.Master, len(a.Minions))
		}
		if a.Master == tl.Unknown {
			fail("%v has the Unknown sentinel as master", a)
		}
	}

	all := tl.Hits().All()
	if effectsByAgent != len(tl.effects) || cap(tl.effects) != len(tl.effects) || missilesByAgent != len(tl.missiles) || cap(tl.missiles) != len(tl.missiles) {
		fail("effects %d of %d, missiles %d of %d", effectsByAgent, len(tl.effects), missilesByAgent, len(tl.missiles))
	}
	for i := 1; i < len(tl.effects); i++ {
		if tl.effects[i].Interval.Start < tl.effects[i-1].Interval.Start {
			fail("global effects are not sorted")
		}
	}
	for i := 1; i < len(tl.missiles); i++ {
		if tl.missiles[i].Interval.Start < tl.missiles[i-1].Interval.Start {
			fail("global missiles are not sorted")
		}
	}
	exact("ping samples", tl.Ping.Len(), cap(tl.Ping.samples))
	exact("ground markers", len(tl.GroundMarkers), cap(tl.GroundMarkers))
	checkSeries(tb, "ping", tl.Ping.Series)
	for i, gm := range tl.GroundMarkers {
		if gm.Event == nil {
			fail("ground marker %d has no event", i)
			continue
		}
		if _, removal := groundMarkerPlace(gm.Event); removal || gm.Squad != squadOfIndex(gm.Index) || gm.Event.SkillID != gm.Index || gm.Interval.Start != tl.rel(gm.Event.Time) || gm.Interval.Start > gm.Interval.End || gm.Interval.End > tl.Duration || (i > 0 && gm.Interval.Start < tl.GroundMarkers[i-1].Interval.Start) {
			fail("ground marker %d is inconsistent: %+v", i, gm)
		}
		if gm.Removed() != (gm.Remove != nil) {
			fail("ground marker %d has an inconsistent removal: %+v", i, gm)
		}
		if gm.Remove != nil {
			if _, removal := groundMarkerPlace(gm.Remove); !removal || gm.Remove.SkillID != gm.Index || gm.Interval.End != tl.rel(gm.Remove.Time) {
				fail("ground marker %d was not ended by its removal: %+v", i, gm)
			}
		}
		// A squad marker is on the ground once at a time.
		for _, o := range tl.GroundMarkers[i+1:] {
			if o.Interval.Start >= gm.Interval.End {
				break
			}
			if o.Index == gm.Index {
				fail("ground markers overlap for index %d: %v and %v", gm.Index, gm.Interval, o.Interval)
			}
		}
	}
	exact("rewards", len(tl.Rewards), cap(tl.Rewards))
	exact("map changes", len(tl.MapChanges), cap(tl.MapChanges))
	exact("extensions", len(tl.Extensions), cap(tl.Extensions))
	exact("integrity messages", len(tl.Integrity), cap(tl.Integrity))
	for i, m := range tl.Integrity {
		if m.Event == nil || m.Event.IsStateChange != evtc.StateIntegrity {
			fail("integrity message %d is inconsistent: %+v", i, m)
		}
	}
	if len(tl.extensions) != len(registeredSignatures(tl)) {
		fail("%d extensions indexed for %d signatures", len(tl.extensions), len(registeredSignatures(tl)))
	}
	for sig, x := range tl.extensions {
		if x == nil || x.Signature != sig || tl.Extension(sig) != x {
			fail("extension index of %#x is inconsistent: %+v", sig, x)
		}
	}
	for i, r := range tl.Rewards {
		if r.Event == nil || r.Event.IsStateChange != evtc.StateReward || (i > 0 && r.Time < tl.Rewards[i-1].Time) {
			fail("reward %d is inconsistent: %+v", i, r)
		}
	}
	for i, mc := range tl.MapChanges {
		if mc.Event == nil || mc.Event.IsStateChange != evtc.StateMapChange || (i > 0 && mc.Time < tl.MapChanges[i-1].Time) {
			fail("map change %d is inconsistent: %+v", i, mc)
		}
	}
	// Extension combat events belong to the first registration of their
	// signature, all of them and nothing else.
	combatBySig := map[uint32]int{}
	for _, e := range tl.events {
		if e.IsStateChange == evtc.StateExtensionCombat {
			combatBySig[extensionSignature(e)]++
		}
	}
	registered := map[uint32]bool{}
	for i, x := range tl.Extensions {
		if x.Event == nil || x.Event.IsStateChange != evtc.StateExtension || x.Timeline != tl || x.Signature != uint32(x.Event.SrcAgent) || (i > 0 && x.Time < tl.Extensions[i-1].Time) {
			fail("extension %d is inconsistent: %+v", i, x)
		}
		exact("events of extension "+strconv.Itoa(i), len(x.events), cap(x.events))
		if want := combatBySig[x.Signature]; registered[x.Signature] {
			if len(x.events) != 0 {
				fail("extension %d, a second registration of %#x, owns %d events", i, x.Signature, len(x.events))
			}
		} else if len(x.events) != want {
			fail("extension %d owns %d events, want %d", i, len(x.events), want)
		}
		if x.Decoded != nil && (registered[x.Signature] || extensionDecoder(x.Signature) == nil) {
			fail("extension %d holds decoded data without a decoder of its own: %+v", i, x.Decoded)
		}
		registered[x.Signature] = true
		for j, e := range x.events {
			if e.IsStateChange != evtc.StateExtensionCombat || extensionSignature(e) != x.Signature || (j > 0 && e.Time < x.events[j-1].Time) || tl.ExtensionOf(e) != x {
				fail("event %d of extension %d is inconsistent: %+v", j, i, e)
			}
		}
	}
	// A stack ended by the despawn of its receiver has no removal and its
	// receiver is gone from there; a stack still open at the end cannot sit
	// on an agent that left after the application.
	for i, s := range tl.stacks {
		exact("changes of stack "+strconv.Itoa(i), len(s.Changes), cap(s.Changes))
		for j, c := range s.Changes {
			if c.IsStateChange != evtc.StateBuffChange || trackableID(c) != s.ID || !s.Interval.Contains(tl.rel(c.Time)) || (j > 0 && c.Time < s.Changes[j-1].Time) {
				fail("change %d of stack %d is inconsistent: %+v", j, i, c)
			}
		}
		if s.EndedByDespawn && (s.Remove != nil || s.Superseded || s.Receiver.LifeStateAt(s.Interval.End) != LifeGone) {
			fail("stack %d ended by despawn is inconsistent: %+v", i, s)
		}
		if s.Remove == nil && !s.Superseded && !s.EndedByDespawn && s.Receiver != tl.Unknown {
			if gone, ok := s.Receiver.Life.Last(); ok && gone.Value == LifeGone && gone.Start > s.Interval.Start {
				fail("stack %d is open on %v, gone since %v", i, s.Receiver, gone.Start)
			}
		}
	}
	// States are bounded by the lifetime, Life excepted.
	for _, a := range agents {
		if a == tl.Unknown {
			continue
		}
		for what, end := range map[string]spanEnd{
			"defiance": ending(a.Defiance), "combat": ending(a.InCombat), "targetable": ending(a.Targetable),
			"team": ending(a.Team), "weapon set": ending(a.WeaponSet), "stealth": ending(a.Stealth),
			"gliding": ending(a.Gliding), "transformation": ending(a.Transformation),
			"airborne": ending(a.Airborne), "name visibility": ending(a.NameVisible),
		} {
			if end.ok && end.at > a.Lifetime.End {
				fail("%s of %v ends at %v, after its lifetime %v", what, a, end.at, a.Lifetime)
			}
		}
	}
	for _, s := range tl.Skills {
		exact("timings of "+s.String(), len(s.Timings), cap(s.Timings))
		exact("missiles of "+s.String(), len(s.missiles), cap(s.missiles))
		for _, m := range s.missiles {
			if m.Skill != s {
				fail("missile of %v points to %v", s, m.Skill)
			}
		}
	}
	for _, buff := range tl.Buffs {
		exact("formulas of "+buff.String(), len(buff.Formulas), cap(buff.Formulas))
	}
	if hitsBySrc != len(all) || hitsByDst != len(all) || cap(tl.hits) != len(tl.hits) {
		fail("hits: %d by source, %d by target, %d total", hitsBySrc, hitsByDst, len(all))
	}
	for i, h := range all {
		if i > 0 && h.Time < all[i-1].Time {
			fail("global hits are not sorted")
		}
		if h.Src == nil || h.Dst == nil || h.Skill == nil || h.Event == nil {
			fail("hit %d has nil links: %+v", i, h)
		}
		if h.Cast != nil && (h.Cast.Caster != h.Src || h.Cast.Skill != h.Skill || !slices.Contains(h.Cast.hits, h) || h.Time < h.Cast.Interval.Start) {
			fail("hit %d disagrees with its cast %+v", i, h.Cast)
		}
		if h.Down != nil && h.Down.Cause != h {
			fail("hit %d is not the cause of its down", i)
		}
		if h.Death != nil && h.Death.Cause != h {
			fail("hit %d is not the cause of its death", i)
		}
	}
	casts := tl.Casts().All()
	if castsByCaster != len(casts) || cap(tl.casts) != len(tl.casts) {
		fail("casts: %d by caster, %d total", castsByCaster, len(casts))
	}
	for i, c := range casts {
		if i > 0 && c.Interval.Start < casts[i-1].Interval.Start {
			fail("global casts are not sorted by start")
		}
		if c.Caster == nil || c.Skill == nil || (c.Start == nil && c.Stop == nil) || c.Interval.Start > c.Interval.End {
			fail("cast %d is inconsistent: %+v", i, c)
		}
		exact("hits of a cast", len(c.hits), cap(c.hits))
		for _, h := range c.hits {
			if h.Cast != c {
				fail("hit of cast %d points to another cast", i)
			}
		}
		if c.Ended() != (c.Stop != nil) || c.Full() != (c.Activation == evtc.ActivationReset) || c.Completed() != (c.Full() || c.Activation == evtc.ActivationMinimum || c.Activation == evtc.ActivationNoData) {
			fail("cast %d predicates are inconsistent", i)
		}
	}
	stacks := tl.Stacks().All()
	for i, s := range stacks {
		exact("activity of a stack", s.Active.Len(), cap(s.Active.spans))
		checkSpans(tb, "stack activity", s.Active, true)
		first, _ := s.Active.First()
		last, _ := s.Active.Last()
		if s.Active.Len() == 0 || first.Start != s.Interval.Start || first.Value != s.ActiveOnApply || first.Event != nil || last.End != s.Interval.End {
			fail("activity of stack %d %v does not cover its interval %v", i, s.Active.All(), s.Interval)
		}
	}
	if stacksByReceiver != len(stacks) || stacksByApplier != len(stacks) || cap(tl.stacks) != len(tl.stacks) {
		fail("stacks: %d by receiver, %d by applier, %d total", stacksByReceiver, stacksByApplier, len(stacks))
	}
	for i, s := range stacks {
		if i > 0 && s.Interval.Start < stacks[i-1].Interval.Start {
			fail("global stacks are not sorted by start")
		}
		if s.Buff == nil || s.Buff.Skill.Buff != s.Buff || s.Applier == nil || s.Receiver == nil || s.Apply == nil {
			fail("stack %d has nil links: %+v", i, s)
		}
		if s.Interval.Start > s.Interval.End {
			fail("stack %d runs backwards: %v", i, s.Interval)
		}
		if s.Open() && s.Interval.End != tl.Duration {
			fail("open stack %d ends at %v, not at the log end %v", i, s.Interval.End, tl.Duration)
		}
		if s.Remove != nil && s.Remove.IsStateChange == evtc.StateBuffRemoveSingle && trackableID(s.Remove) != s.ID {
			fail("stack %d was closed by the removal of another id", i)
		}
		if s.Superseded && (s.Remove != nil || s.Open()) {
			fail("superseded stack %d has a removal or is open", i)
		}
	}

	skillHits, skillCasts, buffStacks := 0, 0, 0
	for i, s := range tl.Skills {
		if i > 0 && s.ID <= tl.Skills[i-1].ID {
			fail("skills are not sorted by unique id")
		}
		if tl.Skill(s.ID) != s || s.Name == "" {
			fail("skill %v is not registered", s)
		}
		exact("hits of skill "+s.String(), len(s.hits), cap(s.hits))
		exact("casts of skill "+s.String(), len(s.casts), cap(s.casts))
		skillHits += len(s.hits)
		skillCasts += len(s.casts)
		for _, h := range s.hits {
			if h.Skill != s {
				fail("hit of skill %v has skill %v", s, h.Skill)
			}
		}
		for j, c := range s.casts {
			if c.Skill != s {
				fail("cast of skill %v has skill %v", s, c.Skill)
			}
			if j > 0 && c.Interval.Start < s.casts[j-1].Interval.Start {
				fail("casts of skill %v are not sorted", s)
			}
		}
		if s.Buff != nil && (s.Buff.Skill != s || tl.Buff(s.ID) != s.Buff) {
			fail("buff link of %v is broken", s)
		}
	}
	for i, buff := range tl.Buffs {
		if i > 0 && buff.Skill.ID <= tl.Buffs[i-1].Skill.ID {
			fail("buffs are not sorted by unique id")
		}
		exact("stacks of buff "+buff.String(), len(buff.stacks), cap(buff.stacks))
		buffStacks += len(buff.stacks)
		for _, s := range buff.stacks {
			if s.Buff != buff {
				fail("stack of buff %v has buff %v", buff, s.Buff)
			}
		}
	}
	if skillHits != len(all) || skillCasts != len(casts) || buffStacks != len(stacks) {
		fail("skill edges: %d hits, %d casts, %d stacks", skillHits, skillCasts, buffStacks)
	}

	seenTargets := map[*Agent]bool{}
	for i, tg := range tl.targets {
		if tg.Agent == nil || tg.Agent.Target != tg || seenTargets[tg.Agent] {
			fail("target %d is inconsistent: %+v", i, tg)
		}
		seenTargets[tg.Agent] = true
		if tg.Boss && i > 0 && !tl.targets[i-1].Boss {
			fail("boss %v listed after a non-boss target", tg)
		}
	}
	for _, p := range tl.players {
		if len(p.characters) == 0 || !slices.Contains(p.characters, p.main) {
			fail("player %v has no main character", p)
		}
		for _, c := range p.characters {
			if c.Player != p || c.Kind != KindPlayer {
				fail("character %v of %v is inconsistent", c, p)
			}
		}
	}
	for a := range tl.NPCs().Seq() {
		if a.Kind != KindNPC {
			fail("%v listed as NPC", a)
		}
	}
	for a := range tl.Gadgets().Seq() {
		if a.Kind != KindGadget {
			fail("%v listed as gadget", a)
		}
	}
	if tl.POV != nil && tl.POV.Ref().Player != tl.POV {
		fail("POV is inconsistent")
	}
	events := tl.Events().All()
	for i, e := range events {
		if i > 0 && e.Time < events[i-1].Time {
			fail("global events are not sorted")
		}
		if !hasTime(e.IsStateChange) {
			fail("untimed event %v listed", e.IsStateChange)
		}
	}
	if tl.Duration < 0 {
		fail("negative duration %v", tl.Duration)
	}
}

// checkSeries verifies that samples are sorted.
func checkSeries[T any](tb testing.TB, what string, s Series[T]) {
	tb.Helper()
	var prev time.Duration
	for i, smp := range s.Samples() {
		if i > 0 && smp.Time < prev {
			tb.Errorf("%s: samples are not sorted", what)
			return
		}
		if smp.Event == nil {
			tb.Errorf("%s: sample %d has no event", what, i)
		}
		prev = smp.Time
	}
}

// checkSpans verifies that spans are sorted, well formed and, when
// contiguous is set, that each span ends where the next one starts.
func checkSpans[T any](tb testing.TB, what string, s Spans[T], contiguous bool) {
	tb.Helper()
	spans := s.All()
	for i, sp := range spans {
		if sp.Start > sp.End {
			tb.Errorf("%s: span %d runs backwards: %v", what, i, sp.Interval)
		}
		if i == 0 {
			continue
		}
		prev := spans[i-1]
		if sp.Start < prev.Start || (contiguous && prev.End != sp.Start) {
			tb.Errorf("%s: span %d %v does not follow %v", what, i, sp.Interval, prev.Interval)
		}
	}
}

func TestInvariantsSynthetic(t *testing.T) {
	for _, o := range []genOptions{
		{players: 1, adds: 0, duration: 5 * time.Second, seed: 1},
		{players: 5, adds: 3, duration: 60 * time.Second, seed: 2},
		{players: 10, adds: 20, duration: 240 * time.Second, seed: 3},
	} {
		l := genLog(o)
		tl := mustBuild(t, l)
		t.Logf("seed %d: %d events, %d hits, %d casts, %d stacks, %d targets", o.seed, len(l.Events), tl.Hits().Count(), tl.Casts().Count(), tl.Stacks().Count(), len(tl.targets))
		checkInvariants(t, tl)
		if tl.Hits().Count() == 0 || tl.Casts().Count() == 0 || tl.Stacks().Count() == 0 || len(tl.targets) == 0 {
			t.Errorf("seed %d generated an empty log", o.seed)
		}
	}
}

func TestInvariantsSample(t *testing.T) {
	checkInvariants(t, loadSample(t))
}

// TestQueriesAgainstBruteForce compares the binary searches of the
// temporal primitives with linear scans on random intervals.
func TestQueriesAgainstBruteForce(t *testing.T) {
	tl := mustBuild(t, genLog(genOptions{players: 8, adds: 6, duration: 120 * time.Second, seed: 42}))
	r := rand.New(rand.NewPCG(7, 11))
	randomInterval := func() Interval {
		a := time.Duration(r.Int64N(int64(tl.Duration)+int64(2*time.Second))) - time.Second
		b := time.Duration(r.Int64N(int64(tl.Duration)+int64(2*time.Second))) - time.Second
		return NewInterval(a, b)
	}
	hits := tl.Hits().All()
	casts := tl.Casts().All()
	stacks := tl.Stacks().All()
	boss := tl.targets[0]
	p := tl.characters[0]

	for range 200 {
		iv := randomInterval()
		t0 := iv.Start

		want := 0
		for _, h := range hits {
			if iv.Contains(h.Time) {
				want++
			}
		}
		if got := tl.Hits().Between(iv).Count(); got != want {
			t.Errorf("Hits.Between(%v) = %d, want %d", iv, got, want)
		}
		want = 0
		for _, c := range casts {
			if c.Interval.Overlaps(iv) {
				want++
			}
		}
		if got := tl.Casts().Between(iv).Count(); got != want {
			t.Errorf("Casts.Between(%v) = %d, want %d", iv, got, want)
		}
		want = 0
		for _, s := range stacks {
			if s.Interval.Overlaps(iv) {
				want++
			}
		}
		if got := tl.Stacks().Between(iv).Count(); got != want {
			t.Errorf("Stacks.Between(%v) = %d, want %d", iv, got, want)
		}
		want = 0
		for _, s := range p.stacks {
			if s.Interval.Contains(t0) {
				want++
			}
		}
		if got := p.Stacks().CountAt(t0); got != want {
			t.Errorf("CountAt(%v) = %d, want %d", t0, got, want)
		}
		if got, want := p.Stacks().Uptime(iv), bruteUptime(p.stacks, iv); got != want {
			t.Errorf("Uptime(%v) = %v, want %v", iv, got, want)
		}
		want = 0
		for _, e := range tl.events {
			if iv.Contains(tl.TimeOf(e)) {
				want++
			}
		}
		if got := tl.Events().Between(iv).Count(); got != want {
			t.Errorf("Events.Between(%v) = %d, want %d", iv, got, want)
		}
		hv, hok := boss.Health.At(t0)
		if got, want := (valueOK[float64]{hv, hok}), bruteStep(boss.Health, t0); got != want {
			t.Errorf("Health.At(%v) = %v, want %v", t0, got, want)
		}
		pv, pok := p.Position.At(t0)
		if got, want := (valueOK[Vec3]{pv, pok}), brutePosition(p.Position, t0); got != want {
			t.Errorf("Position.At(%v) = %v, want %v", t0, got, want)
		}
		if got, want := p.LifeStateAt(t0), bruteState(p.Life, t0); got != want {
			t.Errorf("LifeStateAt(%v) = %v, want %v", t0, got, want)
		}
		sub := boss.Health.Between(iv)
		want = 0
		for _, s := range boss.Health.Samples() {
			if iv.Contains(s.Time) {
				want++
			}
		}
		if sub.Len() != want {
			t.Errorf("Health.Between(%v) = %d samples, want %d", iv, sub.Len(), want)
		}
	}
	if got, want := boss.Health.Crossings(75, 50, 25), bruteCrossings(boss.Health, 75, 50, 25); !slices.Equal(got, want) {
		t.Errorf("Crossings = %v, want %v", got, want)
	}
}

type valueOK[T comparable] struct {
	v  T
	ok bool
}

func bruteStep(n Numbers[float64], t time.Duration) valueOK[float64] {
	if !n.Interval().Contains(t) {
		return valueOK[float64]{}
	}
	var last *Sample[float64]
	for i := range n.samples {
		if n.samples[i].Time <= t {
			last = &n.samples[i]
		}
	}
	if last == nil {
		if n.holdBefore && len(n.samples) > 0 {
			return valueOK[float64]{n.samples[0].Value, true}
		}
		return valueOK[float64]{}
	}
	return valueOK[float64]{last.Value, true}
}

func brutePosition(s Series[Vec3], t time.Duration) valueOK[Vec3] {
	if !s.Interval().Contains(t) {
		return valueOK[Vec3]{}
	}
	for i := range s.samples {
		if s.samples[i].Time > t {
			if i == 0 {
				return valueOK[Vec3]{}
			}
			prev, next := s.samples[i-1], s.samples[i]
			gap := next.Time - prev.Time
			if next.Break || gap > MoveGap {
				return valueOK[Vec3]{prev.Value, true}
			}
			return valueOK[Vec3]{LerpVec3(prev.Value, next.Value, float64(t-prev.Time)/float64(gap)), true}
		}
	}
	if len(s.samples) == 0 {
		return valueOK[Vec3]{}
	}
	return valueOK[Vec3]{s.samples[len(s.samples)-1].Value, true}
}

func bruteState(s Spans[LifeState], t time.Duration) LifeState {
	for _, sp := range s.All() {
		if sp.Contains(t) {
			return sp.Value
		}
	}
	return LifeUnknown
}

func bruteUptime(stacks []*BuffStack, iv Interval) time.Duration {
	var clipped []Interval
	for _, s := range stacks {
		if c, ok := s.Interval.Intersect(iv); ok {
			clipped = append(clipped, c)
		}
	}
	slices.SortFunc(clipped, func(a, b Interval) int { return cmp.Compare(a.Start, b.Start) })
	var total time.Duration
	var cur Interval
	for i, c := range clipped {
		if i == 0 {
			cur = c
			continue
		}
		if c.Start <= cur.End {
			cur.End = max(cur.End, c.End)
			continue
		}
		total += cur.Duration()
		cur = c
	}
	if len(clipped) > 0 {
		total += cur.Duration()
	}
	return total
}

func bruteCrossings(n Numbers[float64], levels ...float64) []Crossing[float64] {
	var out []Crossing[float64]
	s := n.Samples()
	for i := 1; i < len(s); i++ {
		for _, level := range levels {
			if s[i-1].Value >= level && s[i].Value < level {
				out = append(out, Crossing[float64]{Time: s[i].Time, Level: level, Direction: Falling, From: s[i-1], To: s[i]})
			}
			if s[i-1].Value < level && s[i].Value >= level {
				out = append(out, Crossing[float64]{Time: s[i].Time, Level: level, Direction: Rising, From: s[i-1], To: s[i]})
			}
		}
	}
	return out
}

// spanEnd is the end of the last span of a state, when it has one.
type spanEnd struct {
	at time.Duration
	ok bool
}

func ending[T any](s Spans[T]) spanEnd {
	last, ok := s.Last()
	return spanEnd{last.End, ok}
}

// registeredSignatures returns the distinct signatures of the extensions.
func registeredSignatures(tl *Timeline) map[uint32]bool {
	sigs := map[uint32]bool{}
	for _, x := range tl.Extensions {
		sigs[x.Signature] = true
	}
	return sigs
}
