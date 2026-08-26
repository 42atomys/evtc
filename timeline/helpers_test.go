package timeline

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/42atomys/evtc"
)

// logBuilder assembles an in-memory evtc.Log for tests. Times are raw
// event times; the squad combat start is written at rawEpoch so that a
// raw time of rawEpoch+n is n milliseconds on the timeline.
type logBuilder struct {
	l    *evtc.Log
	inst map[uint64]uint16
}

const rawEpoch = 1000

func newLog() *logBuilder {
	b := &logBuilder{
		l:    &evtc.Log{Header: evtc.Header{Build: "20260816", Revision: 1, TargetSpeciesID: 15375}},
		inst: map[uint64]uint16{},
	}
	b.add(evtc.Event{Time: rawEpoch, SrcAgent: 0x637261, Value: 1787685623, BuffDamage: 1787685624, IsStateChange: evtc.StateSquadCombatStart})
	return b
}

// build returns the log, appending a squad combat end at rawEpoch+end.
func (b *logBuilder) build(end uint64) *evtc.Log {
	b.add(evtc.Event{Time: rawEpoch + end, SrcAgent: 0x637261, Value: 1787685623 + int32(end/1000), IsStateChange: evtc.StateSquadCombatEnd})
	return b.l
}

func (b *logBuilder) player(addr uint64, inst uint16, name, account, sub string, prof, elite uint32) {
	b.l.Agents = append(b.l.Agents, evtc.Agent{Addr: addr, Profession: prof, IsElite: elite, Name: name, Account: account, Subgroup: sub, HitboxWidth: 24})
	b.inst[addr] = inst
}

func (b *logBuilder) npc(addr uint64, inst uint16, species uint16, name string) {
	b.l.Agents = append(b.l.Agents, evtc.Agent{Addr: addr, Profession: uint32(species), IsElite: 0xffffffff, Name: name, Toughness: 100})
	b.inst[addr] = inst
}

func (b *logBuilder) gadget(addr uint64, inst uint16, id uint16, name string) {
	b.l.Agents = append(b.l.Agents, evtc.Agent{Addr: addr, Profession: 0xffff0000 | uint32(id), IsElite: 0xffffffff, Name: name})
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
	if e.DstInstanceID == 0 && dstIsAgent(e.IsStateChange) {
		e.DstInstanceID = b.inst[e.DstAgent]
	}
	b.l.Events = append(b.l.Events, e)
}

func (b *logBuilder) at(t uint64) uint64 { return rawEpoch + t }

func (b *logBuilder) hit(t, src, dst uint64, skill uint32, value int32, result evtc.Result) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, Value: value, Result: result, IFF: evtc.IFFFoe})
}

func (b *logBuilder) buffTick(t, src, dst uint64, skill uint32, dmg int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, BuffDamage: dmg, Buff: 1, Result: evtc.ResultBuffDamageCycle})
}

func (b *logBuilder) castStart(t, src, dst uint64, skill uint32, expected, control int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, Value: expected, BuffDamage: control, IsStateChange: evtc.StateAnimationStart})
}

func (b *logBuilder) castStop(t, src uint64, skill uint32, elapsed int32, act evtc.Activation) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: src, SkillID: skill, Value: elapsed, BuffDamage: elapsed, IsActivation: act, IsStateChange: evtc.StateAnimationStop})
}

func withTrackable(e evtc.Event, id uint32) evtc.Event {
	e.Pad61, e.Pad62, e.Pad63, e.Pad64 = uint8(id), uint8(id>>8), uint8(id>>16), uint8(id>>24)
	return e
}

func (b *logBuilder) buffApply(t, src, dst uint64, skill uint32, dur int32, id uint32) {
	b.add(withTrackable(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, Value: dur, IsShields: 1, IsStateChange: evtc.StateBuffApply}, id))
}

