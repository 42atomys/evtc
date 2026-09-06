package healingstats

import (
	"math/rand/v2"

	"github.com/42atomys/evtc"
)

// genLog generates a deterministic synthetic log with the events of the
// addon: Alpha records it, Bravo shares its stats, so the heals between
// them are written twice, Charlie does not, and the pet of Alpha heals as
// well. Alpha casts its healing skill every two seconds.
func genLog(seed uint64, seconds int) *evtc.Log {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	b := fixture()
	b.register(0, "gen 1.0", SupportedRevision)
	agents := []uint64{addrAlpha, addrBravo, addrCharlie, addrPet}
	recorded := func(a uint64) bool { return a == addrAlpha || a == addrBravo || a == addrPet }
	end := uint64(seconds) * 1000
	for t := uint64(100); t < end-100; t += 100 {
		if t%2000 == 0 {
			b.castStart(t, addrAlpha, addrBravo, skillHeal, 500)
			b.castStop(t+500, addrAlpha, skillHeal, 500)
		}
		if r.IntN(2) == 0 {
			continue
		}
		src, dst := agents[r.IntN(len(agents))], agents[r.IntN(3)]
		amount := int32(50 + r.IntN(500))
		var flags uint8
		if recorded(src) {
			flags |= fromSrc
		}
		if recorded(dst) {
			flags |= fromDst
		}
		if flags == 0 {
			continue
		}
		if r.IntN(10) == 0 {
			flags |= flagArcDowned
		}
		kind := r.IntN(4)
		write := func(t uint64, flags uint8) {
			switch {
			case kind == 0:
				b.tick(t, src, dst, skillRegen, amount, flags)
			case kind == 1:
				b.barrier(t, src, dst, skillSand, amount, flags)
			case src == addrPet:
				b.minionHeal(t, src, instAlpha, dst, skillHeal, amount, flags)
			default:
				b.heal(t, src, dst, skillHeal, amount, flags)
			}
		}
		// A heal between the two recording clients is written by both, the
		// copy of the squad member a little later.
		if flags&both == both && src != dst && (src == addrBravo || dst == addrBravo) {
			write(t, flags&^fromDst)
			write(t+uint64(20+r.IntN(200)), flags&^fromSrc)
			continue
		}
		write(t, flags)
	}
	return b.build(end)
}

// encodeEvents writes events in the wire format, for the fuzzer.
func encodeEvents(events []evtc.Event) []byte {
	var out []byte
	for i := range events {
		b := events[i].Bytes()
		out = append(out, b[:]...)
	}
	return out
}
