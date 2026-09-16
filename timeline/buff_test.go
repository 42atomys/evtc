package timeline

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestStackActivity(t *testing.T) {
	b := fixture()
	b.buffApply(1000, addrP2, addrP1, skillBuff, 5000, 1)
	b.buffDeactive(2000, addrP1, 1, 4000)
	b.buffActive(3000, addrP1, 1, 4000)
	b.buffRemoveSingle(4000, addrP1, 0, skillBuff, 0, 1, evtc.BuffRemoveSingle)
	b.buffActive(4500, addrP1, 1, 100)
	b.buffApply(5000, addrP2, addrP1, skillBuff, 5000, 2)
	b.buffDeactive(5500, addrP1, 2, 1)
	b.buffApply(6000, addrP2, addrP1, skillBuff, 5000, 2)
	b.buffActive(7000, addrP1, 2, 3000)
	b.buffApply(8000, addrP2, addrP1, skillBurn, 1000, 9)
	b.buffDeactive(8500, addrP1, 9, 1)
	b.buffRemoveAll(9000, addrP1, 0, skillBurn)
	b.buffActive(9500, addrP1, 9, 1)
	tl := mustBuild(t, b.build(10000))
	p1 := tl.players[0]
	stacks := p1.Stacks().All()
	if len(stacks) != 4 {
		t.Fatalf("stacks = %d", len(stacks))
	}
	s1, s2, s3, s9 := stacks[0], stacks[1], stacks[2], stacks[3]
	if s1.Active.Len() != 3 || !s1.IsActiveAt(1500*msec) || s1.IsActiveAt(2500*msec) || !s1.IsActiveAt(3500*msec) || s1.IsActiveAt(4500*msec) || !s1.ActiveOnApply {
		t.Errorf("activity of stack 1 = %v", s1.Active.All())
	}
	if last, _ := s1.Active.Last(); last.End != 4*time.Second {
		t.Errorf("activity of stack 1 ends at %v", last.End)
	}
	if !s2.Superseded || s2.Active.Len() != 2 || s2.IsActiveAt(5700*msec) || !s2.IsActiveAt(5200*msec) {
		t.Errorf("activity of the superseded stack = %v", s2.Active.All())
	}
	if last, _ := s2.Active.Last(); last.End != 6*time.Second {
		t.Errorf("superseded activity ends at %v", last.End)
	}
	if s3.Active.Len() != 2 || !s3.IsActiveAt(6500*msec) || !s3.IsActiveAt(8*time.Second) {
		t.Errorf("activity of stack 3 = %v", s3.Active.All())
	}
	if s9.Active.Len() != 2 || s9.IsActiveAt(8700*msec) || s9.IsActiveAt(9200*msec) || s9.Interval.End != 9*time.Second {
		t.Errorf("activity of the stack removed by remove all = %v", s9.Active.All())
	}
	if p1.Stacks().ActiveAt(2500*msec).Count() != 0 || p1.Stacks().ActiveAt(3500*msec).Count() != 1 || p1.Stacks().ActiveAt(5200*msec).Count() != 1 || p1.Stacks().ActiveAt(5700*msec).Count() != 0 || p1.Stacks().ActiveAt(8200*msec).Count() != 2 {
		t.Error("Stacks.ActiveAt is wrong")
	}
	checkInvariants(t, tl)
}

