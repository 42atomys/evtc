package healingstats

import (
	"encoding/binary"
	"testing"

	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

// Agents, instance ids and skills of the fixture: three players, a pet of
// the first one and a boss.
const (
	addrAlpha   = 0x1001 // the recording player
	addrBravo   = 0x1002
	addrCharlie = 0x1003
	addrPet     = 0x2003
	addrBoss    = 0x2001
	instAlpha   = 11
	instBravo   = 12
	instCharlie = 13
	instPet     = 23
	instBoss    = 21
	skillHeal   = 100 // a healing skill
	skillRegen  = 718 // regeneration
	skillSand   = 200 // a barrier skill
	skillSlam   = 300 // a strike
	rawEpoch    = 1000
)

// The recording flags of the addon, as tests write them.
const (
	fromSrc = flagFromSrc
	fromDst = flagFromDst
	both    = flagFromSrc | flagFromDst
)

// logBuilder assembles an in-memory evtc.Log with the events of the addon.
// Times are raw event times; the squad combat start is written at rawEpoch
// so that a raw time of rawEpoch+n is n milliseconds on the timeline.
type logBuilder struct {
	l    *evtc.Log
	inst map[uint64]uint16
}

// fixture builds the standard cast: Alpha records the log, Bravo and
// Charlie are squad members, the pet belongs to Alpha.
func fixture() *logBuilder {
	b := &logBuilder{
		l:    &evtc.Log{Header: evtc.Header{Build: "20260816", Revision: 1, TargetSpeciesID: 15375}},
		inst: map[uint64]uint16{},
	}
	b.add(evtc.Event{Time: rawEpoch, SrcAgent: 0x637261, Value: 1787685623, BuffDamage: 1787685624, IsStateChange: evtc.StateSquadCombatStart})
	b.player(addrAlpha, instAlpha, "Alpha", ":Alpha.1234", "1", 1, 62)
	b.player(addrBravo, instBravo, "Bravo", ":Bravo.5678", "1", 2, 0)
	b.player(addrCharlie, instCharlie, "Charlie", ":Charlie.9012", "2", 3, 0)
	b.npc(addrPet, instPet, 6524, "Elemental")
	b.npc(addrBoss, instBoss, 15375, "Sabetha")
	b.skill(skillHeal, "Shelter")
	b.skill(skillRegen, "Regeneration")
	b.skill(skillSand, "Sand Cascade")
	b.skill(skillSlam, "Slam")
	b.add(evtc.Event{Time: rawEpoch, SrcAgent: addrAlpha, IsStateChange: evtc.StatePointOfView})
	b.add(evtc.Event{Time: rawEpoch, SrcAgent: 15375, DstAgent: addrBoss, IsStateChange: evtc.StateLogNPCUpdate})
	// Every agent is tracked from the start to the end of the log, so that
	// masters resolve at any instant.
	for _, a := range []uint64{addrAlpha, addrBravo, addrCharlie, addrPet, addrBoss} {
		b.state(0, a, evtc.StateEnterCombat)
	}
	return b
}

// build returns the log, appending a squad combat end at rawEpoch+end and
// an exit combat for every agent just before it.
func (b *logBuilder) build(end uint64) *evtc.Log {
	for _, a := range []uint64{addrAlpha, addrBravo, addrCharlie, addrPet, addrBoss} {
		b.state(end-1, a, evtc.StateExitCombat)
	}
	b.add(evtc.Event{Time: rawEpoch + end, SrcAgent: 0x637261, Value: 1787685623 + int32(end/1000), IsStateChange: evtc.StateSquadCombatEnd})
	return b.l
}

func (b *logBuilder) player(addr uint64, inst uint16, name, account, sub string, prof, elite uint32) {
	b.l.Agents = append(b.l.Agents, evtc.Agent{Addr: addr, Profession: prof, IsElite: elite, Name: name, Account: account, Subgroup: sub, HitboxWidth: 24})
	b.inst[addr] = inst
}

func (b *logBuilder) npc(addr uint64, inst uint16, species uint16, name string) {
	b.l.Agents = append(b.l.Agents, evtc.Agent{Addr: addr, Profession: uint32(species), IsElite: 0xffffffff, Name: name})
	b.inst[addr] = inst
}

func (b *logBuilder) skill(id int32, name string) {
	b.l.Skills = append(b.l.Skills, evtc.Skill{ID: id, Name: name})
}

// add appends an event, filling the instance ids of known agents.
func (b *logBuilder) add(e evtc.Event) {
	if e.SrcInstanceID == 0 {
		e.SrcInstanceID = b.inst[e.SrcAgent]
	}
	if e.DstInstanceID == 0 && (e.IsStateChange == evtc.StateExtensionCombat || e.IsStateChange == evtc.StateCombat || e.IsStateChange == evtc.StateAnimationStart) {
		e.DstInstanceID = b.inst[e.DstAgent]
	}
	b.l.Events = append(b.l.Events, e)
}

func (b *logBuilder) at(t uint64) uint64 { return rawEpoch + t }

func (b *logBuilder) state(t, agent uint64, kind evtc.StateChange) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, IsStateChange: kind})
}

