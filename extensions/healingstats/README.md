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

| Node                               | Reached from        | Points to                                                                                                                                       |
| ---------------------------------- | ------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `Stats`                            | `Of(tl)`            | `Heals()`, `Agents()`, `Players()`, `Unknown`, `Recorded`, `Agent(entity)`, `Timeline`, `Extension`, `Version`, `Revision`, `Merged`, `LeftOut` |
| `Agent` (embeds `*timeline.Agent`) | `Stats`, every heal | `Heals()`, `HealsCredited()`, `HealsTaken()`, `Recorded`, `Stats`, and everything a timeline agent has                                          |
| `Heal`                             | `Heals` queries     | `Src`, `Dst`, `Skill`, `Cast`, `Amount`, `IsBarrier`, `IsBuff`, `TargetDowned`, `SrcRecorded`, `DstRecorded`, `Event`, `PeerEvent`              |

An `Agent` node exists for every agent of the timeline, so `h.Agent(p)` is
never nil for a player, target or agent of `tl`. The node embeds the
timeline agent: `node.Name`, `node.PositionAt(t)`, `node.Hits()` all work,
and the node is accepted wherever the timeline takes an `Entity`
(`tl.Hits().By(node)`). A player stands for the node of their main
character, the filters `By`, `CreditedTo` and `On` given a player match
every character, and `Players()` holds one node per character.

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
  written by both clients: the recording client writes its own record at
  once, and the copy a squad member shares when it comes, within 500 ms
  for 97 of 100 and up to a minute later for the others. A heal counts
  once, by the record of the client of its source, as in the windows of
  the addon. The record of the client of the destination stands for the
  heal when no client wrote heals of its source. The decoder pairs the
  two records (same instance ids, skill, amount and kind, opposite
  recording flags, within `PeerWindow`) to date the heal: `Event` is the
  record of the recording player's own client and `PeerEvent` the other
  one, with both `SrcRecorded` and `DstRecorded` set. The closest records
  pair first, within 500 ms, then 5 s, 20 s and a minute, so that a late
  copy takes the place of no other. `Stats.Merged` counts the pairs.
  `Stats.LeftOut` lists the records of a destination that found no record
  of the source though its client writes: they are behind no heal.
  Identical heals written by one client in the same instant (one heal per
  boon granted, for example) are distinct heals and stay so.
- **Whose client.** The client behind a side of a record is the one of
  the master the record names for that side, otherwise the one of its
  agent. `Recorded` follows the records, not the masters of the timeline.
- **Credit.** `Heal.Credited()` is the master of a minion source, as for
  hits; `HealsCredited` and `PerAgent` follow that rule. The master is
  the one the timeline gives, from the events of arcdps. An agent arcdps
  does not list and gives no master takes the first one a record names,
  and its `Agent.Master` is nil.
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
  at the time of the heal, as the timeline does for despawns: the agent
  arcdps saw with that instance id then, or the agent of the address
  when arcdps saw none. A source that stays unknown is `Stats.Unknown`,
  the node of the Unknown sentinel of the timeline. The skill of every
  heal is a `Skill` of the timeline, which knows the ids of the extension
  events as arcdps adds them to its skill table.
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

### Copies that come late or never

