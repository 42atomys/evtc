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

## Cookbook

Every snippet below is an `Example` test of the package
(`example_readme_test.go`), run on a five second synthetic fight: two
players, Alpha and Bravo, against Sabetha. The outputs are checked by
`go test`.

### Open a log

```go
log := cookbookLog() // evtc.ParseFile("fight.zevtc") on a real file
tl, err := timeline.Build(log)
if err != nil {
	panic(err) // timeline.ErrLegacyLog for logs older than arcdps 20260501
}
fmt.Println(tl.Boss().Name, "fought by", len(tl.Players), "players for", tl.Duration)
fmt.Println("arcdps build", tl.Build, "events", tl.Events().Count(), "hits", tl.Hits().Count())
// Sabetha fought by 2 players for 5s
// arcdps build 20260816 events 50 hits 8
```

### Players and lookups

```go
for _, p := range tl.Players {
	fmt.Println(p.Name, p.Account, "group", p.Subgroup, p.Spec())
}
fmt.Println(tl.PlayerByAccount("Bravo.5678").Toughness, tl.PlayerByName("Nobody") == nil)
// Alpha Alpha.1234 group 1 Firebrand
// Bravo Bravo.5678 group 2 Warrior
// 1500 true
```

```go
fmt.Println(tl.AgentAt(instBoss, time.Second), tl.TargetBySpeciesIDAt(15375, time.Second).Boss)
fmt.Println(tl.Agent(alpha).Player.Spec(), tl.Agent(0xdead) == nil)
// Sabetha(NPC#15375) true
// Firebrand true
```

### Damage per player

```go
landed := tl.Boss().HitsTaken().Landed()
fmt.Println("total", landed.Damage(), "damage,", landed.Count(), "landed hits")
for _, c := range landed.PerAgent() {
	fmt.Printf("%s %d (%.0f%%)\n", c.Agent.Name, c.Hits.Damage(), 100*float64(c.Hits.Damage())/float64(landed.Damage()))
}
// total 1500 damage, 3 landed hits
// Alpha 1000 (67%)
// Bravo 500 (33%)
```

`PerAgent` credits the hits of a minion (pet, clone, turret) to its master.
From the player's side, `p.HitsCredited().On(boss)` holds the same hits.

### Rankings

```go
for _, s := range tl.Boss().HitsTaken().Landed().PerSkill() {
	fmt.Println(s.Skill.Name, s.Hits.Count(), "hits,", s.Hits.Damage(), "damage")
}
for _, c := range tl.Hits().Landed().PerTarget() {
	fmt.Println(c.Agent.Name, "took", c.Hits.Damage())
}
for _, c := range tl.Casts().PerSkill() {
	fmt.Println(c.Skill.Name, "cast", c.Casts.Count(), "time by", c.Casts.First().Caster.Name)
}
// Slam 3 hits, 1500 damage
// Sabetha took 1500
// Alpha took 100
// Slam cast 1 time by Alpha
```

### Phases by health and DPS

```go
boss := tl.Boss()
for i, phase := range boss.PhasesByHealth(66.6, 33.3) {
	fmt.Printf("phase %d %v: %.0f dps\n", i+1, phase, boss.HitsTaken().Landed().DPS(phase))
}
at, _ := boss.HealthBelow(33.3)
fmt.Println("below 33.3% at", at, "health at 2s:", boss.HealthAt(2*time.Second))
// phase 1 [0s, 1.8s]: 833 dps
// phase 2 [1.8s, 4s]: 0 dps
// phase 3 [4s, 4.5s]: 0 dps
// below 33.3% at 4s health at 2s: 60
```

`PhasesByBuff(id)` removes the spans where the agent carried a buff, for
bosses that phase through invulnerability.

### From a cast to its hits

```go
alpha, boss := tl.Players[0], tl.Boss()
cast := alpha.Casts().OfSkill(slam).First()
fmt.Println(cast.Skill, cast.Interval, "completed:", cast.Completed())
fmt.Println("hits:", cast.Hits().Count(), "crit:", cast.Hits().Crits().Count(), "damage:", cast.Hits().Damage())
fmt.Println("blocked by Alpha:", boss.Hits().On(alpha).Blocked().Any())
// Slam (100) [1s, 1.5s] completed: true
// hits: 2 crit: 1 damage: 1000
// blocked by Alpha: true
```

