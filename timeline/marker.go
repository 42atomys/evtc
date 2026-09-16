package timeline

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/42atomys/evtc"
)

// SquadMarker is one of the eight markers a commander puts on the ground
// or over an agent.
type SquadMarker uint8

const (
	// SquadNone is not a squad marker: a commander tag, a marker of an
	// encounter mechanic, or nothing.
	SquadNone SquadMarker = iota
	// SquadArrow is the arrow, the first marker of the squad marker menu.
	SquadArrow
	// SquadCircle is the circle.
	SquadCircle
	// SquadHeart is the heart.
	SquadHeart
	// SquadSquare is the square.
	SquadSquare
	// SquadStar is the star.
	SquadStar
	// SquadSpiral is the spiral, which Elite Insights names swirl.
	SquadSpiral
	// SquadTriangle is the triangle.
	SquadTriangle
	// SquadX is the cross.
	SquadX
)

var squadNames = []string{"None", "Arrow", "Circle", "Heart", "Square", "Star", "Spiral", "Triangle", "X"}

// String returns the marker name.
func (s SquadMarker) String() string { return enumString(squadNames, "SquadMarker", int(s)) }

// TagColor is the colour of a commander tag.
type TagColor uint8

// Colours of a commander tag, in the order of the game.
const (
	// TagNone is not a commander tag.
	TagNone TagColor = iota
	TagRed
	TagOrange
	TagYellow
	TagGreen
	TagCyan
	TagBlue
	TagPurple
	TagPink
	TagWhite
)

var tagNames = []string{"None", "Red", "Orange", "Yellow", "Green", "Cyan", "Blue", "Purple", "Pink", "White"}

// String returns the colour name.
func (c TagColor) String() string { return enumString(tagNames, "TagColor", int(c)) }

// Content GUIDs of the squad markers and of the commander tags. A marker
// id changes with game builds; the GUID names the content itself and is
// the stable way to recognize a marker (Marker.GUID).
var (
	MarkerArrow    = mustGUID("C3A56F1E045E3848B07CBAC5BBDD2C32")
	MarkerCircle   = mustGUID("73C880AE431C9F4D8A5972ACF7066F4E")
	MarkerHeart    = mustGUID("185008E2437B184D8FDAD647DD972D9F")
	MarkerSquare   = mustGUID("6E5997457B3F6A45B984C613806FA72A")
	MarkerStar     = mustGUID("5140125657C6084D94226C8EC0216649")
	MarkerSpiral   = mustGUID("EBBE113AE2E53F4E96F3E92FB1353ECE")
	MarkerTriangle = mustGUID("46EBC4397F8A3740B900333B591F6183")
	MarkerX        = mustGUID("8BDCF5C47F8A8340A251F102AF3B5905")

	CommanderTagRed    = mustGUID("4242F370667CE54EB3BF22BE8D06F986")
	CommanderTagOrange = mustGUID("E57AAE9EE7FC5D458B0CF16BE4B096BF")
	CommanderTagYellow = mustGUID("AF9442A290C6214596E0B339EB3BDE92")
	CommanderTagGreen  = mustGUID("74AD480E531F4740A407879976C8CA91")
	CommanderTagCyan   = mustGUID("96F4AB5CDEC5294388375C7A03AB7614")
	CommanderTagBlue   = mustGUID("AE714FC5E4EA464C8961CD78E86F9291")
	CommanderTagPurple = mustGUID("1993FADB6FB70E4383A223A54D311F7D")
	CommanderTagPink   = mustGUID("E911D8C0EF2FDF4D8D252E5FB1283C62")
	CommanderTagWhite  = mustGUID("A59678CDFB5732439D7FCBF58D8BCEC3")

	CatmanderTagRed    = mustGUID("CA76AB023593B0448F692FE29DF03D17")
	CatmanderTagOrange = mustGUID("9FDF03E9BA09A2458C1EDDA4D81BC34D")
	CatmanderTagYellow = mustGUID("6BCE90E99016B448969EB317784A8334")
	CatmanderTagGreen  = mustGUID("2CA226E07262C743BA193ACF6F9D0AF6")
	CatmanderTagCyan   = mustGUID("A8072D65CE35924BABBAC831B12019D7")
	CatmanderTagBlue   = mustGUID("9B94F0FD616E7F4AA58EFDC8C59FB689")
	CatmanderTagPurple = mustGUID("7224A4AF710E4243BFE032629E17CA6E")
	CatmanderTagPink   = mustGUID("4387BE6146D43246AA7B333168EA58EA")
	CatmanderTagWhite  = mustGUID("A0B0EC076BC83B40A293C1CDEC4A7DE7")
)

