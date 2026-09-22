package evtc

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"weak"
)

const (
	magic      = "EVTC"
	zipMagic   = "PK\x03\x04"
	headerSize = 16 // magic + build date + revision + target id + pad
	agentSize  = 96
	skillSize  = 68
	eventSize  = 64 // cbtevent, revision 1
)

// ErrInvalidFormat is returned when the input is neither a raw EVTC log nor
// a zip archive containing exactly one.
var ErrInvalidFormat = errors.New("evtc: invalid file format")

// maxLogSize bounds the uncompressed size a .zevtc entry may declare, so
// that a crafted archive cannot make Parse allocate an arbitrary amount of
// memory before reading a single byte.
const maxLogSize = 1 << 31

// ParseFile reads and decodes the .evtc or .zevtc file at path.
func ParseFile(path string) (*Log, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(data)
}

// Parse decodes an EVTC log from r. The input may be a raw .evtc log or a
// .zevtc zip archive containing exactly one file.
func Parse(r io.Reader) (*Log, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("evtc: read: %w", err)
	}
	return parse(data)
}

// parse decodes a log held in memory, inflating it first when it is an
// archive.
func parse(data []byte) (*Log, error) {
	if bytes.HasPrefix(data, []byte(zipMagic)) {
		var err error
		if data, err = unzip(data); err != nil {
			return nil, err
		}
	}
	return decode(data)
}

// unzip extracts the single log contained in a .zevtc archive.
func unzip(archive []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("evtc: open zip: %w", err)
	}
	if len(zr.File) != 1 {
		return nil, fmt.Errorf("%w: archive contains %d files, want 1", ErrInvalidFormat, len(zr.File))
	}

	entry := zr.File[0]
	if entry.UncompressedSize64 > maxLogSize {
		return nil, fmt.Errorf("%w: entry %q declares %d bytes", ErrInvalidFormat, entry.Name, entry.UncompressedSize64)
	}
	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("evtc: open %q: %w", entry.Name, err)
	}
	defer rc.Close()

	// The uncompressed size is known, so allocate once.
	data := make([]byte, entry.UncompressedSize64)
	if _, err := io.ReadFull(rc, data); err != nil {
		return nil, fmt.Errorf("evtc: read %q: %w", entry.Name, err)
	}
	// An entry longer than declared would silently drop its last events.
	if n, err := rc.Read(make([]byte, 1)); n != 0 || err != io.EOF {
		return nil, fmt.Errorf("%w: entry %q is longer than declared", ErrInvalidFormat, entry.Name)
	}
	return data, nil
}

// decode parses an uncompressed EVTC log.
func decode(data []byte) (*Log, error) {
	if len(data) < headerSize || string(data[:4]) != magic {
		return nil, fmt.Errorf("%w: bad magic", ErrInvalidFormat)
	}

	l := &Log{
		Header: Header{
			Build:           string(data[4:12]),
			Revision:        data[12],
			TargetSpeciesID: binary.LittleEndian.Uint16(data[13:15]),
			// data[15] is unused.
		},
	}

	if l.Header.Revision != 1 {
		return nil, fmt.Errorf("evtc: unsupported revision %d", l.Header.Revision)
	}

	offset := headerSize

	// Agents: u32 count followed by count * 96 bytes.
	agents, n, err := table(data[offset:], "agent", agentSize)
	if err != nil {
		return nil, err
	}
	l.Agents = decodeAgents(agents)
	offset += n

	// Skills: u32 count followed by count * 68 bytes.
	skills, n, err := table(data[offset:], "skill", skillSize)
	if err != nil {
		return nil, err
	}
	l.Skills = decodeSkills(skills)
	offset += n

	// Events: no count is written, they run until end of file.
	events := data[offset:]
	if rest := len(events) % eventSize; rest != 0 {
		return nil, fmt.Errorf("evtc: truncated log: %d trailing bytes after last event", rest)
	}
	l.Events = decodeEvents(events)

	l.seen = &observation{owner: weak.Make(l)}

	return l, nil
}