func (b *logBuilder) buffInitial(t, src, dst uint64, skill uint32, remaining, original int32, id uint32) {
	b.add(withTrackable(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, Value: remaining, BuffDamage: original, IsStateChange: evtc.StateBuffInitial}, id))
}

func (b *logBuilder) buffChange(t, dst uint64, skill uint32, diff int32, id uint32) {
	b.add(withTrackable(evtc.Event{Time: b.at(t), DstAgent: dst, SkillID: skill, Value: diff, IsStateChange: evtc.StateBuffChange}, id))
}

func (b *logBuilder) buffRemoveSingle(t, agent, remover uint64, skill uint32, remaining int32, id uint32, how evtc.BuffRemove) {
	b.add(withTrackable(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: remover, SkillID: skill, Value: remaining, IsBuffRemove: how, IsStateChange: evtc.StateBuffRemoveSingle}, id))
}

func (b *logBuilder) buffRemoveAll(t, agent, remover uint64, skill uint32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: remover, SkillID: skill, IsBuffRemove: evtc.BuffRemoveAll, IsStateChange: evtc.StateBuffRemoveAll})
}

func packVec3(x, y, z float32) (uint64, int32) {
	return uint64(math.Float32bits(x)) | uint64(math.Float32bits(y))<<32, int32(math.Float32bits(z))
}

func (b *logBuilder) move(t, agent uint64, kind evtc.StateChange, x, y, z float32) {
	dst, value := packVec3(x, y, z)
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: dst, Value: value, IsStateChange: kind})
}

func (b *logBuilder) facing(t, agent uint64, x, y float32) {
	dst, _ := packVec3(x, y, 0)
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: dst, IsStateChange: evtc.StateFacing})
}

func (b *logBuilder) health(t, agent uint64, pct float64) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: uint64(pct * 100), IsStateChange: evtc.StateHealthPctUpdate})
}

func (b *logBuilder) maxHealth(t, agent uint64, hp uint64) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: hp, IsStateChange: evtc.StateMaxHealthUpdate})
}

func (b *logBuilder) state(t, agent uint64, kind evtc.StateChange) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, IsStateChange: kind})
}

func (b *logBuilder) defianceState(t, agent uint64, state DefianceState) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, Value: int32(state), IsStateChange: evtc.StateDefianceBarState})
}

func (b *logBuilder) defiancePercent(t, agent uint64, fraction float32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, Value: int32(math.Float32bits(fraction)), IsStateChange: evtc.StateDefianceBarPercent})
}

func (b *logBuilder) targetable(t, agent uint64, on bool) {
	var v uint64
	if on {
		v = 1
	}
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: v, IsStateChange: evtc.StateTargetable})
}

func (b *logBuilder) attackTarget(t, target, gadget uint64) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: target, DstAgent: gadget, IsStateChange: evtc.StateAttackTarget})
}

func (b *logBuilder) logNPCUpdate(t uint64, species uint16, addr uint64) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: uint64(species), DstAgent: addr, IsStateChange: evtc.StateLogNPCUpdate})
}

func (b *logBuilder) pov(addr uint64) {
	b.add(evtc.Event{Time: rawEpoch, SrcAgent: addr, IsStateChange: evtc.StatePointOfView})
}

// minionHit appends a hit whose source is a minion of the master with the
// given instance id.
func (b *logBuilder) minionHit(t, src, dst uint64, master uint16, skill uint32, value int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SrcMasterInstanceID: master, SkillID: skill, Value: value, IFF: evtc.IFFFoe})
}

func mustBuild(tb testing.TB, l *evtc.Log) *Timeline {
	tb.Helper()
	tl, err := Build(l)
	if err != nil {
		tb.Fatal(err)
	}
	return tl
}

