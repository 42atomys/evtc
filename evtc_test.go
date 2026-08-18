package evtc

import (
	"math/rand/v2"
	"testing"
)

func TestEventBytesRoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 9))
	for range 200 {
		e := Event{
			Time: r.Uint64(), SrcAgent: r.Uint64(), DstAgent: r.Uint64(),
			Value: int32(r.Uint32()), BuffDamage: int32(r.Uint32()), OverstackValue: r.Uint32(), SkillID: r.Uint32(),
			SrcInstanceID: uint16(r.Uint32()), DstInstanceID: uint16(r.Uint32()), SrcMasterInstanceID: uint16(r.Uint32()), DstMasterInstanceID: uint16(r.Uint32()),
			IFF: IFF(r.Uint32()), Buff: uint8(r.Uint32()), Result: Result(r.Uint32()), IsActivation: Activation(r.Uint32()), IsBuffRemove: BuffRemove(r.Uint32()),
			IsNinety: uint8(r.Uint32()), IsFifty: uint8(r.Uint32()), IsMoving: uint8(r.Uint32()), IsStateChange: StateChange(r.Uint32()), IsFlanking: uint8(r.Uint32()),
			IsShields: uint8(r.Uint32()), IsOffcycle: uint8(r.Uint32()), Pad61: uint8(r.Uint32()), Pad62: uint8(r.Uint32()), Pad63: uint8(r.Uint32()), Pad64: uint8(r.Uint32()),
		}
		b := e.Bytes()
		if got := decodeEvents(b[:]); len(got) != 1 || got[0] != e {
			t.Fatalf("round trip changed the event: %+v became %+v", e, got)
		}
	}
}
