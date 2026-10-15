# combine

`combine` merges the recordings several players made of one fight into
one log.

arcdps records what happens around the player who runs it: the agents
further than 5,000 units on an axis are not in the log, nor what they did.
When a squad spreads over a large arena, the whole fight is in the logs of
several players, each with its own addresses, its own clock and its own
view of the same agents.

```go
import "github.com/42atomys/evtc/combine"

log, err := combine.Files("mine.zevtc", "theirs.zevtc")
if err != nil {
	return err
}
err = log.WriteFile("fight.zevtc")
```

`go run ./cmd/evtccombine -o fight.zevtc mine.zevtc theirs.zevtc` does the
same from a terminal.

The combined log is an ordinary log: `timeline.Build` reads it, and so
does anything that reads a `.zevtc` file. Every agent of the fight is in
it once, under one address, whatever the number of recordings that held
it. A strike, a cast or a buff application that several clients were told
of is in it once as well.

The logs must have been written by the same arcdps build, one that the
timeline reads (20240613 or later). They are left as they are.

## What the combined log holds

Every event kind follows one rule, by what the kind is.

| Events                 | Kinds                                                                                | In the combined log                                                            |
| ---------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ |
| Messages of the server | strikes, casts, buff events, effects, missiles, downs, deaths, teleports, rewards... | Once. Those several recordings hold are paired on what the server put in them. |
| Made by the client     | positions, health, barrier, defiance bars, markers, targetable, buff damage ticks    | From one recording per agent at a time.                                        |
| Spawn or leave         | `Spawn`, `Despawn`, `TeamChange` to no team                                          | At the bounds of what the recordings held together.                            |
| About the fight        | `SquadCombatStart`, `SquadCombatEnd`, `LogNPCUpdate`                                 | Paired. The fight starts at the earliest start and ends at the latest end.     |
| About a recording      | `PointOfView`, `Language`, the builds, the map, the shard                            | Those of the first recording.                                                  |
| Of a client over time  | `Tick`, `RateHealth`, `MapChange`                                                    | From one recording at a time: the first one that was going on.                 |
| Definitions            | `BuffInfo`, `BuffFormula`, `SkillInfo`, `SkillTiming`, `IDToGUID`                    | United.                                                                        |
| Extensions             | `Extension`, extension combat events                                                 | Paired on all of their payload and on the instance ids of their agents.        |
| Address changes        | `IIDChange`                                                                          | Dropped: addresses are rewritten.                                              |

`rules.go` holds the rule of each kind and what names its messages:
values, stack numbers, effect and missile ids. Flags such as flanking or
moving are left out, each client computes them. `TestRulesCoverKinds`
fails when the decoder names a kind without a rule.

## Conventions

- **The first recording.** The order given does not matter. The log that
  holds the most events is read first, and gives the combined log its
  header, its clock, its addresses and what a client writes once about
  its own recording: the point of view, the language, the builds. Logs
  that hold as many events are told apart by what they hold.
- **A log that started late or stopped early.** The combined log holds
  all the recordings tell, whatever the one that is read first. What a
  client writes about itself over time, its ping for one, comes from one
  recording at a time: the first one that was going on, then the next
  one where it was not. A reward is there once, whoever got it.
- **Builds.** The package reads what the timeline reads, arcdps 20240613
  and later, in the format before 20260501 and in the typed one. It
  holds no table of builds. A log of a later build is combined like the
  others, and the events of a kind the package has no rule for come
  from the first recording alone. The logs given to one call come from
  the same build.
- **Clocks.** The offset between two recordings is the median gap between
  the strikes both hold once. Two recordings of one instance that share
  fewer than `MinStrikes` strikes, as two recorders too far apart to see
  the same creatures, are set on the ticks of the server: arcdps writes
  one a second since build 20260701, the count of ticks since the
  instance started, the same for every client. On 19 pairs of recordings
  measured, the offset by the ticks and the offset by the strikes agree
  within 18 ms; the strikes stand closer together and set the clock when
  both logs hold them. A recording that shares too little with the first
  one is set on another one whose clock is set. arcdps 20260920 dates
  what comes from the server with an even time and the rest with an odd
  one: a time keeps its parity.
