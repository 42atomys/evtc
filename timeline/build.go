package timeline

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/42atomys/evtc"
)

// The builder runs two passes over the timed events of the log. The scan
// pass resolves agents, measures lifetimes and counts every node and edge
// so that the fill pass can allocate each arena once and never grow a
// slice. Pointers into the arenas are therefore stable and the graph is
// built with one allocation per node type.

// CauseWindow is how far a downing or killing hit may be from the state
// change it explains.
const CauseWindow = 150 * time.Millisecond

// MoveGap is the largest gap between two movement samples across which
// positions are interpolated.
const MoveGap = 1500 * time.Millisecond

// agentCounts tallies the nodes and edges of one agent during the scan.
// The builder keeps one per agent, indexed by Agent.idx.
type agentCounts struct {
	seen        bool
	first, last uint64
	// fought is set when the agent exchanged hits with a player, at
	// foughtAt.
	fought   bool
	foughtAt uint64

	events, hits, hitsTaken, casts, stacks, stacksApplied int
	pos, vel, facing, health, barrier, maxHealth, defPct  int
	states, defiance, combat, targetable                  int
	downs, deaths, minions, breakbars                     int
	markers, stunbreaks, team, swaps, stealth, gliding    int
	transform, effects, missiles                          int
	airborne, nameVisible, gadgetAnims                    int
	// openIDs are the trackable ids of the stacks present on the agent
	// during the scan, openMarkers the ids of the markers it wears.
	openIDs     []uint32
	openMarkers []uint32
}

type castKey struct {
	caster *Agent
	skill  uint32
}

type stackKey struct {
	receiver *Agent
	buff     uint32
}

type builder struct {
	tl  *Timeline
	log *evtc.Log

	counts     []agentCounts
	srcs, dsts []*Agent
	// shared holds the addresses that several agent table entries carry.
	shared map[uint64]*sharedAddr
	// builds are the subgroup, profession and specialization a player
	// announced when entering combat, in time order.
	builds map[*Player][]buildChange
	bosses []*Agent

	nHits, nCasts, nStacks, nDowns, nDeaths int
	nEffects, nMissiles, nTicks, nGround    int
	nRewards, nMapChanges                   int
	// extRegs are the extension registration events in time order and
	// extCombat counts the extension combat events by signature.
	extRegs   []*evtc.Event
	extCombat map[uint32]int
	extBySig  map[uint32]*Extension
	// stackActive and stackChanges count the activity and duration change
	// events of each stack, by stack ordinal; missileLaunches and
	// missileEffects do the same for missiles.
	stackActive     []int
	stackChanges    []int
	changeArena     []*evtc.Event
	missileLaunches []int
	missileEffects  []int
	// castHits counts the hits attributed to each cast, by cast index.
	castHits    []int
	castHitRefs []*Hit

	hitArena   []Hit
	castArena  []Cast
	stackArena []BuffStack
	downArena  []Down
	deathArena []Death
	buffArena  []Buff
	// Fill cursors into the arenas.
	hitIdx, castIdx, stackIdx, downIdx, deathIdx int

	markerArena        []Marker
	stunArena          []StunBreak
	effectArena        []Effect
	missileArena       []Missile
	launchArena        []Launch
	missileEffectArena []MissileEffect
	groundArena        []GroundMarker
	activeArena        []Span[bool]
	gadgetAnimArena    []GadgetAnimation
	rewardArena        []Reward
	mapChangeArena     []MapChange
	extensionArena     []Extension
	// Fill cursors into the arenas above.
	markerIdx, stunIdx, effectIdx, missileIdx, groundIdx int
	gadgetAnimIdx, rewardIdx, mapChangeIdx               int

	lastCast     map[castKey]*Cast
	openStacks   map[uint32]*BuffStack
	openByKey    map[stackKey][]*BuffStack
	openByAgent  [][]*BuffStack
	openDown     map[*Agent]*Down
	openEffects  map[uint32]*Effect
	openMissiles map[uint32]*Missile
	// openGround is the placement on the ground of each squad marker
	// index, allocated by the first placement.
	openGround map[uint32]*GroundMarker
}

func (b *builder) run() *Timeline {
	b.makeAgents()
	b.collectEvents()
	b.makeSkills()
	b.scan()
	b.dropUnclaimed()
	b.allocateAgents()
	b.allocateSkills()
	b.fill()
	b.finish()
	b.decodeExtensions()
	return b.tl
}

// decodeExtensions hands every extension that owns events to the decoder
// registered for its signature, once the core graph is complete.
func (b *builder) decodeExtensions() {
	tl := b.tl
	for _, x := range tl.Extensions {
		if tl.extensions[x.Signature] != x {
			continue
		}
		if d := extensionDecoder(x.Signature); d != nil {
			x.Decoded = d.Decode(x)
		}
	}
}

// cnt returns the scan counts of an agent.
func (b *builder) cnt(a *Agent) *agentCounts { return &b.counts[a.idx] }

// allAgents returns the agents plus the Unknown sentinel.
func (b *builder) allAgents() []*Agent {
	return append(slices.Clone(b.tl.agents), b.tl.Unknown)
}

// makeAgents creates one Agent per agent table entry plus the Unknown
// sentinel.
func (b *builder) makeAgents() {
	tl, l := b.tl, b.log
	arena := make([]Agent, len(l.Agents))
	nPlayers := 0
	for i := range l.Agents {
		if l.Agents[i].IsElite != 0xffffffff {
			nPlayers++
		}
	}
	players := make([]Player, 0, nPlayers)
	characters := make([]Character, 0, nPlayers)
	byAccount := make(map[string]*Player, nPlayers)
	tl.agents = make([]*Agent, len(l.Agents))
	tl.byAddr = make(map[uint64]*Agent, len(l.Agents)+1)
	tl.alias = map[uint64]uint64{}
	b.counts = make([]agentCounts, len(l.Agents)+1)
	for i := range l.Agents {
		raw := &l.Agents[i]
		a := &arena[i]
		*a = Agent{
			Timeline:      tl,
			Raw:           raw,
			Addr:          raw.Addr,
			Name:          raw.Name,
			Toughness:     raw.Toughness,
			Concentration: raw.Concentration,
			Healing:       raw.Healing,
			Condition:     raw.Condition,
			HitboxWidth:   raw.HitboxWidth,
			HitboxHeight:  raw.HitboxHeight,
			idx:           i,
		}
		switch {
		case raw.IsElite != 0xffffffff:
			a.Kind = KindPlayer
			account := strings.TrimPrefix(raw.Account, ":")
			p := byAccount[account]
			if p == nil {
				players = append(players, Player{Timeline: tl, Account: account})
				p = &players[len(players)-1]
				byAccount[account] = p
				tl.players = append(tl.players, p)
			}
			characters = append(characters, Character{Agent: a, Profession: Profession(raw.Profession)})
			a.Player, a.Character = p, &characters[len(characters)-1]
			p.characters = append(p.characters, a.Character)
		case raw.Profession>>16 == 0xffff:
			a.Kind = KindGadget
			a.SpeciesID = uint16(raw.Profession)
			tl.gadgets = append(tl.gadgets, a)
		default:
			a.Kind = KindNPC
			a.SpeciesID = uint16(raw.Profession)
			tl.npcs = append(tl.npcs, a)
		}
		tl.agents[i] = a
		first, dup := tl.byAddr[a.Addr]
		if !dup {
			tl.byAddr[a.Addr] = a
			continue
		}
		s := b.shared[a.Addr]
		if s == nil {
			s = &sharedAddr{entries: []*Agent{first}, owner: map[uint16]*Agent{}, cur: first}
			if b.shared == nil {
				b.shared = map[uint64]*sharedAddr{}
			}
			b.shared[a.Addr] = s
		}
		s.entries = append(s.entries, a)
	}
	tl.Unknown = &Agent{Timeline: tl, Kind: KindUnknown, Name: "Unknown", idx: len(l.Agents)}
}

// collectEvents gathers the timed events in time order and locates the
// origin and the end of the timeline.
func (b *builder) collectEvents() {
	tl, l := b.tl, b.log
	tl.events = make([]*evtc.Event, 0, len(l.Events))
	for i := range l.Events {
		e := &l.Events[i]
		if !hasTime(e.IsStateChange) {
			continue
		}
		tl.events = append(tl.events, e)
		if e.IsStateChange == evtc.StateIIDChange {
			b.aliasAddr(e.SrcAgent, e.DstAgent)
		}
	}
	slices.SortStableFunc(tl.events, func(a, b *evtc.Event) int { return cmp.Compare(a.Time, b.Time) })

	var start *evtc.Event
	for _, e := range tl.events {
		if e.IsStateChange == evtc.StateSquadCombatStart {
			start = e
			break
		}
	}
	switch {
	case start != nil:
		tl.epoch = start.Time
		tl.Start = time.Unix(int64(uint32(start.Value)), 0).UTC()
		tl.LocalStart = time.Unix(int64(uint32(start.BuffDamage)), 0).UTC()
	case len(tl.events) > 0:
		tl.epoch = tl.events[0].Time
	}

	// Events further than maxSpan from the origin are corrupt: they would
	// overflow the Duration arithmetic, so they stay out of the graph.
	kept := tl.events[:0]
	for _, e := range tl.events {
		if distance(e.Time, tl.epoch) <= uint64(maxSpan/time.Millisecond) {
			kept = append(kept, e)
		}
	}
	tl.events = kept

	var end *evtc.Event
	for _, e := range tl.events {
		switch e.IsStateChange {
		case evtc.StateSquadCombatEnd:
			end = e
		case evtc.StateMapID:
			tl.MapID = uint32(e.SrcAgent)
		}
	}
	// The log lasts until its last timed event, even when the squad combat
	// end event is not the last one.
	if len(tl.events) > 0 {
		tl.Duration = tl.TimeOf(tl.events[len(tl.events)-1])
	}
	if end != nil {
		tl.Duration = max(tl.Duration, tl.TimeOf(end))
	}
}