func TestStacksEndWithDespawn(t *testing.T) {
	b := fixture()
	b.buffApply(1000, addrP1, addrPet, skillBuff, 5000, 31)
	b.buffApply(1200, addrP1, addrPet, skillBurn, 5000, 32)
	b.buffActive(1300, addrPet, 31, 4000)
	b.stateByInst(2000, instPet, evtc.StateDespawn)
	b.buffActive(2200, addrPet, 31, 1000)                                         // after the despawn: ignored
	b.buffApply(2500, addrP1, addrPet, skillBuff, 5000, 33)                       // applied after the despawn: stays open
	b.buffRemoveSingle(3000, addrPet, 0, skillBuff, 0, 31, evtc.BuffRemoveSingle) // removal of a closed stack: ignored
	b.buffApply(2500, addrP2, addrP1, skillBuff, 5000, 34)
	b.buffRemoveAll(3500, addrP2, 0, skillBuff) // nothing open on p2: ignored
	b.state(4000, addrP1, evtc.StateDespawn)    // a player leaving takes its stacks away too
	tl := mustBuild(t, b.build(5000))
	pet, p1 := tl.Agent(addrPet), tl.players[0]
	stacks := pet.Stacks().All()
	if len(stacks) != 3 {
		t.Fatalf("stacks on the pet = %d", len(stacks))
	}
	s1, s2, s3 := stacks[0], stacks[1], stacks[2]
	if !s1.EndedByDespawn || s1.Remove != nil || s1.Superseded || s1.Interval.End != 2*time.Second || s1.Removal != evtc.BuffRemoveNone || s1.RemovedBy != nil {
		t.Errorf("stack ended by despawn = %+v", s1)
	}
	if s1.Active.Len() != 2 || !s1.IsActiveAt(1500*msec) || s1.Active.All()[1].End != 2*time.Second {
		t.Errorf("active spans of the stack = %v", s1.Active.All())
	}
	if !s2.EndedByDespawn || s2.Interval.End != 2*time.Second || s3.EndedByDespawn || s3.Remove != nil || s3.Interval.End != tl.Duration {
		t.Errorf("stacks = %+v / %+v", s2, s3)
	}
	if up := pet.Stacks().OfBuff(skillBuff).Uptime(tl.Interval()); up != 3500*msec {
		t.Errorf("uptime = %v", up)
	}
	if ps := p1.Stacks().All(); len(ps) != 1 || !ps[0].EndedByDespawn || ps[0].Interval.End != 4*time.Second || p1.LifeStateAt(4500*msec) != LifeGone {
		t.Errorf("player stacks = %+v", ps)
	}
	if tl.Unknown.Stacks().Count() != 0 || tl.Stacks().Count() != 4 {
		t.Errorf("stack counts: unknown %d, total %d", tl.Unknown.Stacks().Count(), tl.Stacks().Count())
	}
	checkInvariants(t, tl)
}

func TestStacking(t *testing.T) {
	if StackingIntensity.String() != "Intensity" || StackingQueue.String() != "Queue" || Stacking(9).String() != "Stacking(9)" {
		t.Error("stacking names are wrong")
	}
	if !StackingIntensity.Intensity() || !StackingConditionalLoss.Intensity() || StackingQueue.Intensity() || StackingRegeneration.Intensity() || StackingCappedDuration.Intensity() || StackingForce.Intensity() {
		t.Error("Intensity is wrong")
	}
	might := &Buff{Stacking: StackingIntensity, StackLimit: 25}
	quickness := &Buff{Stacking: StackingQueue, StackLimit: 99}
	unknown := &Buff{}
	for _, tt := range []struct {
		buff *Buff
		n    int
		want int
	}{{might, 0, 0}, {might, 3, 3}, {might, 30, 25}, {quickness, 0, 0}, {quickness, 1, 1}, {quickness, 32, 1}, {unknown, 7, 7}} {
		if got := tt.buff.EffectiveStacks(tt.n); got != tt.want {
			t.Errorf("EffectiveStacks(%d) with %v = %d, want %d", tt.n, tt.buff.Stacking, got, tt.want)
		}
	}
	if might.IsBoon() || !(&Buff{Info: &evtc.Event{}}).IsBoon() || !(&Buff{Category: 2}).IsCondition() || might.IsCondition() {
		t.Error("IsBoon or IsCondition is wrong")
	}
}

