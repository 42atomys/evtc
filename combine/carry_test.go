package combine

import (
	"github.com/42atomys/evtc"
)

// healths returns the health updates of a log as times of the scene and
// percentages.
func healths(l *evtc.Log) [][2]uint64 {
	var out [][2]uint64
	for _, e := range of(l, evtc.StateHealthPctUpdate) {
		out = append(out, [2]uint64{e.Time - rawEpoch, e.DstAgent / 100})
	}
	return out
}

func sameHealths(got [][2]uint64, want ...[2]uint64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
