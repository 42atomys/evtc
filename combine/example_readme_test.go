package combine_test

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/combine"
	"github.com/42atomys/evtc/timeline"
)

// The examples of this file are the cookbook of README.md, one function
// per snippet.

// Instance ids and skills of the cookbook fight. The address of an agent
// is its instance id, in the high half for the client of Bravo.
const (
	alpha    = 11
	bravo    = 12
	charlie  = 13
	sabetha  = 21 // stands by Alpha
	knuckles = 22 // stands between Alpha and Bravo
	kernan   = 23 // stands by Bravo, 8,000 units from Alpha
	heat     = 100
	epoch    = 1000
)

// client writes the log of one player of the cookbook fight: under its
// own addresses and on its own clock.
type client struct {
	log         *evtc.Log
	base, clock uint64
}

// newClient starts the log of the player given, who lists the squad and
// the creatures within its view.
func newClient(player uint64, base, clock uint64, creatures ...uint64) *client {
	c := &client{log: &evtc.Log{Header: evtc.Header{Build: "20260920", Revision: 1, TargetSpeciesID: 15375}}, base: base, clock: clock}
	for i, name := range []string{"Alpha", "Bravo", "Charlie"} {
		id := uint64(alpha + i)
		c.log.Agents = append(c.log.Agents, evtc.Agent{Addr: base | id, Profession: uint32(i + 1), Name: name, Account: fmt.Sprintf(":%s.%d", name, 1234*(i+1)), Subgroup: "1"})
	}
	names := map[uint64]string{sabetha: "Sabetha", knuckles: "Knuckles", kernan: "Kernan"}
	species := map[uint64]uint32{sabetha: 15375, knuckles: 15404, kernan: 15372}
	for _, id := range creatures {
		c.log.Agents = append(c.log.Agents, evtc.Agent{Addr: base | id, Profession: species[id], IsElite: 0xffffffff, Name: names[id]})
	}
	c.log.Skills = []evtc.Skill{{ID: heat, Name: "Heat"}}
	c.add(0, 0, 0, evtc.Event{SrcAgent: 0x637261, Value: 1787685623, BuffDamage: 1787685624, IsStateChange: evtc.StateSquadCombatStart})
	c.add(0, player, 0, evtc.Event{IsStateChange: evtc.StatePointOfView})
	return c
}

// add writes an event t milliseconds after the squad combat start.
func (c *client) add(t, src, dst uint64, e evtc.Event) {
	e.Time = epoch + c.clock + t
	if src != 0 {
		e.SrcAgent, e.SrcInstanceID = c.base|src, uint16(src)
	}
	if dst != 0 {
		e.DstAgent, e.DstInstanceID = c.base|dst, uint16(dst)
	}
	c.log.Events = append(c.log.Events, e)
}

func (c *client) hit(t, src, dst uint64, damage int32) {
	c.add(t, src, dst, evtc.Event{SkillID: heat, Value: damage, IFF: evtc.IFFFoe})
}

func (c *client) position(t, agent uint64, x float32) {
	c.add(t, agent, 0, evtc.Event{DstAgent: uint64(math.Float32bits(x)), IsStateChange: evtc.StatePosition})
}

