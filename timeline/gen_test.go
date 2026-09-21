package timeline

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/42atomys/evtc"
)

// genOptions parametrizes genLog.
type genOptions struct {
	players  int
	adds     int
	duration time.Duration
	seed     uint64
}

// genLog generates a deterministic synthetic log: a boss, respawning adds
// sharing instance ids, a gadget with an attack target, and players that
// move, cast, hit, receive buffs, go down and die. Events are appended in
// the order arcdps would write them, which is only nearly sorted by time.
func genLog(o genOptions) *evtc.Log {
	r := rand.New(rand.NewPCG(o.seed, o.seed^0x9e3779b97f4a7c15))
	b := newLog()

	const (
		boss   = uint64(0x20000)
		gadget = uint64(0x30000)
		target = uint64(0x30001)
		might  = uint32(740)
		burn   = uint32(737)
		bossCC = uint32(1200)
		// healSig is the signature of an extension writing heals.
		healSig = uint32(0x9c9b3c99)
	)
	players := make([]uint64, o.players)
	for i := range players {
		players[i] = uint64(0x10000 + i)
		b.player(players[i], uint16(100+i), fmt.Sprintf("P%d", i), fmt.Sprintf(":Acc%d.%04d", i, i), strconv.Itoa(i/5+1), uint32(1+i%9), uint32(i%3*10))
	}
	b.npc(boss, 500, 15375, "Boss")
	adds := make([]uint64, o.adds)
	for i := range adds {
		adds[i] = uint64(0x20001 + i)
		// Five instance ids are shared by the adds, as the game reuses them.
		b.npc(adds[i], uint16(600+i%5), uint16(1000+i%7), fmt.Sprintf("Add%d", i))
	}
	b.gadget(gadget, 900, 7307, "Cannon")
	b.gadget(target, 901, 0, "at30000-7307")
	strikes := []uint32{1000, 1001, 1002, 1003, 1004}
	for _, id := range strikes {
		b.skill(int32(id), fmt.Sprintf("Strike %d", id))
	}
	b.skill(int32(might), "Might")
	b.skill(int32(burn), "Burning")
	b.skill(int32(bossCC), "Slam")
	b.add(evtc.Event{SkillID: might, OverstackValue: 30000, SrcMasterInstanceID: 25, Pad61: 4, IsStateChange: evtc.StateBuffInfo})
	b.add(evtc.Event{SkillID: burn, OverstackValue: 0, SrcMasterInstanceID: 1500, Pad61: 4, IsOffcycle: 2, IsStateChange: evtc.StateBuffInfo})
	b.pov(players[0])
	b.logNPCUpdate(0, 15375, boss)
	b.session(evtc.StateLanguage, 2)
	b.session(evtc.StateGWBuild, 170000)
	b.session(evtc.StateShardID, 7)
	b.session(evtc.StateRuleset, 1)
	b.instanceStart(0, 0)
	b.arcBuild("arcdps gen")
	b.idToGUID(ContentEffect, 6000, GUID{0x60, 0, 1}, 3000)
	b.idToGUID(ContentSkill, strikes[0], GUID{0x10, 0, 1}, 0)
	b.skillInfo(strikes[0], 3, 130, 900, 0.5)
	b.skillTiming(strikes[0], 1, 200)
	b.buffFormula(might, [11]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	b.guild(0, players[0], GUID{0xAB, 0xCD})
	b.idToGUID(ContentMarker, 1, CommanderTagRed, 0)
	b.idToGUID(ContentMarker, 3, MarkerHeart, 0)
	b.marker(0, players[0], 1, true)
	b.extension(0, 0x07000002_00000000|uint64(healSig), "gen 1.0")
	b.integrity("generated log")
	b.attackTarget(0, target, gadget)
	b.maxHealth(0, boss, 30_000_000)
	for _, p := range players {
		b.state(0, p, evtc.StateEnterCombat)
	}

	results := []evtc.Result{
		evtc.ResultStrikeDamageNormal, evtc.ResultStrikeDamageNormal, evtc.ResultStrikeDamageCrit,
		evtc.ResultStrikeDamageGlance, evtc.ResultBlock, evtc.ResultEvade, evtc.ResultAbsorb, evtc.ResultBlind,
	}
	type playerState struct {
		x, y      float32
		stacks    []uint32
		downedAt  uint64
		dead      bool
		down      bool
		castSkill uint32
	}
	states := make([]playerState, o.players)
	for i := range states {
		states[i].x, states[i].y = float32(r.IntN(2000)), float32(r.IntN(2000))
	}
	stackID := uint32(1)
	bossHP := 100.0
	nextAdd, addSpawned := 0, uint64(0)
	activeAdd := uint64(0)
	type pending struct {
		id    uint32
		agent uint64
		at    uint64
		kind  evtc.StateChange
	}
	var removals []pending
	trackable := uint32(5000)
	sets := make([]uint32, o.players)

	end := uint64(o.duration / time.Millisecond)
	for t := uint64(100); t < end; t += 100 {
		if t%300 == 0 {
			for i, p := range players {
				s := &states[i]
				if s.dead {
					continue
				}
				s.x += float32(r.IntN(61) - 30)
				s.y += float32(r.IntN(61) - 30)
				b.move(t, p, evtc.StatePosition, s.x, s.y, -2400)
				if r.IntN(50) == 0 {
					// One teleport out of two has no target, as in the logs
					// of arcdps 20260915.
					if t%600 == 0 {
						b.move(t, p, evtc.StateTeleport, 0, 0, 0)
					} else {
						b.move(t, p, evtc.StateTeleport, s.x+500, s.y, -2400)
					}
				}
			}
			b.facing(t, boss, float32(r.Float64()*2-1), float32(r.Float64()*2-1))
			bossHP = max(bossHP-r.Float64()*0.2, 0)
			b.health(t, boss, bossHP)
		}
		for i, p := range players {
			s := &states[i]
			if s.dead {
				continue
			}
			if s.down {
				if t >= s.downedAt+3000 {
					if r.IntN(3) == 0 {
						b.hit(t, 0, p, evtc.SkillGenericKill, 0, evtc.ResultKillingBlow)
						b.state(t, p, evtc.StateChangeDead)
						s.dead = true
					} else {
						b.state(t, p, evtc.StateChangeUp)
						s.down = false
					}
				}
				continue
			}
			if s.castSkill == 0 && r.IntN(10) == 0 {
				s.castSkill = strikes[r.IntN(len(strikes))]
				b.castStart(t, p, boss, s.castSkill, 400, 600)
				for range 1 + r.IntN(3) {
					b.hit(t+50, p, boss, s.castSkill, int32(500+r.IntN(4000)), results[r.IntN(len(results))])
				}
			} else if s.castSkill != 0 {
				act := evtc.ActivationReset
				if r.IntN(8) == 0 {
					act = evtc.ActivationCancel
				}
				b.castStop(t, p, s.castSkill, 100, act)
				s.castSkill = 0
			}
			if r.IntN(3) == 0 && len(s.stacks) < 25 {
				b.buffApply(t, players[r.IntN(len(players))], p, might, 10000, stackID)
				s.stacks = append(s.stacks, stackID)
				stackID++
			}
			if r.IntN(4) == 0 && len(s.stacks) > 0 {
				j := r.IntN(len(s.stacks))
				remover := uint64(0)
				if r.IntN(4) == 0 {
					remover = boss
				}
				b.buffRemoveSingle(t, p, remover, might, int32(r.IntN(10000)), s.stacks[j], evtc.BuffRemoveSingle)
				s.stacks[j] = s.stacks[len(s.stacks)-1]
				s.stacks = s.stacks[:len(s.stacks)-1]
			}
			if r.IntN(400) == 0 && len(s.stacks) > 0 {
				b.buffRemoveAll(t, p, p, might)
				s.stacks = s.stacks[:0]
			}
			if r.IntN(5) == 0 {
				b.buffTick(t, p, boss, burn, int32(100+r.IntN(400)))
			}
			if r.IntN(25) == 0 && len(s.stacks) > 0 {
				id := s.stacks[r.IntN(len(s.stacks))]
				if r.IntN(2) == 0 {
					b.buffDeactive(t, p, id, 5000)
				} else {
					b.buffActive(t, p, id, 5000)
				}
			}
			if r.IntN(150) == 0 {
				from := sets[i]
				sets[i] = (sets[i] + 1) % 2
				b.weaponSwap(t, p, from, sets[i])
			}
			if r.IntN(300) == 0 {
				b.stealth(t, p, uint64(r.IntN(3)))
			}
			if r.IntN(500) == 0 {
				b.glider(t, p, r.IntN(2) == 0)
			}
			if r.IntN(700) == 0 {
				b.transformation(t, p, uint32(r.IntN(2))*strikes[2], 3000)
			}
			if r.IntN(800) == 0 {
				b.stunBreak(t, p, int32(r.IntN(2000)))
			}
			if r.IntN(900) == 0 {
				b.marker(t, p, uint32(r.IntN(9)), false)
			}
			if r.IntN(2000) == 0 {
				b.teamChange(t, p, uint32(1+r.IntN(3)), 0)
			}
			if r.IntN(400) == 0 {
				b.jump(t, p, true)
				b.jump(t+300, p, false)
			}
			if r.IntN(60) == 0 {
				b.extensionCombat(t, p, players[r.IntN(len(players))], strikes[3], -int32(100+r.IntN(500)), healSig)
			}
			if r.IntN(40) == 0 {
				trackable++
				if r.IntN(2) == 0 {
					b.groundEffect(t, p, 6000+uint32(r.IntN(3)), trackable, Vec3{s.x, s.y, -2400}, Vec3{0, 0, 1}, uint32(r.IntN(3))*1000, 1000, false, 0)
					removals = append(removals, pending{trackable, 0, t + uint64(500+r.IntN(2500)), evtc.StateEffectGroundRemove})
				} else {
					b.agentEffect(t, p, 6000+uint32(r.IntN(3)), trackable, uint32(r.IntN(3))*1000)
					removals = append(removals, pending{trackable, p, t + uint64(500+r.IntN(2500)), evtc.StateEffectAgentRemove})
				}
			}
			if r.IntN(60) == 0 {
				trackable++
				b.missileCreate(t, p, strikes[r.IntN(len(strikes))], trackable, Vec3{s.x, s.y, -2400}, 0)
				b.missileLaunch(t+50, p, boss, trackable, Vec3{100, 100, -2400}, Vec3{s.x, s.y, -2400}, 1, 20, 0, true, 800)
				if r.IntN(3) == 0 {
					b.missileEffect(t+80, p, trackable, 6000, 1000)
				}
				removals = append(removals, pending{trackable, p, t + uint64(300+r.IntN(1500)), evtc.StateMissileRemove})
			}
			if r.IntN(3000) == 0 {
				b.hit(t, boss, p, bossCC, 0, evtc.ResultDowned)
				b.state(t, p, evtc.StateChangeDown)
				s.down, s.downedAt = true, t
			}
		}
		if t%2000 == 0 {
			victim := players[r.IntN(len(players))]
			b.castStart(t, boss, victim, bossCC, 484, 716)
			for _, p := range players[:min(3, len(players))] {
				res := evtc.ResultStrikeDamageNormal
				if r.IntN(2) == 0 {
					res = evtc.ResultBlock
				}
				b.hit(t+500, boss, p, bossCC, int32(1000+r.IntN(2000)), res)
			}
			b.castStop(t+1400, boss, bossCC, 1400, evtc.ActivationReset)
		}
		if t%30000 == 0 && len(players) > 0 {
			b.defianceState(t, boss, DefianceActive)
			b.defiancePercent(t, boss, 1)
			b.defiancePercent(t+1000, boss, 0.5)
			b.hit(t+500, players[0], boss, strikes[0], 1000, evtc.ResultDefianceDamageNormal)
			b.hit(t+800, 0, boss, evtc.SkillDefianceDamage, -50, evtc.ResultDefianceDamageNormal)
			b.defiancePercent(t+2000, boss, 0)
			b.defianceState(t+2100, boss, DefianceRecover)
		}
		if t%2500 == 0 {
			b.tick(t, t/25, int32(r.IntN(120)))
		}
		if t%15000 == 0 {
			b.groundMarker(t, uint32(r.IntN(8)), Vec3{float32(r.IntN(2000)), float32(r.IntN(2000)), -2400})
		}
		if t%15000 == 7500 {
			b.groundMarker(t, uint32(r.IntN(8)), Vec3{})
		}
		if t%12000 == 0 {
			b.gadgetAnimation(t, gadget, uint64(1+r.IntN(5)))
			b.gadgetName(t, gadget, uint64(r.IntN(3)))
		}
		if t%45000 == 0 {
			b.reward(t, uint64(r.IntN(1000)), int32(r.IntN(5)))
		}
		if t == 30000 {
			b.mapChange(t, 1155, 1062, 4)
		}
		kept := removals[:0]
		for _, rm := range removals {
			if rm.at > t {
				kept = append(kept, rm)
				continue
			}
			switch rm.kind {
			case evtc.StateMissileRemove:
				b.missileRemove(t, rm.agent, strikes[0], rm.id, int32(r.IntN(100)), r.IntN(2) == 0, Vec3{100, 100, -2400})
			default:
				b.effectRemove(t, rm.agent, rm.kind, rm.id)
			}
		}
		removals = kept
		if len(adds) > 0 && t%20000 == 0 {
			if activeAdd != 0 {
				b.state(t-100, activeAdd, evtc.StateDespawn)
			}
			activeAdd = adds[nextAdd%len(adds)]
			nextAdd++
			addSpawned = t
			b.state(t, activeAdd, evtc.StateSpawn)
		}
		if activeAdd != 0 && t-addSpawned < 8000 && r.IntN(4) == 0 && len(players) > 0 {
			b.hit(t, activeAdd, players[r.IntN(len(players))], strikes[1], int32(200+r.IntN(300)), evtc.ResultStrikeDamageNormal)
			if r.IntN(3) == 0 {
				trackable++
				b.buffApply(t, players[r.IntN(len(players))], activeAdd, burn, 3000, trackable)
			}
		}
		if activeAdd != 0 && t-addSpawned >= 8000 {
			if nextAdd%2 == 0 {
				b.stateByInst(t, uint16(600+(nextAdd-1)%5), evtc.StateDespawn)
			} else {
				b.state(t, activeAdd, evtc.StateDespawn)
			}
			activeAdd = 0
		}
	}
	for _, p := range players {
		b.state(end, p, evtc.StateExitCombat)
	}
	return b.build(end)
}

// encodeLog serializes a log in the on-disk EVTC layout, for fuzz seeds
// and files.
func encodeLog(l *evtc.Log) []byte {
	var out bytes.Buffer
	out.WriteString("EVTC")
	build := l.Header.Build
	for len(build) < 8 {
		build += "0"
	}
	out.WriteString(build[:8])
	out.WriteByte(l.Header.Revision)
	binary.Write(&out, binary.LittleEndian, l.Header.TargetSpeciesID)
	out.WriteByte(0)

	binary.Write(&out, binary.LittleEndian, uint32(len(l.Agents)))
	for _, a := range l.Agents {
		rec := make([]byte, 96)
		binary.LittleEndian.PutUint64(rec[0:], a.Addr)
		binary.LittleEndian.PutUint32(rec[8:], a.Profession)
		binary.LittleEndian.PutUint32(rec[12:], a.IsElite)
		binary.LittleEndian.PutUint16(rec[16:], uint16(a.Toughness))
		binary.LittleEndian.PutUint16(rec[18:], uint16(a.Concentration))
		binary.LittleEndian.PutUint16(rec[20:], uint16(a.Healing))
		binary.LittleEndian.PutUint16(rec[22:], a.HitboxWidth)
		binary.LittleEndian.PutUint16(rec[24:], uint16(a.Condition))
		binary.LittleEndian.PutUint16(rec[26:], a.HitboxHeight)
		name := a.Name + "\x00" + a.Account + "\x00" + a.Subgroup + "\x00"
		copy(rec[28:92], name)
		out.Write(rec)
	}
	binary.Write(&out, binary.LittleEndian, uint32(len(l.Skills)))
	for _, s := range l.Skills {
		rec := make([]byte, 68)
		binary.LittleEndian.PutUint32(rec[0:], uint32(s.ID))
		copy(rec[4:68], s.Name+"\x00")
		out.Write(rec)
	}
	out.Write(encodeEvents(l.Events))
	return out.Bytes()
}

// encodeEvents serializes events in the 64-byte revision 1 layout.
func encodeEvents(events []evtc.Event) []byte {
	out := make([]byte, 0, 64*len(events))
	for i := range events {
		rec := events[i].Bytes()
		out = append(out, rec[:]...)
	}
	return out
}
