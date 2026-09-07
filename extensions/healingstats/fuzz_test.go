package healingstats

import (
	"bytes"
	"testing"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// FuzzDecode keeps the agent and skill tables of the fixture and its
// registration of the addon, and fuzzes the event stream: any log that
// decodes must build without panicking and satisfy the invariants of the
// stats.
func FuzzDecode(f *testing.F) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	prefix := encodeLog(b.build(5000))

	b = fixture()
	b.heal(1000, addrBravo, addrAlpha, skillHeal, 300, fromDst)
	b.heal(1120, addrBravo, addrAlpha, skillHeal, 300, fromSrc)
	b.tick(2000, addrAlpha, addrAlpha, skillRegen, 130, both|flagDowned)
	b.barrier(2500, addrCharlie, addrAlpha, skillSand, 1200, fromDst)
	b.minionHeal(3000, addrPet, instAlpha, addrCharlie, skillHeal, 50, fromSrc)
	b.castStart(3500, addrAlpha, 0, skillHeal, 500)
	b.heal(3700, addrAlpha, addrBravo, skillHeal, 900, fromSrc)
	b.castStop(4000, addrAlpha, skillHeal, 500)
	b.heal(4200, 0, addrAlpha, 4242, 10, fromDst)
	l := b.build(5000)
	f.Add(encodeEvents(l.Events[len(l.Events)-14:]))
	f.Add(encodeEvents(l.Events[len(l.Events)-3:]))

	f.Fuzz(func(t *testing.T, data []byte) {
		data = data[:len(data)-len(data)%64]
		l, err := evtc.Parse(bytes.NewReader(append(append([]byte{}, prefix...), data...)))
		if err != nil {
			return
		}
		tl, err := timeline.Build(l)
		if err != nil {
			return
		}
		if s := Of(tl); s != nil {
			checkInvariants(t, s)
		}
	})
}