// maxSpan is how far from its origin a log may extend. Times beyond it
// cannot come from arcdps and would overflow time.Duration.
const maxSpan = 366 * 24 * time.Hour

// distance returns |a - b| for raw millisecond times.
func distance(a, b uint64) uint64 {
	if a >= b {
		return a - b
	}
	return b - a
}

// aliasAddr records an address change so that both addresses resolve to
// the agent table entry.
func (b *builder) aliasAddr(old, updated uint64) {
	switch {
	case old == updated:
	case b.tl.byAddr[updated] != nil:
		b.tl.alias[old] = updated
	case b.tl.byAddr[old] != nil:
		b.tl.alias[updated] = old
	default:
		b.tl.alias[old] = updated
	}
}

// sharedAddr is an address that several agent table entries carry. A
// player who leaves and comes back keeps their address but gets a new
// instance id and a new table entry. The table lists the entries in order
// of appearance, so each new instance id claims the next one. An entry
// that names the character already on the field is folded into it: the
// character keeps one agent across its stays.
type sharedAddr struct {
	entries []*Agent
	owner   map[uint16]*Agent
	// next is the first entry no instance id claimed yet.
	next int
	// cur is the agent of the last instance id to appear. Events written
	// without an instance id go to it.
	cur *Agent
	// folded are the entries folded into another one, with the raw time
	// they came back at.
	folded []foldedEntry
}

// foldedEntry is a table entry that named the character already on the
// field.
type foldedEntry struct {
	entry, into *Agent
	at          uint64
}

// pick returns the agent an event with the given instance id belongs to.
// Once every entry is claimed, further instance ids go to the last agent.
func (s *sharedAddr) pick(inst uint16, at uint64) *Agent {
	if inst == 0 {
		return s.cur
	}
	a := s.owner[inst]
	if a == nil {
		a = s.cur
		if s.next < len(s.entries) {
			entry := s.entries[s.next]
			s.next++
			if s.next > 1 && sameCharacter(entry, s.cur) {
				s.folded = append(s.folded, foldedEntry{entry, s.cur, at})
			} else {
				a = entry
			}
		}
		s.owner[inst] = a
		s.cur = a
	}
	return a
}

// sameCharacter reports whether two table entries name the same character.
func sameCharacter(a, b *Agent) bool {
	return a.Name == b.Name && a.Raw.Profession == b.Raw.Profession && a.Player == b.Player
}

// buildChange is the subgroup, profession and specialization of a player
// from an instant on.
type buildChange struct {
	at       time.Duration
	subgroup int
	prof     Profession
	spec     EliteSpec
	event    *evtc.Event
}

// tableBuild reads a build from an agent table entry.
func tableBuild(a *Agent, at time.Duration) buildChange {
	sub, _ := strconv.Atoi(a.Raw.Subgroup)
	return buildChange{at: at, subgroup: sub, prof: Profession(a.Raw.Profession), spec: EliteSpec(a.Raw.IsElite)}
}

// finishPlayers builds the subgroup, profession and specialization spans
// of the players and merges the nodes of their characters.
func (b *builder) finishPlayers() {
	tl := b.tl
	// A stay is a presence on the field: one per character, and one more
	// each time a character came back.
	type stay struct {
		buildChange
		c *Character
	}
	for _, p := range tl.players {
		slices.SortStableFunc(p.characters, func(x, y *Character) int { return cmp.Compare(x.Lifetime.Start, y.Lifetime.Start) })
		var stays []stay
		for i, c := range p.characters {
			at := c.Lifetime.Start
			if i == 0 {
				at = min(at, 0)
			}
			stays = append(stays, stay{tableBuild(c.Agent, at), c})
		}
		for _, s := range b.shared {
			for _, f := range s.folded {
				if f.into.Player == p {
					stays = append(stays, stay{tableBuild(f.entry, tl.rel(f.at)), f.into.Character})
				}
			}
		}
		slices.SortStableFunc(stays, func(x, y stay) int { return cmp.Compare(x.at, y.at) })

		changes := b.builds[p]
		for i, st := range stays {
			until := time.Duration(math.MaxInt64)
			if i+1 < len(stays) {
				until = stays[i+1].at
			}
			// arcdps fills the table when the log ends, so an entry
			// holds the last build of its stay. What the player
			// announced first is what the stay started on.
			last := st.buildChange
			if len(changes) > 0 && changes[0].at < until {
				first := changes[0]
				st.subgroup, st.prof, st.spec = first.subgroup, first.prof, first.spec
			}
			p.pushBuild(st.buildChange, tl.Duration)
			for len(changes) > 0 && changes[0].at < until {
				last = changes[0]
				p.pushBuild(last, tl.Duration)
				changes = changes[1:]
			}
			st.c.spec = last.spec
		}
		p.merge()
		tl.characters = append(tl.characters, p.characters...)
	}
}

// pushBuild opens a span for each value of c that differs from the one
// the player held.
func (p *Player) pushBuild(c buildChange, end time.Duration) {
	if last, ok := p.Subgroup.Last(); !ok || last.Value != c.subgroup {
		pushSpan(&p.Subgroup.spans, c.at, end, c.subgroup, c.event)
	}
	if last, ok := p.Profession.Last(); !ok || last.Value != c.prof {
		pushSpan(&p.Profession.spans, c.at, end, c.prof, c.event)
	}
	if last, ok := p.EliteSpec.Last(); !ok || last.Value != c.spec {
		pushSpan(&p.EliteSpec.spans, c.at, end, c.spec, c.event)
	}
}

// dropUnclaimed removes from the players the entries that stand for no
// character: those folded into another entry and those no event reached.
// A player keeps their first entry when none was reached. The entries stay
// in the agents, which mirror the table.
func (b *builder) dropUnclaimed() {
	for _, p := range b.tl.players {
		if len(p.characters) == 1 {
			continue
		}
		var kept []*Character
		for _, c := range p.characters {
			if b.cnt(c.Agent).seen {
				kept = append(kept, c)
			}
		}
		if len(kept) == 0 {
			kept = p.characters[:1]
		}
		for _, c := range p.characters {
			if !slices.Contains(kept, c) {
				c.Agent.Character = nil
			}
		}
		p.characters = kept
	}
}

// resolve returns the agent for an address, synthesizing one for addresses
// missing from the agent table. The instance id tells apart the table
// entries that share an address.
func (b *builder) resolve(addr uint64, inst uint16, at uint64) *Agent {
	tl := b.tl
	if addr == 0 {
		return tl.Unknown
	}
	addr = tl.canonical(addr)
	if a := tl.byAddr[addr]; a != nil {
		if s := b.shared[addr]; s != nil {
			return s.pick(inst, at)
		}
		return a
	}
	a := &Agent{Timeline: tl, Addr: addr, Kind: KindUnknown, Name: fmt.Sprintf("Unknown %x", addr), idx: len(b.counts)}
	b.counts = append(b.counts, agentCounts{})
	tl.byAddr[addr] = a
	tl.agents = append(tl.agents, a)
	return a
}

// makeSkills creates the skills of the skill table and the buffs described
// by StateBuffInfo events.
func (b *builder) makeSkills() {
	tl, l := b.tl, b.log
	arena := make([]Skill, len(l.Skills))
	tl.skills = make(map[uint32]*Skill, len(l.Skills)+64)
	nBuffs := 0
	for i := range l.Events {
		if l.Events[i].IsStateChange == evtc.StateBuffInfo {
			nBuffs++
		}
	}
	tl.buffs = make(map[uint32]*Buff, nBuffs+16)
	tl.Buffs = make([]*Buff, 0, nBuffs+16)
	b.buffArena = make([]Buff, nBuffs+16)
	for i := range l.Skills {
		raw := &l.Skills[i]
		id := uint32(raw.ID)
		if _, dup := tl.skills[id]; dup {
			continue
		}
		s := &arena[i]
		*s = Skill{ID: id, Name: raw.Name, Custom: isCustomSkill(id)}
		if s.Name == "" {
			s.Name = skillName(id)
		}
		tl.skills[id] = s
	}
	tl.guids = map[contentKey]GUID{}
	tl.effectDefaults = map[uint32]time.Duration{}
	nTimings, nFormulas, nIntegrity := 0, 0, 0
	for i := range l.Events {
		e := &l.Events[i]
		switch e.IsStateChange {
		case evtc.StateBuffInfo:
			buff := b.ensureBuff(e.SkillID)
			buff.Info = e
			buff.MaxDuration = ms(int64(e.OverstackValue))
			buff.StackLimit = int(e.SrcMasterInstanceID)
			buff.Category = e.IsOffcycle
			buff.Stacking = Stacking(e.Pad61)
			buff.Invuln = e.IsFlanking != 0
			buff.Invert = e.IsShields != 0
			buff.Resistance = e.Pad62 != 0
		case evtc.StateSkillInfo:
			skillInfo(b.ensureSkill(e.SkillID), e)
		case evtc.StateSkillTiming:
			b.ensureSkill(e.SkillID).cnt.timings++
			nTimings++
		case evtc.StateBuffFormula:
			b.ensureBuff(e.SkillID).nFormulas++
			nFormulas++
		case evtc.StateIDToGUID:
			bts := e.Bytes()
			kind := ContentKind(e.OverstackValue)
			tl.guids[contentKey{kind, e.SkillID}] = guidAt(&bts, offSrc)
			if kind == ContentEffect {
				if d := floatMS(f32At(&bts, offBuffDmg)); d > 0 {
					tl.effectDefaults[e.SkillID] = d
				}
			}
		case evtc.StateArcBuild:
			bts := e.Bytes()
			tl.ArcBuild = cstrAt(&bts, offSrc)
		case evtc.StateIntegrity:
			nIntegrity++
		}
	}
	if nIntegrity > 0 {
		arena := make([]IntegrityMessage, 0, nIntegrity)
		tl.Integrity = make([]*IntegrityMessage, 0, nIntegrity)
		for i := range l.Events {
			if e := &l.Events[i]; e.IsStateChange == evtc.StateIntegrity {
				bts := e.Bytes()
				arena = append(arena, IntegrityMessage{Message: textAt(&bts, offTime, 32), Event: e})
				tl.Integrity = append(tl.Integrity, &arena[len(arena)-1])
			}
		}
	}
	timings := make([]SkillTiming, nTimings)
	formulas := make([]BuffFormula, nFormulas)
	for _, s := range tl.skills {
		s.Timings = carve(&timings, s.cnt.timings)
	}
	for _, buff := range tl.buffs {
		buff.Formulas = carve(&formulas, buff.nFormulas)
	}
	for i := range l.Events {
		e := &l.Events[i]
		switch e.IsStateChange {
		case evtc.StateSkillTiming:
			s := tl.skills[e.SkillID]
			s.Timings = append(s.Timings, SkillTiming{Kind: uint32(e.SrcAgent), At: ms(int64(e.DstAgent)), Event: e})
		case evtc.StateBuffFormula:
			buff := tl.buffs[e.SkillID]
			buff.Formulas = append(buff.Formulas, formula(e))
		}
	}
}

