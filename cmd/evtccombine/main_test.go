package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/combine"
)

// recording returns the log a client wrote of a short fight: a player
// strikes a boss thirty times. The client writes under its own addresses
// and on its own clock.
func recording(base, clock uint64) *evtc.Log {
	l := &evtc.Log{
		Header: evtc.Header{Build: "20260920", Revision: 1, TargetSpeciesID: 15375},
		Agents: []evtc.Agent{
			{Addr: base | 11, Profession: 1, Name: "Alpha", Account: ":Alpha.1234", Subgroup: "1"},
			{Addr: base | 21, Profession: 15375, IsElite: 0xffffffff, Name: "Sabetha"},
		},
		Skills: []evtc.Skill{{ID: 100, Name: "Heat"}},
	}
	l.Events = append(l.Events, evtc.Event{Time: 1000 + clock, Value: 1787685623, IsStateChange: evtc.StateSquadCombatStart})
	for i := range uint64(30) {
		l.Events = append(l.Events, evtc.Event{
			Time: 1000 + clock + 100*i, SrcAgent: base | 11, SrcInstanceID: 11, DstAgent: base | 21, DstInstanceID: 21,
			SkillID: 100, Value: int32(1000 + i), IFF: evtc.IFFFoe,
		})
	}
	l.Events = append(l.Events, evtc.Event{Time: 1000 + clock + 3000, Value: 1787685626, IsStateChange: evtc.StateSquadCombatEnd})
	return l
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	mine, theirs, out := filepath.Join(dir, "mine.zevtc"), filepath.Join(dir, "theirs.evtc"), filepath.Join(dir, "fight.zevtc")
	if err := errors.Join(recording(0x1000, 0).WriteFile(mine), recording(0x2000, 500_000).WriteFile(theirs)); err != nil {
		t.Fatal(err)
	}
	if err := run(out, []string{mine, theirs}); err != nil {
		t.Fatalf("run = %v", err)
	}
	l, err := evtc.ParseFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Agents) != 2 || len(l.Events) != 32 {
		t.Errorf("%d agents, %d events", len(l.Agents), len(l.Events))
	}

	if err := run(mine, []string{mine, theirs}); err == nil {
		t.Error("a recording was given as the output")
	}
	if got, err := evtc.ParseFile(mine); err != nil || len(got.Events) != 32 {
		t.Errorf("the recording was written over: %v", err)
	}
	if err := run(out, []string{mine, filepath.Join(dir, "missing.zevtc")}); err == nil {
		t.Error("run of a missing file succeeded")
	}
	other := recording(0x3000, 0)
	other.Header.Build = "20260816"
	older := filepath.Join(dir, "older.zevtc")
	if err := other.WriteFile(older); err != nil {
		t.Fatal(err)
	}
	if err := run(out, []string{mine, older}); !errors.Is(err, combine.ErrBuildMismatch) {
		t.Errorf("run = %v", err)
	}
	if err := run(filepath.Join(dir, "missing", "fight.zevtc"), []string{mine, theirs}); err == nil {
		t.Error("the log was written in a missing folder")
	}
}

func TestSame(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.zevtc")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		a, b string
		want bool
	}{
		{path, path, true},
		{path, filepath.Join(dir, ".", "log.zevtc"), true},
		{path, filepath.Join(dir, "other.zevtc"), false},
		{filepath.Join(dir, "new.zevtc"), filepath.Join(dir, "sub", "..", "new.zevtc"), true},
	} {
		if got := same(tt.a, tt.b); got != tt.want {
			t.Errorf("same(%q, %q) = %v", tt.a, tt.b, got)
		}
	}
}