// The GUID tables indexed by SquadMarker and TagColor.
var (
	squadGUIDs     = [...]GUID{SquadArrow: MarkerArrow, SquadCircle: MarkerCircle, SquadHeart: MarkerHeart, SquadSquare: MarkerSquare, SquadStar: MarkerStar, SquadSpiral: MarkerSpiral, SquadTriangle: MarkerTriangle, SquadX: MarkerX}
	tagGUIDs       = [...]GUID{TagRed: CommanderTagRed, TagOrange: CommanderTagOrange, TagYellow: CommanderTagYellow, TagGreen: CommanderTagGreen, TagCyan: CommanderTagCyan, TagBlue: CommanderTagBlue, TagPurple: CommanderTagPurple, TagPink: CommanderTagPink, TagWhite: CommanderTagWhite}
	catmanderGUIDs = [...]GUID{TagRed: CatmanderTagRed, TagOrange: CatmanderTagOrange, TagYellow: CatmanderTagYellow, TagGreen: CatmanderTagGreen, TagCyan: CatmanderTagCyan, TagBlue: CatmanderTagBlue, TagPurple: CatmanderTagPurple, TagPink: CatmanderTagPink, TagWhite: CatmanderTagWhite}
)

// ParseGUID parses the 32 hexadecimal digits of a content GUID, in the
// byte order of the log as GUID.String prints it.
func ParseGUID(s string) (GUID, error) {
	var g GUID
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(g) {
		return GUID{}, fmt.Errorf("timeline: invalid GUID %q", s)
	}
	copy(g[:], b)
	return g, nil
}

// mustGUID parses a GUID literal of the tables above.
func mustGUID(s string) GUID {
	g, err := ParseGUID(s)
	if err != nil {
		panic(err)
	}
	return g
}

// squadOf returns the squad marker with the given GUID, SquadNone when the
// GUID names none.
func squadOf(g GUID) SquadMarker {
	for s := SquadArrow; s <= SquadX; s++ {
		if squadGUIDs[s] == g {
			return s
		}
	}
	return SquadNone
}

// squadOfIndex returns the squad marker at a ground marker index, in the
// order of the squad marker menu; arcdps only documents that 0 is the
// arrow.
func squadOfIndex(i uint32) SquadMarker {
	if i < uint32(SquadX) {
		return SquadMarker(i + 1)
	}
	return SquadNone
}

// tagOf returns the colour of the commander tag with the given GUID and
// whether it is a catmander tag, TagNone when the GUID names no tag.
func tagOf(g GUID) (TagColor, bool) {
	for c := TagRed; c <= TagWhite; c++ {
		if tagGUIDs[c] == g {
			return c, false
		}
		if catmanderGUIDs[c] == g {
			return c, true
		}
	}
	return TagNone, false
}

