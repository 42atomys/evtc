# AGENTS.md

Guidance for AI agents working in this repository. Keep it in sync with the
code; the README files are the user-facing documentation.

## What this is

Everything around the arcdps EVTC combat logs of Guild Wars 2: the file
decoder at the root, the timeline SDK, one package per arcdps extension
and the package that combines several recordings of one fight. Module
`github.com/42atomys/evtc`, Go 1.27, no dependencies outside the standard
library. `cmd/` holds two commands.

| Path                       | Role                                                                                                                                                                                                                                                   |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `.` (package `evtc`)       | Raw decoder: `.evtc`/`.zevtc` file to header, agent table, skill table and events, and the writer that puts a log back in a file (`write.go`). No interpretation beyond `Has` and `Warnings`.                                                          |
| `timeline/`                | Temporal graph over a decoded log: agents, hits, casts, buff stacks, effects, missiles, states over time, lazy queries. The main API. Core arcdps only: extension events stay attached to their `Extension` and go to a registered `ExtensionDecoder`. |
| `extensions/healingstats/` | Decoder of the healing stats addon: heals and barrier as nodes linked to the timeline agents, skills and casts, with `Heals` queries. One package per extension under `extensions/`, each registering its decoder in `init`.                           |
| `combine/`                 | Merges the recordings several players made of one fight into one log, read like any other: every agent once, every event once. Built on the timelines of the recordings. See "The combine package" below.                                              |
| `cmd/evtcparser/`          | Tiny command printing a log summary.                                                                                                                                                                                                                   |
| `cmd/evtccombine/`         | Tiny command writing the log that combines the recordings given.                                                                                                                                                                                       |
| `examples/sabetha/`        | A full raid report written with the public API only. Run it after API changes: `go run ./examples/sabetha`.                                                                                                                                            |
| `tests_fixtures/`          | Sample logs (`*.zevtc`), used by integration tests when present. Not part of the API.                                                                                                                                                                  |
| `docs/`                    | `format-history.md`: how the log format changed since 2016, with the dates behind `capabilityTable` and `defects`.                                                                                                                                     |

Read `timeline/README.md` first: model table, conventions, cookbook and the
table "What is read from the log" that maps every arcdps state change to
the graph. The arcdps reference is
`https://www.deltaconnected.com/arcdps/evtc/README.txt` (section
`enum cbtstatechange` gives the field layout of each event kind).

## Key files in `timeline/`

- `build.go`: the builder. `scan` counts everything, `allocateAgents` and
  `allocateSkills` carve exact arenas, `fill` creates and links the nodes,
  `finish` derives the rest (interpolated series, credited hits, lifetime
  bounds). `payload.go` decodes packed payloads by byte offset.
- `agent.go`, `hit.go`, `cast.go`, `buff.go`, `effect.go`, `missile.go`,
  `lifecycle.go`, `skill.go`, `session.go`: the node types and their query
  wrappers. `timeline.go`: the root type and lookups.
- `query.go`: generic lazy `Query[E]` (filters, `Skip`/`Limit`/`Reverse`
  window, terminals). `series.go`, `spans.go`, `interval.go`: time-indexed
  values.
- `events.go`: which event kinds carry agents or a time (`srcIsAgent`,
  `dstIsAgent`, `hasTime`) and the raw `Events` query.
- `capability.go`: `Has`, `Capabilities`, `Missing` and `Warnings` of
  `Timeline`. `Build` asks the decoded log once (`survey`), before the
  build so that an extension decoder can call `Has`, keeps the answers,
  and refuses a build date the log calls invalid.
- `extension.go`: the `Extension` node, the `ExtensionDecoder` interface
  and the registry (`RegisterExtension`); `decodeExtensions` in `build.go`
  runs the decoders after `finish`.

## Extension packages (`extensions/<name>/`)

- The timeline never imports them and decodes nothing of their events;
  they import the timeline and build their graph on top of it. Adding or
  changing an extension must not touch the timeline beyond the hook.
- Each package registers its decoder from `init`, so importing it is the
  only step for a user, and offers a typed accessor (`healingstats.Of`).
  `Decode` runs on the complete graph inside `timeline.Build`.
- Their nodes point to timeline nodes (`*timeline.Agent`, `*timeline.Skill`,
  `*timeline.Cast`) and their agent nodes embed `*timeline.Agent`, so they
  satisfy `timeline.Entity` and the timeline filters accept them.
- Their queries embed `timeline.Query` and use `Query.Narrow` (binary
  search by time) and `Query.Reversed` (to keep groups in time order),
  both exported for extension packages.
