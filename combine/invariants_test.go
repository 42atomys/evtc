package combine

import (
	"cmp"
	"fmt"
	"slices"
	"testing"
	"time"

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

// holding tells how a combined log holds the events of a log.
type holding struct {
	// ordered is set when the events come in the order they were written,
	// those of one instant too.
	ordered bool
	// left tells the events the combined log has no reason to hold.
	left func(*evtc.Event) bool
	// first is the timeline of the recording read first when it holds a
	// part of the log, and mask what tells the addresses of the recording
	// that brings the rest from those of the log, as moved wrote them.
	first *timeline.Timeline
	mask  uint64
}

// sameEvents checks, event by event, that a combined log holds what a log
// holds: tl is the log of the whole fight, on the clock of the recording
// read first. Every event but the address changes is there once, as it was
// written, and nothing else is. An address follows the address changes,
// and an event written without address may name the agent its timeline
// gave it to. A leave written in two events stands where the last one
// does: a spawn or a leave is dated within sameBound of what was written.
// It returns the number of events compared and of those that name an agent
// the log left out.
func sameEvents(tb testing.TB, tl *timeline.Timeline, l *evtc.Log, as holding) (compared, filled int) {
	tb.Helper()
	typed := tl.Has(evtc.CapabilityTypedEvents)
	of := func(e *evtc.Event) rule { return ruleOf(e, kindOf(e, typed), typed) }
	source := slices.DeleteFunc(slices.Clone(tl.Log.Events), func(e evtc.Event) bool {
		return e.IsStateChange == evtc.StateIIDChange || (as.left != nil && as.left(&e))
	})
	sortEvents(source)
	if len(source) != len(l.Events) {
		tb.Errorf("%d events, want %d", len(l.Events), len(source))
	}
	differ := 0
	report := func(format string, args ...any) {
		if differ++; differ <= 3 {
			tb.Errorf(format, args...)
		}
	}
	// named reports whether the combined log names by held the agent the
	// log names by addr.
	named := func(addr, held uint64) bool {
		a := tl.Agent(addr)
		switch {
		case addr == 0:
			// The combined log may name the agent the log left out.
			return true
		case as.first == nil && a != nil:
			return held == a.Addr
		case as.first == nil:
			// The first recording keeps an address it names no agent at.
			return held == addr
		case a == nil:
			// The address of another recording than the first tells
			// nothing, and is left out.
			return held == addr || held == 0
		case as.first.Agent(a.Addr) != nil:
			return held == a.Addr
		}
		// The other recording brings the agent, unless the first one named
		// it in a field the timeline reads no agent from.
		return held == a.Addr^as.mask || held == a.Addr
	}
	// same reports whether g is w, as the combined log writes it.
	same := func(w, g *evtc.Event) bool {
		e, r := *w, of(w)
		if r.src && named(w.SrcAgent, g.SrcAgent) {
			e.SrcAgent = g.SrcAgent
		}
		if r.dst && named(w.DstAgent, g.DstAgent) {
			e.DstAgent = g.DstAgent
		}
		if r.class == classBound {
			e.Time = g.Time
		}
		return e == *g
	}
	// content is what tells an event from the others: all of it but the
	// agents and the time of a bound.
	content := func(e *evtc.Event) evtc.Event {
		c, r := *e, of(e)
		if r.src {
			c.SrcAgent = 0
		}
		if r.dst {
			c.DstAgent = 0
		}
		if r.class == classBound {
			c.Time = 0
		}
		return c
	}
	held := map[evtc.Event][]*evtc.Event{}
	for i := range l.Events {
		g := &l.Events[i]
		held[content(g)] = append(held[content(g)], g)
	}
	place := 0
	for i := range source {
		w := &source[i]
		r := of(w)
		if as.ordered && r.class != classBound {
			// The next event of the combined log that is no bound.
			for place < len(l.Events) && of(&l.Events[place]).class == classBound {
				place++
			}
			if place == len(l.Events) || !same(w, &l.Events[place]) {
				report("%v written at %d comes out of order", w.IsStateChange, w.Time)
			}
			place++
		}
		// The first event of this content that names the same agents.
		k := content(w)
		at := slices.IndexFunc(held[k], func(g *evtc.Event) bool { return same(w, g) })
		if at < 0 {
			report("%v written at %d under the instance ids %d and %d is not in the combined log: %+v", w.IsStateChange, w.Time, w.SrcInstanceID, w.DstInstanceID, *w)
			continue
		}
		g := held[k][at]
		held[k] = slices.Delete(held[k], at, at+1)
		compared++
		if (r.src && w.SrcAgent == 0 && g.SrcAgent != 0) || (r.dst && w.DstAgent == 0 && g.DstAgent != 0) {
			filled++
		}
		if late := int64(g.Time) - int64(w.Time); late < 0 || late > sameBound.Milliseconds() {
			report("%v of the instance id %d written at %d is dated %d ms later", w.IsStateChange, w.SrcInstanceID, w.Time, late)
		}
	}
	for _, alone := range held {
		for _, g := range alone {
			report("%v at %d under the instance ids %d and %d is in the combined log alone: %+v", g.IsStateChange, g.Time, g.SrcInstanceID, g.DstInstanceID, *g)
		}
	}
	if differ > 3 {
		tb.Errorf("%d events differ", differ)
	}
	return compared, filled
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

// margins tells how far a run stood from its tolerances. What names a
// message is its content, and the time tells apart the messages of one
// content: a pair the recordings date far apart, or a message left alone
// close to one of its content, is where a tolerance decided.
type margins struct {
	// pairs counts the pairs made, those of extensions aside, within
	// counts those dated at most 30 ms, 100 ms and 500 ms apart, and
	// furthest is the widest gap.
	pairs    int
	within   [3]int
	furthest time.Duration
	// doubtful counts the messages left alone less than 5 s from one of
	// their content: the same message dated far apart, or another one.
	doubtful int
	// inversions counts the pairs of messages of the server the two
	// recordings tell in another order: every client receives them in
	// the same order, so a pair out of order is a pair made wrong.
	inversions int
}

// inversions counts the pairs of messages of the server that the two
// recordings order the other way: sorted by the time one recording gave
// them, two pairs in a row whose other times go backwards. Pairs dated at
// one instant by a recording are in no order.
func inversions(m *combiner) int {
	var pairs []paired
	for _, p := range m.audit.order {
		if rules[p.kind].class == classServer {
			pairs = append(pairs, p)
		}
	}
	slices.SortFunc(pairs, func(x, y paired) int { return cmp.Compare(x.a, y.a) })
	n := 0
	for i := 1; i < len(pairs); i++ {
		if pairs[i].a > pairs[i-1].a && pairs[i].b < pairs[i-1].b {
			n++
		}
	}
	return n
}

// String tells the margins for the report of a test.
func (g margins) String() string {
	share := func(n int) float64 { return 100 * float64(n) / float64(max(g.pairs, 1)) }
	return fmt.Sprintf("%d pairs, dated within 30 ms %.2f%%, 100 ms %.2f%%, 500 ms %.3f%%, the furthest %v apart; %d messages left alone within 5 s of one of their content; %d pairs of messages of the server out of order",
		g.pairs, share(g.within[0]), share(g.within[1]), share(g.within[2]), g.furthest.Round(time.Millisecond), g.doubtful, g.inversions)
}