func TestEffectiveStacks(t *testing.T) {
	const quickness = 1187
	b := fixture()
	b.skill(quickness, "Quickness")
	b.buffInfo(skillBuff, StackingIntensity, 25, 0)
	b.buffInfo(quickness, StackingQueue, 99, 0)
	b.buffInfo(skillBurn, StackingIntensity, 1500, 2)
	// Thirty might stacks and three quickness applications, all over
	// [1s, 3s], on player one.
	for i := range 30 {
		b.buffApply(1000, addrP2, addrP1, skillBuff, 2000, uint32(1+i))
		b.buffRemoveSingle(3000, addrP1, 0, skillBuff, 0, uint32(1+i), evtc.BuffRemoveSingle)
	}
	for i := range 3 {
		b.buffApply(1000, addrP2, addrP1, quickness, 2000, uint32(100+i))
		b.buffRemoveSingle(3000, addrP1, 0, quickness, 0, uint32(100+i), evtc.BuffRemoveSingle)
	}
	// One burning stack on player two, cleansed by player one.
	b.buffApply(1500, addrBoss, addrP2, skillBurn, 5000, 200)
	b.buffRemoveSingle(2000, addrP2, addrP1, skillBurn, 4500, 200, evtc.BuffRemoveManual)
	tl := mustBuild(t, b.build(4000))
	p1, p2 := tl.players[0], tl.players[1]
	might, quick, burn := tl.Buff(skillBuff), tl.Buff(quickness), tl.Buff(skillBurn)

	if might.Stacking != StackingIntensity || might.StackLimit != 25 || !might.IsBoon() || quick.Stacking != StackingQueue || !burn.IsCondition() || burn.IsBoon() {
		t.Errorf("buff definitions: might %v/%d quickness %v burn category %d", might.Stacking, might.StackLimit, quick.Stacking, burn.Category)
	}
	at := 2 * time.Second
	if p1.Stacks().OfBuff(skillBuff).CountAt(at) != 30 || p1.Stacks().OfBuff(skillBuff).EffectiveAt(at) != 25 {
		t.Errorf("might: %d present, %d effective", p1.Stacks().OfBuff(skillBuff).CountAt(at), p1.Stacks().OfBuff(skillBuff).EffectiveAt(at))
	}
	if p1.Stacks().OfBuff(quickness).CountAt(at) != 3 || p1.Stacks().OfBuff(quickness).EffectiveAt(at) != 1 {
		t.Errorf("quickness: %d present, %d effective", p1.Stacks().OfBuff(quickness).CountAt(at), p1.Stacks().OfBuff(quickness).EffectiveAt(at))
	}
	if p1.Stacks().EffectiveAt(at) != 26 || p1.Stacks().EffectiveAt(500*msec) != 0 || p1.Stacks().EffectiveAt(3500*msec) != 0 {
		t.Errorf("mixed effective = %d", p1.Stacks().EffectiveAt(at))
	}
	span := tl.Interval()
	if got := p1.Stacks().OfBuff(skillBuff).EffectiveAverage(span); got != 12.5 {
		t.Errorf("might EffectiveAverage = %v, want 12.5", got)
	}
	if got := p1.Stacks().OfBuff(skillBuff).Average(span); got != 15 {
		t.Errorf("might Average = %v, want 15", got)
	}
	if got := p1.Stacks().OfBuff(quickness).EffectiveAverage(span); got != 0.5 {
		t.Errorf("quickness EffectiveAverage = %v, want 0.5", got)
	}
	if got := p1.Stacks().EffectiveAverage(NewInterval(time.Second, 3*time.Second)); got != 26 {
		t.Errorf("mixed EffectiveAverage = %v, want 26", got)
	}
	if p1.Stacks().EffectiveAverage(At(at)) != 0 || p2.Stacks().OfBuff(skillBuff).EffectiveAverage(span) != 0 {
		t.Error("EffectiveAverage of nothing is not 0")
	}

	// Removal, rankings.
	if tl.Stacks().RemovedBy(p1).Count() != 1 || tl.Stacks().RemovedBy(p1).First().Buff != burn || tl.Stacks().RemovedBy(p2).Count() != 0 || tl.Stacks().RemovedBy(nil).Count() != 0 {
		t.Errorf("RemovedBy = %d", tl.Stacks().RemovedBy(p1).Count())
	}
	if s := tl.Stacks().RemovedBy(p1).First(); s.Removal != evtc.BuffRemoveManual || s.Receiver != p2.Agent || s.Remaining != 4500*msec {
		t.Errorf("cleansed stack = %+v", s)
	}
	buffs := tl.Stacks().PerBuff()
	if len(buffs) != 3 || buffs[0].Buff != might || buffs[0].Stacks.Count() != 30 || buffs[1].Buff != quick || buffs[2].Buff != burn || buffs[2].Stacks.First().Receiver != p2.Agent {
		t.Errorf("PerBuff = %v", buffs)
	}
	receivers := tl.Stacks().PerReceiver()
	if len(receivers) != 2 || receivers[0].Agent != p1.Agent || receivers[0].Stacks.Count() != 33 || receivers[1].Agent != p2.Agent {
		t.Errorf("PerReceiver = %v", receivers)
	}
	appliers := tl.Stacks().PerApplier()
	if len(appliers) != 2 || appliers[0].Agent != p2.Agent || appliers[1].Agent != tl.Targets[0].Agent || appliers[1].Stacks.Count() != 1 {
		t.Errorf("PerApplier = %v", appliers)
	}
	if r := tl.Stacks().Reverse().PerBuff(); r[0].Stacks.First().ID != 1 {
		t.Error("reversed PerBuff is not in start order")
	}
	if p2.Stacks().OfBuff(quickness).PerBuff() != nil || p2.Stacks().OfBuff(quickness).PerReceiver() != nil || p2.Stacks().OfBuff(quickness).PerApplier() != nil {
		t.Error("rankings of nothing are not nil")
	}
	checkInvariants(t, tl)
}

