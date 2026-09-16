package timeline_test

import (
	"fmt"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// A tiny log: one player hits a boss twice during one cast, and the boss
// answers with a blocked attack.
func exampleLog() *evtc.Log {
	const (
		player = 0x1001
		boss   = 0x2001
		slam   = 100
	)
	const epoch = 1000
	return &evtc.Log{
		Header: evtc.Header{Build: "20260816", Revision: 1, TargetSpeciesID: 15375},
		Agents: []evtc.Agent{
			{Addr: player, Profession: 1, IsElite: 0, Name: "Alpha", Account: ":Alpha.1234", Subgroup: "1"},
			{Addr: boss, Profession: 15375, IsElite: 0xffffffff, Name: "Sabetha"},
		},
		Skills: []evtc.Skill{{ID: slam, Name: "Slam"}},
		Events: []evtc.Event{
			{Time: epoch, Value: 1787685623, IsStateChange: evtc.StateSquadCombatStart},
			{Time: epoch, SrcAgent: 15375, DstAgent: boss, IsStateChange: evtc.StateLogNPCUpdate},
			{Time: epoch + 1000, SrcAgent: boss, DstAgent: 10000, IsStateChange: evtc.StateHealthPctUpdate},
			{Time: epoch + 1500, SrcAgent: boss, DstAgent: 6000, IsStateChange: evtc.StateHealthPctUpdate},
			{Time: epoch + 1000, SrcAgent: player, DstAgent: boss, SkillID: slam, Value: 484, IsStateChange: evtc.StateAnimationStart},
			{Time: epoch + 1200, SrcAgent: player, DstAgent: boss, SkillID: slam, Value: 700, Result: evtc.ResultStrikeDamageCrit},
			{Time: epoch + 1400, SrcAgent: player, DstAgent: boss, SkillID: slam, Value: 300},
			{Time: epoch + 1500, SrcAgent: player, SkillID: slam, Value: 500, IsActivation: evtc.ActivationReset, IsStateChange: evtc.StateAnimationStop},
			{Time: epoch + 2000, SrcAgent: boss, DstAgent: player, SkillID: slam, Result: evtc.ResultBlock},
			{Time: epoch + 3000, IsStateChange: evtc.StateSquadCombatEnd},
		},
	}
}

func Example() {
	tl, err := timeline.Build(exampleLog())
	if err != nil {
		panic(err)
	}
	player, boss := tl.Players().First(), tl.Targets[0]

	cast := player.Casts().First()
	fmt.Println("cast:", cast.Skill.Name, cast.Interval, "completed:", cast.Completed())
	fmt.Println("damage on boss:", boss.HitsTaken().Landed().Damage())
	fmt.Println("hits of the cast:", cast.Hits().Count(), "first crit:", cast.Hits().First().Crit())
	fmt.Println("blocked by player:", boss.Hits().On(player).Blocked().Any())
	fmt.Println("boss alive at 2s:", boss.IsAliveAt(2*time.Second))
	fmt.Println("log duration:", tl.Duration)
	// Output:
	// cast: Slam [1s, 1.5s] completed: true
	// damage on boss: 1000
	// hits of the cast: 2 first crit: true
	// blocked by player: true
	// boss alive at 2s: true
	// log duration: 3s
}

func ExampleTimeline_lookups() {
	tl, err := timeline.Build(exampleLog())
	if err != nil {
		panic(err)
	}
	p := tl.PlayerByAccount(":Alpha.1234")
	fmt.Println(p.Name, "subgroup", p.Subgroup, "hits:", p.Hits().Count())
	fmt.Println(tl.Boss().Name, tl.TargetBySpeciesID(15375) == tl.Boss())
	fmt.Println(tl.PlayerByName("Nobody") == nil, tl.Players().InSubgroup(2).Count())
	// Output:
	// Alpha subgroup 1 hits: 2
	// Sabetha true
	// true 0
}

func ExampleAgent_HealthBelow() {
	tl, err := timeline.Build(exampleLog())
	if err != nil {
		panic(err)
	}
	boss := tl.Boss()
	at, ok := boss.HealthBelow(66.6)
	fmt.Println(at, ok)
	fmt.Println(boss.HealthAt(2 * time.Second))
	for _, c := range boss.HealthCrossings(90) {
		fmt.Println(c.Time, c.Direction, c.From.Value, "->", c.To.Value)
	}
	// Output:
	// 1.5s true
	// 60
	// 1.5s Falling 100 -> 60
}

func ExampleInterval_Split() {
	tl, err := timeline.Build(exampleLog())
	if err != nil {
		panic(err)
	}
	for _, phase := range tl.Interval().Split(1300*time.Millisecond, 1900*time.Millisecond) {
		fmt.Println(phase, tl.Hits().Between(phase).Count())
	}
	// Output:
	// [0s, 1.3s] 1
	// [1.3s, 1.9s] 1
	// [1.9s, 3s] 1
}

func ExampleHits_PerAgent() {
	tl, err := timeline.Build(exampleLog())
	if err != nil {
		panic(err)
	}
	for _, c := range tl.Boss().HitsTaken().PerAgent() {
		fmt.Println(c.Agent.Name, c.Hits.Count(), "hits,", c.Hits.Damage(), "damage")
	}
	// Output:
	// Alpha 2 hits, 1000 damage
}

func ExampleHits_Reverse() {
	tl, err := timeline.Build(exampleLog())
	if err != nil {
		panic(err)
	}
	hits := tl.Players().First().Hits()
	last := hits.Reverse().First()
	fmt.Println(last.Time, last.Damage)
	fmt.Println(hits.Reverse().Limit(1).Damage(), hits.Skip(1).Count())
	// Output:
	// 1.4s 300
	// 300 1
}

func ExampleAgent_PhasesByHealth() {
	tl, err := timeline.Build(exampleLog())
	if err != nil {
		panic(err)
	}
	boss := tl.Boss()
	for _, phase := range boss.PhasesByHealth(66.6) {
		fmt.Printf("%v %d damage, %.0f dps\n", phase, boss.HitsTaken().Between(phase).Damage(), boss.HitsTaken().DPS(phase))
	}
	// Output:
	// [0s, 1.5s] 1000 damage, 667 dps
	// [1.5s, 2s] 0 damage, 0 dps
}

func ExamplePlayer_Spec() {
	tl, err := timeline.Build(exampleLog())
	if err != nil {
		panic(err)
	}
	p := tl.Players().First()
	fmt.Println(p.Profession, p.EliteSpec, p.Spec())
	// Output:
	// Guardian None Guardian
}
