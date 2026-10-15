package combine

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// MaxLogs is the most logs Logs combines at once.
const MaxLogs = 64

var (
	// ErrNoLog is returned when no log is given, or a nil one.
	ErrNoLog = errors.New("combine: no log")
	// ErrTooManyLogs is returned when more than MaxLogs logs are given.
	ErrTooManyLogs = fmt.Errorf("combine: more than %d logs", MaxLogs)
	// ErrBuildMismatch is returned when the logs were not written by the
	// same arcdps build: two builds do not write the same events.
	ErrBuildMismatch = errors.New("combine: logs of several arcdps builds")
	// ErrFightMismatch is returned when a log shares fewer than MinStrikes
	// strikes with the others and fewer than MinTicks ticks of their
	// instance, or started at another time by the clock of the server: it
	// recorded another fight, or too little of this one for its clock to
	// be set.
	ErrFightMismatch = errors.New("combine: logs of several fights")
)

// Logs combines the recordings several players made of one fight into one
// log: every agent once, under one address, and every event once, on one
// clock. The logs must come from the same arcdps build, one the timeline
// reads, and are left as they are. A single log is returned as it is.
//
// The order of the logs does not matter. The one that holds the most
// events gives the combined log its header, its clock, its addresses and
// what a client writes once about its own recording: the point of view,
// the language, the builds. Where it started late or stopped early, the
// others tell the fight.
func Logs(logs ...*evtc.Log) (*evtc.Log, error) {
	switch {
	case len(logs) == 0 || slices.Contains(logs, nil):
		return nil, ErrNoLog
	case len(logs) > MaxLogs:
		return nil, ErrTooManyLogs
	case len(logs) == 1:
		return logs[0], nil
	}
	out, _, err := combine(ranked(logs), false)
	return out, err
}

// Files combines the logs of the .evtc or .zevtc files at the paths
// given, as Logs does.
func Files(paths ...string) (*evtc.Log, error) {
	logs := make([]*evtc.Log, len(paths))
	for i, p := range paths {
		l, err := evtc.ParseFile(p)
		if err != nil {
			return nil, err
		}
		logs[i] = l
	}
	return Logs(logs...)
}

// ranked returns the logs in the order they are read: the one that holds
// the most events first. Logs that hold as many are told apart by what
// they hold, so that the order given never matters.
func ranked(logs []*evtc.Log) []*evtc.Log {
	out := slices.Clone(logs)
	slices.SortStableFunc(out, func(a, b *evtc.Log) int {
		if c := cmp.Compare(len(b.Events), len(a.Events)); c != 0 {
			return c
		}
		if c := cmp.Compare(len(b.Agents), len(a.Agents)); c != 0 {
			return c
		}
		for i := range a.Events {
			x, y := a.Events[i].Bytes(), b.Events[i].Bytes()
			if c := bytes.Compare(x[:], y[:]); c != 0 {
				return c
			}
		}
		return 0
	})
	return out
}

// combine reads the logs in the order given, the first one the reference,
// and returns the combined log, with the state of the run when audited.
func combine(logs []*evtc.Log, audited bool) (*evtc.Log, *combiner, error) {
	recs, err := recordings(logs)
	if err != nil {
		return nil, nil, err
	}

	m := &combiner{recs: recs, byInst: map[uint16][]*entity{}, tellings: map[int32]map[slot][]told{}, before: map[int32]map[slot]told{}}
	if audited {
		m.audit = &audit{pairs: map[key]int{}, gaps: map[evtc.StateChange][]time.Duration{}, alone: map[evtc.StateChange][]time.Duration{}}
	}
	m.ents = entities(recs)
	for _, x := range m.ents {
		if x.inst != 0 && x.kind != timeline.KindPlayer && x.known {
			m.byInst[x.inst] = append(m.byInst[x.inst], x)
		}
	}
	for _, r := range recs {
		for _, c := range r.tl.Characters().All() {
			if x := r.entity[c.Agent]; x != nil && c.InstanceID != 0 && !slices.Contains(m.byInst[c.InstanceID], x) {
				m.byInst[c.InstanceID] = append(m.byInst[c.InstanceID], x)
			}
		}
	}

	kept := m.gather()
	kept = append(kept, m.carry()...)
	m.masters(kept)
	slices.SortStableFunc(kept, before)

	out := &evtc.Log{Header: logs[0].Header}
	kept = m.settle(kept)
	out.Events = make([]evtc.Event, len(kept))
	for i := range kept {
		out.Events[i] = kept[i].e
	}
	if audited {
		m.audit.origin = make([]int8, len(kept))
		for i := range kept {
			m.audit.origin[i] = kept[i].origin
		}
	}
	out.Agents = m.table()
	out.Skills = skills(logs)
	return out, m, nil
}

