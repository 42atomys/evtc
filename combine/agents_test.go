package combine

import (
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// iv returns the interval between two times in milliseconds.
func iv(start, end int) timeline.Interval {
	return timeline.Interval{Start: time.Duration(start) * time.Millisecond, End: time.Duration(end) * time.Millisecond}
}

// ms returns times in milliseconds.
func ms(v ...int) []time.Duration {
	out := make([]time.Duration, len(v))
	for i, x := range v {
		out[i] = time.Duration(x) * time.Millisecond
	}
	return out
}

func TestSet(t *testing.T) {
	var s set
	s.add(0)
	s.add(63)
	if !s.has(0) || !s.has(63) || s.has(1) || s.has(62) {
		t.Errorf("set = %#x", uint64(s))
	}
}

func TestShares(t *testing.T) {
	tests := []struct {
		a, b timeline.Interval
		want bool
	}{
		{iv(0, 1000), iv(500, 2000), true},
		{iv(0, 1000), iv(900, 2000), true},
		{iv(0, 1000), iv(901, 2000), false}, // less than shared
		{iv(0, 1000), iv(1000, 2000), false},
		{iv(0, 1000), iv(1500, 2000), false},
		{iv(0, 1000), iv(400, 450), true}, // all of the shorter one
		{iv(400, 450), iv(0, 1000), true},
		{iv(0, 1000), iv(0, 1000), true},
	}
	for _, tt := range tests {
		if got := shares(tt.a, tt.b); got != tt.want {
			t.Errorf("shares(%v, %v) = %v", tt.a, tt.b, got)
		}
	}
}

func TestPresence(t *testing.T) {
	tests := []struct {
		name  string
		holds []hold
		want  []timeline.Interval
	}{
		{"nothing held", nil, nil},
		{"one entry", []hold{{rec: 0, held: []timeline.Interval{iv(0, 1000)}}}, []timeline.Interval{iv(0, 1000)}},
		{"two entries that overlap", []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 1000)}},
			{rec: 1, held: []timeline.Interval{iv(800, 2000)}},
		}, []timeline.Interval{iv(0, 2000)}},
		{"two clients that do not date alike", []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 1000)}},
			{rec: 1, held: []timeline.Interval{iv(1300, 2000)}},
		}, []timeline.Interval{iv(0, 2000)}},
		{"a gap longer than sameBound", []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 1000)}},
			{rec: 1, held: []timeline.Interval{iv(1501, 2000)}},
		}, []timeline.Interval{iv(0, 1000), iv(1501, 2000)}},
		{"a short gap one recording tells", []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 1000), iv(1200, 2000)}},
		}, []timeline.Interval{iv(0, 1000), iv(1200, 2000)}},
		{"a short gap another recording covers", []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 1000), iv(1200, 2000)}},
			{rec: 1, held: []timeline.Interval{iv(500, 1500)}},
		}, []timeline.Interval{iv(0, 2000)}},
	}
	for _, tt := range tests {
		if got := presence(tt.holds); !slices.Equal(got, tt.want) {
			t.Errorf("%s: presence = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSegments(t *testing.T) {
	var owners set
	owners.add(1)
	tests := []struct {
		name   string
		holds  []hold
		owners set
		want   []segment
	}{
		{"the longest view first", []hold{
			{rec: 0, entry: 1, track: iv(0, 3000)},
			{rec: 1, entry: 2, track: iv(1000, 9000)},
		}, 0, []segment{{iv(0, 999), 0, 1}, {iv(1000, 9000), 1, 2}}},
		{"views of the same length in the order of the recordings", []hold{
			{rec: 1, entry: 2, track: iv(1000, 4500)},
			{rec: 0, entry: 1, track: iv(0, 3000)},
		}, 0, []segment{{iv(0, 3000), 0, 1}, {iv(3001, 4500), 1, 2}}},
		{"the client of the player", []hold{
			{rec: 0, entry: 1, track: iv(0, 9000)},
			{rec: 1, entry: 2, track: iv(1000, 3000)},
		}, owners, []segment{{iv(0, 999), 0, 1}, {iv(1000, 3000), 1, 2}, {iv(3001, 9000), 0, 1}}},
		{"a view that blinks inside one that lasts", []hold{
			{rec: 0, entry: 1, track: iv(0, 9000)},
			{rec: 1, entry: 2, track: iv(1000, 3000)},
		}, 0, []segment{{iv(0, 9000), 0, 1}}},
		{"the same spawn dated some milliseconds apart", []hold{
			{rec: 0, entry: 1, track: iv(1000, 3000)},
			{rec: 1, entry: 2, track: iv(1040, 9000)},
		}, 0, []segment{{iv(1000, 9000), 1, 2}}},
	}
	for _, tt := range tests {
		if got := segments(tt.holds, tt.owners); !slices.Equal(got, tt.want) {
			t.Errorf("%s: segments = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestInner(t *testing.T) {
	x := &entity{
		spans: []timeline.Interval{iv(1000, 9000)},
		holds: []hold{
			{rec: 0, held: []timeline.Interval{iv(1000, 5000)}, spawns: ms(1000), leaves: ms(5000)},
			{rec: 1, held: []timeline.Interval{iv(3000, 9000)}, spawns: ms(3000), leaves: ms(9000)},
		},
	}
	tests := []struct {
		at    int
		spawn bool
		want  bool
	}{
		{1000, true, false},  // the start of the span
		{1400, true, false},  // within sameBound of it
		{3000, true, true},   // the second client gains the agent
		{5000, false, true},  // the first one loses it
		{8600, false, false}, // within sameBound of the end
		{9000, false, false},
	}
	for _, tt := range tests {
		if got := x.inner(time.Duration(tt.at)*time.Millisecond, tt.spawn); got != tt.want {
			t.Errorf("inner(%d ms, spawn %v) = %v", tt.at, tt.spawn, got)
		}
	}
	// A bound every entry that held the agent wrote is told.
	agreed := &entity{
		spans: []timeline.Interval{iv(0, 9000)},
		holds: []hold{
			{rec: 0, held: []timeline.Interval{iv(0, 4000), iv(4000, 9000)}, spawns: ms(4000), leaves: ms(4000)},
			{rec: 1, held: []timeline.Interval{iv(0, 4100), iv(4100, 9000)}, spawns: ms(4100), leaves: ms(4100)},
		},
	}
	if agreed.inner(4000*time.Millisecond, false) || agreed.inner(4100*time.Millisecond, true) {
		t.Error("a bound the recordings agree on is dropped")
	}
	if (&entity{}).inner(0, true) {
		t.Error("a bound of an agent nobody held is inside its life")
	}
}

func TestSpeaker(t *testing.T) {
	x := &entity{speak: []segment{{iv(0, 999), 0, 1}, {iv(1000, 3000), 1, 2}}}
	if s := x.speaker(500 * time.Millisecond); s == nil || s.entry != 1 {
		t.Errorf("speaker at 500ms = %v", s)
	}
	if s := x.speaker(4 * time.Second); s != nil {
		t.Errorf("speaker at 4s = %v", s)
	}
	if !x.speaks(2, 2*time.Second) || x.speaks(1, 2*time.Second) || !x.speaks(1, 4*time.Second) {
		t.Error("an entry speaks when it holds the instant, or when nobody does")
	}
}

// agentsNamed returns the agents of a timeline that carry a name.
func agentsNamed(tl *timeline.Timeline, name string) []*timeline.Agent {
	return tl.Agents().Where(func(a *timeline.Agent) bool { return a.Name == name }).All()
}

// TestJoinAcrossViews checks that an agent two clients held at other
// times is one agent, held from the first spawn to the last leave.
func TestJoinAcrossViews(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	a.npc(crow, 1083, "Crow")
	b.npc(crow, 1083, "Crow")
	a.spawn(1000, crow)
	for ts := uint64(1000); ts <= 3000; ts += 100 {
		a.position(ts, crow, float32(ts), 0)
	}
	a.despawn(3000, crow)
	b.spawn(2000, crow)
	for ts := uint64(2000); ts < 4000; ts += 100 {
		b.position(ts+1, crow, float32(ts), 5)
	}
	b.despawn(4000, crow)
	l, tl, m := audited(t, s.logs(5000)...)

	crows := agentsNamed(tl, "Crow")
	if len(crows) != 1 {
		t.Fatalf("%d crows in the combined log", len(crows))
	}
	c := crows[0]
	if c.Lifetime != iv(1000, 4000) || c.Addr != a.addr(crow) || c.InstanceID != crow {
		t.Errorf("crow %#x lives over %v", c.Addr, c.Lifetime)
	}
	if got := count(l, evtc.StateSpawn); got != 1 {
		t.Errorf("%d spawns, want the first one", got)
	}
	if got := of(l, evtc.StateDespawn); len(got) != 1 || got[0].Time != a.at(4000) {
		t.Errorf("despawns = %+v, want the last one", got)
	}
	// The first client speaks over what it held, the second one after.
	if got, want := c.Position.Len(), 21+10; got != want {
		t.Errorf("%d position samples, want %d", got, want)
	}
	if p := c.PositionAt(2500 * time.Millisecond); p.Y != 0 {
		t.Errorf("position at 2.5s = %v, want the one of the first client", p)
	}
	if p := c.PositionAt(3500 * time.Millisecond); p.Y != 5 {
		t.Errorf("position at 3.5s = %v, want the one of the second client", p)
	}
	x := m.recs[0].entity[m.recs[0].tl.Agent(a.addr(crow))]
	if x == nil || x != m.recs[1].entity[m.recs[1].tl.Agent(b.addr(crow))] || len(x.holds) != 2 {
		t.Fatalf("the two entries stand for %v", x)
	}
	if !slices.Equal(x.spans, []timeline.Interval{iv(1000, 4000)}) {
		t.Errorf("spans = %v", x.spans)
	}
}

// TestJoinInstanceReused checks that the agents that carried one instance
// id one after the other stay apart.
func TestJoinInstanceReused(t *testing.T) {
	s := newScene(2)
	for i, r := range s.recs {
		// Each client dates what it sees some milliseconds apart.
		d := uint64(i * 20)
		r.npc(crow, 1083, "Crow")
		r.l.Agents = append(r.l.Agents, evtc.Agent{Addr: r.base | 0x100 | crow, Profession: 1084, IsElite: 0xffffffff, Name: "Rat"})
		r.spawn(1000+d, crow)
		r.position(1100+d, crow, 10, 10)
		r.hit(1500, alpha, crow, skillSlam, 111)
		r.despawn(2000+d, crow)
		rat := evtc.Event{SrcAgent: r.base | 0x100 | crow, SrcInstanceID: crow}
		for _, e := range []struct {
			t    uint64
			kind evtc.StateChange
		}{{2500 + d, evtc.StateSpawn}, {2600 + d, evtc.StateEnterCombat}, {3500 + d, evtc.StateDespawn}} {
			ev := rat
			ev.Time, ev.IsStateChange = r.at(e.t), e.kind
			r.l.Events = append(r.l.Events, ev)
		}
	}
	_, tl, _ := audited(t, s.logs(5000)...)
	crows, rats := agentsNamed(tl, "Crow"), agentsNamed(tl, "Rat")
	if len(crows) != 1 || len(rats) != 1 {
		t.Fatalf("%d crows and %d rats", len(crows), len(rats))
	}
	if crows[0].Lifetime != iv(1000, 2020) || rats[0].Lifetime != iv(2500, 3520) {
		t.Errorf("crow over %v, rat over %v", crows[0].Lifetime, rats[0].Lifetime)
	}
	if tl.Hits().On(crows[0]).Count() != 1 {
		t.Errorf("%d hits on the crow", tl.Hits().On(crows[0]).Count())
	}
}

// TestJoinSpecies checks that two creatures of other species are not
// joined by the instance id they share.
func TestJoinSpecies(t *testing.T) {
	s := newScene(2)
	s.recs[0].npc(crow, 1083, "Crow")
	s.recs[1].npc(crow, 1084, "Rat")
	s.each(func(r *recorder) {
		r.spawn(1000, crow)
		r.position(1100, crow, 10, 10)
		r.despawn(3000, crow)
	})
	_, tl, _ := audited(t, s.logs(5000)...)
	if len(agentsNamed(tl, "Crow")) != 1 || len(agentsNamed(tl, "Rat")) != 1 {
		t.Errorf("crows %d, rats %d", len(agentsNamed(tl, "Crow")), len(agentsNamed(tl, "Rat")))
	}
}

// TestJoinPlayers checks that players are joined by their account, under
// the address of the first recording, and that a player only another
// recording lists is added.
func TestJoinPlayers(t *testing.T) {
	s := newScene(2)
	b := s.recs[1]
	b.player(14, "Delta", ":Delta.3456", "2")
	b.ev(1000, evtc.StateEnterCombat, 14, 0, evtc.Event{})
	b.hit(1500, 14, boss, skillSlam, 321)
	l, tl, _ := audited(t, s.logs(5000)...)
	if tl.Players().Count() != 4 || len(l.Agents) != 5 {
		t.Fatalf("%d players, %d agents", tl.Players().Count(), len(l.Agents))
	}
	for _, name := range []string{"Alpha", "Bravo", "Charlie"} {
		p := tl.PlayerByName(name)
		if p == nil || p.Ref().Addr>>32 != 1 {
			t.Errorf("%s = %v", name, p)
		}
	}
	d := tl.PlayerByName("Delta")
	if d == nil || d.Ref().Addr != b.addr(14) || tl.Hits().By(d).Count() != 1 {
		t.Errorf("Delta = %v", d)
	}
}

// TestJoinWithoutInstance checks the agents without instance id that
// share a name: nothing tells them apart from a recording to the other,
// and the first one that holds some keeps its own.
func TestJoinWithoutInstance(t *testing.T) {
	s := newScene(2)
	s.each(func(r *recorder) {
		for i := range 2 {
			addr := r.base | uint64(0x500+i)
			r.l.Agents = append(r.l.Agents, evtc.Agent{Addr: addr, Profession: 0xffff0000 | 7307, IsElite: 0xffffffff, Name: "Turret"})
			r.l.Events = append(r.l.Events, evtc.Event{Time: r.at(uint64(1000 + 100*i)), SrcAgent: addr, DstAgent: 1, IsStateChange: evtc.StateTargetable})
		}
	})
	l, tl, m := audited(t, s.logs(5000)...)
	if got := len(agentsNamed(tl, "Turret")); got != 2 {
		t.Errorf("%d turrets, want those of the first recording", got)
	}
	if got := count(l, evtc.StateTargetable); got != 2 {
		t.Errorf("%d events of the turrets", got)
	}
	if len(m.recs[0].unnamed) != 0 || len(m.recs[1].unnamed) != 2 {
		t.Errorf("entries left out: %d and %d", len(m.recs[0].unnamed), len(m.recs[1].unnamed))
	}

	// One per recording is the same agent.
	s = newScene(2)
	s.each(func(r *recorder) {
		addr := r.base | 0x500
		r.l.Agents = append(r.l.Agents, evtc.Agent{Addr: addr, Profession: 0xffff0000 | 7307, IsElite: 0xffffffff, Name: "Turret"})
		r.l.Events = append(r.l.Events, evtc.Event{Time: r.at(1000), SrcAgent: addr, DstAgent: 1, IsStateChange: evtc.StateTargetable})
	})
	l, tl, _ = audited(t, s.logs(5000)...)
	if len(agentsNamed(tl, "Turret")) != 1 || count(l, evtc.StateTargetable) != 1 {
		t.Errorf("%d turrets, %d events", len(agentsNamed(tl, "Turret")), count(l, evtc.StateTargetable))
	}
}

// TestJoinStill checks that an agent that never moved, met by two clients
// at other times, is told by where it stands.
func TestJoinStill(t *testing.T) {
	for _, tt := range []struct {
		x    float32
		want int
	}{{100, 1}, {900, 2}} {
		s := newScene(2)
		a, b := s.recs[0], s.recs[1]
		a.gadget(cannon, 7307, "Cannon")
		b.gadget(cannon, 7307, "Cannon")
		a.position(1000, cannon, 100, 100)
		a.despawn(1500, cannon)
		b.spawn(3000, cannon)
		b.position(3001, cannon, tt.x, 100)
		_, tl, _ := audited(t, s.logs(5000)...)
		if got := len(agentsNamed(tl, "Cannon")); got != tt.want {
			t.Errorf("met at %v: %d cannons, want %d", tt.x, got, tt.want)
		}
	}
}

// TestMasters checks that the events of a minion get the master another
// recording names.
func TestMasters(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	a.npc(pet, 6524, "Elemental")
	b.npc(pet, 6524, "Elemental")
	s.each(func(r *recorder) { r.spawn(1000, pet) })
	// The client of the master knows whose minion it is.
	a.ev(1500, evtc.StateCombat, pet, boss, evtc.Event{SkillID: skillSlam, Value: 50, SrcMasterInstanceID: alpha, IFF: evtc.IFFFoe})
	b.hit(1500, pet, boss, skillSlam, 50)
	b.hit(2500, pet, boss, skillSlam, 60)
	b.hit(2600, boss, pet, skillSlam, 70)
	// A master the second client lists and never met.
	b.npc(50, 6000, "Golem")
	b.ev(100, evtc.StateCombat, crow, boss, evtc.Event{SkillID: skillSlam, Value: 80, SrcMasterInstanceID: 50, IFF: evtc.IFFFoe})
	s.each(func(r *recorder) { r.despawn(3000, pet) })
	l, tl, _ := audited(t, s.logs(5000)...)
	pets := agentsNamed(tl, "Elemental")
	if len(pets) != 1 || pets[0].Master == nil || pets[0].Master.Name != "Alpha" {
		t.Fatalf("pets = %v", pets)
	}
	if on := strikesOf(l, skillSlam); len(on) != 4 || on[3].DstMasterInstanceID != alpha || on[0].SrcMasterInstanceID != 50 {
		t.Errorf("strikes = %+v", on)
	}
	hits := tl.Hits().By(pets[0]).All()
	if len(hits) != 2 {
		t.Fatalf("%d hits of the pet", len(hits))
	}
	for _, h := range hits {
		if h.Event.SrcMasterInstanceID != alpha {
			t.Errorf("hit at %v names the master %d", h.Time, h.Event.SrcMasterInstanceID)
		}
	}
	if got := tl.Hits().CreditedTo(tl.PlayerByName("Alpha")).Where(func(h *timeline.Hit) bool { return h.Src == pets[0] }).Count(); got != 2 {
		t.Errorf("%d hits of the pet credited to its master", got)
	}
}

// TestMasterOf checks the master a recording names for an agent: in the
// events that name the agent by its address, as their source or as their
// destination.
func TestMasterOf(t *testing.T) {
	s := newScene(1)
	r := s.recs[0]
	r.npc(pet, 6524, "Elemental")
	r.npc(crow, 1083, "Crow")
	r.spawn(1000, pet)
	r.spawn(1000, crow)
	// The heal of an addon names no master for the recording.
	r.heal(1200, pet, alpha, skillRegen, 100, 0x80)
	r.ev(1500, evtc.StateCombat, boss, pet, evtc.Event{SkillID: skillSlam, Value: 50, DstMasterInstanceID: alpha, IFF: evtc.IFFFoe})
	// An event written without address names the crow by its instance id
	// alone.
	r.l.Events = append(r.l.Events, evtc.Event{Time: r.at(1600), SrcInstanceID: crow, SrcMasterInstanceID: bravo, IsStateChange: evtc.StateDespawn})
	rec := mustRecord(t, s.logs(5000)...)[0]
	for _, tt := range []struct {
		name string
		want string
	}{{"Elemental", "Alpha"}, {"Crow", ""}, {"Alpha", ""}, {"Sabetha", ""}} {
		a := agentsNamed(rec.tl, tt.name)[0]
		got := ""
		if of := masterOf(rec, a); of != nil {
			got = of.Name
		}
		if got != tt.want {
			t.Errorf("master of %s = %q, want %q", tt.name, got, tt.want)
		}
	}
	if c := agentsNamed(rec.tl, "Crow")[0]; c.Master == nil {
		t.Error("the timeline gives the crow no master: the test checks nothing")
	}
}

// TestMastersLeftOut checks the events a client that knows the master of
// a minion writes without it, as it does when the minion comes: they stay
// as they were written, whatever the other clients tell.
func TestMastersLeftOut(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) {
		r.npc(pet, 6524, "Elemental")
		r.spawn(1000, pet)
		r.hit(1000, pet, boss, skillSlam, 50)
	})
	a.ev(1500, evtc.StateCombat, pet, boss, evtc.Event{SkillID: skillSlam, Value: 60, SrcMasterInstanceID: alpha, IFF: evtc.IFFFoe})
	b.hit(1500, pet, boss, skillSlam, 60)
	b.hit(2500, pet, boss, skillSlam, 70)
	// A strike the second client tells alone before the first one names
	// the master gets it: the master of a minion is the same over its
	// life.
	b.hit(1200, pet, boss, skillSlam, 55)
	s.each(func(r *recorder) { r.despawn(3000, pet) })
	for _, logs := range [][]*evtc.Log{s.logs(5000), {s.recs[1].l, s.recs[0].l}} {
		l, _, _ := audited(t, logs...)
		var got []uint16
		for _, e := range strikesOf(l, skillSlam) {
			got = append(got, e.SrcMasterInstanceID)
		}
		if !slices.Equal(got, []uint16{0, alpha, alpha, alpha}) {
			t.Errorf("masters of the strikes = %v", got)
		}
	}
}

// TestMastersListedAnew checks the master of a minion a client lists
// anew: the entry that names the master gives it to the events of the
// entries that never name it, those told by another client included.
func TestMastersListedAnew(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) {
		r.npc(pet, 6524, "Elemental")
		r.spawn(1000, pet)
		r.hit(1500, pet, boss, skillSlam, 50)
	})
	a.despawn(2000, pet)
	// The first client lists the minion anew, under another address, and
	// learns whose it is this time.
	again := a.base | 0x10000 | uint64(pet)
	a.l.Agents = append(a.l.Agents, evtc.Agent{Addr: again, Profession: 6524, IsElite: 0xffffffff, Name: "Elemental"})
	a.l.Events = append(a.l.Events,
		evtc.Event{Time: a.at(3000), SrcAgent: again, SrcInstanceID: pet, IsStateChange: evtc.StateSpawn},
		evtc.Event{Time: a.at(3500), SrcAgent: again, SrcInstanceID: pet, SrcMasterInstanceID: alpha, DstAgent: a.addr(boss), DstInstanceID: boss, SkillID: skillSlam, Value: 60, IFF: evtc.IFFFoe},
		evtc.Event{Time: a.at(4000), SrcAgent: again, SrcInstanceID: pet, IsStateChange: evtc.StateDespawn})
	b.hit(2500, pet, boss, skillSlam, 55)
	b.hit(3500, pet, boss, skillSlam, 60)
	b.despawn(4000, pet)
	for _, logs := range [][]*evtc.Log{s.logs(5000), {b.l, a.l}} {
		l, tl, _ := audited(t, logs...)
		var got []uint16
		for _, e := range strikesOf(l, skillSlam) {
			got = append(got, e.SrcMasterInstanceID)
		}
		if !slices.Equal(got, []uint16{alpha, alpha, alpha}) {
			t.Errorf("masters of the strikes = %v", got)
		}
		if pets := agentsNamed(tl, "Elemental"); len(pets) != 1 || pets[0].Master == nil || pets[0].Master.Name != "Alpha" {
			t.Errorf("pets = %v", pets)
		}
	}
}

// TestMastersNamed checks that the master of an agent is one a recording
// names in the events of that agent. A player has none, and an event of an
// extension, which may name the address of another agent, gives none.
func TestMastersNamed(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) {
		r.npc(crow, 1083, "Crow")
		r.spawn(1000, crow)
		r.hit(1500, crow, boss, skillSlam, 50)
		r.despawn(4000, crow)
	})
	a.hit(2000, bravo, boss, skillSlam, 60)
	a.hit(2100, crow, boss, skillSlam, 70)
	// The addon of the second client names the address of a player, then
	// of the crow, under the instance id of a minion of alpha.
	b.ev(3000, evtc.StateExtensionCombat, alpha, 0, withID(evtc.Event{DstAgent: b.addr(bravo), DstInstanceID: pet, DstMasterInstanceID: alpha, SkillID: skillRegen, Value: -900, IsOffcycle: 0x40}, healing))
	b.ev(3100, evtc.StateExtensionCombat, alpha, 0, withID(evtc.Event{DstAgent: b.addr(crow), DstInstanceID: pet, DstMasterInstanceID: alpha, SkillID: skillRegen, Value: -800, IsOffcycle: 0x40}, healing))
	l, _, _ := audited(t, s.logs(5000)...)
	for _, e := range strikesOf(l, skillSlam) {
		if e.SrcMasterInstanceID != 0 {
			t.Errorf("the strike of %d names the master %d", e.SrcInstanceID, e.SrcMasterInstanceID)
		}
	}
	if got := len(strikesOf(l, skillSlam)); got != 3 {
		t.Errorf("%d strikes", got)
	}
}

