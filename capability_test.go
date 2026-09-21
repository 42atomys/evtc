package evtc

import (
	"bytes"
	"os"
	"slices"
	"sync"
	"testing"
)

// logOf assembles a log by hand, as a test of another package would.
func logOf(build string, events ...Event) *Log {
	return &Log{Header: Header{Build: build, Revision: 1}, Events: events}
}

func TestCapabilityTable(t *testing.T) {
	if capabilityCount > 128 {
		t.Fatalf("%d capabilities do not fit the set", capabilityCount)
	}
	last := 0
	for c := range capabilityCount {
		info := capabilityTable[c]
		// Parse reads revision 1 alone, the default since the release of
		// 20181002.
		if info.name == "" || info.build < 20181002 {
			t.Errorf("capability %d: name %q, build %d", c, info.name, info.build)
		}
		if info.build < last {
			t.Errorf("%v: build %d comes after %d", c, info.build, last)
		}
		last = info.build
		if c.String() != info.name || c.FirstBuild() != info.build {
			t.Errorf("%d: String %q, FirstBuild %d", c, c.String(), c.FirstBuild())
		}
	}
	if got := Capability(200); got.String() != "Capability(200)" || got.FirstBuild() != 0 {
		t.Errorf("unknown capability: %q, %d", got.String(), got.FirstBuild())
	}
}

func TestHasByBuild(t *testing.T) {
	tests := []struct {
		build string
		c     Capability
		want  bool
	}{
		{"20181213", CapabilityStackIdentity, false},
		{"20181214", CapabilityStackIdentity, true},
		{"20260430", CapabilityTypedEvents, false},
		{"20260501", CapabilityTypedEvents, true},
		{"20260816", CapabilityTeleports, true},
		{"20260816", CapabilityJumps, false},
		{" 20260915", CapabilityJumps, true},
		{"20260916", Capability(200), false},
		{"99999999", CapabilityJumps, false},
		{"2026091", CapabilityStackIdentity, false},
		{"20151231", CapabilityMapID, false},
		{"20261301", CapabilityMapID, false},
		{"20260100", CapabilityMapID, false},
		{"20260132", CapabilityMapID, false},
		{"+2026081", CapabilityMapID, false},
	}
	for _, tt := range tests {
		if got := logOf(tt.build).Has(tt.c); got != tt.want {
			t.Errorf("build %q: Has(%v) = %v, want %v", tt.build, tt.c, got, tt.want)
		}
	}
}

// TestHasByProof checks what the events prove on a build too old for the
// capability, and what they must not.
func TestHasByProof(t *testing.T) {
	const old = "20181002"
	tests := []struct {
		name  string
		event Event
		c     Capability
		want  bool
	}{
		{"kind", Event{IsStateChange: StateStealthChange}, CapabilityStealth, true},
		{"one kind of several", Event{IsStateChange: StateBuffChange}, CapabilityTypedEvents, true},
		{"retired effect kind", Event{IsStateChange: StateEffect2Defunc}, CapabilityEffects, true},
		{"another kind", Event{IsStateChange: StateStealthChange}, CapabilityJumps, false},

		{"defiance strike", Event{Result: ResultDefianceDamageNormal}, CapabilityDefianceDamage, true},
		{"skill cast strike", Event{Result: ResultSkillCast}, CapabilityCastSignals, true},
		{"ten stacks removed", Event{Result: 10, IsBuffRemove: BuffRemoveAll, Buff: 1}, CapabilityDefianceDamage, false},
		{"result of a cast", Event{Result: 10, IsActivation: ActivationReset}, CapabilityDefianceDamage, false},
		{"result of a tick", Event{Result: 10, Buff: 1}, CapabilityDefianceDamage, false},
		{"flank angle", Event{IsFlanking: 90}, CapabilityFlankAngle, true},
		{"flank flag", Event{IsFlanking: 1}, CapabilityFlankAngle, false},
		{"target moving", Event{IsMoving: 2}, CapabilityTargetMoving, true},
		{"source moving", Event{IsMoving: 1}, CapabilityTargetMoving, false},
		{"launch flags of a missile", Event{IsStateChange: StateMissileLaunch, IsMoving: 2}, CapabilityTargetMoving, false},

		{"trackable id", Event{IsStateChange: StateBuffApply, Pad63: 1}, CapabilityStackIdentity, true},
		{"trackable id of an initial buff", Event{IsStateChange: StateBuffInitial, Pad61: 1}, CapabilityStackIdentity, true},
		{"application without id", Event{IsStateChange: StateBuffInitial}, CapabilityStackIdentity, false},
		{"original duration", Event{IsStateChange: StateBuffInitial, BuffDamage: 5000}, CapabilityOriginalDuration, true},
		{"buff damage of an application", Event{IsStateChange: StateBuffApply, BuffDamage: 5000}, CapabilityOriginalDuration, false},
		{"profession on enter combat", Event{IsStateChange: StateEnterCombat, Value: 4}, CapabilitySpecChanges, true},
		{"old team", Event{IsStateChange: StateTeamChange, Value: 705}, CapabilityPreviousTeam, true},
		{"old weapon set", Event{IsStateChange: StateWeaponSwap, Value: 4}, CapabilityPreviousWeaponSet, true},
		{"markers removed", Event{IsStateChange: StateMarker}, CapabilityMarkerChanges, true},
		{"marker set", Event{IsStateChange: StateMarker, Value: 7}, CapabilityMarkerChanges, false},
		{"map exit", Event{IsStateChange: StateSquadCombatEnd, DstAgent: 1}, CapabilityMapExit, true},
		{"ping", Event{IsStateChange: StateTick, Value: 42}, CapabilityPing, true},
		{"tick without ping", Event{IsStateChange: StateTick}, CapabilityPing, false},
	}
	for _, tt := range tests {
		if got := logOf(old, tt.event).Has(tt.c); got != tt.want {
			t.Errorf("%s: Has(%v) = %v, want %v", tt.name, tt.c, got, tt.want)
		}
	}
}

