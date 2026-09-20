package timeline

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc"
)

func TestMarkers(t *testing.T) {
	b := fixture()
	b.idToGUID(ContentMarker, 10, MarkerHeart, 0)
	b.idToGUID(ContentMarker, 11, MarkerStar, 0)
	b.idToGUID(ContentMarker, 20, CommanderTagRed, 0)
	b.idToGUID(ContentMarker, 21, CatmanderTagBlue, 0)
	mechanic := GUID{1, 2, 3}
	b.idToGUID(ContentMarker, 30, mechanic, 0)
	b.marker(100, 0, 0, false)          // noise from an unknown source
	b.marker(500, addrP1, 0, false)     // a removal with nothing to remove
	b.marker(1000, addrP1, 20, false)   // a tag is known by its GUID, flagged or not
	b.marker(1500, addrP2, 10, false)   // the heart on p2
	b.marker(2000, addrP1, 20, true)    // the tag written again while worn is the same marker
	b.marker(2500, addrP2, 0, false)    // the heart removed
	b.marker(3000, addrP2, 11, false)   // the star on p2
	b.marker(3000, addrP2, 30, false)   // a mechanic marker next to it
	b.marker(3200, addrP2, 31, false)   // a marker without GUID
	b.marker(3400, addrP2, 31, true)    // flagged as a tag only when written again
	b.marker(3500, addrBoss, 30, false) // and on the boss, never removed
	b.marker(4000, addrP1, 21, true)    // a catmander tag next to the commander tag
	b.marker(4500, addrP1, 0, false)    // both removed
	b.marker(5000, addrP2, 0, false)    // the star and the mechanic marker removed
	b.health(4000, addrBoss, 50)
	b.move(6000, addrP1, evtc.StatePosition, 1, 2, 3)
	b.move(6000, addrP2, evtc.StatePosition, 1, 2, 3)
	tl := mustBuild(t, b.build(8000))
	p1, p2, boss := tl.players[0], tl.players[1], tl.Boss()

	if len(p1.Markers) != 2 || len(p2.Markers) != 4 || len(boss.Markers) != 1 || len(tl.Unknown.Markers) != 0 {
		t.Fatalf("markers: p1 %d, p2 %d, boss %d, unknown %d", len(p1.Markers), len(p2.Markers), len(boss.Markers), len(tl.Unknown.Markers))
	}
	tag, cat := p1.Markers[0], p1.Markers[1]
	if tag.ID != 20 || tag.GUID != CommanderTagRed || !tag.Commander || tag.Tag != TagRed || tag.Catmander || tag.Squad != SquadNone || tag.Interval != NewInterval(time.Second, 4500*msec) || !tag.Removed() || tag.Remove.Value != 0 || tag.Agent != p1.Agent || tag.Event == nil || tag.Event.Buff != 0 {
		t.Errorf("tag = %+v", tag)
	}
	if cat.Tag != TagBlue || !cat.Catmander || !cat.Commander || cat.Interval != NewInterval(4*time.Second, 4500*msec) || cat.Remove != tag.Remove {
		t.Errorf("catmander tag = %+v", cat)
	}
	heart, star, mech, late := p2.Markers[0], p2.Markers[1], p2.Markers[2], p2.Markers[3]
	if heart.Squad != SquadHeart || heart.Commander || heart.Tag != TagNone || heart.Interval != NewInterval(1500*msec, 2500*msec) || !heart.Removed() {
		t.Errorf("heart = %+v", heart)
	}
	if star.Squad != SquadStar || star.Interval != NewInterval(3*time.Second, 5*time.Second) || !star.Removed() || mech.Squad != SquadNone || mech.GUID != mechanic || mech.Interval != star.Interval || mech.Remove != star.Remove {
		t.Errorf("star = %+v, mechanic = %+v", star, mech)
	}
	if late.ID != 31 || !late.GUID.IsZero() || !late.Commander || late.Tag != TagNone || late.Interval != NewInterval(3200*msec, 5*time.Second) || late.Remove != star.Remove || late.Event.Buff != 0 {
		t.Errorf("late tag = %+v", late)
	}
	// The marker of the boss ends with its lifetime: no removal was logged.
	if m := boss.Markers[0]; m.Interval != NewInterval(3500*msec, 4*time.Second) || m.Removed() || m.Agent != boss.Agent {
		t.Errorf("boss marker = %+v", m)
	}
	if p2.SquadMarkerAt(2*time.Second) != SquadHeart || p2.SquadMarkerAt(2500*msec) != SquadHeart || p2.SquadMarkerAt(2600*msec) != SquadNone || p2.SquadMarkerAt(4*time.Second) != SquadStar || p1.SquadMarkerAt(2*time.Second) != SquadNone || p2.SquadMarkerAt(-time.Second) != SquadNone {
		t.Error("SquadMarkerAt is wrong")
	}
	if !p1.IsCommanderAt(1500*msec) || !p1.IsCommanderAt(2*time.Second) || !p1.IsCommanderAt(4500*msec) || p1.IsCommanderAt(4600*msec) || p1.IsCommanderAt(500*msec) || p2.IsCommanderAt(3*time.Second) || !p2.IsCommanderAt(3500*msec) {
		t.Error("IsCommanderAt is wrong")
	}
	// Two players wear a tag at 3.5 s: the first in table order answers.
	if tl.Commander() != p1 || tl.CommanderAt(3500*msec) != p1 || tl.CommanderAt(4800*msec) != p2 || tl.CommanderAt(5*time.Second+msec) != nil || tl.CommanderAt(999*msec) != nil {
		t.Errorf("commander %v, at 3.5s %v", tl.Commander(), tl.CommanderAt(3500*msec))
	}
	if tl.Events().Of(evtc.StateMarker).Count() != 14 || tl.Unknown.Events().Of(evtc.StateMarker).Count() != 1 {
		t.Errorf("marker events = %d", tl.Events().Of(evtc.StateMarker).Count())
	}
	checkInvariants(t, tl)
}

func TestMustGUIDPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("mustGUID accepted a bad literal")
		}
	}()
	MustParseGUID("bad")
}

func TestGroundMarkers(t *testing.T) {
	b := fixture()
	b.groundMarker(500, 2, Vec3{})                             // a removal with nothing placed
	b.groundMarker(1000, 2, Vec3{100, 200, 0})                 // the heart placed
	b.groundMarker(2000, 2, Vec3{300, 400, 0})                 // the heart moved
	b.groundMarker(2500, 0, Vec3{1, 1, 1})                     // the arrow placed
	b.groundMarker(3000, 2, Vec3{float32(math.Inf(-1)), 0, 0}) // the heart removed
	b.groundMarker(3500, 9, Vec3{5, 5, 5})                     // an index arcdps does not document
	tl := mustBuild(t, b.build(5000))
	if len(tl.GroundMarkers) != 4 {
		t.Fatalf("ground markers = %+v", tl.GroundMarkers)
	}
	heart, moved, arrow, other := tl.GroundMarkers[0], tl.GroundMarkers[1], tl.GroundMarkers[2], tl.GroundMarkers[3]
	if heart.Squad != SquadHeart || heart.Index != 2 || heart.Position != (Vec3{100, 200, 0}) || heart.Interval != NewInterval(time.Second, 2*time.Second) || heart.Removed() || heart.Event == nil {
		t.Errorf("heart = %+v", heart)
	}
	if moved.Position != (Vec3{300, 400, 0}) || moved.Interval != NewInterval(2*time.Second, 3*time.Second) || !moved.Removed() || moved.Remove.SkillID != 2 {
		t.Errorf("moved heart = %+v", moved)
	}
	if arrow.Squad != SquadArrow || arrow.Interval != NewInterval(2500*msec, 5*time.Second) || arrow.Removed() || other.Squad != SquadNone || other.Index != 9 || other.Interval != NewInterval(3500*msec, 5*time.Second) {
		t.Errorf("arrow = %+v, other = %+v", arrow, other)
	}
	if tl.GroundMarkerAt(SquadHeart, 1500*msec) != heart || tl.GroundMarkerAt(SquadHeart, 2*time.Second) != moved || tl.GroundMarkerAt(SquadHeart, 3001*msec) != nil || tl.GroundMarkerAt(SquadArrow, 4*time.Second) != arrow || tl.GroundMarkerAt(SquadStar, 4*time.Second) != nil || tl.GroundMarkerAt(SquadHeart, 999*msec) != nil {
		t.Error("GroundMarkerAt is wrong")
	}
	if tl.Events().Of(evtc.StateSquadMarkerGround).Count() != 6 {
		t.Errorf("ground marker events = %d", tl.Events().Of(evtc.StateSquadMarkerGround).Count())
	}
	checkInvariants(t, tl)
}

func TestMarkerGUIDs(t *testing.T) {
	if g, err := ParseGUID("c3a56f1e045e3848b07cbac5bbdd2c32"); err != nil || g != MarkerArrow || g.String() != "C3A56F1E045E3848B07CBAC5BBDD2C32" {
		t.Errorf("ParseGUID = %v, %v", g, err)
	}
	for _, s := range []string{"", "C3A56F1E", "zz56F1E045E3848B07CBAC5BBDD2C32", "C3A56F1E045E3848B07CBAC5BBDD2C3200"} {
		if _, err := ParseGUID(s); err == nil {
			t.Errorf("ParseGUID(%q) accepted", s)
		}
	}
	if squadOf(MarkerX) != SquadX || squadOf(CommanderTagRed) != SquadNone || squadOf(GUID{}) != SquadNone || squadOfIndex(0) != SquadArrow || squadOfIndex(7) != SquadX || squadOfIndex(8) != SquadNone {
		t.Error("squad lookups are wrong")
	}
	if c, cat := tagOf(CatmanderTagWhite); c != TagWhite || !cat {
		t.Errorf("tagOf(catmander white) = %v %v", c, cat)
	}
	if c, cat := tagOf(CommanderTagGreen); c != TagGreen || cat {
		t.Errorf("tagOf(commander green) = %v %v", c, cat)
	}
	if c, cat := tagOf(MarkerHeart); c != TagNone || cat {
		t.Errorf("tagOf(heart) = %v %v", c, cat)
	}
	if SquadHeart.String() != "Heart" || SquadNone.String() != "None" || SquadMarker(9).String() != "SquadMarker(9)" || TagRed.String() != "Red" || TagColor(10).String() != "TagColor(10)" {
		t.Error("names are wrong")
	}
	// Every GUID of the tables is set once.
	seen := map[GUID]bool{}
	for _, g := range slices.Concat(squadGUIDs[1:], tagGUIDs[1:], catmanderGUIDs[1:]) {
		if g.IsZero() || seen[g] {
			t.Errorf("GUID %v is zero or repeated", g)
		}
		seen[g] = true
	}
	if len(seen) != 26 {
		t.Errorf("%d GUIDs in the tables", len(seen))
	}
}
