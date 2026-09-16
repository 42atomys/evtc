package timeline_test

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// The examples of this file are the cookbook of README.md, one function
// per snippet.

// Agents, skills and instance ids of the cookbook log.
const (
	alpha    = 0x1001
	bravo    = 0x1002
	sabetha  = 0x2001
	instBoss = 21
	slam     = 100
	flak     = 200
	stun     = 1000
	epoch    = 1000
)

// ev builds one event t milliseconds after the squad combat start.
func ev(t uint64, src, dst uint64, e evtc.Event) evtc.Event {
	e.Time, e.SrcAgent, e.DstAgent = epoch+t, src, dst
	inst := map[uint64]uint16{alpha: 11, bravo: 12, sabetha: instBoss}
	e.SrcInstanceID, e.DstInstanceID = inst[src], inst[dst]
	return e
}

// tracked builds a tracking event whose dst_agent field carries a payload
// rather than an agent.
func tracked(t uint64, agent uint64, k evtc.StateChange, payload uint64) evtc.Event {
	e := ev(t, agent, 0, evtc.Event{IsStateChange: k})
	e.DstAgent = payload
	return e
}

func position(t uint64, agent uint64, x, y float32) evtc.Event {
	return tracked(t, agent, evtc.StatePosition, uint64(math.Float32bits(x))|uint64(math.Float32bits(y))<<32)
}

func health(t uint64, agent uint64, pct uint64) evtc.Event {
	return tracked(t, agent, evtc.StateHealthPctUpdate, pct*100)
}

// packI16 packs int16 values little-endian, as arcdps stores coordinates
// divided by ten.
func packI16(v ...int16) uint64 {
	var out uint64
	for i, x := range v {
		out |= uint64(uint16(x)) << (16 * i)
	}
	return out
}

// withDuration spreads the duration of an effect over the iff, buff,
// result and activation bytes.
func withDuration(e evtc.Event, ms uint32) evtc.Event {
	e.IFF, e.Buff, e.Result, e.IsActivation = evtc.IFF(ms), uint8(ms>>8), evtc.Result(ms>>16), evtc.Activation(ms>>24)
	return e
}

// withTrackable writes a trackable id in the pad bytes.
func withTrackable(e evtc.Event, id uint32) evtc.Event {
	e.Pad61, e.Pad62, e.Pad63, e.Pad64 = uint8(id), uint8(id>>8), uint8(id>>16), uint8(id>>24)
	return e
}

// text8 packs at most eight bytes of text into a uint64 field, as an
// extension writes its version.
func text8(s string) uint64 {
	var out uint64
	for i := 0; i < len(s) && i < 8; i++ {
		out |= uint64(s[i]) << (8 * i)
	}
	return out
}

// guidEvent associates a content id with its GUID, whose sixteen bytes
// span the src_agent and dst_agent fields.
func guidEvent(kind timeline.ContentKind, id uint32, g timeline.GUID) evtc.Event {
	return evtc.Event{SrcAgent: binary.LittleEndian.Uint64(g[:8]), DstAgent: binary.LittleEndian.Uint64(g[8:]), SkillID: id, OverstackValue: uint32(kind), IsStateChange: evtc.StateIDToGUID}
}

// groundMarker places the squad marker at index i on the ground at (x, y),
// or removes it when both are zero.
func groundMarker(t uint64, index uint32, x, y float32) evtc.Event {
	return evtc.Event{Time: epoch + t, SrcAgent: uint64(math.Float32bits(x)) | uint64(math.Float32bits(y))<<32, SkillID: index, IsStateChange: evtc.StateSquadMarkerGround}
}

func strike(t uint64, src, dst uint64, skill uint32, dmg int32, r evtc.Result) evtc.Event {
	return ev(t, src, dst, evtc.Event{SkillID: skill, Value: dmg, Result: r, IFF: evtc.IFFFoe})
}

func state(t uint64, agent uint64, k evtc.StateChange, value int32) evtc.Event {
	return ev(t, agent, 0, evtc.Event{Value: value, IsStateChange: k})
}

