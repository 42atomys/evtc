package timeline

import (
	"bytes"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

// FuzzBuild feeds arbitrary bytes through the decoder and the builder: any
// log that decodes must build without panicking and satisfy the graph
// invariants.
func FuzzBuild(f *testing.F) {
	f.Add(encodeLog(fixture().build(5000)))
	f.Add(encodeLog(genLog(genOptions{players: 2, adds: 2, duration: 10 * time.Second, seed: 1})))
	_, legacy := asLegacy(genLog(genOptions{players: 2, adds: 2, duration: 10 * time.Second, seed: 2}))
	f.Add(encodeLog(legacy))
	f.Fuzz(func(t *testing.T, data []byte) {
		l, err := evtc.Parse(bytes.NewReader(data))
		if err != nil {
			return
		}
		tl, err := Build(l)
		if err != nil {
			return
		}
		checkInvariants(t, tl)
	})
}

// FuzzEvents keeps the agent and skill tables of the fixture and fuzzes the
// event stream only, which reaches the builder far more often than
// FuzzBuild.
func FuzzEvents(f *testing.F) {
	prefix := encodeLog(fixture().build(5000))
	prefix = prefix[:len(prefix)-64*2] // drop the combat start and end events

	b := fixture()
	b.castStart(1000, addrBoss, addrP1, skillHeat, 484, 716)
	b.hit(1500, addrBoss, addrP1, skillHeat, 0, evtc.ResultBlock)
	b.castStop(2000, addrBoss, skillHeat, 1000, evtc.ActivationReset)
	b.buffApply(1000, addrP2, addrP1, skillBuff, 5000, 7)
	b.buffRemoveSingle(3000, addrP1, 0, skillBuff, 0, 7, evtc.BuffRemoveSingle)
	b.hit(2990, addrBoss, addrP1, skillHeat, 0, evtc.ResultDowned)
	b.state(3000, addrP1, evtc.StateChangeDown)
	b.state(4000, addrP1, evtc.StateChangeUp)
	b.move(1000, addrP1, evtc.StatePosition, 1, 2, 3)
	b.health(1000, addrBoss, 90)
	b.defianceState(1000, addrAdd, DefianceActive)
	b.minionHit(1500, addrPet, addrBoss, instP1, skillHeat, 50)
	b.legacyEffect(1200, addrBoss, addrP1, 900, 7, 4000, Vec3{}, [3]int16{}, false)
	l := b.build(5000)
	f.Add(encodeEvents(l.Events[1:]))
	f.Add(encodeEvents(l.Events[len(l.Events)-3:]))
	_, legacy := asLegacy(l)
	f.Add(encodeEvents(legacy.Events[1:]))

	f.Fuzz(func(t *testing.T, data []byte) {
		data = data[:len(data)-len(data)%64]
		l, err := evtc.Parse(bytes.NewReader(append(append([]byte{}, prefix...), data...)))
		if err != nil {
			return
		}
		tl, err := Build(l)
		if err != nil {
			return
		}
		checkInvariants(t, tl)
		// Under an older header the same events take the path of the
		// format before typed events, unless they hold some.
		l.Header.Build = "20240613"
		if tl, err = Build(l); err == nil {
			checkInvariants(t, tl)
		}
	})
}
