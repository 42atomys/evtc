package timeline

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/42atomys/evtc"
)

// MinBuild is the first arcdps build whose logs encode casts and buffs as
// dedicated state changes. Older logs use the legacy encoding, which this
// package does not support.
const MinBuild = 20260501

// ErrLegacyLog is returned by Build for logs written by an arcdps build
// older than 20260501.
var ErrLegacyLog = errors.New("timeline: legacy log encoding is not supported")

// InstanceTolerance is how far outside an agent lifetime an instance id
// lookup still matches that agent.
const InstanceTolerance = 300 * time.Millisecond

// Timeline is the temporal graph built from a log. Every node of the
// graph is reachable from here, and nodes point to each other in both
// directions.
//
// Times are time.Duration values relative to the squad combat start event
// of the log (or to its first event when that event is missing). The
// exported fields are read-only after Build; a Timeline is safe for
// concurrent use once built.
type Timeline struct {
	// Log is the decoded log the timeline was built from. It is never
	// modified: nodes point into its agent and event slices.
	Log *evtc.Log
	// Build is the arcdps build date of the log, as yyyymmdd.
	Build int
	// Start is the server wall clock at time zero.
	Start time.Time
	// LocalStart is the clock of the recording client at time zero.
	LocalStart time.Time
	// Duration is the time of the end of the log: the squad combat end
	// event, or the last event.
	Duration time.Duration
	// MapID is the game map id, 0 when the log carries none.
	MapID uint32
	// POV is the recording player, nil when unknown.
	POV *Player
	// Agents holds every agent of the agent table, in table order,
	// followed by the agents synthesized for addresses that events
	// reference without a table entry.
	Agents []*Agent
	// Targets holds the boss of the log first, then every NPC or gadget
	// that exchanged hits with the players, in order of first exchange.
	Targets []*Target
	// Skills are the skill definitions, sorted by id.
	Skills []*Skill
	// Buffs are the buff definitions, sorted by id.
	Buffs []*Buff
	// Unknown is the sentinel agent standing for an unknown source or
	// destination (environment, out of range). It is not part of Agents.
	Unknown *Agent
	// Language is the text language of the recording client.
	Language Language
	// GameBuild is the game build number, 0 when the log carries none.
	GameBuild uint32
	// ShardID is the shard id of the session, 0 when the log carries none.
	ShardID uint32
	// FractalScale is the fractal scale, 0 outside fractals.
	FractalScale uint32
	// Ruleset is the game mode of the session, 0 when the log carries
	// none.
	Ruleset Ruleset
	// InstanceStart is the server wall clock at which the map instance
	// started, zero when unknown.
	InstanceStart time.Time
	// ArcBuild is the build string of the arcdps that wrote the log.
	ArcBuild string
	// EndedByMapExit is set when the log ended because the recording
	// player left the map.
	EndedByMapExit bool
	// Ping is the latency of the recording client over time, sampled from
	// the tick events; a value of 0 means unknown.
	Ping Numbers[int]
	// GroundMarkers are the placements of squad markers on the ground, in
	// time order; each lasts until the marker is removed or moved.
	// GroundMarkerAt finds the one covering an instant.
	GroundMarkers []*GroundMarker
	// Rewards are the rewards received during the log, in time order.
	Rewards []*Reward
	// MapChanges are the map changes of the recording client, in time
	// order.
	MapChanges []*MapChange
	// Integrity holds the diagnostic messages arcdps wrote into the log,
	// in log order.
	Integrity []*IntegrityMessage
	// Extensions are the arcdps extensions that registered in the log, in
	// registration order.
	Extensions []*Extension

	players  []*Player
	npcs     []*Agent
	gadgets  []*Agent
	byAddr   map[uint64]*Agent
	alias    map[uint64]uint64
	byInst   map[uint16][]*Agent
	skills   map[uint32]*Skill
	buffs    map[uint32]*Buff
	epoch    uint64
	events   []*evtc.Event
	hits     []*Hit
	casts    []*Cast
	stacks   []*BuffStack
	effects  []*Effect
	missiles []*Missile
	guids    map[contentKey]GUID
	// effectDefaults holds the default duration of effect ids, from the
	// id to GUID associations.
	effectDefaults map[uint32]time.Duration
	extensions     map[uint32]*Extension
}