// cookbookLog is the five second fight of the cookbook: two players
// against Sabetha. A real log comes from evtc.ParseFile instead.
func cookbookLog() *evtc.Log {
	l := &evtc.Log{
		Header: evtc.Header{Build: "20260816", Revision: 1, TargetSpeciesID: 15375},
		Agents: []evtc.Agent{
			{Addr: alpha, Profession: 1, IsElite: 62, Name: "Alpha", Account: ":Alpha.1234", Subgroup: "1", Toughness: 10},
			{Addr: bravo, Profession: 2, IsElite: 0, Name: "Bravo", Account: ":Bravo.5678", Subgroup: "2", Toughness: 1500},
			{Addr: sabetha, Profession: 15375, IsElite: 0xffffffff, Name: "Sabetha"},
		},
		Skills: []evtc.Skill{{ID: slam, Name: "Slam"}, {ID: flak, Name: "Flak Shot"}, {ID: int32(timeline.BuffMight), Name: "Might"}, {ID: stun, Name: "Stun"}},
	}
	fire := withTrackable(withDuration(ev(1900, sabetha, 0, evtc.Event{SkillID: 7000, IsStateChange: evtc.StateEffectGroundCreate}), 2000), 500)
	fire.DstAgent = packI16(30, 40, 0, 0) // origin 300, 400, 0
	mightApply := evtc.Event{SkillID: timeline.BuffMight, Value: 5000, IsStateChange: evtc.StateBuffApply, Pad61: 1}
	mightRemove := evtc.Event{SkillID: timeline.BuffMight, IsBuffRemove: evtc.BuffRemoveSingle, IsStateChange: evtc.StateBuffRemoveSingle, Pad61: 1}
	l.Events = []evtc.Event{
		{Time: epoch, Value: 1787685623, IsStateChange: evtc.StateSquadCombatStart},
		{Time: epoch, SrcAgent: 15375, DstAgent: sabetha, IsStateChange: evtc.StateLogNPCUpdate},
		position(500, alpha, 0, 0), position(1500, alpha, 300, 0),
		position(500, sabetha, 300, 400), position(1500, sabetha, 300, 400),
		health(500, sabetha, 100),
		ev(1000, alpha, sabetha, evtc.Event{SkillID: slam, Value: 500, IsStateChange: evtc.StateAnimationStart}),
		ev(1000, bravo, alpha, mightApply),
		strike(1200, alpha, sabetha, slam, 700, evtc.ResultStrikeDamageCrit),
		strike(1400, alpha, sabetha, slam, 300, evtc.ResultStrikeDamageNormal),
		ev(1500, alpha, 0, evtc.Event{SkillID: slam, Value: 500, IsActivation: evtc.ActivationReset, IsStateChange: evtc.StateAnimationStop}),
		strike(1600, bravo, sabetha, slam, 500, evtc.ResultStrikeDamageNormal),
		health(1800, sabetha, 60),
		strike(2000, sabetha, alpha, flak, 0, evtc.ResultBlock),
		strike(2200, sabetha, bravo, flak, 0, evtc.ResultDowned),
		state(2200, bravo, evtc.StateChangeDown, 0),
		ev(2500, alpha, 0, mightRemove),
		state(3000, bravo, evtc.StateChangeUp, 0),
		state(3000, sabetha, evtc.StateDefianceBarState, int32(timeline.DefianceActive)),
		state(3000, sabetha, evtc.StateDefianceBarPercent, int32(math.Float32bits(1))),
		strike(3200, alpha, sabetha, stun, 200, evtc.ResultDefianceDamageNormal),
		strike(3400, bravo, sabetha, stun, 300, evtc.ResultDefianceDamageNormal),
		state(3500, sabetha, evtc.StateDefianceBarPercent, 0),
		state(3500, sabetha, evtc.StateDefianceBarState, int32(timeline.DefianceRecover)),
		health(4000, sabetha, 30),
		strike(4500, sabetha, alpha, flak, 100, evtc.ResultStrikeDamageNormal),
		ev(0, alpha, 0, evtc.Event{Value: 1, Buff: 1, IsStateChange: evtc.StateMarker}),
		guidEvent(timeline.ContentMarker, 1, timeline.CommanderTagRed),
		guidEvent(timeline.ContentMarker, 2, timeline.MarkerHeart),
		ev(1000, bravo, 0, evtc.Event{Value: 2, IsStateChange: evtc.StateMarker}),
		ev(3000, bravo, 0, evtc.Event{IsStateChange: evtc.StateMarker}),
		groundMarker(500, 2, 300, 400), groundMarker(2000, 2, 100, 100), groundMarker(4000, 2, 0, 0),
		{Time: epoch, SrcAgent: 2, IsStateChange: evtc.StateLanguage},
		{Time: epoch, SrcAgent: 205780, IsStateChange: evtc.StateGWBuild},
		tracked(2500, alpha, evtc.StateWeaponSwap, 1),
		fire,
		withTrackable(evtc.Event{Time: epoch + 2400, IsStateChange: evtc.StateEffectGroundRemove}, 500),
		withTrackable(ev(1100, alpha, 0, evtc.Event{SkillID: slam, Value: int32(packI16(18, 0)), IsStateChange: evtc.StateMissileCreate}), 600),
		withTrackable(ev(1150, alpha, sabetha, evtc.Event{Value: int32(packI16(30, 40)), BuffDamage: int32(packI16(0, 18)), IsFlanking: 1, IsStateChange: evtc.StateMissileLaunch}), 600),
		withTrackable(ev(1200, alpha, 0, evtc.Event{SkillID: slam, BuffDamage: int32(packI16(30, 40)), IsFlanking: 1, IsStateChange: evtc.StateMissileRemove}), 600),
		tracked(500, sabetha, evtc.StateGadgetName, 1),
		tracked(3000, sabetha, evtc.StateGadgetName, 0),
		tracked(1000, sabetha, evtc.StateGadgetAnimation, 272061484),
		tracked(1100, alpha, evtc.StateJump, 1),
		tracked(1400, alpha, evtc.StateJump, 0),
		{Time: epoch + 100, SrcAgent: 0x9c9b3c99, DstAgent: text8("2.18rc1"), IsStateChange: evtc.StateExtension},
		withTrackable(ev(1300, bravo, alpha, evtc.Event{SkillID: slam, BuffDamage: -262, Buff: 1, IsStateChange: evtc.StateExtensionCombat}), 0x9c9b3c99),
		{Time: epoch + 4800, DstAgent: 55, Value: 1, IsStateChange: evtc.StateReward},
		{Time: epoch + 5000, IsStateChange: evtc.StateSquadCombatEnd},
	}
	return l
}