- **The same fight.** Two recordings share at least `MinStrikes` strikes
  whose gaps stand together, and started at the same time by the clock
  of the server, within five seconds. Two squads that fight the same boss
  at the same time share strikes by chance, minutes apart. Without
  strikes enough, two recordings are of one instance when they name the
  same socket of the server and an instance that started at the same
  time, within five seconds, and hold `MinTicks` ticks of the server in
  common: two instances of one map started 36 s apart at the least over
  the corpus measured. `Logs` returns `ErrFightMismatch` otherwise, and
  `ErrBuildMismatch` for the logs of two arcdps builds.
- **Agents.** A table entry is what a recording held of an agent under one
  address. Two entries of two recordings are the same agent when they
  carry the same instance id, the same species for a creature, the same
  account and name for a player, and either started within a second of
  each other, or share time, or never moved and stand at the same place.
  Entries that share less than half a second because a client learned
  late that an agent left stay two agents. A recording never gets two of
  its entries in one agent when their events share time. A log of a
  public instance lists the other players without account: their name
  and their instance id tell them apart.
- **Presence.** An agent is there over the spans its entries held it,
  united. A spawn is kept at the start of a span and a leave at its end:
  the earliest spawn, the latest leave. Inside a span they are dropped,
  unless every entry that held the agent across the bound wrote it.
- **Pairs.** What names a message is what the server put in it: the
  instance ids of its agents, its skill, its value, its result, the
  number of its stack, effect or missile. The log holds no id of the
  event and no time of the server, and each client dates what it
  receives on its own clock: the time only tells apart the messages of
  one content. A message pairs with the nearest one of its content that
  another recording tells, the close ones first, and none further than
  2 s. The messages both tell once between two agents tell how late a
  recording gets them, and the pairing takes it off. See "What the
  tolerances decide" below.
- **The same log every time.** Nothing is drawn, and nothing depends on
  the order given or on the machine: the same logs give the same log,
  byte for byte.
- **Who speaks for an agent.** The samples and the states of an agent come
  from one entry at a time. The client of a player speaks for its own
  character. For the others, the entry that held the agent the longest
  speaks over all it held, then the next one over what is left. A
  teleport or a death is dated by the entry that speaks, so that it does
  not fall between two of its samples. When the speaker changes, the
  combined log gets what the new one held of each state, unless it holds
  that value already, or a telling that is newer. What the new speaker
  tells first, with the value the one before told last and within half a
  second of it, is that telling dated by another client. It is another
  event when the new speaker had told the value as many times already.
- **Buff stacks.** A client numbers the stacks it holds. Two clients
  agree on the stacks both see; a stack one of them holds alone gets a
  number of its own, from 2^31 on, when its number is taken. A client
  that gives the number of a stack it told to the next one ended the
  first, as within one log. The stacks a client tells when it gains an
  agent are dropped when the combined log holds them. The buff events of
  the character of a recording player are those its client tells.
- **Masters.** A recording that does not know the master of a minion
  leaves it out of its events. A client learns the master per table
  entry, and may list the minion anew without learning it again: the
  events of the entries that never name the master, of every recording
  that tells them, get the master another entry names, when the master is
  there. What an entry that names the master wrote without it stays so: a
  client writes the first events of a minion before it learns whose it
  is. When no event kept names the master while it is there, the first
  event dropped that does is kept as well: a state told again, or one an
  entry that does not speak for the agent tells. Over 147 fights recorded
  twice or more, this keeps a master for 122 minions that would have
  none. A player has no master.
- **Teams.** A client writes the team of an agent when it changes, and no
  team when it stops holding the agent, with the team it had. A client
  that gained the agent without learning its team leaves it with none:
  the leave of the combined log tells the team the entries that knew it
  told.
- **The start and the end of a log.** A client writes a start of the
  squad combat when its log starts and an end when it ends. Those of a
  client that started late or stopped early, while another one was
  recording, are left out.
