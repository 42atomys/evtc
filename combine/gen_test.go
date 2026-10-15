package combine

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/42atomys/evtc"
)

// genOptions parametrizes genFight.
type genOptions struct {
	// recorders is the number of players who record, players the size of
	// the squad.
	recorders, players int
	adds               int
	duration           time.Duration
	seed               uint64
}

// genTruth is what happened in a generated fight, as far as one client at
// least recorded it.
type genTruth struct {
	strikes, stacks, deaths int
	// creatures is the number of creatures a client at least held, and
	// apart the number of times one was held again after every client
	// lost it: nothing tells it is the same one.
	creatures, apart int
	// held is the time the clients held creatures, as far as one at least
	// did, in milliseconds.
	held uint64
}

// view is how far a client sees on each axis.
const view = 5000

// genStack is an open stack of a generated fight.
type genStack struct {
	buff  uint32
	until uint64
}

// genAgent is an agent of a generated fight.
type genAgent struct {
	id, species uint16
	low         uint64
	name        string
	player      bool
	x, y        float64
	born, died  uint64
	health      float64
	stacks      map[uint32]*genStack
	// holders is the number of clients that hold the creature, met tells
	// that one held it already.
	holders int
	met     bool
}

// open returns the numbers of the open stacks of the agent, in order.
func (a *genAgent) open() []uint32 {
	ids := make([]uint32, 0, len(a.stacks))
	for id := range a.stacks {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// genClient is a client that records a generated fight.
type genClient struct {
	*recorder
	player *genAgent
	// held holds the creatures the client holds and told the health it
	// wrote of each.
	held map[*genAgent]bool
	told map[*genAgent]int
	// entry holds the address of each creature: a client that finds
	// again a creature it lost lists it anew.
	entry   map[*genAgent]uint64
	entries uint64
	// lag is how late the client dates what it learns, an odd number of
	// milliseconds.
	lag uint64
}

// sees reports whether a creature is within the view of the client.
func (c *genClient) sees(a *genAgent) bool {
	return math.Abs(a.x-c.player.x) <= view && math.Abs(a.y-c.player.y) <= view
}

// knows reports whether the client is told of an agent: a squad member,
// or a creature it holds.
func (c *genClient) knows(a *genAgent) bool { return a.player || c.held[a] }

// addr returns the address the client gives an agent.
func (c *genClient) addr(a *genAgent) uint64 {
	if a.player {
		return c.base | a.low
	}
	return c.entry[a]
}

// write writes an event of the agents given, under the addresses of the
// client.
func (c *genClient) write(t uint64, kind evtc.StateChange, src, dst *genAgent, e evtc.Event) {
	e.Time, e.IsStateChange = c.at(t), kind
	if src != nil {
		e.SrcAgent, e.SrcInstanceID = c.addr(src), src.id
	}
	if dst != nil {
		e.DstAgent, e.DstInstanceID = c.addr(dst), dst.id
	}
	c.l.Events = append(c.l.Events, e)
}

// place writes where the client sees an agent.
func (c *genClient) place(t uint64, a *genAgent) {
	c.write(t, evtc.StatePosition, a, nil, evtc.Event{DstAgent: uint64(math.Float32bits(float32(a.x))) | uint64(math.Float32bits(float32(a.y)))<<32})
}

// genFight generates a deterministic fight spread over more than a client
// sees, and the log every recording player writes of it: a boss, creatures
// that come and die and whose instance ids serve again, players that move,
// strike and give buffs. A client holds the creatures within its view,
// writes what the server tells it of them some milliseconds late, and
// samples positions and health on its own. Times are even for what the
// server tells and odd for the rest, as arcdps 20260920 writes them.
func genFight(o genOptions) ([]*evtc.Log, genTruth) {
	rng := rand.New(rand.NewPCG(o.seed, o.seed^0x9e3779b97f4a7c15))
	var truth genTruth
	end := uint64(o.duration / time.Millisecond)

	players := make([]*genAgent, o.players)
	for i := range players {
		players[i] = &genAgent{id: uint16(100 + i), low: uint64(0x1000 + i), name: fmt.Sprintf("P%d", i), player: true, stacks: map[uint32]*genStack{}}
	}
	creatures := []*genAgent{{id: 500, species: 15375, low: 0x2000, name: "Boss", x: 1000, died: end, health: 100, stacks: map[uint32]*genStack{}}}
	width := 3500 * (o.players - 1)
	for i := range o.adds {
		born := uint64(rng.IntN(int(end)*4/5)) / 20 * 20
		creatures = append(creatures, &genAgent{
			// Five instance ids serve for every creature.
			id: uint16(600 + i%5), species: uint16(1000 + i%3), low: uint64(0x2001 + i), name: fmt.Sprintf("Add%d", i),
			x: float64(rng.IntN(width+4000) - 2000), y: float64(rng.IntN(3000) - 1500),
			born: born, died: born + uint64(4000+rng.IntN(8000))/20*20, health: 100, stacks: map[uint32]*genStack{},
		})
	}
	// A creature takes an instance id a second after the one before it
	// died.
	slices.SortStableFunc(creatures[1:], func(a, b *genAgent) int { return int(a.born) - int(b.born) })
	free := map[uint16]uint64{}
	for _, c := range creatures[1:] {
		if at, ok := free[c.id]; ok && c.born < at+1000 {
			c.died += at + 1000 - c.born
			c.born = at + 1000
		}
		c.died = min(c.died, end)
		free[c.id] = c.died
	}
	creatures = slices.DeleteFunc(creatures, func(c *genAgent) bool { return c.born >= c.died })
	everyone := append(slices.Clone(players), creatures...)

	clients := make([]*genClient, o.recorders)
	for i := range clients {
		r := &recorder{
			l:     &evtc.Log{Header: evtc.Header{Build: "20260920", Revision: 1, TargetSpeciesID: 15375}},
			base:  uint64(i+1) << 32,
			shift: uint64(i) * 7_654_322,
		}
		r.l.Events = append(r.l.Events, evtc.Event{Time: r.at(0), SrcAgent: 0x637261, Value: 1787685623, BuffDamage: 1787685624, IsStateChange: evtc.StateSquadCombatStart})
		for j, p := range players {
			r.l.Agents = append(r.l.Agents, evtc.Agent{Addr: r.base | p.low, Profession: uint32(1 + j%9), Name: p.name, Account: fmt.Sprintf(":Acc%d.%04d", j, j), Subgroup: "1", HitboxWidth: 24})
		}
		r.l.Skills = []evtc.Skill{{ID: skillHeat, Name: "Heat"}, {ID: 737, Name: "Burning"}, {ID: skillMight, Name: "Might"}}
		c := &genClient{recorder: r, player: players[i], held: map[*genAgent]bool{}, told: map[*genAgent]int{}, entry: map[*genAgent]uint64{}, lag: uint64(1 + 2*rng.IntN(15))}
		c.write(0, evtc.StatePointOfView, players[i], nil, evtc.Event{})
		for _, p := range players {
			c.write(0, evtc.StateEnterCombat, p, nil, evtc.Event{})
		}
		clients[i] = c
	}

	// tell writes a message of the server to the clients that are told of
	// its agents, and reports whether one at least was.
	tell := func(t uint64, kind evtc.StateChange, src, dst *genAgent, e evtc.Event) bool {
		seen := false
		for _, c := range clients {
			if (src == nil || c.knows(src)) && (dst == nil || c.knows(dst)) {
				c.write(t+c.lag-1, kind, src, dst, e)
				seen = true
			}
		}
		return seen
	}
	// The server numbers the stacks: a number serves again once its stack
	// ended.
	numbers := map[uint32]bool{}
	apply := func(t uint64, src, dst *genAgent, buff uint32, duration uint64) {
		id := uint32(1)
		for numbers[id] {
			id++
		}
		numbers[id] = true
		dst.stacks[id] = &genStack{buff: buff, until: t + duration}
		if tell(t, evtc.StateBuffApply, src, dst, withID(evtc.Event{SkillID: buff, Value: int32(duration), IsShields: 1}, id)) {
			truth.stacks++
		}
	}

	for t := uint64(0); t < end; t += 20 {
		for i, p := range players {
			p.x = float64(3500*i) + 1500*math.Sin(float64(t)/4000+float64(i))
		}
		for _, a := range creatures {
			if a.died == t && t < end {
				if tell(t, evtc.StateChangeDead, a, nil, evtc.Event{}) {
					truth.deaths++
				}
			}
			if a.holders > 0 {
				truth.held += 20
			}
		}
		for _, a := range everyone {
			alive := a.player || t < a.died
			for _, id := range a.open() {
				if st := a.stacks[id]; st.until <= t || !alive {
					delete(a.stacks, id)
					delete(numbers, id)
					if alive {
						tell(t, evtc.StateBuffRemoveSingle, a, nil, withID(evtc.Event{SkillID: st.buff, IsBuffRemove: evtc.BuffRemoveSingle}, id))
					}
				}
			}
		}
		for ci, c := range clients {
			for _, a := range creatures {
				there := a.born <= t && t < a.died && c.sees(a)
				switch {
				case there && !c.held[a]:
					c.entries++
					c.entry[a] = c.base | 0x2000 + c.entries
					c.l.Agents = append(c.l.Agents, evtc.Agent{Addr: c.entry[a], Profession: uint32(a.species), IsElite: 0xffffffff, Name: a.name})
					c.held[a] = true
					if a.holders++; a.holders == 1 {
						if a.met {
							truth.apart++
						} else {
							truth.creatures++
						}
					}
					a.met = true
					if t > 0 {
						c.write(t+c.lag, evtc.StateSpawn, a, nil, evtc.Event{})
					}
					// What the agent is, told on gain.
					c.place(t+c.lag, a)
					c.told[a] = int(a.health)
					c.write(t+c.lag+2, evtc.StateHealthPctUpdate, a, nil, evtc.Event{DstAgent: uint64(int(a.health)) * 100})
					for _, id := range a.open() {
						st := a.stacks[id]
						c.write(t+c.lag, evtc.StateBuffApply, nil, a, withID(evtc.Event{SkillID: st.buff, Value: int32(st.until - t), IsShields: 1}, id))
						// A stack ends for a log when the last client that
						// held its wearer loses it: told then, it is a
						// stack anew.
						if a.holders == 1 {
							truth.stacks++
						}
					}
				case !there && c.held[a]:
					delete(c.held, a)
					a.holders--
					c.write(t+c.lag, evtc.StateDespawn, a, nil, evtc.Event{})
					c.write(t+c.lag, evtc.StateTeamChange, a, nil, evtc.Event{})
				}
			}
			// The samples of the client, on its own beat.
			if (t+uint64(ci)*40)%300 == 0 {
				for _, p := range players {
					c.place(t+1, p)
				}
			}
			if t%200 == 0 {
				for _, a := range creatures {
					if c.held[a] && c.told[a] != int(a.health) {
						c.told[a] = int(a.health)
						c.write(t+c.lag, evtc.StateHealthPctUpdate, a, nil, evtc.Event{DstAgent: uint64(int(a.health)) * 100})
					}
				}
			}
		}
		if t%500 == 0 && t > 0 {
			for i, p := range players {
				// The nearest creature within reach.
				var target *genAgent
				reach := 2500.0
				for _, a := range creatures {
					if d := math.Hypot(a.x-p.x, a.y-p.y); a.born <= t && t+400 < a.died && d <= reach {
						target, reach = a, d
					}
				}
				if target == nil {
					continue
				}
				damage := int32(500 + rng.IntN(4000))
				target.health = max(target.health-float64(damage)/2000, 1)
				if tell(t+uint64(i)*20, evtc.StateCombat, p, target, evtc.Event{SkillID: skillHeat, Value: damage, IFF: evtc.IFFFoe}) {
					truth.strikes++
				}
				if t%1000 == 0 {
					apply(t+uint64(i)*20, p, target, 737, 3000)
				}
			}
		}
		if t%2000 == 0 && t > 0 {
			for _, p := range players {
				apply(t, p, p, skillMight, 5000)
			}
		}
	}

	logs := make([]*evtc.Log, len(clients))
	for i, c := range clients {
		for _, a := range everyone {
			if a.player || c.held[a] {
				c.write(end-1, evtc.StateTeamChange, a, nil, evtc.Event{})
			}
		}
		c.l.Events = append(c.l.Events, evtc.Event{Time: c.at(end), SrcAgent: 0x637261, Value: 1787685623 + int32(end/1000), IsStateChange: evtc.StateSquadCombatEnd})
		logs[i] = c.l
	}
	return logs, truth
}
