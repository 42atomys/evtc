package evtc

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"
)

// buildLog assembles a minimal revision 1 log: one player agent, one skill
// and one event.
func buildLog() []byte {
	var b bytes.Buffer

	// Header.
	b.WriteString("EVTC20260816")
	b.WriteByte(1)
	binary.Write(&b, binary.LittleEndian, uint16(15375)) // sabetha
	b.WriteByte(0)

	// Agents.
	binary.Write(&b, binary.LittleEndian, uint32(1))
	agent := make([]byte, agentSize)
	binary.LittleEndian.PutUint64(agent[0:], 0xdeadbeef)
	binary.LittleEndian.PutUint32(agent[8:], 4)  // profession
	binary.LittleEndian.PutUint32(agent[12:], 5) // elite
	binary.LittleEndian.PutUint16(agent[16:], 10)
	binary.LittleEndian.PutUint16(agent[26:], 42)
	copy(agent[28:], "Char\x00Account.1234\x001\x00")
	b.Write(agent)

	// Skills.
	binary.Write(&b, binary.LittleEndian, uint32(1))
	skill := make([]byte, skillSize)
	binary.LittleEndian.PutUint32(skill[0:], 740)
	copy(skill[4:], "Might\x00")
	b.Write(skill)

	// Events.
	ev := make([]byte, eventSize)
	binary.LittleEndian.PutUint64(ev[0:], 123456)
	binary.LittleEndian.PutUint64(ev[8:], 0xdeadbeef)
	binary.LittleEndian.PutUint32(ev[36:], 740)
	ev[56] = uint8(StateBuffApply)
	binary.LittleEndian.PutUint32(ev[60:], 0x04030201)
	b.Write(ev)

	return b.Bytes()
}

func checkLog(t *testing.T, l *Log) {
	t.Helper()

	if l.Header.Build != "20260816" || l.Header.Revision != 1 || l.Header.TargetSpeciesID != 15375 {
		t.Errorf("header = %+v", l.Header)
	}

	if len(l.Agents) != 1 {
		t.Fatalf("agents = %d, want 1", len(l.Agents))
	}
	a := l.Agents[0]
	if a.Addr != 0xdeadbeef || a.Profession != 4 || a.IsElite != 5 || a.Toughness != 10 || a.HitboxHeight != 42 {
		t.Errorf("agent = %+v", a)
	}
	if a.Name != "Char" || a.Account != "Account.1234" || a.Subgroup != "1" {
		t.Errorf("agent names = %q %q %q", a.Name, a.Account, a.Subgroup)
	}

	if len(l.Skills) != 1 || l.Skills[0].ID != 740 || l.Skills[0].Name != "Might" {
		t.Errorf("skills = %+v", l.Skills)
	}

	if len(l.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(l.Events))
	}
	e := l.Events[0]
	if e.Time != 123456 || e.SrcAgent != 0xdeadbeef || e.SkillID != 740 || e.IsStateChange != StateBuffApply {
		t.Errorf("event = %+v", e)
	}
	if e.Pad61 != 1 || e.Pad62 != 2 || e.Pad63 != 3 || e.Pad64 != 4 {
		t.Errorf("pad = %d %d %d %d, want 1 2 3 4", e.Pad61, e.Pad62, e.Pad63, e.Pad64)
	}
}

func TestParseRaw(t *testing.T) {
	l, err := Parse(bytes.NewReader(buildLog()))
	if err != nil {
		t.Fatal(err)
	}
	checkLog(t, l)
}

