# AGENTS.md

Guidance for AI agents working in this repository. Keep it in sync with the
code; the README files are the user-facing documentation.

## What this is

Everything around the arcdps EVTC combat logs of Guild Wars 2: the file
decoder at the root, the timeline SDK and one package per arcdps
extension. Module `github.com/42atomys/evtc`, Go 1.27, no dependencies
outside the standard library. `cmd/` holds one small command.

| Path                       | Role                                                                                                                                                                                                                                                   |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `.` (package `evtc`)       | Raw decoder: `.evtc`/`.zevtc` file to header, agent table, skill table and events. No interpretation.                                                                                                                                                  |
| `timeline/`                | Temporal graph over a decoded log: agents, hits, casts, buff stacks, effects, missiles, states over time, lazy queries. The main API. Core arcdps only: extension events stay attached to their `Extension` and go to a registered `ExtensionDecoder`. |
| `extensions/healingstats/` | Decoder of the healing stats addon: heals and barrier as nodes linked to the timeline agents, skills and casts, with `Heals` queries. One package per extension under `extensions/`, each registering its decoder in `init`.                           |
| `cmd/evtcparser/`          | Tiny command printing a log summary.                                                                                                                                                                                                                   |
| `examples/sabetha/`        | A full raid report written with the public API only. Run it after API changes: `go run ./examples/sabetha`.                                                                                                                                            |
| `tests_fixtures/`          | Sample logs (`*.zevtc`), used by integration tests when present. Not part of the API.                                                                                                                                                                  |

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
- Queries are lazy: filters compose predicates, `Skip`/`Limit`/`Reverse`
  describe the traversal, terminals iterate. `Query.Seq` keeps a tight
  inlinable loop for the plain forward case: check `BenchmarkHitsFiltered`
  and `BenchmarkHitsAll` after touching it. `TestQueryAllocations` pins
  0 allocations on point lookups and lazy terminals.
- Package `evtc` is a raw decoder. Interpretation belongs to `timeline`.
- Every exported identifier has a doc comment; `TestDocComments` (root)
  fails otherwise. Comments are short English sentences.
- The cookbook in `timeline/README.md` is a verbatim copy of the Example
  functions in `timeline/example_readme_test.go`: change both together.

## Working on it

```sh
gofmt -l . && go vet ./... && go test ./...          # always green before finishing
go test ./timeline -bench . -benchmem                 # reference: ~190 ns and 150 B per event
go test ./timeline -run XXX -fuzz FuzzEvents -fuzztime 30s   # after touching build.go
EVTC_REAL_LOGS=1 go test ./timeline -run TestRealLogs -v     # every log under tests_fixtures/: invariants, API smoke, event coverage report
EVTC_REAL_LOGS=1 go test ./extensions/healingstats -run TestRealLogs -v   # every log with the healing addon: invariants, merge report
go test ./extensions/healingstats -run XXX -fuzz FuzzDecode -fuzztime 30s # after touching its build.go
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
  otherwise.
- `TestConcurrentQueries` is meant to run with `-race`, which needs cgo;
  the CI does it.
- When adding an event kind: read its layout in the arcdps README, measure
  it on real logs first (values, units, which field holds what), then add
  the node or field, counts in `scan`, arena in `allocate*`, linking in
  `fill`, invariants, fixture helper, unit test, generator coverage,
  allocation pin if it adds a lookup, README table row and cookbook entry.

## Format facts that are easy to get wrong

- Logs older than arcdps 20260501 use another encoding; `Build` returns
  `ErrLegacyLog` for them.
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
  through the agent last seen with that instance id. Agents leave tracking
  with their buffs: a despawn ends their open stacks (`EndedByDespawn`).
- Instance ids are reused by successive agents; resolve them with
  `tl.AgentAt(inst, t)`, never by id alone.
- Extension combat events carry the extension signature in `pad61..pad64`;
  the timeline keeps them raw, adds their `skillid` to the skills (arcdps
  does the same in its skill table) and hands them to the decoder of the
  signature.
- Healing stats addon: the field layout is the table "What is read from
  the log" of `extensions/healingstats/README.md`. A heal between two
  clients running the addon is written twice and the decoder pairs the
  records within `PeerWindow`; identical heals in the same millisecond
  from one client are real (one per boon granted, for example).
