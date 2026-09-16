package timeline

import (
	"iter"
	"sort"
	"time"

	"github.com/42atomys/evtc"
)

// Span is a value that held over an interval. Event points to the raw log
// event that started it, when there is one.
type Span[T any] struct {
	Interval
	// Value is the value held over the interval.
	Value T
	// Event is the raw event that started the span, nil when there is
	// none.
	Event *evtc.Event
}

// Spans is a sorted list of non-overlapping spans describing a value that
// changes in steps: alive/down/dead states, defiance bar states, combat
// presence.
type Spans[T any] struct {
	spans []Span[T]
}

// NewSpans wraps spans sorted by start. The slice is not copied.
func NewSpans[T any](spans []Span[T]) Spans[T] { return Spans[T]{spans: spans} }

// Len returns the number of spans.
func (s Spans[T]) Len() int { return len(s.spans) }

// All returns the underlying spans. The slice must not be modified.
func (s Spans[T]) All() []Span[T] { return s.spans }

// Seq iterates over the spans in order.
func (s Spans[T]) Seq() iter.Seq[Span[T]] {
	return func(yield func(Span[T]) bool) {
		for _, sp := range s.spans {
			if !yield(sp) {
				return
			}
		}
	}
}

// First returns the earliest span. The boolean is false when there is none.
func (s Spans[T]) First() (Span[T], bool) {
	if len(s.spans) == 0 {
		return Span[T]{}, false
	}
	return s.spans[0], true
}

// Last returns the latest span. The boolean is false when there is none.
func (s Spans[T]) Last() (Span[T], bool) {
	if len(s.spans) == 0 {
		return Span[T]{}, false
	}
	return s.spans[len(s.spans)-1], true
}

// At returns the span covering t. The boolean is false when no span does.
func (s Spans[T]) At(t time.Duration) (Span[T], bool) {
	i := sort.Search(len(s.spans), func(i int) bool { return s.spans[i].Start > t })
	if i == 0 || s.spans[i-1].End < t {
		return Span[T]{}, false
	}
	return s.spans[i-1], true
}

// ValueAt returns the value held at t. The boolean is false when no span
// covers t.
func (s Spans[T]) ValueAt(t time.Duration) (T, bool) {
	sp, ok := s.At(t)
	return sp.Value, ok
}

// Between returns the spans overlapping iv as a sub-list. Nothing is copied.
func (s Spans[T]) Between(iv Interval) Spans[T] {
	lo := sort.Search(len(s.spans), func(i int) bool { return s.spans[i].End >= iv.Start })
	hi := lo + sort.Search(len(s.spans)-lo, func(i int) bool { return s.spans[lo+i].Start > iv.End })
	s.spans = s.spans[lo:hi]
	return s
}

// Where returns the spans accepted by p.
func (s Spans[T]) Where(p func(Span[T]) bool) []Span[T] {
	var out []Span[T]
	for _, sp := range s.spans {
		if p(sp) {
			out = append(out, sp)
		}
	}
	return out
}

// Map converts every span with f.
func (s Spans[T]) Map[U any](f func(Span[T]) U) []U {
	out := make([]U, len(s.spans))
	for i, sp := range s.spans {
		out[i] = f(sp)
	}
	return out
}

// Intervals returns the intervals covered by the spans accepted by p, in
// time order. Accepted spans that touch or overlap are merged into one
// interval. A nil p accepts every span.
func (s Spans[T]) Intervals(p func(Span[T]) bool) []Interval {
	var out []Interval
	for _, sp := range s.spans {
		if p != nil && !p(sp) {
			continue
		}
		if n := len(out); n > 0 && sp.Start <= out[n-1].End {
			out[n-1] = out[n-1].Union(sp.Interval)
			continue
		}
		out = append(out, sp.Interval)
	}
	return out
}

// Total returns the summed duration of the spans accepted by p, clipped to
// iv. A nil p accepts every span.
func (s Spans[T]) Total(iv Interval, p func(Span[T]) bool) time.Duration {
	var total time.Duration
	for _, sp := range s.Between(iv).spans {
		if p != nil && !p(sp) {
			continue
		}
		if clipped, ok := sp.Intersect(iv); ok {
			total += clipped.Duration()
		}
	}
	return total
}
