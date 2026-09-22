package evtc

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
)

// codes returns the codes of the warnings of a log.
func codes(l *Log) []WarningCode {
	var out []WarningCode
	for _, w := range l.Warnings() {
		out = append(out, w.Code)
	}
	return out
}

func TestWarnings(t *testing.T) {
	marker := Event{IsStateChange: StateMarker, Value: 7}
	tests := []struct {
		name   string
		build  string
		target uint16
		events []Event
		want   []WarningCode
	}{
		{name: "current build", build: "20260816", target: 15375},
		{name: "build is not a date", build: "garbage!", want: []WarningCode{WarningBuild}},

		{name: "generic target", build: "20221125", target: 1, want: []WarningCode{WarningGenericTarget}},
		{name: "WvW target", build: "20221126", target: 1},
		{name: "boss target of the same era", build: "20221125", target: 15375},

		{name: "release of the defect", build: "20190108", want: []WarningCode{WarningDurationChangeAgents}},
		{name: "release of the fix", build: "20190109"},
		{name: "day before the defect", build: "20190107"},
		{name: "defect fixed three days later", build: "20250808", want: []WarningCode{WarningTarget}},

		{name: "defect of a kind the log lacks", build: "20241210"},
		{name: "defect of a kind the log holds", build: "20241210", events: []Event{marker}, want: []WarningCode{WarningMarkers}},
		{
			name: "three defects of one release", build: "20230718",
			events: []Event{{IsStateChange: StateLanguage}, {IsStateChange: StateEffect2Defunc}},
			want:   []WarningCode{WarningLanguage, WarningEffectGUIDs, WarningEffectOrientation},
		},
		{name: "defective GUIDs", build: "20220701", events: []Event{{IsStateChange: StateIDToGUID}}, want: []WarningCode{WarningGUIDs}},
		{name: "several sources", build: "20220701", target: 1, events: []Event{{SkillID: 740}, {IsStateChange: StateIDToGUID}}, want: []WarningCode{WarningGenericTarget, WarningNoSkills, WarningGUIDs}},
	}
	for _, tt := range tests {
		l := logOf(tt.build, tt.events...)
		l.Header.TargetSpeciesID = tt.target
		if got := codes(l); !slices.Equal(got, tt.want) || !slices.IsSorted(got) {
			t.Errorf("%s: warnings %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestWarningsParsedLog checks the kinds a parsed log keeps from its first
// scan, whichever of Has and Warnings runs it.
func TestWarningsParsedLog(t *testing.T) {
	raw := buildLog()
	copy(raw[4:], "20241210")
	raw[len(raw)-eventSize+56] = byte(StateMarker)
	for _, hasFirst := range []bool{false, true} {
		l, err := Parse(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if hasFirst && !l.Has(CapabilityMarkers) {
			t.Error("the marker proves nothing")
		}
		for range 2 {
			if got := codes(l); !slices.Equal(got, []WarningCode{WarningMarkers}) {
				t.Errorf("Has first %v: warnings %v", hasFirst, got)
			}
		}
	}
}

func TestWarningNoSkills(t *testing.T) {
	strike := Event{SkillID: 740}
	l := logOf("20260701", strike)
	if got := codes(l); !slices.Equal(got, []WarningCode{WarningNoSkills}) {
		t.Errorf("warnings %v", got)
	}
	// The table arcdps 20260701 writes: one row, for skill 0.
	l.Skills = []Skill{{ID: 0, Name: "0"}}
	if w := l.Warnings(); len(w) != 1 || w[0].Code != WarningNoSkills || !strings.Contains(w[0].Message, "arcdps 20260701") {
		t.Errorf("warnings with the table of arcdps 20260701: %v", w)
	}
	l.Header.Build = "20260816"
	if w := l.Warnings(); len(w) != 1 || strings.Contains(w[0].Message, "arcdps") {
		t.Errorf("warnings of another build: %v", w)
	}
	// The skills arcdps emits itself have no row, the generic kill of a
	// killing blow for one.
	l.Events = []Event{{SkillID: SkillGenericKill, Result: ResultKillingBlow}, strike}
	l.Skills = []Skill{{ID: 740, Name: "Might"}}
	if got := codes(l); got != nil {
		t.Errorf("warnings with a skill table: %v", got)
	}
	// A ground marker keeps its index in SkillID and names no skill.
	if got := codes(logOf("20260701", Event{IsStateChange: StateSquadMarkerGround, SkillID: 3})); got != nil {
		t.Errorf("warnings without any skill use: %v", got)
	}
}

// TestWarningsSample checks the real log kept in tests_fixtures/ when
// present has nothing to report.
func TestWarningsSample(t *testing.T) {
	const path = "tests_fixtures/sabetha-05-fd9b6f3a.zevtc"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s not available", path)
	}
	l, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if w := l.Warnings(); len(w) != 0 {
		t.Errorf("Warnings = %v", w)
	}
}

func TestWarningMessage(t *testing.T) {
	w := logOf("20240612", Event{IsStateChange: StateChangeDead}).Warnings()
	if len(w) != 1 || w[0].Code != WarningDeadDown || !strings.HasPrefix(w[0].Message, "arcdps 20240612: ") {
		t.Errorf("warnings = %+v", w)
	}
	var none *Log
	if none.Warnings() != nil {
		t.Error("a nil log has warnings")
	}
}

func TestWarningCodeString(t *testing.T) {
	for c := range warningCount {
		if name := c.String(); name == "" || strings.HasPrefix(name, "WarningCode(") {
			t.Errorf("code %d has no name", c)
		}
	}
	if got := WarningCode(200).String(); got != "WarningCode(200)" {
		t.Errorf("unknown code: %q", got)
	}
}

// TestDefects checks the table: ranges in order, each with its fix after
// its release, none older than 20181002, when revision 1 became the
// default.
func TestDefects(t *testing.T) {
	last := 0
	for _, d := range defects {
		if d.from < 20181002 || d.fixed <= d.from || d.from < last || d.text == "" || d.code >= warningCount {
			t.Errorf("%v: from %d, fixed %d, after %d", d.code, d.from, d.fixed, last)
		}
		last = d.from
	}
}