// cookbookLogs returns the logs Alpha and Bravo recorded of a ten second
// fight spread over 8,000 units. Alpha stands by Sabetha and Bravo by
// Kernan, each out of the view of the other; both see Charlie fight
// Knuckles between them.
func cookbookLogs() (byAlpha, byBravo *evtc.Log) {
	a := newClient(alpha, 0x1000_0000, 0, sabetha, knuckles)
	b := newClient(bravo, 0x2000_0000, 7_654_322, knuckles, kernan)
	for t := uint64(0); t < 10000; t += 500 {
		a.position(t+1, alpha, 0)
		b.position(t+1, bravo, 8000)
		for _, c := range []*client{a, b} {
			c.position(t+1, charlie, 4000)
		}
	}
	for i := range uint64(25) {
		a.hit(400*i, charlie, knuckles, int32(1000+i))
		b.hit(400*i, charlie, knuckles, int32(1000+i))
	}
	for i := range uint64(10) {
		a.hit(1000*i+100, alpha, sabetha, int32(2000+i))
		a.add(1000*i+101, sabetha, 0, evtc.Event{DstAgent: 10000 - 100*i, IsStateChange: evtc.StateHealthPctUpdate})
		b.hit(1000*i+200, bravo, kernan, int32(3000+i))
	}
	for _, c := range []*client{a, b} {
		c.add(10000, 0, 0, evtc.Event{SrcAgent: 0x637261, Value: 1787685633, IsStateChange: evtc.StateSquadCombatEnd})
	}
	return a.log, b.log
}

// mustBuild builds the timeline of a log.
func mustBuild(l *evtc.Log) *timeline.Timeline {
	tl, err := timeline.Build(l)
	if err != nil {
		panic(err)
	}
	return tl
}

func Example_combine() {
	byAlpha, byBravo := cookbookLogs() // evtc.ParseFile("alpha.zevtc") on real files
	log, err := combine.Logs(byAlpha, byBravo)
	if err != nil {
		panic(err)
	}
	for _, l := range []*evtc.Log{byAlpha, byBravo, log} {
		tl := mustBuild(l)
		fmt.Println(len(l.Events), "events |", tl.NPCs().Count(), "creatures |", tl.Hits().Count(), "hits for", tl.Hits().Damage(), "damage")
	}
	// Output:
	// 88 events | 2 creatures | 35 hits for 45345 damage
	// 78 events | 2 creatures | 35 hits for 55345 damage
	// 118 events | 3 creatures | 45 hits for 75390 damage
}

// cookbookFiles writes the logs of the cookbook in a temporary folder and
// returns the folder.
func cookbookFiles() string {
	dir, err := os.MkdirTemp("", "combine")
	if err != nil {
		panic(err)
	}
	byAlpha, byBravo := cookbookLogs()
	if err := errors.Join(byAlpha.WriteFile(filepath.Join(dir, "alpha.zevtc")), byBravo.WriteFile(filepath.Join(dir, "bravo.zevtc"))); err != nil {
		panic(err)
	}
	return dir
}

func Example_files() {
	dir := cookbookFiles() // the folder of the logs
	defer os.RemoveAll(dir)
	log, err := combine.Files(filepath.Join(dir, "alpha.zevtc"), filepath.Join(dir, "bravo.zevtc"))
	if err != nil {
		panic(err)
	}
	fight := filepath.Join(dir, "fight.zevtc")
	if err := log.WriteFile(fight); err != nil {
		panic(err)
	}
	tl, err := timeline.ParseFile(fight) // an ordinary log
	if err != nil {
		panic(err)
	}
	fmt.Println(filepath.Base(fight), "|", tl.Players().Count(), "players,", tl.NPCs().Count(), "creatures,", tl.Duration)
	// Output:
	// fight.zevtc | 3 players, 3 creatures, 10s
}

func Example_everyAgentOnce() {
	byAlpha, byBravo := cookbookLogs()
	log, err := combine.Logs(byAlpha, byBravo)
	if err != nil {
		panic(err)
	}
	tl := mustBuild(log)
	for a := range tl.NPCs().Seq() {
		fmt.Printf("%s: %d hits taken, from %s\n", a.Name, a.HitsTaken().Count(), a.HitsTaken().First().Src.Name)
	}
	// Both clients told the strikes on Knuckles: they are there once.
	first := tl.Agents().Where(func(a *timeline.Agent) bool { return a.Name == "Knuckles" }).First().HitsTaken().First()
	fmt.Println("first strike on Knuckles at", first.Time, "for", first.Damage)
	// Output:
	// Sabetha: 10 hits taken, from Alpha
	// Knuckles: 25 hits taken, from Charlie
	// Kernan: 10 hits taken, from Bravo
	// first strike on Knuckles at 0s for 1000
}