// skillName names an id the skill table left unnamed.
func skillName(id uint32) string {
	if name, ok := customSkillNames[id]; ok {
		return name
	}
	return strconv.FormatUint(uint64(id), 10)
}

// ensureSkill returns the skill with the given id, creating it when the
// skill table does not have it.
func (b *builder) ensureSkill(id uint32) *Skill {
	if s, ok := b.tl.skills[id]; ok {
		return s
	}
	s := &Skill{ID: id, Name: skillName(id), Custom: isCustomSkill(id)}
	b.tl.skills[id] = s
	return s
}

// ensureBuff returns the buff with the given skill id, creating it when
// needed.
func (b *builder) ensureBuff(id uint32) *Buff {
	if buff, ok := b.tl.buffs[id]; ok {
		return buff
	}
	s := b.ensureSkill(id)
	var buff *Buff
	if len(b.buffArena) > 0 {
		buff = &b.buffArena[0]
		b.buffArena = b.buffArena[1:]
	} else {
		buff = new(Buff)
	}
	buff.Skill = s
	s.Buff = buff
	b.tl.buffs[id] = buff
	b.tl.Buffs = append(b.tl.Buffs, buff)
	return buff
}

// see records that an agent was involved in an event.
func (b *builder) see(a *Agent, e *evtc.Event, inst uint16) {
	c := b.cnt(a)
	if !c.seen || e.Time < c.first {
		c.first = e.Time
	}
	if !c.seen || e.Time > c.last {
		c.last = e.Time
	}
	c.seen = true
	c.events++
	if inst != 0 && a.InstanceID == 0 && a != b.tl.Unknown {
		a.InstanceID = inst
	}
}

// scan resolves the agents of every event, measures lifetimes and counts
// the nodes and edges to allocate.
func (b *builder) scan() {
	tl := b.tl
	events := tl.events
	b.srcs = make([]*Agent, len(events))
	b.dsts = make([]*Agent, len(events))
	lastCast := map[castKey]int{}
	openCast := map[castKey]bool{}
	// Open stacks by trackable id and by (receiver, buff), mirroring the
	// bookkeeping of the fill pass so that activity events are counted
	// for the stack the fill pass will attach them to.
	openStackOrd := map[uint32]int{}
	openStackKey := map[uint32]stackKey{}
	openIDsByKey := map[stackKey][]uint32{}
	closeStackID := func(id uint32) {
		key, ok := openStackKey[id]
		if !ok {
			return
		}
		delete(openStackOrd, id)
		delete(openStackKey, id)
		ids := openIDsByKey[key]
		if i := slices.Index(ids, id); i >= 0 {
			openIDsByKey[key] = slices.Delete(ids, i, i+1)
		}
		c := b.cnt(key.receiver)
		if i := slices.Index(c.openIDs, id); i >= 0 {
			c.openIDs = slices.Delete(c.openIDs, i, i+1)
		}
	}
	openMissile := map[uint32]int{}
	b.extCombat = map[uint32]int{}
	// byInst is the agent last seen with each instance id: arcdps writes
	// the despawn (and a few other state events) of minions and NPCs with
	// src_agent 0 and only the instance id set.
	byInst := make([]*Agent, 1<<16)

	for i, e := range events {
		k := e.IsStateChange
		var src, dst *Agent
		if srcIsAgent(k) {
			src = b.resolve(e.SrcAgent, e.SrcInstanceID, e.Time)
			if src == tl.Unknown && e.SrcAgent == 0 && e.SrcInstanceID != 0 && tracksAgent(k) {
				if a := byInst[e.SrcInstanceID]; a != nil {
					src = a
				}
			}
			b.see(src, e, e.SrcInstanceID)
			if src != tl.Unknown && e.SrcInstanceID != 0 {
				byInst[e.SrcInstanceID] = src
			}
		}
		if dstIsAgent(k) {
			dst = b.resolve(e.DstAgent, e.DstInstanceID, e.Time)
			if dst != src {
				b.see(dst, e, e.DstInstanceID)
			}
		}
		b.srcs[i], b.dsts[i] = src, dst
		if src == tl.Unknown && tracksAgent(k) {
			continue
		}

		switch k {
		case evtc.StateCombat:
			b.nHits++
			b.cnt(src).hits++
			b.cnt(dst).hitsTaken++
			b.ensureSkill(e.SkillID).cnt.hits++
			if idx, ok := lastCast[castKey{src, e.SkillID}]; ok {
				b.castHits[idx]++
			}
			b.markFight(src, dst, e)
		case evtc.StateAnimationStart:
			key := castKey{src, e.SkillID}
			lastCast[key] = b.countCast(src, e.SkillID)
			openCast[key] = true
		case evtc.StateAnimationStop:
			key := castKey{src, e.SkillID}
			if _, ok := lastCast[key]; !ok || !openCast[key] {
				lastCast[key] = b.countCast(src, e.SkillID)
			}
			openCast[key] = false
		case evtc.StateBuffApply, evtc.StateBuffInitial:
			b.nStacks++
			b.cnt(dst).stacks++
			b.cnt(src).stacksApplied++
			b.ensureBuff(e.SkillID).cnt++
			id := trackableID(e)
			closeStackID(id)
			key := stackKey{dst, e.SkillID}
			openStackOrd[id] = b.nStacks - 1
			openStackKey[id] = key
			openIDsByKey[key] = append(openIDsByKey[key], id)
			c := b.cnt(dst)
			c.openIDs = append(c.openIDs, id)
			b.stackActive = append(b.stackActive, 0)
			b.stackChanges = append(b.stackChanges, 0)
		case evtc.StateBuffRemoveSingle:
			b.ensureBuff(e.SkillID)
			closeStackID(trackableID(e))
		case evtc.StateBuffRemoveAll:
			b.ensureBuff(e.SkillID)
			key := stackKey{src, e.SkillID}
			for len(openIDsByKey[key]) > 0 {
				closeStackID(openIDsByKey[key][0])
			}
		case evtc.StateBuffChange:
			b.ensureBuff(e.SkillID)
			if ord, ok := openStackOrd[trackableID(e)]; ok {
				b.stackChanges[ord]++
			}
		case evtc.StateBuffActive:
			if ord, ok := openStackOrd[uint32(e.DstAgent)]; ok {
				b.stackActive[ord]++
			}
		case evtc.StateBuffDeactive:
			if ord, ok := openStackOrd[trackableID(e)]; ok {
				b.stackActive[ord]++
			}
		case evtc.StateMarker:
			// A marker written again while worn is the same marker.
			c := b.cnt(src)
			if e.Value == 0 {
				c.openMarkers = c.openMarkers[:0]
			} else if id := uint32(e.Value); !slices.Contains(c.openMarkers, id) {
				c.openMarkers = append(c.openMarkers, id)
				c.markers++
			}
		case evtc.StateStunBreak:
			b.cnt(src).stunbreaks++
		case evtc.StateTeamChange:
			b.cnt(src).team++
		case evtc.StateWeaponSwap:
			b.cnt(src).swaps++
		case evtc.StateStealthChange:
			b.cnt(src).stealth++
		case evtc.StateGlider:
			b.cnt(src).gliding++
		case evtc.StateTransformation:
			b.cnt(src).transform++
			if e.SkillID != 0 {
				b.ensureSkill(e.SkillID)
			}
		case evtc.StateEffectGroundCreate, evtc.StateEffectAgentCreate:
			b.nEffects++
			b.cnt(src).effects++
		case evtc.StateMissileCreate:
			b.nMissiles++
			b.cnt(src).missiles++
			b.ensureSkill(e.SkillID).cnt.missiles++
			b.missileLaunches = append(b.missileLaunches, 0)
			b.missileEffects = append(b.missileEffects, 0)
			openMissile[trackableID(e)] = b.nMissiles - 1
		case evtc.StateMissileLaunch:
			if ord, ok := openMissile[trackableID(e)]; ok {
				b.missileLaunches[ord]++
			}
		case evtc.StateMissileEffect:
			if ord, ok := openMissile[trackableID(e)]; ok {
				b.missileEffects[ord]++
			}
		case evtc.StateMissileRemove:
			delete(openMissile, trackableID(e))
		case evtc.StateTick:
			b.nTicks++
		case evtc.StateSquadMarkerGround:
			if _, removed := groundMarkerPlace(e); !removed {
				b.nGround++
			}
		case evtc.StateJump:
			b.cnt(src).airborne++
		case evtc.StateGadgetName:
			b.cnt(src).nameVisible++
		case evtc.StateGadgetAnimation:
			b.cnt(src).gadgetAnims++
		case evtc.StateReward:
			b.nRewards++
		case evtc.StateMapChange:
			b.nMapChanges++
		case evtc.StateExtension:
			b.extRegs = append(b.extRegs, e)
		case evtc.StateExtensionCombat:
			b.extCombat[extensionSignature(e)]++
			// arcdps adds the skill of an extension combat event to the skill
			// table, so the graph knows it too.
			b.ensureSkill(e.SkillID)
		case evtc.StatePosition, evtc.StateTeleport:
			b.cnt(src).pos++
		case evtc.StateVelocity:
			b.cnt(src).vel++
		case evtc.StateFacing:
			b.cnt(src).facing++
		case evtc.StateHealthPctUpdate:
			b.cnt(src).health++
		case evtc.StateBarrierPctUpdate:
			b.cnt(src).barrier++
		case evtc.StateMaxHealthUpdate:
			b.cnt(src).maxHealth++
		case evtc.StateDefianceBarPercent:
			b.cnt(src).defPct++
		case evtc.StateDefianceBarState:
			b.cnt(src).defiance++
		case evtc.StateChangeUp, evtc.StateChangeDead, evtc.StateChangeDown, evtc.StateSpawn, evtc.StateDespawn:
			c := b.cnt(src)
			c.states++
			if k == evtc.StateDespawn {
				for len(c.openIDs) > 0 {
					closeStackID(c.openIDs[0])
				}
			}
			if k == evtc.StateChangeDown {
				c.downs++
				b.nDowns++
			}
			if k == evtc.StateChangeDead {
				c.deaths++
				b.nDeaths++
			}
		case evtc.StateEnterCombat, evtc.StateExitCombat:
			b.cnt(src).combat++
		case evtc.StateTargetable:
			b.cnt(src).targetable++
		case evtc.StatePointOfView:
			if src.Player != nil {
				tl.POV = src.Player
			}
		case evtc.StateLogNPCUpdate:
			if dst != tl.Unknown && !slices.Contains(b.bosses, dst) {
				b.bosses = append(b.bosses, dst)
			}
		}
	}
}

