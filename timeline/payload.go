package timeline

import (
	"bytes"
	"encoding/binary"
	"math"
	"time"

	"github.com/42atomys/evtc"
)

// Offsets of the event fields in the 64-byte wire layout, where arcdps
// stores payloads that spill over several fields.
const (
	offTime       = 0
	offSrc        = 8
	offDst        = 16
	offValue      = 24
	offBuffDmg    = 28
	offSrcInst    = 40
	offIFF        = 48
	offResult     = 50
	offBuffRemove = 52
	offFlanking   = 57
	offShields    = 58
	offPad61      = 60
)

func f32At(b *[64]byte, off int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(b[off:]))
}

func i16At(b *[64]byte, off int) int16 { return int16(binary.LittleEndian.Uint16(b[off:])) }

func u32At(b *[64]byte, off int) uint32 { return binary.LittleEndian.Uint32(b[off:]) }

// coords decodes three int16 coordinates stored as the game coordinates
// divided by ten.
func coords(b *[64]byte, off int) Vec3 {
	return Vec3{float32(i16At(b, off)) * 10, float32(i16At(b, off+2)) * 10, float32(i16At(b, off+4)) * 10}
}

// cstrAt decodes the null-terminated string starting at off.
func cstrAt(b *[64]byte, off int) string {
	s := b[off:]
	if i := bytes.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	return string(s)
}

// textAt decodes the null-terminated string of at most n bytes starting
// at off, empty unless every byte before the terminator is printable
// ASCII.
func textAt(b *[64]byte, off, n int) string {
	s := b[off : off+n]
	if i := bytes.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	for _, c := range s {
		if c < 0x20 || c > 0x7e {
			return ""
		}
	}
	return string(s)
}

// extensionSignature returns the extension signature a combat event
// carries in its pad61 to pad64 bytes.
func extensionSignature(e *evtc.Event) uint32 { return trackableID(e) }

// guidAt decodes the 16 bytes of a content GUID starting at off.
func guidAt(b *[64]byte, off int) GUID {
	var g GUID
	copy(g[:], b[off:off+16])
	return g
}

// floatSeconds converts a float number of seconds into a duration.
func floatSeconds(v float32) time.Duration {
	if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return 0
	}
	return time.Duration(float64(v) * float64(time.Second))
}

// floatMS converts a float number of milliseconds into a duration.
func floatMS(v float32) time.Duration {
	if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return 0
	}
	return time.Duration(float64(v) * float64(time.Millisecond))
}

// formula decodes a StateBuffFormula event: nine floats from the time
// field onwards and two more from the instance id fields.
func formula(e *evtc.Event) BuffFormula {
	b := e.Bytes()
	return BuffFormula{
		Type:                 f32At(&b, offTime),
		Attribute1:           f32At(&b, offTime+4),
		Attribute2:           f32At(&b, offTime+8),
		Parameter1:           f32At(&b, offTime+12),
		Parameter2:           f32At(&b, offTime+16),
		Parameter3:           f32At(&b, offTime+20),
		TraitConditionSource: f32At(&b, offTime+24),
		TraitConditionSelf:   f32At(&b, offTime+28),
		ContentReference:     f32At(&b, offTime+32),
		BuffConditionSource:  f32At(&b, offSrcInst),
		BuffConditionSelf:    f32At(&b, offSrcInst+4),
		Event:                e,
	}
}

// skillInfo decodes the four floats of a StateSkillInfo event into the
// skill: cost, both ranges and tooltip time.
func skillInfo(s *Skill, e *evtc.Event) {
	b := e.Bytes()
	s.Info = e
	s.Cost = f32At(&b, offTime)
	s.MinRange = f32At(&b, offTime+4)
	s.MaxRange = f32At(&b, offTime+8)
	s.TooltipTime = floatSeconds(f32At(&b, offTime+12))
}

// effectDuration decodes the uint32 duration effect events store from the
// iff field onwards, 0 when unknown (arcdps writes 0 or all ones).
func effectDuration(e *evtc.Event) time.Duration {
	b := e.Bytes()
	if v := u32At(&b, offIFF); v != 0 && v != 0xFFFFFFFF {
		return ms(int64(v))
	}
	return 0
}

// effectScale decodes the scale of a ground effect, 1 when the event
// gives none.
func effectScale(e *evtc.Event) float32 {
	b := e.Bytes()
	if v := i16At(&b, offShields); v != 0 {
		return float32(v) / 1000
	}
	return 1
}

// groundEffectPlace decodes the origin and orientation of a ground effect
// from the six int16 stored from dst_agent onwards.
func groundEffectPlace(e *evtc.Event) (origin, orientation Vec3) {
	b := e.Bytes()
	origin = coords(&b, offDst)
	orientation = Vec3{float32(i16At(&b, offDst+6)) / 1000, float32(i16At(&b, offDst+8)) / 1000, float32(i16At(&b, offDst+10)) / 1000}
	return origin, orientation
}

// missileOrigin decodes the creation point of a missile from value.
func missileOrigin(e *evtc.Event) Vec3 {
	b := e.Bytes()
	return coords(&b, offValue)
}

// missileRemovePlace decodes the removal point of a missile from buff_dmg.
func missileRemovePlace(e *evtc.Event) Vec3 {
	b := e.Bytes()
	return coords(&b, offBuffDmg)
}

// launch decodes a StateMissileLaunch event.
func launch(e *evtc.Event, t time.Duration, target *Agent) Launch {
	b := e.Bytes()
	return Launch{
		Time:      t,
		Target:    target,
		TargetPos: coords(&b, offValue),
		Position:  coords(&b, offValue+6),
		Motion:    b[offIFF],
		Radius:    i16At(&b, offResult),
		Flags:     u32At(&b, offBuffRemove),
		First:     e.IsFlanking != 0,
		Speed:     i16At(&b, offShields),
		Event:     e,
	}
}

// groundMarkerPlace decodes the location of a squad ground marker and
// whether the event removes it: all coordinates zero or infinite.
func groundMarkerPlace(e *evtc.Event) (Vec3, bool) {
	b := e.Bytes()
	p := Vec3{f32At(&b, offSrc), f32At(&b, offSrc+4), f32At(&b, offSrc+8)}
	removed := (p.X == 0 && p.Y == 0 && p.Z == 0) || math.IsInf(float64(p.X), 0) || math.IsInf(float64(p.Y), 0) || math.IsInf(float64(p.Z), 0)
	return p, removed
}