// Agents of the standard fixture.
const (
	addrP1    = 0x1001
	addrP2    = 0x1002
	addrBoss  = 0x2001
	addrAdd   = 0x2002
	addrPet   = 0x2003
	addrGad   = 0x3001
	addrAT    = 0x3002
	instP1    = 11
	instP2    = 12
	instBoss  = 21
	instAdd   = 22
	instPet   = 23
	instGad   = 31
	instAT    = 32
	skillHeat = 100
	skillSlam = 200
	skillBuff = 740
	skillBurn = 737
)

// fixture builds the standard cast of agents and skills.
func fixture() *logBuilder {
	b := newLog()
	b.player(addrP1, instP1, "Alpha", ":Alpha.1234", "1", 1, 0)
	b.player(addrP2, instP2, "Bravo", ":Bravo.5678", "2", 2, 65)
	b.npc(addrBoss, instBoss, 15375, "Sabetha")
	b.npc(addrAdd, instAdd, 1083, "Crow")
	b.npc(addrPet, instPet, 6524, "Elemental")
	b.gadget(addrGad, instGad, 7307, "Cannon")
	b.gadget(addrAT, instAT, 0, "at3001-7307")
	b.skill(skillHeat, "Heat")
	b.skill(skillSlam, "Slam")
	b.skill(skillBuff, "Might")
	b.skill(skillBurn, "Burning")
	b.pov(addrP1)
	b.logNPCUpdate(200, 15375, addrBoss)
	return b
}

func (b *logBuilder) barrier(t, agent uint64, pct float64) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: uint64(pct * 100), IsStateChange: evtc.StateBarrierPctUpdate})
}

// buffInfo declares a buff: its stacking type, stack limit and category
// (0 boon, 2 condition).
func (b *logBuilder) buffInfo(skill uint32, stacking Stacking, limit uint16, category uint8) {
	b.add(evtc.Event{SkillID: skill, SrcMasterInstanceID: limit, Pad61: uint8(stacking), IsOffcycle: category, IsStateChange: evtc.StateBuffInfo})
}

// held returns the value of a two-value lookup, ignoring the boolean.
func held[T any](v T, _ bool) T { return v }

// eventFromBytes decodes the 64-byte wire layout of an event, the inverse
// of evtc.Event.Bytes, so that tests can write payloads at their offsets.
func eventFromBytes(b [64]byte) evtc.Event {
	return evtc.Event{
		Time:                binary.LittleEndian.Uint64(b[0:]),
		SrcAgent:            binary.LittleEndian.Uint64(b[8:]),
		DstAgent:            binary.LittleEndian.Uint64(b[16:]),
		Value:               int32(binary.LittleEndian.Uint32(b[24:])),
		BuffDamage:          int32(binary.LittleEndian.Uint32(b[28:])),
		OverstackValue:      binary.LittleEndian.Uint32(b[32:]),
		SkillID:             binary.LittleEndian.Uint32(b[36:]),
		SrcInstanceID:       binary.LittleEndian.Uint16(b[40:]),
		DstInstanceID:       binary.LittleEndian.Uint16(b[42:]),
		SrcMasterInstanceID: binary.LittleEndian.Uint16(b[44:]),
		DstMasterInstanceID: binary.LittleEndian.Uint16(b[46:]),
		IFF:                 evtc.IFF(b[48]),
		Buff:                b[49],
		Result:              evtc.Result(b[50]),
		IsActivation:        evtc.Activation(b[51]),
		IsBuffRemove:        evtc.BuffRemove(b[52]),
		IsNinety:            b[53],
		IsFifty:             b[54],
		IsMoving:            b[55],
		IsStateChange:       evtc.StateChange(b[56]),
		IsFlanking:          b[57],
		IsShields:           b[58],
		IsOffcycle:          b[59],
		Pad61:               b[60],
		Pad62:               b[61],
		Pad63:               b[62],
		Pad64:               b[63],
	}
}

// raw appends an event whose payload is written by set on the wire layout.
func (b *logBuilder) raw(e evtc.Event, set func(buf *[64]byte)) {
	buf := e.Bytes()
	set(&buf)
	b.add(eventFromBytes(buf))
}