// recordings builds the timeline of every log and sets their clocks on
// the first one.
func recordings(logs []*evtc.Log) ([]*recording, error) {
	build := strings.TrimSpace(logs[0].Header.Build)
	recs := make([]*recording, len(logs))
	for i, l := range logs {
		if b := strings.TrimSpace(l.Header.Build); b != build {
			return nil, fmt.Errorf("%w: %s and %s", ErrBuildMismatch, build, b)
		}
		tl, err := timeline.Build(l)
		if err != nil {
			return nil, err
		}
		recs[i] = &recording{
			tl: tl, typed: tl.Has(evtc.CapabilityTypedEvents), epoch: epoch(tl),
			entity: map[*timeline.Agent]*entity{}, entry: map[*timeline.Agent]int32{}, members: map[*entity][]*timeline.Agent{},
			unnamed: map[*timeline.Agent]bool{}, given: map[*evtc.Event]*timeline.Agent{},
		}
	}
	if err := setClocks(recs); err != nil {
		return nil, err
	}
	return recs, nil
}

// before orders the events of the combined log: by time, the definitions
// first.
func before(a, b cand) int {
	if c := cmp.Compare(at(&a.e), at(&b.e)); c != 0 {
		return c
	}
	if c := cmp.Compare(a.rank(), b.rank()); c != 0 {
		return c
	}
	// The events a recording wrote at one instant keep the order it
	// wrote them in: a client that gives the number of a stack it removes
	// to the next one writes both at once. Those of the first recording
	// come first.
	if c := cmp.Compare(a.origin, b.origin); c != 0 {
		return c
	}
	return cmp.Compare(a.seq, b.seq)
}

// rank places an event among those of its instant. A bound dated
// otherwise than its recording did comes first for a spawn and after the
// rest for a leave, whatever the recording that tells it; one that stayed
// where it was written keeps its place among the events of its recording.
// What is carried over comes before what the new speaker tells; what an
// entry that does not speak tells comes last of all.
func (c *cand) rank() int {
	switch {
	case c.bound < 0 && c.moved:
		return -2
	case c.carried:
		return -1
	case c.names:
		return 2
	case c.bound > 0 && c.moved:
		return 1
	}
	return 0
}

// table returns the agent table of the combined log: the entries of the
// first recording that stand for an agent, then what the others bring.
func (m *combiner) table() []evtc.Agent {
	heads := map[*evtc.Agent]bool{}
	for _, x := range m.ents {
		if x.raw != nil {
			heads[x.raw] = true
		}
	}
	var out []evtc.Agent
	listed := map[*evtc.Agent]bool{}
	first := m.recs[0].tl.Log.Agents
	for i := range first {
		raw := &first[i]
		// The entries of a player share their address: all are kept.
		if heads[raw] || raw.IsElite != 0xFFFFFFFF {
			listed[raw] = true
			out = append(out, *raw)
		}
	}
	for _, x := range m.ents {
		if x.raw != nil && !listed[x.raw] {
			listed[x.raw] = true
			a := *x.raw
			a.Addr = x.addr
			out = append(out, a)
		}
	}
	return out
}

// skills unites the skill tables of the logs.
func skills(logs []*evtc.Log) []evtc.Skill {
	var out []evtc.Skill
	listed := map[int32]bool{}
	for _, l := range logs {
		for _, s := range l.Skills {
			if !listed[s.ID] {
				listed[s.ID] = true
				out = append(out, s)
			}
		}
	}
	return out
}