func TestSameIdentity(t *testing.T) {
	player := func(name, account string, inst uint16) *timeline.Agent {
		return &timeline.Agent{Kind: timeline.KindPlayer, Name: name, InstanceID: inst, Player: &timeline.Player{Account: account}}
	}
	npc := func(name string, inst, species uint16) *timeline.Agent {
		return &timeline.Agent{Kind: timeline.KindNPC, Name: name, InstanceID: inst, SpeciesID: species}
	}
	gadget := func(name string, inst, model uint16) *timeline.Agent {
		return &timeline.Agent{Kind: timeline.KindGadget, Name: name, InstanceID: inst, SpeciesID: model}
	}
	tests := []struct {
		name string
		a, b *timeline.Agent
		want bool
	}{
		{"a player seen twice", player("Alpha", "Alpha.1234", 11), player("Alpha", "Alpha.1234", 12), true},
		{"two characters of an account", player("Alpha", "Alpha.1234", 11), player("Beta", "Alpha.1234", 11), false},
		{"two accounts", player("Alpha", "Alpha.1234", 11), player("Alpha", "Other.5678", 11), false},
		{"a player without its node", player("Alpha", "Alpha.1234", 11), &timeline.Agent{Kind: timeline.KindPlayer, Name: "Alpha"}, false},
		{"players of a public instance", player("Guardian", "", 11), player("Guardian", "", 11), true},
		{"two players of a public instance", player("Guardian", "", 11), player("Guardian", "", 12), false},
		{"a player and a creature", player("Alpha", "Alpha.1234", 11), npc("Alpha", 11, 0), false},
		{"a creature seen twice", npc("Crow", 22, 1083), npc("Corbeau", 22, 1083), true},
		{"two creatures of one instance id", npc("Crow", 22, 1083), npc("Rat", 22, 1084), false},
		{"two instance ids", npc("Crow", 22, 1083), npc("Crow", 23, 1083), false},
		{"a gadget seen twice", gadget("Cannon", 31, 7307), gadget("Cannon", 31, 7308), true},
		{"agents without instance id", gadget("Turret", 0, 7307), gadget("Turret", 0, 7307), true},
		{"agents without instance id of other names", gadget("Turret", 0, 7307), gadget("Cannon", 0, 7307), false},
		{"agents without instance id of other models", gadget("Turret", 0, 7307), gadget("Turret", 0, 7308), false},
	}
	for _, tt := range tests {
		if got := sameIdentity(tt.a, tt.b); got != tt.want {
			t.Errorf("%s: %v", tt.name, got)
		}
		if got := sameIdentity(tt.b, tt.a); got != tt.want {
			t.Errorf("%s, the other way round: %v", tt.name, got)
		}
	}
}