func putF32(buf *[64]byte, off int, v float32) {
	binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(v))
}

func putI16(buf *[64]byte, off int, v int16) { binary.LittleEndian.PutUint16(buf[off:], uint16(v)) }

func putU32(buf *[64]byte, off int, v uint32) { binary.LittleEndian.PutUint32(buf[off:], v) }

// putCoords writes three coordinates as int16 of the value divided by
// ten, as arcdps does.
func putCoords(buf *[64]byte, off int, v Vec3) {
	putI16(buf, off, int16(v.X/10))
	putI16(buf, off+2, int16(v.Y/10))
	putI16(buf, off+4, int16(v.Z/10))
}

func putTrackable(buf *[64]byte, id uint32) { putU32(buf, offPad61, id) }

func (b *logBuilder) session(kind evtc.StateChange, src uint64) {
	b.add(evtc.Event{Time: rawEpoch, SrcAgent: src, IsStateChange: kind})
}

// instanceStart records that the instance started startedAt milliseconds
// after the log origin.
func (b *logBuilder) instanceStart(t, startedAt uint64) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: b.at(startedAt), Value: 42, IsStateChange: evtc.StateInstanceStart})
}

func (b *logBuilder) arcBuild(s string) {
	b.raw(evtc.Event{IsStateChange: evtc.StateArcBuild}, func(buf *[64]byte) { copy(buf[offSrc:], s) })
}

func (b *logBuilder) tick(t uint64, counter uint64, ping int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: counter, Value: ping, IsStateChange: evtc.StateTick})
}

func (b *logBuilder) groundMarker(t uint64, index uint32, pos Vec3) {
	b.raw(evtc.Event{Time: b.at(t), SkillID: index, IsStateChange: evtc.StateSquadMarkerGround}, func(buf *[64]byte) {
		putF32(buf, offSrc, pos.X)
		putF32(buf, offSrc+4, pos.Y)
		putF32(buf, offSrc+8, pos.Z)
	})
}

func (b *logBuilder) marker(t, agent uint64, id uint32, commander bool) {
	e := evtc.Event{Time: b.at(t), SrcAgent: agent, Value: int32(id), IsStateChange: evtc.StateMarker}
	if commander {
		e.Buff = 1
	}
	b.add(e)
}

func (b *logBuilder) guild(t, agent uint64, g GUID) {
	b.raw(evtc.Event{Time: b.at(t), SrcAgent: agent, IsStateChange: evtc.StateGuild}, func(buf *[64]byte) { copy(buf[offDst:], g[:]) })
}

func (b *logBuilder) teamChange(t, agent uint64, to, from uint32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: uint64(to), Value: int32(from), IsStateChange: evtc.StateTeamChange})
}

func (b *logBuilder) weaponSwap(t, agent uint64, from, to uint32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: uint64(to), Value: int32(from), IsStateChange: evtc.StateWeaponSwap})
}

func (b *logBuilder) stealth(t, agent uint64, state uint64) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: state, IsStateChange: evtc.StateStealthChange})
}

func (b *logBuilder) glider(t, agent uint64, deployed bool) {
	e := evtc.Event{Time: b.at(t), SrcAgent: agent, IsStateChange: evtc.StateGlider}
	if deployed {
		e.Value = 1
	}
	b.add(e)
}

func (b *logBuilder) transformation(t, agent uint64, skill uint32, dur int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, SkillID: skill, Value: dur, IsStateChange: evtc.StateTransformation})
}

func (b *logBuilder) stunBreak(t, agent uint64, remaining int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, Value: remaining, IsStateChange: evtc.StateStunBreak})
}

