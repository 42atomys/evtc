package combine

import (
	"fmt"
	"slices"
	"time"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// MinStrikes is the number of strikes two recordings must share, dated
// alike, for their clocks to be set on each other. Recordings of the same
// fight share thousands; a few seconds on a training golem may not hold
// twenty.
const MinStrikes = 20

// MinTicks is the number of ticks of the server two recordings of one
// instance must both hold for their clocks to be set on each other when
// they share fewer than MinStrikes strikes: two recorders too far apart to
// see the same creatures. arcdps writes the tick once a second since build
// 20260701.
const MinTicks = 20

// startTolerance is how far the clocks of the server may be from what the
// strikes of two recordings tell: the log holds them to the second.
const startTolerance = 5 * time.Second

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

// ticks lists the TICK events of a recording: the tick of the server each
// one names and its time. The tick counts from the start of the instance
// and is the same for every client of the instance; a client may name the
// tick before or after the one another client names at the same second.
type ticks struct {
	tick []int64
	at   []time.Duration
}

// ticksAnEvent is the number of ticks of the server between two TICK
// events: one a second, 25 ticks of 40 ms.
const ticksAnEvent = 25

// ticksOf reads the ticks of a recording, in their order; a tick that does
// not follow the one before is left out.
func ticksOf(r *recording) ticks {
	var s ticks
	for i := range r.tl.Log.Events {
		e := &r.tl.Log.Events[i]
		if e.IsStateChange != evtc.StateTick {
			continue
		}
		if tick := int64(e.SrcAgent); len(s.tick) == 0 || tick > s.tick[len(s.tick)-1] {
			s.tick = append(s.tick, tick)
			s.at = append(s.at, r.tl.TimeOf(e))
		}
	}
	return s
}

// timeAt returns the time of a tick of the server on the clock of the
// recording: the time of its TICK event, or the one between the two TICK
// events around it, and whether the recording covers the tick.
func (s ticks) timeAt(tick int64) (time.Duration, bool) {
	i, found := slices.BinarySearch(s.tick, tick)
	if found {
		return s.at[i], true
	}
	if i == 0 || i == len(s.tick) {
		return 0, false
	}
	span := s.tick[i] - s.tick[i-1]
	if span > 2*ticksAnEvent {
		return 0, false
	}
	return s.at[i-1] + time.Duration(tick-s.tick[i-1])*(s.at[i]-s.at[i-1])/time.Duration(span), true
}

// tickOffset returns the time of b minus the time of a for the same tick
// of the server, the median over the ticks of a that b covers, and how
// many they are.
func tickOffset(a, b ticks) (time.Duration, int) {
	var gaps []time.Duration
	for i, tick := range a.tick {
		if at, ok := b.timeAt(tick); ok {
			gaps = append(gaps, at-a.at[i])
		}
	}
	if len(gaps) == 0 {
		return 0, 0
	}
	slices.Sort(gaps)
	return gaps[len(gaps)/2].Round(time.Millisecond), len(gaps)
}

// socketOf returns the socket of the server a recording names in its
// instance start, 0 when it names none.
func socketOf(r *recording) uint32 {
	for i := range r.tl.Log.Events {
		if e := &r.tl.Log.Events[i]; e.IsStateChange == evtc.StateInstanceStart {
			return uint32(e.Value)
		}
	}
	return 0
}

// sameInstance reports whether two recordings were made in the same
// instance of the map: they name the same socket of the server and an
// instance that started at the same time. Two instances of one map
// started 36 s apart at the least over the corpus measured.
func sameInstance(a, b *recording) bool {
	sa, sb := socketOf(a), socketOf(b)
	if sa == 0 || sa != sb || a.tl.InstanceStart.IsZero() || b.tl.InstanceStart.IsZero() {
		return false
	}
	return a.tl.InstanceStart.Sub(b.tl.InstanceStart).Abs() <= startTolerance
}

// sameStart reports whether the clocks of the server agree with an offset
// read from the strikes: two attempts of a fight by the same squad share
// strikes a few seconds into each, and started minutes apart. A log
// without the time of the server tells nothing.
func sameStart(a, b *timeline.Timeline, off time.Duration) bool {
	if a.Start.IsZero() || b.Start.IsZero() {
		return true
	}
	return (a.Start.Sub(b.Start) - off).Abs() <= startTolerance
}

// epoch returns the raw time of the zero of a timeline.
func epoch(tl *timeline.Timeline) int64 {
	for i := range tl.Log.Events {
		e := &tl.Log.Events[i]
		if timed(e.IsStateChange) && e.IsStateChange != evtc.StateExtension {
			return int64(e.Time) - tl.TimeOf(e).Milliseconds()
		}
	}
	return 0
}

// setClocks gives every recording its offset to the first one. The clock
// of a recording is set on the first one, or on a recording whose clock is
// set when it shares too little with the first: by the strikes both hold,
// or by the ticks of the server when they share too few strikes and were
// made in the same instance.
func setClocks(recs []*recording) error {
	struck := make([]map[strike][]time.Duration, len(recs))
	clocks := make([]ticks, len(recs))
	for i, r := range recs {
		struck[i], clocks[i] = strikes(r), ticksOf(r)
	}
	// offset returns the offset of recording i from recording j, and
	// whether the two can be set on each other.
	offset := func(j, i int) (time.Duration, bool) {
		off, n := offsetOf(struck[j], struck[i])
		if n >= MinStrikes {
			return off, sameStart(recs[j].tl, recs[i].tl, off)
		}
		if !sameInstance(recs[j], recs[i]) {
			return 0, false
		}
		off, n = tickOffset(clocks[j], clocks[i])
		return off, n >= MinTicks
	}
	set := make([]bool, len(recs))
	set[0] = true
	for again := true; again; {
		again = false
		for i := range recs {
			if set[i] {
				continue
			}
			for j := range recs {
				if !set[j] {
					continue
				}
				if off, ok := offset(j, i); ok {
					recs[i].off, set[i], again = recs[j].off+off, true, true
					break
				}
			}
		}
	}
	for i, r := range recs {
		if !set[i] {
			off, n := offsetOf(struck[0], struck[i])
			if n >= MinStrikes {
				return fmt.Errorf("%w: started %v apart by the clock of the server, %v by their strikes", ErrFightMismatch, recs[0].tl.Start.Sub(r.tl.Start), off)
			}
			if sameInstance(recs[0], r) {
				_, m := tickOffset(clocks[0], clocks[i])
				return fmt.Errorf("%w: %d strikes and %d ticks shared, %d and %d wanted", ErrFightMismatch, n, m, MinStrikes, MinTicks)
			}
			return fmt.Errorf("%w: %d strikes shared, %d wanted", ErrFightMismatch, n, MinStrikes)
		}
		// arcdps 20260920 dates what comes from a message of the server
		// with an even time and the rest with an odd one: a time keeps its
		// parity from a recording to the combined log.
		if d := recs[0].epoch - r.epoch - r.off.Milliseconds(); d%2 != 0 {
			r.off += time.Millisecond
		}
		r.span = timeline.Interval{Start: -r.off, End: r.tl.Duration - r.off}
	}
	return nil
}
