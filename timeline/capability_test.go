package timeline

import (
	"slices"
	"strings"
	"testing"

	"github.com/42atomys/evtc"
)

func TestHas(t *testing.T) {
	// The fixture is a log of arcdps 20260816, which writes teleports and
	// no jump yet.
	tl := mustBuild(t, fixture().build(1000))
	if !tl.Has(evtc.CapabilityTeleports) || tl.Has(evtc.CapabilityJumps) || tl.Has(evtc.Capability(200)) {
		t.Errorf("capabilities = %v", tl.Capabilities())
	}
	if len(tl.Warnings()) != 0 {
		t.Errorf("warnings = %v", tl.Warnings())
	}
	checkInvariants(t, tl)
}

// TestHasByProof checks the events of an older build can prove a
// capability, here a jump and a tick carrying a ping, and that Missing
// follows.
func TestHasByProof(t *testing.T) {
	b := fixture()
	b.tick(100, 1, 45)
	b.jump(200, addrP1, true)
	tl := mustBuild(t, b.build(1000))
	if !tl.Has(evtc.CapabilityPing) || !tl.Has(evtc.CapabilityJumps) || tl.Has(evtc.CapabilityGadgetModels) {
		t.Errorf("capabilities = %v", tl.Capabilities())
	}
	// The fixture holds no GUID event.
	if want := []evtc.Capability{evtc.CapabilityGUIDs, evtc.CapabilityGadgetModels, evtc.CapabilityFlyTo}; !slices.Equal(tl.Missing(), want) {
		t.Errorf("Missing = %v, want %v", tl.Missing(), want)
	}
}

// TestWarnings checks the warnings of the log reach the timeline: here a
// strike naming a skill while the skill table is empty.
func TestWarnings(t *testing.T) {
	b := fixture()
	b.hit(100, addrP1, addrBoss, skillSlam, 500, evtc.ResultStrikeDamageNormal)
	l := b.build(1000)
	l.Skills = nil
	tl := mustBuild(t, l)
	w := tl.Warnings()
	if len(w) != 1 || w[0].Code != evtc.WarningNoSkills {
		t.Errorf("warnings = %v", w)
	}
}

// TestBuildRejectsInvalidDate checks Build and the log agree on what a
// build date is: a number that is no date builds nothing.
func TestBuildRejectsInvalidDate(t *testing.T) {
	l := fixture().build(1000)
	l.Header.Build = "20269999"
	if _, err := Build(l); err == nil || !strings.Contains(err.Error(), "invalid build date") {
		t.Errorf("err = %v", err)
	}
}

// capabilityProbe is an ExtensionDecoder that asks Has from inside Build.
type capabilityProbe struct{ teleports, jumps bool }

func (p *capabilityProbe) Signature() uint32 { return 0x7e57ca9a }

func (p *capabilityProbe) Decode(x *Extension) any {
	p.teleports = x.Timeline.Has(evtc.CapabilityTeleports)
	p.jumps = x.Timeline.Has(evtc.CapabilityJumps)
	return nil
}

func TestHasDuringBuild(t *testing.T) {
	probe := &capabilityProbe{}
	RegisterExtension(probe)
	b := fixture()
	b.extension(0, uint64(probe.Signature()), "probe")
	mustBuild(t, b.build(1000))
	if !probe.teleports || probe.jumps {
		t.Errorf("the decoder saw %+v", probe)
	}
}