- **Extensions.** The package cannot read their events: two are the same
  one when all of their payload and the instance ids of their agents
  are. The healing stats addon sends what a client computes to the
  others, which write it up to a minute later and several at once: the
  events of a key pair in their order within a minute, and the earliest
  copy dates them.
- **Events without address.** arcdps writes some events with an instance
  id and no address. The combined log names the agent the timeline of
  the recording gave them to: the last agent seen with an instance id
  is not the same one once recordings are put together. The states of
  an instance id no recording holds an agent of stay those of nobody,
  and come from the first recording that was going on then.
- **Addresses no table lists.** An event may name an address its
  recording lists no agent at. The first recording keeps what it wrote.
  The address of another recording tells nothing among those of the
  first one: it is left out, and the instance id stays.
- **Order.** The events are in time order, the definitions first. Those a
  recording wrote at one instant keep their order.
- **One log.** `Logs` returns a single log as it is, and combines up to
  `MaxLogs` logs.

## Cookbook

Every snippet below is an `Example` test of the package
(`example_readme_test.go`), run on a ten second fight spread over 8,000
units. Alpha stands by Sabetha and Bravo by Kernan, each out of the view
of the other, and both see Charlie fight Knuckles between them. Each
recorded a log. The outputs are checked by `go test`.

### Combine two logs

```go
byAlpha, byBravo := cookbookLogs() // evtc.ParseFile("alpha.zevtc") on real files
log, err := combine.Logs(byAlpha, byBravo)
if err != nil {
	panic(err)
}
for _, l := range []*evtc.Log{byAlpha, byBravo, log} {
	tl := mustBuild(l)
	fmt.Println(len(l.Events), "events |", tl.NPCs().Count(), "creatures |", tl.Hits().Count(), "hits for", tl.Hits().Damage(), "damage")
}
// 88 events | 2 creatures | 35 hits for 45345 damage
// 78 events | 2 creatures | 35 hits for 55345 damage
// 118 events | 3 creatures | 45 hits for 75390 damage
```

### From files to a file

```go
dir := cookbookFiles() // the folder of the logs
defer os.RemoveAll(dir)
log, err := combine.Files(filepath.Join(dir, "alpha.zevtc"), filepath.Join(dir, "bravo.zevtc"))
if err != nil {
	panic(err)
}
fight := filepath.Join(dir, "fight.zevtc")
if err := log.WriteFile(fight); err != nil {
	panic(err)
}
tl, err := timeline.ParseFile(fight) // an ordinary log
if err != nil {
	panic(err)
}
fmt.Println(filepath.Base(fight), "|", tl.Players().Count(), "players,", tl.NPCs().Count(), "creatures,", tl.Duration)
// fight.zevtc | 3 players, 3 creatures, 10s
```

### Every agent once, every strike once

```go
byAlpha, byBravo := cookbookLogs()
log, err := combine.Logs(byAlpha, byBravo)
if err != nil {
	panic(err)
}
tl := mustBuild(log)
for a := range tl.NPCs().Seq() {
	fmt.Printf("%s: %d hits taken, from %s\n", a.Name, a.HitsTaken().Count(), a.HitsTaken().First().Src.Name)
}
// Both clients told the strikes on Knuckles: they are there once.
first := tl.Agents().Where(func(a *timeline.Agent) bool { return a.Name == "Knuckles" }).First().HitsTaken().First()
fmt.Println("first strike on Knuckles at", first.Time, "for", first.Damage)
// Sabetha: 10 hits taken, from Alpha
// Knuckles: 25 hits taken, from Charlie
// Kernan: 10 hits taken, from Bravo
// first strike on Knuckles at 0s for 1000
```

### What a recording alone does not see

