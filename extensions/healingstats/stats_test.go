package healingstats

import (
	"testing"

	"github.com/42atomys/evtc/timeline"
)

func TestOf(t *testing.T) {
	b := fixture()
	b.register(0, "2.19rc2", 2)
	b.heal(1000, addrAlpha, addrBravo, skillHeal, 100, fromSrc)
	tl, err := timeline.Build(b.build(5000))
	if err != nil {
		t.Fatal(err)
	}
	x := tl.Extension(Signature)
	s, ok := x.Decoded.(*Stats)
	if !ok || s == nil || Of(tl) != s || s.Extension != x || s.Timeline != tl || s.Version != "2.19rc2" || s.Revision != 2 || s.Merged != 0 || s.Heals().Count() != 1 {
		t.Fatalf("decoded = %#v", x.Decoded)
	}
	if d := (decoder{}); d.Signature() != Signature || Signature != timeline.ExtensionHealingStats {
		t.Errorf("decoder signature = %#x", d.Signature())
	}
	if s.Players().Count() != 3 || s.Agents().Count() != 5 || s.Unknown == nil || len(s.Recorded) != 1 {
		t.Errorf("nodes: %d players, %d agents, %d recorded", s.Players().Count(), s.Agents().Count(), len(s.Recorded))
	}
	checkInvariants(t, s)
}