func TestHoldOfEntry(t *testing.T) {
	x := &entity{holds: []hold{{entry: 3, held: []timeline.Interval{iv(1000, 2000)}, spawns: ms(1000)}}}
	if x.hold(3) == nil || x.hold(4) != nil {
		t.Error("hold finds the entry given alone")
	}
	for _, tt := range []struct {
		entry           int32
		at              int
		started, gained bool
	}{
		{3, 1000, true, true},
		{3, 1250, true, true},
		{3, 1251, false, false},
		{3, 999, false, false},
		{4, 1000, false, false},
	} {
		at := time.Duration(tt.at) * time.Millisecond
		if x.started(tt.entry, at) != tt.started || x.gained(tt.entry, at) != tt.gained {
			t.Errorf("entry %d at %d ms: started %v, gained %v", tt.entry, tt.at, x.started(tt.entry, at), x.gained(tt.entry, at))
		}
	}
	// Held from the start of the recordings without a spawn: the agent
	// was there, no client gained it.
	x = &entity{holds: []hold{{entry: 3, held: []timeline.Interval{iv(0, 2000)}}}}
	if !x.started(3, 100*time.Millisecond) || x.gained(3, 100*time.Millisecond) {
		t.Error("an agent held from the start was gained")
	}
	// A recording that started after another one gains what it holds.
	x.first = -2 * time.Second
	if !x.gained(3, 100*time.Millisecond) {
		t.Error("a recording that started later gained nothing")
	}
}

