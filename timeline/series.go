package timeline

import (
	"cmp"
	"iter"
	"math"
	"sort"
	"time"

	"github.com/42atomys/evtc"
)

// Sample is one observation of a value at a point in time. Event points to
// the raw log event the sample was decoded from.
type Sample[T any] struct {
	// Time is the time of the observation.
	Time time.Duration
	// Value is the observed value.
	Value T
	// Event is the raw event the sample was decoded from.
	Event *evtc.Event
	// Break marks a discontinuity: the value must not be interpolated
	// between the previous sample and this one (a teleport, for instance).
	Break bool
}

// Series is a time-sorted list of samples of one value. Between two samples
// the value is either held (step series, the default) or interpolated with
// the function given at construction.
//
// Series values are cheap to copy: they only hold a slice header and a few
// scalars, and the samples are never copied.
type Series[T any] struct {
	samples []Sample[T]
	lerp    func(a, b T, f float64) T
	// maxGap bounds the distance between two samples for interpolation to
	// apply; beyond it the previous value is held. Zero means no bound.
	maxGap time.Duration
	// span is the range of time over which At answers. Zero means the
	// range covered by the samples.
	span Interval
	// holdBefore extends the first value backwards to span.Start.
	holdBefore bool
}

// NewSeries builds a step series from samples sorted by time. The slice is
// not copied.
func NewSeries[T any](samples []Sample[T]) Series[T] {
	return Series[T]{samples: samples}
}

// Interpolated returns a copy of the series that interpolates between
// samples closer than maxGap using lerp. A zero maxGap removes the bound.
func (s Series[T]) Interpolated(lerp func(a, b T, f float64) T, maxGap time.Duration) Series[T] {
	s.lerp = lerp
	s.maxGap = maxGap
	return s
}

// Bounded returns a copy of the series that only answers At within span
// and, when holdBefore is set, holds the first value from span.Start up to
// the first sample.
func (s Series[T]) Bounded(span Interval, holdBefore bool) Series[T] {
	s.span = span
	s.holdBefore = holdBefore
	return s
}

// Len returns the number of samples.
func (s Series[T]) Len() int { return len(s.samples) }

// Samples returns the underlying samples. The slice must not be modified.
func (s Series[T]) Samples() []Sample[T] { return s.samples }

// Seq iterates over the samples in order.
func (s Series[T]) Seq() iter.Seq[Sample[T]] {
	return func(yield func(Sample[T]) bool) {
		for _, smp := range s.samples {
			if !yield(smp) {
				return
			}
		}
	}
}

// First returns the earliest sample. The boolean is false when the series
// is empty.
func (s Series[T]) First() (Sample[T], bool) {
	if len(s.samples) == 0 {
		return Sample[T]{}, false
	}
	return s.samples[0], true
}

// Last returns the latest sample. The boolean is false when the series is
// empty.
func (s Series[T]) Last() (Sample[T], bool) {
	if len(s.samples) == 0 {
		return Sample[T]{}, false
	}
	return s.samples[len(s.samples)-1], true
}

// Interval returns the time range the series answers on: the explicit bound
// given with Bounded, otherwise the range covered by the samples.
func (s Series[T]) Interval() Interval {
	if s.span != (Interval{}) || len(s.samples) == 0 {
		return s.span
	}
	return Interval{Start: s.samples[0].Time, End: s.samples[len(s.samples)-1].Time}
}

// Before returns the last sample taken at or before t.
func (s Series[T]) Before(t time.Duration) (Sample[T], bool) {
	i := s.after(t)
	if i == 0 {
		return Sample[T]{}, false
	}
	return s.samples[i-1], true
}

// After returns the first sample taken strictly after t.
func (s Series[T]) After(t time.Duration) (Sample[T], bool) {
	i := s.after(t)
	if i == len(s.samples) {
		return Sample[T]{}, false
	}
	return s.samples[i], true
}

// Between returns the samples taken within iv as a sub-series sharing the
// same interpolation settings. Nothing is copied.
func (s Series[T]) Between(iv Interval) Series[T] {
	lo := s.after(iv.Start - 1)
	hi := s.after(iv.End)
	s.samples = s.samples[lo:hi]
	s.span = iv
	return s
}

// At returns the value at time t. The boolean is false when t is outside
// the series span or before the first sample of a series that does not
// hold its first value backwards.
func (s Series[T]) At(t time.Duration) (T, bool) {
	var zero T
	if len(s.samples) == 0 || (s.span != (Interval{}) && !s.span.Contains(t)) {
		return zero, false
	}
	i := s.after(t)
	if i == 0 {
		if s.holdBefore {
			return s.samples[0].Value, true
		}
		return zero, false
	}
	prev := s.samples[i-1]
	if i == len(s.samples) || s.lerp == nil {
		return prev.Value, true
	}
	next := s.samples[i]
	gap := next.Time - prev.Time
	if next.Break || gap <= 0 || (s.maxGap > 0 && gap > s.maxGap) {
		return prev.Value, true
	}
	return s.lerp(prev.Value, next.Value, float64(t-prev.Time)/float64(gap)), true
}

