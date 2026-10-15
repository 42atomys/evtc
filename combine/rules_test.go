package combine

import (
	"testing"

	"github.com/42atomys/evtc"
)

// TestRulesCoverKinds checks that every kind the decoder names has a
// rule: a kind a later arcdps adds must get one here.
func TestRulesCoverKinds(t *testing.T) {
	for k := range evtc.StateUnknown {
		if _, ok := rules[k]; !ok {
			t.Errorf("%v has no rule", k)
		}
	}
	for k, r := range rules {
		if k >= evtc.StateUnknown {
			t.Errorf("%v is no kind of the decoder", k)
		}
		switch r.class {
		case classServer, classBound:
			if r.key == nil {
				t.Errorf("%v: a message without a key", k)
			}
		case classClient:
			if (r.slot == slotNone) != (r.value == nil) {
				t.Errorf("%v: slot %d and value do not go together", k, r.slot)
			}
		}
		if r.keySrc && !r.src || r.keyDst && !r.dst {
			t.Errorf("%v is named by a field that holds no agent", k)
		}
	}
	// A kind without a rule is kept from the first recording only.
	if r := ruleOf(&evtc.Event{IsStateChange: evtc.StateUnknown}, evtc.StateUnknown, true); r.class != classRecording || r.src || r.dst {
		t.Errorf("rule of an unknown kind = %+v", r)
	}
}

func TestKindOf(t *testing.T) {
	tests := []struct {
		name string
		e    evtc.Event
		want evtc.StateChange
	}{
		{"strike", evtc.Event{Value: 100}, evtc.StateCombat},
		{"buff tick", evtc.Event{Buff: 1, BuffDamage: 50}, evtc.StateCombat},
		{"cast start", evtc.Event{IsActivation: evtc.ActivationStartDefunc}, evtc.StateAnimationStart},
		{"cast start under quickness", evtc.Event{IsActivation: evtc.ActivationQuicknessDefunc}, evtc.StateAnimationStart},
		{"cast end", evtc.Event{IsActivation: evtc.ActivationReset}, evtc.StateAnimationStop},
		{"every stack removed", evtc.Event{Buff: 1, IsBuffRemove: evtc.BuffRemoveAll}, evtc.StateBuffRemoveAll},
		{"one stack removed", evtc.Event{Buff: 1, IsBuffRemove: evtc.BuffRemoveSingle}, evtc.StateBuffRemoveSingle},
		{"buff application", evtc.Event{Buff: 1, Value: 5000}, evtc.StateBuffApply},
		{"duration change", evtc.Event{Buff: 1, Value: 5000, IsOffcycle: 1}, evtc.StateBuffChange},
		{"state", evtc.Event{IsStateChange: evtc.StateSpawn, IsActivation: evtc.ActivationReset}, evtc.StateSpawn},
	}
	for _, tt := range tests {
		if got := kindOf(&tt.e, false); got != tt.want {
			t.Errorf("%s: kind = %v, want %v", tt.name, got, tt.want)
		}
		if got := kindOf(&tt.e, true); got != tt.e.IsStateChange {
			t.Errorf("%s: kind of a typed event = %v", tt.name, got)
		}
	}
}

func TestRuleOf(t *testing.T) {
	tests := []struct {
		name  string
		e     evtc.Event
		typed bool
		class class
		dst   bool
	}{
		{"strike", evtc.Event{Value: 100}, true, classServer, true},
		{"buff tick", evtc.Event{Buff: 1, BuffDamage: 50}, true, classClient, true},
		{"team change", evtc.Event{IsStateChange: evtc.StateTeamChange, DstAgent: 1891}, true, classClient, false},
		{"team change to no team", evtc.Event{IsStateChange: evtc.StateTeamChange}, true, classBound, false},
		{"despawn", evtc.Event{IsStateChange: evtc.StateDespawn}, true, classBound, false},
		{"cast start", evtc.Event{IsStateChange: evtc.StateAnimationStart}, true, classServer, true},
		{"cast start of the older format", evtc.Event{IsActivation: evtc.ActivationStartDefunc}, false, classServer, false},
		{"duration change", evtc.Event{IsStateChange: evtc.StateBuffChange}, true, classServer, true},
		{"effect on the ground", evtc.Event{IsStateChange: evtc.StateEffect2Defunc}, true, classServer, false},
		{"effect on an agent", evtc.Event{IsStateChange: evtc.StateEffect2Defunc, DstAgent: 0x2001}, true, classServer, true},
		{"address change", evtc.Event{IsStateChange: evtc.StateIIDChange}, true, classDropped, false},
		{"reward", evtc.Event{IsStateChange: evtc.StateReward}, true, classServer, false},
		{"language", evtc.Event{IsStateChange: evtc.StateLanguage}, true, classRecording, false},
		{"tick", evtc.Event{IsStateChange: evtc.StateTick}, true, classLocal, false},
		{"heal of an addon", evtc.Event{IsStateChange: evtc.StateExtensionCombat}, true, classExtension, true},
	}
	for _, tt := range tests {
		r := ruleOf(&tt.e, kindOf(&tt.e, tt.typed), tt.typed)
		if r.class != tt.class || r.dst != tt.dst {
			t.Errorf("%s: class %d, dst %v", tt.name, r.class, r.dst)
		}
	}
	// A duration change of the older format names who extended the stack.
	e := evtc.Event{Buff: 1, Value: 5000, IsOffcycle: 1}
	if r := ruleOf(&e, kindOf(&e, false), false); !r.src || !r.dst {
		t.Errorf("duration change of the older format: src %v, dst %v", r.src, r.dst)
	}
}