// TestSegmentsOfOneRecording checks the entries a recording split: they
// speak in the order they were held.
func TestSegmentsOfOneRecording(t *testing.T) {
	got := segments([]hold{
		{rec: 0, entry: 2, track: iv(3000, 5500)},
		{rec: 0, entry: 1, track: iv(0, 2900)},
	}, 0)
	want := []segment{{iv(0, 2900), 0, 1}, {iv(3000, 5500), 0, 2}}
	if !slices.Equal(got, want) {
		t.Errorf("segments = %v, want %v", got, want)
	}
}

func TestHeldOf(t *testing.T) {
	s := newScene(1)
	r := s.recs[0]
	r.npc(crow, 1083, "Crow")
	r.spawn(1000, crow)
	r.despawn(2000, crow)
	// A stray despawn, written for the instance id long after.
	r.l.Events = append(r.l.Events, evtc.Event{Time: r.at(2400), SrcInstanceID: crow, IsStateChange: evtc.StateDespawn})
	r.spawn(3000, crow)
	r.position(3100, crow, 1, 1)
	r.despawn(4000, crow)
	// The client told something of the agent after it left: it was there.
	r.position(4200, crow, 2, 2)
	r.despawn(4600, crow)
	recs := mustRecord(t, s.logs(6000)...)
	var n *node
	nodes := entries(recs)
	for i := range nodes {
		if nodes[i].agent.Name == "Crow" {
			n = &nodes[i]
		}
	}
	if n == nil {
		t.Fatal("no entry for the crow")
	}
	if want := []timeline.Interval{iv(1000, 2400), iv(3000, 4600)}; !slices.Equal(n.held, want) {
		t.Errorf("held = %v, want %v", n.held, want)
	}
	if n.track != iv(1000, 4600) || !slices.Equal(n.spawns, ms(1000, 3000)) || len(n.leaves) != 4 {
		t.Errorf("track %v, spawns %v, leaves %v", n.track, n.spawns, n.leaves)
	}

	// An entry whose track ends before it starts is held over its track.
	odd := &node{hold: hold{track: iv(3000, 2000)}}
	if got := heldOf(odd, recs[0], nil); !slices.Equal(got, []timeline.Interval{iv(3000, 2000)}) {
		t.Errorf("held = %v", got)
	}
}