A hit belongs to the most recent cast of the same skill by the same agent,
so projectiles landing after the animation are still attributed.

### Positions and distances

```go
alpha, boss := tl.Players[0], tl.Boss()
at := time.Second
fmt.Println(alpha.PositionAt(at), boss.PositionAt(at))
fmt.Printf("%.0f units apart\n", alpha.DistanceTo(boss, at))
fmt.Println("unknown:", math.IsNaN(alpha.DistanceTo(boss, 10*time.Second)))
// {150 0 0} {300 400 0}
// 427 units apart
// unknown: true
```

### Buff stacks and uptime

```go
alpha := tl.Players[0]
stacks := alpha.Stacks().OfBuff(timeline.BuffMight)
fmt.Println("might stacks:", stacks.Count(), "at 2s:", stacks.CountAt(2*time.Second), "at 3s:", stacks.CountAt(3*time.Second))
fmt.Println("uptime:", stacks.Uptime(tl.Interval()), "average:", stacks.Average(tl.Interval()), "applied by", stacks.First().Applier.Name)
// might stacks: 1 at 2s: 1 at 3s: 0
// uptime: 1.5s average: 0.3 applied by Bravo
```

### Downs, deaths and life state

```go
bravo := tl.Players[1]
down := bravo.Downs[0]
fmt.Println(bravo.Name, "down", down.Interval, "by", down.Cause.Skill.Name, "from", down.Cause.Src.Name, "recovered:", down.Recovered)
fmt.Println(bravo.IsDownAt(2500*time.Millisecond), bravo.DownedBetween(tl.Since(4*time.Second)), len(bravo.DownsOf(tl.Skill(flak))), len(bravo.DownsBy(tl.Boss())))
fmt.Println("life at 2.5s:", bravo.LifeStateAt(2500*time.Millisecond), "died:", bravo.DiedBefore(tl.Duration))
// Bravo down [2.2s, 3s] by Flak Shot from Sabetha recovered: true
// true false 1 1
// life at 2.5s: Down died: false
```

```go
bravo := tl.Players[1]
died, _ := bravo.DiedAt()
fmt.Println("alive", bravo.AliveTime(tl.Interval()), "down", bravo.DownTime(tl.Interval()), "died:", died)
fmt.Println("in combat", tl.Players[0].CombatTime(tl.Interval()))
// alive 3.2s down 800ms died: 0s
// in combat 0s
```

```go
boss := tl.Boss()
lowest, _ := boss.Health.MinBetween(tl.Until(2 * time.Second))
fmt.Println("lowest health in the first two seconds:", lowest)
fmt.Println("time under 66.6%:", boss.Health.TimeBelow(66.6, tl.Interval()), "of", boss.Lifetime.Duration())
// lowest health in the first two seconds: 60
// time under 66.6%: 2.7s of 4.5s
```

Bravo is only tracked from its first event at 1 s, so its life state starts
there.

### Breakbars and crowd control

```go
bb := tl.Boss().Breakbars[0]
fmt.Println(bb.Interval, "broken:", bb.Broken(), "cc:", bb.TotalCC())
for _, c := range bb.Hits().PerAgent() {
	fmt.Println(c.Agent.Name, c.Hits.Damage(), "cc at", c.Hits.First().Time)
}
// [3s, 3.5s] broken: true cc: 500
// Bravo 300 cc at 3.4s
// Alpha 200 cc at 3.2s
```

### The latest hits

```go
for _, h := range tl.Hits().Landed().Reverse().Limit(2).All() {
	fmt.Println(h.Time, h.Src.Name, "->", h.Dst.Name, h.Damage)
}
fmt.Println("second landed hit:", tl.Hits().Landed().Skip(1).First().Damage)
// 4.5s Sabetha -> Alpha 100
// 1.6s Bravo -> Sabetha 500
// second landed hit: 300
```

### Raw events

```go
bravo := tl.Players[1]
e := tl.Events().Involving(bravo).Of(evtc.StateChangeDown).First()
fmt.Println(tl.TimeOf(e), e.IsStateChange, "src", e.SrcAgent == bravo.Addr)
fmt.Println(bravo.Events().Count(), "events involve", bravo.Name)
// 2.2s ChangeDown src true
// 9 events involve Bravo
```

Every node keeps a pointer to the raw `evtc.Event` it came from, so what
the graph does not model is still reachable through `Event`, and
`evtc.Event.Bytes()` gives the 64-byte wire layout when a payload spans
several fields.

