package healingstats

import (
	"cmp"
	"slices"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// The flags the addon writes in the is_offcycle byte of every event, above
// the bits arcdps uses.
const (
	// flagDowned is set on the tick of a buff received by a downed target.
	// arcdps keeps its own target downed flag in bit 0 of the field, for
	// direct heals too.
	flagDowned    = 1 << 5
	flagArcDowned = 1
	// flagFromDst is set when the client of the destination, or of its
	// master, wrote the event; flagFromSrc when the client of the source
	// did.
	flagFromDst = 1 << 6
	flagFromSrc = 1 << 7
)

// record holds the event of one heal, its parties and, when the clients
// of both parties wrote it, the peer event.
type record struct {
	ev, peer *evtc.Event
	src, dst *Agent
}

// windows are the spans the two records of a heal are looked for within,
// one after the other. The closest records pair first, so that a copy
// that comes late leaves the record of another heal alone.
var windows = [...]time.Duration{500 * time.Millisecond, 5 * time.Second, 20 * time.Second, PeerWindow}

// peerKey identifies the game event behind a record, so that the two
// records of one heal match: the same instance ids, skill, amounts and
// kind. Instance ids rather than addresses, since the record shared by a
// squad member carries only the addresses the recording client could
// translate.
type peerKey struct {
	src, dst       uint16
	skill          uint32
	value, buffDmg int32
	buff, shields  uint8
}

// build decodes the events of the extension into its stats.
func build(x *timeline.Extension) *Stats {
	s := &Stats{Timeline: x.Timeline, Extension: x, Version: x.Version, Revision: int(x.Event.SrcAgent >> 32 & 0xffffff)}
	s.makeAgents()
	s.adopt()
	s.fill(s.merge())
	s.finish()
	return s
}

// makeAgents creates one node per agent of the timeline plus the node of
// the Unknown sentinel.
func (s *Stats) makeAgents() {
	tl := s.Timeline
	count := tl.Agents().Count()
	arena := make([]Agent, count+1)
	s.agents = make([]*Agent, 0, count)
	s.byAgent = make(map[*timeline.Agent]*Agent, len(arena))
	for a := range tl.Agents().Seq() {
		n := &arena[len(s.agents)]
		*n = Agent{Agent: a, Stats: s}
		s.agents = append(s.agents, n)
		s.byAgent[a] = n
	}
	s.Unknown = &arena[count]
	*s.Unknown = Agent{Agent: tl.Unknown, Stats: s}
	s.byAgent[tl.Unknown] = s.Unknown
	s.players = make([]*Agent, 0, tl.Characters().Count())
	for c := range tl.Characters().Seq() {
		s.players = append(s.players, s.byAgent[c.Agent])
	}
}

// adopt gives each agent arcdps does not list, and without a master, the
// first master the records name for it.
func (s *Stats) adopt() {
	tl := s.Timeline
	total := 0
	for _, n := range s.agents {
		a := n.Agent
		if a.Raw != nil || a.Master != nil {
			continue
		}
		for e := range a.Events().Seq() {
			if tl.ExtensionOf(e) != s.Extension {
				continue
			}
			// The record is about a unless its instance id names another
			// agent then.
			t := tl.TimeOf(e)
			if (e.SrcMasterInstanceID != 0 && s.resolve(e.SrcAgent, e.SrcInstanceID, t) == a && s.adoptBy(n, e.SrcMasterInstanceID, t)) ||
				(e.DstMasterInstanceID != 0 && s.resolve(e.DstAgent, e.DstInstanceID, t) == a && s.adoptBy(n, e.DstMasterInstanceID, t)) {
				total++
				break
			}
		}
	}
	if total == 0 {
		return
	}
	refs := make([]*Agent, total)
	for _, a := range s.agents {
		a.adopted = carve(&refs, a.cnt.adopted)
	}
	for _, a := range s.agents {
		if a.master != nil {
			a.master.adopted = append(a.master.adopted, a)
		}
	}
}

// adoptBy gives n the master with the given instance id at t, and reports
// whether the instance id names another agent of the log.
func (s *Stats) adoptBy(n *Agent, master uint16, t time.Duration) bool {
	tl := s.Timeline
	m := tl.AgentAt(master, t)
	if m == nil || m == n.Agent || m == tl.Unknown {
		return false
	}
	n.master = s.byAgent[m]
	n.master.cnt.adopted++
	return true
}

// nodes returns every node, the Unknown one last.
func (s *Stats) nodes() []*Agent { return append(slices.Clone(s.agents), s.Unknown) }

// node returns the node of a timeline agent, the Unknown node for nil.
func (s *Stats) node(a *timeline.Agent) *Agent {
	if n := s.byAgent[a]; n != nil {
		return n
	}
	return s.Unknown
}

// merge collects the records of the extension and returns one per heal,
// in time order. A heal counts once, by the record of the client of its
// source, as in the windows of the addon; the record of the client of the
// destination counts when the client of the source writes nothing. The
// two records of a heal written by both clients are paired to date it:
// the record of the recording player's own client, written at once, is
// kept as the record of the heal. The records of a destination left
// without the record of the source go to LeftOut.
func (s *Stats) merge() []record {
	tl := s.Timeline
	events := s.Extension.Events()
	recs := make([]record, 0, events.Count())
	for e := range events.Seq() {
		recs = append(recs, record{ev: e})
	}
	pair(recs)

	var pov *timeline.Agent
	if tl.POV != nil {
		pov = tl.POV.Ref()
	}
	held := recs[:0]
	for _, r := range recs {
		if r.ev == nil {
			continue
		}
		if r.peer != nil && pov != nil && !s.local(r.ev, pov) && s.local(r.peer, pov) {
			r.ev, r.peer = r.peer, r.ev
		}
		src, dst := s.parties(r.ev)
		r.src, r.dst = s.node(src), s.node(dst)
		s.mark(r.ev, r.src, r.dst)
		if r.peer != nil {
			s.mark(r.peer, r.src, r.dst)
		}
		held = append(held, r)
	}

	n := 0
	for _, r := range held {
		if s.leftOut(r) {
			n++
		}
	}
	if n > 0 {
		s.LeftOut = make([]*evtc.Event, 0, n)
	}
	recs = held[:0]
	for _, r := range held {
		if s.leftOut(r) {
			s.LeftOut = append(s.LeftOut, r.ev)
			continue
		}
		recs = append(recs, r)
	}
	// The record of the recording client can be the later one: restore the
	// time order.
	slices.SortStableFunc(recs, func(a, b record) int { return cmp.Compare(a.ev.Time, b.ev.Time) })
	return recs
}

// pair pairs the two records of the heals both clients wrote: a record
// written by one client and the one the other client wrote of the same
// game event, within each of the windows in turn, the earliest first. The
// later record of a pair leaves recs and becomes the peer of the earlier
// one.
func pair(recs []record) {
	const both = flagFromSrc | flagFromDst
	// The game events the records written by one client name, with the
	// number of their records and the clients that wrote them.
	events := map[peerKey]int32{}
	of := make([]int32, len(recs))
	var counts []int
	var sides []uint8
	for i, r := range recs {
		e := r.ev
		side := e.IsOffcycle & both
		if side != flagFromSrc && side != flagFromDst {
			of[i] = -1
			continue
		}
		k := peerKey{e.SrcInstanceID, e.DstInstanceID, e.SkillID, e.Value, e.BuffDamage, e.Buff, e.IsShields}
		g, ok := events[k]
		if !ok {
			g = int32(len(counts))
			events[k] = g
			counts, sides = append(counts, 0), append(sides, 0)
		}
		of[i] = g
		counts[g]++
		sides[g] |= side
	}
	// Only the events both clients wrote hold pairs: their records, in
	// time order, from one arena.
	total := 0
	for g, n := range counts {
		if sides[g] == both {
			total += n
		}
	}
	if total == 0 {
		return
	}
	arena := make([]int32, total)
	lists := make([][]int32, len(counts))
	for g, n := range counts {
		if sides[g] == both {
			lists[g] = carve(&arena, n)
		}
	}
	for i, g := range of {
		if g >= 0 && sides[g] == both {
			lists[g] = append(lists[g], int32(i))
		}
	}

	// waiting holds the records still alone that the next one may pair
	// with, from the index from on, and alone counts those of each
	// client, the one of the destination first: a wider window is tried
	// while both have some.
	var waiting []int32
	for _, list := range lists {
		for _, w := range windows {
			window := uint64(w / time.Millisecond)
			waiting = waiting[:0]
			from, alone := 0, [2]int{}
			for _, i := range list {
				r := &recs[i]
				if r.ev == nil || r.peer != nil {
					continue
				}
				side := r.ev.IsOffcycle & both
				for from < len(waiting) && r.ev.Time-recs[waiting[from]].ev.Time > window {
					from++
				}
				j := from
				for j < len(waiting) && recs[waiting[j]].ev.IsOffcycle&both == side {
					j++
				}
				if j == len(waiting) {
					waiting = append(waiting, i)
					alone[side>>7]++
					continue
				}
				recs[waiting[j]].peer, r.ev = r.ev, nil
				waiting = append(waiting[:j], waiting[j+1:]...)
				alone[1-side>>7]--
			}
			if alone[0] == 0 || alone[1] == 0 {
				break
			}
		}
	}
}

// teller returns the node of the agent whose client wrote a side of the
// record e, a being the agent of that side and master the instance id e
// names as its master: the agent of that instance id, a itself when e
// names none or one the log does not know. The master the timeline gives
// a may be the one of another agent, since the addon may write the address
// of an agent that left for the one that took its instance id.
func (s *Stats) teller(a *Agent, master uint16, e *evtc.Event) *Agent {
	tl := s.Timeline
	if master == 0 {
		return a
	}
	if m := s.byAgent[tl.AgentAt(master, tl.TimeOf(e))]; m != nil && m != s.Unknown {
		return m
	}
	return a
}

// mark notes what the record e tells of the clients that run the addon,
// src and dst being its parties. The side whose client wrote e is
// recorded, with the master e names for it, and the client of a source
// writes the heals it deals. The Unknown sentinel has no client.
func (s *Stats) mark(e *evtc.Event, src, dst *Agent) {
	if e.IsOffcycle&flagFromSrc != 0 {
		for _, a := range [...]*Agent{src, s.teller(src, e.SrcMasterInstanceID, e)} {
			if a != s.Unknown {
				a.Recorded, a.writes = true, true
			}
		}
	}
	if e.IsOffcycle&flagFromDst != 0 {
		for _, a := range [...]*Agent{dst, s.teller(dst, e.DstMasterInstanceID, e)} {
			if a != s.Unknown {
				a.Recorded = true
			}
		}
	}
}

// leftOut reports whether r is the record the client of a destination
// wrote of a heal the client of its source did not tell, although it
// writes the heals its player or the minions of its player deal.
func (s *Stats) leftOut(r record) bool {
	if r.peer != nil || r.ev.IsOffcycle&(flagFromSrc|flagFromDst) != flagFromDst {
		return false
	}
	return r.src.writes || s.teller(r.src, r.ev.SrcMasterInstanceID, r.ev).writes
}

// local reports whether e was written by the client of the recording
// player: the flags name a side of the event and that side is the player
// or one of its minions.
func (s *Stats) local(e *evtc.Event, pov *timeline.Agent) bool {
	src, dst := s.parties(e)
	return (e.IsOffcycle&flagFromSrc != 0 && s.teller(s.node(src), e.SrcMasterInstanceID, e).Agent == pov) ||
		(e.IsOffcycle&flagFromDst != 0 && s.teller(s.node(dst), e.DstMasterInstanceID, e).Agent == pov)
}

// parties returns the source and the destination of a record. The addon
// rewrites the addresses of the records a squad member shares through its
// own agent table, which misses some agents (minions, allied NPCs): the
// address is then 0 or one the log never declares, while the instance id
// is right. Such an address is resolved through the instance id at the
// time of the record, as the timeline does for the state events arcdps
// writes without a source.
func (s *Stats) parties(e *evtc.Event) (src, dst *timeline.Agent) {
	t := s.Timeline.TimeOf(e)
	return s.resolve(e.SrcAgent, e.SrcInstanceID, t), s.resolve(e.DstAgent, e.DstInstanceID, t)
}

// resolve returns the agent of an address, or of the instance id at t when
// the address is 0 or absent from the agent table; the Unknown sentinel
// when neither names an agent.
func (s *Stats) resolve(addr uint64, inst uint16, t time.Duration) *timeline.Agent {
	tl := s.Timeline
	a := tl.AgentOf(addr, inst)
	if a != nil && a != tl.Unknown && a.Raw != nil {
		return a
	}
	if inst != 0 {
		if b := tl.AgentAt(inst, t); b != nil {
			return b
		}
	}
	if a == nil {
		return tl.Unknown
	}
	return a
}

// fill creates the heals in one arena and links them to their agents.
func (s *Stats) fill(recs []record) {
	tl := s.Timeline
	arena := make([]Heal, len(recs))
	s.heals = make([]*Heal, len(recs))
	for i, r := range recs {
		e := r.ev
		h := &arena[i]
		*h = Heal{
			Event:        e,
			PeerEvent:    r.peer,
			Time:         tl.TimeOf(e),
			Src:          r.src,
			Dst:          r.dst,
			Skill:        tl.Skill(e.SkillID),
			IsBarrier:    e.IsShields != 0,
			IsBuff:       e.Buff != 0,
			TargetDowned: e.IsOffcycle&(flagDowned|flagArcDowned) != 0,
			SrcRecorded:  e.IsOffcycle&flagFromSrc != 0,
			DstRecorded:  e.IsOffcycle&flagFromDst != 0,
			IFF:          e.IFF,
			OverNinety:   e.IsNinety != 0,
			UnderFifty:   e.IsFifty != 0,
			Moving:       e.IsMoving&1 != 0,
			TargetMoving: e.IsMoving&2 != 0,
		}
		if h.IsBuff {
			h.Amount = -e.BuffDamage
		} else {
			h.Amount = -e.Value
		}
		if p := r.peer; p != nil {
			h.SrcRecorded = h.SrcRecorded || p.IsOffcycle&flagFromSrc != 0
			h.DstRecorded = h.DstRecorded || p.IsOffcycle&flagFromDst != 0
			h.TargetDowned = h.TargetDowned || p.IsOffcycle&(flagDowned|flagArcDowned) != 0
			s.Merged++
		}
		s.heals[i] = h
		h.Src.cnt.heals++
		h.Dst.cnt.taken++
	}
	nodes := s.nodes()
	refs := make([]*Heal, 2*len(arena))
	for _, a := range nodes {
		a.heals = carve(&refs, a.cnt.heals)
		a.healsTaken = carve(&refs, a.cnt.taken)
	}
	for _, h := range s.heals {
		h.Src.heals = append(h.Src.heals, h)
		h.Dst.healsTaken = append(h.Dst.healsTaken, h)
	}
}

// carve takes the first n elements of an arena as an empty slice with
// capacity n and advances the arena past them.
func carve[T any](arena *[]T, n int) []T {
	out := (*arena)[:0:n]
	*arena = (*arena)[n:]
	return out
}

// finish derives the credited heals, the casts and the recorded players.
func (s *Stats) finish() {
	nodes := s.nodes()

	// Credited heals: a master whose minions healed gets its own heals
	// merged with theirs, in time order. Everyone else answers with its
	// own heals.
	total := 0
	for _, a := range nodes {
		if n := s.minionHeals(a); n > 0 {
			total += len(a.heals) + n
		}
	}
	credited := make([]*Heal, total)
	for _, a := range nodes {
		n := s.minionHeals(a)
		if n == 0 {
			continue
		}
		a.healsCredited = carve(&credited, len(a.heals)+n)
		a.healsCredited = append(a.healsCredited, a.heals...)
		for _, m := range a.Minions {
			a.healsCredited = append(a.healsCredited, s.byAgent[m].heals...)
		}
		for _, m := range a.adopted {
			a.healsCredited = append(a.healsCredited, m.heals...)
		}
		slices.SortStableFunc(a.healsCredited, func(x, y *Heal) int { return cmp.Compare(x.Time, y.Time) })
	}

	for _, a := range nodes {
		if len(a.heals) > 0 {
			s.attachCasts(a)
		}
	}

	// Recorded players: those the records marked and the recording player,
	// whose client registered the addon.
	if pov := s.Timeline.POV; pov != nil {
		s.byAgent[pov.Ref()].Recorded = true
	}
	n := 0
	for _, p := range s.players {
		if p.Recorded {
			n++
		}
	}
	s.Recorded = make([]*Agent, 0, n)
	for _, p := range s.players {
		if p.Recorded {
			s.Recorded = append(s.Recorded, p)
		}
	}
}

// minionHeals counts the heals dealt by the minions of a, adopted ones
// included.
func (s *Stats) minionHeals(a *Agent) int {
	n := 0
	for _, m := range a.Minions {
		n += len(s.byAgent[m].heals)
	}
	for _, m := range a.adopted {
		n += len(m.heals)
	}
	return n
}

// attachCasts links every heal of a to the most recent cast of its skill
// by a started before it, as the timeline attributes hits to casts.
func (s *Stats) attachCasts(a *Agent) {
	casts := a.Agent.Casts().All()
	if len(casts) == 0 {
		return
	}
	last := map[uint32]*timeline.Cast{}
	j := 0
	for _, h := range a.heals {
		for j < len(casts) && casts[j].Interval.Start <= h.Time {
			last[casts[j].Skill.ID] = casts[j]
			j++
		}
		h.Cast = last[h.Skill.ID]
	}
}
