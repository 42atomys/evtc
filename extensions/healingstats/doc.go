// Package healingstats decodes the combat events written by the healing
// stats addon of arcdps into a graph on top of the timeline of a log:
// every heal and barrier the addon logged, linked to the agents, skills
// and casts of the timeline.
//
// Importing the package registers its decoder with timeline, so a
// Timeline built afterwards carries the decoded stats, reached with Of:
//
//	tl, err := timeline.ParseFile("fight.zevtc")
//	h := healingstats.Of(tl) // nil when the log has no healing stats
//	for _, c := range h.Heals().Healing().PerAgent() {
//		fmt.Println(c.Agent.Name, c.Heals.Healed())
//	}
//
// The addon (https://github.com/Krappa322/arcdps_healing_stats) writes one
// extension combat event per heal it sees: the amount negated in the value
// field, or in buff_dmg for the tick of a buff, is_shields set when the
// skill gave barrier rather than health, and in is_offcycle the flags
// telling whether the target was downed and which client wrote the event.
// Only the recording player and the squad members sharing their stats with
// it write events, each for the heals it dealt or received; the heals of
// everyone else are known only when one of them received them. A heal
// between two such players is written by both clients and the package
// merges the two records into one heal.
package healingstats
