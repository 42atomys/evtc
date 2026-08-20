package timeline

import (
	"cmp"
	"iter"
	"math"
	"slices"
	"sort"
	"time"
)

// Number is the set of numeric types accepted by Query.Sum.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Query is a lazy view over a slice of graph nodes. A filter composes a
// predicate, allocated once, instead of copying the slice; All, Map and
// GroupBy allocate the collection they return, All and Map once with its
// final size.
//
// Skip, Limit and Reverse describe the traversal of the accepted
// elements: Reverse flips its direction, Skip drops its first n elements
// and Limit stops it after n. They apply after every filter of the chain,
// wherever they appear in it, so Limit(3).Where(p) yields at most three
// elements accepted by p rather than filtering the first three.
// Skip(2).Limit(3) and Limit(5).Skip(2) traverse the same window.
//
// The concrete query types of this package (Hits, Casts, Stacks, Events)
// embed Query and add filters that know the node type, so every generic
// terminal below is available on them as well.
type Query[E any] struct {
	items []*E
	pred  func(*E) bool
	// The traversal yields the accepted elements whose index lies in
	// [lo, hi-1); hi is 0 when Limit never bounded it.
	lo, hi int
	desc   bool
}

// From wraps items in a Query. The slice is not copied.
func From[E any](items []*E) Query[E] { return Query[E]{items: items} }

// Where keeps the elements accepted by p. Successive calls are combined
// with a logical and.
func (q Query[E]) Where(p func(*E) bool) Query[E] {
	if q.pred == nil {
		q.pred = p
		return q
	}
	prev := q.pred
	q.pred = func(e *E) bool { return prev(e) && p(e) }
	return q
}

// Skip drops the first n accepted elements of the traversal.
func (q Query[E]) Skip(n int) Query[E] {
	if n > 0 {
		if q.lo += n; q.lo < 0 {
			q.lo = math.MaxInt
		}
	}
	return q
}

// Limit stops the traversal after n accepted elements.
func (q Query[E]) Limit(n int) Query[E] {
	end := q.lo + max(n, 0) + 1
	if end < q.lo {
		end = math.MaxInt
	}
	if q.hi == 0 || end < q.hi {
		q.hi = end
	}
	return q
}

// Reverse flips the direction of the traversal, so that First returns the
// latest element and Limit keeps the latest ones.
func (q Query[E]) Reverse() Query[E] {
	q.desc = !q.desc
	return q
}

// Reversed reports whether the traversal runs from the latest element to
// the earliest, so that a query type built on Query can put the groups it
// returns back in time order.
func (q Query[E]) Reversed() bool { return q.desc }

// Narrow restricts the traversal to the elements whose time, as reported
// by at, lies within iv, with a binary search: the elements must be sorted
// by that time. It is what the Between filters of this package do, offered
// to the query types of other packages.
func (q Query[E]) Narrow(at func(*E) time.Duration, iv Interval) Query[E] {
	q.items = narrow(q.items, at, iv)
	return q
}

// windowed reports whether Skip or Limit narrowed the traversal.
func (q Query[E]) windowed() bool { return q.lo != 0 || q.hi != 0 }

// Seq iterates over the traversal without allocating. The plain forward
// traversal is a tight loop that the compiler inlines into the terminals;
// windows and reversed traversals take the general path of walk.
func (q Query[E]) Seq() iter.Seq[*E] {
	return func(yield func(*E) bool) {
		if q.desc || q.windowed() {
			q.walk(yield)
			return
		}
		for _, e := range q.items {
			if (q.pred == nil || q.pred(e)) && !yield(e) {
				return
			}
		}
	}
}

// walk is the traversal with a window or a reversed direction.
func (q Query[E]) walk(yield func(*E) bool) {
	n := 0
	if q.desc {
		for i := len(q.items) - 1; i >= 0; i-- {
			if !q.step(q.items[i], &n, yield) {
				return
			}
		}
		return
	}
	for _, e := range q.items {
		if !q.step(e, &n, yield) {
			return
		}
	}
}