// second writes an event of another agent that carries the instance id
// of the crow, under an address of its own.
func (r *recorder) second(t uint64, kind evtc.StateChange) {
	r.l.Events = append(r.l.Events, evtc.Event{Time: r.at(t), SrcAgent: r.base | 0x100 | crow, SrcInstanceID: crow, IsStateChange: kind})
}

// TestJoinSplitEntry checks the entries a recording split when another
// one went on holding the agent: they are one agent.
func TestJoinSplitEntry(t *testing.T) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	b.npc(crow, 1083, "Crow")
	b.spawn(1000, crow)
	b.position(1100, crow, 1, 1)
	b.position(3900, crow, 9, 9)
	b.despawn(4000, crow)
	// The first client lost the crow and found it again under another
	// address.
	a.npc(crow, 1083, "Crow")
	a.spawn(1010, crow)
	a.position(1200, crow, 1, 1)
	a.despawn(1400, crow)
	a.l.Agents = append(a.l.Agents, evtc.Agent{Addr: a.base | 0x100 | crow, Profession: 1083, IsElite: 0xffffffff, Name: "Crow"})
	a.second(2000, evtc.StateSpawn)
	a.second(2500, evtc.StateEnterCombat)
	a.second(3000, evtc.StateDespawn)
	l, tl, _ := audited(t, s.logs(5000)...)
	crows := agentsNamed(tl, "Crow")
	if len(crows) != 1 || crows[0].Lifetime != iv(1000, 4000) || crows[0].Addr != a.addr(crow) {
		t.Fatalf("crows = %v", crows)
	}
	if len(l.Agents) != 5 {
		t.Errorf("%d agents listed", len(l.Agents))
	}
	if count(l, evtc.StateSpawn) != 1 || count(l, evtc.StateDespawn) != 1 {
		t.Errorf("%d spawns, %d despawns", count(l, evtc.StateSpawn), count(l, evtc.StateDespawn))
	}
}