func (b *logBuilder) skillInfo(skill uint32, cost, minRange, maxRange, tooltipSeconds float32) {
	b.raw(evtc.Event{SkillID: skill, IsStateChange: evtc.StateSkillInfo}, func(buf *[64]byte) {
		putF32(buf, offTime, cost)
		putF32(buf, offTime+4, minRange)
		putF32(buf, offTime+8, maxRange)
		putF32(buf, offTime+12, tooltipSeconds)
	})
}

func (b *logBuilder) skillTiming(skill, kind uint32, atMS uint64) {
	b.add(evtc.Event{SrcAgent: uint64(kind), DstAgent: atMS, SkillID: skill, IsStateChange: evtc.StateSkillTiming})
}

func (b *logBuilder) buffFormula(skill uint32, values [11]float32) {
	b.raw(evtc.Event{SkillID: skill, IsStateChange: evtc.StateBuffFormula}, func(buf *[64]byte) {
		for i := range 9 {
			putF32(buf, offTime+4*i, values[i])
		}
		putF32(buf, offSrcInst, values[9])
		putF32(buf, offSrcInst+4, values[10])
	})
}

func (b *logBuilder) idToGUID(kind ContentKind, id uint32, g GUID, defaultMS float32) {
	b.raw(evtc.Event{SkillID: id, OverstackValue: uint32(kind), IsStateChange: evtc.StateIDToGUID}, func(buf *[64]byte) {
		copy(buf[offSrc:], g[:])
		putF32(buf, offBuffDmg, defaultMS)
	})
}

func (b *logBuilder) buffActive(t, agent uint64, id uint32, dur int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: uint64(id), Value: dur, IsStateChange: evtc.StateBuffActive})
}

func (b *logBuilder) buffDeactive(t, agent uint64, id uint32, dur int32) {
	b.add(withTrackable(evtc.Event{Time: b.at(t), SrcAgent: agent, Value: dur, IsStateChange: evtc.StateBuffDeactive}, id))
}

func (b *logBuilder) groundEffect(t, agent uint64, effectID, trackable uint32, origin, orient Vec3, durMS uint32, scale int16, moving bool, flags uint8) {
	e := evtc.Event{Time: b.at(t), SrcAgent: agent, SkillID: effectID, IsStateChange: evtc.StateEffectGroundCreate}
	b.raw(e, func(buf *[64]byte) {
		putCoords(buf, offDst, origin)
		putI16(buf, offDst+6, int16(orient.X*1000))
		putI16(buf, offDst+8, int16(orient.Y*1000))
		putI16(buf, offDst+10, int16(orient.Z*1000))
		putU32(buf, offIFF, durMS)
		buf[offBuffRemove] = flags
		if moving {
			buf[offFlanking] = 1
		}
		putI16(buf, offShields, scale)
		putTrackable(buf, trackable)
	})
}

func (b *logBuilder) agentEffect(t, agent uint64, effectID, trackable uint32, durMS uint32) {
	b.raw(evtc.Event{Time: b.at(t), SrcAgent: agent, SkillID: effectID, IsStateChange: evtc.StateEffectAgentCreate}, func(buf *[64]byte) {
		putU32(buf, offIFF, durMS)
		putTrackable(buf, trackable)
	})
}

func (b *logBuilder) effectRemove(t, agent uint64, kind evtc.StateChange, trackable uint32) {
	b.add(withTrackable(evtc.Event{Time: b.at(t), SrcAgent: agent, IsStateChange: kind}, trackable))
}

func (b *logBuilder) missileCreate(t, agent uint64, skill, trackable uint32, origin Vec3, skin uint32) {
	b.raw(evtc.Event{Time: b.at(t), SrcAgent: agent, SkillID: skill, OverstackValue: skin, IsStateChange: evtc.StateMissileCreate}, func(buf *[64]byte) {
		putCoords(buf, offValue, origin)
		putTrackable(buf, trackable)
	})
}

