package timeline

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestSessionAndSquad(t *testing.T) {
	b := fixture()
	b.session(evtc.StateLanguage, 2)
	b.session(evtc.StateGWBuild, 170000)
	b.session(evtc.StateShardID, 1234)
	b.session(evtc.StateFractalScale, 100)
	b.session(evtc.StateRuleset, 3)
	b.instanceStart(1000, 500)
	b.arcBuild("arcdps 20260816.1234")
	b.tick(500, 25, 0)
	b.tick(1500, 50, 45)
	b.tick(2500, 75, 60)
	b.groundMarker(1000, 0, Vec3{1, 2, 3})
	b.groundMarker(2000, 0, Vec3{})
	b.groundMarker(2500, 3, Vec3{float32(math.Inf(1)), 0, 0})
	tag := GUID{0xAA, 0xBB, 0xCC}
	b.idToGUID(ContentMarker, 7, tag, 0)
	b.marker(1000, addrP1, 7, true)
	b.marker(2000, addrP2, 3, false)
	b.marker(3000, addrP1, 0, false)
	b.marker(4000, addrP2, 8, true)
	guild := GUID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	b.guild(1000, addrP1, guild)
	b.teamChange(1000, addrP1, 10, 0)
	b.teamChange(3000, addrP1, 11, 10)
	b.move(4500, addrP1, evtc.StatePosition, 1, 2, 3) // keeps the states of p1 known until 4.5 s
	b.marker(100, 0, 7, true)                         // noise from an unknown source
	l := b.build(10000)
	l.Events[len(l.Events)-1].DstAgent = 1 // the log ended by a map exit
	tl := mustBuild(t, l)
	p1, p2 := tl.characters[0], tl.characters[1]

	if tl.Language != LanguageFrench || tl.Language.String() != "French" || Language(1).String() != "Language(1)" || tl.GameBuild != 170000 || tl.ShardID != 1234 || tl.FractalScale != 100 {
		t.Errorf("session = %v %d %d %d", tl.Language, tl.GameBuild, tl.ShardID, tl.FractalScale)
	}
	if !tl.Ruleset.IsPvE() || !tl.Ruleset.IsWvW() || tl.Ruleset.IsPvP() || tl.Ruleset.String() != "PvE|WvW" || Ruleset(0).String() != "Ruleset(0)" || RulesetPvP.String() != "PvP" {
		t.Errorf("ruleset = %v", tl.Ruleset)
	}
	if tl.InstanceStart != tl.Start.Add(500*msec) || tl.ArcBuild != "arcdps 20260816.1234" || !tl.EndedByMapExit {
		t.Errorf("instance start %v arc build %q map exit %v", tl.InstanceStart, tl.ArcBuild, tl.EndedByMapExit)
	}
	if tl.Ping.Len() != 3 || held(tl.Ping.At(2*time.Second)) != 45 || held(tl.Ping.At(3*time.Second)) != 60 {
		t.Errorf("ping = %v", tl.Ping.Samples())
	}
	// The arrow placed at 1 s is removed at 2 s; the removal of a marker
	// never placed is dropped.
	if len(tl.GroundMarkers) != 1 || tl.GroundMarkers[0].Position != (Vec3{1, 2, 3}) || tl.GroundMarkers[0].Index != 0 || tl.GroundMarkers[0].Squad != SquadArrow || tl.GroundMarkers[0].Interval != NewInterval(time.Second, 2*time.Second) || !tl.GroundMarkers[0].Removed() {
		t.Errorf("ground markers = %+v", tl.GroundMarkers)
	}
	if len(p1.Markers) != 1 || p1.Markers[0].ID != 7 || !p1.Markers[0].Commander || p1.Markers[0].GUID != tag || p1.Markers[0].Agent != p1.Agent || p1.Markers[0].Interval != NewInterval(time.Second, 3*time.Second) || !p1.Markers[0].Removed() || p1.Markers[0].Remove.Value != 0 {
		t.Errorf("markers of p1 = %+v", p1.Markers)
	}
	// The markers of p2 are never removed: they end with its lifetime.
	if len(p2.Markers) != 2 || p2.Markers[0].GUID != (GUID{}) || p2.Markers[0].Interval != NewInterval(2*time.Second, 4*time.Second) || p2.Markers[0].Removed() || p2.Markers[1].Interval != At(4*time.Second) || tl.Commander() != p2.Player || len(tl.Unknown.Markers) != 0 {
		t.Errorf("markers of p2 = %+v, commander %v", p2.Markers, tl.Commander())
	}
	if tl.CommanderAt(2500*msec) != p1.Player || tl.CommanderAt(3*time.Second) != p1.Player || tl.CommanderAt(3500*msec) != nil || tl.CommanderAt(4*time.Second) != p2.Player || p1.IsCommanderAt(3001*msec) {
		t.Error("CommanderAt is wrong")
	}
	if p1.Player.Guild != guild || guild.String() != "0102030405060708090A0B0C0D0E0F10" || !p2.Player.Guild.IsZero() || p1.Player.Guild.IsZero() {
		t.Errorf("guild = %v", p1.Player.Guild)
	}
	if p1.Team.Len() != 3 || held(p1.Team.ValueAt(500*msec)) != 0 || held(p1.Team.ValueAt(2*time.Second)) != 10 || held(p1.Team.ValueAt(4*time.Second)) != 11 || p2.Team.Len() != 0 {
		t.Errorf("team = %v", p1.Team.All())
	}
	if tl.GUID(ContentMarker, 7) != tag || ContentMarker.String() != "Marker" || ContentKind(9).String() != "ContentKind(9)" {
		t.Error("GUID lookup is wrong")
	}
	if !tl.GUID(ContentSkill, 7).IsZero() {
		t.Error("a GUID of another kind matched")
	}
	if tl.PingAt(2*time.Second) != 45 || tl.PingAt(0) != 0 || p1.TeamAt(2*time.Second) != 10 || p2.TeamAt(2*time.Second) != 0 {
		t.Error("PingAt or TeamAt is wrong")
	}
	checkInvariants(t, tl)

	// Without a commander tag, no commander.
	if tl := mustBuild(t, fixture().build(1000)); tl.Commander() != nil || tl.EndedByMapExit || tl.Ping.Len() != 0 || tl.InstanceStart != (time.Time{}) {
		t.Error("an empty log reports session data")
	}
}

