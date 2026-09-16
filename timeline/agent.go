package timeline

import (
	"fmt"
	"math"
	"time"

	"github.com/42atomys/evtc"
)

// Kind classifies an agent from the profession and elite fields of the
// agent table, following the arcdps rules.
type Kind uint8

const (
	// KindUnknown is an agent referenced by events but absent from the
	// agent table, or the Unknown sentinel of a timeline.
	KindUnknown Kind = iota
	// KindPlayer is a player character.
	KindPlayer
	// KindNPC is a non-player character identified by a stable species id.
	KindNPC
	// KindGadget is a world object identified by a volatile id.
	KindGadget
)

var kindNames = []string{"Unknown", "Player", "NPC", "Gadget"}

// String returns the kind name.
func (k Kind) String() string { return enumString(kindNames, "Kind", int(k)) }

// enumString returns names[v] when it exists, otherwise typ(v).
func enumString(names []string, typ string, v int) string {
	if v >= 0 && v < len(names) && names[v] != "" {
		return names[v]
	}
	return fmt.Sprintf("%s(%d)", typ, v)
}

// LifeState is the life state of an agent over a span of time.
type LifeState uint8

const (
	// LifeUnknown when no state event covers the instant.
	LifeUnknown LifeState = iota
	// LifeAlive when the agent is up.
	LifeAlive
	// LifeDown when the agent is downed and can still be revived.
	LifeDown
	// LifeDead when the agent is dead.
	LifeDead
	// LifeGone when the agent left tracking (despawn or disconnect).
	LifeGone
)

var stateNames = []string{"Unknown", "Alive", "Down", "Dead", "Gone"}

// String returns the state name.
func (s LifeState) String() string { return enumString(stateNames, "LifeState", int(s)) }

// DefianceState is the state of a defiance bar (breakbar), with the values
// used by the game.
type DefianceState uint8

const (
	// DefianceActive when the bar is up and can be damaged.
	DefianceActive DefianceState = iota
	// DefianceRecover when the bar was broken and is regenerating.
	DefianceRecover
	// DefianceImmune when the bar cannot be damaged.
	DefianceImmune
	// DefianceNone when the agent has no bar.
	DefianceNone
)

var defianceNames = []string{"Active", "Recover", "Immune", "None"}

// String returns the state name.
func (d DefianceState) String() string { return enumString(defianceNames, "DefianceState", int(d)) }

// Entity is anything that stands for an agent: *Agent, *Player and *Target.
// Query filters accept an Entity so callers never have to unwrap a Player
// or a Target to reach its Agent. A nil Entity matches nothing.
type Entity interface {
	// Ref returns the agent behind the entity.
	Ref() *Agent
}

// ref returns the agent of an entity, nil for a nil entity.
func ref(e Entity) *Agent {
	if e == nil {
		return nil
	}
	return e.Ref()
}

// never is the predicate of a filter given a nil entity.
func never[E any](*E) bool { return false }