// step hands e to yield when the predicate accepts it and it lies inside
// the window, counting the accepted elements in n. It reports whether the
// traversal goes on.
func (q Query[E]) step(e *E, n *int, yield func(*E) bool) bool {
	if q.pred != nil && !q.pred(e) {
		return true
	}
	i := *n
	*n = i + 1
	if i < q.lo {
		return true
	}
	if q.hi != 0 && i >= q.hi-1 {
		return false
	}
	if !yield(e) {
		return false
	}
	return q.hi == 0 || i+1 < q.hi-1
}

// Each calls f on every element of the traversal.
func (q Query[E]) Each(f func(*E)) {
	for e := range q.Seq() {
		f(e)
	}
}

// Count returns the number of elements of the traversal.
func (q Query[E]) Count() int {
	if q.pred == nil {
		n := len(q.items) - q.lo
		if q.hi != 0 {
			n = min(n, q.hi-1-q.lo)
		}
		return max(n, 0)
	}
	n := 0
	for range q.Seq() {
		n++
	}
	return n
}

// Any reports whether the traversal yields at least one element.
func (q Query[E]) Any() bool {
	for range q.Seq() {
		return true
	}
	return false
}

// First returns the first element of the traversal, or nil when it is
// empty.
func (q Query[E]) First() *E {
	for e := range q.Seq() {
		return e
	}
	return nil
}

// Last returns the last element of the traversal, or nil when it is
// empty.
func (q Query[E]) Last() *E {
	if !q.windowed() {
		q.desc = !q.desc
		return q.First()
	}
	var last *E
	for e := range q.Seq() {
		last = e
	}
	return last
}

// All returns the elements of the traversal as a new slice sized exactly.
func (q Query[E]) All() []*E {
	out := make([]*E, 0, q.Count())
	for e := range q.Seq() {
		out = append(out, e)
	}
	return out
}

// Map applies f to every element of the traversal and returns the results.
func (q Query[E]) Map[T any](f func(*E) T) []T {
	out := make([]T, 0, q.Count())
	for e := range q.Seq() {
		out = append(out, f(e))
	}
	return out
}

// GroupBy partitions the elements of the traversal by the key returned by
// f. Each group keeps the traversal order.
func (q Query[E]) GroupBy[K comparable](f func(*E) K) map[K][]*E {
	groups := map[K][]*E{}
	for e := range q.Seq() {
		k := f(e)
		groups[k] = append(groups[k], e)
	}
	return groups
}

// Sum adds the values returned by f over the elements of the traversal.
func (q Query[E]) Sum[N Number](f func(*E) N) N {
	var total N
	for e := range q.Seq() {
		total += f(e)
	}
	return total
}

// narrow restricts items to the elements whose time, as reported by at,
// lies within iv. The slice must be sorted by that time; the result is a
// sub-slice, so nothing is copied.
func narrow[E any](items []*E, at func(*E) time.Duration, iv Interval) []*E {
	lo := sort.Search(len(items), func(i int) bool { return at(items[i]) >= iv.Start })
	hi := lo + sort.Search(len(items)-lo, func(i int) bool { return at(items[lo+i]) > iv.End })
	return items[lo:hi]
}

// narrowStart keeps the elements whose start time is at most iv.End. It is
// the binary-search half of an overlap test for interval nodes sorted by
// start; the caller adds the "end >= iv.Start" half as a predicate.
func narrowStart[E any](items []*E, start func(*E) time.Duration, iv Interval) []*E {
	hi := sort.Search(len(items), func(i int) bool { return start(items[i]) > iv.End })
	return items[:hi]
}

// overlapping keeps the interval nodes, sorted by start, that share at
// least one instant with iv.
func overlapping[E any](q Query[E], start, end func(*E) time.Duration, iv Interval) Query[E] {
	q.items = narrowStart(q.items, start, iv)
	return q.Where(func(e *E) bool { return end(e) >= iv.Start })
}

// sortedByTime sorts items in place by the time reported by at.
func sortedByTime[E any](items []*E, at func(*E) time.Duration) []*E {
	slices.SortStableFunc(items, func(a, b *E) int { return cmp.Compare(at(a), at(b)) })
	return items
}
