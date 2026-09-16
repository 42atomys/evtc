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

// record holds the event of one heal and, when the clients of both
// parties wrote it, the peer event.
type record struct {
	ev, peer *evtc.Event
}

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
	s.fill(s.merge())
	s.finish()
	return s
}

// makeAgents creates one node per agent of the timeline plus the node of
// the Unknown sentinel.
func (s *Stats) makeAgents() {
	tl := s.Timeline
	arena := make([]Agent, len(tl.Agents)+1)
	s.Agents = make([]*Agent, len(tl.Agents))
	s.byAgent = make(map[*timeline.Agent]*Agent, len(arena))
	for i, a := range tl.Agents {
		n := &arena[i]
		*n = Agent{Agent: a, Stats: s}
		s.Agents[i] = n
		s.byAgent[a] = n
	}
	s.Unknown = &arena[len(tl.Agents)]
	*s.Unknown = Agent{Agent: tl.Unknown, Stats: s}
	s.byAgent[tl.Unknown] = s.Unknown
	s.Players = make([]*Agent, 0, tl.Players().Count())
	for p := range tl.Players().Seq() {
		s.Players = append(s.Players, s.byAgent[p.Agent])
	}
}

// nodes returns every node, the Unknown one last.
func (s *Stats) nodes() []*Agent { return append(slices.Clone(s.Agents), s.Unknown) }

// node returns the node of a timeline agent, the Unknown node for nil.
func (s *Stats) node(a *timeline.Agent) *Agent {
	if n := s.byAgent[a]; n != nil {
		return n
	}
	return s.Unknown
}

// merge collects the records of the extension in time order and pairs the
// two records of a heal written by both clients: the record of one side
// and the record of the other side that names the same game event within
// PeerWindow, the earliest first. The record of the recording player's own
// client is kept as the record of the heal.
func (s *Stats) merge() []record {
	tl := s.Timeline
	events := s.Extension.Events()
	recs := make([]record, 0, events.Count())
	pending := map[peerKey][]int{}
	window := uint64(PeerWindow / time.Millisecond)
	for e := range events.Seq() {
		side := e.IsOffcycle & (flagFromSrc | flagFromDst)
		if side != flagFromSrc && side != flagFromDst {
			recs = append(recs, record{ev: e})
			continue
		}
		k := peerKey{e.SrcInstanceID, e.DstInstanceID, e.SkillID, e.Value, e.BuffDamage, e.Buff, e.IsShields}
		list := pending[k]
		for len(list) > 0 && e.Time-recs[list[0]].ev.Time > window {
			list = list[1:]
		}
		matched := false
		for i, ri := range list {
			if recs[ri].ev.IsOffcycle&(flagFromSrc|flagFromDst) != side {
				recs[ri].peer = e
				list = slices.Delete(list, i, i+1)
				matched = true
				break
			}
		}
		if !matched {
			recs = append(recs, record{ev: e})
			list = append(list, len(recs)-1)
		}
		pending[k] = list
	}
	if tl.POV != nil {
		pov := tl.POV.Agent
		for i := range recs {
			r := &recs[i]
			if r.peer != nil && !s.local(r.ev, pov) && s.local(r.peer, pov) {
				r.ev, r.peer = r.peer, r.ev
			}
		}
		// The record of the recording client can be the later one: restore
		// the time order.
		slices.SortStableFunc(recs, func(a, b record) int { return cmp.Compare(a.ev.Time, b.ev.Time) })
	}
	return recs
}

// local reports whether e was written by the client of the recording
// player: the flags name a side of the event and that side is the player
// or one of its minions.
func (s *Stats) local(e *evtc.Event, pov *timeline.Agent) bool {
	src, dst := s.agents(e)
	return (e.IsOffcycle&flagFromSrc != 0 && credited(src) == pov) ||
		(e.IsOffcycle&flagFromDst != 0 && credited(dst) == pov)
}

// agents returns the source and the destination of a record. The addon
// rewrites the addresses of the records a squad member shares through its
// own agent table, which misses some agents (minions, allied NPCs): the
// address is then 0 or one the log never declares, while the instance id
// is right. Such an address is resolved through the instance id at the
// time of the record, as the timeline does for the state events arcdps
// writes without a source.
func (s *Stats) agents(e *evtc.Event) (src, dst *timeline.Agent) {
	t := s.Timeline.TimeOf(e)
	return s.resolve(e.SrcAgent, e.SrcInstanceID, t), s.resolve(e.DstAgent, e.DstInstanceID, t)
}

// resolve returns the agent of an address, or of the instance id at t when
// the address is 0 or absent from the agent table; the Unknown sentinel
// when neither names an agent.
func (s *Stats) resolve(addr uint64, inst uint16, t time.Duration) *timeline.Agent {
	tl := s.Timeline
	a := tl.Agent(addr)
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

// credited returns the master of a minion, otherwise the agent itself.
func credited(a *timeline.Agent) *timeline.Agent {
	if a != nil && a.Master != nil {
		return a.Master
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
		src, dst := s.agents(e)
		*h = Heal{
			Event:        e,
			PeerEvent:    r.peer,
			Time:         tl.TimeOf(e),
			Src:          s.node(src),
			Dst:          s.node(dst),
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

// finish derives the credited heals, the casts and the recorded agents.
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
		slices.SortStableFunc(a.healsCredited, func(x, y *Heal) int { return cmp.Compare(x.Time, y.Time) })
	}

	for _, a := range nodes {
		if len(a.heals) > 0 {
			s.attachCasts(a)
		}
	}

	// Recorded agents: the sides that wrote heals, with their masters, and
	// the recording player, whose client registered the addon.
	for _, h := range s.heals {
		if h.SrcRecorded {
			s.markRecorded(h.Src)
		}
		if h.DstRecorded {
			s.markRecorded(h.Dst)
		}
	}
	if pov := s.Timeline.POV; pov != nil {
		s.byAgent[pov.Agent].Recorded = true
	}
	n := 0
	for _, p := range s.Players {
		if p.Recorded {
			n++
		}
	}
	s.Recorded = make([]*Agent, 0, n)
	for _, p := range s.Players {
		if p.Recorded {
			s.Recorded = append(s.Recorded, p)
		}
	}
}

// minionHeals counts the heals dealt by the minions of a.
func (s *Stats) minionHeals(a *Agent) int {
	n := 0
	for _, m := range a.Minions {
		n += len(s.byAgent[m].heals)
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

// markRecorded marks a node and its master as recorded; the Unknown
// sentinel never is.
func (s *Stats) markRecorded(a *Agent) {
	if a == s.Unknown {
		return
	}
	a.Recorded = true
	if m := a.Master; m != nil {
		s.byAgent[m].Recorded = true
	}
}
