package main

import (
	"os"
	"path/filepath"
	"testing"
)

// minimalLog is a revision 1 log with empty agent, skill and event tables.
var minimalLog = append([]byte("EVTC20260816\x01\x00\x00\x00"), 0, 0, 0, 0, 0, 0, 0, 0)

func TestRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.evtc")
	if err := os.WriteFile(path, minimalLog, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(path); err != nil {
		t.Errorf("run = %v", err)
	}
	if err := run(filepath.Join(t.TempDir(), "missing.evtc")); err == nil {
		t.Error("run of a missing file succeeded")
	}
}