### Session, squad and weapon sets

```go
fmt.Println("commander:", tl.Commander().Name, "| language:", tl.Language, "| game build:", tl.GameBuild)
alpha := tl.Players[0]
fmt.Println("weapon swaps:", alpha.WeaponSet.Len()-1, "| set at 3s:", alpha.WeaponSetAt(3*time.Second))
// commander: Alpha | language: French | game build: 205780
// weapon swaps: 1 | set at 3s: 1
```

### Squad markers and the commander tag

```go
bravo := tl.Players[1]
m := bravo.Markers[0]
fmt.Println(m.Squad, "on", bravo.Name, m.Interval, "removed:", m.Removed(), "| at 2s:", bravo.SquadMarkerAt(2*time.Second), "| at 4s:", bravo.SquadMarkerAt(4*time.Second))
heart := tl.GroundMarkerAt(timeline.SquadHeart, 2500*time.Millisecond)
fmt.Println("heart on the ground at", heart.Position, heart.Interval, "| placements:", len(tl.GroundMarkers))
fmt.Println("commander at 1s:", tl.CommanderAt(time.Second).Name, "| tag:", tl.Commander().Markers[0].Tag)
// Heart on Bravo [1s, 3s] removed: true | at 2s: Heart | at 4s: None
// heart on the ground at {100 100 0} [2s, 4s] | placements: 2
// commander at 1s: Alpha | tag: Red
```

A marker on an agent has no position of its own: `agent.PositionAt(t)` is
where it was drawn. Most markers of a raid log are put by the game on NPCs
for a mechanic (the aspects of Dhuum, the lamps of Qadim); their GUIDs are
not named by the package, so compare `Marker.GUID` with the one you parsed
with `ParseGUID`.

### Effects

```go
f := tl.Boss().Effects().Ground().First()
fmt.Println(f.EffectID, "at", f.Origin, "for", f.Duration, f.Interval, "removed:", f.Removed())
fmt.Println("effects present at 2s:", tl.Effects().At(2*time.Second).Count())
// 7000 at {300 400 0} for 2s [1.9s, 2.4s] removed: true
// effects present at 2s: 1
```

### Missiles

```go
m := tl.Players[0].Missiles().First()
fmt.Println(m.Skill.Name, "from", m.Origin, m.Interval, "aimed at", m.Target().Name, "| hit:", m.HitEnemy)
fmt.Println(len(m.Launches), "launch at", m.Launches[0].Time, "towards", m.Launches[0].TargetPos)
// Slam from {180 0 0} [1.1s, 1.2s] aimed at Sabetha | hit: true
// 1 launch at 1.15s towards {300 400 0}
```

### Gadgets, jumps and rewards

```go
boss := tl.Boss()
fmt.Println("animations:", len(boss.GadgetAnimations), "| name shown at 1s:", boss.IsNameVisibleAt(time.Second), "| at 4s:", boss.IsNameVisibleAt(4*time.Second))
fmt.Println("alpha airborne at 1.2s:", tl.Players[0].IsAirborneAt(1200*time.Millisecond), "| rewards:", len(tl.Rewards))
// animations: 1 | name shown at 1s: true | at 4s: false
// alpha airborne at 1.2s: true | rewards: 1
```

The Sabetha sample log, recorded in a raid instance, carries gadget
animations and name states but no jump, reward, map change or integrity
event; those kinds show up in open-world logs.

### Extensions

```go
x := tl.Extension(timeline.ExtensionHealingStats)
fmt.Printf("extension %#x version %s wrote %d of the %d extension events\n", x.Signature, x.Version, x.Events().Count(), tl.ExtensionEvents().Count())
e := x.Events().On(tl.Players[0]).First()
fmt.Println("first on Alpha:", tl.Agent(e.SrcAgent).Name, "buff_dmg", e.BuffDamage, "at", tl.TimeOf(e), "| written by", tl.ExtensionOf(e).Version)
// extension 0x9c9b3c99 version 2.18rc1 wrote 1 of the 1 extension events
// first on Alpha: Bravo buff_dmg -262 at 1.3s | written by 2.18rc1
```

The healing stats addon is the one extension found in raid logs, and the
[`extensions/healingstats`](../extensions/healingstats/README.md) package
of this module decodes its events into heals linked to the agents, skills
and casts of the timeline: `healingstats.Of(tl)` returns them once the
package is imported. The snippet above is what the timeline offers on its
own for any extension.

