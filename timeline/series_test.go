package timeline

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestNumbersBetween(t *testing.T) {
	n := NewNumbers([]Sample[float64]{{Time: time.Second, Value: 100}, {Time: 2 * time.Second, Value: 70}, {Time: 3 * time.Second, Value: 50}, {Time: 4 * time.Second, Value: 80}, {Time: 5 * time.Second, Value: 20}})
	for _, tt := range []struct {
		iv       Interval
		min, max float64
		ok       bool
	}{
		{NewInterval(2500*msec, 4500*msec), 50, 80, true},
		{NewInterval(3500*msec, 3900*msec), 50, 50, true},
		{NewInterval(0, 500*msec), 0, 0, false},
		{NewInterval(0, 6*time.Second), 20, 100, true},
		{NewInterval(6*time.Second, 7*time.Second), 20, 20, true},
	} {
		lo, okLo := n.MinBetween(tt.iv)
		hi, okHi := n.MaxBetween(tt.iv)
		if lo != tt.min || hi != tt.max || okLo != tt.ok || okHi != tt.ok {
			t.Errorf("MinBetween/MaxBetween(%v) = %v/%v, %v/%v, want %v/%v, %v", tt.iv, lo, hi, okLo, okHi, tt.min, tt.max, tt.ok)
		}
	}
	if n.TimeBelow(60, NewInterval(0, 6*time.Second)) != 2*time.Second || n.TimeAbove(60, NewInterval(0, 6*time.Second)) != 3*time.Second {
		t.Errorf("TimeBelow = %v TimeAbove = %v", n.TimeBelow(60, NewInterval(0, 6*time.Second)), n.TimeAbove(60, NewInterval(0, 6*time.Second)))
	}
	if n.TimeAbove(60, NewInterval(2500*msec, 4500*msec)) != time.Second || n.TimeBelow(60, NewInterval(2500*msec, 4500*msec)) != time.Second {
		t.Error("TimeAbove or TimeBelow on a sub-interval is wrong")
	}
	if n.TimeBelow(60, NewInterval(0, 500*msec)) != 0 || n.TimeBelow(200, NewInterval(6*time.Second, 8*time.Second)) != 2*time.Second || n.TimeBelow(10, At(3*time.Second)) != 0 {
		t.Error("TimeBelow at the edges is wrong")
	}

	// Bounded to a lifetime: the health of the boss is held from the
	// start of its lifetime and known until its end.
	b := fixture()
	b.health(1000, addrBoss, 100)
	b.health(2000, addrBoss, 70)
	b.health(3000, addrBoss, 50)
	b.health(4000, addrBoss, 80)
	b.health(5000, addrBoss, 20)
	tl := mustBuild(t, b.build(10000))
	boss := tl.targets[0]
	if boss.Lifetime != NewInterval(200*msec, 5*time.Second) {
		t.Fatalf("lifetime = %v", boss.Lifetime)
	}
	if got := boss.Health.TimeBelow(66.6, tl.Interval()); got != time.Second {
		t.Errorf("TimeBelow over the log = %v, want 1s", got)
	}
	if got := boss.Health.TimeAbove(66.6, tl.Interval()); got != 3800*msec {
		t.Errorf("TimeAbove over the log = %v, want 3.8s", got)
	}
	if lo, ok := boss.Health.MinBetween(NewInterval(0, 2500*msec)); !ok || lo != 70 {
		t.Errorf("MinBetween with a held start = %v, %v", lo, ok)
	}
	if _, ok := boss.Health.MaxBetween(NewInterval(6*time.Second, 7*time.Second)); ok {
		t.Error("MaxBetween outside the lifetime succeeded")
	}
	if boss.Health.TimeBelow(50, NewInterval(6*time.Second, 7*time.Second)) != 0 {
		t.Error("TimeBelow outside the lifetime is not 0")
	}
	if tl.players[1].Health.TimeBelow(50, tl.Interval()) != 0 {
		t.Error("TimeBelow without samples is not 0")
	}
}