func (b *logBuilder) missileLaunch(t, agent, target uint64, trackable uint32, targetPos, pos Vec3, motion uint8, radius int16, flags uint32, first bool, speed int16) {
	b.raw(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: target, IsStateChange: evtc.StateMissileLaunch}, func(buf *[64]byte) {
		putCoords(buf, offValue, targetPos)
		putCoords(buf, offValue+6, pos)
		buf[offIFF] = motion
		putI16(buf, offResult, radius)
		putU32(buf, offBuffRemove, flags)
		if first {
			buf[offFlanking] = 1
		}
		putI16(buf, offShields, speed)
		putTrackable(buf, trackable)
	})
}

func (b *logBuilder) missileEffect(t, owner uint64, trackable, effectID, durMS uint32) {
	b.add(withTrackable(evtc.Event{Time: b.at(t), DstAgent: owner, SkillID: effectID, Value: int32(durMS), IsStateChange: evtc.StateMissileEffect}, trackable))
}

func (b *logBuilder) missileRemove(t, agent uint64, skill, trackable uint32, friendlyFire int32, hitEnemy bool, pos Vec3) {
	e := evtc.Event{Time: b.at(t), SrcAgent: agent, SkillID: skill, Value: friendlyFire, IsStateChange: evtc.StateMissileRemove}
	if hitEnemy {
		e.IsFlanking = 1
	}
	b.raw(e, func(buf *[64]byte) {
		putCoords(buf, offBuffDmg, pos)
		putTrackable(buf, trackable)
	})
}

func (b *logBuilder) jump(t, agent uint64, leaving bool) {
	e := evtc.Event{Time: b.at(t), SrcAgent: agent, IsStateChange: evtc.StateJump}
	if leaving {
		e.DstAgent = 1
	}
	b.add(e)
}

func (b *logBuilder) gadgetName(t, agent, state uint64) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: state, IsStateChange: evtc.StateGadgetName})
}

func (b *logBuilder) gadgetAnimation(t, agent, token uint64) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: agent, DstAgent: token, IsStateChange: evtc.StateGadgetAnimation})
}

func (b *logBuilder) reward(t, id uint64, kind int32) {
	b.add(evtc.Event{Time: b.at(t), DstAgent: id, Value: kind, IsStateChange: evtc.StateReward})
}

func (b *logBuilder) mapChange(t uint64, to, from uint32, kind int32) {
	b.add(evtc.Event{Time: b.at(t), SrcAgent: uint64(to), DstAgent: uint64(from), Value: kind, IsStateChange: evtc.StateMapChange})
}

// integrity writes a diagnostic message over the time field, cut at the
// 32 bytes of the field.
func (b *logBuilder) integrity(msg string) {
	b.raw(evtc.Event{IsStateChange: evtc.StateIntegrity}, func(buf *[64]byte) { copy(buf[offTime:offTime+32], msg) })
}

// extension registers an extension whose signature is the low 32 bits of
// src and whose version is written as text over dst_agent.
func (b *logBuilder) extension(t, src uint64, version string) {
	b.raw(evtc.Event{Time: b.at(t), SrcAgent: src, IsStateChange: evtc.StateExtension}, func(buf *[64]byte) { copy(buf[offDst:offDst+8], version) })
}

// extensionCombat writes a combat event of the extension with the given
// signature, carried in the pad bytes.
func (b *logBuilder) extensionCombat(t, src, dst uint64, skill uint32, value int32, sig uint32) {
	b.add(withTrackable(evtc.Event{Time: b.at(t), SrcAgent: src, DstAgent: dst, SkillID: skill, BuffDamage: value, Buff: 1, IsStateChange: evtc.StateExtensionCombat}, sig))
}

// stateByInst writes a state event with src_agent 0 and only the instance
// id set, as arcdps does for the despawn of minions.
func (b *logBuilder) stateByInst(t uint64, inst uint16, kind evtc.StateChange) {
	b.add(evtc.Event{Time: b.at(t), SrcInstanceID: inst, IsStateChange: kind})
}