// Map converts every sample with f.
func (s Series[T]) Map[U any](f func(Sample[T]) U) []U {
	out := make([]U, len(s.samples))
	for i, smp := range s.samples {
		out[i] = f(smp)
	}
	return out
}

// after returns the index of the first sample taken strictly after t.
func (s Series[T]) after(t time.Duration) int {
	return sort.Search(len(s.samples), func(i int) bool { return s.samples[i].Time > t })
}

// Numbers is a Series of ordered values, which adds threshold crossing
// detection. Health, barrier and defiance bar percentages are Numbers.
type Numbers[T cmp.Ordered] struct {
	Series[T]
}

// NewNumbers builds a step Numbers series from samples sorted by time.
func NewNumbers[T cmp.Ordered](samples []Sample[T]) Numbers[T] {
	return Numbers[T]{Series: NewSeries(samples)}
}

// Between returns the samples taken within iv as a sub-series.
func (n Numbers[T]) Between(iv Interval) Numbers[T] {
	n.Series = n.Series.Between(iv)
	return n
}

// Bounded is Series.Bounded for Numbers.
func (n Numbers[T]) Bounded(span Interval, holdBefore bool) Numbers[T] {
	n.Series = n.Series.Bounded(span, holdBefore)
	return n
}

// Direction tells which way a value moved through a threshold.
type Direction uint8

const (
	// Falling when the value went from at or above the level to below it.
	Falling Direction = iota
	// Rising when the value went from below the level to at or above it.
	Rising
)

// String returns "Falling" or "Rising".
func (d Direction) String() string {
	if d == Rising {
		return "Rising"
	}
	return "Falling"
}

// Crossing records the moment a series moved through a threshold. Time is
// the time of the first sample on the other side of the level; From and To
// are the samples on each side.
type Crossing[T cmp.Ordered] struct {
	// Time is the time of the first sample on the other side of the level.
	Time time.Duration
	// Level is the threshold crossed.
	Level T
	// Direction tells whether the series rose or fell through the level.
	Direction Direction
	// From is the last sample before the crossing.
	From Sample[T]
	// To is the first sample after the crossing.
	To Sample[T]
}

// Crossings finds every time the series moved through one of the given
// levels, in chronological order. A value equal to a level counts as being
// at or above it.
func (n Numbers[T]) Crossings(levels ...T) []Crossing[T] {
	var out []Crossing[T]
	for i := 1; i < len(n.samples); i++ {
		from, to := n.samples[i-1], n.samples[i]
		for _, level := range levels {
			switch {
			case from.Value >= level && to.Value < level:
				out = append(out, Crossing[T]{Time: to.Time, Level: level, Direction: Falling, From: from, To: to})
			case from.Value < level && to.Value >= level:
				out = append(out, Crossing[T]{Time: to.Time, Level: level, Direction: Rising, From: from, To: to})
			}
		}
	}
	return out
}

// Min returns the smallest sampled value. The boolean is false when the
// series is empty.
func (n Numbers[T]) Min() (T, bool) {
	if len(n.samples) == 0 {
		var zero T
		return zero, false
	}
	v := n.samples[0].Value
	for _, smp := range n.samples[1:] {
		v = min(v, smp.Value)
	}
	return v, true
}

// Max returns the largest sampled value. The boolean is false when the
// series is empty.
func (n Numbers[T]) Max() (T, bool) {
	if len(n.samples) == 0 {
		var zero T
		return zero, false
	}
	v := n.samples[0].Value
	for _, smp := range n.samples[1:] {
		v = max(v, smp.Value)
	}
	return v, true
}

// Vec3 is a position or velocity in game units.
type Vec3 struct {
	// X, Y and Z are the game coordinates.
	X, Y, Z float32
}

// Vec2 is a facing direction in the horizontal plane.
type Vec2 struct {
	// X and Y are the horizontal components.
	X, Y float32
}

// Sub returns v - o.
func (v Vec3) Sub(o Vec3) Vec3 { return Vec3{v.X - o.X, v.Y - o.Y, v.Z - o.Z} }

// Len returns the length of the vector.
func (v Vec3) Len() float64 {
	return math.Sqrt(float64(v.X)*float64(v.X) + float64(v.Y)*float64(v.Y) + float64(v.Z)*float64(v.Z))
}

// DistTo returns the distance between the two points.
func (v Vec3) DistTo(o Vec3) float64 { return v.Sub(o).Len() }

