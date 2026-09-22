package evtc

import "testing"

func TestStateChangeValues(t *testing.T) {
	// Spot-check positions against the arcdps cbtstatechange enum.
	tests := []struct {
		s    StateChange
		want uint8
	}{
		{StateCombat, 0},
		{StateSquadCombatStart, 9},
		{StateSquadCombatEnd, 10},
		{StatePointOfView, 13},
		{StateBuffInfo, 30},
		{StateIDToGUID, 46},
		{StateIIDChange, 64},
		{StateAnimationStart, 67},
		{StateBuffApply, 69},
		{StateBuffRemoveAll, 72},
		{StateTick, 84},
		{StateJump, 86},
		{StateGadgetModelInfo, 87},
		{StateFlyTo, 88},
		{StateUnknown, 89},
	}
	for _, tt := range tests {
		if uint8(tt.s) != tt.want {
			t.Errorf("%s = %d, want %d", tt.s, uint8(tt.s), tt.want)
		}
	}
	if len(stateChangeNames) != int(StateUnknown)+1 {
		t.Errorf("stateChangeNames has %d entries, want %d", len(stateChangeNames), StateUnknown+1)
	}
}

func TestStateChangeString(t *testing.T) {
	tests := []struct {
		s    StateChange
		want string
	}{
		{StateCombat, "Combat"},
		{StateBuffApply, "BuffApply"},
		{StateGadgetModelInfo, "GadgetModelInfo"},
		{StateFlyTo, "FlyTo"},
		{StateUnknown, "Unknown"},
		{StateChange(200), "StateChange(200)"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("StateChange(%d).String() = %q, want %q", uint8(tt.s), got, tt.want)
		}
	}
	for i, name := range stateChangeNames {
		if name == "" {
			t.Errorf("StateChange(%d) has no name", i)
		}
	}
}
