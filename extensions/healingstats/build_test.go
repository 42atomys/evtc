package healingstats

import (
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

func TestDecodeFields(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(1000, addrAlpha, addrBravo, skillHeal, 500, fromSrc)
	b.tick(2000, addrBravo, addrAlpha, skillRegen, 130, fromDst|flagDowned|flagArcDowned)
	b.barrier(3000, addrAlpha, addrAlpha, skillSand, 800, both)
	b.heal(3500, addrAlpha, addrCharlie, skillHeal, 100, fromSrc|flagArcDowned)
	b.heal(4000, 0, addrAlpha, 4242, 50, fromDst) // unknown source, a skill the table does not have
	b.add(signed(evtc.Event{Time: b.at(4500), SrcAgent: addrAlpha, DstAgent: addrBravo, SkillID: skillHeal, Value: -10, IsOffcycle: fromSrc, IsNinety: 1, IsFifty: 1, IsMoving: 3, IFF: evtc.IFFFoe, IsStateChange: evtc.StateExtensionCombat}))
	s := mustBuild(t, b.build(10000))
	alpha, bravo, charlie := player(t, s, addrAlpha), player(t, s, addrBravo), player(t, s, addrCharlie)

	heals := s.Heals().All()
	if len(heals) != 6 {
		t.Fatalf("heals = %d", len(heals))
	}
	h := heals[0]
	if h.Time != time.Second || h.Src != alpha || h.Dst != bravo || h.Skill == nil || h.Skill.Name != "Shelter" || h.Amount != 500 || h.IsBuff || h.IsBarrier || h.TargetDowned || !h.SrcRecorded || h.DstRecorded || h.Healed() != 500 || h.BarrierGiven() != 0 || h.PeerEvent != nil || !h.IsDirect() || !h.IsHealing() || h.Self() || h.Cast != nil || h.Event == nil {
		t.Errorf("direct heal = %+v", h)
	}
	if h = heals[1]; h.Time != 2*time.Second || h.Src != bravo || h.Dst != alpha || h.Amount != 130 || !h.IsBuff || h.IsBarrier || !h.TargetDowned || h.SrcRecorded || !h.DstRecorded || h.IsDirect() || h.Skill.ID != skillRegen {
		t.Errorf("tick = %+v", h)
	}
	if h = heals[2]; h.Amount != 800 || !h.IsBarrier || h.Healed() != 0 || h.BarrierGiven() != 800 || !h.Self() || !h.SrcRecorded || !h.DstRecorded || h.IsHealing() || h.Event.OverstackValue != 800 {
		t.Errorf("barrier = %+v", h)
	}
	if h = heals[3]; !h.TargetDowned || h.Dst != charlie || h.IsBuff {
		t.Errorf("downed direct heal = %+v", h)
	}
	if h = heals[4]; h.Src != s.Unknown || h.Dst != alpha || h.Skill == nil || h.Skill.Name != "4242" || h.Skill != s.Timeline.Skill(4242) || h.Credited() != s.Unknown {
		t.Errorf("heal from an unknown source = %+v", h)
	}
	if h = heals[5]; !h.OverNinety || !h.UnderFifty || !h.Moving || !h.TargetMoving || h.IFF != evtc.IFFFoe || h.Amount != 10 {
		t.Errorf("flags = %+v", h)
	}
	if q := s.Heals(); q.Amount() != 1590 || q.Healed() != 790 || q.BarrierGiven() != 800 || q.Count() != 6 {
		t.Errorf("sums: amount %d healed %d barrier %d", q.Amount(), q.Healed(), q.BarrierGiven())
	}
	if s.Merged != 0 || s.Timeline.ExtensionEvents().Count() != 6 || s.Extension.Events().Count() != 6 {
		t.Errorf("merged %d, events %d", s.Merged, s.Extension.Events().Count())
	}
	checkInvariants(t, s)
}

func TestMergePeers(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	// Bravo heals Alpha, the recording player: the record of Alpha's
	// client comes first, the copy shared by Bravo follows.
	b.heal(1000, addrBravo, addrAlpha, skillHeal, 300, fromDst)
	b.heal(1120, addrBravo, addrAlpha, skillHeal, 300, fromSrc)
	// Alpha heals Bravo: the record of Alpha's client is the source side.
	b.heal(2000, addrAlpha, addrBravo, skillHeal, 200, fromSrc)
	b.heal(2050, addrAlpha, addrBravo, skillHeal, 200, fromDst)
	// The shared copy can come first: the record of Alpha's client is
	// still the one kept.
	b.heal(2990, addrBravo, addrAlpha, skillHeal, 250, fromSrc)
	b.heal(3000, addrBravo, addrAlpha, skillHeal, 250, fromDst)
	// The shared copy comes late.
	b.heal(4000, addrBravo, addrAlpha, skillHeal, 260, fromDst)
	b.heal(4600, addrBravo, addrAlpha, skillHeal, 260, fromSrc)
	// The same side twice is two heals.
	b.heal(5000, addrAlpha, addrBravo, skillHeal, 270, fromSrc)
	b.heal(5010, addrAlpha, addrBravo, skillHeal, 270, fromSrc)
	// A multi-heal written by both clients pairs one to one.
	for range 3 {
		b.tick(6000, addrAlpha, addrBravo, skillRegen, 100, fromSrc)
	}
	for range 3 {
		b.tick(6100, addrAlpha, addrBravo, skillRegen, 100, fromDst)
	}
	// Different amounts never match. The client of Bravo writes the heals
	// Bravo deals: what Alpha alone tells of them is left out.
	b.heal(7000, addrBravo, addrAlpha, skillHeal, 280, fromDst)
	b.heal(7010, addrBravo, addrAlpha, skillHeal, 281, fromSrc)
	// A record carrying both flags never merges.
	b.heal(8000, addrBravo, addrBravo, skillHeal, 290, both)
	b.heal(8010, addrBravo, addrBravo, skillHeal, 290, fromDst)
	// Between two squad members, neither record comes from the recording
	// player: the earlier one is kept.
	b.heal(9000, addrBravo, addrCharlie, skillHeal, 310, fromSrc)
	b.heal(9100, addrBravo, addrCharlie, skillHeal, 310, fromDst)
	s := mustBuild(t, b.build(10000))

	heals := s.Heals().All()
	if s.Merged != 8 || len(heals) != 12 || len(s.LeftOut) != 2 || s.Extension.Events().Count() != 22 {
		t.Fatalf("merged %d, heals %d, left out %d, events %d", s.Merged, len(heals), len(s.LeftOut), s.Extension.Events().Count())
	}
	if at := []time.Duration{s.Timeline.TimeOf(s.LeftOut[0]), s.Timeline.TimeOf(s.LeftOut[1])}; at[0] != 7*time.Second || at[1] != 8010*time.Millisecond {
		t.Errorf("left out at %v", at)
	}
	at := func(h *Heal) []time.Duration {
		out := []time.Duration{h.Time, -1}
		if h.PeerEvent != nil {
			out[1] = s.Timeline.TimeOf(h.PeerEvent)
		}
		return out
	}
	for i, want := range []struct {
		event, peer time.Duration
		flags       uint8
	}{
		{1000 * time.Millisecond, 1120 * time.Millisecond, fromDst},
		{2000 * time.Millisecond, 2050 * time.Millisecond, fromSrc},
		{3000 * time.Millisecond, 2990 * time.Millisecond, fromDst},
		{4000 * time.Millisecond, 4600 * time.Millisecond, fromDst},
		{5000 * time.Millisecond, -1, fromSrc},
		{5010 * time.Millisecond, -1, fromSrc},
		{6000 * time.Millisecond, 6100 * time.Millisecond, fromSrc},
		{6000 * time.Millisecond, 6100 * time.Millisecond, fromSrc},
		{6000 * time.Millisecond, 6100 * time.Millisecond, fromSrc},
		{7010 * time.Millisecond, -1, fromSrc},
		{8000 * time.Millisecond, -1, both},
		{9000 * time.Millisecond, 9100 * time.Millisecond, fromSrc},
	} {
		h := heals[i]
		got := at(h)
		if got[0] != want.event || got[1] != want.peer || h.Event.IsOffcycle&both != want.flags {
			t.Errorf("heal %d: event at %v (flags %#x), peer at %v, want %v %#x %v", i, got[0], h.Event.IsOffcycle, got[1], want.event, want.flags, want.peer)
		}
		if merged := h.PeerEvent != nil; merged != (h.SrcRecorded && h.DstRecorded) && want.flags != both {
			t.Errorf("heal %d: merged %v, recorded %v %v", i, merged, h.SrcRecorded, h.DstRecorded)
		}
	}
	alpha, bravo := player(t, s, addrAlpha), player(t, s, addrBravo)
	if alpha.HealsTaken().Count() != 4 || alpha.HealsTaken().Healed() != 300+250+260+281 || bravo.HealsTaken().Count() != 7 || bravo.HealsTaken().Self().Count() != 1 {
		t.Errorf("alpha took %d heals for %d, bravo %d", alpha.HealsTaken().Count(), alpha.HealsTaken().Healed(), bravo.HealsTaken().Count())
	}
	checkInvariants(t, s)
}

// TestMergeLate checks the copies that come late: the closest records
// pair first, within a minute.
func TestMergeLate(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	// The copy of the first tick never came, the one of the second did at
	// once: it is not the copy of the first.
	b.tick(1000, addrAlpha, addrBravo, skillRegen, 130, fromSrc)
	b.tick(3000, addrAlpha, addrBravo, skillRegen, 130, fromSrc)
	b.tick(3100, addrAlpha, addrBravo, skillRegen, 130, fromDst)
	// Copies that come 3, 15 and 45 seconds late.
	b.heal(10000, addrAlpha, addrBravo, skillHeal, 301, fromSrc)
	b.heal(11000, addrAlpha, addrBravo, skillHeal, 302, fromSrc)
	b.heal(12000, addrAlpha, addrBravo, skillHeal, 303, fromSrc)
	b.heal(13000, addrAlpha, addrBravo, skillHeal, 301, fromDst)
	b.heal(26000, addrAlpha, addrBravo, skillHeal, 302, fromDst)
	b.heal(57000, addrAlpha, addrBravo, skillHeal, 303, fromDst)
	// Over a minute apart, the records tell two heals. The client of Bravo
	// writes the heals Bravo deals, so that the one Alpha tells alone is
	// left out.
	b.heal(60000, addrBravo, addrAlpha, skillHeal, 304, fromDst)
	b.heal(120001, addrBravo, addrAlpha, skillHeal, 304, fromSrc)
	// Two heals of one content, the copies of both late: each record takes
	// the closest one.
	b.heal(130000, addrBravo, addrCharlie, skillHeal, 305, fromSrc)
	b.heal(134000, addrBravo, addrCharlie, skillHeal, 305, fromSrc)
	b.heal(134400, addrBravo, addrCharlie, skillHeal, 305, fromDst)
	b.heal(136000, addrBravo, addrCharlie, skillHeal, 305, fromDst)
	s := mustBuild(t, b.build(140000))
	tl := s.Timeline

	type pair struct{ event, peer time.Duration }
	var got []pair
	for h := range s.Heals().Seq() {
		p := pair{h.Time, -1}
		if h.PeerEvent != nil {
			p.peer = tl.TimeOf(h.PeerEvent)
		}
		got = append(got, p)
	}
	want := []pair{
		{1 * time.Second, -1},
		{3 * time.Second, 3100 * time.Millisecond},
		{10 * time.Second, 13 * time.Second},
		{11 * time.Second, 26 * time.Second},
		{12 * time.Second, 57 * time.Second},
		{120001 * time.Millisecond, -1},
		{130 * time.Second, 136 * time.Second},
		{134 * time.Second, 134400 * time.Millisecond},
	}
	if !slices.Equal(got, want) {
		t.Errorf("heals = %v, want %v", got, want)
	}
	if s.Merged != 6 || len(s.LeftOut) != 1 || tl.TimeOf(s.LeftOut[0]) != time.Minute {
		t.Errorf("merged %d, left out %v", s.Merged, s.LeftOut)
	}
	checkInvariants(t, s)
}

// TestLeftOut checks the records of the client of a destination: they
// stand for a heal when the client of the source writes nothing.
func TestLeftOut(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	// Charlie runs no addon and nobody knows the source of the second
	// heal: what Alpha tells is all the log holds of them.
	b.heal(1000, addrCharlie, addrAlpha, skillHeal, 100, fromDst)
	b.heal(2000, 0, addrAlpha, skillHeal, 110, fromDst)
	b.heal(2500, 0, addrAlpha, skillHeal, 115, both)
	// Bravo shares its stats: its client tells the heals Bravo deals.
	b.heal(3000, addrBravo, addrCharlie, skillHeal, 120, fromSrc)
	b.heal(4000, addrBravo, addrAlpha, skillHeal, 130, fromDst)
	// The pet of Alpha heals Bravo and the client of Bravo alone tells it,
	// though the client of Alpha writes what the pet deals.
	b.minionHeal(5000, addrPet, instAlpha, addrCharlie, skillHeal, 140, fromSrc)
	b.minionHeal(6000, addrPet, instAlpha, addrBravo, skillHeal, 150, fromDst)
	s := mustBuild(t, b.build(10000))
	alpha, bravo, charlie, pet := player(t, s, addrAlpha), player(t, s, addrBravo), player(t, s, addrCharlie), player(t, s, addrPet)

	if got := amounts(s.Heals()); !slices.Equal(got, []int32{100, 110, 115, 120, 140}) {
		t.Errorf("heals = %v", got)
	}
	var left []int32
	for _, e := range s.LeftOut {
		left = append(left, -e.Value)
	}
	if !slices.Equal(left, []int32{130, 150}) || s.Merged != 0 {
		t.Errorf("left out = %v, merged %d", left, s.Merged)
	}
	if !alpha.writes || !bravo.writes || charlie.writes || !pet.writes || s.Unknown.writes {
		t.Errorf("writes: alpha %v bravo %v charlie %v pet %v unknown %v", alpha.writes, bravo.writes, charlie.writes, pet.writes, s.Unknown.writes)
	}
	// A record left out still tells who runs the addon.
	if !alpha.Recorded || !bravo.Recorded || charlie.Recorded || len(s.Recorded) != 2 {
		t.Errorf("recorded = %v", s.Recorded)
	}
	checkInvariants(t, s)

	// The record of a destination tells that its client runs the addon
	// when it is all that client wrote.
	b = fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(1000, addrAlpha, addrCharlie, skillHeal, 100, fromSrc)
	b.heal(2000, addrAlpha, addrBravo, skillHeal, 200, fromDst)
	s = mustBuild(t, b.build(10000))
	if s.Heals().Count() != 1 || len(s.LeftOut) != 1 || len(s.Recorded) != 2 || !player(t, s, addrBravo).Recorded {
		t.Errorf("heals %d, left out %d, recorded %v", s.Heals().Count(), len(s.LeftOut), s.Recorded)
	}
	checkInvariants(t, s)

	// A record that names a master nobody carries the instance id of is
	// the one of the client of its source.
	b = fixture()
	b.register(0, "2.19rc2", 2)
	b.add(signed(evtc.Event{Time: b.at(1000), SrcAgent: addrBravo, SrcMasterInstanceID: 999, DstAgent: addrCharlie, SkillID: skillHeal, Value: -100, IsOffcycle: fromSrc, IsStateChange: evtc.StateExtensionCombat}))
	b.heal(2000, addrBravo, addrAlpha, skillHeal, 200, fromDst)
	s = mustBuild(t, b.build(10000))
	if s.Heals().Count() != 1 || len(s.LeftOut) != 1 || len(s.Recorded) != 2 || !player(t, s, addrBravo).writes {
		t.Errorf("heals %d, left out %d, recorded %v", s.Heals().Count(), len(s.LeftOut), s.Recorded)
	}
	checkInvariants(t, s)
}

// TestLeftOutByMaster checks whose client wrote the heals of a minion:
// the one of the master the record names, whatever the master of the
// agent the addon gives the address of.
func TestLeftOutByMaster(t *testing.T) {
	const addrCrow, instCrow = 0x2004, 24
	b := fixture()
	b.register(0, "2.19rc2", 2)
	// A minion of Charlie, who runs no addon, lives two seconds.
	b.npc(addrCrow, instCrow, 1083, "Crow")
	b.add(evtc.Event{Time: b.at(1000), SrcAgent: addrCrow, SrcMasterInstanceID: instCharlie, IsStateChange: evtc.StateEnterCombat})
	b.state(3000, addrCrow, evtc.StateChangeDead)
	// A minion of Bravo takes its instance id, then a squad member out of
	// the view of Alpha: the addon names both by the address of the crow.
	b.add(signed(evtc.Event{Time: b.at(4000), SrcAgent: addrCrow, SrcMasterInstanceID: instBravo, DstAgent: addrAlpha, SkillID: skillHeal, Value: -100, IsOffcycle: fromSrc, IsStateChange: evtc.StateExtensionCombat}))
	b.heal(6000, addrCrow, addrCrow, skillHeal, 200, both)
	// What Alpha tells of the heals of Charlie is all the log holds of
	// them.
	b.heal(7000, addrCharlie, addrAlpha, skillHeal, 300, fromDst)
	// Bravo is known to write by its minion alone.
	b.heal(8000, addrBravo, addrAlpha, skillHeal, 400, fromDst)
	s := mustBuild(t, b.build(10000))
	bravo, charlie, crow := player(t, s, addrBravo), player(t, s, addrCharlie), player(t, s, addrCrow)

	if crow.Master != charlie.Agent {
		t.Fatalf("crow master = %v", crow.Master)
	}
	if got := amounts(s.Heals()); !slices.Equal(got, []int32{100, 200, 300}) {
		t.Errorf("heals = %v", got)
	}
	if len(s.LeftOut) != 1 || s.LeftOut[0].Value != -400 {
		t.Errorf("left out = %v", s.LeftOut)
	}
	if !bravo.writes || charlie.writes || !crow.writes {
		t.Errorf("writes: bravo %v charlie %v crow %v", bravo.writes, charlie.writes, crow.writes)
	}
	if !bravo.Recorded || charlie.Recorded || !crow.Recorded || len(s.Recorded) != 2 {
		t.Errorf("recorded: bravo %v charlie %v crow %v, %v", bravo.Recorded, charlie.Recorded, crow.Recorded, s.Recorded)
	}
	checkInvariants(t, s)
}

func TestMergeWithoutPOV(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(1010, addrBravo, addrAlpha, skillHeal, 300, fromDst)
	b.heal(1000, addrBravo, addrAlpha, skillHeal, 300, fromSrc)
	l := b.build(5000)
	// Without a point of view, the earlier record is kept.
	for i := range l.Events {
		if l.Events[i].IsStateChange == evtc.StatePointOfView {
			l.Events[i].IsStateChange = evtc.StateIdleEvent
		}
	}
	s := mustBuild(t, l)
	h := s.Heals().First()
	if s.Timeline.POV != nil || s.Merged != 1 || h == nil || h.Time != time.Second || h.Event.IsOffcycle != fromSrc || h.PeerEvent == nil || len(s.Recorded) != 2 {
		t.Errorf("heal = %+v, recorded %v", h, s.Recorded)
	}
	checkInvariants(t, s)
}

func TestRecorded(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(1000, addrCharlie, addrAlpha, skillHeal, 100, fromDst)                // Charlie's client wrote nothing
	b.heal(2000, addrBravo, addrCharlie, skillHeal, 100, fromSrc)                // Bravo's did
	b.minionHeal(3000, addrPet, instAlpha, addrCharlie, skillHeal, 100, fromSrc) // the pet of Alpha
	b.heal(4000, 0, addrAlpha, skillHeal, 100, both)                             // an unknown source never is
	s := mustBuild(t, b.build(10000))
	alpha, bravo, charlie, pet := player(t, s, addrAlpha), player(t, s, addrBravo), player(t, s, addrCharlie), player(t, s, addrPet)
	if pet.Master != alpha.Agent {
		t.Fatalf("pet master = %v", pet.Master)
	}
	if !alpha.Recorded || !bravo.Recorded || charlie.Recorded || !pet.Recorded || s.Unknown.Recorded {
		t.Errorf("recorded: alpha %v bravo %v charlie %v pet %v unknown %v", alpha.Recorded, bravo.Recorded, charlie.Recorded, pet.Recorded, s.Unknown.Recorded)
	}
	if len(s.Recorded) != 2 || s.Recorded[0] != alpha || s.Recorded[1] != bravo {
		t.Errorf("recorded = %v", s.Recorded)
	}
	checkInvariants(t, s)

	// The recording player is recorded without a single heal, since its
	// client registered the addon.
	b = fixture()
	b.register(0, "2.19rc2", 2)
	s = mustBuild(t, b.build(1000))
	if len(s.Recorded) != 1 || s.Recorded[0].Agent != s.Timeline.POV.Ref() || s.Heals().Count() != 0 || s.Merged != 0 {
		t.Errorf("recorded = %v, heals %d", s.Recorded, s.Heals().Count())
	}
	checkInvariants(t, s)
}

func TestCasts(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(900, addrAlpha, addrBravo, skillHeal, 10, fromSrc) // before any cast
	b.castStart(1000, addrAlpha, 0, skillHeal, 500)
	b.heal(1200, addrAlpha, addrBravo, skillHeal, 20, fromSrc)
	b.tick(1300, addrAlpha, addrBravo, skillRegen, 30, fromSrc) // another skill
	b.castStop(1500, addrAlpha, skillHeal, 500)
	b.heal(1800, addrAlpha, addrCharlie, skillHeal, 40, fromSrc) // after the stop, still the last cast
	b.castStart(2000, addrAlpha, 0, skillHeal, 500)
	b.heal(2000, addrAlpha, addrBravo, skillHeal, 50, fromSrc) // at the start of the second cast
	b.castStop(2500, addrAlpha, skillHeal, 500)
	b.heal(3000, addrBravo, addrAlpha, skillHeal, 60, fromDst) // Bravo never cast
	s := mustBuild(t, b.build(10000))
	alpha := player(t, s, addrAlpha)
	casts := alpha.Casts().All()
	if len(casts) != 2 {
		t.Fatalf("casts = %v", casts)
	}
	heals := s.Heals().All()
	want := []*timeline.Cast{nil, casts[0], nil, casts[0], casts[1], nil}
	for i, h := range heals {
		if h.Cast != want[i] {
			t.Errorf("heal %d (%d) has cast %v, want %v", i, h.Amount, h.Cast, want[i])
		}
	}
	if q := s.Heals().OfCast(casts[0]); q.Count() != 2 || q.Amount() != 60 || s.Heals().OfCast(nil).Count() != 0 || s.Heals().OfCast(casts[1]).First().Amount != 50 {
		t.Errorf("heals of the first cast = %v", q.All())
	}
	checkInvariants(t, s)
}

func TestCredited(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(1000, addrAlpha, addrBravo, skillHeal, 100, fromSrc)
	b.minionHeal(1500, addrPet, instAlpha, addrBravo, skillHeal, 50, fromSrc)
	b.heal(2000, addrAlpha, addrCharlie, skillHeal, 100, fromSrc)
	b.minionHeal(500, addrPet, instAlpha, addrCharlie, skillHeal, 25, fromSrc) // written late, happened first
	b.heal(3000, addrBravo, addrAlpha, skillHeal, 400, fromDst)
	s := mustBuild(t, b.build(10000))
	alpha, bravo, pet := player(t, s, addrAlpha), player(t, s, addrBravo), player(t, s, addrPet)
	if pet.Master != alpha.Agent {
		t.Fatalf("pet master = %v", pet.Master)
	}
	if alpha.Heals().Count() != 2 || alpha.HealsCredited().Count() != 4 || alpha.HealsCredited().Amount() != 275 || pet.Heals().Count() != 2 || pet.HealsCredited().Count() != 2 || bravo.HealsCredited().Count() != 1 {
		t.Errorf("alpha heals %d credited %d for %d, pet %d, bravo %d", alpha.Heals().Count(), alpha.HealsCredited().Count(), alpha.HealsCredited().Amount(), pet.HealsCredited().Count(), bravo.HealsCredited().Count())
	}
	times := alpha.HealsCredited().Map(func(h *Heal) time.Duration { return h.Time })
	if want := []time.Duration{500 * time.Millisecond, time.Second, 1500 * time.Millisecond, 2 * time.Second}; len(times) != 4 || times[0] != want[0] || times[1] != want[1] || times[2] != want[2] || times[3] != want[3] {
		t.Errorf("credited times = %v", times)
	}
	if h := s.Heals().First(); h.Src != pet || h.Credited() != alpha || !h.creditedTo(who{one: alpha.Agent}) || h.creditedTo(who{one: bravo.Agent}) {
		t.Errorf("first heal = %+v credited to %v", h, h.Credited())
	}
	if s.Heals().CreditedTo(alpha).Count() != 4 || s.Heals().By(alpha).Count() != 2 || s.Heals().CreditedTo(pet).Count() != 2 {
		t.Errorf("credited to alpha %d, by alpha %d", s.Heals().CreditedTo(alpha).Count(), s.Heals().By(alpha).Count())
	}
	shares := s.Heals().PerAgent()
	if len(shares) != 2 || shares[0].Agent != bravo || shares[0].Heals.Amount() != 400 || shares[1].Agent != alpha || shares[1].Heals.Count() != 4 {
		t.Errorf("PerAgent = %v", shares)
	}
	checkInvariants(t, s)
}

func TestRegistration(t *testing.T) {
	b := fixture()
	b.heal(500, addrAlpha, addrBravo, skillHeal, 100, fromSrc) // written before the registration
	b.register(1000, "2.17rc3", 1)
	b.heal(1500, addrAlpha, addrBravo, skillHeal, 100, fromSrc)
	s := mustBuild(t, b.build(5000))
	if s.Heals().Count() != 2 || s.Revision != 1 || s.Version != "2.17rc3" || s.Extension.Version != "2.17rc3" || s.Extension.Time != time.Second || s.Heals().First().Time != 500*time.Millisecond {
		t.Errorf("stats = %+v", s)
	}
	checkInvariants(t, s)

	// The version is cut at the eight bytes of the field.
	b = fixture()
	b.register(0, "10.0.0rc12", 2)
	if s = mustBuild(t, b.build(1000)); s.Version != "10.0.0rc" || s.Revision != SupportedRevision {
		t.Errorf("version %q revision %d", s.Version, s.Revision)
	}

	// A second registration owns nothing: the stats belong to the first.
	b = fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(500, addrAlpha, addrBravo, skillHeal, 100, fromSrc)
	b.register(1000, "dup", 2)
	s = mustBuild(t, b.build(5000))
	tl := s.Timeline
	if len(tl.Extensions) != 2 || s.Extension != tl.Extensions[0] || tl.Extensions[1].Decoded != nil || s.Heals().Count() != 1 {
		t.Errorf("registrations = %v, stats on %v", tl.Extensions, s.Extension)
	}
	checkInvariants(t, s)
}

func TestWithoutStats(t *testing.T) {
	b := fixture()
	b.heal(1000, addrAlpha, addrBravo, skillHeal, 100, fromSrc)
	tl, err := timeline.Build(b.build(5000))
	if err != nil {
		t.Fatal(err)
	}
	if Of(tl) != nil || tl.ExtensionEvents().Count() != 1 || Of(nil) != nil {
		t.Errorf("stats of a log without registration = %v", Of(tl))
	}
	var s *Stats
	if s.Heals().Count() != 0 || s.Heals().Healed() != 0 || s.Agent(tl.Players().First()) != nil || s.Heals().PerAgent() != nil || s.Agents().Any() || s.Players().Any() {
		t.Error("a nil Stats does not answer as an empty one")
	}
	var a *Agent
	if a.Heals().Count() != 0 || a.HealsTaken().Count() != 0 || a.HealsCredited().Count() != 0 || a.Ref() != nil {
		t.Error("a nil Agent does not answer as an empty one")
	}
}

func TestMasterFromRecords(t *testing.T) {
	const mech, instMech = 0x7d1, 77
	b := fixture()
	b.register(0, "2.19rc2", 2)
	// Bravo shares the heals of a mech the recording client does not
	// track. The first master a record names for it counts.
	b.add(signed(evtc.Event{Time: b.at(1000), SrcAgent: mech, SrcInstanceID: instMech, SrcMasterInstanceID: instBravo, DstAgent: addrCharlie, SkillID: skillHeal, Value: -100, IsOffcycle: fromSrc, IsStateChange: evtc.StateExtensionCombat}))
	b.add(signed(evtc.Event{Time: b.at(1500), SrcAgent: mech, SrcInstanceID: instMech, SrcMasterInstanceID: instAlpha, DstAgent: addrCharlie, SkillID: skillHeal, Value: -50, IsOffcycle: fromSrc, IsStateChange: evtc.StateExtensionCombat}))
	b.heal(2000, addrBravo, addrCharlie, skillHeal, 300, fromSrc)
	// A record naming Charlie under the instance id and master of the pet
	// gives Charlie no master.
	b.add(signed(evtc.Event{Time: b.at(3000), SrcAgent: addrBravo, DstAgent: addrCharlie, DstInstanceID: instPet, DstMasterInstanceID: instAlpha, SkillID: skillHeal, Value: -20, IsOffcycle: fromSrc, IsStateChange: evtc.StateExtensionCombat}))
	s := mustBuild(t, b.build(10000))
	alpha, bravo, charlie := player(t, s, addrAlpha), player(t, s, addrBravo), player(t, s, addrCharlie)
	m := player(t, s, mech)

	if m.Raw != nil || m.Master != nil || m.InstanceID != instMech || s.Timeline.AgentAt(instMech, time.Second) != nil || len(bravo.Minions) != 0 {
		t.Errorf("mech = %+v, minions of Bravo %v", m.Agent, bravo.Minions)
	}
	if charlie.Master != nil || charlie.master != nil || charlie.Recorded {
		t.Errorf("Charlie has the master %v or %v, recorded %v", charlie.Master, charlie.master, charlie.Recorded)
	}
	if h := s.Heals().First(); h.Src != m || h.Credited() != bravo || !h.creditedTo(who{one: bravo.Agent}) || h.creditedTo(who{one: alpha.Agent}) {
		t.Errorf("first heal by %v is credited to %v", h.Src, h.Credited())
	}
	if m.Heals().Count() != 2 || bravo.Heals().Count() != 2 || bravo.HealsCredited().Count() != 4 || bravo.HealsCredited().Amount() != 470 || s.Heals().CreditedTo(bravo).Count() != 4 || alpha.HealsCredited().Count() != 0 {
		t.Errorf("mech %d heals, Bravo %d and %d credited for %d", m.Heals().Count(), bravo.Heals().Count(), bravo.HealsCredited().Count(), bravo.HealsCredited().Amount())
	}
	if shares := s.Heals().PerAgent(); len(shares) != 1 || shares[0].Agent != bravo || shares[0].Heals.Count() != 4 {
		t.Errorf("PerAgent = %v", shares)
	}
	if !m.Recorded || !bravo.Recorded || len(s.Recorded) != 2 {
		t.Errorf("mech recorded %v, Bravo %v, recorded %v", m.Recorded, bravo.Recorded, s.Recorded)
	}
	checkInvariants(t, s)
}

func TestResolveSharedAddresses(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	// Shared by Bravo, the source untranslated: address 0, instance id of
	// Charlie.
	b.add(signed(evtc.Event{Time: b.at(1000), SrcInstanceID: instCharlie, DstAgent: addrBravo, SkillID: skillHeal, Value: -100, IsOffcycle: fromDst, IsStateChange: evtc.StateExtensionCombat}))
	// A destination address the log never declares, instance id of the pet.
	b.add(signed(evtc.Event{Time: b.at(2000), SrcAgent: addrBravo, DstAgent: 0xdead, DstInstanceID: instPet, SkillID: skillHeal, Value: -200, IsOffcycle: fromSrc, IsStateChange: evtc.StateExtensionCombat}))
	// An instance id nobody carries stays unknown.
	b.add(signed(evtc.Event{Time: b.at(3000), SrcInstanceID: 999, DstAgent: addrBravo, SkillID: skillHeal, Value: -300, IsOffcycle: fromDst, IsStateChange: evtc.StateExtensionCombat}))
	// A declared address wins over the instance id.
	b.add(signed(evtc.Event{Time: b.at(4000), SrcAgent: addrAlpha, SrcInstanceID: instCharlie, DstAgent: addrBravo, SkillID: skillHeal, Value: -400, IsOffcycle: fromSrc, IsStateChange: evtc.StateExtensionCombat}))
	s := mustBuild(t, b.build(10000))
	alpha, bravo, charlie, pet := player(t, s, addrAlpha), player(t, s, addrBravo), player(t, s, addrCharlie), player(t, s, addrPet)

	heals := s.Heals().All()
	if len(heals) != 4 {
		t.Fatalf("heals = %d", len(heals))
	}
	if heals[0].Src != charlie || heals[0].Dst != bravo {
		t.Errorf("untranslated source = %v -> %v", heals[0].Src, heals[0].Dst)
	}
	if heals[1].Src != bravo || heals[1].Dst != pet || heals[1].Credited() != bravo {
		t.Errorf("undeclared destination = %v -> %v", heals[1].Src, heals[1].Dst)
	}
	if heals[2].Src != s.Unknown || heals[3].Src != alpha {
		t.Errorf("unresolved source %v, declared source %v", heals[2].Src, heals[3].Src)
	}
	if charlie.Recorded || !bravo.Recorded || pet.HealsTaken().Amount() != 200 || charlie.Heals().Amount() != 100 {
		t.Errorf("charlie recorded %v, pet took %d, charlie healed %d", charlie.Recorded, pet.HealsTaken().Amount(), charlie.Heals().Amount())
	}
	checkInvariants(t, s)
}