// TestHasGUIDs checks the one capability that needs its build and its
// kind: GUID events older than the build are defective, and a newer log
// without any holds no GUID.
func TestHasGUIDs(t *testing.T) {
	guid := Event{IsStateChange: StateIDToGUID}
	tests := []struct {
		build  string
		events []Event
		want   bool
	}{
		{"20220708", []Event{guid}, false},
		{"20220709", nil, false},
		{"20220709", []Event{guid}, true},
		// Without a build date nothing tells the GUIDs are sound.
		{"garbage!", []Event{guid, {IsStateChange: StateStealthChange}}, false},
	}
	for _, tt := range tests {
		if got := logOf(tt.build, tt.events...).Has(CapabilityGUIDs); got != tt.want {
			t.Errorf("build %s, %d events: Has = %v, want %v", tt.build, len(tt.events), got, tt.want)
		}
	}
}

func TestHasWithoutBuild(t *testing.T) {
	l := logOf("garbage!", Event{IsStateChange: StateStealthChange})
	if l.Has(CapabilityStackIdentity) || !l.Has(CapabilityStealth) {
		t.Errorf("capabilities without a build date: %v", l.Capabilities())
	}
	var none *Log
	if none.Has(CapabilityStealth) || none.Capabilities() != nil || none.Missing() != nil {
		t.Error("a nil log has capabilities")
	}
}

func TestCapabilitiesAndMissing(t *testing.T) {
	l := logOf("20260816")
	has, missing := l.Capabilities(), l.Missing()
	// GUIDs need their kind, whatever the build.
	want := []Capability{CapabilityGUIDs, CapabilityJumps, CapabilityPing, CapabilityGadgetModels}
	if !slices.Equal(missing, want) {
		t.Errorf("Missing = %v, want %v", missing, want)
	}
	if len(has)+len(missing) != int(capabilityCount) || !slices.IsSorted(has) {
		t.Errorf("%d capabilities and %d missing of %d", len(has), len(missing), capabilityCount)
	}
}

// TestObservedParsedLog checks a parsed log scans its events once, gives
// the answer of a log assembled by hand, follows its build date and does
// not answer for its copies.
func TestObservedParsedLog(t *testing.T) {
	raw := buildLog()
	copy(raw[4:], "20181002")
	l, err := Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !l.Has(CapabilityTypedEvents) || !l.seen.kinds.has(StateBuffApply) || l.seen.kinds.has(StateBuffChange) {
		t.Fatalf("seen = %+v", l.seen)
	}
	hand := &Log{Header: l.Header, Agents: l.Agents, Skills: l.Skills, Events: l.Events}
	if !slices.Equal(hand.Capabilities(), l.Capabilities()) {
		t.Errorf("by hand %v, parsed %v", hand.Capabilities(), l.Capabilities())
	}
	if n := testing.AllocsPerRun(100, func() { _ = l.Has(CapabilityStealth) }); n != 0 {
		t.Errorf("Has allocates %v times", n)
	}

	// The events are not read again, the build date is.
	stale := *l
	l.Events[0].IsStateChange = StateStealthChange
	if l.Has(CapabilityStealth) || !l.Has(CapabilityTypedEvents) {
		t.Error("the events of a parsed log were scanned again")
	}
	l.Header.Build = "20260602"
	if !l.Has(CapabilityStealth) {
		t.Error("the new build date is ignored")
	}
	// A copy is scanned for itself.
	if stale.Has(CapabilityTypedEvents) || !stale.Has(CapabilityStealth) {
		t.Errorf("a copy answers %v", stale.Capabilities())
	}
}

// TestHasConcurrent is meant for -race: the first calls on a parsed log
// share one scan. The build is one Warnings scans the events for.
func TestHasConcurrent(t *testing.T) {
	raw := buildLog()
	copy(raw[4:], "20241210")
	l, err := Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if !l.Has(CapabilityTypedEvents) || len(l.Warnings()) != 0 {
				t.Error("wrong answer under concurrency")
			}
		})
	}
	wg.Wait()
}

// TestCapabilitiesSample pins the capabilities of the real log kept in
// tests_fixtures/ when present: arcdps 20260816 wrote no jump, no ping and
// no gadget model yet.
func TestCapabilitiesSample(t *testing.T) {
	const path = "tests_fixtures/sabetha-05-fd9b6f3a.zevtc"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s not available", path)
	}
	l, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Capability{CapabilityJumps, CapabilityPing, CapabilityGadgetModels}; !slices.Equal(l.Missing(), want) {
		t.Errorf("Missing = %v, want %v", l.Missing(), want)
	}
}

// BenchmarkObserveSample measures the scan behind the first Has, on the
// events of the real log.
func BenchmarkObserveSample(b *testing.B) {
	raw, err := unzip(sampleBytes(b))
	if err != nil {
		b.Fatal(err)
	}
	l, err := decode(raw)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		observe(l.Events)
	}
}