// Build constructs the timeline of a decoded log.
func Build(l *evtc.Log) (*Timeline, error) {
	if l == nil {
		return nil, errors.New("timeline: nil log")
	}
	build, err := strconv.Atoi(strings.TrimSpace(l.Header.Build))
	if err != nil {
		return nil, fmt.Errorf("timeline: invalid build date %q: %w", l.Header.Build, err)
	}
	if build < MinBuild {
		return nil, fmt.Errorf("%w: build %d is older than %d", ErrLegacyLog, build, MinBuild)
	}
	b := &builder{tl: &Timeline{Log: l, Build: build}, log: l}
	return b.run(), nil
}

// ParseFile decodes the .evtc or .zevtc file at path and builds its
// timeline.
func ParseFile(path string) (*Timeline, error) {
	l, err := evtc.ParseFile(path)
	if err != nil {
		return nil, err
	}
	return Build(l)
}

// NPCs returns the non-player characters of the log, in table order.
func (tl *Timeline) NPCs() []*Agent { return tl.npcs }

// Gadgets returns the gadgets of the log, in table order.
func (tl *Timeline) Gadgets() []*Agent { return tl.gadgets }

// Agent returns the agent with the given address, following address
// changes, or nil when there is none. Address 0, which the log writes when
// it does not know the agent, returns Unknown.
func (tl *Timeline) Agent(addr uint64) *Agent {
	if addr == 0 {
		return tl.Unknown
	}
	return tl.byAddr[tl.canonical(addr)]
}

// canonical follows the address changes recorded by StateIIDChange events
// to the address an agent is registered under.
func (tl *Timeline) canonical(addr uint64) uint64 {
	if len(tl.alias) == 0 {
		return addr
	}
	for range 8 {
		next, ok := tl.alias[addr]
		if !ok || next == addr {
			break
		}
		addr = next
	}
	return addr
}

// AgentAt returns the agent that carried the instance id at time t, or
// nil when there is none. Instance ids are reused by successive agents,
// so the lookup is resolved against agent lifetimes.
func (tl *Timeline) AgentAt(instID uint16, t time.Duration) *Agent {
	var n nearest[*Agent]
	for _, a := range tl.byInst[instID] {
		if n.consider(a, a.Lifetime, t) {
			break
		}
	}
	return n.best
}

// nearest tracks, across a scan of candidates, the one whose lifetime is
// closest to an instant.
type nearest[E any] struct {
	best  E
	dist  time.Duration
	found bool
}

// consider reports whether life contains t, in which case e is the answer
// and the scan can stop. Otherwise e is kept when it is the closest
// candidate so far within InstanceTolerance.
func (n *nearest[E]) consider(e E, life Interval, t time.Duration) bool {
	d := outside(life, t)
	if d == 0 {
		n.best, n.found = e, true
		return true
	}
	if d <= InstanceTolerance && (!n.found || d < n.dist) {
		n.best, n.dist, n.found = e, d, true
	}
	return false
}

// outside returns how far t lies outside iv, 0 when iv contains it.
func outside(iv Interval, t time.Duration) time.Duration {
	return max(iv.Start-t, t-iv.End, 0)
}

// Skill returns the skill with the given id, or nil when the log never
// references it.
func (tl *Timeline) Skill(id uint32) *Skill { return tl.skills[id] }

// Buff returns the buff with the given skill id, or nil when the log
// never references it.
func (tl *Timeline) Buff(id uint32) *Buff { return tl.buffs[id] }

// Interval returns the interval covered by the log, from time zero to
// Duration.
func (tl *Timeline) Interval() Interval { return Interval{End: tl.Duration} }

// WallClock returns the server wall clock at time t.
func (tl *Timeline) WallClock(t time.Duration) time.Time { return tl.Start.Add(t) }

// TimeOf returns the time of a raw event relative to the timeline origin.
// It is only meaningful for events whose Time field is a timestamp.
func (tl *Timeline) TimeOf(e *evtc.Event) time.Duration { return tl.rel(e.Time) }

// Players returns every player of the log, in table order.
func (tl *Timeline) Players() Players { return Players{From(tl.players)} }

// Hits returns every hit of the log, in time order.
func (tl *Timeline) Hits() Hits { return Hits{From(tl.hits)} }

// Casts returns every cast of the log, in start order.
func (tl *Timeline) Casts() Casts { return Casts{From(tl.casts)} }

