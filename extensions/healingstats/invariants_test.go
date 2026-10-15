package healingstats

import (
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// checkInvariants verifies the structural properties every Stats must
// satisfy: exact sizing, symmetric edges, sorted lists, consistent fields
// and every event of the addon behind one heal or left out. It runs on
// synthetic logs, on the real logs and in the fuzzer.
func checkInvariants(tb testing.TB, s *Stats) {
	tb.Helper()
	fail := func(format string, args ...any) {
		tb.Helper()
		tb.Errorf(format, args...)
	}
	exact := func(what string, n, c int) {
		if n != c {
			fail("%s: len %d, cap %d", what, n, c)
		}
	}
	tl := s.Timeline
	x := s.Extension
	if x == nil || x.Timeline != tl || x.Signature != Signature || x.Decoded != s || tl.Extension(Signature) != x || Of(tl) != s || s.Version != x.Version {
		fail("stats are not attached to their extension: %+v", x)
	}
	agents := tl.Agents().All()
	if len(s.agents) != len(agents) || len(s.byAgent) != len(agents)+1 {
		fail("%d nodes for %d agents", len(s.agents), len(agents))
	}
	exact("agents", len(s.agents), cap(s.agents))
	exact("players", len(s.players), cap(s.players))
	exact("recorded", len(s.Recorded), cap(s.Recorded))
	exact("heals", len(s.heals), cap(s.heals))
	for i, a := range s.agents {
		if a.Agent != agents[i] || a.Stats != s || s.byAgent[a.Agent] != a || s.Agent(a) != a || s.Agent(a.Agent) != a {
			fail("node %d is inconsistent: %+v", i, a)
		}
	}
	if s.Unknown == nil || s.Unknown.Agent != tl.Unknown || s.byAgent[tl.Unknown] != s.Unknown || s.Unknown.Stats != s || s.Unknown.Recorded {
		fail("unknown node is inconsistent: %+v", s.Unknown)
	}
	chars := tl.Characters().All()
	for i, p := range s.players {
		if p.Agent != chars[i].Agent || s.Agent(chars[i]) != p {
			fail("player node %d is %v, want the node of %v", i, p, chars[i])
		}
	}

	window := uint64(PeerWindow / time.Millisecond)
	seen := map[*evtc.Event]int{}
	merged := 0
	for i, h := range s.heals {
		if i > 0 && h.Time < s.heals[i-1].Time {
			fail("heals are not sorted at %d", i)
		}
		e := h.Event
		if e == nil || e.IsStateChange != evtc.StateExtensionCombat || tl.ExtensionOf(e) != x || h.Time != tl.TimeOf(e) {
			fail("heal %d has an inconsistent event: %+v", i, h)
			continue
		}
		seen[e]++
		if h.Src == nil || h.Dst == nil || s.byAgent[h.Src.Agent] != h.Src || s.byAgent[h.Dst.Agent] != h.Dst || !names(tl, h.Src.Agent, e.SrcAgent, e.SrcInstanceID, h.Time) || !names(tl, h.Dst.Agent, e.DstAgent, e.DstInstanceID, h.Time) {
			fail("heal %d has inconsistent agents: %v -> %v", i, h.Src, h.Dst)
		}
		if h.Skill == nil || h.Skill != tl.Skill(e.SkillID) {
			fail("heal %d has an inconsistent skill: %v", i, h.Skill)
		}
		want := -e.Value
		if e.Buff != 0 {
			want = -e.BuffDamage
		}
		if h.Amount != want || h.IsBuff != (e.Buff != 0) || h.IsBarrier != (e.IsShields != 0) || h.Healed()+h.BarrierGiven() != h.Amount || h.IsHealing() == h.IsBarrier || h.IsDirect() == h.IsBuff {
			fail("heal %d has inconsistent amounts: %+v", i, h)
		}
		srcRec, dstRec := e.IsOffcycle&flagFromSrc != 0, e.IsOffcycle&flagFromDst != 0
		downed := e.IsOffcycle&(flagDowned|flagArcDowned) != 0
		if p := h.PeerEvent; p != nil {
			merged++
			seen[p]++
			side, pside := e.IsOffcycle&both, p.IsOffcycle&both
			if p.IsStateChange != evtc.StateExtensionCombat || tl.ExtensionOf(p) != x || side == pside || side == 0 || pside == 0 || side == both || pside == both ||
				p.SrcInstanceID != e.SrcInstanceID || p.DstInstanceID != e.DstInstanceID || p.SkillID != e.SkillID || p.Value != e.Value || p.BuffDamage != e.BuffDamage ||
				p.Buff != e.Buff || p.IsShields != e.IsShields || max(p.Time, e.Time)-min(p.Time, e.Time) > window {
				fail("heal %d has an inconsistent peer record: %+v vs %+v", i, e, p)
			}
			srcRec, dstRec = true, true
			downed = downed || p.IsOffcycle&(flagDowned|flagArcDowned) != 0
		}
		if h.SrcRecorded != srcRec || h.DstRecorded != dstRec || h.TargetDowned != downed || h.Self() != (h.Src == h.Dst) {
			fail("heal %d has inconsistent flags: %+v", i, h)
		}
		if c := h.Cast; c != nil && (c.Caster != h.Src.Agent || c.Skill != h.Skill || c.Interval.Start > h.Time) {
			fail("heal %d has an inconsistent cast: %+v", i, c)
		}
		if cr := h.Credited(); cr == nil || (cr != h.Src && cr.Agent != h.Src.Master && cr != h.Src.master) || !h.creditedTo(who{one: cr.Agent}) {
			fail("heal %d is credited to %v", i, cr)
		}
		if h.SrcRecorded && h.Src != s.Unknown && !h.Src.Recorded {
			fail("heal %d was written by an unrecorded source %v", i, h.Src)
		}
		if h.DstRecorded && h.Dst != s.Unknown && !h.Dst.Recorded {
			fail("heal %d was written by an unrecorded destination %v", i, h.Dst)
		}
	}
	if merged != s.Merged {
		fail("Merged = %d, %d heals have a peer record", s.Merged, merged)
	}

	// A heal counts by the record of the client of its source: the record
	// of the client of the destination stands for a heal when the client
	// of the source writes nothing, and is left out otherwise.
	writes, recorded := map[*Agent]bool{}, map[*Agent]bool{}
	if tl.POV != nil {
		recorded[s.Agent(tl.POV)] = true
	}
	told := func(e *evtc.Event, src, dst *Agent) {
		if e.IsOffcycle&fromSrc != 0 {
			for _, a := range []*Agent{src, s.teller(src, e.SrcMasterInstanceID, e)} {
				if a != s.Unknown {
					writes[a], recorded[a] = true, true
				}
			}
		}
		if e.IsOffcycle&fromDst != 0 {
			for _, a := range []*Agent{dst, s.teller(dst, e.DstMasterInstanceID, e)} {
				if a != s.Unknown {
					recorded[a] = true
				}
			}
		}
	}
	for _, h := range s.heals {
		told(h.Event, h.Src, h.Dst)
		if h.PeerEvent != nil {
			told(h.PeerEvent, h.Src, h.Dst)
		}
	}
	for _, e := range s.LeftOut {
		src, dst := s.parties(e)
		told(e, s.node(src), s.node(dst))
	}
	for _, a := range s.nodes() {
		if a.writes != writes[a] {
			fail("%v: writes %v, a client wrote heals it dealt %v", a, a.writes, writes[a])
		}
		if a.Recorded != recorded[a] {
			fail("%v: recorded %v, a client wrote records of it %v", a, a.Recorded, recorded[a])
		}
	}
	written := func(e *evtc.Event, src *Agent) bool {
		return writes[src] || writes[s.teller(src, e.SrcMasterInstanceID, e)]
	}
	for i, h := range s.heals {
		if !h.SrcRecorded && h.DstRecorded && written(h.Event, h.Src) {
			fail("heal %d is told by the client of its destination alone and the client of %v writes", i, h.Src)
		}
	}
	exact("records left out", len(s.LeftOut), cap(s.LeftOut))
	for i, e := range s.LeftOut {
		seen[e]++
		if i > 0 && e.Time < s.LeftOut[i-1].Time {
			fail("records left out are not sorted at %d", i)
		}
		src, _ := s.parties(e)
		if e.IsStateChange != evtc.StateExtensionCombat || tl.ExtensionOf(e) != x || e.IsOffcycle&both != fromDst || !written(e, s.node(src)) {
			fail("record %d is left out: %+v", i, e)
		}
	}
	// Every event of the addon is behind exactly one heal, or left out.
	for ev := range x.Events().Seq() {
		if seen[ev] != 1 {
			fail("event at %v is behind %d heals", tl.TimeOf(ev), seen[ev])
		}
	}
	if total := x.Events().Count(); total != len(s.heals)+s.Merged+len(s.LeftOut) {
		fail("%d events for %d heals, %d merges and %d records left out", total, len(s.heals), s.Merged, len(s.LeftOut))
	}
	// No record stays alone within PeerWindow of the record the other
	// client wrote of the same game event.
	alone := map[peerKey][]*evtc.Event{}
	lone := func(e *evtc.Event) {
		if side := e.IsOffcycle & both; side == fromSrc || side == fromDst {
			k := peerKey{e.SrcInstanceID, e.DstInstanceID, e.SkillID, e.Value, e.BuffDamage, e.Buff, e.IsShields}
			alone[k] = append(alone[k], e)
		}
	}
	for _, h := range s.heals {
		if h.PeerEvent == nil {
			lone(h.Event)
		}
	}
	for _, e := range s.LeftOut {
		lone(e)
	}
	for _, list := range alone {
		for i, a := range list {
			for _, b := range list[i+1:] {
				if a.IsOffcycle&both != b.IsOffcycle&both && max(a.Time, b.Time)-min(a.Time, b.Time) <= window {
					fail("the records at %v and %v tell one heal and stay alone", tl.TimeOf(a), tl.TimeOf(b))
				}
			}
		}
	}

	dealt, taken := 0, 0
	for _, a := range s.nodes() {
		exact("heals of "+a.String(), len(a.heals), cap(a.heals))
		exact("heals taken by "+a.String(), len(a.healsTaken), cap(a.healsTaken))
		exact("credited heals of "+a.String(), len(a.healsCredited), cap(a.healsCredited))
		exact("agents given to "+a.String(), len(a.adopted), cap(a.adopted))
		for _, m := range a.adopted {
			if m.master != a {
				fail("%v is given to %v and has the master %v", m, a, m.master)
			}
		}
		// The records give a master to the agents arcdps does not list, and
		// to those alone.
		if m := a.master; m != nil && (a == s.Unknown || a.Raw != nil || a.Master != nil || m == a || m == s.Unknown || !slices.Contains(m.adopted, a) || !recordNames(s, a, m)) {
			fail("%v has the master %v, which no record gives it", a, m)
		}
		for j, h := range a.heals {
			if h.Src != a || (j > 0 && h.Time < a.heals[j-1].Time) {
				fail("heal %d of %v is inconsistent: %+v", j, a, h)
			}
		}
		for j, h := range a.healsTaken {
			if h.Dst != a || (j > 0 && h.Time < a.healsTaken[j-1].Time) {
				fail("heal taken %d of %v is inconsistent: %+v", j, a, h)
			}
		}
		n := s.minionHeals(a)
		if (a.healsCredited == nil) != (n == 0) || (a.healsCredited != nil && len(a.healsCredited) != len(a.heals)+n) {
			fail("credited heals of %v: %d, want %d", a, len(a.healsCredited), len(a.heals)+n)
		}
		for j, h := range a.healsCredited {
			if !h.creditedTo(who{one: a.Agent}) || (j > 0 && h.Time < a.healsCredited[j-1].Time) {
				fail("credited heal %d of %v is inconsistent: %+v", j, a, h)
			}
		}
		dealt += len(a.heals)
		taken += len(a.healsTaken)
		if a.Player != nil && a.Recorded != slices.Contains(s.Recorded, a) {
			fail("%v: recorded %v but listed %v", a, a.Recorded, !a.Recorded)
		}
		if a.Player == nil && slices.Contains(s.Recorded, a) {
			fail("%v is listed as recorded but is not a player", a)
		}
	}
	if dealt != len(s.heals) || taken != len(s.heals) {
		fail("%d heals dealt and %d taken for %d heals", dealt, taken, len(s.heals))
	}
	if tl.POV != nil && !s.Agent(tl.POV).Recorded {
		fail("the recording player is not recorded")
	}
	j := 0
	for _, p := range s.players {
		if p.Recorded {
			if j >= len(s.Recorded) || s.Recorded[j] != p {
				fail("recorded players are not in table order: %v", s.Recorded)
				break
			}
			j++
		}
	}
}

// recordNames reports whether a record of the addon names a with the
// master m.
func recordNames(s *Stats, a, m *Agent) bool {
	tl := s.Timeline
	for e := range s.Extension.Events().Seq() {
		t := tl.TimeOf(e)
		src, dst := s.parties(e)
		if src == a.Agent && e.SrcMasterInstanceID != 0 && tl.AgentAt(e.SrcMasterInstanceID, t) == m.Agent {
			return true
		}
		if dst == a.Agent && e.DstMasterInstanceID != 0 && tl.AgentAt(e.DstMasterInstanceID, t) == m.Agent {
			return true
		}
	}
	return false
}

// names reports whether a is the agent a record names: the agent of its
// address when the agent table declares it, otherwise the agent that
// carried the instance id at t, otherwise the agent of the address.
func names(tl *timeline.Timeline, a *timeline.Agent, addr uint64, inst uint16, t time.Duration) bool {
	byAddr := tl.AgentOf(addr, inst)
	if byAddr != nil && byAddr != tl.Unknown && byAddr.Raw != nil {
		return a == byAddr
	}
	if byInst := tl.AgentAt(inst, t); inst != 0 && byInst != nil {
		return a == byInst && byInst.InstanceID == inst
	}
	return a == byAddr || (byAddr == nil && a == tl.Unknown)
}

func TestInvariantsSynthetic(t *testing.T) {
	late, left := 0, 0
	for seed := uint64(1); seed <= 8; seed++ {
		s := mustBuild(t, genLog(seed, 60))
		if s.Merged == 0 || s.Heals().Count() == 0 || len(s.Recorded) != 2 {
			t.Errorf("seed %d: %d heals, %d merged, %d recorded", seed, s.Heals().Count(), s.Merged, len(s.Recorded))
		}
		checkInvariants(t, s)
		late += s.Heals().Where(func(h *Heal) bool {
			return h.PeerEvent != nil && s.Timeline.TimeOf(h.PeerEvent)-h.Time > windows[0]
		}).Count()
		left += len(s.LeftOut)
	}
	if late == 0 || left == 0 {
		t.Errorf("%d copies came late, %d records are left out", late, left)
	}
}
