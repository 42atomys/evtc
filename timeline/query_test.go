package timeline

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestQueryWindow(t *testing.T) {
	items := make([]*item, 6)
	for i := range items {
		items[i] = &item{time.Duration(i+1) * time.Second, i + 1}
	}
	q := From(items)
	isOdd := func(i *item) bool { return i.v%2 == 1 }
	odd := q.Where(isOdd)
	values := func(q Query[item]) []int { return q.Map(func(i *item) int { return i.v }) }
	all := []int{1, 2, 3, 4, 5, 6}

	for _, tt := range []struct {
		name string
		q    Query[item]
		want []int
	}{
		{"Skip", q.Skip(2), []int{3, 4, 5, 6}},
		{"Limit", q.Limit(2), []int{1, 2}},
		{"Skip then Limit", q.Skip(2).Limit(3), []int{3, 4, 5}},
		{"Limit then Skip", q.Limit(3).Skip(2), []int{3}},
		{"Limit twice", q.Limit(5).Limit(3), []int{1, 2, 3}},
		{"Limit widened", q.Limit(3).Limit(5), []int{1, 2, 3}},
		{"Skip twice", q.Skip(2).Skip(3), []int{6}},
		{"Limit zero", q.Limit(0), nil},
		{"Limit negative", q.Limit(-1), nil},
		{"Skip negative", q.Skip(-1), all},
		{"Skip beyond", q.Skip(10), nil},
		{"Limit beyond", q.Limit(10), all},
		{"Limit max", q.Limit(math.MaxInt), all},
		{"Skip max", q.Skip(math.MaxInt - 1).Limit(math.MaxInt), nil},
		{"Skip overflow", q.Skip(math.MaxInt).Skip(math.MaxInt), nil},
		{"Reverse", q.Reverse(), []int{6, 5, 4, 3, 2, 1}},
		{"Reverse twice", q.Reverse().Reverse(), all},
		{"Reverse Limit", q.Reverse().Limit(2), []int{6, 5}},
		{"Limit Reverse", q.Limit(2).Reverse(), []int{6, 5}},
		{"Reverse Skip", q.Reverse().Skip(4), []int{2, 1}},
		{"Where Skip", odd.Skip(1), []int{3, 5}},
		{"Where Limit", odd.Limit(2), []int{1, 3}},
		{"Limit then Where", q.Limit(2).Where(isOdd), []int{1, 3}},
		{"Where Reverse Limit", odd.Reverse().Limit(1), []int{5}},
	} {
		got := values(tt.q)
		if !slices.Equal(got, tt.want) || tt.q.Count() != len(tt.want) {
			t.Errorf("%s = %v (count %d), want %v", tt.name, got, tt.q.Count(), tt.want)
		}
		if out := tt.q.All(); len(out) != len(tt.want) || cap(out) != len(tt.want) {
			t.Errorf("%s: All has len %d cap %d, want %d", tt.name, len(out), cap(out), len(tt.want))
		}
		if len(tt.want) == 0 {
			if tt.q.Any() || tt.q.First() != nil || tt.q.Last() != nil {
				t.Errorf("%s: terminals of an empty traversal are wrong", tt.name)
			}
			continue
		}
		if !tt.q.Any() || tt.q.First().v != tt.want[0] || tt.q.Last().v != tt.want[len(tt.want)-1] {
			t.Errorf("%s: First %v Last %v, want %v and %v", tt.name, tt.q.First(), tt.q.Last(), tt.want[0], tt.want[len(tt.want)-1])
		}
	}

	n := 0
	for range q.Skip(1).Limit(4).Seq() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Error("Seq did not stop")
	}
	sum := 0
	q.Skip(4).Each(func(i *item) { sum += i.v })
	if sum != 11 || q.Reverse().Limit(2).Sum(func(i *item) int { return i.v }) != 11 {
		t.Errorf("Each sum = %d", sum)
	}
	groups := q.Limit(3).GroupBy(isOdd)
	if len(groups[true]) != 2 || len(groups[false]) != 1 {
		t.Errorf("GroupBy with a window = %v", groups)
	}
}

