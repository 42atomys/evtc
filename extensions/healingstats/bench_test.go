package healingstats

import (
	"fmt"
	"testing"

	"github.com/42atomys/evtc/timeline"
)

// BenchmarkDecodeSynthetic measures the cost of the decode alone per event
// of the addon, on synthetic logs of three sizes.
func BenchmarkDecodeSynthetic(b *testing.B) {
	for _, seconds := range []int{60, 600, 3600} {
		tl, err := timeline.Build(genLog(1, seconds))
		if err != nil {
			b.Fatal(err)
		}
		x := tl.Extension(Signature)
		b.Run(fmt.Sprintf("events=%d", x.Events().Count()), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				build(x)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(x.Events().Count()), "ns/event")
		})
	}
}

func BenchmarkHealsPerAgent(b *testing.B) {
	s := mustBuild(b, genLog(1, 600))
	b.ReportAllocs()
	for b.Loop() {
		s.Heals().Healing().PerAgent()
	}
}
