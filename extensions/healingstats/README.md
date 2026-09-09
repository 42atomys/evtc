# healingstats

`healingstats` decodes the combat events written by the
[healing stats addon](https://github.com/Krappa322/arcdps_healing_stats) of
arcdps into a graph on top of the timeline of a log: every heal and barrier
the addon logged, linked to the agents, skills and casts of the timeline.

arcdps itself does not log healing. The addon does, as extension combat
events, and only for the clients that run it: the recording player and the
squad members sharing their live stats with it. The timeline knows nothing
of the addon beyond a generic decoder hook.

```go
import (
	"github.com/42atomys/evtc/extensions/healingstats"
	"github.com/42atomys/evtc/timeline"
)

tl, err := timeline.ParseFile("fight.zevtc")
h := healingstats.Of(tl) // nil when the log has no healing stats
for _, c := range h.Heals().Healing().PerAgent() {
	fmt.Println(c.Agent.Name, c.Heals.Healed())
}
```

Importing the package registers its decoder with `timeline`
(`timeline.RegisterExtension`), so every `Timeline` built afterwards
carries the decoded stats in `Extension.Decoded`; `Of` returns them typed.
Nothing else changes for a program that does not import it.

## The model

| Node                               | Reached from        | Points to                                                                                                                          |
| ---------------------------------- | ------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `Stats`                            | `Of(tl)`            | `Heals()`, `Agents`, `Players`, `Unknown`, `Recorded`, `Agent(entity)`, `Timeline`, `Extension`, `Version`, `Revision`, `Merged`   |
| `Agent` (embeds `*timeline.Agent`) | `Stats`, every heal | `Heals()`, `HealsCredited()`, `HealsTaken()`, `Recorded`, `Stats`, and everything a timeline agent has                             |
| `Heal`                             | `Heals` queries     | `Src`, `Dst`, `Skill`, `Cast`, `Amount`, `IsBarrier`, `IsBuff`, `TargetDowned`, `SrcRecorded`, `DstRecorded`, `Event`, `PeerEvent` |

An `Agent` node exists for every agent of the timeline, so `h.Agent(p)` is
never nil for a player, target or agent of `tl`. The node embeds the
timeline agent: `node.Name`, `node.PositionAt(t)`, `node.Hits()` all work,
and the node is accepted wherever the timeline takes an `Entity`
(`tl.Hits().By(node)`).

`Heals` is a query with the lazy semantics of `timeline.Query`: filters
compose a predicate, `Skip`, `Limit` and `Reverse` describe the traversal,
and nothing is copied until a terminal is called.

- Filters: `By`, `CreditedTo`, `On`, `OfSkill`, `Of`, `OfCast`, `Healing`,
  `Barrier`, `Direct`, `Ticks`, `Downed`, `Self`, `Others`, `Between`,
  `Where`.
- Sums: `Amount` (health and barrier alike), `Healed`, `BarrierGiven`,
  `HPS(iv)` and `BPS(iv)` per second of an interval.
- Rankings: `PerAgent` (credited, minions included), `PerTarget` and
  `PerSkill`, sorted by decreasing amount; `GroupBy` for anything else.

## Conventions

- **Amounts.** `Heal.Amount` is the health restored or the barrier given,
  as the addon logs it; `IsBarrier` tells which, `Healed()` and
  `BarrierGiven()` read one or the other. `IsBuff` is set for the tick of
  a buff such as regeneration, `Direct()` and `Ticks()` keep one kind.
- **Who is complete.** Only the clients running the addon write events,
  each for the heals it dealt or received. `Stats.Recorded` lists those
  players and `Agent.Recorded` marks them and their minions: their heals
  are all in the log. Of everyone else, only the heals exchanged with a
  recorded agent are known, so their totals are lower bounds.
- **Two records of one heal.** A heal between two recorded players is
  written by both clients. The decoder pairs the two records (same
  instance ids, skill, amount and kind, opposite recording flags, within
  `PeerWindow`) into one `Heal` with both `SrcRecorded` and `DstRecorded`
  set; `Event` is the record of the recording player's own client and
  `PeerEvent` the other one. `Stats.Merged` counts them. Identical heals
  written by one client in the same instant (one heal per boon granted,
  for example) are distinct heals and stay so.
- **Credit.** `Heal.Credited()` is the master of a minion source, as for
  hits; `HealsCredited` and `PerAgent` follow that rule.
- **Casts.** `Heal.Cast` is the most recent cast of the same skill by the
  same agent started before the heal, as the timeline attributes hits;
  `OfCast` keeps the heals of one cast.
- **Downed targets.** `TargetDowned` reads the flag the addon sets on buff
  ticks and the arcdps flag it keeps on direct heals; both agree with the
  down states of the timeline.
- **Sources.** The addon rewrites the addresses of the records a squad
  member shares through its own agent table, which misses some agents
  (minions, allied NPCs): it writes 0 or an address the log never
  declares. The decoder then resolves the agent through the instance id
  at the time of the heal, as the timeline does for despawns. A source
  that stays unknown is `Stats.Unknown`, the node of the Unknown sentinel
  of the timeline. The skill of every heal is a `Skill` of the timeline,
  which knows the ids of the extension events as arcdps adds them to its
  skill table.
- **Nil is empty.** `Of` returns nil for a log without the addon; the
  methods of `Stats` and `Agent` accept a nil receiver and answer as for
  an empty log. Fields do not: check `h != nil` before reading them.
- **Read-only, concurrent.** Like the timeline it is built on.

## Cookbook

Every snippet below is an `Example` test of the package
(`example_readme_test.go`), run on a five second synthetic fight recorded
by Alpha with the addon, Bravo sharing its stats with it, Charlie without
it. The outputs are checked by `go test`.

### Open the stats

```go
tl, err := timeline.Build(cookbookLog()) // timeline.ParseFile("fight.zevtc") on a real file
if err != nil {
	panic(err)
}
h := healingstats.Of(tl) // nil when the log has no healing stats
fmt.Println("addon", h.Version, "format revision", h.Revision, "|", h.Heals().Count(), "heals,", h.Merged, "written by both clients")
for _, p := range h.Recorded {
	fmt.Println("recorded by", p.Name, "with", p.Heals().Count(), "heals dealt and", p.HealsTaken().Count(), "received")
}
// addon 2.19rc2 format revision 2 | 7 heals, 3 written by both clients
// recorded by Alpha with 4 heals dealt and 4 received
// recorded by Bravo with 2 heals dealt and 1 received
```

### Healing per player

```go
tl, h := mustBuild()
for _, c := range h.Heals().Healing().PerAgent() {
	fmt.Printf("%s: %d heals, %d healed, %.0f hps\n", c.Agent.Name, c.Heals.Count(), c.Heals.Healed(), c.Heals.HPS(tl.Interval()))
}
fmt.Println("barrier:", h.Heals().Barrier().PerAgent()[0].Agent.Name, h.Heals().BarrierGiven())
// Alpha: 4 heals, 3200 healed, 640 hps
// Bravo: 2 heals, 260 healed, 52 hps
// barrier: Charlie 1200
```

`PerAgent` credits the heals of a minion (mech, pet, clone) to its master,
like the hits of the timeline.

### What a player received

```go
tl, h := mustBuild()
a := h.Agent(tl.POV) // every timeline entity has a node
taken := a.HealsTaken()
fmt.Println(a.Name, "received", taken.Healed(), "healing and", taken.BarrierGiven(), "barrier over", taken.Count(), "heals")
fmt.Println("from others:", taken.Others().Count(), "| self:", taken.Self().Amount(), "| regeneration ticks:", taken.Ticks().OfSkill(regen).Count())
fmt.Println("hits dealt by the same node:", tl.Hits().By(a).Count())
// Alpha received 1160 healing and 1200 barrier over 4 heals
// from others: 3 | self: 900 | regeneration ticks: 2
// hits dealt by the same node: 0
```

### Heals written by both clients

```go
tl, h := mustBuild()
heal := h.Heals().On(tl.PlayerByName("Bravo")).First()
fmt.Println(heal.Src.Name, "->", heal.Dst.Name, heal.Skill.Name, heal.Amount, "at", heal.Time)
fmt.Println("written by the client of the source:", heal.SrcRecorded, "| of the destination:", heal.DstRecorded, "| second record at", tl.TimeOf(heal.PeerEvent))
single := h.Heals().On(tl.PlayerByName("Charlie")).First()
fmt.Println("Charlie runs no addon:", single.SrcRecorded, single.DstRecorded, single.PeerEvent == nil)
// Alpha -> Bravo Shelter 900 at 1.2s
// written by the client of the source: true | of the destination: true | second record at 1.26s
// Charlie runs no addon: true false true
```

### From a cast to its heals

```go
tl, h := mustBuild()
cast := h.Agent(tl.POV).Casts().OfSkill(shelter).First()
heals := h.Heals().OfCast(cast)
fmt.Println(cast.Skill.Name, cast.Interval, "healed", heals.Amount(), "over", heals.Count(), "heals; first on", heals.First().Dst.Name)
// Shelter [1s, 1.5s] healed 3200 over 4 heals; first on Bravo
```

### Healing on downed players

```go
tl, h := mustBuild()
for heal := range h.Heals().Downed().Seq() {
	fmt.Println(heal.Dst.Name, "was", heal.Dst.LifeStateAt(heal.Time), "when healed for", heal.Amount, "by", heal.Src.Name, "at", heal.Time)
}
fmt.Println("healing received while down:", h.Agent(tl.PlayerByName("Charlie")).HealsTaken().Downed().Healed())
// Charlie was Down when healed for 500 by Alpha at 4s
// healing received while down: 500
```

### Windows and rankings

```go
tl, h := mustBuild()
second := timeline.NewInterval(2*time.Second, 3*time.Second)
fmt.Println("heals between 2s and 3s:", h.Heals().Between(second).Count(), "| healing:", h.Heals().Between(second).Healed(), "| barrier:", h.Heals().Between(second).BarrierGiven())
fmt.Println("latest heal:", h.Heals().Reverse().First().Skill.Name, "| skills:", len(h.Heals().PerSkill()), "| log:", tl.Duration)
// heals between 2s and 3s: 3 | healing: 260 | barrier: 1200
// latest heal: Shelter | skills: 3 | log: 5s
```

## What is read from the log

The addon registers itself at the squad combat start with an
`EXTENSION` event and writes one `EXTENSIONCOMBAT` event per heal it sees,
through the `e10` export of arcdps, which sets the signature `0x9c9b3c99`
in `pad61` to `pad64` and adds the skill to the skill table. Measured on
117 logs of the addon versions 2.17 to 2.19 (format revision 2).

| Event                                                                                                                   | Fields                                                                                                                                                                                                           | Graph                                                                              |
| ----------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Registration (`EXTENSION`)                                                                                              | `src_agent`: signature in the low 32 bits, format revision in the next 24, length of the version string in the top 8; `dst_agent`: the version string                                                            | `Stats.Revision`, `Version`, `Extension`                                           |
| Heal (`EXTENSIONCOMBAT`, `buff` 0)                                                                                      | `value`: the amount healed, negated                                                                                                                                                                              | `Heal` with `Amount`, `IsBuff` false                                               |
| Tick of a buff (`buff` 1)                                                                                               | `buff_dmg`: the amount healed, negated; `value` 0                                                                                                                                                                | `Heal` with `IsBuff`                                                               |
| Barrier (`is_shields` 1)                                                                                                | the amount in `value` or `buff_dmg` as above, repeated in `overstack_value`                                                                                                                                      | `Heal` with `IsBarrier`                                                            |
| Every heal                                                                                                              | `src_agent`, `dst_agent`, `src_instid`, `dst_instid`, masters, `skillid`, `iff`, `is_ninety`, `is_fifty`, `is_moving` as arcdps writes them                                                                      | `Src`, `Dst`, `Skill`, `IFF`, `OverNinety`, `UnderFifty`, `Moving`, `TargetMoving` |
| Every heal                                                                                                              | `is_offcycle`: bit 7 the client of the source (or of its master) wrote the event, bit 6 the client of the destination did, bit 5 the target of a buff tick was downed, bit 0 the arcdps flag for a downed target | `SrcRecorded`, `DstRecorded`, `TargetDowned`, `Agent.Recorded`, `Stats.Recorded`   |
| Two records with opposite recording flags, the same instance ids, skill, amounts and kind, within `PeerWindow` (500 ms) |                                                                                                                                                                                                                  | one `Heal` with `PeerEvent`, counted in `Stats.Merged`                             |

`result`, `is_activation` and `is_buffremove` are always zero;
`is_flanking` carries no flag on these events and is not read. Elite
Insights reads the same events and, for the heals written twice, keeps
the records of one client per source, destination and skill instead of
pairing them.

