package combine

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

// mustRecord builds the recordings of the logs, their clocks set.
func mustRecord(tb testing.TB, logs ...*evtc.Log) []*recording {
	tb.Helper()
	recs, err := recordings(logs)
	if err != nil {
		tb.Fatal(err)
	}
	return recs
}

func TestOffsetOf(t *testing.T) {
	a := map[strike][]time.Duration{
		{skill: 1}: ms(100), {skill: 2}: ms(200), {skill: 3}: ms(300),
		{skill: 4}: ms(400, 450), // told twice by a: no gap
		{skill: 5}: ms(500),      // told twice by b
		{skill: 6}: ms(600),      // missing from b
	}
	b := map[strike][]time.Duration{
		{skill: 1}: ms(110), {skill: 2}: ms(212), {skill: 3}: ms(340),
		{skill: 4}: ms(410), {skill: 5}: ms(510, 520),
	}
	if off, n := offsetOf(a, b); off != 12*time.Millisecond || n != 3 {
		t.Errorf("offset = %v over %d strikes, want the median 12ms over 3", off, n)
	}
	if off, n := offsetOf(a, nil); off != 0 || n != 0 {
		t.Errorf("offset = %v over %d strikes, want none", off, n)
	}

	// Strikes two fights share by chance tell gaps far apart: those that
	// stand together count.
	a, b = map[strike][]time.Duration{}, map[strike][]time.Duration{}
	for i, gap := range []int{-200_000, -90_000, -40_000, 1480, 1500, 1500, 1510, 3400, 60_000, 61_000, 150_000, 400_000} {
		a[strike{skill: uint32(i)}] = ms(1000 * i)
		b[strike{skill: uint32(i)}] = ms(1000*i + gap)
	}
	if off, n := offsetOf(a, b); off != 1500*time.Millisecond || n != 5 {
		t.Errorf("offset = %v over %d strikes, want 1.5s over 5", off, n)
	}
}

func TestTickOffset(t *testing.T) {
	ms := func(v int) time.Duration { return time.Duration(v) * time.Millisecond }
	a := ticks{tick: []int64{100, 125, 150, 175}, at: []time.Duration{ms(1000), ms(2000), ms(3000), ms(4000)}}
	// The second recording names the ticks one later, 40 ms after, and
	// runs 12 ms behind.
	b := ticks{tick: []int64{101, 126, 151, 176}, at: []time.Duration{ms(1052), ms(2052), ms(3052), ms(4052)}}
	if off, n := tickOffset(a, b); off != 12*time.Millisecond || n != 3 {
		t.Errorf("offset = %v over %d ticks, want 12ms over 3", off, n)
	}
	if off, n := tickOffset(a, ticks{}); off != 0 || n != 0 {
		t.Errorf("offset = %v over %d ticks, want none", off, n)
	}
	// A hole wider than two events is not crossed.
	c := ticks{tick: []int64{100, 200}, at: []time.Duration{ms(1000), ms(5000)}}
	if _, ok := c.timeAt(150); ok {
		t.Error("a tick is placed across a hole of 100 ticks")
	}
	if at, ok := c.timeAt(200); !ok || at != ms(5000) {
		t.Errorf("the time of a tick named = %v, %v", at, ok)
	}
}
