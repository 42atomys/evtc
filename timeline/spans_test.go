package timeline

import (
	"testing"
	"time"
)

func TestSpans(t *testing.T) {
	s := NewSpans([]Span[LifeState]{
		{Interval: NewInterval(0, 2*time.Second), Value: LifeAlive},
		{Interval: NewInterval(2*time.Second, 3*time.Second), Value: LifeDown},
		{Interval: NewInterval(3*time.Second, 5*time.Second), Value: LifeAlive},
		{Interval: NewInterval(7*time.Second, 9*time.Second), Value: LifeDead},
	})
	if v, ok := s.ValueAt(2500 * time.Millisecond); !ok || v != LifeDown {
		t.Errorf("ValueAt = %v, %v", v, ok)
	}
	if _, ok := s.ValueAt(6 * time.Second); ok {
		t.Error("ValueAt in a hole succeeded")
	}
	if _, ok := s.ValueAt(10 * time.Second); ok {
		t.Error("ValueAt after the last span succeeded")
	}
	if sp, ok := s.At(2 * time.Second); !ok || sp.Value != LifeDown {
		t.Errorf("At on a boundary = %v, %v", sp, ok)
	}
	if got := s.Between(NewInterval(2500*time.Millisecond, 8*time.Second)); got.Len() != 3 || got.All()[0].Value != LifeDown {
		t.Errorf("Between = %v", got.All())
	}
	if got := s.Between(NewInterval(5500*time.Millisecond, 6*time.Second)); got.Len() != 0 {
		t.Errorf("Between a hole = %v", got.All())
	}
	down := func(sp Span[LifeState]) bool { return sp.Value == LifeDown }
	if got := s.Total(NewInterval(0, 10*time.Second), down); got != time.Second {
		t.Errorf("Total down = %v", got)
	}
	if got := s.Total(NewInterval(2500*time.Millisecond, 10*time.Second), nil); got != 4500*time.Millisecond {
		t.Errorf("Total all = %v", got)
	}
	if got := s.Where(down); len(got) != 1 {
		t.Errorf("Where = %v", got)
	}
	alive := func(sp Span[LifeState]) bool { return sp.Value == LifeAlive }
	if got := s.Intervals(alive); len(got) != 2 || got[0] != NewInterval(0, 2*time.Second) || got[1] != NewInterval(3*time.Second, 5*time.Second) {
		t.Errorf("Intervals alive = %v", got)
	}
	if got := s.Intervals(nil); len(got) != 2 || got[0] != NewInterval(0, 5*time.Second) || got[1] != NewInterval(7*time.Second, 9*time.Second) {
		t.Errorf("Intervals all = %v", got)
	}
	if got := s.Intervals(func(Span[LifeState]) bool { return false }); got != nil {
		t.Errorf("Intervals none = %v", got)
	}
	if got := s.Map(func(sp Span[LifeState]) LifeState { return sp.Value }); len(got) != 4 || got[3] != LifeDead {
		t.Errorf("Map = %v", got)
	}
	if f, _ := s.First(); f.Value != LifeAlive {
		t.Error("First is wrong")
	}
	if l, _ := s.Last(); l.Value != LifeDead {
		t.Error("Last is wrong")
	}
	n := 0
	for range s.Seq() {
		n++
	}
	if n != 4 {
		t.Errorf("Seq visited %d spans", n)
	}
	var empty Spans[bool]
	if _, ok := empty.First(); ok {
		t.Error("First of empty spans succeeded")
	}
	if _, ok := empty.Last(); ok {
		t.Error("Last of empty spans succeeded")
	}
}