- Same rules as the timeline: exact arena sizing checked by their own
  `checkInvariants`, read-only after build, nil-safe query methods on the
  root and agent nodes, doc comments everywhere (`TestDocComments` lists
  the directory), a README whose cookbook is the verbatim copy of
  `example_readme_test.go`, a `sample_test.go` on the Sabetha log and a
  `reallogs_test.go` under `EVTC_REAL_LOGS`.

## The combine package (`combine/`)

Read `combine/README.md` first: the rule of every class of events and the
conventions. `Logs` ranks the logs, builds their timelines and runs
`combine` (`combine.go`).

- `rules.go`: one rule per event kind (its class, the fields that hold an
  address, what names a message, the state it sets), and the typing of
  the older format, a copy of `timeline/legacy.go`. A kind added to
  `state.go` gets a rule here: `TestRulesCoverKinds` fails otherwise.
  `tracks` is a copy of `tracksAgent` of the timeline.
- `clock.go`: the offset between two recordings, from the strikes both
  hold once, or from the ticks of the server when they share too few and
  were made in the same instance, and what tells two fights apart.
- `agents.go`: from the table entries of the recordings to the agents of
  the combined log (`entities`), the spans an agent is there over
  (`presence`) and the entry that speaks for it over time (`segments`).
- `gather.go`: reads the recordings in turn (`read`), pairs the messages
  of each with those gathered (`pair`, `claim`), keeps the rest (`keep`)
  and names the masters the recordings left out (`masters`). `carry.go`:
  the states carried over when the speaker of an agent changes.
  `settle.go`: drops what tells again what the combined log holds,
  numbers the buff stacks, gives the leave of an agent the team the
  entries that knew it told and keeps the one event that names a master
  no other event does.
- The package never changes the logs it is given, and the timeline knows
  nothing of it.
- The combined log holds all the recordings tell. A recording that
  started late or stopped early, the one read first included, leaves the
  others to tell: a rule that keeps the events of the first recording
  alone loses the rest of the fight.
- What names an event is its content; the time only tells apart the
  events of one content, and the agents that carried one instance id.
  The thresholds are measures on real recordings (`windows`, `gain`,
  `relay`, `sameBound`, `shared`, `startTolerance`, `MinStrikes`,
  `MinTicks`). Before changing one, run `TestRealLogs` on recordings of
  one fight and compare its report: messages left alone near one of their
  content, events told twice, states that differ, how far apart the pairs
  were dated.
- `Logs` gives the same log for the same logs, whatever the order given:
  no map is ranged over where the order of its keys could reach the
  outcome. `FuzzLogs` compares both orders.
- A log combined with itself, or with what a client that stopped early
  holds of it, comes back event by event: `sameEvents` compares them on
  every real log. Totals alone let wrong agents through: a change that
  moves what `sameEvents` accepts needs a reason in the README.
- `invariants_test.go` holds the checks of a combined log
  (`checkInvariants`, `sameEvents`, `unpaired`, `toldTwice`, `states`,
  `margins`, `inversions`), `gen_test.go` a generator of fights recorded
  by several clients, with what happened in them (`genFight`), and
  `helpers_test.go` a scene several clients record (`newScene`) and what
  a client that stopped early or wrote under other addresses holds of a
  log (`cut`, `moved`).

## Rules that must hold

- Exact sizing: every slice carved from an arena has `len == cap` after
  build. Any new node or edge is counted in `scan` before being appended in
  `fill`, and both passes must skip the same events. `checkInvariants`
  (`invariants_test.go`) verifies this, plus symmetric edges, sorted lists
  and contiguous spans; extend it with every addition.
- The graph is read-only after `Build` and safe for concurrent reads.
  Pointers go both ways; nodes keep a pointer to their raw `*evtc.Event`.
- Times are `time.Duration` relative to the squad combat start. `Interval`
  is closed on both ends.
- Point helpers (`PositionAt`, `HealthAt`, `TeamAt`, `IsGlidingAt`...)
  return the zero value when unknown, which includes any instant outside
  the agent's `Lifetime`; `Life` alone keeps its final state to the end.
  `DistanceTo` returns NaN. The two-value forms are `Series.At` and
  `Spans.ValueAt`.
- `tl.Unknown` is the sentinel for address 0 (environment, out of range).
  It never gets a master and never carries tracked states. Filters accept
  an `Entity` and a nil entity matches nothing.
- The master of an agent and its instance id come from the events of
  arcdps, and a player has no master. An extension event is in the
  events of the agents its addresses name and tells nothing else of
  them; `checkNaming` (`invariants_test.go`) verifies it. An instance id
  only an extension gives belongs to an agent no table lists and stays
  out of `AgentAt`.
- Queries are lazy: filters compose predicates, `Skip`/`Limit`/`Reverse`
  describe the traversal, terminals iterate. `Query.Seq` keeps a tight
  inlinable loop for the plain forward case: check `BenchmarkHitsFiltered`
  and `BenchmarkHitsAll` after touching it. `TestQueryAllocations` pins
  0 allocations on point lookups and lazy terminals.
