package healingstats

import (
	"time"

	"github.com/42atomys/evtc/timeline"
)

// Signature is the extension signature of the healing stats addon.
const Signature = timeline.ExtensionHealingStats

// SupportedRevision is the revision of the log format of the addon this
// package was written for. Logs of other revisions are decoded the same
// way; Stats.Revision tells which one a log carries.
const SupportedRevision = 2

// PeerWindow is how far apart the two records of one heal may be when the
// clients of both parties wrote it: the record of the recording player's
// own client comes first and the copy shared by the squad member follows
// within a few hundred milliseconds.
const PeerWindow = 500 * time.Millisecond

// Stats is the healing graph of a log: every heal and barrier the healing
// stats addon wrote, linked to the agents, skills and casts of the
// timeline. Of returns it for a timeline. Every method accepts a nil
// receiver, which stands for a log without the addon, and answers as for
// an empty one.
//
// The exported fields are read-only once the timeline is built.
type Stats struct {
	// Timeline is the timeline the stats are built on.
	Timeline *timeline.Timeline
	// Extension is the registration of the addon in the log.
	Extension *timeline.Extension
	// Version is the version of the addon that wrote the log, as text.
	Version string
	// Revision is the revision of the log format the addon wrote, from its
	// registration event; SupportedRevision is the one this package
	// decodes.
	Revision int
	// Unknown is the node of the Unknown sentinel of the timeline: the
	// source of the heals whose source the log does not know.
	Unknown *Agent
	// Recorded lists the players whose client wrote heals into the log:
	// the recording player and the squad members sharing their stats with
	// it, in table order. Every heal they dealt or received is in the log;
	// of everyone else, only the heals exchanged with one of them are.
	Recorded []*Agent
	// Merged is the number of heals written by both the client of their
	// source and the client of their destination, whose two records were
	// merged into one heal.
	Merged int

	agents  []*Agent
	players []*Agent
	heals   []*Heal
	byAgent map[*timeline.Agent]*Agent
}

// Of returns the healing stats decoded for a timeline, nil when the log
// carries no registration of the addon. Importing this package registers
// its decoder before any timeline is built, so the stats of a log that has
// the addon are always there.
func Of(tl *timeline.Timeline) *Stats {
	if tl == nil {
		return nil
	}
	x := tl.Extension(Signature)
	if x == nil {
		return nil
	}
	s, _ := x.Decoded.(*Stats)
	return s
}

// decoder is the timeline.ExtensionDecoder of the addon.
type decoder struct{}

// Signature returns the signature of the addon.
func (decoder) Signature() uint32 { return Signature }

// Decode builds the stats of the extension.
func (decoder) Decode(x *timeline.Extension) any { return build(x) }

func init() { timeline.RegisterExtension(decoder{}) }

// Heals returns every heal of the log, in time order.
func (s *Stats) Heals() Heals {
	if s == nil {
		return Heals{}
	}
	return Heals{timeline.From(s.heals)}
}

// Agents returns one node per agent of the timeline, in the order of
// Timeline.Agents.
func (s *Stats) Agents() Agents {
	if s == nil {
		return Agents{}
	}
	return Agents{timeline.From(s.agents)}
}

// Players returns the nodes of the players, in the order of
// Timeline.Players.
func (s *Stats) Players() Agents {
	if s == nil {
		return Agents{}
	}
	return Agents{timeline.From(s.players)}
}

// Agent returns the node of an agent of the timeline, nil for a nil
// entity or an agent of another timeline.
func (s *Stats) Agent(e timeline.Entity) *Agent {
	if s == nil {
		return nil
	}
	return s.byAgent[ref(e)]
}

// ref returns the timeline agent of an entity, nil for a nil entity.
func ref(e timeline.Entity) *timeline.Agent {
	if e == nil {
		return nil
	}
	return e.Ref()
}
