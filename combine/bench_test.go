package combine

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkLogsSynthetic measures a run per event read, on generated
// fights recorded by two to four clients.
func BenchmarkLogsSynthetic(b *testing.B) {
	for _, recorders := range []int{2, 4} {
		for _, minutes := range []int{1, 5} {
			logs, _ := genFight(genOptions{recorders: recorders, players: 10, adds: 40 * minutes, duration: time.Duration(minutes) * time.Minute, seed: 1})
			events := 0
			for _, l := range logs {
				events += len(l.Events)
			}
			b.Run(fmt.Sprintf("recordings=%d/events=%d", recorders, events), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := Logs(logs...); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(events), "ns/event")
			})
		}
	}
}

// BenchmarkLogsSample measures a run on the sample and on the same log as
// another client would write it, the timelines of both included.
func BenchmarkLogsSample(b *testing.B) {
	tl := loadSample(b)
	other := moved(tl, 0x5a5a_0000_5a5a, 7_654_322)
	events := 2 * len(tl.Log.Events)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Logs(tl.Log, other); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(events), "ns/event")
}
