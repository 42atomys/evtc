// Package timeline builds a temporal, fully linked graph from a decoded
// arcdps EVTC log and lets callers query it by time.
//
// Build turns an evtc.Log into a Timeline whose nodes point to each other
// in both directions: an Agent reaches its hits, casts, buff stacks, downs,
// deaths, breakbars, effects and missiles; a Hit reaches its source,
// target, skill, cast and the down or death it caused; a Cast reaches its
// hits; a BuffStack reaches its applier, receiver and buff definition; a
// Skill reaches its casts, hits and metadata. Values that change over
// time, such as positions, health percentages and life states, are Series
// and Spans that answer "what was the value at time t" with a binary
// search.
//
// All times are time.Duration values relative to the squad combat start of
// the log. The graph is built once, with one allocation per node type, and
// is read-only afterwards, so a Timeline can be queried concurrently.
// Queries such as Hits, Casts, Stacks and Events are lazy: filters compose a
// predicate, Skip, Limit and Reverse only describe the traversal, and
// nothing is copied until a terminal such as All or Map is called.
//
// The combat events of arcdps extensions are kept on their Extension and
// decoded by the package that knows the extension: it registers an
// ExtensionDecoder with RegisterExtension and Build stores what it decodes
// in Extension.Decoded. The extensions/healingstats package does so for
// the healing stats addon.
//
// Only logs written by arcdps 20260501 or later are supported: earlier
// logs encode casts and buffs differently and Build returns ErrLegacyLog
// for them. The package requires Go 1.27 for the generic methods of its
// query types.
package timeline