func TestQueryWindowBruteForce(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 11))
	items := make([]*item, 25)
	for i := range items {
		items[i] = &item{time.Duration(i) * time.Second, i}
	}
	preds := []func(*item) bool{nil, func(i *item) bool { return i.v%2 == 0 }, func(i *item) bool { return i.v > 10 }}
	value := func(i *item) int { return i.v }
	for range 1000 {
		q := From(items)
		p := preds[r.IntN(len(preds))]
		if p != nil {
			q = q.Where(p)
		}
		desc := r.IntN(2) == 1
		if desc {
			q = q.Reverse()
		}
		skip, limit := r.IntN(10), r.IntN(10)-1
		limitFirst := r.IntN(2) == 1
		if limitFirst {
			if limit >= 0 {
				q = q.Limit(limit)
			}
			q = q.Skip(skip)
		} else {
			q = q.Skip(skip)
			if limit >= 0 {
				q = q.Limit(limit)
			}
		}

		var accepted []int
		for _, it := range items {
			if p == nil || p(it) {
				accepted = append(accepted, it.v)
			}
		}
		if desc {
			slices.Reverse(accepted)
		}
		lo, hi := skip, len(accepted)
		if limit >= 0 && limitFirst {
			hi = min(hi, limit)
		} else if limit >= 0 {
			hi = min(hi, skip+limit)
		}
		var want []int
		if lo < hi {
			want = accepted[lo:hi]
		}

		got := q.Map(value)
		if !slices.Equal(got, want) || q.Count() != len(want) || q.Any() != (len(want) > 0) {
			t.Fatalf("pred %v desc %v skip %d limit %d limitFirst %v: got %v, want %v", p != nil, desc, skip, limit, limitFirst, got, want)
		}
		if len(want) > 0 && (q.First().v != want[0] || q.Last().v != want[len(want)-1]) {
			t.Fatalf("First %v Last %v, want %v", q.First(), q.Last(), want)
		}
		if len(want) == 0 && (q.First() != nil || q.Last() != nil) {
			t.Fatal("First or Last of an empty traversal is not nil")
		}
	}
}

