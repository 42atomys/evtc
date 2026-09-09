package healingstats_test

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/extensions/healingstats"
	"github.com/42atomys/evtc/timeline"
)

// The examples of this file are the cookbook of README.md, one function
// per snippet.

// Agents, instance ids and skills of the cookbook log.
const (
	alpha       = 0x1001 // the recording player
	bravo       = 0x1002 // a squad member sharing its stats
	charlie     = 0x1003 // a squad member without the addon
	sabetha     = 0x2001
	shelter     = 100
	regen       = 718
	sandCascade = 200
	epoch       = 1000
)

// ev builds one event t milliseconds after the squad combat start.
func ev(t uint64, src, dst uint64, e evtc.Event) evtc.Event {
	e.Time, e.SrcAgent, e.DstAgent = epoch+t, src, dst
	inst := map[uint64]uint16{alpha: 11, bravo: 12, charlie: 13, sabetha: 21}
	e.SrcInstanceID, e.DstInstanceID = inst[src], inst[dst]
	return e
}

// The recording flags the addon writes in is_offcycle: which client wrote
// the event.
const (
	fromSrc = 1 << 7
	fromDst = 1 << 6
)

// heal writes a heal of the addon: the amount negated in value, or in
// buff_dmg for the tick of a buff, the signature in the pad bytes.
func heal(t uint64, src, dst uint64, skill uint32, amount int32, flags uint8, buff bool) evtc.Event {
	e := ev(t, src, dst, evtc.Event{SkillID: skill, IsOffcycle: flags, IsStateChange: evtc.StateExtensionCombat})
	if buff {
		e.Buff, e.BuffDamage = 1, -amount
	} else {
		e.Value = -amount
	}
	sig := uint32(healingstats.Signature)
	e.Pad61, e.Pad62, e.Pad63, e.Pad64 = uint8(sig), uint8(sig>>8), uint8(sig>>16), uint8(sig>>24)
	return e
}

// barrier writes the barrier a skill gave: a heal with is_shields set.
func barrier(t uint64, src, dst uint64, skill uint32, amount int32, flags uint8) evtc.Event {
	e := heal(t, src, dst, skill, amount, flags, false)
	e.IsShields, e.OverstackValue = 1, uint32(amount)
	return e
}

// registration writes the registration event of the addon: signature,
// format revision and version length packed in src_agent, the version
// string over dst_agent.
func registration(version string, revision uint64) evtc.Event {
	var dst [8]byte
	copy(dst[:], version)
	return evtc.Event{Time: epoch, SrcAgent: uint64(healingstats.Signature) | revision<<32 | uint64(len(version))<<56, DstAgent: binary.LittleEndian.Uint64(dst[:]), IsStateChange: evtc.StateExtension}
}

func state(t uint64, agent uint64, k evtc.StateChange) evtc.Event {
	return ev(t, agent, 0, evtc.Event{IsStateChange: k})
}

// cookbookLog is a five second fight recorded by Alpha with the addon,
// Bravo sharing its stats with it: Alpha casts Shelter on the squad, Bravo
// keeps regeneration on Alpha, Charlie gives barrier and goes down, and is
// healed while down. A real log comes from evtc.ParseFile instead.
func cookbookLog() *evtc.Log {
	l := &evtc.Log{
		Header: evtc.Header{Build: "20260816", Revision: 1, TargetSpeciesID: 15375},
		Agents: []evtc.Agent{
			{Addr: alpha, Profession: 1, IsElite: 62, Name: "Alpha", Account: ":Alpha.1234", Subgroup: "1"},
			{Addr: bravo, Profession: 2, IsElite: 0, Name: "Bravo", Account: ":Bravo.5678", Subgroup: "1"},
			{Addr: charlie, Profession: 8, IsElite: 34, Name: "Charlie", Account: ":Charlie.9012", Subgroup: "2"},
			{Addr: sabetha, Profession: 15375, IsElite: 0xffffffff, Name: "Sabetha"},
		},
		Skills: []evtc.Skill{{ID: shelter, Name: "Shelter"}, {ID: regen, Name: "Regeneration"}, {ID: sandCascade, Name: "Sand Cascade"}},
	}
	l.Events = []evtc.Event{
		{Time: epoch, Value: 1787685623, IsStateChange: evtc.StateSquadCombatStart},
		{Time: epoch, SrcAgent: alpha, IsStateChange: evtc.StatePointOfView},
		{Time: epoch, SrcAgent: 15375, DstAgent: sabetha, IsStateChange: evtc.StateLogNPCUpdate},
		registration("2.19rc2", 2),
		state(0, alpha, evtc.StateEnterCombat), state(0, bravo, evtc.StateEnterCombat), state(0, charlie, evtc.StateEnterCombat),
		ev(1000, alpha, 0, evtc.Event{SkillID: shelter, Value: 500, IsStateChange: evtc.StateAnimationStart}),
		// Shelter heals the squad: the heal on Bravo is written by both
		// clients, Bravo's copy a little later.
		heal(1200, alpha, bravo, shelter, 900, fromSrc, false),
		heal(1260, alpha, bravo, shelter, 900, fromDst, false),
		heal(1200, alpha, charlie, shelter, 900, fromSrc, false),
		heal(1200, alpha, alpha, shelter, 900, fromSrc|fromDst, false),
		ev(1500, alpha, 0, evtc.Event{SkillID: shelter, Value: 500, IsActivation: evtc.ActivationReset, IsStateChange: evtc.StateAnimationStop}),
		// Regeneration from Bravo ticks on Alpha, written by both clients.
		heal(2000, bravo, alpha, regen, 130, fromDst, true), heal(2040, bravo, alpha, regen, 130, fromSrc, true),
		heal(3000, bravo, alpha, regen, 130, fromDst, true), heal(3050, bravo, alpha, regen, 130, fromSrc, true),
		barrier(2500, charlie, alpha, sandCascade, 1200, fromDst),
		state(3500, charlie, evtc.StateChangeDown),
		heal(4000, alpha, charlie, shelter, 500, fromSrc|1, false), // the arcdps flag: the target was downed
		state(4500, charlie, evtc.StateChangeUp),
		state(4999, alpha, evtc.StateExitCombat), state(4999, bravo, evtc.StateExitCombat), state(4999, charlie, evtc.StateExitCombat),
		{Time: epoch + 5000, IsStateChange: evtc.StateSquadCombatEnd},
	}
	return l
}

