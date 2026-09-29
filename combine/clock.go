package combine

import (
	"slices"
	"time"

	"github.com/42atomys/evtc"
)

// MinStrikes is the number of strikes two recordings must share, dated
// alike, for their clocks to be set on each other. Recordings of the same
// fight share thousands; a few seconds on a training golem may not hold
// twenty.
const MinStrikes = 20

// strike names a strike by what every client writes alike.
type strike struct {
	src, dst uint16
	skill    uint32
	value    int32
	result   evtc.Result
}

// strikes lists the times of the strikes of a recording, per strike.
func strikes(r *recording) map[strike][]time.Duration {
	out := map[strike][]time.Duration{}
	for i := range r.tl.Log.Events {
		e := &r.tl.Log.Events[i]
		if kindOf(e, r.typed) != evtc.StateCombat || e.Buff != 0 {
			continue
		}
		k := strike{e.SrcInstanceID, e.DstInstanceID, e.SkillID, e.Value, e.Result}
		out[k] = append(out[k], r.tl.TimeOf(e))
	}
	return out
}

// offsetOf returns the time of b minus the time of a for the same instant,
// and the number of strikes that tell it. A strike both hold once gives a
// gap. The gaps of one fight stand together, within what two clients date
// apart, and the offset is their median. Two fights that look alike share
// strikes too, by chance, whose gaps are minutes apart: the gaps that
// count are those of the densest span.
func offsetOf(a, b map[strike][]time.Duration) (time.Duration, int) {
	var gaps []time.Duration
	for k, ta := range a {
		if tb := b[k]; len(ta) == 1 && len(tb) == 1 {
			gaps = append(gaps, tb[0]-ta[0])
		}
	}
	slices.Sort(gaps)
	var together []time.Duration
	for i, from := 0, 0; i < len(gaps); i++ {
		for gaps[i]-gaps[from] > 2*widest {
			from++
		}
		if i+1-from > len(together) {
			together = gaps[from : i+1]
		}
	}
	if len(together) == 0 {
		return 0, 0
	}
	return together[len(together)/2].Round(time.Millisecond), len(together)
}
