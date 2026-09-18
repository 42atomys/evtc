package evtc

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// sameLog reports whether two logs hold the same header, tables and events.
func sameLog(a, b *Log) bool {
	return a.Header == b.Header && slices.Equal(a.Agents, b.Agents) &&
		slices.Equal(a.Skills, b.Skills) && slices.Equal(a.Events, b.Events)
}

func TestWriteTo(t *testing.T) {
	raw := buildLog()
	l, err := Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	n, err := l.WriteTo(&out)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(raw)) || !bytes.Equal(out.Bytes(), raw) {
		t.Errorf("wrote %d bytes that differ from the %d read", n, len(raw))
	}
}

func TestWriteFile(t *testing.T) {
	l, err := Parse(bytes.NewReader(buildLog()))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()

	for _, name := range []string{"fight.zevtc", "fight.evtc", "FIGHT.EVTC", "fight"} {
		path := filepath.Join(dir, name)
		if err := l.WriteFile(path); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		back, err := ParseFile(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !sameLog(l, back) {
			t.Errorf("%s reads back another log", name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if raw := strings.EqualFold(filepath.Ext(name), ".evtc"); raw != bytes.HasPrefix(data, []byte(magic)) {
			t.Errorf("%s: raw = %v", name, !raw)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, "fight.zevtc"))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "fight" || zr.File[0].Method != zip.Deflate {
		t.Errorf("archive = %d entries, first %q", len(zr.File), zr.File[0].Name)
	}

	if err := l.WriteFile(filepath.Join(dir, "missing", "fight.zevtc")); err == nil {
		t.Error("a log was written in a missing folder")
	}
}