// Agent is one participant of the log: a player, an NPC, a gadget or an
// unknown source. It is the hub of the graph: every hit, cast, buff stack,
// state span and sample that involves the agent is reachable from it, and
// each of those points back to the agent.
//
// The exported fields are read-only after Build.
type Agent struct {
	// Timeline owns the agent.
	Timeline *Timeline
	// Raw is the agent table entry, nil for an agent synthesized from
	// events only.
	Raw *evtc.Agent
	// Addr is the agent address used by events (Event.SrcAgent and
	// Event.DstAgent).
	Addr uint64
	// InstanceID is the in-game instance id seen on events. Instance ids
	// are reused by successive agents, see Timeline.AgentAt.
	InstanceID uint16
	// Kind tells whether the agent is a player, an NPC, a gadget or
	// unknown.
	Kind Kind
	// SpeciesID is the species id of an NPC or the volatile id of a gadget.
	// It is zero for players.
	SpeciesID uint16
	// Name is the agent name, in the language of the log.
	Name string
	// Player is set when the agent is a player.
	Player *Player
	// Target is set when the agent is part of Timeline.Targets().
	Target *Target
	// Master is the owner of a minion (pet, clone, mech, turret), nil
	// otherwise.
	Master *Agent
	// Minions are the agents whose Master is this agent.
	Minions []*Agent
	// AttackTargets are the attack target agents attached to a gadget.
	AttackTargets []*Agent
	// Gadget is the gadget an attack target belongs to.
	Gadget *Agent
	// Lifetime spans the first and last event involving the agent.
	Lifetime Interval

	// Toughness is the toughness of a player, as arcdps reports it.
	Toughness int16
	// Concentration is the concentration of a player, as arcdps reports it.
	Concentration int16
	// Healing is the healing power of a player, as arcdps reports it.
	Healing int16
	// Condition is the condition damage of a player, as arcdps reports it.
	Condition int16
	// HitboxWidth is the hitbox width of the agent.
	HitboxWidth uint16
	// HitboxHeight is the hitbox height of the agent.
	HitboxHeight uint16

	// Position is the position over time, interpolated between samples
	// closer than MoveGap; a teleport is a sample that breaks the
	// interpolation.
	Position Series[Vec3]
	// Velocity is the velocity over time, interpolated like Position.
	Velocity Series[Vec3]
	// Facing is the facing direction over time, interpolated like Position.
	Facing Series[Vec2]
	// Health is the health percentage over time, 0 to 100, held between
	// samples and from the start of the lifetime to the first sample.
	Health Numbers[float64]
	// Barrier is the barrier percentage over time, 0 to 100, held like
	// Health.
	Barrier Numbers[float64]
	// MaxHealth is the maximum health over time, reported for non-players
	// only.
	MaxHealth Numbers[int64]
	// DefiancePercent is the defiance bar percentage over time, 0 to 100,
	// held like Health.
	DefiancePercent Numbers[float64]
	// Life is the alive, down, dead or gone state over time. Every
	// tracked agent starts alive at the start of its lifetime, and the last
	// span extends to the end of the log.
	Life Spans[LifeState]
	// Defiance is the defiance bar state over time.
	Defiance Spans[DefianceState]
	// InCombat is true over the spans where the agent was in combat.
	InCombat Spans[bool]
	// Targetable is the targetable flag over time; the arcdps value 2
	// (unsupported) counts as not targetable.
	Targetable Spans[bool]

	// Downs are the downed states of the agent, in time order.
	Downs []*Down
	// Deaths are the deaths of the agent, in time order.
	Deaths []*Death
	// Breakbars are the periods during which the defiance bar of the agent
	// was active, in time order.
	Breakbars []*Breakbar
	// Markers are the markers the agent wore (commander tag, squad markers,
	// encounter mechanics), in order of application. SquadMarkerAt and
	// IsCommanderAt read them at an instant.
	Markers []*Marker
	// StunBreaks are the disables the agent broke early, in time order.
	StunBreaks []*StunBreak
	// Team is the team id of the agent over time, from the team change
	// events; the first span holds the team before the first change.
	Team Spans[uint32]
	// WeaponSet is the weapon set id of the agent over time, as the game
	// logs it: 4 and 5 are the two land sets, kits, bundles and
	// transformations use other ids. The first span holds the set in use
	// before the first swap.
	WeaponSet Spans[uint32]
	// Stealth is the raw arcdps stealth state of the agent over time. The
	// arcdps README documents 0 for false, 1 for true and 2 for
	// unsupported, but build 20260816 writes 1 for every player when it
	// starts tracking it, so the value is kept as logged.
	Stealth Spans[uint8]
	// Gliding is true while the glider of the agent was deployed.
	Gliding Spans[bool]
	// Transformation is the transformation skill of the agent over time,
	// nil when it wore none.
	Transformation Spans[*Skill]
	// Airborne is true from a jump to its landing.
	Airborne Spans[bool]
	// NameVisible is the name visibility of a gadget or NPC over time; the
	// arcdps value 2 (unsupported) reads as not visible.
	NameVisible Spans[bool]
	// GadgetAnimations are the model animations a gadget or NPC played, in
	// time order.
	GadgetAnimations []*GadgetAnimation

	hits          []*Hit
	hitsTaken     []*Hit
	hitsCredited  []*Hit
	effects       []*Effect
	missiles      []*Missile
	casts         []*Cast
	stacks        []*BuffStack
	stacksApplied []*BuffStack
	events        []*evtc.Event

	// idx is the index of the agent in the counts of the builder.
	idx int
}

