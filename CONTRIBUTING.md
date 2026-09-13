# Contributing

Issues and pull requests are welcome. For anything bigger than a fix,
open an issue first so that we agree on the shape of the API before you
write it.

## Before sending a change

```sh
gofmt -l . && go vet ./... && go test ./...
```

The CI runs the same commands, with `-race` on the tests. After touching
one of the builders, give the fuzzers some time:

```sh
go test ./timeline -run XXX -fuzz FuzzEvents -fuzztime 30s
go test ./extensions/healingstats -run XXX -fuzz FuzzDecode -fuzztime 30s
```

and check that the benchmarks did not move
(`go test ./timeline -bench . -benchmem`: about 190 ns and 150 bytes per
event on a raid log).

## Logs

No combat log is committed: they are large and carry the account names of
the whole squad. Tests that need one skip when it is missing. Put your own
logs under `tests_fixtures/`, which git ignores:

- `EVTC_REAL_LOGS=1 go test ./timeline -run TestRealLogs -v` builds every
  log of the folder, checks the invariants of the graph and reports how
  much of each log it reads. The same command on
  `./extensions/healingstats` reports the heals of the logs recorded with
  the healing stats addon.
- The `TestSample*` tests assert exact values of one Sabetha log
  (`tests_fixtures/sabetha-05-fd9b6f3a.zevtc`) and only pass on that file.

## How the code is organized

- Package `evtc` decodes the file and nothing else. Interpretation belongs
  to `timeline`.
- The timeline builder works in two passes: `scan` counts every node and
  edge, `fill` creates and links them in arenas sized from those counts.
  Both passes must skip the same events. `checkInvariants`
  (`timeline/invariants_test.go`) verifies the sizing, symmetric edges,
  sorted lists and contiguous spans: extend it with every new node or
  edge.
- The graph is read-only after `Build` and safe for concurrent reads.
- Queries are lazy and point lookups do not allocate;
  `TestQueryAllocations` pins it.
- Every exported identifier has a doc comment (`TestDocComments`).
- The cookbooks of the READMEs are copies of the `example_readme_test.go`
  files of their package, whose outputs `go test` checks: change both
  together.
- Test files are named after the source file they exercise.

## Adding an event kind

Read its layout in the
[arcdps README](https://www.deltaconnected.com/arcdps/evtc/README.txt)
(`enum cbtstatechange`) and look at its values on real logs before
modeling it. Then add the node or field, its count in `scan`, its arena,
its linking in `fill`, the invariants, a fixture helper in
`helpers_test.go`, a unit test, coverage in the synthetic generator
(`gen_test.go`) and a row in the table "What is read from the log" of
`timeline/README.md`.

## Adding an extension

Each arcdps extension gets its own package under `extensions/`. It
imports the timeline, never the reverse, and registers its
`timeline.ExtensionDecoder` from `init`. `extensions/healingstats` is the
reference for the layout, tests and README.