```go
byAlpha, byBravo := cookbookLogs()
log, err := combine.Logs(byAlpha, byBravo)
if err != nil {
	panic(err)
}
// Alpha never saw Bravo: its log has no position for that player.
at := 5 * time.Second
for _, l := range []*evtc.Log{byAlpha, log} {
	tl := mustBuild(l)
	b := tl.PlayerByName("Bravo").Ref()
	fmt.Println(b.Position.Len(), "samples of Bravo, at", b.PositionAt(at).X, "| distance to Alpha", tl.PlayerByName("Alpha").Ref().DistanceTo(b, at))
}
// 0 samples of Bravo, at 0 | distance to Alpha NaN
// 20 samples of Bravo, at 8000 | distance to Alpha 8000
```

### The order does not matter

```go
byAlpha, byBravo := cookbookLogs()
one, err := combine.Logs(byAlpha, byBravo)
if err != nil {
	panic(err)
}
other, err := combine.Logs(byBravo, byAlpha)
if err != nil {
	panic(err)
}
fmt.Println("the same log:", slices.Equal(one.Events, other.Events), slices.Equal(one.Agents, other.Agents))
// The log of Alpha holds the most events: the combined log is
// recorded by Alpha, under its addresses.
tl := mustBuild(one)
fmt.Printf("recorded by %s | Bravo at %#x, Kernan at %#x\n", tl.POV.Account, tl.PlayerByName("Bravo").Ref().Addr, tl.NPCs().Last().Addr)
// the same log: true true
// recorded by Alpha.1234 | Bravo at 0x1000000c, Kernan at 0x20000017
```

### A log that stopped early

```go
byAlpha, byBravo := cookbookLogs()
// Alpha lost its connection eight seconds into the fight.
early := *byAlpha
early.Events = nil
for _, e := range byAlpha.Events {
	switch at := e.Time - epoch; {
	case e.IsStateChange == evtc.StateSquadCombatEnd:
		e.Time = epoch + 8000
		fallthrough
	case at <= 8000:
		early.Events = append(early.Events, e)
	}
}
log, err := combine.Logs(&early, byBravo)
if err != nil {
	panic(err)
}
// Bravo tells the rest of the fight. Nobody else saw Sabetha.
for _, l := range []*evtc.Log{&early, log} {
	tl := mustBuild(l)
	fmt.Print(tl.Duration, " |")
	for a := range tl.NPCs().Seq() {
		fmt.Printf(" %s: %d hits taken.", a.Name, a.HitsTaken().Count())
	}
	fmt.Println()
}
// 8s | Sabetha: 8 hits taken. Knuckles: 21 hits taken.
// 10s | Knuckles: 25 hits taken. Kernan: 10 hits taken. Sabetha: 8 hits taken.
```

### Logs that do not go together

```go
byAlpha, byBravo := cookbookLogs()
older := *byBravo
older.Header.Build = "20260816"
_, err := combine.Logs(byAlpha, &older)
fmt.Println(errors.Is(err, combine.ErrBuildMismatch), err)

// Another fight: none of its strikes is in the log of Alpha.
other := *byBravo
other.Events = nil
for _, e := range byBravo.Events {
	e.Value += 5000
	other.Events = append(other.Events, e)
}
_, err = combine.Logs(byAlpha, &other)
fmt.Println(errors.Is(err, combine.ErrFightMismatch), err)
// true combine: logs of several arcdps builds: 20260920 and 20260816
// true combine: logs of several fights: 0 strikes shared, 20 wanted
```

## What it does not do

- Logs of two arcdps builds, and logs the timeline refuses.
- Two recordings that share fewer than `MinStrikes` strikes and fewer
  than `MinTicks` ticks of the server: a few seconds on a training golem,
  or logs older than arcdps 20260701 whose recorders stood too far apart
  to see the same creatures.
- The point of view and the language of the other recordings: a log has
  one recorder.
- The ping of a client while a recording read before it was going on:
  one client tells it at a time.
- Agents without instance id that share a name: nothing tells them apart
  from a recording to the other, and the first one that holds some keeps
  its own.
- A creature every client lost and one of them found again: nothing
  tells it is the same one, and the combined log holds two agents, as the
  log of that client does.
- Markers and defiance bar states are made by each client and differ
  between two recordings of one fight. The combined log holds those of
  the recording that speaks.