// Ref returns the agent itself, satisfying Entity.
func (a *Agent) Ref() *Agent { return a }

// IsPlayer reports whether the agent is a player.
func (a *Agent) IsPlayer() bool { return a.Kind == KindPlayer }

// IsNPC reports whether the agent is a non-player character.
func (a *Agent) IsNPC() bool { return a.Kind == KindNPC }

// IsGadget reports whether the agent is a gadget.
func (a *Agent) IsGadget() bool { return a.Kind == KindGadget }

// Hits returns the hits dealt by the agent itself, in time order. The
// hits of its minions are in HitsCredited.
func (a *Agent) Hits() Hits { return Hits{From(a.hits)} }

// HitsCredited returns the hits dealt by the agent and by its minions
// (pets, clones, turrets, mechs), in time order: what a damage meter
// credits to a player.
func (a *Agent) HitsCredited() Hits {
	if a.hitsCredited != nil {
		return Hits{From(a.hitsCredited)}
	}
	return Hits{From(a.hits)}
}

// HitsTaken returns the hits received by the agent, in time order.
func (a *Agent) HitsTaken() Hits { return Hits{From(a.hitsTaken)} }

// Casts returns the casts of the agent, in start order.
func (a *Agent) Casts() Casts { return Casts{From(a.casts)} }

// Effects returns the effects played around the agent or placed by it, in
// creation order.
func (a *Agent) Effects() Effects { return Effects{From(a.effects)} }

// Missiles returns the missiles owned by the agent, in creation order.
func (a *Agent) Missiles() Missiles { return Missiles{From(a.missiles)} }

// Stacks returns the buff stacks received by the agent, in apply order.
func (a *Agent) Stacks() Stacks { return Stacks{From(a.stacks)} }

// StacksApplied returns the buff stacks applied by the agent on anyone.
func (a *Agent) StacksApplied() Stacks { return Stacks{From(a.stacksApplied)} }

// Events returns the raw log events where the agent is the source or the
// destination, in time order.
func (a *Agent) Events() Events { return Events{Query: From(a.events), tl: a.Timeline} }

// LifeStateAt returns the life state of the agent at t.
func (a *Agent) LifeStateAt(t time.Duration) LifeState {
	s, ok := a.Life.ValueAt(t)
	if !ok {
		return LifeUnknown
	}
	return s
}

// IsAliveAt reports whether the agent was up at t.
func (a *Agent) IsAliveAt(t time.Duration) bool { return a.LifeStateAt(t) == LifeAlive }

// IsDownAt reports whether the agent was downed at t.
func (a *Agent) IsDownAt(t time.Duration) bool { return a.LifeStateAt(t) == LifeDown }

// IsDeadAt reports whether the agent was dead at t.
func (a *Agent) IsDeadAt(t time.Duration) bool { return a.LifeStateAt(t) == LifeDead }