func mustBuild() *timeline.Timeline {
	tl, err := timeline.Build(cookbookLog())
	if err != nil {
		panic(err)
	}
	return tl
}

func ExampleBuild() {
	log := cookbookLog() // evtc.ParseFile("fight.zevtc") on a real file
	tl, err := timeline.Build(log)
	if err != nil {
		panic(err) // timeline.ErrLegacyLog for logs older than arcdps 20260501
	}
	fmt.Println(tl.Boss().Name, "fought by", tl.Players().Count(), "players for", tl.Duration)
	fmt.Println("arcdps build", tl.Build, "events", tl.Events().Count(), "hits", tl.Hits().Count())
	// Output:
	// Sabetha fought by 2 players for 5s
	// arcdps build 20260816 events 50 hits 8
}

func ExampleTimeline_Players() {
	tl := mustBuild()
	for p := range tl.Players().Seq() {
		fmt.Println(p.Name, p.Account, "group", p.Subgroup, p.Spec())
	}
	fmt.Println(tl.PlayerByAccount("Bravo.5678").Toughness, tl.PlayerByName("Nobody") == nil)
	fmt.Println(tl.Players().InSubgroup(2).First().Name, tl.Players().OfProfession(timeline.ProfessionGuardian).Count())
	// Output:
	// Alpha Alpha.1234 group 1 Firebrand
	// Bravo Bravo.5678 group 2 Warrior
	// 1500 true
	// Bravo 1
}