func TestSeries(t *testing.T) {
	samples := []Sample[float64]{
		{Time: time.Second, Value: 10},
		{Time: 2 * time.Second, Value: 20},
		{Time: 4 * time.Second, Value: 40, Break: true},
		{Time: 5 * time.Second, Value: 50},
	}
	step := NewSeries(samples)
	if v, ok := step.At(1500 * time.Millisecond); !ok || v != 10 {
		t.Errorf("step At = %v, %v", v, ok)
	}
	if _, ok := step.At(500 * time.Millisecond); ok {
		t.Error("step At before the first sample succeeded")
	}
	if v, ok := step.At(9 * time.Second); !ok || v != 50 {
		t.Errorf("step At after the last sample = %v, %v", v, ok)
	}
	lerp := func(a, b float64, f float64) float64 { return a + (b-a)*f }
	linear := step.Interpolated(lerp, 1500*time.Millisecond)
	if v, _ := linear.At(1500 * time.Millisecond); v != 15 {
		t.Errorf("linear At = %v", v)
	}
	if v, _ := linear.At(3 * time.Second); v != 20 {
		t.Errorf("At across a gap wider than maxGap = %v, want the previous value", v)
	}
	if v, _ := linear.At(4500 * time.Millisecond); v != 45 {
		t.Errorf("At after a break = %v", v)
	}
	unbounded := step.Interpolated(lerp, 0)
	if v, _ := unbounded.At(3 * time.Second); v != 20 {
		t.Errorf("At into a break = %v, want the previous value", v)
	}
	held := step.Bounded(NewInterval(0, 6*time.Second), true)
	if v, ok := held.At(0); !ok || v != 10 {
		t.Errorf("held At = %v, %v", v, ok)
	}
	if _, ok := held.At(7 * time.Second); ok {
		t.Error("At outside the bound succeeded")
	}
	if held.Interval() != NewInterval(0, 6*time.Second) || step.Interval() != NewInterval(time.Second, 5*time.Second) {
		t.Error("Interval is wrong")
	}
	if s, ok := step.Before(2 * time.Second); !ok || s.Value != 20 {
		t.Errorf("Before = %v, %v", s, ok)
	}
	if s, ok := step.After(2 * time.Second); !ok || s.Value != 40 {
		t.Errorf("After = %v, %v", s, ok)
	}
	if _, ok := step.After(5 * time.Second); ok {
		t.Error("After the last sample succeeded")
	}
	sub := linear.Between(NewInterval(2*time.Second, 4*time.Second))
	if sub.Len() != 2 || sub.Interval() != NewInterval(2*time.Second, 4*time.Second) {
		t.Errorf("Between = %d samples, span %v", sub.Len(), sub.Interval())
	}
	if v, ok := sub.At(4500 * time.Millisecond); ok {
		t.Errorf("sub-series answered outside its span: %v", v)
	}
	if got := step.Map(func(s Sample[float64]) float64 { return s.Value }); len(got) != 4 || got[2] != 40 {
		t.Errorf("Map = %v", got)
	}
	if f, ok := step.First(); !ok || f.Value != 10 {
		t.Error("First is wrong")
	}
	if l, ok := step.Last(); !ok || l.Value != 50 {
		t.Error("Last is wrong")
	}
	n := 0
	for range step.Seq() {
		n++
	}
	if n != 4 {
		t.Errorf("Seq visited %d samples", n)
	}
	var empty Series[float64]
	if _, ok := empty.At(0); ok || empty.Len() != 0 || empty.Interval() != (Interval{}) {
		t.Error("empty series is wrong")
	}
}

func TestNumbers(t *testing.T) {
	n := NewNumbers([]Sample[float64]{
		{Time: 0, Value: 100},
		{Time: time.Second, Value: 70},
		{Time: 2 * time.Second, Value: 66},
		{Time: 3 * time.Second, Value: 50},
		{Time: 4 * time.Second, Value: 30},
		{Time: 5 * time.Second, Value: 40},
		{Time: 6 * time.Second, Value: 33.3},
	})
	cs := n.Crossings(66.6, 33.3)
	want := []Crossing[float64]{
		{Time: 2 * time.Second, Level: 66.6, Direction: Falling},
		{Time: 4 * time.Second, Level: 33.3, Direction: Falling},
		{Time: 5 * time.Second, Level: 33.3, Direction: Rising},
	}
	if len(cs) != len(want) {
		t.Fatalf("Crossings = %d, want %d: %+v", len(cs), len(want), cs)
	}
	for i, c := range cs {
		if c.Time != want[i].Time || c.Level != want[i].Level || c.Direction != want[i].Direction {
			t.Errorf("crossing %d = %v %v at %v, want %v %v at %v", i, c.Direction, c.Level, c.Time, want[i].Direction, want[i].Level, want[i].Time)
		}
	}
	if cs[0].From.Value != 70 || cs[0].To.Value != 66 {
		t.Errorf("crossing samples = %v -> %v", cs[0].From.Value, cs[0].To.Value)
	}
	if v, _ := n.Min(); v != 30 {
		t.Errorf("Min = %v", v)
	}
	if v, _ := n.Max(); v != 100 {
		t.Errorf("Max = %v", v)
	}
	if v, ok := n.Between(NewInterval(time.Second, 3*time.Second)).Min(); !ok || v != 50 {
		t.Errorf("Between Min = %v, %v", v, ok)
	}
	if v, ok := n.Bounded(NewInterval(0, 10*time.Second), true).At(8 * time.Second); !ok || v != 33.3 {
		t.Errorf("Bounded At = %v, %v", v, ok)
	}
	if Falling.String() != "Falling" || Rising.String() != "Rising" {
		t.Error("Direction names are wrong")
	}
	var empty Numbers[int64]
	if _, ok := empty.Min(); ok {
		t.Error("Min of an empty series succeeded")
	}
	if _, ok := empty.Max(); ok {
		t.Error("Max of an empty series succeeded")
	}
}