```go
l := cookbookLog()
// The copy Bravo shares of the heal it received comes two seconds late
// and its record of the second tick never does.
for i := range l.Events {
	if l.Events[i].Time == epoch+1260 {
		l.Events[i].Time = epoch + 3260
	}
}
l.Events = slices.DeleteFunc(l.Events, func(e evtc.Event) bool { return e.Time == epoch+3050 })
tl, err := timeline.Build(l)
if err != nil {
	panic(err)
}
h := healingstats.Of(tl)
fmt.Println(h.Heals().Count(), "heals,", h.Merged, "written by both clients,", len(h.LeftOut), "record left out")
late := h.Heals().On(tl.PlayerByName("Bravo")).First()
fmt.Println(late.Skill.Name, "at", late.Time, "| copy of Bravo written at", tl.TimeOf(late.PeerEvent))
// The client of Bravo writes the heals Bravo deals: the tick Alpha
// tells alone is no heal.
for _, e := range h.LeftOut {
	fmt.Println("left out:", tl.Skill(e.SkillID).Name, "at", tl.TimeOf(e), "| ticks of Bravo:", h.Heals().By(tl.PlayerByName("Bravo")).Ticks().Count())
}
// 6 heals, 2 written by both clients, 1 record left out
// Shelter at 1.2s | copy of Bravo written at 3.26s
// left out: Regeneration at 3s | ticks of Bravo: 1
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
117 logs of the addon versions 2.17 to 2.19 (format revision 2). The
heals written twice were measured on 3,155 logs: of 1,894,214 records,
46,863 pairs, 45,512 of them within 500 ms, 776 within 5 s, 486 within
20 s and 89 within a minute, and 22 records left out.

| Event                                                                                                                                     | Fields                                                                                                                                                                                                            | Graph                                                                                            |
| ----------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Registration (`EXTENSION`)                                                                                                                | `src_agent`: signature in the low 32 bits, format revision in the next 24, length of the version string in the top 8; `dst_agent`: the version string                                                             | `Stats.Revision`, `Version`, `Extension`                                                         |
| Heal (`EXTENSIONCOMBAT`, `buff` 0)                                                                                                        | `value`: the amount healed, negated                                                                                                                                                                               | `Heal` with `Amount`, `IsBuff` false                                                             |
| Tick of a buff (`buff` 1)                                                                                                                 | `buff_dmg`: the amount healed, negated; `value` 0                                                                                                                                                                 | `Heal` with `IsBuff`                                                                             |
| Barrier (`is_shields` 1)                                                                                                                  | the amount in `value` or `buff_dmg` as above, repeated in `overstack_value`                                                                                                                                       | `Heal` with `IsBarrier`                                                                          |
| Every heal                                                                                                                                | `src_agent`, `dst_agent`, `src_instid`, `dst_instid`, `skillid`, `iff`, `is_ninety`, `is_fifty`, `is_moving` as arcdps writes them; `src_master_instid` and `dst_master_instid` for an agent arcdps does not list | `Src`, `Dst`, `Skill`, `IFF`, `OverNinety`, `UnderFifty`, `Moving`, `TargetMoving`, `Credited()` |
| Every heal                                                                                                                                | `is_offcycle`: bit 7 the client of the source (or of its master) wrote the event, bit 6 the client of the destination did, bit 5 the target of a buff tick was downed, bit 0 the arcdps flag for a downed target  | `SrcRecorded`, `DstRecorded`, `TargetDowned`, `Agent.Recorded`, `Stats.Recorded`                 |
| Two records with opposite recording flags, the same instance ids, skill, amounts and kind, within `PeerWindow` (1 min), the closest first |                                                                                                                                                                                                                   | one `Heal` with `PeerEvent`, counted in `Stats.Merged`                                           |
| A record of the client of the destination alone, when a client wrote heals of its source                                                  |                                                                                                                                                                                                                   | no `Heal`: the record is in `Stats.LeftOut`                                                      |

`result`, `is_activation` and `is_buffremove` are always zero;
`is_flanking` carries no flag on these events and is not read. Elite
Insights reads the same events and, for the heals written twice, keeps
the records of one client per source, destination and skill instead of
pairing them.

## Performance

The decode runs inside `timeline.Build`, after the core graph, in one
arena per node type: about 180 ns and 200 bytes per event of the addon,
under a millisecond for the two thousand heals of a raid log. Queries
follow the rules of the timeline: point lookups and lazy terminals
allocate nothing, a filter allocates its closure once.

`go test ./extensions/healingstats -bench .` prints the numbers for your
machine; the integration tests run when
`tests_fixtures/sabetha-05-fd9b6f3a.zevtc` is present at the repository
root and skip otherwise, and
`EVTC_REAL_LOGS=1 go test ./extensions/healingstats -run TestRealLogs -v`
decodes every log under `tests_fixtures/`, or under the folder the
variable names instead of 1 (relative to the repository root, or
absolute), and reports, per log, the heals, the records merged, those
left out and the heals between recording players that only one client
wrote.
