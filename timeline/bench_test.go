package timeline

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

// benchLog returns the real log when available, a synthetic one otherwise,
// so that query benchmarks run everywhere and stay comparable on one
// machine.
func benchLog(b *testing.B) *Timeline {
	b.Helper()
	if _, err := os.Stat(samplePath); err == nil {
		return loadSample(b)
	}
	return mustBuild(b, genLog(genOptions{players: 10, adds: 20, duration: 320 * time.Second, seed: 1}))
}

// BenchmarkBuildSynthetic measures the build cost per event at three
// scales, up to the size of a large WvW log.
func BenchmarkBuildSynthetic(b *testing.B) {
	for _, o := range []genOptions{
		{players: 5, adds: 5, duration: 60 * time.Second, seed: 1},
		{players: 10, adds: 20, duration: 320 * time.Second, seed: 1},
		{players: 50, adds: 50, duration: 900 * time.Second, seed: 1},
	} {
		l := genLog(o)
		b.Run(fmt.Sprintf("events=%d", len(l.Events)), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Build(l); err != nil {
					b.Fatal(err)
				}
			}
			perEvent := float64(b.Elapsed().Nanoseconds()) / float64(b.N) / float64(len(l.Events))
			b.ReportMetric(perEvent, "ns/event")
		})
	}
}

func BenchmarkPositionAt(b *testing.B) {
	tl := benchLog(b)
	p := tl.Players[0]
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		p.Position.At(time.Duration(i%300) * time.Second)
	}
}

func BenchmarkHealthAt(b *testing.B) {
	tl := benchLog(b)
	boss := tl.Targets[0]
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		boss.Health.At(time.Duration(i%300) * time.Second)
	}
}

func BenchmarkStateAt(b *testing.B) {
	tl := benchLog(b)
	p := tl.Players[0]
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		p.LifeStateAt(time.Duration(i%300) * time.Second)
	}
}

func BenchmarkAgentAt(b *testing.B) {
	tl := benchLog(b)
	p := tl.Players[0]
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		tl.AgentAt(p.InstanceID, time.Duration(i%300)*time.Second)
	}
}

func BenchmarkHitsBetween(b *testing.B) {
	tl := benchLog(b)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		start := time.Duration(i%300) * time.Second
		tl.Hits().Between(NewInterval(start, start+10*time.Second)).Count()
	}
}

func BenchmarkHitsFiltered(b *testing.B) {
	tl := benchLog(b)
	boss, p := tl.Targets[0], tl.Players[0]
	b.ReportAllocs()
	for b.Loop() {
		boss.HitsTaken().By(p).Strikes().Damage()
	}
}

func BenchmarkCastsHits(b *testing.B) {
	tl := benchLog(b)
	boss := tl.Targets[0]
	iv := NewInterval(0, 60*time.Second)
	b.ReportAllocs()
	for b.Loop() {
		boss.Casts().Between(iv).Hits().Blocked().Count()
	}
}

func BenchmarkStacksCountAt(b *testing.B) {
	tl := benchLog(b)
	p := tl.Players[0]
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		p.Stacks().CountAt(time.Duration(i%300) * time.Second)
	}
}

func BenchmarkStacksUptime(b *testing.B) {
	tl := benchLog(b)
	p := tl.Players[0]
	span := tl.Interval()
	b.ReportAllocs()
	for b.Loop() {
		p.Stacks().OfBuff(740).Uptime(span)
	}
}

func BenchmarkCrossings(b *testing.B) {
	tl := benchLog(b)
	boss := tl.Targets[0]
	b.ReportAllocs()
	for b.Loop() {
		boss.Health.Crossings(66.6, 33.3)
	}
}

func BenchmarkGroupByDamage(b *testing.B) {
	tl := benchLog(b)
	boss := tl.Targets[0]
	b.ReportAllocs()
	for b.Loop() {
		for _, hits := range boss.HitsTaken().Landed().GroupBy(func(h *Hit) *Agent { return h.Src }) {
			hits.Damage()
		}
	}
}

func BenchmarkEventsInvolving(b *testing.B) {
	tl := benchLog(b)
	p := tl.Players[0]
	iv := NewInterval(0, 30*time.Second)
	b.ReportAllocs()
	for b.Loop() {
		tl.Events().Between(iv).Involving(p).Of(evtc.StateCombat).Count()
	}
}

func BenchmarkHitsAll(b *testing.B) {
	tl := benchLog(b)
	boss := tl.Targets[0]
	b.ReportAllocs()
	for b.Loop() {
		boss.HitsTaken().All()
	}
}