### Projecting to your own types

```go
type row struct {
	At     time.Duration
	Skill  string
	Damage int32
}
rows := tl.Boss().HitsTaken().Strikes().Map(func(h *timeline.Hit) row {
	return row{h.Time, h.Skill.Name, h.Damage}
})
fmt.Printf("%+v\n", rows)
// [{At:1.2s Skill:Slam Damage:700} {At:1.4s Skill:Slam Damage:300} {At:1.6s Skill:Slam Damage:500}]
```

The graph is cyclic, so it has no JSON form of its own: project the nodes
you need with `Map` and encode that.

## Recipes

Answers that need a rule of the game or of arcdps rather than a method,
taken from `examples/sabetha`, a full report of a raid log.

**Cleanses and strips.** When a skill removes a buff from someone else,
arcdps writes one manual removal per stack naming the agent that did it;
natural expiries are single removals.

```go
cleansed := tl.Stacks().RemovedBy(player).Where(func(s *timeline.BuffStack) bool {
	return s.Receiver != player.Agent && s.Removal == evtc.BuffRemoveManual && s.Buff.IsCondition()
})
```

**Boon generation.** What a player gives depends on how the boon stacks:
stacks for might or stability, uptime for fury, quickness or alacrity.

```go
applied := player.StacksApplied().OfBuff(id).On(receiver)
if tl.Buff(id).Stacking.Intensity() {
	stacks := applied.EffectiveAverage(iv)
} else {
	uptime := applied.Uptime(iv).Seconds() / iv.Duration().Seconds()
}
```

**Destroyed gadgets.** A cannon of Sabetha starts at 0% health, is armed
at 100% and its health collapses when it is destroyed, without reaching
zero before arcdps stops updating it: every fall under 25% is one
destruction, found with `gadget.Health.Crossings(25)` and the `Falling`
direction. `FirstBelow` would answer with the start of the lifetime.

**Time alive.** A dead player sits at 0% health and keeps no buff: measure
uptimes, averages and `TimeBelow` over `tl.Until(died)` when `DiedAt`
reports a death.

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

## Performance

Building the graph costs about 190 ns and 150 bytes per event, in one
allocation per node type: the 183k events of a five minute raid log take
about 35 ms and 30 MB, effects, missiles, buff activity and duration
changes included. Point lookups (`PositionAt`, `HealthAt`,
`LifeStateAt`, `AgentAt`) run in 3 to 20 ns without allocating; a filter
allocates its closure once and a traversal allocates nothing, so a query
such as `boss.Hits().On(player).Blocked().Any()` costs a handful of
allocations however many hits the log holds.

`go test ./timeline -bench . -benchmem` prints the numbers for your
machine; the integration tests and benchmarks on a real log run when
`tests_fixtures/sabetha-05-fd9b6f3a.zevtc` is present at the repository
root and skip otherwise. `go run ./examples/sabetha` prints a full report
of that log written with the public API.

`EVTC_REAL_LOGS=1 go test ./timeline -run TestRealLogs -v` builds every
log under `tests_fixtures/`, checks the invariants, runs the API on each
and reports how much of every log the graph consumes: over 152 logs of
raids, strikes, fractals and convergences (27.9 million events), 96.9% of
the events are held by a node or read into a field, the rest being
remove-all summaries, marker removals on agents and ground positions that
wore none (96% of the marker events arcdps writes), capture points and a
few removes whose creation predates the log.

## Extensions

arcdps extensions write combat events whose layout belongs to them. The
timeline keeps those events attached to their `Extension` and decodes
nothing of them itself: each extension is handled by a package of its
own. Such a package implements `ExtensionDecoder` and registers it from
its `init` function with `RegisterExtension`: `Build` hands every
registered extension found in a log to its decoder once the core graph
is complete, and keeps the result in `Extension.Decoded`. Importing the
package is all a program does; the package offers a typed accessor for
the result.

`extensions/healingstats` decodes the healing stats addon this way; its
agent nodes embed the timeline agents, so every filter of this package
accepts them.

## Not modeled

Capture points (`GADGETCAPTURE*`), WvW objectives and the retired
`RATEHEALTH` are only available as raw events. The graph does not guess
which player a boss is chasing: the log carries no aggro information.
