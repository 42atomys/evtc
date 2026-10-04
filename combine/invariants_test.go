package combine

import (
	"slices"
	"testing"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// checkInvariants verifies what holds of every combined log: its events
// are in time order and name agents of its table, under the instance ids
// the recordings name them under, every agent of a recording is in it
// once, the spans of an agent follow each other, and no message two
// recordings tell is in it twice.
func checkInvariants(tb testing.TB, l *evtc.Log, tl *timeline.Timeline, m *combiner) {
	tb.Helper()
	if len(m.audit.origin) != len(l.Events) {
		tb.Fatalf("%d origins for %d events", len(m.audit.origin), len(l.Events))
	}
	listed := map[uint64]bool{}
	for _, a := range l.Agents {
		if listed[a.Addr] && a.IsElite == 0xFFFFFFFF {
			tb.Errorf("%q is listed twice at %#x", a.Name, a.Addr)
		}
		listed[a.Addr] = true
	}
	named, under := namings(m)
	for _, x := range m.ents {
		named[x.addr] = true
	}
	typed := m.recs[0].typed
	var last uint64
	strangers := 0
	for i := range l.Events {
		e := &l.Events[i]
		if at(e) < last {
			tb.Fatalf("event %d (%v) at %d comes after one at %d", i, e.IsStateChange, at(e), last)
		}
		last = at(e)
		k := kindOf(e, typed)
		if k == evtc.StateIIDChange {
			tb.Errorf("event %d is an address change", i)
		}
		r := ruleOf(e, k, typed)
		if r.class.ofRecording() || r.class == classExtension {
			continue
		}
		if r.src && e.SrcAgent != 0 && !named[e.SrcAgent] {
			tb.Errorf("event %d (%v) names the source %#x, which is no agent", i, k, e.SrcAgent)
		}
		if r.dst && e.DstAgent != 0 && !named[e.DstAgent] {
			tb.Errorf("event %d (%v) names the destination %#x, which is no agent", i, k, e.DstAgent)
		}
		if r.src && e.SrcAgent != 0 && e.SrcInstanceID != 0 && !under[naming{e.SrcAgent, e.SrcInstanceID}] {
			if strangers++; strangers <= 3 {
				tb.Errorf("event %d (%v) names the source %#x under the instance id %d, as no recording does", i, k, e.SrcAgent, e.SrcInstanceID)
			}
		}
		if r.dst && e.DstAgent != 0 && e.DstInstanceID != 0 && !under[naming{e.DstAgent, e.DstInstanceID}] {
			if strangers++; strangers <= 3 {
				tb.Errorf("event %d (%v) names the destination %#x under the instance id %d, as no recording does", i, k, e.DstAgent, e.DstInstanceID)
			}
		}
	}
	if strangers > 3 {
		tb.Errorf("%d events name an agent under an instance id no recording names it under", strangers)
	}

	ids := map[int32]bool{}
	for _, x := range m.ents {
		if ids[x.id] || x.id <= 0 {
			tb.Errorf("agent %d is there twice", x.id)
		}
		ids[x.id] = true
		if x.raw != nil && !listed[x.addr] {
			tb.Errorf("%v is not listed", x)
		}
		for i := range x.spans {
			if sp := x.spans[i]; sp.End < sp.Start || (i > 0 && sp.Start <= x.spans[i-1].End) {
				tb.Errorf("%v: spans %v", x, x.spans)
			}
		}
		for i := range x.speak {
			if sg := x.speak[i]; sg.iv.End < sg.iv.Start || (i > 0 && sg.iv.Start <= x.speak[i-1].iv.End) || x.hold(sg.entry) == nil {
				tb.Errorf("%v: speakers %v", x, x.speak)
			}
		}
		if len(x.holds) > 0 && len(x.spans) == 0 {
			tb.Errorf("%v is held and never there", x)
		}
	}
	for ri, r := range m.recs {
		for a := range r.tl.Agents().Seq() {
			x := r.entity[a]
			switch {
			case x == nil && !r.unnamed[a] && (ri == 0 || a.Events().Count() > 0 || a.Kind == timeline.KindPlayer):
				tb.Errorf("recording %d: %s %q stands for no agent", ri, a.Kind, a.Name)
			case x != nil && a.Raw != nil && tl.AgentOf(x.addr, a.InstanceID) == nil:
				tb.Errorf("recording %d: %s %q is not in the combined log", ri, a.Kind, a.Name)
			case x != nil && !slices.Contains(r.members[x], a):
				tb.Errorf("recording %d: %s %q is no member of its agent", ri, a.Kind, a.Name)
			}
		}
	}
	if left := unpaired(m); left != 0 {
		tb.Errorf("%d pairs left unmade", left)
	}
}

// naming is an agent of the combined log, by its address, and an instance
// id an event names it under.
type naming struct {
	addr uint64
	inst uint16
}

// namings lists the instance ids under which the events of the recordings
// name each agent, those of extensions aside: an event that names an agent
// under another one names the wrong agent. The first recording keeps the
// addresses it names no agent of its timeline at: strangers holds them.
func namings(m *combiner) (strangers map[uint64]bool, under map[naming]bool) {
	strangers, under = map[uint64]bool{}, map[naming]bool{}
	add := func(r *recording, addr uint64, inst uint16) {
		a := r.tl.AgentOf(addr, inst)
		switch x := r.entity[a]; {
		case addr == 0:
		case a == nil && r == m.recs[0]:
			strangers[addr], under[naming{addr, inst}] = true, true
		case x != nil:
			under[naming{x.addr, inst}] = true
		}
	}
	for _, r := range m.recs {
		for i := range r.tl.Log.Events {
			e := &r.tl.Log.Events[i]
			if !timed(e.IsStateChange) {
				continue
			}
			rl := ruleOf(e, kindOf(e, r.typed), r.typed)
			if rl.class.ofRecording() || rl.class == classExtension {
				continue
			}
			if rl.src {
				add(r, e.SrcAgent, e.SrcInstanceID)
			}
			if rl.dst {
				add(r, e.DstAgent, e.DstInstanceID)
			}
		}
	}
	return strangers, under
}

// unpaired counts the messages a run on two recordings left alone while
// one of their content, left alone as well, stands within half a second in
// the other recording: a pair the pairing would have made. A message left
// alone otherwise is one the other recording did not see, or dated
// further. Two messages of one content left alone within the widest
// window, 2 s, are as likely two messages: 2 of 60 million pairs over 147
// fights, 1.7 s and 2 s apart. Matching the two lists in order would
// count 175 more over 147 fights, none of them a pair.
func unpaired(m *combiner) int {
	if len(m.recs) != 2 {
		return 0
	}
	left := 0
	for k, a := range m.audit.lone[0] {
		b := m.audit.lone[1][k]
		if len(b) == 0 {
			continue
		}
		slices.Sort(b)
		w := windows[len(windows)-2]
		if k.kind == evtc.StateExtensionCombat {
			w = relay
		}
		for _, t := range a {
			if j, _ := slices.BinarySearch(b, t-w); j < len(b) && b[j] <= t+w {
				left++
			}
		}
	}
	return left
}