// countCast counts one more cast for the caster and the skill and returns
// its index. The fill pass numbers casts in the same order.
func (b *builder) countCast(caster *Agent, skill uint32) int {
	idx := b.nCasts
	b.nCasts++
	b.cnt(caster).casts++
	b.castHits = append(b.castHits, 0)
	b.ensureSkill(skill).cnt.casts++
	return idx
}

// markFight flags an NPC or gadget that exchanged hits with a player.
func (b *builder) markFight(src, dst *Agent, e *evtc.Event) {
	var foe *Agent
	switch {
	case src.Kind == KindPlayer && (dst.Kind == KindNPC || dst.Kind == KindGadget):
		foe = dst
	case dst.Kind == KindPlayer && (src.Kind == KindNPC || src.Kind == KindGadget):
		foe = src
	default:
		return
	}
	if c := b.cnt(foe); !c.fought {
		c.fought = true
		c.foughtAt = e.Time
	}
}

// carve takes the first n elements of an arena as an empty slice with
// capacity n and advances the arena past them.
func carve[T any](arena *[]T, n int) []T {
	out := (*arena)[:0:n]
	*arena = (*arena)[n:]
	return out
}

// allocateAgents sizes the node arenas and every per-agent slice from the
// scan counts, and sets the agent lifetimes.
func (b *builder) allocateAgents() {
	tl := b.tl
	agents := b.allAgents()

	b.hitArena = make([]Hit, b.nHits)
	b.castArena = make([]Cast, b.nCasts)
	b.stackArena = make([]BuffStack, b.nStacks)
	b.downArena = make([]Down, b.nDowns)
	b.deathArena = make([]Death, b.nDeaths)
	b.effectArena = make([]Effect, b.nEffects)
	b.missileArena = make([]Missile, b.nMissiles)
	b.groundArena = make([]GroundMarker, b.nGround)
	b.rewardArena = make([]Reward, b.nRewards)
	b.mapChangeArena = make([]MapChange, b.nMapChanges)
	tl.Rewards = make([]*Reward, 0, b.nRewards)
	tl.MapChanges = make([]*MapChange, 0, b.nMapChanges)
	b.allocateExtensions()
	tl.effects = make([]*Effect, 0, b.nEffects)
	tl.missiles = make([]*Missile, 0, b.nMissiles)
	tl.GroundMarkers = make([]*GroundMarker, 0, b.nGround)
	tl.Ping.samples = make([]Sample[int], 0, b.nTicks)
	nLaunches, nMissileEffects, nActive, nChanges := 0, 0, 0, 0
	for _, n := range b.missileLaunches {
		nLaunches += n
	}
	for _, n := range b.missileEffects {
		nMissileEffects += n
	}
	for _, n := range b.stackActive {
		nActive += n + 1
	}
	for _, n := range b.stackChanges {
		nChanges += n
	}
	b.launchArena = make([]Launch, nLaunches)
	b.missileEffectArena = make([]MissileEffect, nMissileEffects)
	b.activeArena = make([]Span[bool], nActive)
	b.changeArena = make([]*evtc.Event, nChanges)
	tl.hits = make([]*Hit, 0, b.nHits)
	tl.casts = make([]*Cast, 0, b.nCasts)
	tl.stacks = make([]*BuffStack, 0, b.nStacks)

	var c agentCounts
	for _, a := range agents {
		n := b.cnt(a)
		c.events += n.events
		c.hits += n.hits + n.hitsTaken
		c.casts += n.casts
		c.stacks += n.stacks + n.stacksApplied
		c.pos += n.pos
		c.vel += n.vel
		c.facing += n.facing
		c.health += n.health
		c.barrier += n.barrier
		c.maxHealth += n.maxHealth
		c.defPct += n.defPct
		c.states += n.states + 1
		c.defiance += n.defiance
		c.combat += n.combat + 1
		c.targetable += n.targetable
		c.downs += n.downs
		c.deaths += n.deaths
		c.markers += n.markers
		c.stunbreaks += n.stunbreaks
		if n.team > 0 {
			c.team += n.team + 1
		}
		if n.swaps > 0 {
			c.swaps += n.swaps + 1
		}
		c.stealth += n.stealth
		c.gliding += n.gliding
		c.transform += n.transform
		c.effects += n.effects
		c.missiles += n.missiles
		c.airborne += n.airborne
		c.nameVisible += n.nameVisible
		c.gadgetAnims += n.gadgetAnims
	}
	events := make([]*evtc.Event, c.events)
	hits := make([]*Hit, c.hits)
	casts := make([]*Cast, c.casts)
	stacks := make([]*BuffStack, c.stacks)
	pos := make([]Sample[Vec3], c.pos)
	vel := make([]Sample[Vec3], c.vel)
	facing := make([]Sample[Vec2], c.facing)
	health := make([]Sample[float64], c.health)
	barrier := make([]Sample[float64], c.barrier)
	maxHealth := make([]Sample[int64], c.maxHealth)
	defPct := make([]Sample[float64], c.defPct)
	states := make([]Span[LifeState], c.states)
	defiance := make([]Span[DefianceState], c.defiance)
	combat := make([]Span[bool], c.combat)
	targetable := make([]Span[bool], c.targetable)
	downs := make([]*Down, c.downs)
	deaths := make([]*Death, c.deaths)
	b.markerArena = make([]Marker, c.markers)
	b.stunArena = make([]StunBreak, c.stunbreaks)
	markers := make([]*Marker, c.markers)
	stunbreaks := make([]*StunBreak, c.stunbreaks)
	team := make([]Span[uint32], c.team)
	swaps := make([]Span[uint32], c.swaps)
	stealth := make([]Span[uint8], c.stealth)
	gliding := make([]Span[bool], c.gliding)
	transform := make([]Span[*Skill], c.transform)
	effects := make([]*Effect, c.effects)
	missiles := make([]*Missile, c.missiles)
	airborne := make([]Span[bool], c.airborne)
	nameVisible := make([]Span[bool], c.nameVisible)
	b.gadgetAnimArena = make([]GadgetAnimation, c.gadgetAnims)
	gadgetAnims := make([]*GadgetAnimation, c.gadgetAnims)

	for _, a := range agents {
		n := b.cnt(a)
		if n.seen && a != tl.Unknown {
			a.Lifetime = Interval{Start: tl.rel(n.first), End: tl.rel(n.last)}
		}
		a.events = carve(&events, n.events)
		a.hits = carve(&hits, n.hits)
		a.hitsTaken = carve(&hits, n.hitsTaken)
		a.casts = carve(&casts, n.casts)
		a.stacks = carve(&stacks, n.stacks)
		a.stacksApplied = carve(&stacks, n.stacksApplied)
		a.Position.samples = carve(&pos, n.pos)
		a.Velocity.samples = carve(&vel, n.vel)
		a.Facing.samples = carve(&facing, n.facing)
		a.Health.samples = carve(&health, n.health)
		a.Barrier.samples = carve(&barrier, n.barrier)
		a.MaxHealth.samples = carve(&maxHealth, n.maxHealth)
		a.DefiancePercent.samples = carve(&defPct, n.defPct)
		a.Life.spans = carve(&states, n.states+1)
		a.Defiance.spans = carve(&defiance, n.defiance)
		a.InCombat.spans = carve(&combat, n.combat+1)
		a.Targetable.spans = carve(&targetable, n.targetable)
		a.Downs = carve(&downs, n.downs)
		a.Deaths = carve(&deaths, n.deaths)
		a.Markers = carve(&markers, n.markers)
		a.StunBreaks = carve(&stunbreaks, n.stunbreaks)
		if n.team > 0 {
			a.Team.spans = carve(&team, n.team+1)
		}
		if n.swaps > 0 {
			a.WeaponSet.spans = carve(&swaps, n.swaps+1)
		}
		a.Stealth.spans = carve(&stealth, n.stealth)
		a.Gliding.spans = carve(&gliding, n.gliding)
		a.Transformation.spans = carve(&transform, n.transform)
		a.effects = carve(&effects, n.effects)
		a.missiles = carve(&missiles, n.missiles)
		a.Airborne.spans = carve(&airborne, n.airborne)
		a.NameVisible.spans = carve(&nameVisible, n.nameVisible)
		a.GadgetAnimations = carve(&gadgetAnims, n.gadgetAnims)
	}

	// Agents by instance id, from one arena of pointers.
	perInst := map[uint16]int{}
	for _, a := range agents {
		if a.InstanceID != 0 {
			perInst[a.InstanceID]++
		}
	}
	tl.byInst = make(map[uint16][]*Agent, len(perInst))
	instRefs := make([]*Agent, len(agents))
	for _, a := range agents {
		if a.InstanceID == 0 {
			continue
		}
		list, ok := tl.byInst[a.InstanceID]
		if !ok {
			list = carve(&instRefs, perInst[a.InstanceID])
		}
		tl.byInst[a.InstanceID] = append(list, a)
	}
	// A character that came back answers to each of its instance ids.
	for addr, s := range b.shared {
		if tl.sharedInst == nil {
			tl.sharedInst = map[uint64]map[uint16]*Agent{}
		}
		tl.sharedInst[addr] = s.owner
		for inst, a := range s.owner {
			if inst != a.InstanceID {
				tl.byInst[inst] = append(tl.byInst[inst], a)
			}
		}
	}
	for _, list := range tl.byInst {
		slices.SortStableFunc(list, func(x, y *Agent) int { return cmp.Compare(x.Lifetime.Start, y.Lifetime.Start) })
	}
}

