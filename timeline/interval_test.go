package timeline

import (
	"testing"
	"time"
)

func TestIntervalConstructors(t *testing.T) {
	if Around(5*time.Second, time.Second) != NewInterval(4*time.Second, 6*time.Second) || Around(5*time.Second, -time.Second) != NewInterval(4*time.Second, 6*time.Second) {
		t.Errorf("Around = %v", Around(5*time.Second, time.Second))
	}
	iv := NewInterval(0, 10*time.Second)
	if got := iv.Split(); len(got) != 1 || got[0] != iv {
		t.Errorf("Split() = %v", got)
	}
	got := iv.Split(4*time.Second, 2*time.Second, 2*time.Second, 0, 10*time.Second, 15*time.Second, -time.Second)
	want := []Interval{NewInterval(0, 2*time.Second), NewInterval(2*time.Second, 4*time.Second), NewInterval(4*time.Second, 10*time.Second)}
	if len(got) != len(want) {
		t.Fatalf("Split = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("piece %d = %v, want %v", i, got[i], want[i])
		}
	}
	if got := At(3 * time.Second).Split(3 * time.Second); len(got) != 1 || got[0] != At(3*time.Second) {
		t.Errorf("Split of an instant = %v", got)
	}
}

func TestIntervalEvery(t *testing.T) {
	iv := NewInterval(time.Second, 8*time.Second)
	got := iv.Every(3 * time.Second)
	want := []Interval{NewInterval(time.Second, 4*time.Second), NewInterval(4*time.Second, 7*time.Second), NewInterval(7*time.Second, 8*time.Second)}
	if len(got) != len(want) {
		t.Fatalf("Every = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("piece %d = %v, want %v", i, got[i], want[i])
		}
	}
	if got := NewInterval(0, 6*time.Second).Every(2 * time.Second); len(got) != 3 || got[2] != NewInterval(4*time.Second, 6*time.Second) {
		t.Errorf("Every with an exact fit = %v", got)
	}
	for _, d := range []time.Duration{0, -time.Second, 7 * time.Second, time.Minute} {
		if got := iv.Every(d); len(got) != 1 || got[0] != iv {
			t.Errorf("Every(%v) = %v", d, got)
		}
	}
	if got := At(3 * time.Second).Every(time.Second); len(got) != 1 || got[0] != At(3*time.Second) {
		t.Errorf("Every of an instant = %v", got)
	}
}

func TestInterval(t *testing.T) {
	iv := NewInterval(3*time.Second, time.Second)
	if iv != (Interval{Start: time.Second, End: 3 * time.Second}) {
		t.Errorf("NewInterval did not swap bounds: %v", iv)
	}
	if iv.Duration() != 2*time.Second {
		t.Errorf("Duration = %v", iv.Duration())
	}
	if !iv.Contains(time.Second) || !iv.Contains(3*time.Second) || iv.Contains(4*time.Second) {
		t.Error("Contains is wrong at the bounds")
	}
	if !iv.Overlaps(NewInterval(3*time.Second, 5*time.Second)) || iv.Overlaps(NewInterval(4*time.Second, 5*time.Second)) {
		t.Error("Overlaps is wrong")
	}
	got, ok := iv.Intersect(NewInterval(2*time.Second, 5*time.Second))
	if !ok || got != NewInterval(2*time.Second, 3*time.Second) {
		t.Errorf("Intersect = %v, %v", got, ok)
	}
	if _, ok := iv.Intersect(At(10 * time.Second)); ok {
		t.Error("Intersect of disjoint intervals succeeded")
	}
	if iv.Union(At(10*time.Second)) != NewInterval(time.Second, 10*time.Second) {
		t.Error("Union is wrong")
	}
	if iv.Clamp(0) != time.Second || iv.Clamp(9*time.Second) != 3*time.Second || iv.Clamp(2*time.Second) != 2*time.Second {
		t.Error("Clamp is wrong")
	}
	if iv.String() != "[1s, 3s]" {
		t.Errorf("String = %q", iv.String())
	}
}