func TestVectors(t *testing.T) {
	a, b := Vec3{0, 0, 0}, Vec3{3, 4, 12}
	if a.DistTo(b) != 13 || a.DistTo2D(b) != 5 || b.Len() != 13 {
		t.Error("distances are wrong")
	}
	if b.Sub(a) != b || b.Dot(Vec3{1, 0, 0}) != 3 || b.XY() != (Vec2{3, 4}) {
		t.Error("vector operations are wrong")
	}
	if LerpVec3(a, b, 0.5) != (Vec3{1.5, 2, 6}) {
		t.Errorf("LerpVec3 = %v", LerpVec3(a, b, 0.5))
	}
	f := Vec2{0, 2}
	if f.Len() != 2 || f.Dot(Vec2{0, 1}) != 2 || f.Sub(Vec2{0, 1}) != (Vec2{0, 1}) {
		t.Error("Vec2 operations are wrong")
	}
	if got := f.Angle(); got < 1.57 || got > 1.58 {
		t.Errorf("Angle = %v", got)
	}
	if LerpVec2(Vec2{0, 0}, f, 0.25) != (Vec2{0, 0.5}) {
		t.Error("LerpVec2 is wrong")
	}
}

func TestSeriesAndSpansIteration(t *testing.T) {
	s := NewSeries([]Sample[int]{{Time: 1, Value: 1}, {Time: 2, Value: 2}, {Time: 3, Value: 3}})
	n := 0
	for range s.Seq() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Error("Series.Seq did not stop")
	}
	if _, ok := s.Before(0); ok {
		t.Error("Before the first sample succeeded")
	}
	var empty Series[int]
	if _, ok := empty.First(); ok {
		t.Error("First of an empty series succeeded")
	}
	if _, ok := empty.Last(); ok {
		t.Error("Last of an empty series succeeded")
	}
	sp := NewSpans([]Span[int]{{Interval: NewInterval(0, 1), Value: 1}, {Interval: NewInterval(1, 2), Value: 2}})
	n = 0
	for range sp.Seq() {
		n++
		break
	}
	if n != 1 {
		t.Error("Spans.Seq did not stop")
	}
}