func ExampleHits_PerAgent_damage() {
	tl := mustBuild()
	landed := tl.Boss().HitsTaken().Landed()
	fmt.Println("total", landed.Damage(), "damage,", landed.Count(), "landed hits")
	for _, c := range landed.PerAgent() {
		fmt.Printf("%s %d (%.0f%%)\n", c.Agent.Name, c.Hits.Damage(), 100*float64(c.Hits.Damage())/float64(landed.Damage()))
	}
	// Output:
	// total 1500 damage, 3 landed hits
	// Alpha 1000 (67%)
	// Bravo 500 (33%)
}

func ExampleAgent_PhasesByHealth_dps() {
	tl := mustBuild()
	boss := tl.Boss()
	for i, phase := range boss.PhasesByHealth(66.6, 33.3) {
		fmt.Printf("phase %d %v: %.0f dps\n", i+1, phase, boss.HitsTaken().Landed().DPS(phase))
	}
	at, _ := boss.HealthBelow(33.3)
	fmt.Println("below 33.3% at", at, "health at 2s:", boss.HealthAt(2*time.Second))
	// Output:
	// phase 1 [0s, 1.8s]: 833 dps
	// phase 2 [1.8s, 4s]: 0 dps
	// phase 3 [4s, 4.5s]: 0 dps
	// below 33.3% at 4s health at 2s: 60
}

func ExampleCasts_Hits() {
	tl := mustBuild()
	alpha, boss := tl.Players().First(), tl.Boss()
	cast := alpha.Casts().OfSkill(slam).First()
	fmt.Println(cast.Skill, cast.Interval, "completed:", cast.Completed())
	fmt.Println("hits:", cast.Hits().Count(), "crit:", cast.Hits().Crits().Count(), "damage:", cast.Hits().Damage())
	fmt.Println("blocked by Alpha:", boss.Hits().On(alpha).Blocked().Any())
	// Output:
	// Slam (100) [1s, 1.5s] completed: true
	// hits: 2 crit: 1 damage: 1000
	// blocked by Alpha: true
}

func ExampleAgent_PositionAt() {
	tl := mustBuild()
	alpha, boss := tl.Players().First(), tl.Boss()
	at := time.Second
	fmt.Println(alpha.PositionAt(at), boss.PositionAt(at))
	fmt.Printf("%.0f units apart\n", alpha.DistanceTo(boss, at))
	fmt.Println("unknown:", math.IsNaN(alpha.DistanceTo(boss, 10*time.Second)))
	// Output:
	// {150 0 0} {300 400 0}
	// 427 units apart
	// unknown: true
}

func ExampleStacks_Uptime() {
	tl := mustBuild()
	alpha := tl.Players().First()
	stacks := alpha.Stacks().OfBuff(timeline.BuffMight)
	fmt.Println("might stacks:", stacks.Count(), "at 2s:", stacks.CountAt(2*time.Second), "at 3s:", stacks.CountAt(3*time.Second))
	fmt.Println("uptime:", stacks.Uptime(tl.Interval()), "average:", stacks.Average(tl.Interval()), "applied by", stacks.First().Applier.Name)
	// Output:
	// might stacks: 1 at 2s: 1 at 3s: 0
	// uptime: 1.5s average: 0.3 applied by Bravo
}

func ExampleAgent_DownsOf() {
	tl := mustBuild()
	bravo := tl.Players().Skip(1).First()
	down := bravo.Downs[0]
	fmt.Println(bravo.Name, "down", down.Interval, "by", down.Cause.Skill.Name, "from", down.Cause.Src.Name, "recovered:", down.Recovered)
	fmt.Println(bravo.IsDownAt(2500*time.Millisecond), bravo.DownedBetween(tl.Since(4*time.Second)), len(bravo.DownsOf(tl.Skill(flak))), len(bravo.DownsBy(tl.Boss())))
	fmt.Println("life at 2.5s:", bravo.LifeStateAt(2500*time.Millisecond), "died:", bravo.DiedBefore(tl.Duration))
	// Output:
	// Bravo down [2.2s, 3s] by Flak Shot from Sabetha recovered: true
	// true false 1 1
	// life at 2.5s: Down died: false
}