// allocateSkills sizes the per-skill, per-buff and per-cast edges from the
// scan counts.
func (b *builder) allocateSkills() {
	tl := b.tl
	skillHits := make([]*Hit, b.nHits)
	skillCasts := make([]*Cast, b.nCasts)
	skillMissiles := make([]*Missile, b.nMissiles)
	for _, s := range tl.skills {
		s.hits = carve(&skillHits, s.cnt.hits)
		s.casts = carve(&skillCasts, s.cnt.casts)
		s.missiles = carve(&skillMissiles, s.cnt.missiles)
	}
	buffStacks := make([]*BuffStack, b.nStacks)
	for _, buff := range tl.buffs {
		buff.stacks = carve(&buffStacks, buff.cnt)
	}
	total := 0
	for _, n := range b.castHits {
		total += n
	}
	b.castHitRefs = make([]*Hit, total)
}

// allocateExtensions creates the extension nodes from the registration
// events seen by scan, so that combat events written before a
// registration are attached too. The first registration of a signature
// owns its combat events.
func (b *builder) allocateExtensions() {
	tl := b.tl
	b.extensionArena = make([]Extension, len(b.extRegs))
	tl.Extensions = make([]*Extension, 0, len(b.extRegs))
	b.extBySig = make(map[uint32]*Extension, len(b.extRegs))
	total := 0
	for _, e := range b.extRegs {
		sig := uint32(e.SrcAgent)
		if _, dup := b.extBySig[sig]; !dup {
			b.extBySig[sig] = nil
			total += b.extCombat[sig]
		}
	}
	refs := make([]*evtc.Event, total)
	for i, e := range b.extRegs {
		x := &b.extensionArena[i]
		bts := e.Bytes()
		*x = Extension{Time: tl.rel(e.Time), Signature: uint32(e.SrcAgent), Version: textAt(&bts, offDst, 8), Event: e, Timeline: tl}
		if b.extBySig[x.Signature] == nil {
			b.extBySig[x.Signature] = x
			x.events = carve(&refs, b.extCombat[x.Signature])
		}
		tl.Extensions = append(tl.Extensions, x)
	}
	tl.extensions = b.extBySig
}

// clipSpans ends the last span at end when it runs past it.
func clipSpans[T any](s *Spans[T], end time.Duration) {
	if n := len(s.spans); n > 0 && s.spans[n-1].End > end {
		s.spans[n-1].End = max(end, s.spans[n-1].Start)
	}
}

// pushSpan closes the current span at t and opens a new one holding v.
func pushSpan[T any](spans *[]Span[T], t, end time.Duration, v T, e *evtc.Event) {
	if n := len(*spans); n > 0 {
		(*spans)[n-1].End = t
	}
	*spans = append(*spans, Span[T]{Interval: Interval{Start: t, End: end}, Value: v, Event: e})
}