func TestRewardsMapsAndIntegrity(t *testing.T) {
	b := fixture()
	b.reward(1000, 42, 3)
	b.reward(2000, 43, 0)
	b.mapChange(1500, 1155, 1062, 4)
	b.integrity("buffer full")
	b.integrity("")
	b.integrity(strings.Repeat("x", 40)) // longer than the field: cut at 32 bytes
	b.integrity("caf\xe9")               // not printable
	tl := mustBuild(t, b.build(5000))

	if len(tl.Rewards) != 2 || tl.Rewards[0].ID != 42 || tl.Rewards[0].Kind != 3 || tl.Rewards[0].Time != time.Second || tl.Rewards[1].ID != 43 || tl.Rewards[1].Event == nil {
		t.Errorf("rewards = %+v", tl.Rewards)
	}
	if len(tl.MapChanges) != 1 || tl.MapChanges[0].From != 1062 || tl.MapChanges[0].To != 1155 || tl.MapChanges[0].Kind != 4 || tl.MapChanges[0].Time != 1500*msec || tl.MapChanges[0].Event == nil {
		t.Errorf("map changes = %+v", tl.MapChanges)
	}
	var msgs []string
	for _, m := range tl.Integrity {
		if m.Event == nil || m.Event.IsStateChange != evtc.StateIntegrity {
			t.Errorf("integrity message without its event: %+v", m)
		}
		msgs = append(msgs, m.Message)
	}
	if want := []string{"buffer full", "", strings.Repeat("x", 32), ""}; !slices.Equal(msgs, want) {
		t.Errorf("integrity = %q, want %q", msgs, want)
	}
	if tl.Events().Of(evtc.StateReward).Count() != 2 || tl.Unknown.Events().Of(evtc.StateReward).Count() != 0 || tl.Unknown.Events().Of(evtc.StateMapChange).Count() != 0 {
		t.Errorf("rewards or map changes were attached to an agent")
	}
	if tl2 := mustBuild(t, fixture().build(1000)); len(tl2.Rewards) != 0 || len(tl2.MapChanges) != 0 || len(tl2.Integrity) != 0 || len(tl2.Extensions) != 0 || tl2.ExtensionEvents().Count() != 0 {
		t.Errorf("empty log has extras: %+v", tl2)
	}
	checkInvariants(t, tl)
}