// String formats the agent as name, kind and species id.
func (a *Agent) String() string {
	if a == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s(%s#%d)", a.Name, a.Kind, a.SpeciesID)
}

// Player is an agent controlled by a player, with the account data of the
// agent table. Every Agent method and field is available through the
// embedded Agent.
type Player struct {
	*Agent
	// Account is the account name without its leading colon.
	Account string
	// Subgroup is the squad subgroup, 0 when unknown.
	Subgroup int
	// Profession is the profession of the player.
	Profession Profession
	// EliteSpec is the elite specialization of the player, EliteNone for
	// a core build; Spec names the build.
	EliteSpec EliteSpec
	// Guild is the guild of the player, zero when unknown.
	Guild GUID
}

// Ref returns the agent of the player, or nil for a nil player.
func (p *Player) Ref() *Agent {
	if p == nil {
		return nil
	}
	return p.Agent
}

// Target is an enemy agent of interest: the boss of the log first, then
// every NPC or gadget that exchanged hits with the players.
type Target struct {
	*Agent
	// Boss is set for the agent designated by the log as its boss.
	Boss bool
}

// Ref returns the agent of the target, or nil for a nil target.
func (t *Target) Ref() *Agent {
	if t == nil {
		return nil
	}
	return t.Agent
}

// PositionAt returns the position of the agent at t, interpolated between
// samples closer than MoveGap. arcdps only samples a moving agent, so
// across a wider gap the last position is held: the agent stood still. It
// is the zero Vec3 when the position is unknown, outside the lifetime or
// before the first sample; Position.At tells that apart from the origin.
func (a *Agent) PositionAt(t time.Duration) Vec3 {
	v, _ := a.Position.At(t)
	return v
}

// VelocityAt returns the velocity of the agent at t, held like the
// position, or the zero Vec3 when it is unknown.
func (a *Agent) VelocityAt(t time.Duration) Vec3 {
	v, _ := a.Velocity.At(t)
	return v
}

// FacingAt returns the direction the agent faced at t, held like the
// position, or the zero Vec2 when it is unknown.
func (a *Agent) FacingAt(t time.Duration) Vec2 {
	v, _ := a.Facing.At(t)
	return v
}

// HealthAt returns the health percentage of the agent at t, or 0 when it
// is unknown.
func (a *Agent) HealthAt(t time.Duration) float64 {
	v, _ := a.Health.At(t)
	return v
}

// BarrierAt returns the barrier percentage of the agent at t, or 0 when it
// is unknown.
func (a *Agent) BarrierAt(t time.Duration) float64 {
	v, _ := a.Barrier.At(t)
	return v
}

// MaxHealthAt returns the maximum health of the agent at t, or 0 when it
// is unknown; it is only reported for non-players.
func (a *Agent) MaxHealthAt(t time.Duration) int64 {
	v, _ := a.MaxHealth.At(t)
	return v
}

// DefiancePercentAt returns the defiance bar percentage of the agent at
// t, or 0 when it is unknown.
func (a *Agent) DefiancePercentAt(t time.Duration) float64 {
	v, _ := a.DefiancePercent.At(t)
	return v
}

// DefianceStateAt returns the state of the defiance bar of the agent at t,
// DefianceNone when no bar state is known at t.
func (a *Agent) DefianceStateAt(t time.Duration) DefianceState {
	s, ok := a.Defiance.ValueAt(t)
	if !ok {
		return DefianceNone
	}
	return s
}

// DistanceTo returns the distance in game units between the agent and e
// at t. It is NaN when e is nil or either position is unknown, so that
// every comparison with it is false; test it with math.IsNaN.
func (a *Agent) DistanceTo(e Entity, t time.Duration) float64 {
	o := ref(e)
	if o == nil {
		return math.NaN()
	}
	p, ok := a.Position.At(t)
	q, ok2 := o.Position.At(t)
	if !ok || !ok2 {
		return math.NaN()
	}
	return p.DistTo(q)
}

// IsInCombatAt reports whether the agent was in combat at t.
func (a *Agent) IsInCombatAt(t time.Duration) bool {
	v, ok := a.InCombat.ValueAt(t)
	return ok && v
}

// IsTargetableAt reports whether the agent was targetable at t.
func (a *Agent) IsTargetableAt(t time.Duration) bool {
	v, ok := a.Targetable.ValueAt(t)
	return ok && v
}

// HealthCrossings returns every time the health of the agent moved through
// one of the given percentages, in both directions and in chronological
// order.
func (a *Agent) HealthCrossings(levels ...float64) []Crossing[float64] {
	return a.Health.Crossings(levels...)
}

// HealthBelow returns the first time the health of the agent was below
// the given percentage. The boolean is false when it never was.
func (a *Agent) HealthBelow(level float64) (time.Duration, bool) {
	return a.Health.FirstBelow(level)
}

// HealthAbove returns the first time the health of the agent was at or
// above the given percentage. The boolean is false when it never was.
func (a *Agent) HealthAbove(level float64) (time.Duration, bool) {
	return a.Health.FirstAbove(level)
}

// DiedBefore reports whether the agent died at or before t.
func (a *Agent) DiedBefore(t time.Duration) bool {
	return len(a.Deaths) > 0 && a.Deaths[0].Time <= t
}

// DiedBetween reports whether the agent died within iv.
func (a *Agent) DiedBetween(iv Interval) bool {
	for _, d := range a.Deaths {
		if iv.Contains(d.Time) {
			return true
		}
	}
	return false
}

// DownedBetween reports whether the agent was down at some instant of iv.
func (a *Agent) DownedBetween(iv Interval) bool {
	for _, d := range a.Downs {
		if d.Overlaps(iv) {
			return true
		}
	}
	return false
}

// DownsOf returns the downs whose Cause hit is of skill s, nil when there
// are none or s is nil.
func (a *Agent) DownsOf(s *Skill) []*Down {
	if s == nil {
		return nil
	}
	return a.DownsOfSkill(s.ID)
}

// DownsOfSkill returns the downs whose Cause hit is of the skill with the
// given id, nil when there are none.
func (a *Agent) DownsOfSkill(id uint32) []*Down {
	return a.downs(func(h *Hit) bool { return h.Skill.ID == id })
}

// DownsBy returns the downs whose Cause hit was dealt by e or by one of
// its minions, nil when there are none or e is nil.
func (a *Agent) DownsBy(e Entity) []*Down {
	o := ref(e)
	if o == nil {
		return nil
	}
	return a.downs(func(h *Hit) bool { return h.creditedTo(o) })
}

// downs returns the downs whose Cause hit is accepted by p.
func (a *Agent) downs(p func(*Hit) bool) []*Down {
	var out []*Down
	for _, d := range a.Downs {
		if d.Cause != nil && p(d.Cause) {
			out = append(out, d)
		}
	}
	return out
}

// DeathsOf returns the deaths whose Cause hit is of skill s, nil when
// there are none or s is nil.
func (a *Agent) DeathsOf(s *Skill) []*Death {
	if s == nil {
		return nil
	}
	return a.DeathsOfSkill(s.ID)
}

// DeathsOfSkill returns the deaths whose Cause hit is of the skill with
// the given id, nil when there are none.
func (a *Agent) DeathsOfSkill(id uint32) []*Death {
	return a.deaths(func(h *Hit) bool { return h.Skill.ID == id })
}

// DeathsBy returns the deaths whose Cause hit was dealt by e or by one of
// its minions, nil when there are none or e is nil.
func (a *Agent) DeathsBy(e Entity) []*Death {
	o := ref(e)
	if o == nil {
		return nil
	}
	return a.deaths(func(h *Hit) bool { return h.creditedTo(o) })
}

// deaths returns the deaths whose Cause hit is accepted by p.
func (a *Agent) deaths(p func(*Hit) bool) []*Death {
	var out []*Death
	for _, d := range a.Deaths {
		if d.Cause != nil && p(d.Cause) {
			out = append(out, d)
		}
	}
	return out
}

// PhasesByHealth splits the lifetime of the agent at the first time its
// health fell below each of the given percentages and returns the pieces
// in time order. A percentage never reached, or reached from the start,
// adds no cut.
func (a *Agent) PhasesByHealth(levels ...float64) []Interval {
	cuts := make([]time.Duration, 0, len(levels))
	for _, level := range levels {
		if t, ok := a.HealthBelow(level); ok {
			cuts = append(cuts, t)
		}
	}
	return a.Lifetime.Split(cuts...)
}

// PhasesByBuff returns the parts of the lifetime of the agent during which
// it did not carry the buff with the given id, in time order: the lifetime
// minus the union of the stacks of that buff. It is the whole lifetime
// when the buff never applied and nil when it never wore off.
func (a *Agent) PhasesByBuff(id uint32) []Interval {
	var out []Interval
	cursor := a.Lifetime.Start
	for s := range a.Stacks().OfBuff(id).Seq() {
		if s.Interval.Start > cursor {
			out = append(out, Interval{Start: cursor, End: s.Interval.Start})
		}
		cursor = max(cursor, s.Interval.End)
	}
	if cursor < a.Lifetime.End {
		out = append(out, Interval{Start: cursor, End: a.Lifetime.End})
	}
	return out
}

// CombatTime returns how long the agent was in combat within iv.
func (a *Agent) CombatTime(iv Interval) time.Duration {
	return a.InCombat.Total(iv, func(sp Span[bool]) bool { return sp.Value })
}

// AliveTime returns how long the agent was up within iv.
func (a *Agent) AliveTime(iv Interval) time.Duration { return a.lifeTime(iv, LifeAlive) }

// DownTime returns how long the agent was down within iv.
func (a *Agent) DownTime(iv Interval) time.Duration { return a.lifeTime(iv, LifeDown) }

// lifeTime returns how long the agent spent in state s within iv.
func (a *Agent) lifeTime(iv Interval, s LifeState) time.Duration {
	return a.Life.Total(iv, func(sp Span[LifeState]) bool { return sp.Value == s })
}

// DiedAt returns the time of the first death of the agent. The boolean is
// false when it never died.
func (a *Agent) DiedAt() (time.Duration, bool) {
	if len(a.Deaths) == 0 {
		return 0, false
	}
	return a.Deaths[0].Time, true
}

// TeamAt returns the team id of the agent at t, 0 when unknown.
func (a *Agent) TeamAt(t time.Duration) uint32 {
	v, _ := a.Team.ValueAt(t)
	return v
}

// WeaponSetAt returns the weapon set of the agent at t, 0 when unknown.
func (a *Agent) WeaponSetAt(t time.Duration) uint32 {
	v, _ := a.WeaponSet.ValueAt(t)
	return v
}

// StealthAt returns the raw stealth state of the agent at t, 0 when
// unknown.
func (a *Agent) StealthAt(t time.Duration) uint8 {
	v, _ := a.Stealth.ValueAt(t)
	return v
}

// TransformationAt returns the transformation skill of the agent at t,
// nil when it wore none or nothing is known.
func (a *Agent) TransformationAt(t time.Duration) *Skill {
	v, _ := a.Transformation.ValueAt(t)
	return v
}

// IsGlidingAt reports whether the glider of the agent was deployed at t.
func (a *Agent) IsGlidingAt(t time.Duration) bool {
	v, ok := a.Gliding.ValueAt(t)
	return ok && v
}

// GlidingTime returns how long the glider of the agent was deployed
// within iv.
func (a *Agent) GlidingTime(iv Interval) time.Duration {
	return a.Gliding.Total(iv, func(sp Span[bool]) bool { return sp.Value })
}

// IsAirborneAt reports whether the agent was between a jump and its
// landing at t.
func (a *Agent) IsAirborneAt(t time.Duration) bool {
	v, ok := a.Airborne.ValueAt(t)
	return ok && v
}

// AirborneTime returns how long the agent was between a jump and its
// landing within iv.
func (a *Agent) AirborneTime(iv Interval) time.Duration {
	return a.Airborne.Total(iv, func(s Span[bool]) bool { return s.Value })
}

// IsNameVisibleAt reports whether the name of the gadget or NPC was shown
// at t.
func (a *Agent) IsNameVisibleAt(t time.Duration) bool {
	v, ok := a.NameVisible.ValueAt(t)
	return ok && v
}

// NameVisibleTime returns how long the name of the gadget or NPC was
// shown within iv.
func (a *Agent) NameVisibleTime(iv Interval) time.Duration {
	return a.NameVisible.Total(iv, func(s Span[bool]) bool { return s.Value })
}