func TestWindowOnQueries(t *testing.T) {
	b := fixture()
	// Casts come first so that a hit at the same instant belongs to them.
	b.castStart(1000, addrP1, addrBoss, skillSlam, 500, 500)
	b.castStop(1500, addrP1, skillSlam, 500, evtc.ActivationReset)
	b.castStart(3000, addrP1, addrBoss, skillSlam, 500, 500)
	b.castStop(3500, addrP1, skillSlam, 500, evtc.ActivationReset)
	for i := range 6 {
		b.hit(uint64(1000+i*1000), addrP1, addrBoss, skillSlam, int32(10*(i+1)), evtc.ResultStrikeDamageNormal)
	}
	b.buffApply(1000, addrP2, addrP1, skillBuff, 5000, 1)
	b.buffRemoveSingle(4000, addrP1, 0, skillBuff, 0, 1, evtc.BuffRemoveSingle)
	b.buffApply(2000, addrP2, addrP1, skillBuff, 5000, 2)
	b.buffApply(3000, addrP2, addrP1, skillBuff, 1000, 3)
	b.buffRemoveSingle(3500, addrP1, 0, skillBuff, 0, 3, evtc.BuffRemoveSingle)
	tl := mustBuild(t, b.build(10000))
	p1 := tl.characters[0]
	damage := func(h *Hit) int32 { return h.Damage }

	hits := p1.Hits()
	if got := hits.Reverse().Limit(2).Map(damage); !slices.Equal(got, []int32{60, 50}) {
		t.Errorf("Reverse.Limit = %v", got)
	}
	iv := NewInterval(2*time.Second, 4*time.Second)
	if hits.Skip(1).Between(iv).Count() != 2 || hits.Between(iv).Skip(1).Count() != 2 || hits.Between(iv).Skip(1).First().Damage != 30 {
		t.Error("Between and Skip do not compose")
	}
	if hits.Reverse().First().Damage != 60 || hits.Reverse().Last().Damage != 10 || hits.Limit(3).Damage() != 60 || hits.Reverse().Limit(3).Damage() != 150 {
		t.Error("hit terminals with a window are wrong")
	}

	casts := p1.Casts()
	if casts.Reverse().First().Interval.Start != 3*time.Second || casts.Skip(1).First().Interval.Start != 3*time.Second || casts.Limit(1).Hits().Count() != 2 || casts.Reverse().Limit(1).Hits().Count() != 4 {
		t.Error("cast windows are wrong")
	}
	if casts.Reverse().Hits().First().Time != time.Second {
		t.Error("Casts.Hits is not sorted by time after Reverse")
	}

	stacks := p1.Stacks()
	if stacks.Reverse().First().ID != 3 || stacks.Skip(1).Limit(1).First().ID != 2 || stacks.Reverse().CountAt(2500*msec) != 2 {
		t.Error("stack windows are wrong")
	}
	span := tl.Interval()
	if stacks.Uptime(span) != 9*time.Second || stacks.Reverse().Uptime(span) != 9*time.Second {
		t.Errorf("Uptime = %v reversed %v", stacks.Uptime(span), stacks.Reverse().Uptime(span))
	}
	if stacks.Limit(1).Uptime(span) != 3*time.Second || stacks.Reverse().Limit(1).Uptime(span) != 500*msec || stacks.Reverse().Limit(2).Uptime(span) != 8*time.Second {
		t.Errorf("Uptime with windows = %v, %v, %v", stacks.Limit(1).Uptime(span), stacks.Reverse().Limit(1).Uptime(span), stacks.Reverse().Limit(2).Uptime(span))
	}
	if stacks.Average(span) != 1.15 || stacks.Average(NewInterval(2*time.Second, 4*time.Second)) != 2.25 || stacks.Average(At(3*time.Second)) != 0 || stacks.Reverse().Average(span) != 1.15 || stacks.Limit(1).Average(span) != 0.3 {
		t.Errorf("Average = %v, %v", stacks.Average(span), stacks.Average(NewInterval(2*time.Second, 4*time.Second)))
	}

	events := tl.Events()
	if events.Reverse().First().IsStateChange != evtc.StateSquadCombatEnd || events.Limit(2).Count() != 2 || p1.Events().Skip(1).First() != p1.Events().All()[1] {
		t.Error("event windows are wrong")
	}
	// Groups of a reversed traversal are back in time order, so that
	// Between and First keep working on them.
	bySkill := hits.Reverse().GroupBy(func(h *Hit) *Skill { return h.Skill })
	if g := bySkill[tl.Skill(skillSlam)]; g.Count() != 6 || g.First().Damage != 10 || g.Between(At(3*time.Second)).Count() != 1 {
		t.Errorf("reversed GroupBy = %v", g.All())
	}
	byCaster := casts.Reverse().GroupBy(func(c *Cast) *Agent { return c.Caster })
	if g := byCaster[p1.Agent]; g.Count() != 2 || g.First().Interval.Start != time.Second || g.Between(At(3*time.Second)).Count() != 1 {
		t.Errorf("reversed Casts.GroupBy = %v", g.All())
	}
	byBuff := stacks.Reverse().GroupBy(func(s *BuffStack) *Buff { return s.Buff })
	if g := byBuff[tl.Buff(skillBuff)]; g.Count() != 3 || g.First().ID != 1 || g.At(3200*msec).Count() != 3 {
		t.Errorf("reversed Stacks.GroupBy = %v", g.All())
	}
	kinds := events.GroupBy(func(e *evtc.Event) evtc.StateChange { return e.IsStateChange })
	if kinds[evtc.StateAnimationStart].Count() != 2 || kinds[evtc.StateAnimationStart].Between(NewInterval(2*time.Second, 4*time.Second)).Count() != 1 || kinds[evtc.StateCombat].Involving(p1).Count() != 6 {
		t.Errorf("Events.GroupBy = %d starts, %d hits", kinds[evtc.StateAnimationStart].Count(), kinds[evtc.StateCombat].Count())
	}
	if g := events.Reverse().GroupBy(func(e *evtc.Event) evtc.StateChange { return e.IsStateChange })[evtc.StateCombat]; g.First().Time > g.Last().Time || g.Between(At(3*time.Second)).Count() != 1 {
		t.Errorf("reversed Events.GroupBy = %v", g.All())
	}
	checkInvariants(t, tl)
}