func TestBuffStacks(t *testing.T) {
	b := fixture()
	b.buffInitial(0, addrP2, addrP1, skillBuff, 3000, 10000, 5)
	b.buffApply(1000, addrP2, addrP1, skillBuff, 5000, 7)
	b.buffChange(1500, addrP1, skillBuff, 1000, 7)
	b.buffApply(2000, 0, addrP1, skillBuff, 5000, 8)
	b.buffRemoveSingle(3000, addrP1, 0, skillBuff, 0, 5, evtc.BuffRemoveSingle)
	b.buffRemoveSingle(4000, addrP1, addrBoss, skillBuff, 2000, 7, evtc.BuffRemoveSingle)
	b.buffApply(4500, addrP2, addrP1, skillBuff, 5000, 9)
	b.buffRemoveSingle(6000, addrP1, addrP1, skillBuff, 1000, 8, evtc.BuffRemoveManual)
	b.buffRemoveAll(6000, addrP1, addrP1, skillBuff)
	b.buffApply(7000, addrP2, addrP1, skillBurn, 3000, 10)
	b.buffTick(7500, addrP2, addrP1, skillBurn, 250)
	b.buffApply(8000, addrP2, addrP2, skillBuff, 5000, 7)
	tl := mustBuild(t, b.build(10000))

	p1, p2, boss := tl.players[0], tl.players[1], tl.Targets[0]
	if p1.Stacks().Count() != 5 || tl.Stacks().Count() != 6 || p2.StacksApplied().Count() != 5 || p2.Stacks().Count() != 1 {
		t.Fatalf("stacks = %d total %d", p1.Stacks().Count(), tl.Stacks().Count())
	}
	might := tl.Buff(skillBuff)
	if might == nil || might.Skill.Name != "Might" || might.Stacks().Count() != 5 || tl.Skill(skillBuff).Buff != might || len(tl.Buffs) != 2 || tl.Buffs[0].Skill.ID != skillBurn {
		t.Fatalf("buffs = %v", tl.Buffs)
	}
	stacks := p1.Stacks().OfBuff(skillBuff)
	for _, tt := range []struct {
		at   time.Duration
		want int
	}{{2500 * msec, 3}, {3500 * msec, 2}, {5000 * msec, 2}, {6500 * msec, 0}, {500 * msec, 1}} {
		if got := stacks.CountAt(tt.at); got != tt.want {
			t.Errorf("CountAt(%v) = %d, want %d", tt.at, got, tt.want)
		}
	}
	byID := func(id uint32) *BuffStack {
		return p1.Stacks().Where(func(s *BuffStack) bool { return s.ID == id }).First()
	}
	s5 := byID(5)
	if !s5.Initial || s5.Original != 10*time.Second || s5.Applied != 3*time.Second || !s5.Expired() || s5.RemovedBy != nil || s5.Interval != NewInterval(0, 3*time.Second) || s5.ActiveOnApply {
		t.Errorf("initial stack = %+v", s5)
	}
	s7 := byID(7)
	if s7.Applier != p2.Agent || s7.Receiver != p1.Agent || s7.Applied != 5*time.Second || s7.Extended != time.Second || s7.Interval != NewInterval(time.Second, 4*time.Second) || s7.RemovedBy != boss.Agent || s7.Expired() || s7.Removal != evtc.BuffRemoveSingle || s7.Remaining != 2*time.Second || !s7.ActiveOnApply {
		t.Errorf("stripped stack = %+v", s7)
	}
	s8 := byID(8)
	if s8.Applier != tl.Unknown || s8.Removal != evtc.BuffRemoveManual || !s8.Expired() || s8.Duration() != 4*time.Second {
		t.Errorf("manual stack = %+v", s8)
	}
	s9 := byID(9)
	if s9.Removal != evtc.BuffRemoveAll || s9.Interval.End != 6*time.Second || s9.Remove == nil || s9.RemovedBy != p1.Agent || s9.Remaining != 0 {
		t.Errorf("remove-all stack = %+v", s9)
	}
	s10 := byID(10)
	if !s10.Open() || s10.Interval.End != 10*time.Second || s10.Buff.Skill.ID != skillBurn || s10.Removal != evtc.BuffRemoveNone {
		t.Errorf("open stack = %+v", s10)
	}
	if p1.Stacks().Open().Count() != 1 || p1.Stacks().Of(might).Count() != 4 || p1.Stacks().By(p2).Count() != 4 || tl.Stacks().On(p2).Count() != 1 {
		t.Error("stack filters are wrong")
	}
	if got := stacks.Uptime(tl.Interval()); got != 6*time.Second {
		t.Errorf("Uptime = %v", got)
	}
	if got := stacks.Uptime(NewInterval(3500*msec, 8*time.Second)); got != 2500*msec {
		t.Errorf("Uptime clipped = %v", got)
	}
	if stacks.Between(NewInterval(5*time.Second, 8*time.Second)).Count() != 2 || stacks.At(6*time.Second).Count() != 2 || stacks.At(6001*msec).Count() != 0 {
		t.Error("Between and At are wrong")
	}
	if groups := tl.Stacks().GroupBy(func(s *BuffStack) *Agent { return s.Receiver }); len(groups) != 2 || groups[p1.Agent].Count() != 5 {
		t.Errorf("GroupBy = %v", groups)
	}
	tick := p1.HitsTaken().BuffDamage().First()
	if tick == nil || tick.Damage != 250 || !tick.IsBuff || !tick.Landed() || tick.Skill.Buff != tl.Buff(skillBurn) || tick.Barrier != 0 {
		t.Errorf("buff tick = %+v", tick)
	}
	if p1.HitsTaken().Strikes().Count() != 0 || p2.Hits().BuffDamage().Damage() != 250 {
		t.Error("buff damage filters are wrong")
	}
}

func TestDuplicateStackID(t *testing.T) {
	b := fixture()
	b.buffApply(1000, addrP2, addrP1, skillBuff, 5000, 7)
	b.buffApply(2000, addrP2, addrP1, skillBuff, 5000, 7)
	b.buffRemoveSingle(3000, addrP1, 0, skillBuff, 0, 7, evtc.BuffRemoveSingle)
	tl := mustBuild(t, b.build(10000))

	stacks := tl.players[0].Stacks().All()
	if len(stacks) != 2 || stacks[0].Interval != NewInterval(time.Second, 2*time.Second) || stacks[0].Removal != evtc.BuffRemoveNone || stacks[0].Remove != nil {
		t.Errorf("reused id: first stack = %+v", stacks[0])
	}
	if !stacks[0].Superseded || stacks[0].Open() || stacks[0].Expired() || tl.players[0].Stacks().Open().Count() != 0 {
		t.Errorf("superseded stack reports open=%v expired=%v", stacks[0].Open(), stacks[0].Expired())
	}
	if stacks[1].Interval != NewInterval(2*time.Second, 3*time.Second) || stacks[1].Remove == nil || stacks[1].Superseded {
		t.Errorf("reused id: second stack = %+v", stacks[1])
	}
}