// fill creates the nodes in the arenas and links them.
func (b *builder) fill() {
	tl := b.tl
	end := tl.Duration
	b.lastCast = map[castKey]*Cast{}
	b.openStacks = map[uint32]*BuffStack{}
	b.openByKey = map[stackKey][]*BuffStack{}
	b.openByAgent = make([][]*BuffStack, len(b.counts))
	b.openDown = map[*Agent]*Down{}
	b.openEffects = map[uint32]*Effect{}
	b.openMissiles = map[uint32]*Missile{}
	b.builds = map[*Player][]buildChange{}

	// Every tracked agent starts alive and out of combat.
	for _, a := range b.allAgents() {
		n := b.cnt(a)
		if a == tl.Unknown || !n.seen {
			continue
		}
		pushSpan(&a.Life.spans, a.Lifetime.Start, end, LifeAlive, nil)
		if n.combat > 0 {
			pushSpan(&a.InCombat.spans, a.Lifetime.Start, end, false, nil)
		}
	}

	for i, e := range tl.events {
		src, dst := b.srcs[i], b.dsts[i]
		t := tl.rel(e.Time)
		if src != nil {
			src.events = append(src.events, e)
			b.resolveMaster(src, e.SrcMasterInstanceID, t)
		}
		if dst != nil && dst != src {
			dst.events = append(dst.events, e)
			b.resolveMaster(dst, e.DstMasterInstanceID, t)
		}
		if src == tl.Unknown && tracksAgent(e.IsStateChange) {
			continue
		}

		switch e.IsStateChange {
		case evtc.StateCombat:
			b.newHit(e, t, src, dst)
		case evtc.StateAnimationStart:
			b.startCast(e, t, src, dst)
		case evtc.StateAnimationStop:
			b.stopCast(e, t, src)
		case evtc.StateBuffApply, evtc.StateBuffInitial:
			b.newStack(e, t, src, dst)
		case evtc.StateBuffRemoveSingle:
			if s := b.openStacks[trackableID(e)]; s != nil {
				b.closeStack(s, e, t, remover(e, dst), e.IsBuffRemove)
			}
		case evtc.StateBuffRemoveAll:
			key := stackKey{src, e.SkillID}
			for _, s := range slices.Clone(b.openByKey[key]) {
				b.closeStack(s, e, t, remover(e, dst), e.IsBuffRemove)
			}
		case evtc.StateBuffChange:
			if s := b.openStacks[trackableID(e)]; s != nil {
				s.Extended += ms(int64(e.Value))
				s.Changes = append(s.Changes, e)
			}
		case evtc.StateBuffActive:
			if s := b.openStacks[uint32(e.DstAgent)]; s != nil {
				pushSpan(&s.Active.spans, t, end, true, e)
			}
		case evtc.StateBuffDeactive:
			if s := b.openStacks[trackableID(e)]; s != nil {
				pushSpan(&s.Active.spans, t, end, false, e)
			}
		case evtc.StatePosition:
			src.Position.samples = append(src.Position.samples, Sample[Vec3]{Time: t, Value: vec3(e), Event: e})
		case evtc.StateTeleport:
			src.Position.samples = append(src.Position.samples, Sample[Vec3]{Time: t, Value: vec3(e), Event: e, Break: true})
		case evtc.StateVelocity:
			src.Velocity.samples = append(src.Velocity.samples, Sample[Vec3]{Time: t, Value: vec3(e), Event: e})
		case evtc.StateFacing:
			src.Facing.samples = append(src.Facing.samples, Sample[Vec2]{Time: t, Value: vec2(e), Event: e})
		case evtc.StateHealthPctUpdate:
			src.Health.samples = append(src.Health.samples, Sample[float64]{Time: t, Value: percent(e), Event: e})
		case evtc.StateBarrierPctUpdate:
			src.Barrier.samples = append(src.Barrier.samples, Sample[float64]{Time: t, Value: percent(e), Event: e})
		case evtc.StateMaxHealthUpdate:
			src.MaxHealth.samples = append(src.MaxHealth.samples, Sample[int64]{Time: t, Value: int64(e.DstAgent), Event: e})
		case evtc.StateDefianceBarPercent:
			src.DefiancePercent.samples = append(src.DefiancePercent.samples, Sample[float64]{Time: t, Value: defiancePercent(e), Event: e})
		case evtc.StateDefianceBarState:
			pushSpan(&src.Defiance.spans, t, end, defianceState(e), e)
		case evtc.StateChangeUp, evtc.StateSpawn, evtc.StateChangeDown, evtc.StateChangeDead, evtc.StateDespawn:
			b.changeState(e, t, src)
			if e.IsStateChange == evtc.StateDespawn {
				// An agent leaves tracking with its buffs: no removal
				// will be logged for them.
				for len(b.openByAgent[src.idx]) > 0 {
					s := b.openByAgent[src.idx][0]
					s.EndedByDespawn = true
					b.closeStack(s, nil, t, nil, evtc.BuffRemoveNone)
				}
			}
		case evtc.StateEnterCombat:
			pushSpan(&src.InCombat.spans, t, end, true, e)
			// Older arcdps versions leave the profession out of the event.
			if p := src.Player; p != nil && e.Value != 0 {
				b.builds[p] = append(b.builds[p], buildChange{
					at: t, subgroup: int(e.DstAgent), prof: Profession(e.Value), spec: EliteSpec(e.BuffDamage), event: e,
				})
			}
		case evtc.StateExitCombat:
			pushSpan(&src.InCombat.spans, t, end, false, e)
		case evtc.StateTargetable:
			pushSpan(&src.Targetable.spans, t, end, e.DstAgent == 1, e)
		case evtc.StateMarker:
			b.marker(e, t, src)
		case evtc.StateStunBreak:
			s := &b.stunArena[b.stunIdx]
			b.stunIdx++
			*s = StunBreak{Time: t, Remaining: ms(int64(e.Value)), Agent: src, Event: e}
			src.StunBreaks = append(src.StunBreaks, s)
		case evtc.StateTeamChange:
			if len(src.Team.spans) == 0 {
				pushSpan(&src.Team.spans, src.Lifetime.Start, end, uint32(e.Value), nil)
			}
			pushSpan(&src.Team.spans, t, end, uint32(e.DstAgent), e)
		case evtc.StateWeaponSwap:
			if len(src.WeaponSet.spans) == 0 {
				pushSpan(&src.WeaponSet.spans, src.Lifetime.Start, end, uint32(e.Value), nil)
			}
			pushSpan(&src.WeaponSet.spans, t, end, uint32(e.DstAgent), e)
		case evtc.StateStealthChange:
			pushSpan(&src.Stealth.spans, t, end, uint8(e.DstAgent), e)
		case evtc.StateGlider:
			pushSpan(&src.Gliding.spans, t, end, e.Value == 1, e)
		case evtc.StateTransformation:
			var s *Skill
			if e.SkillID != 0 {
				s = tl.skills[e.SkillID]
			}
			pushSpan(&src.Transformation.spans, t, end, s, e)
		case evtc.StateGuild:
			if src.Player != nil {
				bts := e.Bytes()
				src.Player.Guild = guidAt(&bts, offDst)
			}
		case evtc.StateEffectGroundCreate, evtc.StateEffectAgentCreate:
			b.newEffect(e, t, src)
		case evtc.StateEffectGroundRemove, evtc.StateEffectAgentRemove:
			if f := b.openEffects[trackableID(e)]; f != nil {
				b.closeEffect(f, e, t)
			}
		case evtc.StateMissileCreate:
			b.newMissile(e, t, src)
		case evtc.StateMissileLaunch:
			if m := b.openMissiles[trackableID(e)]; m != nil {
				target := dst
				if e.DstAgent == 0 {
					target = nil
				}
				m.Launches = append(m.Launches, launch(e, t, target))
			}
		case evtc.StateMissileEffect:
			if m := b.openMissiles[trackableID(e)]; m != nil {
				me := MissileEffect{Time: t, EffectID: e.SkillID, Duration: ms(int64(uint32(e.Value))), Event: e}
				me.GUID = tl.GUID(ContentEffect, e.SkillID)
				m.Effects = append(m.Effects, me)
			}
		case evtc.StateMissileRemove:
			if m := b.openMissiles[trackableID(e)]; m != nil {
				b.closeMissile(m, e, t)
			}
		case evtc.StateTick:
			tl.Ping.samples = append(tl.Ping.samples, Sample[int]{Time: t, Value: int(e.Value), Event: e})
		case evtc.StateSquadMarkerGround:
			b.groundMarker(e, t)
		case evtc.StateLanguage:
			tl.Language = Language(e.SrcAgent)
		case evtc.StateGWBuild:
			tl.GameBuild = uint32(e.SrcAgent)
		case evtc.StateShardID:
			tl.ShardID = uint32(e.SrcAgent)
		case evtc.StateFractalScale:
			tl.FractalScale = uint32(e.SrcAgent)
		case evtc.StateRuleset:
			tl.Ruleset = Ruleset(e.SrcAgent)
		case evtc.StateInstanceStart:
			// The instance start is a timestamp of the clock the events use.
			tl.InstanceStart = tl.WallClock(tl.rel(e.SrcAgent))
		case evtc.StateSquadCombatEnd:
			tl.EndedByMapExit = e.DstAgent&1 != 0
		case evtc.StateJump:
			pushSpan(&src.Airborne.spans, t, end, e.DstAgent == 1, e)
		case evtc.StateGadgetName:
			pushSpan(&src.NameVisible.spans, t, end, e.DstAgent == 1, e)
		case evtc.StateGadgetAnimation:
			ga := &b.gadgetAnimArena[b.gadgetAnimIdx]
			b.gadgetAnimIdx++
			*ga = GadgetAnimation{Time: t, Token: e.DstAgent, Agent: src, Event: e}
			src.GadgetAnimations = append(src.GadgetAnimations, ga)
		case evtc.StateReward:
			r := &b.rewardArena[b.rewardIdx]
			b.rewardIdx++
			*r = Reward{Time: t, ID: e.DstAgent, Kind: e.Value, Event: e}
			tl.Rewards = append(tl.Rewards, r)
		case evtc.StateMapChange:
			mc := &b.mapChangeArena[b.mapChangeIdx]
			b.mapChangeIdx++
			*mc = MapChange{Time: t, From: uint32(e.DstAgent), To: uint32(e.SrcAgent), Kind: e.Value, Event: e}
			tl.MapChanges = append(tl.MapChanges, mc)
		case evtc.StateExtensionCombat:
			if x := b.extBySig[extensionSignature(e)]; x != nil {
				x.events = append(x.events, e)
			}
		case evtc.StateAttackTarget:
			if src != tl.Unknown && dst != tl.Unknown {
				src.Gadget = dst
				dst.AttackTargets = append(dst.AttackTargets, src)
			}
		}
	}
}

// tracksAgent reports whether an event kind describes the state of its
// source agent (movement, health, life state, combat, targetable). Such
// events with an unknown source are noise and are not attached to the
// Unknown sentinel.
func tracksAgent(k evtc.StateChange) bool {
	switch k {
	case evtc.StatePosition, evtc.StateTeleport, evtc.StateVelocity, evtc.StateFacing,
		evtc.StateHealthPctUpdate, evtc.StateBarrierPctUpdate, evtc.StateMaxHealthUpdate,
		evtc.StateDefianceBarPercent, evtc.StateDefianceBarState,
		evtc.StateChangeUp, evtc.StateChangeDown, evtc.StateChangeDead, evtc.StateSpawn, evtc.StateDespawn,
		evtc.StateEnterCombat, evtc.StateExitCombat, evtc.StateTargetable,
		evtc.StateWeaponSwap, evtc.StateTeamChange, evtc.StateStealthChange, evtc.StateGlider,
		evtc.StateTransformation, evtc.StateStunBreak, evtc.StateMarker, evtc.StateGuild,
		evtc.StateJump, evtc.StateGadgetAnimation, evtc.StateGadgetName,
		evtc.StateGadgetModelInfo:
		return true
	}
	return false
}

// resolveMaster sets the master of a from a master instance id, the first
// time one is seen.
func (b *builder) resolveMaster(a *Agent, masterInst uint16, t time.Duration) {
	// Events without a source may still carry a master instance id (a
	// minion despawning, for one); the Unknown sentinel never gets a
	// master, or every unknown hit would be credited to that agent.
	if masterInst == 0 || a.Master != nil || a == b.tl.Unknown {
		return
	}
	if m := b.tl.AgentAt(masterInst, t); m != nil && m != a && m != b.tl.Unknown {
		a.Master = m
	}
}

// remover returns the agent named as remover by a buff removal event, nil
// when the event names none.
func remover(e *evtc.Event, dst *Agent) *Agent {
	if e.DstAgent == 0 {
		return nil
	}
	return dst
}

// newHit creates the hit of a combat event and links it to its agents,
// its skill and its cast.
func (b *builder) newHit(e *evtc.Event, t time.Duration, src, dst *Agent) {
	h := &b.hitArena[b.hitIdx]
	b.hitIdx++
	*h = Hit{
		Event:        e,
		Time:         t,
		Src:          src,
		Dst:          dst,
		Skill:        b.tl.skills[e.SkillID],
		Result:       e.Result,
		IsBuff:       e.Buff != 0,
		IFF:          e.IFF,
		OverNinety:   e.IsNinety != 0,
		UnderFifty:   e.IsFifty != 0,
		Moving:       e.IsMoving&1 != 0,
		TargetMoving: e.IsMoving&2 != 0,
		Flanking:     e.IsFlanking != 0,
		Shielded:     e.IsShields != 0,
		TargetDowned: e.IsOffcycle != 0,
	}
	if h.IsBuff {
		h.Damage = e.BuffDamage
	} else {
		h.Damage = e.Value
	}
	// overstack_value holds the barrier part of strikes and buff ticks
	// alike, and is only meaningful when is_shields is set.
	if h.Shielded {
		h.Barrier = int32(e.OverstackValue)
	}
	src.hits = append(src.hits, h)
	dst.hitsTaken = append(dst.hitsTaken, h)
	h.Skill.hits = append(h.Skill.hits, h)
	b.tl.hits = append(b.tl.hits, h)
	if c := b.lastCast[castKey{src, e.SkillID}]; c != nil {
		h.Cast = c
		c.hits = append(c.hits, h)
	}
}