// Marker is a marker worn by an agent: a commander tag, a squad marker
// (arrow, circle, heart...) or the marker of an encounter mechanic, told
// apart by the GUID. It lasts from its first application to the removal of
// the markers of the agent or to the end of the lifetime of the agent;
// arcdps may write a worn marker again, which is the same marker. A marker
// has no position of its own: it is drawn over the agent, at
// Agent.PositionAt.
//
// The exported fields are read-only after Build.
type Marker struct {
	// ID is the marker definition id, which changes with game builds.
	ID uint32
	// GUID is the content GUID of the marker, zero when the log carries no
	// association for the id.
	GUID GUID
	// Squad is the squad marker the GUID names, SquadNone for a tag or a
	// mechanic marker.
	Squad SquadMarker
	// Commander is set for a commander or catmander tag: arcdps flags one
	// of its events as such, or the GUID is one of a tag.
	Commander bool
	// Tag is the colour of the tag, TagNone for any other marker.
	Tag TagColor
	// Catmander is set when the tag is a catmander tag.
	Catmander bool
	// Agent is the agent wearing the marker.
	Agent *Agent
	// Interval runs from the application of the marker to its end.
	Interval Interval
	// Event is the first application event of the marker.
	Event *evtc.Event
	// Remove is the event that removed the markers of the agent, nil when
	// the marker ended with the lifetime of the agent or the log.
	Remove *evtc.Event
}

// Removed reports whether the log holds the removal of the marker.
func (m *Marker) Removed() bool { return m.Remove != nil }

// GroundMarker is a placement of a squad marker on the ground, from the
// placement to the removal of the marker or to its next placement
// elsewhere.
//
// The exported fields are read-only after Build.
type GroundMarker struct {
	// Index is the marker index as logged, 0 for the arrow.
	Index uint32
	// Squad is the squad marker at that index.
	Squad SquadMarker
	// Position is the position of the marker.
	Position Vec3
	// Interval runs from the placement to the removal of the marker, to its
	// next placement or to the end of the log.
	Interval Interval
	// Event is the placement event.
	Event *evtc.Event
	// Remove is the removal event, nil when the marker was moved instead or
	// was still on the ground at the end of the log.
	Remove *evtc.Event
}

// Removed reports whether the log holds the removal of the marker.
func (gm *GroundMarker) Removed() bool { return gm.Remove != nil }

// Commander returns the player who wore a commander tag last, nil when the
// log holds none. CommanderAt tells who wore one at a given time.
func (tl *Timeline) Commander() *Player {
	var best *Player
	var at time.Duration
	for _, p := range tl.players {
		for _, m := range p.Markers {
			if m.Commander && (best == nil || m.Interval.Start >= at) {
				best, at = p, m.Interval.Start
			}
		}
	}
	return best
}

// CommanderAt returns the player wearing a commander tag at t, the first
// in table order when several do, nil when none does.
func (tl *Timeline) CommanderAt(t time.Duration) *Player {
	for _, p := range tl.players {
		if p.IsCommanderAt(t) {
			return p
		}
	}
	return nil
}

// GroundMarkerAt returns the placement of the squad marker s that covers t,
// nil when the marker was not on the ground at t.
func (tl *Timeline) GroundMarkerAt(s SquadMarker, t time.Duration) *GroundMarker {
	var found *GroundMarker
	for _, gm := range tl.GroundMarkers {
		if gm.Interval.Start > t {
			break
		}
		if gm.Squad == s && gm.Interval.Contains(t) {
			found = gm
		}
	}
	return found
}

// SquadMarkerAt returns the squad marker worn by the agent at t, SquadNone
// when it wore none.
func (a *Agent) SquadMarkerAt(t time.Duration) SquadMarker {
	s := SquadNone
	for _, m := range a.Markers {
		if m.Interval.Start > t {
			break
		}
		if m.Squad != SquadNone && m.Interval.Contains(t) {
			s = m.Squad
		}
	}
	return s
}

// IsCommanderAt reports whether the agent wore a commander tag at t.
func (a *Agent) IsCommanderAt(t time.Duration) bool {
	for _, m := range a.Markers {
		if m.Interval.Start > t {
			break
		}
		if m.Commander && m.Interval.Contains(t) {
			return true
		}
	}
	return false
}
