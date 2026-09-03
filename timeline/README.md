# timeline

`timeline` turns an arcdps EVTC combat log into a temporal graph you can
query by time: who was where, at what health, casting what, carrying which
buffs, and what every hit did, at any instant of the fight.

Every node of the graph points to its neighbours in both directions. An
agent reaches its hits, casts, buff stacks, downs, deaths and breakbars; a
hit reaches its source, target, skill, cast and the down or death it
caused; a cast reaches its hits; a buff stack reaches its applier, receiver
and definition. Values that change over time are series and spans that
answer "what was the value at t" with a binary search.

```go
import (
	"github.com/42atomys/evtc"
	"github.com/42atomys/evtc/timeline"
)

tl, err := timeline.ParseFile("fight.zevtc") // evtc.ParseFile + timeline.Build
```

Requires Go 1.27 (the query types use generic methods). Logs written by
arcdps builds older than 20260501 use the legacy cast and buff encoding and
are rejected with `timeline.ErrLegacyLog`.

## The model

| Node                                      | Reached from                                                        | Points to                                                                                                                                                         |
| ----------------------------------------- | ------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Timeline`                                | `Build`, `ParseFile`                                                | `Agents`, `Players`, `Targets`, `Skills`, `Buffs`, `POV`, `Unknown`, the raw `Log`                                                                                |
| `Agent` (`Player`, `Target` wrap it)      | `Timeline`, every node                                              | `Hits()`, `HitsCredited()`, `HitsTaken()`, `Casts()`, `Stacks()`, `StacksApplied()`, `Downs`, `Deaths`, `Breakbars`, `Master`, `Minions`, `Events()`              |
| `Hit`                                     | `Hits` queries                                                      | `Src`, `Dst`, `Skill`, `Cast`, `Down`, `Death`, `Event`                                                                                                           |
| `Cast`                                    | `Casts` queries                                                     | `Caster`, `Target`, `Skill`, `Hits()`, `Start`, `Stop`                                                                                                            |
| `BuffStack`                               | `Stacks` queries                                                    | `Buff`, `Applier`, `Receiver`, `RemovedBy`, `Apply`, `Remove`                                                                                                     |
| `Down`, `Death`                           | `Agent.Downs`, `Agent.Deaths`                                       | `Agent`, `Cause` (a `Hit`), each other                                                                                                                            |
| `Breakbar`                                | `Agent.Breakbars`                                                   | `Agent`, `Hits()`, `Percent`                                                                                                                                      |
| `Effect`                                  | `Effects` queries (`Timeline.Effects()`, `Agent.Effects()`)         | `Agent`, `Origin`, `Duration`, `Interval`, `Create`, `Remove`                                                                                                     |
| `Missile`                                 | `Missiles` queries (`Timeline.Missiles()`, `Agent.Missiles()`)      | `Owner`, `Skill`, `Launches` (each with a `Target`), `Effects`, `Remove`                                                                                          |
| `Marker`, `GroundMarker`                  | `Agent.Markers`, `Timeline.GroundMarkers`, `GroundMarkerAt`         | `Agent`, `Squad`, `Tag`, `Interval`, `Event`, `Remove`                                                                                                            |
| `StunBreak`                               | `Agent.StunBreaks`                                                  | `Agent`, `Event`                                                                                                                                                  |
| `GadgetAnimation`                         | `Agent.GadgetAnimations`                                            | `Agent`, `Event`                                                                                                                                                  |
| `Reward`, `MapChange`, `IntegrityMessage` | `Timeline.Rewards`, `MapChanges`, `Integrity`                       | `Event`                                                                                                                                                           |
| `Extension`                               | `Timeline.Extensions`, `Extension(signature)`, `ExtensionOf(event)` | `Events()` (the raw combat events it wrote), `Signature`, `Version`, `Event`, `Decoded` (the graph of the package that knows the extension, see Extensions below) |
| `Skill`, `Buff`                           | `Timeline.Skill(id)`, `Buff(id)`, every node                        | `Casts()`, `Hits()`, `Stacks()`, raw info events                                                                                                                  |

Time-varying values live on `Agent`:

- `Position`, `Velocity`, `Facing` are `Series` interpolated between
  samples closer than `MoveGap` (1.5 s).
- `Health`, `Barrier`, `MaxHealth`, `DefiancePercent` are `Numbers`,
  step series with threshold crossings (`Crossings`, `FirstBelow`).
- `Life`, `Defiance`, `InCombat`, `Targetable`, `Team`, `WeaponSet`,
  `Stealth`, `Gliding`, `Transformation`, `Airborne` and `NameVisible` are
  `Spans`, contiguous states with `At`, `ValueAt`, `Total`, and with
  `TeamAt`, `WeaponSetAt`, `StealthAt`, `TransformationAt`,
  `IsGlidingAt`, `GlidingTime`, `IsAirborneAt`, `AirborneTime`,
  `IsNameVisibleAt` and `NameVisibleTime` as helpers.

Each `BuffStack` carries its own `Active` spans, and `Timeline.Ping`
samples the latency of the recording client (`PingAt`).

Times are `time.Duration` values relative to the squad combat start of the
log. `Interval{Start, End}` is closed on both ends; `tl.Interval()` is the
whole log, `tl.Since(t)`, `tl.Until(t)`, `Around(t, d)` and
`Interval.Split(times...)` build the usual sub-ranges. `tl.WallClock(t)`
converts back to server time.

`Targets` holds the boss of the log first, then every NPC or gadget that
exchanged hits with the players. `tl.Unknown` is the sentinel agent behind
hits and events whose source the log does not know; it is never nil.

## Conventions

- **Read-only, concurrent.** The graph is built once in contiguous arenas
  and never mutated afterwards; a `Timeline` can be queried from any
  number of goroutines.
- **Lazy queries.** `Hits`, `Casts`, `Stacks` and `Events` compose
  predicates; nothing is copied until `All`, `Map`, `GroupBy` or `PerAgent`
  is called, and point lookups never allocate. `Skip`, `Limit` and `Reverse`
  describe the traversal and apply after every filter of the chain,
  wherever they appear: `Limit(3).Where(p)` yields at most three elements
  accepted by `p`. `Reverse` flips the traversal, so `Reverse().Limit(3)`
  keeps the three latest elements.
- **Missing values.** Point helpers (`PositionAt`, `HealthAt`, `FacingAt`,
  `BarrierAt`, `MaxHealthAt`, `DefiancePercentAt`) return the zero value
  when nothing is known at `t`: outside the lifetime or before the first
  sample. State helpers (`TeamAt`, `IsInCombatAt`, `IsGlidingAt`,
  `IsNameVisibleAt`...) follow the same rule, their spans end with the
  lifetime of the agent; `Life` alone keeps its final state, dead or
  gone, to the end of the log. The two-value forms are `Position.At(t)`
  and friends. `DistanceTo` returns `NaN` instead, because 0 is a real
  distance. `DefianceStateAt` returns `DefianceNone`. Times of events use
  `(time.Duration, bool)`, because 0 is a valid instant.
- **Movement gaps.** arcdps only samples an agent that moves. Across a gap
  wider than `MoveGap` the last position is held: the agent stood still.
- **Entities.** Every filter taking an agent accepts a `*Agent`, a
  `*Player` or a `*Target`; a nil one matches nothing.
- **Damage.** `Hit.Damage` is the damage as arcdps logs it, health and
  barrier parts combined; `Barrier` is the part absorbed by barrier and
  `HealthDamage()` the rest. Sums on queries follow the same names.
  `Foes` and `Friends` keep the hits by their friend or foe flag.
- **Causes.** `Down.Cause` and `Death.Cause` are the downing or killing
  hit closest to the state change, within `CauseWindow` (150 ms) on either
  side; `DownsOf(skill)` and `DownsBy(entity)` compare that single hit.
- **Instance ids** are reused by successive agents, so `tl.AgentAt(id, t)`
  and `tl.TargetBySpeciesIDAt(id, t)` resolve them against lifetimes with
  `InstanceTolerance` (300 ms) of slack.
- **Minions.** `Agent.Hits()` holds the hits of the agent itself;
  `HitsCredited()` adds those of its pets, clones, turrets and mechs, which
  is what a damage meter shows. `PerAgent` credits minions the same way.
- **Rankings.** `PerAgent`, `PerTarget` and `PerSkill` on `Hits`, and
  `PerSkill` on `Casts`, return shares sorted by damage or by number of
  casts, each holding a query of its own. `Breakbar.CCHits()` keeps the
  crowd control hits and drops the regeneration ticks of the bar.
- **Well-known buffs.** `BuffMight`, `BuffQuickness`, `BuffBurning` and the
  other constants of `buffs.go` name the boons, conditions and common
  effects by their stable GW2 API id; `Boons` and `Conditions` list them.
- **Session and squad.** `Timeline` carries the language, game build,
  shard, fractal scale, ruleset, instance start, arcdps build and whether
  the log ended by a map exit; `Commander()` is the player who wore the
  commander tag last; `GUID(kind, id)` maps a content id to its GUID and
  `Skill.GUID`, `Effect.GUID` and `Marker.GUID` are filled from it.
- **Markers.** `Agent.Markers` holds the markers an agent wore, each from
  its first application to the removal of the markers of the agent or to
  the end of its lifetime (arcdps may write a worn marker again: it is the
  same marker): a commander tag (`Commander`, `Tag`, `Catmander`), a
  squad marker (`Squad`) or the marker of an encounter mechanic, told
  apart by GUID since marker ids change with game builds.
  `Timeline.GroundMarkers` holds the placements of squad markers on the
  ground, each until the marker is removed or moved. `CommanderAt`,
  `IsCommanderAt`, `SquadMarkerAt` and `GroundMarkerAt` answer the point
  questions; the `Marker*`, `CommanderTag*` and `CatmanderTag*` variables
  name the GUIDs and `ParseGUID` reads one.
- **Effects and missiles.** An `Effect` is an instance of a visual effect
  with its trackable id; most effects arcdps logs carry no id, are never
  removed and last their announced or default duration, or are
  instantaneous. A `Missile` runs from its creation to its removal with
  every launch in between. Both keep the raw client data the arcdps
  README does not explain (`Launch.Motion`, `Launch.Flags`,
  `Effect.Flags`).
- **Raw where arcdps is unclear.** `Agent.Stealth` keeps the logged
  state byte (build 20260816 writes 1 for every player at spawn),
  `SkillTiming.Kind` and `BuffFormula` keep their floats, and `WeaponSet`
  keeps the set ids of the game (4 and 5 for the two land sets).
- **Durations.** `CombatTime`, `AliveTime`, `DownTime` and `DiedAt` on an
  agent, `Uptime` and `Average` on stacks, answer the usual time questions
  without touching the spans. On a `Numbers` series, `MinBetween` and
  `MaxBetween` include the value held at the start of the interval, and
  `TimeBelow` and `TimeAbove` integrate the step function.
- **Casts.** `Completed()` means the skill went off: the animation reached
  its first trigger point (`Minimum`, `NoData`) or played to its end
  (`Reset`). `Full()` is the latter alone and is rare, since the next skill
  usually cuts the animation short. `Cancelled()` means the skill did not
  go off.
- **Stacks.** `CountAt` and `Average` count every application present,
  queued ones included: a boon such as quickness queues its durations and
  might can be applied beyond its cap. `Buff.Stacking` tells how a buff
  stacks, `EffectiveAt` and `EffectiveAverage` count what the game applies:
  the stacks capped at `StackLimit` for intensity stacking, one otherwise.
  `Buff.IsBoon` and `IsCondition` read the arcdps category;
  `Stacks.RemovedBy`, `PerBuff`, `PerReceiver` and `PerApplier` complete
  the filters and rankings. A stack ends with its removal event, with the
  reuse of its id before any removal (`Superseded`) or with the despawn of
  its receiver (`EndedByDespawn`): minions and NPCs leave tracking with
  their buffs and no removal is logged for them.

## What is read from the log

The builder follows the field layout of the arcdps `README.txt`
(`enum cbtstatechange`). Everything else is kept untouched in `Events()`.

| arcdps state change                                                                                 | fields used                                                                                                                                                                                                                                                                                        | graph                                                                                                                                                                                        |
| --------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `CBTS_COMBAT`                                                                                       | `src_agent`, `dst_agent`, `value` (strike damage) or `buff_dmg` (buff tick), `overstack_value` (barrier part when `is_shields`), `skillid`, `result`, `iff`, `buff`, `is_ninety`, `is_fifty`, `is_moving` (bit 0 source, bit 1 target), `is_flanking`, `is_shields`, `is_offcycle` (target downed) | `Hit`                                                                                                                                                                                        |
| `ENTERCOMBAT`, `EXITCOMBAT`                                                                         | `src_agent`                                                                                                                                                                                                                                                                                        | `Agent.InCombat`                                                                                                                                                                             |
| `CHANGEUP`, `CHANGEDOWN`, `CHANGEDEAD`, `SPAWN`, `DESPAWN`                                          | `src_agent`                                                                                                                                                                                                                                                                                        | `Agent.Life`, `Downs`, `Deaths`                                                                                                                                                              |
| `HEALTHPCTUPDATE`, `BARRIERPCTUPDATE`                                                               | `dst_agent` = percent × 10000                                                                                                                                                                                                                                                                      | `Agent.Health`, `Agent.Barrier` (0 to 100)                                                                                                                                                   |
| `MAXHEALTHUPDATE`                                                                                   | `dst_agent`                                                                                                                                                                                                                                                                                        | `Agent.MaxHealth`                                                                                                                                                                            |
| `SQCOMBATSTART`, `SQCOMBATEND`                                                                      | `value` server time, `buff_dmg` local time, event time                                                                                                                                                                                                                                             | time origin, `Timeline.Start`, `LocalStart`, `Duration`                                                                                                                                      |
| `POINTOFVIEW`, `MAPID`, `LOGNPCUPDATE`                                                              | `src_agent`; `dst_agent` for the boss                                                                                                                                                                                                                                                              | `Timeline.POV`, `MapID`, `Target.Boss`                                                                                                                                                       |
| `POSITION`, `VELOCITY`, `TELEPORT`                                                                  | `dst_agent` as float[2] plus `value` as float                                                                                                                                                                                                                                                      | `Agent.Position`, `Velocity` (a teleport breaks interpolation)                                                                                                                               |
| `FACING`                                                                                            | `dst_agent` as float[2]                                                                                                                                                                                                                                                                            | `Agent.Facing`                                                                                                                                                                               |
| `ATTACKTARGET`                                                                                      | `src_agent` attack target, `dst_agent` gadget                                                                                                                                                                                                                                                      | `Agent.Gadget`, `AttackTargets`                                                                                                                                                              |
| `TARGETABLE`                                                                                        | `dst_agent` 0, 1 or 2 (unsupported, read as false)                                                                                                                                                                                                                                                 | `Agent.Targetable`                                                                                                                                                                           |
| `DEFIANCEBARSTATE`, `DEFIANCEBARPERCENT`                                                            | `dst_agent` (or `value` on builds that write it there), `value` as float                                                                                                                                                                                                                           | `Agent.Defiance`, `DefiancePercent`, `Breakbars`                                                                                                                                             |
| `IIDCHANGE`                                                                                         | `src_agent` old id, `dst_agent` new id                                                                                                                                                                                                                                                             | `Timeline.Agent` follows the change                                                                                                                                                          |
| `BUFFINFO`                                                                                          | `overstack_value`, `src_master_instid`, `is_offcycle`, `pad61`, `is_flanking`, `is_shields`, `pad62`                                                                                                                                                                                               | `Buff`                                                                                                                                                                                       |
| `ANIMATIONSTART`, `ANIMATIONSTOP`                                                                   | `src_agent`, `dst_agent` target, `value`, `buff_dmg`, `skillid`, `is_activation`                                                                                                                                                                                                                   | `Cast`                                                                                                                                                                                       |
| `BUFFAPPLY`, `BUFFINITIAL`, `BUFFCHANGE`, `BUFFREMOVE_SINGLE`, `BUFFREMOVE_ALL`                     | `src_agent`, `dst_agent`, `value`, `buff_dmg` (original duration), `is_shields` (active on apply), `is_buffremove`, `pad61` to `pad64` as the trackable id                                                                                                                                         | `BuffStack`; a remove-all closes the stacks its single removes left open (it summarizes them: `value` sums their durations and `result` counts them)                                         |
| `DESPAWN` with `src_agent` 0                                                                        | `src_instid` names the agent, as arcdps writes it for minions and NPCs                                                                                                                                                                                                                             | `Agent.Life` (gone), the stacks of the agent end there                                                                                                                                       |
| `BUFFACTIVE`, `BUFFDEACTIVE`                                                                        | `dst_agent` (active) or `pad61` (deactive) as the trackable id                                                                                                                                                                                                                                     | `BuffStack.Active`                                                                                                                                                                           |
| `SKILLINFO`, `SKILLTIMING`, `BUFFFORMULA`                                                           | `time` as float[4] (cost, ranges, tooltip seconds); `src_agent` kind and `dst_agent` time; `time` as float[9] and `src_instid` as float[2]                                                                                                                                                         | `Skill.Cost`, `Timings`, `Buff.Formulas`                                                                                                                                                     |
| `IDTOGUID`                                                                                          | `src_agent` as 16 bytes, `overstack_value` kind, `skillid` id, `buff_dmg` as float default duration                                                                                                                                                                                                | `Timeline.GUID`, `Skill.GUID`, effect defaults                                                                                                                                               |
| `LANGUAGE`, `GWBUILD`, `SHARDID`, `FRACTALSCALE`, `RULESET`, `INSTANCESTART`, `ARCBUILD`            | `src_agent` (a timestamp of the event clock for the instance start, a string for the arcdps build)                                                                                                                                                                                                 | `Timeline` session fields                                                                                                                                                                    |
| `SQCOMBATEND`                                                                                       | `dst_agent` bit 0                                                                                                                                                                                                                                                                                  | `Timeline.EndedByMapExit`                                                                                                                                                                    |
| `MARKER`, `SQUADMARKER_GROUND`                                                                      | `value` id (0 removes every marker of the agent) and `buff` commander flag; `src_agent` as float[3] (zero or infinite to remove) and `skillid` index                                                                                                                                               | `Agent.Markers`, `Timeline.GroundMarkers`, each from its application or placement to its removal; a removal with nothing to remove, or a marker written again while worn, is not attached    |
| `GUILD`, `TEAMCHANGE`, `WEAPSWAP`, `STEALTHCHANGE`, `GLIDER`, `TRANSFORMATION`, `STUNBREAK`, `TICK` | `dst_agent` as 16 bytes; `dst_agent` new and `value` old; `dst_agent` state; `value`; `skillid` and `value`; `value`; `value` ping                                                                                                                                                                 | `Player.Guild`, `Agent.Team`, `WeaponSet`, `Stealth`, `Gliding`, `Transformation`, `StunBreaks`, `Timeline.Ping`                                                                             |
| `EFFECTGROUNDCREATE`, `EFFECTAGENTCREATE`, `EFFECTGROUNDREMOVE`, `EFFECTAGENTREMOVE`                | `dst_agent` as int16[6] (origin over ten, orientation times a thousand), `iff` as uint32 duration, `is_buffremove` flags, `is_flanking`, `is_shields` as int16 scale, `pad61` id                                                                                                                   | `Effect`                                                                                                                                                                                     |
| `MISSILECREATE`, `MISSILELAUNCH`, `MISSILEREMOVE`, `MISSILEEFFECT`                                  | `value` as int16[3] or int16[6] coordinates over ten, `overstack_value` skin, `dst_agent` target or owner, `iff` motion, `result` radius, `is_buffremove` flags, `is_flanking`, `is_shields` speed, `pad61` id                                                                                     | `Missile`, `Launch`, `MissileEffect`                                                                                                                                                         |
| `JUMP`, `GADGETNAME`, `GADGETANIMATION`                                                             | `dst_agent` 1 leaving the ground or 0 landing; `dst_agent` 0, 1 or 2 (unsupported, read as hidden); `dst_agent` token                                                                                                                                                                              | `Agent.Airborne`, `NameVisible`, `GadgetAnimations`                                                                                                                                          |
| `REWARD`, `MAPCHANGE`, `INTEGRITY`                                                                  | `dst_agent` id and `value` type; `src_agent` new map, `dst_agent` old map and `value` type; `time` as char[32]                                                                                                                                                                                     | `Timeline.Rewards`, `MapChanges`, `Integrity`                                                                                                                                                |
| `EXTENSION`, `EXTENSIONCOMBAT`                                                                      | `src_agent` low 32 bits as the signature and `dst_agent` as text; `pad61` to `pad64` as the signature of the writer, `skillid` as a skill (arcdps adds it to the skill table), the rest as logged                                                                                                  | `Timeline.Extensions`, `Extension.Events()`, `Timeline.ExtensionEvents()`, `Skill`; the events go to the `ExtensionDecoder` registered for the signature, whose graph is `Extension.Decoded` |

## Not modeled

Capture points (`GADGETCAPTURE*`), WvW objectives and the retired
`RATEHEALTH` are only available as raw events. The graph does not guess
which player a boss is chasing: the log carries no aggro information.