type item struct {
	t time.Duration
	v int
}

func itemTime(i *item) time.Duration { return i.t }

func TestQuery(t *testing.T) {
	items := []*item{{time.Second, 1}, {2 * time.Second, 2}, {3 * time.Second, 3}, {4 * time.Second, 4}}
	q := From(items)
	if q.Count() != 4 || !q.Any() || q.First() != items[0] || q.Last() != items[3] {
		t.Error("unfiltered terminals are wrong")
	}
	odd := q.Where(func(i *item) bool { return i.v%2 == 1 })
	if odd.Count() != 2 || odd.First() != items[0] || odd.Last() != items[2] {
		t.Error("Where terminals are wrong")
	}
	big := odd.Where(func(i *item) bool { return i.v > 1 })
	if got := big.All(); len(got) != 1 || got[0] != items[2] {
		t.Errorf("composed Where = %v", got)
	}
	if got := q.Map(func(i *item) int { return i.v * 10 }); len(got) != 4 || got[3] != 40 {
		t.Errorf("Map = %v", got)
	}
	groups := q.GroupBy(func(i *item) int { return i.v % 2 })
	if len(groups) != 2 || len(groups[0]) != 2 || groups[1][0] != items[0] {
		t.Errorf("GroupBy = %v", groups)
	}
	if got := odd.Sum(func(i *item) int { return i.v }); got != 4 {
		t.Errorf("Sum = %d", got)
	}
	n := 0
	q.Each(func(*item) { n++ })
	if n != 4 {
		t.Errorf("Each visited %d items", n)
	}
	for range q.Seq() {
		n++
		break
	}
	if n != 5 {
		t.Error("Seq did not stop when asked")
	}
	empty := From[item](nil)
	if empty.Any() || empty.First() != nil || empty.Last() != nil || len(empty.All()) != 0 {
		t.Error("empty query terminals are wrong")
	}

	if got := narrow(items, itemTime, NewInterval(2*time.Second, 3*time.Second)); len(got) != 2 || got[0] != items[1] {
		t.Errorf("narrow = %v", got)
	}
	if got := narrow(items, itemTime, NewInterval(5*time.Second, 6*time.Second)); len(got) != 0 {
		t.Errorf("narrow outside = %v", got)
	}
	if got := narrowStart(items, itemTime, NewInterval(0, 2500*time.Millisecond)); len(got) != 2 {
		t.Errorf("narrowStart = %v", got)
	}
	shuffled := []*item{items[2], items[0], items[3], items[1]}
	if got := sortedByTime(shuffled, itemTime); got[0] != items[0] || got[3] != items[3] {
		t.Errorf("sortedByTime = %v", got)
	}
}

func TestQueryNarrowAndReversed(t *testing.T) {
	items := make([]*item, 6)
	for i := range items {
		items[i] = &item{time.Duration(i+1) * time.Second, i + 1}
	}
	at := func(i *item) time.Duration { return i.t }
	q := From(items)
	values := func(q Query[item]) []int { return q.Map(func(i *item) int { return i.v }) }
	if got := values(q.Narrow(at, NewInterval(2*time.Second, 4*time.Second))); !slices.Equal(got, []int{2, 3, 4}) {
		t.Errorf("Narrow = %v", got)
	}
	if got := values(q.Where(func(i *item) bool { return i.v%2 == 0 }).Narrow(at, NewInterval(0, 5*time.Second)).Reverse()); !slices.Equal(got, []int{4, 2}) {
		t.Errorf("Narrow with a filter, reversed = %v", got)
	}
	if got := values(q.Narrow(at, NewInterval(10*time.Second, 20*time.Second))); len(got) != 0 {
		t.Errorf("Narrow outside = %v", got)
	}
	if q.Reversed() || !q.Reverse().Reversed() || q.Reverse().Reverse().Reversed() {
		t.Error("Reversed does not follow Reverse")
	}
}