// zipped wraps a raw log in a single-entry archive, as arcdps does.
func zipped(t testing.TB, raw []byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create("20260816-212543")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

// zippedRaw writes a stored entry whose header declares the given sizes,
// whatever the content actually is.
func zippedRaw(t testing.TB, content []byte, declared uint64) []byte {
	t.Helper()
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name:               "log",
		Method:             zip.Store,
		CRC32:              crc32.ChecksumIEEE(content),
		CompressedSize64:   declared,
		UncompressedSize64: declared,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func TestParseZip(t *testing.T) {
	l, err := Parse(bytes.NewReader(zipped(t, buildLog())))
	if err != nil {
		t.Fatal(err)
	}
	checkLog(t, l)
}

func TestParseErrors(t *testing.T) {
	full := buildLog()

	tests := []struct {
		name string
		data []byte
		want error // nil means any error
	}{
		{"empty", nil, ErrInvalidFormat},
		{"bad magic", []byte("NOPE0000000000000"), ErrInvalidFormat},
		{"revision 0", append(append([]byte{}, full[:12]...), 0, 0, 0, 0), nil},
		{"revision 2", append(append([]byte{}, full[:12]...), 2, 0, 0, 0), nil},
		{"missing agent count", full[:headerSize], nil},
		{"agents do not fit", full[:headerSize+4], nil},
		{"trailing bytes", full[:len(full)-1], nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(bytes.NewReader(tt.data))
			if err == nil {
				t.Fatal("expected an error")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestParseSample decodes the real log kept in tests_fixtures/ when present.
func TestParseSample(t *testing.T) {
	const path = "tests_fixtures/sabetha-05-fd9b6f3a.zevtc"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s not available", path)
	}

	l, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if l.Header.Build != "20260816" || l.Header.TargetSpeciesID != 15375 {
		t.Errorf("header = %+v", l.Header)
	}
	if len(l.Agents) != 453 || len(l.Skills) != 598 || len(l.Events) != 182964 {
		t.Errorf("counts = %d agents, %d skills, %d events", len(l.Agents), len(l.Skills), len(l.Events))
	}
}

func TestParseFileErrors(t *testing.T) {
	if _, err := ParseFile(filepath.Join(t.TempDir(), "missing.zevtc")); err == nil {
		t.Error("a missing file was parsed")
	}
}

// TestParseFileClosesFile checks that the file handle is released: on
// Windows a file still open cannot be removed.
func TestParseFileClosesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.evtc")
	if err := os.WriteFile(path, buildLog(), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	checkLog(t, l)
	if err := os.Remove(path); err != nil {
		t.Fatalf("the file is still open after ParseFile: %v", err)
	}
}

func TestParseReadError(t *testing.T) {
	boom := errors.New("boom")
	if _, err := Parse(iotest.ErrReader(boom)); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}

func TestParseZipErrors(t *testing.T) {
	raw := buildLog()
	var two bytes.Buffer
	zw := zip.NewWriter(&two)
	for _, name := range []string{"a", "b"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(raw)
	}
	zw.Close()

	tests := []struct {
		name string
		data []byte
		want error // nil means any error
	}{
		{"truncated archive", []byte("PK\x03\x04garbage"), nil},
		{"two entries", two.Bytes(), ErrInvalidFormat},
		{"declared size above the cap", zippedRaw(t, raw, maxLogSize+1), ErrInvalidFormat},
		{"declared size longer than the content", zippedRaw(t, raw, uint64(len(raw))+64), nil},
		{"declared size shorter than the content", zippedRaw(t, raw, uint64(len(raw))-64), ErrInvalidFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(bytes.NewReader(tt.data))
			if err == nil {
				t.Fatal("expected an error")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}

	l, err := Parse(bytes.NewReader(zippedRaw(t, raw, uint64(len(raw)))))
	if err != nil {
		t.Fatalf("an exact raw entry failed: %v", err)
	}
	checkLog(t, l)
}

func TestTableErrors(t *testing.T) {
	full := buildLog()
	agentsEnd := headerSize + 4 + agentSize
	tests := []struct {
		name string
		data []byte
	}{
		{"missing skill count", full[:agentsEnd]},
		{"skills do not fit", full[:agentsEnd+4]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(bytes.NewReader(tt.data)); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestCString(t *testing.T) {
	s, rest := cstring([]byte("abc\x00def\x00"))
	if s != "abc" || string(rest) != "def\x00" {
		t.Errorf("cstring = %q, %q", s, rest)
	}
	s, rest = cstring([]byte("abc"))
	if s != "abc" || rest != nil {
		t.Errorf("cstring without terminator = %q, %v", s, rest)
	}
	s, rest = cstring(nil)
	if s != "" || rest != nil {
		t.Errorf("cstring of nil = %q, %v", s, rest)
	}
}

// TestParseEmptyTables checks that a log without agents, skills or events
// decodes to empty slices.
func TestParseEmptyTables(t *testing.T) {
	data := append([]byte("EVTC20260816\x01\x00\x00\x00"), 0, 0, 0, 0, 0, 0, 0, 0)
	l, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Agents) != 0 || len(l.Skills) != 0 || len(l.Events) != 0 {
		t.Errorf("log = %+v", l)
	}
}

// FuzzParse checks that no input makes Parse panic and that a decoded log
// is internally consistent.
func FuzzParse(f *testing.F) {
	raw := buildLog()
	f.Add(raw)
	f.Add(zipped(f, raw))
	f.Add([]byte("EVTC"))
	f.Add([]byte("PK\x03\x04"))
	f.Fuzz(func(t *testing.T, data []byte) {
		l, err := Parse(bytes.NewReader(data))
		if err != nil {
			if l != nil {
				t.Fatal("a log was returned with an error")
			}
			return
		}
		if len(l.Header.Build) != 8 {
			t.Fatalf("build = %q", l.Header.Build)
		}
		for _, a := range l.Agents {
			if len(a.Name) > 64 || len(a.Account) > 64 || len(a.Subgroup) > 64 {
				t.Fatalf("agent names too long: %+v", a)
			}
		}
		for _, s := range l.Skills {
			if len(s.Name) > 64 {
				t.Fatalf("skill name too long: %+v", s)
			}
		}
	})
}

func sampleBytes(b *testing.B) []byte {
	b.Helper()
	const path = "tests_fixtures/sabetha-05-fd9b6f3a.zevtc"
	data, err := os.ReadFile(path)
	if err != nil {
		b.Skipf("%s not available", path)
	}
	return data
}

// BenchmarkParseSample measures the full decoding of the real log from
// memory, inflating included.
func BenchmarkParseSample(b *testing.B) {
	data := sampleBytes(b)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeSample measures the binary decoding alone, on the
// inflated log.
func BenchmarkDecodeSample(b *testing.B) {
	raw, err := unzip(sampleBytes(b))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := decode(raw); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseSynthetic measures the decoding of an in-memory raw log,
// available without the sample.
func BenchmarkParseSynthetic(b *testing.B) {
	raw := buildLog()
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(bytes.NewReader(raw)); err != nil {
			b.Fatal(err)
		}
	}
}
