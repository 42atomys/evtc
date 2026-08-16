// Package evtc decodes arcdps EVTC combat logs (.evtc and zip-compressed
// .zevtc files).
//
// The binary layout follows the arcdps reference:
//   - https://www.deltaconnected.com/arcdps/evtc/README.txt
//   - https://www.deltaconnected.com/arcdps/evtc/writeencounter.cpp
//
// Only revision 1 of the cbtevent structure is supported.
package evtc

import "encoding/binary"

// Header is the 16-byte file header.
type Header struct {
	// Build is the arcdps build date as written in the file, formatted
	// yyyymmdd. It identifies the format version of the log.
	Build string
	// Revision of the cbtevent structure. Only revision 1 is supported.
	Revision uint8
	// TargetSpeciesID is the species id of the logged boss. A value of 1
	// means a WvW log, 2 means a map log.
	TargetSpeciesID uint16
}

// Agent mirrors the evtc_agent structure (96 bytes on disk).
type Agent struct {
	// Addr is the agent address events refer to.
	Addr uint64
	// Profession is the profession id of a player, or the species id of
	// an NPC or the volatile id of a gadget in its low 16 bits.
	Profession uint32
	// IsElite is the elite specialization id of a player, 0xFFFFFFFF for
	// an NPC or a gadget.
	IsElite uint32
	// Toughness is the toughness of a player, as arcdps reports it.
	Toughness int16
	// Concentration is the concentration of a player, as arcdps reports it.
	Concentration int16
	// Healing is the healing power of a player, as arcdps reports it.
	Healing int16
	// HitboxWidth is the hitbox width of the agent.
	HitboxWidth uint16
	// Condition is the condition damage of a player, as arcdps reports it.
	Condition int16
	// HitboxHeight is the hitbox height of the agent.
	HitboxHeight uint16
	// Name is the agent name, decoded from the 64-byte combo string
	// "name\0account\0subgroup\0".
	Name string
	// Account is the account name of a player, empty otherwise.
	Account string
	// Subgroup is the squad subgroup of a player, empty otherwise.
	Subgroup string
}

// Skill mirrors the evtc_skill structure (68 bytes on disk).
type Skill struct {
	// ID is the skill id.
	ID int32
	// Name is the skill name, possibly empty.
	Name string
}

// Event mirrors the cbtevent structure, revision 1 (64 bytes on disk).
// All fields except Time are event-specific: their meaning depends on
// IsStateChange. See the arcdps README for each event type.
type Event struct {
	// Time is the event time in milliseconds, or a payload for the
	// metadata events that carry none.
	Time uint64
	// SrcAgent is the address of the source agent, or a payload.
	SrcAgent uint64
	// DstAgent is the address of the destination agent, or a payload.
	DstAgent uint64
	// Value is the strike damage, the buff duration or a payload.
	Value int32
	// BuffDamage is the damage of a buff tick, or a payload.
	BuffDamage int32
	// OverstackValue is the overstack or barrier value, or a payload.
	OverstackValue uint32
	// SkillID is the skill or buff id of the event.
	SkillID uint32
	// SrcInstanceID is the instance id of the source agent.
	SrcInstanceID uint16
	// DstInstanceID is the instance id of the destination agent.
	DstInstanceID uint16
	// SrcMasterInstanceID is the instance id of the master of the source,
	// 0 when it has none.
	SrcMasterInstanceID uint16
	// DstMasterInstanceID is the instance id of the master of the
	// destination, 0 when it has none.
	DstMasterInstanceID uint16
	// IFF is the friend or foe relation of the source to the destination.
	IFF IFF
	// Buff is non-zero for buff events.
	Buff uint8
	// Result is the outcome of a strike, or a payload.
	Result Result
	// IsActivation is the activation kind of a legacy cast event.
	IsActivation Activation
	// IsBuffRemove is the kind of a buff removal.
	IsBuffRemove BuffRemove
	// IsNinety is set when the source was above 90% health.
	IsNinety uint8
	// IsFifty is set when the destination was below 50% health.
	IsFifty uint8
	// IsMoving has bit 0 set when the source was moving and bit 1 when
	// the destination was.
	IsMoving uint8
	// IsStateChange is the kind of the event, StateCombat for a strike or
	// a buff tick.
	IsStateChange StateChange
	// IsFlanking is set when the source was flanking the destination.
	IsFlanking uint8
	// IsShields is set when barrier absorbed part of a strike, or when a
	// buff was active on application.
	IsShields uint8
	// IsOffcycle is set when the destination was down, or carries the
	// category of a buff.
	IsOffcycle uint8
	// Pad61 to Pad64 are four pad bytes carrying event-specific data, such
	// as the trackable id of buff, missile and effect events.
	Pad61, Pad62, Pad63, Pad64 uint8
}

// Log is a fully decoded EVTC file.
type Log struct {
	// Header is the file header.
	Header Header
	// Agents is the agent table.
	Agents []Agent
	// Skills is the skill table.
	Skills []Skill
	// Events are the combat events, in file order.
	Events []Event
}

// Bytes returns the 64-byte wire layout of the event, so that the payloads
// arcdps spreads over several fields (float arrays, int16 coordinates,
// GUIDs, strings) can be decoded at their documented offsets.
func (e *Event) Bytes() [64]byte {
	var b [64]byte
	binary.LittleEndian.PutUint64(b[0:], e.Time)
	binary.LittleEndian.PutUint64(b[8:], e.SrcAgent)
	binary.LittleEndian.PutUint64(b[16:], e.DstAgent)
	binary.LittleEndian.PutUint32(b[24:], uint32(e.Value))
	binary.LittleEndian.PutUint32(b[28:], uint32(e.BuffDamage))
	binary.LittleEndian.PutUint32(b[32:], e.OverstackValue)
	binary.LittleEndian.PutUint32(b[36:], e.SkillID)
	binary.LittleEndian.PutUint16(b[40:], e.SrcInstanceID)
	binary.LittleEndian.PutUint16(b[42:], e.DstInstanceID)
	binary.LittleEndian.PutUint16(b[44:], e.SrcMasterInstanceID)
	binary.LittleEndian.PutUint16(b[46:], e.DstMasterInstanceID)
	b[48] = uint8(e.IFF)
	b[49] = e.Buff
	b[50] = uint8(e.Result)
	b[51] = uint8(e.IsActivation)
	b[52] = uint8(e.IsBuffRemove)
	b[53] = e.IsNinety
	b[54] = e.IsFifty
	b[55] = e.IsMoving
	b[56] = uint8(e.IsStateChange)
	b[57] = e.IsFlanking
	b[58] = e.IsShields
	b[59] = e.IsOffcycle
	b[60], b[61], b[62], b[63] = e.Pad61, e.Pad62, e.Pad63, e.Pad64
	return b
}
