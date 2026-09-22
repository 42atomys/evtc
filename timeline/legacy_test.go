package timeline

import (
	"crypto/sha256"
	"fmt"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

// legacyBuild is the header of the logs asLegacy writes: the newest build
// of the older format among the logs this package was checked on.
const legacyBuild = "20260416"

// asLegacy encodes a log of the typed format the way arcdps wrote the same
// facts before 20260501, and returns next to it the typed log restricted
// to what that format can say: both must build the same timeline. The
// older format names no target on a cast start, reads as a buff tick a
// duration change or an application of 0 ms, and as a strike a cast end
// that does not say how the cast ended.
func asLegacy(l *evtc.Log) (typed, legacy *evtc.Log) {
	typed = &evtc.Log{Header: l.Header, Agents: l.Agents, Skills: l.Skills}
	legacy = &evtc.Log{Header: l.Header, Agents: l.Agents, Skills: l.Skills}
	legacy.Header.Build = legacyBuild
	for _, e := range l.Events {
		switch e.IsStateChange {
		case evtc.StateBuffApply, evtc.StateBuffChange:
			if e.Value == 0 {
				continue
			}
		case evtc.StateAnimationStop:
			if e.IsActivation == evtc.ActivationNone {
				continue
			}
		case evtc.StateAnimationStart:
			e.DstAgent, e.DstInstanceID, e.DstMasterInstanceID = 0, 0, 0
		}
		typed.Events = append(typed.Events, e)

		switch e.IsStateChange {
		case evtc.StateAnimationStart:
			e.IsActivation = evtc.ActivationStartDefunc
			// What the logs hold there is no agent.
			e.DstAgent = 0x000001dc30ea3066
		case evtc.StateAnimationStop:
		case evtc.StateBuffApply:
			e.Buff, e.BuffDamage, e.IsOffcycle = 1, 0, 0
		case evtc.StateBuffChange:
			e.Buff, e.BuffDamage, e.IsOffcycle = 1, 0, 1
		case evtc.StateBuffRemoveSingle:
			if e.IsBuffRemove != evtc.BuffRemoveManual {
				e.IsBuffRemove = evtc.BuffRemoveSingle
			}
		case evtc.StateBuffRemoveAll:
			e.IsBuffRemove = evtc.BuffRemoveAll
		case evtc.StateCombat:
			if e.Buff != 0 {
				e.Result, e.IsOffcycle, e.Pad61 = legacyTickFields(e)
			}
			legacy.Events = append(legacy.Events, e)
			continue
		default:
			legacy.Events = append(legacy.Events, e)
			continue
		}
		e.IsStateChange = evtc.StateCombat
		legacy.Events = append(legacy.Events, e)
	}
	return typed, legacy
}

// legacyTickFields moves the fields of a typed buff tick where the older
// format keeps them: the cycle in is_offcycle, the downed flag in pad61,
// and a result that only tells a negated tick.
func legacyTickFields(e evtc.Event) (result evtc.Result, cycle, downed uint8) {
	downed = e.IsOffcycle
	switch e.Result {
	case evtc.ResultBuffDamageCycle:
		return 0, uint8(evtc.BuffCycleCycle), downed
	case evtc.ResultBuffDamageNotCycle:
		return 0, uint8(evtc.BuffCycleNotCycle), downed
	case evtc.ResultBuffDamageNotCycleDmgToTargetOnHit:
		return 0, uint8(evtc.BuffCycleNotCycleDmgToTargetOnHit), downed
	case evtc.ResultBuffDamageNotCycleDmgToSourceOnHit:
		return 0, uint8(evtc.BuffCycleNotCycleDmgToSourceOnHit), downed
	case evtc.ResultBuffDamageNotCycleDmgToTargetOnStackRemove:
		return 0, uint8(evtc.BuffCycleNotCycleDmgToTargetOnStackRemove), downed
	case evtc.ResultAbsorb:
		// 1: invulnerable through a buff.
		return 1, 0, downed
	}
	return e.Result, 0, downed
}

// fingerprint passes to emit one line per agent, hit, cast, stack and
// effect, with the values read from the log and without the raw events:
// two logs telling the same facts in two formats give the same lines.
func fingerprint(tl *Timeline, emit func(line string)) {
	idx := func(a *Agent) int {
		if a == nil {
			return -1
		}
		return a.idx
	}
	add := func(format string, args ...any) { emit(fmt.Sprintf(format, args...)) }
	castIdx := make(map[*Cast]int, len(tl.casts))
	for i, c := range tl.casts {
		castIdx[c] = i
	}
	for _, a := range append(slices.Clone(tl.agents), tl.Unknown) {
		add("agent %d %v %q life %v events %d hits %d taken %d casts %d stacks %d applied %d effects %d downs %d deaths %d master %d",
			a.idx, a.Kind, a.Name, a.Lifetime, len(a.events), len(a.hits), len(a.hitsTaken), len(a.casts), len(a.stacks), len(a.stacksApplied),
			len(a.effects), len(a.Downs), len(a.Deaths), idx(a.Master))
		for _, d := range a.Downs {
			add("  down %v recovered %v cause %v", d.Interval, d.Recovered, d.Cause != nil)
		}
		for _, d := range a.Deaths {
			add("  death %v cause %v", d.Time, d.Cause != nil)
		}
	}
	for _, h := range tl.hits {
		cast := -1
		if h.Cast != nil {
			cast = castIdx[h.Cast]
		}
		add("hit %v %d>%d skill %d %v dmg %d barrier %d buff %v iff %v flags %v %v %v %v %v %v %v cast %d", h.Time, idx(h.Src), idx(h.Dst), h.Skill.ID, h.Result,
			h.Damage, h.Barrier, h.IsBuff, h.IFF, h.OverNinety, h.UnderFifty, h.Moving, h.TargetMoving, h.Flanking, h.Shielded, h.TargetDowned, cast)
	}
	for i, c := range tl.casts {
		add("cast %d %v by %d skill %d target %d expected %v control %v elapsed %v unscaled %v %v start %v stop %v hits %d", i, c.Interval, idx(c.Caster),
			c.Skill.ID, idx(c.Target), c.Expected, c.Control, c.Elapsed, c.ElapsedUnscaled, c.Activation, c.Start != nil, c.Stop != nil, len(c.hits))
	}
	for _, s := range tl.stacks {
		var active []string
		for _, sp := range s.Active.All() {
			active = append(active, fmt.Sprintf("%v:%v", sp.Interval, sp.Value))
		}
		add("stack %d buff %d %d>%d %v applied %v extended %v initial %v original %v active %v removal %v by %d remaining %v despawn %v superseded %v changes %d spans %v",
			s.ID, s.Buff.Skill.ID, idx(s.Applier), idx(s.Receiver), s.Interval, s.Applied, s.Extended, s.Initial, s.Original, s.ActiveOnApply, s.Removal,
			idx(s.RemovedBy), s.Remaining, s.EndedByDespawn, s.Superseded, len(s.Changes), active)
	}
	for _, f := range tl.effects {
		add("effect %d id %d on %d ground %v %v %v", f.ID, f.EffectID, idx(f.Agent), f.Ground, f.Interval, f.Duration)
	}
}

// sameTimeline fails the test at the first line the fingerprints of two
// timelines differ on. It compares digests first: a raid log gives
// millions of lines.
func sameTimeline(tb testing.TB, typed, legacy *Timeline) {
	tb.Helper()
	digest := func(tl *Timeline) [sha256.Size]byte {
		h := sha256.New()
		fingerprint(tl, func(line string) { io.WriteString(h, line+"\n") })
		return [sha256.Size]byte(h.Sum(nil))
	}
	if digest(typed) == digest(legacy) {
		return
	}
	var a, b []string
	fingerprint(typed, func(line string) { a = append(a, line) })
	fingerprint(legacy, func(line string) { b = append(b, line) })
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			tb.Errorf("line %d differs:\n typed  %s\n legacy %s", i, a[i], b[i])
			return
		}
	}
	tb.Errorf("%d lines from the typed log, %d from the legacy one", len(a), len(b))
}