func TestMovement(t *testing.T) {
	b := fixture()
	b.move(1000, addrP1, evtc.StatePosition, 0, 0, 0)
	b.move(1300, addrP1, evtc.StatePosition, 300, 0, 0)
	b.move(2000, addrP1, evtc.StateTeleport, 1000, 1000, 0)
	b.move(2300, addrP1, evtc.StatePosition, 1000, 1300, 0)
	b.move(5000, addrP1, evtc.StatePosition, 2000, 1300, 0)
	b.move(1000, addrP1, evtc.StateVelocity, 1, 0, 0)
	b.facing(1000, addrP1, 0, 1)
	tl := mustBuild(t, b.build(10000))

	p1 := tl.players[0]
	// The point of view event of the fixture opens the lifetime at 0; the
	// teleport counts as a position sample.
	if p1.Position.Len() != 5 || p1.Lifetime != NewInterval(0, 5*time.Second) {
		t.Fatalf("positions = %d lifetime %v", p1.Position.Len(), p1.Lifetime)
	}
	for _, tt := range []struct {
		at   time.Duration
		want Vec3
		ok   bool
	}{
		{1150 * msec, Vec3{150, 0, 0}, true},
		{1900 * msec, Vec3{300, 0, 0}, true},
		{2150 * msec, Vec3{1000, 1150, 0}, true},
		{4 * time.Second, Vec3{1000, 1300, 0}, true},
		{5 * time.Second, Vec3{2000, 1300, 0}, true},
		{999 * msec, Vec3{}, false},
		{6 * time.Second, Vec3{}, false},
	} {
		got, ok := p1.Position.At(tt.at)
		if ok != tt.ok || got != tt.want {
			t.Errorf("Position.At(%v) = %v, %v, want %v, %v", tt.at, got, ok, tt.want, tt.ok)
		}
	}
	if s, ok := p1.Position.Before(1400 * msec); !ok || s.Value.X != 300 || s.Event.IsStateChange != evtc.StatePosition {
		t.Errorf("Before = %+v", s)
	}
	if s, ok := p1.Position.After(1300 * msec); !ok || !s.Break {
		t.Errorf("After = %+v", s)
	}
	if p1.Position.Between(NewInterval(time.Second, 2*time.Second)).Len() != 3 {
		t.Error("Between is wrong")
	}
	if samples := p1.Position.Samples(); len(samples) != 5 || samples[2].Value != (Vec3{1000, 1000, 0}) || !samples[2].Break {
		t.Errorf("Samples = %v", samples)
	}
	if v, ok := p1.Velocity.At(1500 * msec); !ok || v != (Vec3{1, 0, 0}) {
		t.Errorf("Velocity = %v, %v", v, ok)
	}
	if f, ok := p1.Facing.At(time.Second); !ok || f != (Vec2{0, 1}) {
		t.Errorf("Facing = %v, %v", f, ok)
	}
	pos, _ := p1.Position.At(2300 * msec)
	if d := pos.DistTo(Vec3{1000, 1000, 0}); d != 300 {
		t.Errorf("DistTo = %v", d)
	}
}

func TestHealth(t *testing.T) {
	b := fixture()
	b.maxHealth(100, addrBoss, 34015256)
	b.health(100, addrBoss, 100)
	b.health(1000, addrBoss, 70)
	b.health(2000, addrBoss, 66)
	b.health(3000, addrBoss, 50)
	b.health(4000, addrBoss, 30)
	b.health(5000, addrBoss, 40)
	b.add(evtc.Event{Time: b.at(2500), SrcAgent: addrP1, DstAgent: 2550, IsStateChange: evtc.StateBarrierPctUpdate})
	b.hit(5000, addrP1, addrBoss, skillSlam, 1, evtc.ResultStrikeDamageNormal)
	tl := mustBuild(t, b.build(10000))

	boss := tl.targets[0]
	if boss.Lifetime.Start != 100*msec {
		t.Errorf("lifetime = %v", boss.Lifetime)
	}
	if v, ok := boss.Health.At(100 * msec); !ok || v != 100 {
		t.Errorf("Health.At(100ms) = %v, %v", v, ok)
	}
	if v, ok := boss.Health.At(1500 * msec); !ok || v != 70 {
		t.Errorf("Health.At(1.5s) = %v, %v", v, ok)
	}
	if _, ok := boss.Health.At(50 * msec); ok {
		t.Error("Health before the lifetime succeeded")
	}
	cs := boss.Health.Crossings(66.6, 33.3)
	if len(cs) != 3 || cs[0].Time != 2*time.Second || cs[0].Direction != Falling || cs[1].Time != 4*time.Second || cs[2].Direction != Rising || cs[2].Time != 5*time.Second {
		t.Errorf("Crossings = %+v", cs)
	}
	if v, ok := boss.MaxHealth.At(3 * time.Second); !ok || v != 34015256 {
		t.Errorf("MaxHealth = %v, %v", v, ok)
	}
	if v, _ := boss.Health.Min(); v != 30 {
		t.Errorf("Min = %v", v)
	}
	if v, ok := tl.players[0].Barrier.At(3 * time.Second); !ok || v != 25.5 {
		t.Errorf("Barrier = %v, %v", v, ok)
	}
	if v, ok := tl.players[0].Barrier.At(2500 * msec); !ok || v != 25.5 {
		t.Errorf("Barrier at its only sample = %v, %v", v, ok)
	}
	if v, ok := tl.players[0].Barrier.At(time.Second); !ok || v != 25.5 {
		t.Errorf("Barrier held before its first sample = %v, %v", v, ok)
	}
	if _, ok := tl.players[0].Barrier.At(6 * time.Second); ok {
		t.Error("Barrier after the lifetime succeeded")
	}
}
