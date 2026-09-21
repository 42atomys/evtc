package timeline

import "time"

// Agents is a query over agents in table order.
type Agents struct{ Query[Agent] }

// Where keeps the agents accepted by p.
func (q Agents) Where(p func(*Agent) bool) Agents { return Agents{q.Query.Where(p)} }

// Named keeps the agents with the given name.
func (q Agents) Named(name string) Agents {
	return q.Where(func(a *Agent) bool { return a.Name == name })
}

// OfKind keeps the agents of the kind.
func (q Agents) OfKind(k Kind) Agents {
	return q.Where(func(a *Agent) bool { return a.Kind == k })
}

// OfSpecies keeps the NPCs of the species id and the gadgets of the
// volatile id.
func (q Agents) OfSpecies(id uint16) Agents {
	return q.Where(func(a *Agent) bool { return a.SpeciesID == id && (a.IsNPC() || a.IsGadget()) })
}

// AliveAt keeps the agents that were up at t.
func (q Agents) AliveAt(t time.Duration) Agents {
	return q.Where(func(a *Agent) bool { return a.IsAliveAt(t) })
}

// Skip drops the first n agents of the traversal.
func (q Agents) Skip(n int) Agents {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n agents.
func (q Agents) Limit(n int) Agents {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the agents from the last in table order to the first.
func (q Agents) Reverse() Agents {
	q.Query = q.Query.Reverse()
	return q
}

// GroupBy partitions the matching agents by the key returned by f. Each
// group is in table order.
func (q Agents) GroupBy[K comparable](f func(*Agent) K) map[K]Agents {
	return groupInto(q.Query, f, func(items []*Agent) Agents { return Agents{From(items)} })
}

// Players is a query over players, in the table order of their first
// character.
type Players struct{ Query[Player] }

// Where keeps the players accepted by p.
func (q Players) Where(p func(*Player) bool) Players { return Players{q.Query.Where(p)} }

// InSubgroup keeps the players who were in squad subgroup n at some point.
func (q Players) InSubgroup(n int) Players {
	return q.Where(func(p *Player) bool { return everHeld(p.Subgroup, n) })
}

// OfProfession keeps the players who played the profession at some point.
func (q Players) OfProfession(prof Profession) Players {
	return q.Where(func(p *Player) bool { return everHeld(p.Profession, prof) })
}

// OfEliteSpec keeps the players who played the elite specialization at
// some point; EliteNone keeps those who played a core build.
func (q Players) OfEliteSpec(spec EliteSpec) Players {
	return q.Where(func(p *Player) bool { return everHeld(p.EliteSpec, spec) })
}

// everHeld reports whether one of the spans holds v.
func everHeld[T comparable](s Spans[T], v T) bool {
	for _, sp := range s.spans {
		if sp.Value == v {
			return true
		}
	}
	return false
}

// AliveAt keeps the players who were up at t.
func (q Players) AliveAt(t time.Duration) Players {
	return q.Where(func(p *Player) bool { return p.IsAliveAt(t) })
}

// Skip drops the first n players of the traversal.
func (q Players) Skip(n int) Players {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n players.
func (q Players) Limit(n int) Players {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the players from the last to the first.
func (q Players) Reverse() Players {
	q.Query = q.Query.Reverse()
	return q
}

// GroupBy partitions the matching players by the key returned by f. Each
// group is in the order of Players.
func (q Players) GroupBy[K comparable](f func(*Player) K) map[K]Players {
	return groupInto(q.Query, f, func(items []*Player) Players { return Players{From(items)} })
}

// Characters is a query over the characters of the players, in the order
// of Players and, for one player, in order of appearance.
type Characters struct{ Query[Character] }

// Where keeps the characters accepted by p.
func (q Characters) Where(p func(*Character) bool) Characters {
	return Characters{q.Query.Where(p)}
}

// OfProfession keeps the characters of the profession.
func (q Characters) OfProfession(prof Profession) Characters {
	return q.Where(func(c *Character) bool { return c.Profession == prof })
}

// AliveAt keeps the characters that were up at t.
func (q Characters) AliveAt(t time.Duration) Characters {
	return q.Where(func(c *Character) bool { return c.IsAliveAt(t) })
}

// Skip drops the first n characters of the traversal.
func (q Characters) Skip(n int) Characters {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n characters.
func (q Characters) Limit(n int) Characters {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the characters from the last to the first.
func (q Characters) Reverse() Characters {
	q.Query = q.Query.Reverse()
	return q
}

// GroupBy partitions the matching characters by the key returned by f.
// Each group is in the order of Characters.
func (q Characters) GroupBy[K comparable](f func(*Character) K) map[K]Characters {
	return groupInto(q.Query, f, func(items []*Character) Characters { return Characters{From(items)} })
}

// Targets is a query over targets, the boss first, then in order of first
// exchange with the players.
type Targets struct{ Query[Target] }

// Where keeps the targets accepted by p.
func (q Targets) Where(p func(*Target) bool) Targets { return Targets{q.Query.Where(p)} }

// OfSpecies keeps the NPCs of the species id and the gadgets of the
// volatile id.
func (q Targets) OfSpecies(id uint16) Targets {
	return q.Where(func(t *Target) bool { return t.SpeciesID == id })
}

// AliveAt keeps the targets that were up at t.
func (q Targets) AliveAt(t time.Duration) Targets {
	return q.Where(func(tg *Target) bool { return tg.IsAliveAt(t) })
}

// Skip drops the first n targets of the traversal.
func (q Targets) Skip(n int) Targets {
	q.Query = q.Query.Skip(n)
	return q
}

// Limit stops the traversal after n targets.
func (q Targets) Limit(n int) Targets {
	q.Query = q.Query.Limit(n)
	return q
}

// Reverse traverses the targets from the last engaged to the first.
func (q Targets) Reverse() Targets {
	q.Query = q.Query.Reverse()
	return q
}

// GroupBy partitions the matching targets by the key returned by f. Each
// group is in the order of Targets.
func (q Targets) GroupBy[K comparable](f func(*Target) K) map[K]Targets {
	return groupInto(q.Query, f, func(items []*Target) Targets { return Targets{From(items)} })
}