// buildBoth builds a typed log and its legacy encoding, and checks the
// invariants of the second.
func buildBoth(tb testing.TB, l *evtc.Log) (typed, legacy *Timeline) {
	tb.Helper()
	typedLog, legacyLog := asLegacy(l)
	typed, legacy = mustBuild(tb, typedLog), mustBuild(tb, legacyLog)
	if legacy.Has(evtc.CapabilityTypedEvents) {
		tb.Fatal("the legacy encoding still holds typed events")
	}
	checkInvariants(tb, legacy)
	return typed, legacy
}

func TestLegacyKind(t *testing.T) {
	for _, tt := range []struct {
		name string
		e    evtc.Event
		want evtc.StateChange
	}{
		{"strike", evtc.Event{Value: 500, Result: evtc.ResultStrikeDamageCrit}, evtc.StateCombat},
		{"buff tick", evtc.Event{Buff: 1, BuffDamage: 120}, evtc.StateCombat},
		{"negated buff tick", evtc.Event{Buff: 1, Result: 1}, evtc.StateCombat},
		{"cast start", evtc.Event{IsActivation: evtc.ActivationStartDefunc, Value: 750, IFF: evtc.IFFUnknown}, evtc.StateAnimationStart},
		{"cast start under quickness", evtc.Event{IsActivation: evtc.ActivationQuicknessDefunc}, evtc.StateAnimationStart},
		{"cast end", evtc.Event{IsActivation: evtc.ActivationReset, Value: 750}, evtc.StateAnimationStop},
		{"cast end without data", evtc.Event{IsActivation: evtc.ActivationNoData}, evtc.StateAnimationStop},
		{"removal of every stack", evtc.Event{Buff: 1, IsBuffRemove: evtc.BuffRemoveAll, Value: 3000, BuffDamage: 3000}, evtc.StateBuffRemoveAll},
		{"removal of one stack", evtc.Event{Buff: 1, IsBuffRemove: evtc.BuffRemoveSingle, Value: 3000}, evtc.StateBuffRemoveSingle},
		{"removal derived by arcdps", evtc.Event{Buff: 1, IsBuffRemove: evtc.BuffRemoveManual}, evtc.StateBuffRemoveSingle},
		{"application", evtc.Event{Buff: 1, Value: 5000, IsShields: 1}, evtc.StateBuffApply},
		{"duration change", evtc.Event{Buff: 1, Value: -200, OverstackValue: 4800, IsOffcycle: 1}, evtc.StateBuffChange},
		{"state change", evtc.Event{IsStateChange: evtc.StateChangeDown, Value: 5000, Buff: 1}, evtc.StateChangeDown},
		{"initial buff", evtc.Event{IsStateChange: evtc.StateBuffInitial, SkillID: 740, Value: 5000}, evtc.StateBuffInitial},
		{"initial buff naming no buff", evtc.Event{IsStateChange: evtc.StateBuffInitial, Value: 1, Buff: 18}, kindSkipped},
	} {
		if got := legacyKind(&tt.e); got != tt.want {
			t.Errorf("%s: kind %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestLegacyTick(t *testing.T) {
	for _, tt := range []struct {
		e      evtc.Event
		want   evtc.Result
		downed bool
	}{
		{evtc.Event{}, evtc.ResultBuffDamageCycle, false},
		{evtc.Event{IsOffcycle: 1}, evtc.ResultBuffDamageNotCycle, false},
		{evtc.Event{IsOffcycle: 2}, evtc.ResultBuffDamageNotCycle, false},
		{evtc.Event{IsOffcycle: 3, Pad61: 1}, evtc.ResultBuffDamageNotCycleDmgToTargetOnHit, true},
		{evtc.Event{IsOffcycle: 4}, evtc.ResultBuffDamageNotCycleDmgToSourceOnHit, false},
		{evtc.Event{IsOffcycle: 5}, evtc.ResultBuffDamageNotCycleDmgToTargetOnStackRemove, false},
		// Invulnerable through a buff, then through a skill of the target.
		{evtc.Event{Result: 1, IsOffcycle: 3}, evtc.ResultAbsorb, false},
		{evtc.Event{Result: 3, Pad61: 1}, evtc.ResultAbsorb, true},
		{evtc.Event{Result: evtc.ResultInvert}, evtc.ResultInvert, false},
	} {
		if got, downed := legacyTick(&tt.e); got != tt.want || downed != tt.downed {
			t.Errorf("%+v: %v downed %v, want %v downed %v", tt.e, got, downed, tt.want, tt.downed)
		}
	}
}

// TestLegacyLog builds a fight written both ways and checks the older
// encoding gives the same graph, from events that stay what the log holds.
func TestLegacyLog(t *testing.T) {
	b := fixture()
	b.castStart(1000, addrP1, addrBoss, skillSlam, 750, 600)
	b.hit(1500, addrP1, addrBoss, skillSlam, 900, evtc.ResultStrikeDamageCrit)
	b.castStop(1750, addrP1, skillSlam, 750, evtc.ActivationReset)
	b.castStart(2000, addrBoss, 0, skillHeat, 1000, 0)
	b.castStop(2400, addrBoss, skillHeat, 400, evtc.ActivationCancel)
	b.buffInitial(0, addrP2, addrP1, BuffMight, 9000, 12000, 1)
	b.buffApply(1000, addrP2, addrP1, BuffMight, 5000, 2)
	b.buffApply(1100, addrP2, addrP1, BuffMight, 5000, 3)
	b.buffChange(2000, addrP1, BuffMight, -500, 2)
	b.buffRemoveSingle(3000, addrP1, addrBoss, BuffMight, 3000, 2, evtc.BuffRemoveSingle)
	b.buffRemoveSingle(4000, addrP1, 0, BuffMight, 2100, 3, evtc.BuffRemoveManual)
	b.buffRemoveAll(4000, addrP1, 0, BuffMight)
	b.buffTick(2500, addrP1, addrBoss, skillHeat, 120)
	b.add(evtc.Event{Time: b.at(2600), SrcAgent: addrP1, DstAgent: addrBoss, SkillID: skillHeat, Buff: 1, Result: evtc.ResultAbsorb, IsOffcycle: 1})
	typed, legacy := buildBoth(t, b.build(10000))
	sameTimeline(t, typed, legacy)

	p1 := legacy.characters[0]
	slam := p1.Casts().First()
	if slam == nil || slam.Target != nil || slam.Start.IsStateChange != evtc.StateCombat || slam.Start.IsActivation != evtc.ActivationStartDefunc || slam.Hits().Count() != 1 {
		t.Errorf("cast = %+v", slam)
	}
	if typed.characters[0].Casts().First().Target != nil {
		t.Error("the typed log kept its cast target")
	}
	stacks := p1.Stacks().OfBuff(BuffMight).All()
	if len(stacks) != 3 || !stacks[0].Initial || stacks[0].Removal != evtc.BuffRemoveAll || stacks[0].Apply.IsStateChange != evtc.StateBuffInitial {
		t.Fatalf("stacks = %v", stacks)
	}
	if s := stacks[1]; s.Extended != -500*msec || s.Removal != evtc.BuffRemoveSingle || s.RemovedBy != legacy.targets[0].Agent || s.Remaining != 3*time.Second || s.Apply.IsStateChange != evtc.StateCombat {
		t.Errorf("stack removed by the boss = %+v", s)
	}
	if s := stacks[2]; s.Removal != evtc.BuffRemoveManual || s.RemovedBy != nil || s.Remaining != 2100*msec {
		t.Errorf("stack removed by arcdps = %+v", s)
	}
	ticks := legacy.Hits().Where(func(h *Hit) bool { return h.IsBuff }).All()
	if len(ticks) != 2 || ticks[0].Result != evtc.ResultBuffDamageCycle || !ticks[0].Landed() || ticks[1].Result != evtc.ResultAbsorb || ticks[1].Landed() || !ticks[1].TargetDowned {
		t.Errorf("ticks = %+v", ticks)
	}
	if legacy.Events().Of(evtc.StateAnimationStart).Count() != 0 {
		t.Error("the legacy log holds typed events")
	}
}

// TestLegacySynthetic checks generated fights build the same graph from
// both encodings.
func TestLegacySynthetic(t *testing.T) {
	for _, o := range []genOptions{
		{players: 1, adds: 0, duration: 5 * time.Second, seed: 1},
		{players: 5, adds: 3, duration: 60 * time.Second, seed: 2},
		{players: 10, adds: 20, duration: 240 * time.Second, seed: 3},
	} {
		typed, legacy := buildBoth(t, genLog(o))
		sameTimeline(t, typed, legacy)
		if legacy.Casts().Count() == 0 || legacy.Stacks().Count() == 0 {
			t.Errorf("seed %d: %d casts and %d stacks from the legacy encoding", o.seed, legacy.Casts().Count(), legacy.Stacks().Count())
		}
	}
}

// TestLegacySample checks the real log kept in tests_fixtures/ builds the
// same graph once written the older way.
func TestLegacySample(t *testing.T) {
	loadSample(t)
	l, err := evtc.ParseFile(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	typed, legacy := buildBoth(t, l)
	sameTimeline(t, typed, legacy)
}

// legacyEffect writes a StateEffect2Defunc event: played at an agent when
// at is set, on the ground at origin otherwise.
func (b *logBuilder) legacyEffect(t, owner, at uint64, effectID, trackable, durMS uint32, origin Vec3, orient [3]int16, moving bool) {
	e := evtc.Event{Time: b.at(t), SrcAgent: owner, DstAgent: at, SkillID: effectID, IsStateChange: evtc.StateEffect2Defunc}
	if moving {
		e.IsFlanking = 1
	}
	b.raw(e, func(buf *[64]byte) {
		putF32(buf, offValue, origin.X)
		putF32(buf, offValue+4, origin.Y)
		putF32(buf, offValue+8, origin.Z)
		putU32(buf, offIFF, durMS)
		putU32(buf, offBuffRemove, trackable)
		for i, v := range orient {
			putI16(buf, offShields+2*i, v)
		}
	})
}

func TestLegacyEffects(t *testing.T) {
	b := fixture()
	b.l.Header.Build = "20240709"
	b.idToGUID(ContentEffect, 900, GUID{1}, 2500)
	b.legacyEffect(1000, addrBoss, 0, 900, 0, 0, Vec3{100, 200, 300}, [3]int16{0, 0, 1571}, true)
	b.legacyEffect(2000, addrBoss, addrP1, 901, 0, 4000, Vec3{}, [3]int16{}, false)
	b.legacyEffect(3000, addrBoss, 0, 902, 7, 0, Vec3{1, 2, 3}, [3]int16{}, false)
	b.legacyEffect(5000, 0, 0, 0, 7, 0, Vec3{}, [3]int16{}, false)
	tl := mustBuild(t, b.build(10000))
	checkInvariants(t, tl)

	boss, p1 := tl.targets[0], tl.characters[0]
	ground := boss.Effects().All()
	if len(ground) != 2 || tl.Effects().Count() != 3 {
		t.Fatalf("%d effects of the boss, %d in all", len(ground), tl.Effects().Count())
	}
	f := ground[0]
	if !f.Ground || f.Origin != (Vec3{100, 200, 300}) || f.Orientation != (Vec3{0, 0, 1.571}) || !f.MovingPlatform || f.Scale != 1 || f.GUID != (GUID{1}) ||
		f.Duration != 2500*msec || f.Interval != NewInterval(time.Second, 3500*msec) || f.Removed() || f.Create.IsStateChange != evtc.StateEffect2Defunc {
		t.Errorf("ground effect = %+v", f)
	}
	if f := ground[1]; f.ID != 7 || !f.Removed() || f.Interval != NewInterval(3*time.Second, 5*time.Second) {
		t.Errorf("tracked effect = %+v", f)
	}
	at := p1.Effects().First()
	if at == nil || at.Ground || at.Agent != p1.Agent || at.EffectID != 901 || at.Interval != NewInterval(2*time.Second, 6*time.Second) {
		t.Errorf("effect at an agent = %+v", at)
	}
}

// TestLegacyWeaponSets checks a log older than arcdps 20240627, whose
// swaps do not name the set left, claims no set before the first swap.
func TestLegacyWeaponSets(t *testing.T) {
	b := fixture()
	b.l.Header.Build = "20240613"
	b.weaponSwap(2000, addrP1, 0, 5)
	b.weaponSwap(4000, addrP1, 0, 4)
	tl := mustBuild(t, b.build(10000))
	checkInvariants(t, tl)
	p1 := tl.characters[0]
	if p1.WeaponSet.Len() != 2 || p1.WeaponSetAt(time.Second) != 0 || p1.WeaponSetAt(3*time.Second) != 5 {
		t.Errorf("weapon sets = %v", p1.WeaponSet.All())
	}
	if first, _ := p1.WeaponSet.First(); first.Event == nil {
		t.Error("a set was claimed before the first swap")
	}
}

// TestLegacyEmptyInitialBuffs checks the initial buffs naming no buff,
// which arcdps 20250708 to 20250913 writes by the thousand, give no stack.
func TestLegacyEmptyInitialBuffs(t *testing.T) {
	b := fixture()
	b.l.Header.Build = "20250809"
	b.add(evtc.Event{Time: b.at(0), DstAgent: addrP1, Value: 1, Buff: 18, IsStateChange: evtc.StateBuffInitial})
	b.buffInitial(0, addrP2, addrP1, BuffMight, 9000, 12000, 1)
	tl := mustBuild(t, b.build(10000))
	checkInvariants(t, tl)
	if n := tl.Stacks().Count(); n != 1 {
		t.Errorf("%d stacks, want the one naming a buff", n)
	}
}