func ExampleBreakbar() {
	tl := mustBuild()
	bb := tl.Boss().Breakbars[0]
	fmt.Println(bb.Interval, "broken:", bb.Broken(), "cc:", bb.TotalCC())
	for _, c := range bb.Hits().PerAgent() {
		fmt.Println(c.Agent.Name, c.Hits.Damage(), "cc at", c.Hits.First().Time)
	}
	// Output:
	// [3s, 3.5s] broken: true cc: 500
	// Bravo 300 cc at 3.4s
	// Alpha 200 cc at 3.2s
}

func ExampleHits_Limit() {
	tl := mustBuild()
	for _, h := range tl.Hits().Landed().Reverse().Limit(2).All() {
		fmt.Println(h.Time, h.Src.Name, "->", h.Dst.Name, h.Damage)
	}
	fmt.Println("second landed hit:", tl.Hits().Landed().Skip(1).First().Damage)
	// Output:
	// 4.5s Sabetha -> Alpha 100
	// 1.6s Bravo -> Sabetha 500
	// second landed hit: 300
}

func ExampleEvents_Involving() {
	tl := mustBuild()
	bravo := tl.Players().Skip(1).First()
	e := tl.Events().Involving(bravo).Of(evtc.StateChangeDown).First()
	fmt.Println(tl.TimeOf(e), e.IsStateChange, "src", e.SrcAgent == bravo.Addr)
	fmt.Println(bravo.Events().Count(), "events involve", bravo.Name)
	// Output:
	// 2.2s ChangeDown src true
	// 9 events involve Bravo
}

func ExampleTimeline_AgentAt() {
	tl := mustBuild()
	fmt.Println(tl.AgentAt(instBoss, time.Second), tl.TargetBySpeciesIDAt(15375, time.Second).Boss)
	fmt.Println(tl.Agent(alpha).Player.Spec(), tl.Agent(0xdead) == nil)
	// Output:
	// Sabetha(NPC#15375) true
	// Firebrand true
}

func ExampleHits_Map() {
	tl := mustBuild()
	type row struct {
		At     time.Duration
		Skill  string
		Damage int32
	}
	rows := tl.Boss().HitsTaken().Strikes().Map(func(h *timeline.Hit) row {
		return row{h.Time, h.Skill.Name, h.Damage}
	})
	fmt.Printf("%+v\n", rows)
	// Output:
	// [{At:1.2s Skill:Slam Damage:700} {At:1.4s Skill:Slam Damage:300} {At:1.6s Skill:Slam Damage:500}]
}

func ExampleHits_PerSkill() {
	tl := mustBuild()
	for _, s := range tl.Boss().HitsTaken().Landed().PerSkill() {
		fmt.Println(s.Skill.Name, s.Hits.Count(), "hits,", s.Hits.Damage(), "damage")
	}
	for _, c := range tl.Hits().Landed().PerTarget() {
		fmt.Println(c.Agent.Name, "took", c.Hits.Damage())
	}
	for _, c := range tl.Casts().PerSkill() {
		fmt.Println(c.Skill.Name, "cast", c.Casts.Count(), "time by", c.Casts.First().Caster.Name)
	}
	// Output:
	// Slam 3 hits, 1500 damage
	// Sabetha took 1500
	// Alpha took 100
	// Slam cast 1 time by Alpha
}

func ExampleAgent_AliveTime() {
	tl := mustBuild()
	bravo := tl.Players().Skip(1).First()
	died, _ := bravo.DiedAt()
	fmt.Println("alive", bravo.AliveTime(tl.Interval()), "down", bravo.DownTime(tl.Interval()), "died:", died)
	fmt.Println("in combat", tl.Players().First().CombatTime(tl.Interval()))
	// Output:
	// alive 3.2s down 800ms died: 0s
	// in combat 0s
}

func ExampleNumbers_TimeBelow() {
	tl := mustBuild()
	boss := tl.Boss()
	lowest, _ := boss.Health.MinBetween(tl.Until(2 * time.Second))
	fmt.Println("lowest health in the first two seconds:", lowest)
	fmt.Println("time under 66.6%:", boss.Health.TimeBelow(66.6, tl.Interval()), "of", boss.Lifetime.Duration())
	// Output:
	// lowest health in the first two seconds: 60
	// time under 66.6%: 2.7s of 4.5s
}