func mustBuild() (*timeline.Timeline, *healingstats.Stats) {
	tl, err := timeline.Build(cookbookLog())
	if err != nil {
		panic(err)
	}
	return tl, healingstats.Of(tl)
}

func ExampleOf() {
	tl, err := timeline.Build(cookbookLog()) // timeline.ParseFile("fight.zevtc") on a real file
	if err != nil {
		panic(err)
	}
	h := healingstats.Of(tl) // nil when the log has no healing stats
	fmt.Println("addon", h.Version, "format revision", h.Revision, "|", h.Heals().Count(), "heals,", h.Merged, "written by both clients")
	for _, p := range h.Recorded {
		fmt.Println("recorded by", p.Name, "with", p.Heals().Count(), "heals dealt and", p.HealsTaken().Count(), "received")
	}
	// Output:
	// addon 2.19rc2 format revision 2 | 7 heals, 3 written by both clients
	// recorded by Alpha with 4 heals dealt and 4 received
	// recorded by Bravo with 2 heals dealt and 1 received
}

func ExampleHeals_PerAgent() {
	tl, h := mustBuild()
	for _, c := range h.Heals().Healing().PerAgent() {
		fmt.Printf("%s: %d heals, %d healed, %.0f hps\n", c.Agent.Name, c.Heals.Count(), c.Heals.Healed(), c.Heals.HPS(tl.Interval()))
	}
	fmt.Println("barrier:", h.Heals().Barrier().PerAgent()[0].Agent.Name, h.Heals().BarrierGiven())
	// Output:
	// Alpha: 4 heals, 3200 healed, 640 hps
	// Bravo: 2 heals, 260 healed, 52 hps
	// barrier: Charlie 1200
}

func ExampleAgent_HealsTaken() {
	tl, h := mustBuild()
	a := h.Agent(tl.POV) // every timeline entity has a node
	taken := a.HealsTaken()
	fmt.Println(a.Name, "received", taken.Healed(), "healing and", taken.BarrierGiven(), "barrier over", taken.Count(), "heals")
	fmt.Println("from others:", taken.Others().Count(), "| self:", taken.Self().Amount(), "| regeneration ticks:", taken.Ticks().OfSkill(regen).Count())
	fmt.Println("hits dealt by the same node:", tl.Hits().By(a).Count())
	// Output:
	// Alpha received 1160 healing and 1200 barrier over 4 heals
	// from others: 3 | self: 900 | regeneration ticks: 2
	// hits dealt by the same node: 0
}

func ExampleHeal_PeerEvent() {
	tl, h := mustBuild()
	heal := h.Heals().On(tl.PlayerByName("Bravo")).First()
	fmt.Println(heal.Src.Name, "->", heal.Dst.Name, heal.Skill.Name, heal.Amount, "at", heal.Time)
	fmt.Println("written by the client of the source:", heal.SrcRecorded, "| of the destination:", heal.DstRecorded, "| second record at", tl.TimeOf(heal.PeerEvent))
	single := h.Heals().On(tl.PlayerByName("Charlie")).First()
	fmt.Println("Charlie runs no addon:", single.SrcRecorded, single.DstRecorded, single.PeerEvent == nil)
	// Output:
	// Alpha -> Bravo Shelter 900 at 1.2s
	// written by the client of the source: true | of the destination: true | second record at 1.26s
	// Charlie runs no addon: true false true
}

func ExampleHeals_OfCast() {
	tl, h := mustBuild()
	cast := h.Agent(tl.POV).Casts().OfSkill(shelter).First()
	heals := h.Heals().OfCast(cast)
	fmt.Println(cast.Skill.Name, cast.Interval, "healed", heals.Amount(), "over", heals.Count(), "heals; first on", heals.First().Dst.Name)
	// Output:
	// Shelter [1s, 1.5s] healed 3200 over 4 heals; first on Bravo
}

func ExampleHeals_Downed() {
	tl, h := mustBuild()
	for heal := range h.Heals().Downed().Seq() {
		fmt.Println(heal.Dst.Name, "was", heal.Dst.LifeStateAt(heal.Time), "when healed for", heal.Amount, "by", heal.Src.Name, "at", heal.Time)
	}
	fmt.Println("healing received while down:", h.Agent(tl.PlayerByName("Charlie")).HealsTaken().Downed().Healed())
	// Output:
	// Charlie was Down when healed for 500 by Alpha at 4s
	// healing received while down: 500
}

func ExampleHeals_Between() {
	tl, h := mustBuild()
	second := timeline.NewInterval(2*time.Second, 3*time.Second)
	fmt.Println("heals between 2s and 3s:", h.Heals().Between(second).Count(), "| healing:", h.Heals().Between(second).Healed(), "| barrier:", h.Heals().Between(second).BarrierGiven())
	fmt.Println("latest heal:", h.Heals().Reverse().First().Skill.Name, "| skills:", len(h.Heals().PerSkill()), "| log:", tl.Duration)
	// Output:
	// heals between 2s and 3s: 3 | healing: 260 | barrier: 1200
	// latest heal: Shelter | skills: 3 | log: 5s
}