// register writes the registration event of the addon: the signature,
// the format revision and the length of the version string packed in
// src_agent, the version string over dst_agent.
func (b *logBuilder) register(t uint64, version string, revision uint32) {
	e := evtc.Event{Time: b.at(t), SrcAgent: uint64(Signature) | uint64(revision&0xffffff)<<32 | uint64(len(version))<<56, IsStateChange: evtc.StateExtension}
	var dst [8]byte
	copy(dst[:], version)
	e.DstAgent = binary.LittleEndian.Uint64(dst[:])
	b.add(e)
}

// signed writes the signature of the addon in the pad bytes of an event.
func signed(e evtc.Event) evtc.Event {
	sig := uint32(Signature)
	e.Pad61, e.Pad62, e.Pad63, e.Pad64 = uint8(sig), uint8(sig>>8), uint8(sig>>16), uint8(sig>>24)
	return e
}

// heal writes the direct heal of a skill: the amount negated in value and
// the recording flags in is_offcycle.
func (b *logBuilder) heal(t, src, dst uint64, skill uint32, amount int32, flags uint8) {
	b.add(signed(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, Value: -amount, IsOffcycle: flags, IsStateChange: evtc.StateExtensionCombat}))
}

// tick writes the tick of a buff: the amount negated in buff_dmg.
func (b *logBuilder) tick(t, src, dst uint64, skill uint32, amount int32, flags uint8) {
	b.add(signed(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, BuffDamage: -amount, Buff: 1, IsOffcycle: flags, IsStateChange: evtc.StateExtensionCombat}))
}

// barrier writes the barrier a skill gave: as a heal with is_shields set
// and the amount repeated in overstack_value, as the addon does.
func (b *logBuilder) barrier(t, src, dst uint64, skill uint32, amount int32, flags uint8) {
	b.add(signed(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, Value: -amount, OverstackValue: uint32(amount), IsShields: 1, IsOffcycle: flags, IsStateChange: evtc.StateExtensionCombat}))
}

// minionHeal writes a heal whose source is a minion of the master with the
// given instance id.
func (b *logBuilder) minionHeal(t, src uint64, master uint16, dst uint64, skill uint32, amount int32, flags uint8) {
	b.add(signed(evtc.Event{Time: b.at(t), SrcAgent: src, SrcMasterInstanceID: master, DstAgent: dst, SkillID: skill, Value: -amount, IsOffcycle: flags, IsStateChange: evtc.StateExtensionCombat}))
}

func (b *logBuilder) hit(t, src, dst uint64, skill uint32, value int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, Value: value, IFF: evtc.IFFFoe})
}

func (b *logBuilder) castStart(t, src, dst uint64, skill uint32, expected int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, Value: expected, BuffDamage: expected, IsStateChange: evtc.StateAnimationStart})
}

func (b *logBuilder) castStop(t, src uint64, skill uint32, elapsed int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: src, SkillID: skill, Value: elapsed, BuffDamage: elapsed, IsActivation: evtc.ActivationReset, IsStateChange: evtc.StateAnimationStop})
}

// mustBuild builds the timeline of a log and returns its healing stats,
// failing when the log carries none.
func mustBuild(tb testing.TB, l *evtc.Log) *Stats {
	tb.Helper()
	tl, err := timeline.Build(l)
	if err != nil {
		tb.Fatal(err)
	}
	s := Of(tl)
	if s == nil {
		tb.Fatal("the log carries no healing stats")
	}
	return s
}

// player returns the node of the player with the given address.
func player(tb testing.TB, s *Stats, addr uint64) *Agent {
	tb.Helper()
	a := s.Agent(s.Timeline.Agent(addr))
	if a == nil {
		tb.Fatalf("no agent %#x", addr)
	}
	return a
}

// encodeLog writes a log in the evtc wire format, for the fuzzer.
func encodeLog(l *evtc.Log) []byte {
	out := []byte("EVTC" + l.Header.Build)
	out = append(out, l.Header.Revision)
	out = binary.LittleEndian.AppendUint16(out, l.Header.TargetSpeciesID)
	out = append(out, 0)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(l.Agents)))
	for _, a := range l.Agents {
		var r [96]byte
		binary.LittleEndian.PutUint64(r[0:], a.Addr)
		binary.LittleEndian.PutUint32(r[8:], a.Profession)
		binary.LittleEndian.PutUint32(r[12:], a.IsElite)
		binary.LittleEndian.PutUint16(r[16:], uint16(a.Toughness))
		binary.LittleEndian.PutUint16(r[18:], uint16(a.Concentration))
		binary.LittleEndian.PutUint16(r[20:], uint16(a.Healing))
		binary.LittleEndian.PutUint16(r[22:], a.HitboxWidth)
		binary.LittleEndian.PutUint16(r[24:], uint16(a.Condition))
		binary.LittleEndian.PutUint16(r[26:], a.HitboxHeight)
		copy(r[28:92], a.Name+"\x00"+a.Account+"\x00"+a.Subgroup+"\x00")
		out = append(out, r[:]...)
	}
	out = binary.LittleEndian.AppendUint32(out, uint32(len(l.Skills)))
	for _, s := range l.Skills {
		var r [68]byte
		binary.LittleEndian.PutUint32(r[0:], uint32(s.ID))
		copy(r[4:], s.Name)
		out = append(out, r[:]...)
	}
	return append(out, encodeEvents(l.Events)...)
}