// Stacks returns every buff stack of the log, in application order.
func (tl *Timeline) Stacks() Stacks { return Stacks{From(tl.stacks)} }

// Events returns every timed raw event of the log, in time order.
func (tl *Timeline) Events() Events { return Events{Query: From(tl.events), tl: tl} }

// Boss returns the boss of the log, nil when the log designates none.
func (tl *Timeline) Boss() *Target {
	for _, t := range tl.Targets {
		if t.Boss {
			return t
		}
	}
	return nil
}

// TargetBySpeciesID returns the first target of the given species in
// order of engagement, or nil. Gadgets match on their volatile id.
func (tl *Timeline) TargetBySpeciesID(id uint16) *Target {
	for _, t := range tl.Targets {
		if t.SpeciesID == id {
			return t
		}
	}
	return nil
}

// TargetsBySpeciesID returns every target of the given species in order
// of engagement, nil when there is none.
func (tl *Timeline) TargetsBySpeciesID(id uint16) []*Target {
	var out []*Target
	for _, t := range tl.Targets {
		if t.SpeciesID == id {
			out = append(out, t)
		}
	}
	return out
}

// TargetBySpeciesIDAt returns the target of the given species that was
// tracked at t: the one whose lifetime contains t, otherwise the nearest
// within InstanceTolerance, otherwise nil.
func (tl *Timeline) TargetBySpeciesIDAt(id uint16, t time.Duration) *Target {
	var n nearest[*Target]
	for _, tg := range tl.Targets {
		if tg.SpeciesID == id && n.consider(tg, tg.Lifetime, t) {
			break
		}
	}
	return n.best
}

// PlayerByAccount returns the player with the given account name, with or
// without its leading colon, or nil.
func (tl *Timeline) PlayerByAccount(account string) *Player {
	account = strings.TrimPrefix(account, ":")
	for _, p := range tl.players {
		if p.Account == account {
			return p
		}
	}
	return nil
}

// PlayerByName returns the player with the given character name, or nil.
func (tl *Timeline) PlayerByName(name string) *Player {
	for _, p := range tl.players {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// AgentsNamed returns every agent with the given name in table order, nil
// when there is none.
func (tl *Timeline) AgentsNamed(name string) []*Agent {
	var out []*Agent
	for _, a := range tl.Agents {
		if a.Name == name {
			out = append(out, a)
		}
	}
	return out
}

// Since returns the part of the log interval from t onwards. Times
// outside the log are clamped to it.
func (tl *Timeline) Since(t time.Duration) Interval {
	return Interval{Start: tl.Interval().Clamp(t), End: tl.Duration}
}

// Until returns the part of the log interval up to t. Times outside the
// log are clamped to it.
func (tl *Timeline) Until(t time.Duration) Interval {
	return Interval{End: tl.Interval().Clamp(t)}
}

// Effects returns every effect of the log, in creation order.
func (tl *Timeline) Effects() Effects { return Effects{From(tl.effects)} }

// Missiles returns every missile of the log, in creation order.
func (tl *Timeline) Missiles() Missiles { return Missiles{From(tl.missiles)} }

// ExtensionEvents returns the combat events written by arcdps extensions,
// registered or not, in time order. Extension.Events narrows them to one
// extension.
func (tl *Timeline) ExtensionEvents() Events { return tl.Events().Of(evtc.StateExtensionCombat) }

// Extension returns the extension registered with the given signature,
// nil when none did.
func (tl *Timeline) Extension(sig uint32) *Extension { return tl.extensions[sig] }

// ExtensionOf returns the extension that wrote an extension combat event,
// nil for any other event and for a signature that never registered.
func (tl *Timeline) ExtensionOf(e *evtc.Event) *Extension {
	if e == nil || e.IsStateChange != evtc.StateExtensionCombat {
		return nil
	}
	return tl.extensions[extensionSignature(e)]
}

// GUID returns the content GUID the log associates with the id of the
// given kind, zero when the log carries none.
func (tl *Timeline) GUID(kind ContentKind, id uint32) GUID {
	return tl.guids[contentKey{kind, id}]
}

// PingAt returns the latency of the recording client at t in milliseconds,
// 0 when unknown.
func (tl *Timeline) PingAt(t time.Duration) int {
	v, _ := tl.Ping.At(t)
	return v
}