// nextCast takes the next cast of the arena, with its hit slice sized by
// the scan, and links it to its caster, its skill and the timeline.
func (b *builder) nextCast(caster *Agent, skill uint32) *Cast {
	c := &b.castArena[b.castIdx]
	c.hits = carve(&b.castHitRefs, b.castHits[b.castIdx])
	b.castIdx++
	c.Caster = caster
	c.Skill = b.tl.skills[skill]
	caster.casts = append(caster.casts, c)
	c.Skill.casts = append(c.Skill.casts, c)
	b.tl.casts = append(b.tl.casts, c)
	return c
}

// startCast opens a cast from an animation start event.
func (b *builder) startCast(e *evtc.Event, t time.Duration, src, dst *Agent) {
	c := b.nextCast(src, e.SkillID)
	c.Start = e
	if e.DstAgent != 0 {
		c.Target = dst
	}
	c.Expected = ms(int64(e.Value))
	c.Control = ms(int64(e.BuffDamage))
	c.Interval = Interval{Start: t, End: t + max(c.Expected, c.Control, 0)}
	b.lastCast[castKey{src, e.SkillID}] = c
}

// stopCast closes the open cast of the caster and skill, or creates a cast
// that began before its start was seen.
func (b *builder) stopCast(e *evtc.Event, t time.Duration, src *Agent) {
	key := castKey{src, e.SkillID}
	c := b.lastCast[key]
	if c == nil || c.Stop != nil {
		c = b.nextCast(src, e.SkillID)
		c.Interval = Interval{Start: t - max(ms(int64(e.BuffDamage)), 0), End: t}
		b.lastCast[key] = c
	}
	c.Stop = e
	c.Elapsed = ms(int64(e.Value))
	c.ElapsedUnscaled = ms(int64(e.BuffDamage))
	c.Activation = e.IsActivation
	c.Interval.End = t
}

// newStack opens a buff stack from an application event.
func (b *builder) newStack(e *evtc.Event, t time.Duration, src, dst *Agent) {
	s := &b.stackArena[b.stackIdx]
	b.stackIdx++
	*s = BuffStack{
		ID:            trackableID(e),
		Buff:          b.tl.buffs[e.SkillID],
		Applier:       src,
		Receiver:      dst,
		Interval:      Interval{Start: t, End: b.tl.Duration},
		Applied:       ms(int64(e.Value)),
		Initial:       e.IsStateChange == evtc.StateBuffInitial,
		ActiveOnApply: e.IsShields != 0,
		Apply:         e,
		IFF:           e.IFF,
	}
	if s.Initial {
		s.Original = ms(int64(e.BuffDamage))
	}
	s.Active.spans = carve(&b.activeArena, 1+b.stackActive[b.stackIdx-1])
	s.Changes = carve(&b.changeArena, b.stackChanges[b.stackIdx-1])
	pushSpan(&s.Active.spans, t, b.tl.Duration, s.ActiveOnApply, nil)
	dst.stacks = append(dst.stacks, s)
	src.stacksApplied = append(src.stacksApplied, s)
	s.Buff.stacks = append(s.Buff.stacks, s)
	b.tl.stacks = append(b.tl.stacks, s)
	if prev := b.openStacks[s.ID]; prev != nil {
		// The id was reused before a removal was seen: the previous
		// stack is gone.
		prev.Superseded = true
		b.closeStack(prev, nil, t, nil, evtc.BuffRemoveNone)
	}
	b.openStacks[s.ID] = s
	key := stackKey{dst, e.SkillID}
	s.openIdx = len(b.openByKey[key])
	b.openByKey[key] = append(b.openByKey[key], s)
	b.openByAgent[dst.idx] = append(b.openByAgent[dst.idx], s)
}

// closeStack ends a stack and drops it from the open lists. A nil event
// marks a stack ended by the reuse of its id.
func (b *builder) closeStack(s *BuffStack, e *evtc.Event, t time.Duration, by *Agent, how evtc.BuffRemove) {
	s.Remove = e
	s.Removal = how
	s.RemovedBy = by
	s.Interval.End = t
	if n := len(s.Active.spans); n > 0 {
		s.Active.spans[n-1].End = t
	}
	if e != nil && e.IsStateChange == evtc.StateBuffRemoveSingle {
		s.Remaining = ms(int64(e.Value))
	}
	delete(b.openStacks, s.ID)
	key := stackKey{s.Receiver, s.Buff.Skill.ID}
	list := b.openByKey[key]
	if s.openIdx < len(list) && list[s.openIdx] == s {
		last := list[len(list)-1]
		list[s.openIdx] = last
		last.openIdx = s.openIdx
		b.openByKey[key] = list[:len(list)-1]
	}
	if mine := b.openByAgent[s.Receiver.idx]; len(mine) > 0 {
		if i := slices.Index(mine, s); i >= 0 {
			b.openByAgent[s.Receiver.idx] = slices.Delete(mine, i, i+1)
		}
	}
}

// changeState records a life state event and the down or death it opens
// or closes.
func (b *builder) changeState(e *evtc.Event, t time.Duration, src *Agent) {
	end := b.tl.Duration
	closeDown := func(recovered bool, death *Death) {
		d := b.openDown[src]
		if d == nil {
			return
		}
		d.End = t
		d.Recovered = recovered
		d.Death = death
		if death != nil {
			death.Down = d
		}
		delete(b.openDown, src)
	}
	switch e.IsStateChange {
	case evtc.StateChangeUp, evtc.StateSpawn:
		pushSpan(&src.Life.spans, t, end, LifeAlive, e)
		closeDown(true, nil)
	case evtc.StateChangeDown:
		pushSpan(&src.Life.spans, t, end, LifeDown, e)
		// A down reported twice ends the first one where the second begins.
		closeDown(false, nil)
		d := &b.downArena[b.downIdx]
		b.downIdx++
		*d = Down{Interval: Interval{Start: t, End: end}, Agent: src, Event: e}
		src.Downs = append(src.Downs, d)
		b.openDown[src] = d
	case evtc.StateChangeDead:
		pushSpan(&src.Life.spans, t, end, LifeDead, e)
		de := &b.deathArena[b.deathIdx]
		b.deathIdx++
		*de = Death{Time: t, Agent: src, Event: e}
		src.Deaths = append(src.Deaths, de)
		closeDown(false, de)
	case evtc.StateDespawn:
		pushSpan(&src.Life.spans, t, end, LifeGone, e)
		closeDown(false, nil)
	}
}

// marker records a marker put on an agent or, for a zero id, the removal
// of every marker the agent wears. The worn markers of an agent all sit
// after the last one a removal ended, since a removal ends every marker.
func (b *builder) marker(e *evtc.Event, t time.Duration, src *Agent) {
	id := uint32(e.Value)
	for i := len(src.Markers) - 1; i >= 0; i-- {
		m := src.Markers[i]
		if m.Remove != nil {
			break
		}
		if id == 0 {
			m.Remove = e
			m.Interval.End = t
		} else if m.ID == id {
			// The marker is written again while worn: it is the same
			// marker, and the scan counted no new one.
			m.Commander = m.Commander || e.Buff != 0
			return
		}
	}
	if id == 0 {
		return
	}
	m := &b.markerArena[b.markerIdx]
	b.markerIdx++
	g := b.tl.GUID(ContentMarker, id)
	tag, catmander := tagOf(g)
	*m = Marker{
		ID:        id,
		GUID:      g,
		Squad:     squadOf(g),
		Commander: e.Buff != 0 || tag != TagNone,
		Tag:       tag,
		Catmander: catmander,
		Agent:     src,
		Interval:  Interval{Start: t, End: b.tl.Duration},
		Event:     e,
	}
	src.Markers = append(src.Markers, m)
}

// groundMarker records a squad marker placed on the ground or, for zero
// or infinite coordinates, removed from it. A placement ends the previous
// placement of the same index: the marker moved.
func (b *builder) groundMarker(e *evtc.Event, t time.Duration) {
	pos, removed := groundMarkerPlace(e)
	if prev := b.openGround[e.SkillID]; prev != nil {
		prev.Interval.End = t
		if removed {
			prev.Remove = e
		}
		delete(b.openGround, e.SkillID)
	}
	if removed {
		return
	}
	gm := &b.groundArena[b.groundIdx]
	b.groundIdx++
	*gm = GroundMarker{Index: e.SkillID, Squad: squadOfIndex(e.SkillID), Position: pos, Interval: Interval{Start: t, End: b.tl.Duration}, Event: e}
	if b.openGround == nil {
		b.openGround = map[uint32]*GroundMarker{}
	}
	b.openGround[e.SkillID] = gm
	b.tl.GroundMarkers = append(b.tl.GroundMarkers, gm)
}