// table reads a u32 element count and returns the slice holding the
// elements, plus the total number of bytes consumed.
func table(data []byte, name string, elemSize int) ([]byte, int, error) {
	if len(data) < 4 {
		return nil, 0, fmt.Errorf("evtc: truncated log: missing %s count", name)
	}
	count := int(binary.LittleEndian.Uint32(data))
	data = data[4:]
	if count < 0 || count > len(data)/elemSize {
		return nil, 0, fmt.Errorf("evtc: truncated log: %d %ss do not fit", count, name)
	}
	size := count * elemSize
	return data[:size], 4 + size, nil
}

func decodeAgents(data []byte) []Agent {
	out := make([]Agent, 0, len(data)/agentSize)
	for off := 0; off+agentSize <= len(data); off += agentSize {
		r := data[off : off+agentSize]
		a := Agent{
			Addr:          binary.LittleEndian.Uint64(r[0:8]),
			Profession:    binary.LittleEndian.Uint32(r[8:12]),
			IsElite:       binary.LittleEndian.Uint32(r[12:16]),
			Toughness:     int16(binary.LittleEndian.Uint16(r[16:18])),
			Concentration: int16(binary.LittleEndian.Uint16(r[18:20])),
			Healing:       int16(binary.LittleEndian.Uint16(r[20:22])),
			HitboxWidth:   binary.LittleEndian.Uint16(r[22:24]),
			Condition:     int16(binary.LittleEndian.Uint16(r[24:26])),
			HitboxHeight:  binary.LittleEndian.Uint16(r[26:28]),
		}
		// r[28:92] is the 64-byte name buffer, r[92:96] is struct padding.
		var rest []byte
		a.Name, rest = cstring(r[28:92])
		a.Account, rest = cstring(rest)
		a.Subgroup, _ = cstring(rest)
		out = append(out, a)
	}
	return out
}

func decodeSkills(data []byte) []Skill {
	out := make([]Skill, 0, len(data)/skillSize)
	for off := 0; off+skillSize <= len(data); off += skillSize {
		r := data[off : off+skillSize]
		s := Skill{ID: int32(binary.LittleEndian.Uint32(r[0:4]))}
		s.Name, _ = cstring(r[4:68])
		out = append(out, s)
	}
	return out
}

func decodeEvents(data []byte) []Event {
	out := make([]Event, 0, len(data)/eventSize)
	for off := 0; off+eventSize <= len(data); off += eventSize {
		r := data[off : off+eventSize]
		out = append(out, Event{
			Time:                binary.LittleEndian.Uint64(r[0:8]),
			SrcAgent:            binary.LittleEndian.Uint64(r[8:16]),
			DstAgent:            binary.LittleEndian.Uint64(r[16:24]),
			Value:               int32(binary.LittleEndian.Uint32(r[24:28])),
			BuffDamage:          int32(binary.LittleEndian.Uint32(r[28:32])),
			OverstackValue:      binary.LittleEndian.Uint32(r[32:36]),
			SkillID:             binary.LittleEndian.Uint32(r[36:40]),
			SrcInstanceID:       binary.LittleEndian.Uint16(r[40:42]),
			DstInstanceID:       binary.LittleEndian.Uint16(r[42:44]),
			SrcMasterInstanceID: binary.LittleEndian.Uint16(r[44:46]),
			DstMasterInstanceID: binary.LittleEndian.Uint16(r[46:48]),
			IFF:                 IFF(r[48]),
			Buff:                r[49],
			Result:              Result(r[50]),
			IsActivation:        Activation(r[51]),
			IsBuffRemove:        BuffRemove(r[52]),
			IsNinety:            r[53],
			IsFifty:             r[54],
			IsMoving:            r[55],
			IsStateChange:       StateChange(r[56]),
			IsFlanking:          r[57],
			IsShields:           r[58],
			IsOffcycle:          r[59],
			Pad61:               r[60],
			Pad62:               r[61],
			Pad63:               r[62],
			Pad64:               r[63],
		})
	}
	return out
}

// cstring returns the null-terminated string at the start of b and the
// remaining bytes after the terminator.
func cstring(b []byte) (string, []byte) {
	s, rest, found := bytes.Cut(b, []byte{0})
	if !found {
		return string(b), nil
	}
	return string(s), rest
}