func Example_positions() {
	byAlpha, byBravo := cookbookLogs()
	log, err := combine.Logs(byAlpha, byBravo)
	if err != nil {
		panic(err)
	}
	// Alpha never saw Bravo: its log has no position for that player.
	at := 5 * time.Second
	for _, l := range []*evtc.Log{byAlpha, log} {
		tl := mustBuild(l)
		b := tl.PlayerByName("Bravo").Ref()
		fmt.Println(b.Position.Len(), "samples of Bravo, at", b.PositionAt(at).X, "| distance to Alpha", tl.PlayerByName("Alpha").Ref().DistanceTo(b, at))
	}
	// Output:
	// 0 samples of Bravo, at 0 | distance to Alpha NaN
	// 20 samples of Bravo, at 8000 | distance to Alpha 8000
}

func Example_order() {
	byAlpha, byBravo := cookbookLogs()
	one, err := combine.Logs(byAlpha, byBravo)
	if err != nil {
		panic(err)
	}
	other, err := combine.Logs(byBravo, byAlpha)
	if err != nil {
		panic(err)
	}
	fmt.Println("the same log:", slices.Equal(one.Events, other.Events), slices.Equal(one.Agents, other.Agents))
	// The log of Alpha holds the most events: the combined log is
	// recorded by Alpha, under its addresses.
	tl := mustBuild(one)
	fmt.Printf("recorded by %s | Bravo at %#x, Kernan at %#x\n", tl.POV.Account, tl.PlayerByName("Bravo").Ref().Addr, tl.NPCs().Last().Addr)
	// Output:
	// the same log: true true
	// recorded by Alpha.1234 | Bravo at 0x1000000c, Kernan at 0x20000017
}

func Example_errors() {
	byAlpha, byBravo := cookbookLogs()
	older := *byBravo
	older.Header.Build = "20260816"
	_, err := combine.Logs(byAlpha, &older)
	fmt.Println(errors.Is(err, combine.ErrBuildMismatch), err)

	// Another fight: none of its strikes is in the log of Alpha.
	other := *byBravo
	other.Events = nil
	for _, e := range byBravo.Events {
		e.Value += 5000
		other.Events = append(other.Events, e)
	}
	_, err = combine.Logs(byAlpha, &other)
	fmt.Println(errors.Is(err, combine.ErrFightMismatch), err)
	// Output:
	// true combine: logs of several arcdps builds: 20260920 and 20260816
	// true combine: logs of several fights: 0 strikes shared, 20 wanted
}

func Example_stoppedEarly() {
	byAlpha, byBravo := cookbookLogs()
	// Alpha lost its connection eight seconds into the fight.
	early := *byAlpha
	early.Events = nil
	for _, e := range byAlpha.Events {
		switch at := e.Time - epoch; {
		case e.IsStateChange == evtc.StateSquadCombatEnd:
			e.Time = epoch + 8000
			fallthrough
		case at <= 8000:
			early.Events = append(early.Events, e)
		}
	}
	log, err := combine.Logs(&early, byBravo)
	if err != nil {
		panic(err)
	}
	// Bravo tells the rest of the fight. Nobody else saw Sabetha.
	for _, l := range []*evtc.Log{&early, log} {
		tl := mustBuild(l)
		fmt.Print(tl.Duration, " |")
		for a := range tl.NPCs().Seq() {
			fmt.Printf(" %s: %d hits taken.", a.Name, a.HitsTaken().Count())
		}
		fmt.Println()
	}
	// Output:
	// 8s | Sabetha: 8 hits taken. Knuckles: 21 hits taken.
	// 10s | Knuckles: 25 hits taken. Kernan: 10 hits taken. Sabetha: 8 hits taken.
}