// finish resolves the derived links: series settings, minions, causes,
// breakbars and targets.
func (b *builder) finish() {
	tl := b.tl
	agents := b.allAgents()

	for _, a := range agents {
		life := a.Lifetime
		a.Position = a.Position.Interpolated(LerpVec3, MoveGap).Bounded(life, false)
		a.Velocity = a.Velocity.Interpolated(LerpVec3, MoveGap).Bounded(life, false)
		a.Facing = a.Facing.Interpolated(LerpVec2, MoveGap).Bounded(life, false)
		a.Health = a.Health.Bounded(life, true)
		a.Barrier = a.Barrier.Bounded(life, true)
		a.MaxHealth = a.MaxHealth.Bounded(life, true)
		a.DefiancePercent = a.DefiancePercent.Bounded(life, true)
		// States are known within the lifetime too; Life alone keeps its
		// final state (dead, gone) to the end of the log.
		if a != tl.Unknown {
			clipSpans(&a.Defiance, life.End)
			clipSpans(&a.InCombat, life.End)
			clipSpans(&a.Targetable, life.End)
			clipSpans(&a.Team, life.End)
			clipSpans(&a.WeaponSet, life.End)
			clipSpans(&a.Stealth, life.End)
			clipSpans(&a.Gliding, life.End)
			clipSpans(&a.Transformation, life.End)
			clipSpans(&a.Airborne, life.End)
			clipSpans(&a.NameVisible, life.End)
			for _, m := range a.Markers {
				m.Interval.End = min(m.Interval.End, life.End)
			}
		}
		if a.Master != nil {
			b.cnt(a.Master).minions++
		}
	}

	// Minions.
	total := 0
	for _, a := range agents {
		total += b.cnt(a).minions
	}
	minions := make([]*Agent, total)
	for _, a := range agents {
		a.Minions = carve(&minions, b.cnt(a).minions)
	}
	for _, a := range agents {
		if a.Master != nil {
			a.Master.Minions = append(a.Master.Minions, a)
		}
	}

	// Credited hits: a master whose minions hit gets its own hits merged
	// with theirs, in time order. Everyone else answers with its own hits,
	// so nothing is duplicated for them.
	total = 0
	for _, a := range agents {
		if n := minionHits(a); n > 0 {
			total += len(a.hits) + n
		}
	}
	credited := make([]*Hit, total)
	for _, a := range agents {
		n := minionHits(a)
		if n == 0 {
			continue
		}
		a.hitsCredited = carve(&credited, len(a.hits)+n)
		a.hitsCredited = append(a.hitsCredited, a.hits...)
		for _, m := range a.Minions {
			a.hitsCredited = append(a.hitsCredited, m.hits...)
		}
		sortedByTime(a.hitsCredited, hitTime)
	}

	// Causes of downs and deaths.
	for _, a := range agents {
		for _, d := range a.Downs {
			if h := cause(a, d.Start, evtc.ResultDowned); h != nil {
				d.Cause = h
				h.Down = d
			}
		}
		for _, de := range a.Deaths {
			if h := cause(a, de.Time, evtc.ResultKillingBlow); h != nil {
				de.Cause = h
				h.Death = de
			}
		}
	}

	// Breakbars: one per active defiance span.
	nBars, nBarHits := 0, 0
	for _, a := range agents {
		for _, sp := range a.Defiance.spans {
			if sp.Value == DefianceActive {
				nBars++
				b.cnt(a).breakbars++
				nBarHits += defianceHits(a, sp.Interval).Count()
			}
		}
	}
	bars := make([]Breakbar, nBars)
	barRefs := make([]*Breakbar, nBars)
	barHits := make([]*Hit, nBarHits)
	barIdx := 0
	for _, a := range agents {
		a.Breakbars = carve(&barRefs, b.cnt(a).breakbars)
		spans := a.Defiance.spans
		for i, sp := range spans {
			if sp.Value != DefianceActive {
				continue
			}
			bar := &bars[barIdx]
			barIdx++
			hits := defianceHits(a, sp.Interval)
			bar.Agent = a
			bar.Interval = sp.Interval
			bar.Percent = a.DefiancePercent.Between(sp.Interval)
			bar.hits = carve(&barHits, hits.Count())
			for h := range hits.Seq() {
				bar.hits = append(bar.hits, h)
			}
			bar.End = DefianceNone
			if i+1 < len(spans) {
				bar.End = spans[i+1].Value
			}
			a.Breakbars = append(a.Breakbars, bar)
		}
	}

	// Targets: bosses first, then foes in order of first exchange.
	if len(b.bosses) == 0 {
		if id := tl.Log.Header.TargetSpeciesID; id > 2 {
			for _, a := range tl.npcs {
				if a.SpeciesID == id && b.cnt(a).seen {
					b.bosses = append(b.bosses, a)
					break
				}
			}
		}
	}
	var foes []*Agent
	for _, a := range tl.agents {
		if b.cnt(a).fought && !slices.Contains(b.bosses, a) {
			foes = append(foes, a)
		}
	}
	slices.SortStableFunc(foes, func(x, y *Agent) int { return cmp.Compare(b.cnt(x).foughtAt, b.cnt(y).foughtAt) })
	targets := make([]Target, 0, len(b.bosses)+len(foes))
	for _, a := range b.bosses {
		targets = append(targets, Target{Agent: a, Boss: true})
	}
	for _, a := range foes {
		targets = append(targets, Target{Agent: a})
	}
	tl.targets = make([]*Target, len(targets))
	for i := range targets {
		tl.targets[i] = &targets[i]
		targets[i].Agent.Target = &targets[i]
	}

	// Sorted skill and buff lists.
	tl.Skills = make([]*Skill, 0, len(tl.skills))
	for _, s := range tl.skills {
		tl.Skills = append(tl.Skills, s)
	}
	slices.SortFunc(tl.Skills, func(x, y *Skill) int { return cmp.Compare(x.ID, y.ID) })
	slices.SortFunc(tl.Buffs, func(x, y *Buff) int { return cmp.Compare(x.Skill.ID, y.Skill.ID) })

	// Cast lists are filled in event order; a cast whose stop was seen
	// without its start begins before that event, so restore the start
	// order the Between filters rely on.
	sortedByTime(tl.casts, castStart)
	b.finishPlayers()
	for _, a := range agents {
		sortedByTime(a.casts, castStart)
	}
	for _, s := range tl.Skills {
		sortedByTime(s.casts, castStart)
		s.GUID = tl.GUID(ContentSkill, s.ID)
	}

	// An effect still open at the end of the log ends with its announced
	// duration when it has one.
	for _, f := range tl.effects {
		if !f.closed && f.Duration > 0 {
			f.Interval.End = min(f.Interval.Start+f.Duration, tl.Duration)
		}
	}
}

// newEffect creates an effect from its creation event.
func (b *builder) newEffect(e *evtc.Event, t time.Duration, src *Agent) {
	f := &b.effectArena[b.effectIdx]
	b.effectIdx++
	*f = Effect{
		ID:       trackableID(e),
		EffectID: e.SkillID,
		Agent:    src,
		Ground:   e.IsStateChange == evtc.StateEffectGroundCreate,
		Scale:    1,
		Duration: effectDuration(e),
		Interval: Interval{Start: t, End: b.tl.Duration},
		Create:   e,
	}
	f.GUID = b.tl.GUID(ContentEffect, f.EffectID)
	if f.Duration == 0 {
		f.Duration = b.tl.effectDefaults[f.EffectID]
	}
	if f.Ground {
		f.Origin, f.Orientation = groundEffectPlace(e)
		f.Scale = effectScale(e)
		f.MovingPlatform = e.IsFlanking != 0
		f.Flags = uint8(e.IsBuffRemove)
	}
	if f.ID == 0 {
		// An effect without a trackable id is never removed by an event: it
		// lasts its announced duration, or is instantaneous.
		f.Interval.End = min(t+f.Duration, b.tl.Duration)
		f.closed = true
	} else {
		if prev := b.openEffects[f.ID]; prev != nil {
			b.closeEffect(prev, nil, t)
		}
		b.openEffects[f.ID] = f
	}
	src.effects = append(src.effects, f)
	b.tl.effects = append(b.tl.effects, f)
}

// closeEffect ends an effect at t. A nil event marks an effect ended by
// the reuse of its id.
func (b *builder) closeEffect(f *Effect, e *evtc.Event, t time.Duration) {
	f.Remove = e
	f.Interval.End = t
	f.closed = true
	delete(b.openEffects, f.ID)
}

// newMissile creates a missile from its creation event.
func (b *builder) newMissile(e *evtc.Event, t time.Duration, src *Agent) {
	m := &b.missileArena[b.missileIdx]
	ord := b.missileIdx
	b.missileIdx++
	*m = Missile{
		ID:       trackableID(e),
		Skill:    b.tl.skills[e.SkillID],
		Owner:    src,
		Origin:   missileOrigin(e),
		Skin:     e.OverstackValue,
		Interval: Interval{Start: t, End: b.tl.Duration},
		Create:   e,
	}
	m.Launches = carve(&b.launchArena, b.missileLaunches[ord])
	m.Effects = carve(&b.missileEffectArena, b.missileEffects[ord])
	if prev := b.openMissiles[m.ID]; prev != nil {
		b.closeMissile(prev, nil, t)
	}
	b.openMissiles[m.ID] = m
	src.missiles = append(src.missiles, m)
	m.Skill.missiles = append(m.Skill.missiles, m)
	b.tl.missiles = append(b.tl.missiles, m)
}

// closeMissile ends a missile at t. A nil event marks a missile ended by
// the reuse of its id.
func (b *builder) closeMissile(m *Missile, e *evtc.Event, t time.Duration) {
	m.Remove = e
	m.Interval.End = t
	if e != nil {
		m.FriendlyFire = e.Value
		m.HitEnemy = e.IsFlanking != 0
		m.RemovedAt = missileRemovePlace(e)
	}
	delete(b.openMissiles, m.ID)
}

// defianceHits returns the defiance bar hits received by a within iv.
func defianceHits(a *Agent, iv Interval) Hits {
	return Hits{From(narrow(a.hitsTaken, hitTime, iv))}.Defiance()
}

// cause finds the hit with the given result received by a around t.
func cause(a *Agent, t time.Duration, r evtc.Result) *Hit {
	window := Interval{Start: t - CauseWindow, End: t + CauseWindow}
	var best *Hit
	var bestDist time.Duration
	for _, h := range narrow(a.hitsTaken, hitTime, window) {
		if h.Result != r {
			continue
		}
		dist := max(h.Time-t, t-h.Time)
		if best == nil || dist < bestDist {
			best, bestDist = h, dist
		}
	}
	return best
}

// rel converts a raw event time to a timeline time.
func (tl *Timeline) rel(raw uint64) time.Duration {
	return time.Duration(int64(raw)-int64(tl.epoch)) * time.Millisecond
}

// minionHits counts the hits dealt by the minions of a.
func minionHits(a *Agent) int {
	n := 0
	for _, m := range a.Minions {
		n += len(m.hits)
	}
	return n
}