- The ground markers two clients tell differently when their log starts:
  both tellings are kept, the last one stands.
- An effect on the ground a client is told of when it comes in sight of
  it, more than 2 s after it was placed, is in the combined log twice:
  29 of 100,925 measured.
- An event of an extension that reaches a client more than a minute
  late is in the combined log twice. Of the 46,869 heals two clients
  wrote in 3,155 logs, 6 at most came so late.

## What the tolerances decide

Measured on 19 pairs of recordings of 14 fights: 1,444,540 events that
both recordings tell, those of extensions aside.

| Events                                                      | Share      |
| ----------------------------------------------------------- | ---------- |
| Whose content each recording tells once                     | 55.3%      |
| Whose content each recording tells as many times            | 43.7%      |
| Whose content one recording tells more often than the other | 1.0%       |
| Dated at most 30 ms apart by the two clients                | 98.1%      |
| Dated at most 100 ms apart                                  | 99.8%      |
| Dated at most 500 ms apart                                  | 99.98%     |
| Dated more than 500 ms apart                                | 294 events |

Two clients date the same event 8 ms apart, by the median. Of the 294
events dated more than 500 ms apart, 254 are applications of a buff,
most of them the stacks a client tells when it gains an agent, which it
dates when it gains it. One is a strike, of 112,673.

129 events found no pair though the other recording holds one of their
content, left alone as well, between 2 s and 5 s away. 93 are
applications of a buff told by a client that gained the agent: the
combined log holds the stack and drops them. 29 are effects on the
ground that a client was told of when it came in sight of them, seconds
after they were placed: the combined log holds them twice.

A tolerance that is wrong leaves an event twice in the combined log, or
takes two events of one content for one. The report of `TestRealLogs`
gives these counts for the recordings it combines.

## Checks

The tests combine logs whose outcome is known.

- Generated fights (`TestGenerated`), spread over more than a client
  sees: the combined log holds every strike, death and buff stack a
  client at least was told of once, and every creature once.
- A log with itself: it comes back as it is. Its events are compared one
  by one (`sameEvents`): all of them are there but the address changes,
  in their order, each as it was written. An event written without
  address may name its agent, and a leave written in two events stands
  where the last one does.
- A log with what a client that stopped at three fifths of the fight
  would hold of it, under other addresses and another clock: the whole
  fight comes back, whichever of the two is read first. Its events are
  compared one by one as well.
- Every combined log: its events name the agents of its table, each
  under an instance id a recording names it under.
- The heals of the healing stats addon (`TestSampleHeals`): those of a
  log combined with itself are those of the log.
- Recordings of one fight: no two messages of one content are left alone
  within half a second of each other (`unpaired`), the states of the
  combined timeline are those of the recording that speaks, of what it
  was there to see (`states`), the facts two recordings tell a
  moment apart are counted (`toldTwice`), and so are how far apart the
  pairs were dated (`margins`) and the pairs of messages of the server
  the two recordings tell in another order (`inversions`): every client
  receives them in the same order, so a pair out of order was dated
  apart by the two clients. On the Deimos recordings, 198 pairs of
  533,400 are, most of them two messages within 20 ms of each other,
  the rest stacks a client tells when it gains an agent.

`EVTC_REAL_LOGS=1 go test ./combine -run TestRealLogs -v` runs the two
checks of a log alone on every log under `tests_fixtures/`, or under the
folder the variable names instead of 1 (relative to the repository root,
or absolute), and combines the logs of the folder that recorded the same
fight. Over 10,573 logs, the 10,533 that hold strikes enough come back
as they are, event by event: 1.58 billion events of 48 arcdps builds
from 20240613 to 20260920, 35 of them of the older format.

`TestRecordings` asserts exact values of four recordings of one Deimos
fight, read from `tests_fixtures/deimos-multi-pov/` when the folder is
present.

## Performance

`Logs` builds the timeline of every log, then reads their events once:
about one microsecond and one kilobyte per event read, 300 ms for two
recordings of a raid log of 183,000 events.

`go test ./combine -bench . -benchmem` prints the numbers for your
machine.
