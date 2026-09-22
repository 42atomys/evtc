package timeline

import (
	"time"

	"github.com/42atomys/evtc"
)

// Player is a member of the squad, one per account. What happened on the
// field belongs to the characters the player brought: most players bring
// one, a player who swapped during the log has several. The queries of a
// player cover all of them.
//
// The exported fields are read-only after Build.
type Player struct {
	// Timeline owns the player.
	Timeline *Timeline
	// Account is the account name without its leading colon.
	Account string
	// Guild is the guild of the player, zero when unknown.
	Guild GUID
	// Subgroup is the squad subgroup over time, 0 when unknown.
	Subgroup Spans[int]
	// Profession is the profession over time. It changes when the player
	// swaps to another character.
	Profession Spans[Profession]
	// EliteSpec is the elite specialization over time, EliteNone for a
	// core build.
	EliteSpec Spans[EliteSpec]

	characters []*Character
	main       *Character
	// merged holds the nodes of every character in time order. It is only
	// filled for a player with several characters.
	merged *playerNodes
}

// playerNodes are the nodes of the characters of a player, merged.
type playerNodes struct {
	hits          []*Hit
	hitsTaken     []*Hit
	hitsCredited  []*Hit
	casts         []*Cast
	effects       []*Effect
	missiles      []*Missile
	stacks        []*BuffStack
	stacksApplied []*BuffStack
	events        []*evtc.Event
}

// Character is a character a player brought on the field, with the agent
// that stands for it. Every Agent method and field is available through
// the embedded Agent.
type Character struct {
	*Agent
	// Profession is the profession of the character.
	Profession Profession

	// spec is the elite specialization the character ended on.
	spec EliteSpec
}

// Ref returns the agent of the character, or nil for a nil character.
func (c *Character) Ref() *Agent {
	if c == nil {
		return nil
	}
	return c.Agent
}

// EliteSpecAt returns the elite specialization of the player at t, which
// is that of the character during its lifetime.
func (c *Character) EliteSpecAt(t time.Duration) EliteSpec { return c.Player.EliteSpecAt(t) }

// Spec returns the name of the elite specialization the character ended
// on, or the name of its profession for a core build or an unknown
// specialization.
func (c *Character) Spec() string {
	if info := c.spec.info(); info.name != "" {
		return info.name
	}
	return c.Profession.String()
}

// Characters returns the characters of the player, in order of appearance.
// A character brought back after another one counts again.
func (p *Player) Characters() []*Character { return p.characters }

// Main returns the character with the longest lifetime, the first of
// them on a tie.
func (p *Player) Main() *Character { return p.main }

// CharacterAt returns the character the player had on the field at t: the
// last one that appeared at or before t, the first one when t precedes
// them all.
func (p *Player) CharacterAt(t time.Duration) *Character {
	at := p.characters[0]
	for _, c := range p.characters[1:] {
		if c.Lifetime.Start > t {
			break
		}
		at = c
	}
	return at
}

// Ref returns the agent of the main character, or nil for a nil player.
// Query filters given a player match every character.
func (p *Player) Ref() *Agent {
	if p == nil {
		return nil
	}
	return p.main.Agent
}

// agents returns the agents of every character.
func (p *Player) agents() []*Agent {
	out := make([]*Agent, len(p.characters))
	for i, c := range p.characters {
		out[i] = c.Agent
	}
	return out
}

// String returns the account name.
func (p *Player) String() string {
	if p == nil {
		return "<nil>"
	}
	return p.Account
}

// SubgroupAt returns the squad subgroup of the player at t, 0 when unknown.
func (p *Player) SubgroupAt(t time.Duration) int {
	v, _ := p.Subgroup.ValueAt(t)
	return v
}

// ProfessionAt returns the profession of the player at t.
func (p *Player) ProfessionAt(t time.Duration) Profession {
	v, _ := p.Profession.ValueAt(t)
	return v
}

// EliteSpecAt returns the elite specialization of the player at t.
func (p *Player) EliteSpecAt(t time.Duration) EliteSpec {
	v, _ := p.EliteSpec.ValueAt(t)
	return v
}

// SpecAt returns the name of the elite specialization of the player at t,
// or the name of the profession for a core build or an unknown
// specialization.
func (p *Player) SpecAt(t time.Duration) string {
	if info := p.EliteSpecAt(t).info(); info.name != "" {
		return info.name
	}
	return p.ProfessionAt(t).String()
}

// Hits returns the hits dealt by the characters themselves, in time order.
// The hits of their minions are in HitsCredited.
func (p *Player) Hits() Hits {
	if p.merged != nil {
		return Hits{From(p.merged.hits)}
	}
	return p.main.Hits()
}

