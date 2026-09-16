package timeline

import "time"

// Players is a query over players in table order.
type Players struct{ Query[Player] }

// Where keeps the players accepted by p.
func (q Players) Where(p func(*Player) bool) Players { return Players{q.Query.Where(p)} }

// InSubgroup keeps the players of squad subgroup n.
func (q Players) InSubgroup(n int) Players {
	return q.Where(func(p *Player) bool { return p.Subgroup == n })
}

// OfProfession keeps the players of the profession.
func (q Players) OfProfession(prof Profession) Players {
	return q.Where(func(p *Player) bool { return p.Profession == prof })
}

// OfEliteSpec keeps the players of the elite specialization; EliteNone
// keeps the core builds.
func (q Players) OfEliteSpec(spec EliteSpec) Players {
	return q.Where(func(p *Player) bool { return p.EliteSpec == spec })
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

// Reverse traverses the players from the last in table order to the first.
func (q Players) Reverse() Players {
	q.Query = q.Query.Reverse()
	return q
}

// GroupBy partitions the matching players by the key returned by f. Each
// group is in table order.
func (q Players) GroupBy[K comparable](f func(*Player) K) map[K]Players {
	return groupInto(q.Query, f, func(items []*Player) Players { return Players{From(items)} })
}
