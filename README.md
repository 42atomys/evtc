# evtc

[![Go Reference](https://pkg.go.dev/badge/github.com/42atomys/evtc.svg)](https://pkg.go.dev/github.com/42atomys/evtc)
[![ci](https://github.com/42atomys/evtc/actions/workflows/ci.yml/badge.svg)](https://github.com/42atomys/evtc/actions/workflows/ci.yml)

Go tooling around the arcdps EVTC combat logs of Guild Wars 2: the file
decoder, the timeline SDK and the decoders of arcdps extensions, each in
its own package of this module. Standard library only.

```sh
go get github.com/42atomys/evtc@latest
```

| Package | Purpose |
| --- | --- |
| `github.com/42atomys/evtc` (`evtc`) | Decodes `.evtc` and `.zevtc` files into their header, agent table, skill table and raw events, without interpretation. |
| `github.com/42atomys/evtc/timeline` | Builds a temporal graph over a decoded log and lets you query it by time: positions, health, casts, hits, buffs, downs, breakbars. See [timeline/README.md](timeline/README.md). |
| `github.com/42atomys/evtc/extensions/healingstats` | Decodes the events of the healing stats addon into heals and barrier linked to the timeline; importing it is enough. See [extensions/healingstats/README.md](extensions/healingstats/README.md). |
| `cmd/evtcparser` | Prints the header and table sizes of a log: `go run ./cmd/evtcparser fight.zevtc`. |
| `examples/sabetha` | A full report of a Sabetha raid log written with the timeline API: `go run ./examples/sabetha fight.zevtc`. |

```go
tl, err := timeline.ParseFile("fight.zevtc")
if err != nil {
	return err
}
for _, c := range tl.Boss().HitsTaken().Landed().PerAgent() {
	fmt.Println(c.Agent.Name, c.Hits.Damage())
}
if h := healingstats.Of(tl); h != nil { // the log has the healing stats addon
	for _, c := range h.Heals().Healing().PerAgent() {
		fmt.Println(c.Agent.Name, c.Heals.Healed())
	}
}
```

Each arcdps extension gets a package under `extensions/`, plugged into the
timeline through a decoder hook (see
[Extensions](timeline/README.md#extensions)).

Requires Go 1.27. Only logs of arcdps 20260501 and later are supported.
The API may still change before a v1.0.0 tag.

## Development

`go test ./...` runs the suite, `go test ./timeline -bench .` the
benchmarks and `go test ./timeline -run XXX -fuzz FuzzEvents` the event
stream fuzzer. See [CONTRIBUTING.md](CONTRIBUTING.md) to run the tests
that need a combat log.

## License

[MIT](LICENSE). Guild Wars 2 is a trademark of ArenaNet; this project is
not affiliated with ArenaNet or with the author of arcdps.
