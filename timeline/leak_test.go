package timeline

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// heapLive returns the live heap after a full collection.
func heapLive() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// TestBuildReleasesMemory checks that nothing outside a Timeline keeps its
// graph alive: once the last reference is dropped, the memory it used is
// collected.
func TestBuildReleasesMemory(t *testing.T) {
	l := genLog(genOptions{players: 10, adds: 10, duration: 300 * time.Second, seed: 9})
	base := heapLive()
	tl, err := Build(l)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the queries too, so that any cache they might keep is
	// included in the measurement.
	tl.targets[0].Casts().Hits().Blocked().Count()
	tl.players[0].Stacks().Uptime(tl.Interval())
	tl.targets[0].Health.Crossings(50)
	withGraph := heapLive()
	runtime.KeepAlive(tl)
	graph := int64(withGraph) - int64(base)
	tl = nil
	after := heapLive()
	retained := int64(after) - int64(base)
	t.Logf("log %d events: graph %d KiB, retained after release %d KiB", len(l.Events), graph/1024, retained/1024)
	if graph <= 0 {
		t.Fatalf("the graph did not show up on the heap: %d bytes", graph)
	}
	if retained > graph/4 {
		t.Errorf("%d KiB of %d KiB are still reachable after dropping the timeline", retained/1024, graph/1024)
	}
	runtime.KeepAlive(l)
}

// TestBuildStartsNoGoroutine checks that building and querying never
// leaves a goroutine behind.
func TestBuildStartsNoGoroutine(t *testing.T) {
	before := runtime.NumGoroutine()
	tl := mustBuild(t, genLog(genOptions{players: 4, adds: 2, duration: 30 * time.Second, seed: 10}))
	tl.Hits().Between(tl.Interval()).Count()
	if after := runtime.NumGoroutine(); after != before {
		t.Errorf("goroutines: %d before, %d after", before, after)
	}
}

// TestConcurrentQueries runs many queries in parallel on one timeline; the
// race detector validates the read-only claim.
func TestConcurrentQueries(t *testing.T) {
	tl := mustBuild(t, genLog(genOptions{players: 8, adds: 6, duration: 60 * time.Second, seed: 11}))
	boss := tl.targets[0]
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := tl.players[g%len(tl.players)]
			for i := range 200 {
				at := time.Duration(i) * 250 * time.Millisecond
				iv := NewInterval(at, at+5*time.Second)
				boss.Casts().Between(iv).Hits().On(p).Blocked().Count()
				p.Position.At(at)
				boss.Health.At(at)
				p.LifeStateAt(at)
				p.Stacks().CountAt(at)
				p.Stacks().Uptime(iv)
				tl.Events().Between(iv).Involving(p).Count()
				tl.AgentAt(p.InstanceID, at)
				boss.Health.Crossings(50)
				boss.HitsTaken().Between(iv).GroupBy(func(h *Hit) *Agent { return h.Src })
			}
		}()
	}
	wg.Wait()
	checkInvariants(t, tl)
}