func ExampleTimeline_Commander() {
	tl := mustBuild()
	fmt.Println("commander:", tl.Commander().Name, "| language:", tl.Language, "| game build:", tl.GameBuild)
	alpha := tl.Players().First()
	fmt.Println("weapon swaps:", alpha.WeaponSet.Len()-1, "| set at 3s:", alpha.WeaponSetAt(3*time.Second))
	// Output:
	// commander: Alpha | language: French | game build: 205780
	// weapon swaps: 1 | set at 3s: 1
}

func ExampleAgent_SquadMarkerAt() {
	tl := mustBuild()
	bravo := tl.Players().Skip(1).First()
	m := bravo.Markers[0]
	fmt.Println(m.Squad, "on", bravo.Name, m.Interval, "removed:", m.Removed(), "| at 2s:", bravo.SquadMarkerAt(2*time.Second), "| at 4s:", bravo.SquadMarkerAt(4*time.Second))
	heart := tl.GroundMarkerAt(timeline.SquadHeart, 2500*time.Millisecond)
	fmt.Println("heart on the ground at", heart.Position, heart.Interval, "| placements:", len(tl.GroundMarkers))
	fmt.Println("commander at 1s:", tl.CommanderAt(time.Second).Name, "| tag:", tl.Commander().Markers[0].Tag)
	// Output:
	// Heart on Bravo [1s, 3s] removed: true | at 2s: Heart | at 4s: None
	// heart on the ground at {100 100 0} [2s, 4s] | placements: 2
	// commander at 1s: Alpha | tag: Red
}

func ExampleEffects() {
	tl := mustBuild()
	f := tl.Boss().Effects().Ground().First()
	fmt.Println(f.EffectID, "at", f.Origin, "for", f.Duration, f.Interval, "removed:", f.Removed())
	fmt.Println("effects present at 2s:", tl.Effects().At(2*time.Second).Count())
	// Output:
	// 7000 at {300 400 0} for 2s [1.9s, 2.4s] removed: true
	// effects present at 2s: 1
}

func ExampleMissiles() {
	tl := mustBuild()
	m := tl.Players().First().Missiles().First()
	fmt.Println(m.Skill.Name, "from", m.Origin, m.Interval, "aimed at", m.Target().Name, "| hit:", m.HitEnemy)
	fmt.Println(len(m.Launches), "launch at", m.Launches[0].Time, "towards", m.Launches[0].TargetPos)
	// Output:
	// Slam from {180 0 0} [1.1s, 1.2s] aimed at Sabetha | hit: true
	// 1 launch at 1.15s towards {300 400 0}
}

func ExampleAgent_IsNameVisibleAt() {
	tl := mustBuild()
	boss := tl.Boss()
	fmt.Println("animations:", len(boss.GadgetAnimations), "| name shown at 1s:", boss.IsNameVisibleAt(time.Second), "| at 4s:", boss.IsNameVisibleAt(4*time.Second))
	fmt.Println("alpha airborne at 1.2s:", tl.Players().First().IsAirborneAt(1200*time.Millisecond), "| rewards:", len(tl.Rewards))
	// Output:
	// animations: 1 | name shown at 1s: true | at 4s: false
	// alpha airborne at 1.2s: true | rewards: 1
}

func ExampleExtension() {
	tl := mustBuild()
	x := tl.Extension(timeline.ExtensionHealingStats)
	fmt.Printf("extension %#x version %s wrote %d of the %d extension events\n", x.Signature, x.Version, x.Events().Count(), tl.ExtensionEvents().Count())
	e := x.Events().On(tl.Players().First()).First()
	fmt.Println("first on Alpha:", tl.Agent(e.SrcAgent).Name, "buff_dmg", e.BuffDamage, "at", tl.TimeOf(e), "| written by", tl.ExtensionOf(e).Version)
	// Output:
	// extension 0x9c9b3c99 version 2.18rc1 wrote 1 of the 1 extension events
	// first on Alpha: Bravo buff_dmg -262 at 1.3s | written by 2.18rc1
}
