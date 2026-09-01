package timeline

import (
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestExtensions(t *testing.T) {
	const heal = 0x9c9b3c99
	b := fixture()
	b.extensionCombat(500, addrP1, addrP2, skillHeat, -300, heal) // written before the registration
	b.extension(1000, 0x07000002_9c9b3c99, "2.18rc1")
	b.extensionCombat(1500, addrP2, addrP1, skillHeat, -200, heal)
	b.extensionCombat(1600, addrP1, addrP1, skillSlam, -100, 0xDEAD) // unregistered signature
	b.extension(2000, heal, "dup")                                   // a second registration owns nothing
	b.extension(2500, 0x11, "\x01\x02")                              // not printable
	b.extension(2600, 0x22, "12345678")                              // fills the field
	b.extensionCombat(3000, 0, addrP1, skillHeat, -50, heal)         // unknown source
	tl := mustBuild(t, b.build(5000))

	if len(tl.Extensions) != 4 {
		t.Fatalf("extensions = %+v", tl.Extensions)
	}
	x := tl.Extensions[0]
	if x.Signature != heal || x.Version != "2.18rc1" || x.Time != time.Second || x.Event == nil || x.Events().Count() != 3 || x.Events().First().BuffDamage != -300 || x.Events().Last().BuffDamage != -50 {
		t.Errorf("extension = %+v, %d events", x, x.Events().Count())
	}
	if x.Events().Between(NewInterval(0, 1500*msec)).Count() != 2 || x.Events().Involving(tl.Players[1]).Count() != 2 {
		t.Errorf("extension events between = %d, involving p2 = %d", x.Events().Between(NewInterval(0, 1500*msec)).Count(), x.Events().Involving(tl.Players[1]).Count())
	}
	if dup := tl.Extensions[1]; dup.Signature != heal || dup.Version != "dup" || dup.Events().Count() != 0 || dup.Time != 2*time.Second {
		t.Errorf("duplicate registration = %+v", dup)
	}
	if raw, full := tl.Extensions[2], tl.Extensions[3]; raw.Signature != 0x11 || raw.Version != "" || full.Signature != 0x22 || full.Version != "12345678" || full.Events().Count() != 0 {
		t.Errorf("versions = %q %q", raw.Version, full.Version)
	}
	if tl.ExtensionEvents().Count() != 4 || tl.ExtensionEvents().Between(NewInterval(0, 1500*msec)).Count() != 2 {
		t.Errorf("extension events = %d", tl.ExtensionEvents().Count())
	}
	if p1 := tl.Players[0]; p1.Events().Of(evtc.StateExtensionCombat).Count() != 4 || tl.Unknown.Events().Of(evtc.StateExtensionCombat).Count() != 1 || tl.Hits().Count() != 0 {
		t.Errorf("extension events on agents: p1 %d, unknown %d, hits %d", p1.Events().Of(evtc.StateExtensionCombat).Count(), tl.Unknown.Events().Of(evtc.StateExtensionCombat).Count(), tl.Hits().Count())
	}
	if tl.Extension(heal) != x || tl.Extension(0xDEAD) != nil || tl.Extension(0x22) != tl.Extensions[3] || tl.ExtensionOf(x.Events().First()) != x || tl.ExtensionOf(nil) != nil || tl.ExtensionOf(tl.Events().First()) != nil {
		t.Errorf("extension lookups: %v %v %v", tl.Extension(heal), tl.Extension(0xDEAD), tl.ExtensionOf(x.Events().First()))
	}
	if orphan := tl.ExtensionEvents().Where(func(e *evtc.Event) bool { return extensionSignature(e) == 0xDEAD }).First(); orphan == nil || tl.ExtensionOf(orphan) != nil {
		t.Errorf("orphan extension event = %+v", orphan)
	}
	p1, p2 := tl.Players[0], tl.Players[1]
	if x.Events().By(p1).Count() != 1 || x.Events().On(p1).Count() != 2 || x.Events().By(p2).Count() != 1 || x.Events().On(p2).Count() != 1 || tl.ExtensionEvents().By(tl.Unknown).Count() != 1 || x.Events().By(nil).Count() != 0 || x.Events().On(nil).Count() != 0 {
		t.Errorf("by p1 %d, on p1 %d, by p2 %d, on p2 %d, by unknown %d", x.Events().By(p1).Count(), x.Events().On(p1).Count(), x.Events().By(p2).Count(), x.Events().On(p2).Count(), tl.ExtensionEvents().By(tl.Unknown).Count())
	}
	if sum := x.Events().On(p1).Sum(func(e *evtc.Event) int64 { return int64(e.BuffDamage) }); sum != -250 {
		t.Errorf("healing received by p1 = %d", sum)
	}
	if tl.Agent(0) != tl.Unknown || tl.Agent(0).Name != "Unknown" || tl.Agent(0xBAD) != nil {
		t.Errorf("agent 0 = %v", tl.Agent(0))
	}
	checkInvariants(t, tl)
}

// fakeDecoded is what fakeDecoder builds: what it saw of the graph at
// decode time.
type fakeDecoded struct {
	events int
	boss   *Target
	skill  *Skill
}

// fakeDecoder is an ExtensionDecoder for a signature of the tests.
type fakeDecoder struct {
	sig  uint32
	seen []*Extension
}

func (d *fakeDecoder) Signature() uint32 { return d.sig }

func (d *fakeDecoder) Decode(x *Extension) any {
	d.seen = append(d.seen, x)
	return fakeDecoded{events: x.Events().Count(), boss: x.Timeline.Boss(), skill: x.Timeline.Skill(4242)}
}

func TestExtensionDecoder(t *testing.T) {
	const sig = 0xFEED
	d := &fakeDecoder{sig: sig}
	RegisterExtension(d)
	b := fixture()
	b.extensionCombat(500, addrP1, addrP2, skillHeat, -300, sig)
	b.extension(1000, 0x07000002_0000FEED, "fake 1")
	b.extensionCombat(1500, addrP2, addrP1, 4242, -200, sig)    // a skill the table does not have
	b.extension(2000, sig, "again")                             // a second registration is not decoded
	b.extension(2500, uint64(ExtensionHealingStats), "2.18rc1") // no decoder registered here
	b.extensionCombat(3000, addrP1, addrP1, 4243, -100, 0xBEEF)
	tl := mustBuild(t, b.build(5000))

	x := tl.Extension(sig)
	if x == nil || x.Timeline != tl || len(d.seen) != 1 || d.seen[0] != x {
		t.Fatalf("extension = %+v, decoder saw %v", x, d.seen)
	}
	// The decoder ran on the complete graph: the boss and the skills of
	// the extension events were there.
	if got, ok := x.Decoded.(fakeDecoded); !ok || got.events != 2 || got.boss == nil || got.boss.Agent != tl.Agent(addrBoss) || got.skill == nil || got.skill.Name != "4242" {
		t.Errorf("decoded = %#v", x.Decoded)
	}
	if dup := tl.Extensions[1]; dup.Signature != sig || dup.Decoded != nil || dup.Timeline != tl {
		t.Errorf("second registration = %+v", dup)
	}
	if h := tl.Extension(ExtensionHealingStats); h.Decoded != nil {
		t.Errorf("extension without a decoder was decoded: %+v", h.Decoded)
	}
	// Every extension combat event names a skill of the graph, as arcdps
	// adds it to the skill table, attached to a registration or not.
	for _, id := range []uint32{skillHeat, 4242, 4243} {
		if s := tl.Skill(id); s == nil {
			t.Errorf("skill %d of an extension event is missing", id)
		}
	}
	// A later registration for the signature replaces the decoder.
	d2 := &fakeDecoder{sig: sig}
	RegisterExtension(d2)
	if tl2 := mustBuild(t, b.build(5000)); len(d2.seen) != 1 || len(d.seen) != 1 || tl2.Extension(sig).Decoded.(fakeDecoded).events != 2 {
		t.Errorf("replaced decoder saw %d, the old one %d", len(d2.seen), len(d.seen))
	}
	checkInvariants(t, tl)

	defer func() {
		if recover() == nil {
			t.Error("RegisterExtension(nil) did not panic")
		}
	}()
	RegisterExtension(nil)
}
