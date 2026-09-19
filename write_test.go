package evtc

import (
	"archive/zip"
	"bytes"
	"errors"
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

// failing is a writer that refuses everything.
type failing struct{ err error }

func (f failing) Write([]byte) (int, error) { return 0, f.err }

func TestWriteErrors(t *testing.T) {
	long := strings.Repeat("x", 65)
	tests := []struct {
		name string
		edit func(l *Log)
	}{
		{"short build", func(l *Log) { l.Header.Build = "2026" }},
		{"long build", func(l *Log) { l.Header.Build = "202608160" }},
		{"revision 0", func(l *Log) { l.Header.Revision = 0 }},
		{"revision 2", func(l *Log) { l.Header.Revision = 2 }},
		{"agent name too long", func(l *Log) { l.Agents[0].Name = long }},
		{"agent names too long together", func(l *Log) { l.Agents[0].Name, l.Agents[0].Account = long[:40], long[:24] }},
		{"agent name with a null byte", func(l *Log) { l.Agents[0].Account = "a\x00b" }},
		{"skill name too long", func(l *Log) { l.Skills[0].Name = long }},
		{"skill name with a null byte", func(l *Log) { l.Skills[0].Name = "\x00" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := Parse(bytes.NewReader(buildLog()))
			if err != nil {
				t.Fatal(err)
			}
			tt.edit(l)
			var out bytes.Buffer
			if n, err := l.WriteTo(&out); !errors.Is(err, ErrUnwritable) || n != 0 || out.Len() != 0 {
				t.Errorf("WriteTo = %d, %v with %d bytes written", n, err, out.Len())
			}
			path := filepath.Join(t.TempDir(), "fight.zevtc")
			if err := l.WriteFile(path); !errors.Is(err, ErrUnwritable) {
				t.Errorf("WriteFile = %v", err)
			}
			if _, err := os.Stat(path); err == nil {
				t.Error("a file was left behind")
			}
		})
	}

	l, err := Parse(bytes.NewReader(buildLog()))
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	if _, err := l.WriteTo(failing{boom}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
	if err := zipTo(failing{boom}, "fight", buildLog()); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}

// TestWriteNames checks the names that fill their buffer: the last one may
// run to its end without a terminator.
func TestWriteNames(t *testing.T) {
	x := func(n int) string { return strings.Repeat("x", n) }
	tests := []struct {
		name, account, subgroup string
		fits                    bool
	}{
		{"", "", "", true},
		{"Char", "", "", true},
		{"", "", "1", true},
		{"", "Account.1234", "", true},
		{x(64), "", "", true},
		{x(65), "", "", false},
		{x(40), x(23), "", true},
		{x(40), x(24), "", false},
		{x(40), x(21), "1", true},
		{x(40), x(22), "1", false},
		{x(64), "", "1", false},
	}
	for _, tt := range tests {
		l := &Log{Header: Header{Build: "20260816", Revision: 1}, Agents: []Agent{{Name: tt.name, Account: tt.account, Subgroup: tt.subgroup}}}
		var out bytes.Buffer
		_, err := l.WriteTo(&out)
		if (err == nil) != tt.fits {
			t.Errorf("%d, %d and %d bytes: err = %v", len(tt.name), len(tt.account), len(tt.subgroup), err)
			continue
		}
		if err != nil {
			continue
		}
		back, err := Parse(&out)
		if err != nil {
			t.Fatal(err)
		}
		if !sameLog(l, back) {
			t.Errorf("%d, %d and %d bytes read back as %+v", len(tt.name), len(tt.account), len(tt.subgroup), back.Agents[0])
		}
	}

	l := &Log{Header: Header{Build: "20260816", Revision: 1}, Skills: []Skill{{ID: -1, Name: x(64)}, {ID: 740}}}
	var out bytes.Buffer
	if _, err := l.WriteTo(&out); err != nil {
		t.Fatal(err)
	}
	back, err := Parse(&out)
	if err != nil {
		t.Fatal(err)
	}
	if !sameLog(l, back) {
		t.Errorf("skills read back as %+v", back.Skills)
	}
}

// TestWriteEmpty checks that a log without agents, skills or events is
// written with its two empty tables.
func TestWriteEmpty(t *testing.T) {
	l := &Log{Header: Header{Build: "20260816", Revision: 1, TargetSpeciesID: 15375}}
	var out bytes.Buffer
	if _, err := l.WriteTo(&out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != headerSize+8 {
		t.Errorf("wrote %d bytes, want %d", out.Len(), headerSize+8)
	}
	back, err := Parse(&out)
	if err != nil {
		t.Fatal(err)
	}
	if !sameLog(l, back) {
		t.Errorf("read back %+v", back)
	}
}