- Package `evtc` is a raw decoder. Interpretation belongs to `timeline`,
  with one exception: `capability.go` and `warning.go` read the events
  far enough to tell what a log can carry and what is wrong with it.
- Every exported identifier has a doc comment; `TestDocComments` (root)
  fails otherwise. Comments are short English sentences.
- The cookbook in `timeline/README.md` is a verbatim copy of the Example
  functions in `timeline/example_readme_test.go`: change both together.
  The same holds for `combine/README.md` and each extension.
- A log written by `WriteTo` or `WriteFile` reads back as it was
  (`FuzzParse`). The bytes are those arcdps wrote, the four bytes of
  padding that end each agent aside.

## Working on it

```sh
gofmt -l . && go vet ./... && go test ./...          # always green before finishing
go test ./timeline -bench . -benchmem                 # reference: ~190 ns and 150 B per event
go test ./timeline -run XXX -fuzz FuzzEvents -fuzztime 30s   # after touching build.go
EVTC_REAL_LOGS=1 go test ./timeline -run TestRealLogs -v     # every log under tests_fixtures/, or under EVTC_REAL_LOGS=<folder>: invariants, API smoke, event coverage report
EVTC_REAL_LOGS=1 go test ./extensions/healingstats -run TestRealLogs -v   # same folders, every log with the healing addon: invariants, merge report
go test ./extensions/healingstats -run XXX -fuzz FuzzDecode -fuzztime 30s # after touching its build.go
EVTC_REAL_LOGS=1 go test ./combine -run TestRealLogs -v      # same folders: every log with itself and with a client that stopped early, then the recordings of one fight together
go test ./combine -run XXX -fuzz FuzzLogs -fuzztime 30s      # after touching gather.go, carry.go or settle.go
go test ./combine -bench . -benchmem                  # reference: ~800 ns and 1 kB per event read
```

- Test files are named after the source file they exercise
  (`agent_test.go` for `agent.go`, `build_test.go` for the builder edge
  cases, and so on); the cross-cutting ones are `invariants_test.go`,
  `alloc_test.go`, `bench_test.go`, `fuzz_test.go`, `leak_test.go`,
  `sample_test.go`, `reallogs_test.go` and the `example_*_test.go` files.
- Tests use the fixture builder in `helpers_test.go` (`fixture()`, `b.hit`,
  `b.buffApply`, `b.raw` for packed payloads, `b.build(end)`, `held`), the
  synthetic generator `genLog` in `gen_test.go`, and `mustBuild`.
  `sample_test.go` runs against
  `tests_fixtures/sabetha-05-fd9b6f3a.zevtc` when present and skips
  otherwise. `TestRecordings` of `combine` reads four recordings of one
  Deimos fight from `tests_fixtures/deimos-multi-pov/` when the folder is
  present.
- `TestConcurrentQueries` is meant to run with `-race`, which needs cgo;
  the CI does it.
- When adding an event kind: read its layout in the arcdps README, measure
  it on real logs first (values, units, which field holds what), then add
  the node or field, counts in `scan`, arena in `allocate*`, linking in
  `fill`, invariants, fixture helper, unit test, generator coverage,
  allocation pin if it adds a lookup, README table row and cookbook entry.
  A kind added by a new arcdps release also gets a `Capability` (a
  constant and its row of `capabilityTable`), dated by the first build
  that writes usable data.

## Format facts that are easy to get wrong

- Logs older than arcdps 20260501 write casts and buff events as
  `StateCombat` events. `timeline/legacy.go` gives each event the kind of
  the typed format, and `scan` and `fill` switch on that kind, never on
  `e.IsStateChange`. `Build` returns `ErrLegacyLog` below `MinBuild`
  (20240613), the oldest build `docs/format-history.md` has more than a
  handful of logs for.
- What a log holds depends on the build that wrote it. `Log.Has`
  (`capability.go`) answers from the build date and from what the events
  prove, so that an empty result can be told from a log that cannot say;
  `Log.Warnings` (`warning.go`) lists what is known to be wrong with a
  log, mostly defects the arcdps changelog admits for its release.
- Events are only nearly sorted; the builder stable-sorts them. Metadata
  kinds (`BUFFINFO`, `SKILLINFO`, `IDTOGUID`, `INTEGRITY`...) carry a
  payload in their time field and are excluded from `tl.Events()`.
- Coordinates in effect and missile events are int16 values of the game
  coordinate divided by ten; effect durations are milliseconds with 0 and
  0xFFFFFFFF meaning unknown; `Hit.Damage` combines health and barrier
  damage, `Barrier` is the absorbed part.