func TestTimed(t *testing.T) {
	for k := range evtc.StateUnknown {
		want := true
		switch k {
		case evtc.StateBuffInfo, evtc.StateBuffFormula, evtc.StateSkillInfo, evtc.StateSkillTiming,
			evtc.StateIntegrity, evtc.StateIDToGUID, evtc.StateArcBuild:
			want = false
		}
		if timed(k) != want {
			t.Errorf("timed(%v) = %v", k, !want)
		}
		// A definition has no time, and nothing else is one.
		if def := rules[k].class == classDefinition; def && timed(k) {
			t.Errorf("%v is a definition with a time", k)
		}
	}
}

// TestTracks checks that the kinds that tell the state of their source
// name it: the rule of each holds an address in src_agent.
func TestTracks(t *testing.T) {
	n := 0
	for k := range evtc.StateUnknown {
		if !tracks(k) {
			continue
		}
		n++
		if r := rules[k]; !r.src || r.class.ofRecording() || r.class == classDefinition {
			t.Errorf("%v tells a state of no agent", k)
		}
	}
	if n != 30 {
		t.Errorf("%d kinds tell a state, want 30", n)
	}
}

func TestPayloads(t *testing.T) {
	e := withID(evtc.Event{DstAgent: 7, Value: -2, BuffDamage: 3, OverstackValue: 4, SkillID: 5}, 0x04030201)
	if trackable(&e) != 0x04030201 {
		t.Errorf("trackable = %#x", trackable(&e))
	}
	if got := byTrackable(&e); got != (payload{0x04030201}) {
		t.Errorf("byTrackable = %v", got)
	}
	if got := byDstValue(&e); got != (payload{7, 0xfffffffe}) {
		t.Errorf("byDstValue = %v", got)
	}
	if byDst(&e) != (payload{7}) || byValue(&e) != (payload{0xfffffffe}) || noPayload(&e) != (payload{}) {
		t.Error("payloads of one field")
	}
	if valDst(&e) != (state{7}) || valSkill(&e) != (state{5}) || valValue(&e) != (state{0xfffffffe}) || constant(9)(&e) != (state{9}) {
		t.Error("values of one field")
	}
	// Everything but the time and the source tells two states apart.
	base := whole(&e)
	for _, other := range []evtc.Event{
		{DstAgent: 8}, {Value: 1}, {BuffDamage: 1}, {OverstackValue: 1}, {SkillID: 1}, {IFF: 1}, {Buff: 1},
		{Result: 1}, {IsActivation: 1}, {IsBuffRemove: 1}, {IsFlanking: 1}, {IsShields: 1}, {IsOffcycle: 1},
	} {
		if whole(&other) == (state{}) {
			t.Errorf("whole reads nothing of %+v", other)
		}
	}
	moved := e
	moved.Time, moved.SrcAgent, moved.SrcInstanceID = 99, 0x1001, 11
	if whole(&moved) != base {
		t.Error("whole reads the time or the source")
	}
	old := evtc.Event{IsBuffRemove: 1, IsNinety: 2, IsFifty: 3, IsMoving: 4}
	if legacyTrackable(&old) != 0x04030201 {
		t.Errorf("legacyTrackable = %#x", legacyTrackable(&old))
	}
}

func TestSlotMoving(t *testing.T) {
	for sl := slotNone; sl <= slotMarker; sl++ {
		if want := sl == slotPosition || sl == slotVelocity || sl == slotFacing; sl.moving() != want {
			t.Errorf("slot %d moving = %v", sl, !want)
		}
	}
}

// TestKeys checks what names the messages of the kinds that carry more
// than an id.
func TestKeys(t *testing.T) {
	e := withID(evtc.Event{SrcAgent: 1, DstAgent: 2, Value: 3, BuffDamage: 0x70004, OverstackValue: 5, Result: 6, Buff: 7, SkillID: 8}, 9)
	old := evtc.Event{IsBuffRemove: 1, IsNinety: 2, IsFifty: 3, IsMoving: 4, DstAgent: 2, Buff: 1, Value: 3, IsOffcycle: 1}
	tests := []struct {
		name string
		rule rule
		e    *evtc.Event
		want payload
	}{
		{"strike", rules[evtc.StateCombat], &e, payload{3, 0x70004, 5<<8 | 6}},
		{"duration change", rules[evtc.StateBuffChange], &e, payload{9, 3}},
		{"missile launch", rules[evtc.StateMissileLaunch], &e, payload{9, 3, 4}},
		{"effect on the ground", rules[evtc.StateEffectGroundCreate], &e, payload{9, 2}},
		{"effect of the older kind", rules[evtc.StateEffect2Defunc], &old, payload{0x04030201}},
		{"effect of the older kind on an agent", ruleOf(&old, evtc.StateEffect2Defunc, true), &old, payload{0x04030201}},
		{"ground marker", rules[evtc.StateSquadMarkerGround], &e, payload{1, 2}},
		{"objective", rules[evtc.StateWvWObjectiveStatus], &e, payload{3, 0x70004, 7}},
		{"duration change of the older format", ruleOf(&old, evtc.StateBuffChange, false), &e, payload{9, 3}},
	}
	for _, tt := range tests {
		if got := tt.rule.key(tt.e); got != tt.want {
			t.Errorf("%s: key = %v, want %v", tt.name, got, tt.want)
		}
	}
	// A marker is its id and whether it is a commander tag.
	if got := rules[evtc.StateMarker].value(&e); got != (state{3, 7}) {
		t.Errorf("marker = %v", got)
	}
}
