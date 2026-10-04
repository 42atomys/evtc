package combine

import (
	"math"
	"slices"
	"testing"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// The cast of a scene. The id of an agent is the instance id the server
// gives it, and the low part of its address in the log of every client.
const (
	alpha   = 11 // records the first log
	bravo   = 12 // records the second one
	charlie = 13 // records the third one
	boss    = 21
	crow    = 22
	pet     = 23 // a minion of alpha
	cannon  = 31

	skillHeat  = 100
	skillSlam  = 200
	skillMight = 740
	skillRegen = 718

	rawEpoch = 1000
	// healing is the signature of the healing stats addon.
	healing = 0x9c9b3c99
)

// recorder assembles the log of one client of a scene. Times are those of
// the scene, in milliseconds after the squad combat start; the client
// writes them on its own clock and the agents under its own addresses.
type recorder struct {
	l *evtc.Log
	// base is the high part of the addresses of the client, shift what
	// its clock adds to the time of the scene.
	base, shift uint64
}

// scene is a fight several clients record.
type scene struct {
	recs []*recorder
}

// newScene returns a scene recorded by the first n players of the cast.
// Every client lists the three players and the boss, holds them from the
// start and tells the first strikes of the fight, which set the clocks:
// forty strikes of alpha on the boss, one every 50 ms from 100 ms on.
func newScene(n int) *scene {
	s := &scene{}
	for i := range n {
		r := &recorder{
			l:     &evtc.Log{Header: evtc.Header{Build: "20260920", Revision: 1, TargetSpeciesID: 15375}},
			base:  uint64(i+1) << 32,
			shift: uint64(i) * 7_654_322,
		}
		r.l.Events = append(r.l.Events, evtc.Event{Time: r.at(0), SrcAgent: 0x637261, Value: 1787685623, BuffDamage: 1787685624, IsStateChange: evtc.StateSquadCombatStart})
		r.player(alpha, "Alpha", ":Alpha.1234", "1")
		r.player(bravo, "Bravo", ":Bravo.5678", "1")
		r.player(charlie, "Charlie", ":Charlie.9012", "2")
		r.npc(boss, 15375, "Sabetha")
		r.l.Skills = []evtc.Skill{{ID: skillHeat, Name: "Heat"}, {ID: skillSlam, Name: "Slam"}, {ID: skillRegen, Name: "Regeneration"}, {ID: skillMight, Name: "Might"}}
		r.ev(0, evtc.StatePointOfView, uint16(alpha+i), 0, evtc.Event{})
		r.ev(0, evtc.StateLanguage, 0, 0, evtc.Event{SrcAgent: uint64(i)})
		r.ev(0, evtc.StateLogNPCUpdate, 0, boss, evtc.Event{SrcAgent: 15375})
		for _, id := range []uint16{alpha, bravo, charlie, boss} {
			r.ev(0, evtc.StateEnterCombat, id, 0, evtc.Event{})
		}
		s.recs = append(s.recs, r)
	}
	for i := range 40 {
		s.each(func(r *recorder) { r.hit(uint64(100+50*i), alpha, boss, skillHeat, int32(1000+i)) })
	}
	return s
}

// each runs f on every recorder.
func (s *scene) each(f func(r *recorder)) {
	for _, r := range s.recs {
		f(r)
	}
}

// logs ends the scene at the time given and returns the log of every
// client: each writes the end of the squad combat and that it stops
// holding the agents it held.
func (s *scene) logs(end uint64) []*evtc.Log {
	out := make([]*evtc.Log, len(s.recs))
	for i, r := range s.recs {
		held := map[uint64]uint16{}
		for j := range r.l.Events {
			e := &r.l.Events[j]
			if k := e.IsStateChange; tracks(k) && e.SrcAgent != 0 {
				if isLeave(e, k) {
					delete(held, e.SrcAgent)
				} else {
					held[e.SrcAgent] = e.SrcInstanceID
				}
			}
		}
		var ids []uint16
		for _, id := range held {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			r.leave(end, id)
		}
		r.l.Events = append(r.l.Events, evtc.Event{Time: r.at(end), SrcAgent: 0x637261, Value: 1787685623 + int32(end/1000), IsStateChange: evtc.StateSquadCombatEnd})
		out[i] = r.l
	}
	return out
}

func (r *recorder) at(t uint64) uint64 { return rawEpoch + r.shift + t }

// addr returns the address the client gives an agent, 0 for no agent.
func (r *recorder) addr(id uint16) uint64 {
	if id == 0 {
		return 0
	}
	return r.base | uint64(id)
}

func (r *recorder) player(id uint16, name, account, sub string) {
	r.l.Agents = append(r.l.Agents, evtc.Agent{Addr: r.addr(id), Profession: uint32(id % 9), Name: name, Account: account, Subgroup: sub, HitboxWidth: 24})
}

func (r *recorder) npc(id, species uint16, name string) {
	r.l.Agents = append(r.l.Agents, evtc.Agent{Addr: r.addr(id), Profession: uint32(species), IsElite: 0xffffffff, Name: name})
}

func (r *recorder) gadget(id, model uint16, name string) {
	r.l.Agents = append(r.l.Agents, evtc.Agent{Addr: r.addr(id), Profession: 0xffff0000 | uint32(model), IsElite: 0xffffffff, Name: name})
}

// ev writes an event of an agent. A destination of 0 leaves the
// destination fields as e holds them.
func (r *recorder) ev(t uint64, kind evtc.StateChange, src, dst uint16, e evtc.Event) {
	e.Time, e.IsStateChange = r.at(t), kind
	if src != 0 {
		e.SrcAgent, e.SrcInstanceID = r.addr(src), src
	}
	if dst != 0 {
		e.DstAgent, e.DstInstanceID = r.addr(dst), dst
	}
	r.l.Events = append(r.l.Events, e)
}

func withID(e evtc.Event, id uint32) evtc.Event {
	e.Pad61, e.Pad62, e.Pad63, e.Pad64 = uint8(id), uint8(id>>8), uint8(id>>16), uint8(id>>24)
	return e
}

func (r *recorder) hit(t uint64, src, dst uint16, skill uint32, value int32) {
	r.ev(t, evtc.StateCombat, src, dst, evtc.Event{SkillID: skill, Value: value, IFF: evtc.IFFFoe})
}

func (r *recorder) tick(t uint64, src, dst uint16, skill uint32, damage int32) {
	r.ev(t, evtc.StateCombat, src, dst, evtc.Event{SkillID: skill, BuffDamage: damage, Buff: 1, Result: evtc.ResultBuffDamageCycle})
}

func (r *recorder) apply(t uint64, src, dst uint16, buff uint32, duration int32, stack uint32) {
	r.ev(t, evtc.StateBuffApply, src, dst, withID(evtc.Event{SkillID: buff, Value: duration, IsShields: 1}, stack))
}

func (r *recorder) initial(t uint64, src, dst uint16, buff uint32, duration int32, stack uint32) {
	r.ev(t, evtc.StateBuffInitial, src, dst, withID(evtc.Event{SkillID: buff, Value: duration, BuffDamage: duration}, stack))
}

func (r *recorder) remove(t uint64, agent uint16, buff uint32, stack uint32) {
	r.ev(t, evtc.StateBuffRemoveSingle, agent, 0, withID(evtc.Event{SkillID: buff, IsBuffRemove: evtc.BuffRemoveSingle}, stack))
}

// serverTick writes the TICK event of a second: the tick of the server
// and the ping of the client.
func (r *recorder) serverTick(t uint64, tick uint64, ping int32) {
	r.ev(t, evtc.StateTick, 0, 0, evtc.Event{SrcAgent: tick, Value: ping})
}

// instance writes the start of the instance: how long before t it
// started, on the clock of the client, and the socket of the server.
func (r *recorder) instance(t uint64, ago uint64, socket uint32) {
	r.ev(t, evtc.StateInstanceStart, 0, 0, evtc.Event{SrcAgent: r.at(t) - ago, Value: int32(socket)})
}

func (r *recorder) spawn(t uint64, id uint16)   { r.ev(t, evtc.StateSpawn, id, 0, evtc.Event{}) }
func (r *recorder) despawn(t uint64, id uint16) { r.ev(t, evtc.StateDespawn, id, 0, evtc.Event{}) }

// leave writes that the client stops holding the agent: a team change to
// no team.
func (r *recorder) leave(t uint64, id uint16) {
	r.ev(t, evtc.StateTeamChange, id, 0, evtc.Event{})
}

func (r *recorder) team(t uint64, id uint16, to, from uint32) {
	r.ev(t, evtc.StateTeamChange, id, 0, evtc.Event{DstAgent: uint64(to), Value: int32(from)})
}

func (r *recorder) position(t uint64, id uint16, x, y float32) {
	r.ev(t, evtc.StatePosition, id, 0, evtc.Event{DstAgent: uint64(math.Float32bits(x)) | uint64(math.Float32bits(y))<<32})
}

func (r *recorder) health(t uint64, id uint16, percent float64) {
	r.ev(t, evtc.StateHealthPctUpdate, id, 0, evtc.Event{DstAgent: uint64(percent * 100)})
}

func (r *recorder) marker(t uint64, id uint16, marker int32) {
	r.ev(t, evtc.StateMarker, id, 0, evtc.Event{Value: marker})
}

// heal writes a heal as the healing stats addon does: the amount negated
// and the client that computed it in is_offcycle.
func (r *recorder) heal(t uint64, src, dst uint16, skill uint32, amount int32, flags uint8) {
	r.ev(t, evtc.StateExtensionCombat, src, dst, withID(evtc.Event{SkillID: skill, Value: -amount, IsOffcycle: flags}, healing))
}

// audited combines the logs in the order given, the first one the
// reference, and returns the combined log, its timeline and the run.
func audited(tb testing.TB, logs ...*evtc.Log) (*evtc.Log, *timeline.Timeline, *combiner) {
	tb.Helper()
	l, m, err := combine(logs, true)
	if err != nil {
		tb.Fatal(err)
	}
	tl, err := timeline.Build(l)
	if err != nil {
		tb.Fatalf("the combined log does not build: %v", err)
	}
	return l, tl, m
}

// count returns the number of events of a kind in a log.
func count(l *evtc.Log, kind evtc.StateChange) int {
	n := 0
	for i := range l.Events {
		if l.Events[i].IsStateChange == kind {
			n++
		}
	}
	return n
}

// of returns the events of a kind in a log, in log order.
func of(l *evtc.Log, kind evtc.StateChange) []evtc.Event {
	var out []evtc.Event
	for i := range l.Events {
		if l.Events[i].IsStateChange == kind {
			out = append(out, l.Events[i])
		}
	}
	return out
}