- Buff stacks are matched by the trackable id in `pad61..pad64`; ids are
  reused once a stack ended. `BUFFREMOVE_ALL` events summarize the single
  removes written with them and are not attached to anything.
- `DESPAWN` (and a few other state events) of minions and NPCs are written
  with `src_agent` 0 and only `src_instid` set; the builder resolves them
  through the agent arcdps saw last with that instance id. Agents leave
  tracking with their buffs: a despawn ends their open stacks
  (`EndedByDespawn`).
- Instance ids are reused by successive agents; resolve them with
  `tl.AgentAt(inst, t)`, never by id alone.
- Extension combat events carry the extension signature in `pad61..pad64`;
  the timeline keeps them raw, adds their `skillid` to the skills (arcdps
  does the same in its skill table) and hands them to the decoder of the
  signature.
- Healing stats addon: the field layout is the table "What is read from
  the log" of `extensions/healingstats/README.md`. A heal between two
  clients running the addon is written twice. It counts once, by the
  record of the client of its source, as in the windows of the addon; the
  decoder pairs the two records within `PeerWindow`, the closest first,
  to date it. Identical heals in the same millisecond from one client
  are real (one per boon granted, for example). The addon sends its
  records from a client to the others one at a time, from a queue that
  drops what overflows: of 46,863 copies, 45,512 are written within
  500 ms, 89 between 20 s and a minute later, and some never.
- The client that wrote a record of the healing stats addon is the one
  of the master the record names, not of the master the timeline gives
  the agent of its address, which may be another agent (see the list of
  agents of an extension below). With the master of the timeline, 983
  heals of players who do not run the addon were taken for copies.
- arcdps records the agents within 5,000 units of the player on each
  axis. Two recordings of one fight do not hold the same agents, nor the
  same events.
- Addresses and clocks are those of a client: two recordings share
  neither. Instance ids and what the server put in a message are the
  same for every client. Stack numbers are given over the whole log, not
  per agent, and two clients agree on those of the stacks both see.
- A client that finds again an agent it lost lists it anew, under
  another address: 5 of 77,197 entries of 152 logs hold their agent over
  several spans. It writes the stacks the agent wears as applications, in
  the millisecond of the spawn or after it, never before.
- Two squads that fight the same boss at the same time, each in its own
  instance of the map, share strikes by chance: 10 to 40 measured, with
  gaps minutes apart. Recordings of one fight share thousands, whose
  gaps stand within 300 ms of their median, and their server clocks agree
  within a second.
- An extension keeps a list of agents of its own. Its events may name an
  address no table lists, or the address of an agent under the instance
  id and the master of another one. The master of an agent, and the
  instance ids it carries, are those its other events name. Over 10,565
  logs, 3,155 with the healing addon: 6,196 of 3.8 million named agents
  come with the instance id arcdps gives another address (294 logs),
  5,599 of them at an address no table lists; 258 name a player with a
  master (68 logs), and no event of arcdps ever does. The instance ids
  and the masters are right, the addresses are those the addon kept: it
  names a gyrocopter by the address of the clone that held its instance
  id 38 s before.
- `EffectAgentCreate` names an agent in `dst_agent`. In 5 logs of 10,565
  it names one no table lists and no other event names.
- A client writes `SquadCombatStart` when its log starts and
  `SquadCombatEnd` when it ends. 24 logs of 10,565 hold more, 2 to 22 of
  each: the squad left combat and came back to it.
- A client writes the first events of a minion, its spawn among them,
  before it learns whose it is: without master.
- `TICK` (since 20260701) writes in `src_agent` the count of ticks of the
  server since the instance started, 25 a second, the same for every
  client of the instance within one tick; `dst_agent` is the ticks since
  the last real update, up to 3, so the time of a tick is extrapolated;
  `value` is the ping since 20260915. `INSTANCESTART` writes the socket
  of the server in `value`, the same for every client of an instance.
  Over the corpus, recordings of one instance date its start within a
  second of each other, two instances of one map 36 s apart at the
  least. Two recordings that share no strike are set on their ticks: the
  offsets by ticks and by strikes agree within 18 ms on 19 pairs.
- A client learns the master of a minion per table entry. When it lists
  the minion anew, after losing it, it may never learn the master again:
  in one fight of `tmp/multi-povs`, a recording lists a minion ten times
  and names its master in three of them.
- `TEAMCHANGE` is written when the team changes. Before 20260920 that is
  at the leave alone, with the team the agent had in `value` and 0 in
  `dst_agent`; a client that never learned the team writes 0 there. Since
  20260920 the server message that changes the team, `value` old and
  `dst_agent` new, may be dated 1 ms after the leave the client writes at
  the same instant: a log read alone then takes the team the agent left
  with for the team it had.