// DistTo2D returns the distance between the two points ignoring height.
func (v Vec3) DistTo2D(o Vec3) float64 {
	dx, dy := float64(v.X-o.X), float64(v.Y-o.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

// Dot returns the dot product of the two vectors.
func (v Vec3) Dot(o Vec3) float64 {
	return float64(v.X)*float64(o.X) + float64(v.Y)*float64(o.Y) + float64(v.Z)*float64(o.Z)
}

// XY drops the height component.
func (v Vec3) XY() Vec2 { return Vec2{v.X, v.Y} }

// Len returns the length of the vector.
func (v Vec2) Len() float64 { return math.Hypot(float64(v.X), float64(v.Y)) }

// Dot returns the dot product of the two vectors.
func (v Vec2) Dot(o Vec2) float64 { return float64(v.X)*float64(o.X) + float64(v.Y)*float64(o.Y) }

// Sub returns v - o.
func (v Vec2) Sub(o Vec2) Vec2 { return Vec2{v.X - o.X, v.Y - o.Y} }

// Angle returns the angle of the vector in radians, measured from the X
// axis towards the Y axis.
func (v Vec2) Angle() float64 { return math.Atan2(float64(v.Y), float64(v.X)) }

// LerpVec3 interpolates linearly between a and b; it is the interpolation
// used by position and velocity series.
func LerpVec3(a, b Vec3, f float64) Vec3 {
	g := float32(f)
	return Vec3{a.X + (b.X-a.X)*g, a.Y + (b.Y-a.Y)*g, a.Z + (b.Z-a.Z)*g}
}

// LerpVec2 interpolates linearly between a and b.
func LerpVec2(a, b Vec2, f float64) Vec2 {
	g := float32(f)
	return Vec2{a.X + (b.X-a.X)*g, a.Y + (b.Y-a.Y)*g}
}

// FirstBelow returns the first time the value was below level. When the
// first sample is already below level and the series holds it backwards,
// that time is the start of the series interval. The boolean is false when
// the value never was below level.
func (n Numbers[T]) FirstBelow(level T) (time.Duration, bool) {
	return n.first(func(v T) bool { return v < level })
}

// FirstAbove returns the first time the value was at or above level, with
// the same rule as FirstBelow for a first sample already above it. The
// boolean is false when the value never was at or above level.
func (n Numbers[T]) FirstAbove(level T) (time.Duration, bool) {
	return n.first(func(v T) bool { return v >= level })
}

// first returns the time of the first sample accepted by p, moved back to
// the start of the interval when it is the first sample of a series that
// holds it backwards.
func (n Numbers[T]) first(p func(T) bool) (time.Duration, bool) {
	for i, s := range n.samples {
		if p(s.Value) {
			if i == 0 && n.holdBefore {
				return n.Interval().Start, true
			}
			return s.Time, true
		}
	}
	return 0, false
}

// MinBetween returns the smallest value over iv, the value held at the
// start of iv included. The boolean is false when nothing is known over
// iv.
func (n Numbers[T]) MinBetween(iv Interval) (T, bool) {
	v, ok := n.At(iv.Start)
	for s := range n.Between(iv).Seq() {
		if !ok || s.Value < v {
			v, ok = s.Value, true
		}
	}
	return v, ok
}

// MaxBetween returns the largest value over iv, the value held at the
// start of iv included. The boolean is false when nothing is known over
// iv.
func (n Numbers[T]) MaxBetween(iv Interval) (T, bool) {
	v, ok := n.At(iv.Start)
	for s := range n.Between(iv).Seq() {
		if !ok || s.Value > v {
			v, ok = s.Value, true
		}
	}
	return v, ok
}

// TimeBelow returns how long the value was below level within iv, the
// series being a step function held between samples. Time before the
// first known value does not count.
func (n Numbers[T]) TimeBelow(level T, iv Interval) time.Duration {
	return n.timeWhere(iv, func(v T) bool { return v < level })
}

// TimeAbove returns how long the value was at or above level within iv,
// with the rules of TimeBelow.
func (n Numbers[T]) TimeAbove(level T, iv Interval) time.Duration {
	return n.timeWhere(iv, func(v T) bool { return v >= level })
}

// timeWhere integrates the time within iv during which p accepts the
// held value, iv being clipped to the interval the series answers on.
func (n Numbers[T]) timeWhere(iv Interval, p func(T) bool) time.Duration {
	if n.span != (Interval{}) {
		clipped, ok := iv.Intersect(n.span)
		if !ok {
			return 0
		}
		iv = clipped
	}
	var total time.Duration
	v, ok := n.At(iv.Start)
	prev := iv.Start
	for s := range n.Between(iv).Seq() {
		if ok && p(v) {
			total += s.Time - prev
		}
		v, ok, prev = s.Value, true, s.Time
	}
	if ok && p(v) {
		total += iv.End - prev
	}
	return total
}