// HitsCredited returns the hits dealt by the characters and by their
// minions, in time order: what a damage meter credits to the player.
func (p *Player) HitsCredited() Hits {
	if p.merged != nil {
		return Hits{From(p.merged.hitsCredited)}
	}
	return p.main.HitsCredited()
}

// HitsTaken returns the hits received by the characters, in time order.
func (p *Player) HitsTaken() Hits {
	if p.merged != nil {
		return Hits{From(p.merged.hitsTaken)}
	}
	return p.main.HitsTaken()
}

// Casts returns the casts of the characters, in start order.
func (p *Player) Casts() Casts {
	if p.merged != nil {
		return Casts{From(p.merged.casts)}
	}
	return p.main.Casts()
}

// Effects returns the effects played around the characters or placed by
// them, in creation order.
func (p *Player) Effects() Effects {
	if p.merged != nil {
		return Effects{From(p.merged.effects)}
	}
	return p.main.Effects()
}

// Missiles returns the missiles owned by the characters, in creation order.
func (p *Player) Missiles() Missiles {
	if p.merged != nil {
		return Missiles{From(p.merged.missiles)}
	}
	return p.main.Missiles()
}

// Stacks returns the buff stacks received by the characters, in apply
// order.
func (p *Player) Stacks() Stacks {
	if p.merged != nil {
		return Stacks{From(p.merged.stacks)}
	}
	return p.main.Stacks()
}

// StacksApplied returns the buff stacks applied by the characters on
// anyone.
func (p *Player) StacksApplied() Stacks {
	if p.merged != nil {
		return Stacks{From(p.merged.stacksApplied)}
	}
	return p.main.StacksApplied()
}

// Events returns the raw log events where a character is the source or
// the destination, in time order.
func (p *Player) Events() Events {
	if p.merged != nil {
		return Events{Query: From(p.merged.events), tl: p.Timeline}
	}
	return p.main.Events()
}

// LifeStateAt returns the life state at t of the character on the field.
func (p *Player) LifeStateAt(t time.Duration) LifeState { return p.CharacterAt(t).LifeStateAt(t) }

// IsAliveAt reports whether the player was up at t.
func (p *Player) IsAliveAt(t time.Duration) bool { return p.LifeStateAt(t) == LifeAlive }

// IsDownAt reports whether the player was downed at t.
func (p *Player) IsDownAt(t time.Duration) bool { return p.LifeStateAt(t) == LifeDown }

// IsDeadAt reports whether the player was dead at t.
func (p *Player) IsDeadAt(t time.Duration) bool { return p.LifeStateAt(t) == LifeDead }

// PositionAt returns the position at t of the character on the field, see
// Agent.PositionAt.
func (p *Player) PositionAt(t time.Duration) Vec3 { return p.CharacterAt(t).PositionAt(t) }

// HealthAt returns the health percentage at t of the character on the
// field, or 0 when it is unknown.
func (p *Player) HealthAt(t time.Duration) float64 { return p.CharacterAt(t).HealthAt(t) }

// IsCommanderAt reports whether the player wore a commander tag at t.
func (p *Player) IsCommanderAt(t time.Duration) bool { return p.CharacterAt(t).IsCommanderAt(t) }

// merge fills the merged nodes of a player with several characters and
// picks the main character.
func (p *Player) merge() {
	p.main = p.characters[0]
	for _, c := range p.characters[1:] {
		if c.Lifetime.Duration() > p.main.Lifetime.Duration() {
			p.main = c
		}
	}
	if len(p.characters) == 1 {
		return
	}
	m := &playerNodes{}
	for _, c := range p.characters {
		m.hits = append(m.hits, c.hits...)
		m.hitsTaken = append(m.hitsTaken, c.hitsTaken...)
		m.hitsCredited = append(m.hitsCredited, c.HitsCredited().items...)
		m.casts = append(m.casts, c.casts...)
		m.effects = append(m.effects, c.effects...)
		m.missiles = append(m.missiles, c.missiles...)
		m.stacks = append(m.stacks, c.stacks...)
		m.stacksApplied = append(m.stacksApplied, c.stacksApplied...)
		m.events = append(m.events, c.events...)
	}
	sortedByTime(m.hits, hitTime)
	sortedByTime(m.hitsTaken, hitTime)
	sortedByTime(m.hitsCredited, hitTime)
	sortedByTime(m.casts, castStart)
	sortedByTime(m.effects, effectStart)
	sortedByTime(m.missiles, missileStart)
	sortedByTime(m.stacks, stackStart)
	sortedByTime(m.stacksApplied, stackStart)
	sortedByTime(m.events, p.Timeline.TimeOf)
	p.merged = m
}
