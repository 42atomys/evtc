// Package combine merges the recordings several players made of one fight
// into one log.
//
// arcdps records what happens around the player who runs it: the agents
// further than 5,000 units are not in the log, nor what they did. A squad
// spread over a large arena holds the whole fight in the logs of several
// players, each with its own addresses, its own clock and its own view of
// the same agents. Logs puts them together:
//
//	log, err := combine.Files("mine.zevtc", "theirs.zevtc")
//	if err != nil {
//		return err
//	}
//	err = log.WriteFile("fight.zevtc")
//
// The combined log is an ordinary log: timeline.Build reads it, and every
// agent of the fight is in it once, whatever the number of recordings that
// held it. A strike, a cast or a buff application that several clients
// were told of is in it once as well.
//
// Clocks are set on the strikes the recordings share. The entries of the
// agent tables are joined into agents by instance id, species, account and
// the time they were held. Then every event kind follows one rule. The
// messages of the server are paired from a recording to the other. The
// samples and the states a client makes on its own, positions and health
// among them, come from one recording per agent at a time, the client of
// a player speaking for its own character. Events of extensions are paired
// on their payload.
//
// The logs must have been written by the same arcdps build, one that the
// timeline reads.
package combine
