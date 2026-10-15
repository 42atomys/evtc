package combine

import (
	"bytes"
	"testing"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// encodeEvents writes events in the wire format, for the fuzzer.
func encodeEvents(events []evtc.Event) []byte {
	out := make([]byte, 0, 64*len(events))
	for i := range events {
		b := events[i].Bytes()
		out = append(out, b[:]...)
	}
	return out
}

// fuzzScene returns two recordings of a fight and the events the second
// client wrote beyond the first strikes: a crow that comes and goes, its
// samples and its buffs, a minion, heals of an addon.
func fuzzScene() (first, second *evtc.Log, extra []evtc.Event) {
	s := newScene(2)
	a, b := s.recs[0], s.recs[1]
	s.each(func(r *recorder) {
		r.npc(crow, 1083, "Crow")
		r.npc(pet, 6524, "Elemental")
	})
	a.spawn(1000, crow)
	a.position(1100, crow, 10, 20)
	a.health(1500, crow, 80)
	a.apply(1600, alpha, crow, skillMight, 9000, 5)
	a.hit(2500, bravo, crow, skillSlam, 700)
	a.despawn(3000, crow)
	from := len(b.l.Events)
	b.spawn(2000, crow)
	b.initial(2001, alpha, crow, skillMight, 8600, 5)
	b.health(2001, crow, 80)
	b.position(2101, crow, 30, 20)
	b.hit(2504, bravo, crow, skillSlam, 700)
	b.marker(2600, crow, 3)
	b.team(2700, crow, 5, 3)
	b.ev(2800, evtc.StateCombat, pet, boss, evtc.Event{SkillID: skillSlam, Value: 50, SrcMasterInstanceID: alpha, IFF: evtc.IFFFoe})
	b.heal(2900, alpha, bravo, skillRegen, 900, 0x80)
	b.remove(3500, crow, skillMight, 5)
	b.l.Events = append(b.l.Events, evtc.Event{Time: b.at(3900), SrcInstanceID: crow, IsStateChange: evtc.StateChangeDead})
	b.despawn(4000, crow)
	extra = append(extra, b.l.Events[from:]...)
	b.l.Events = b.l.Events[:from]
	logs := s.logs(5000)
	return logs[0], logs[1], extra
}

// FuzzLogs keeps a recording of a fight and the tables of a second one,
// and fuzzes the events of the latter: the logs that combine must give a
// log that builds, the same one whatever the order given.
func FuzzLogs(f *testing.F) {
	first, second, extra := fuzzScene()
	var prefix bytes.Buffer
	if _, err := second.WriteTo(&prefix); err != nil {
		f.Fatal(err)
	}
	f.Add(encodeEvents(extra))
	f.Add(encodeEvents(extra[:4]))
	f.Add(encodeEvents(extra[len(extra)-3:]))

	f.Fuzz(func(t *testing.T, data []byte) {
		data = data[:len(data)-len(data)%64]
		second, err := evtc.Parse(bytes.NewReader(append(append([]byte{}, prefix.Bytes()...), data...)))
		if err != nil {
			return
		}
		want, err := Logs(first, second)
		if err != nil {
			return
		}
		got, err := Logs(second, first)
		if err != nil || !sameLog(got, want) {
			t.Fatalf("the order given matters: %v", err)
		}
		var last uint64
		for i := range got.Events {
			if at(&got.Events[i]) < last {
				t.Fatalf("event %d comes after a later one", i)
			}
			last = at(&got.Events[i])
		}
		if _, err := timeline.Build(got); err != nil {
			t.Fatalf("the combined log does not build: %v", err)
		}
	})
}
