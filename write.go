package evtc

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnwritable is returned when a log holds something the file layout has
// no room for: a build that is not eight bytes long, a revision other than
// 1, names longer than the 64 bytes of their buffer or holding a null byte.
var ErrUnwritable = errors.New("evtc: log cannot be written")

// WriteFile writes the log at path: a raw log when the path ends in .evtc,
// otherwise a zip archive holding it alone, as arcdps compresses its logs.
// The entry of the archive is named after the file.
func (l *Log) WriteFile(path string) error {
	data, err := l.encode()
	if err != nil {
		return err
	}
	if strings.EqualFold(filepath.Ext(path), ".evtc") {
		return os.WriteFile(path, data, 0o644)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if err := zipTo(f, name, data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// WriteTo writes the log to w in the raw layout of an .evtc file, the one
// Parse reads back. The bytes of a parsed log are those of its file, the
// four bytes of padding that end each agent aside: arcdps leaves them as
// its memory held them, and they are written as zeros.
func (l *Log) WriteTo(w io.Writer) (int64, error) {
	data, err := l.encode()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// zipTo writes an archive holding data as its single entry.
func zipTo(w io.Writer, name string, data []byte) error {
	zw := zip.NewWriter(w)
	entry, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("evtc: create %q: %w", name, err)
	}
	if _, err := entry.Write(data); err != nil {
		return fmt.Errorf("evtc: write %q: %w", name, err)
	}
	return zw.Close()
}

// encode lays the log out as decode reads it.
func (l *Log) encode() ([]byte, error) {
	if len(l.Header.Build) != 8 {
		return nil, fmt.Errorf("%w: build %q is not 8 bytes long", ErrUnwritable, l.Header.Build)
	}
	if l.Header.Revision != 1 {
		return nil, fmt.Errorf("%w: unsupported revision %d", ErrUnwritable, l.Header.Revision)
	}

	data := make([]byte, headerSize+4+len(l.Agents)*agentSize+4+len(l.Skills)*skillSize+len(l.Events)*eventSize)
	copy(data, magic)
	copy(data[4:12], l.Header.Build)
	data[12] = l.Header.Revision
	binary.LittleEndian.PutUint16(data[13:15], l.Header.TargetSpeciesID)
	// data[15] is unused.
	off := headerSize

	binary.LittleEndian.PutUint32(data[off:], uint32(len(l.Agents)))
	off += 4
	for i := range l.Agents {
		a := &l.Agents[i]
		r := data[off : off+agentSize]
		binary.LittleEndian.PutUint64(r[0:8], a.Addr)
		binary.LittleEndian.PutUint32(r[8:12], a.Profession)
		binary.LittleEndian.PutUint32(r[12:16], a.IsElite)
		binary.LittleEndian.PutUint16(r[16:18], uint16(a.Toughness))
		binary.LittleEndian.PutUint16(r[18:20], uint16(a.Concentration))
		binary.LittleEndian.PutUint16(r[20:22], uint16(a.Healing))
		binary.LittleEndian.PutUint16(r[22:24], a.HitboxWidth)
		binary.LittleEndian.PutUint16(r[24:26], uint16(a.Condition))
		binary.LittleEndian.PutUint16(r[26:28], a.HitboxHeight)
		// r[92:96] is struct padding.
		if err := putStrings(r[28:92], a.Name, a.Account, a.Subgroup); err != nil {
			return nil, fmt.Errorf("%w: agent %d: %v", ErrUnwritable, i, err)
		}
		off += agentSize
	}

	binary.LittleEndian.PutUint32(data[off:], uint32(len(l.Skills)))
	off += 4
	for i := range l.Skills {
		s := &l.Skills[i]
		r := data[off : off+skillSize]
		binary.LittleEndian.PutUint32(r[0:4], uint32(s.ID))
		if err := putStrings(r[4:68], s.Name); err != nil {
			return nil, fmt.Errorf("%w: skill %d: %v", ErrUnwritable, s.ID, err)
		}
		off += skillSize
	}

	for i := range l.Events {
		b := l.Events[i].Bytes()
		off += copy(data[off:], b[:])
	}
	return data, nil
}

// putStrings writes null-terminated strings one after the other in a
// zeroed buffer, up to the last one that is not empty. That one may run to
// the end of the buffer without its terminator, as cstring reads it.
func putStrings(buf []byte, s ...string) error {
	last := -1
	for i, v := range s {
		if strings.IndexByte(v, 0) >= 0 {
			return fmt.Errorf("%q holds a null byte", v)
		}
		if v != "" {
			last = i
		}
	}
	off := 0
	for _, v := range s[:last+1] {
		if off+len(v) > len(buf) {
			return fmt.Errorf("%q does not fit in %d bytes", strings.Join(s[:last+1], " "), len(buf))
		}
		off += copy(buf[off:], v) + 1
	}
	return nil
}
