package healingstats

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/42atomys/evtc/timeline"
)

// samplePath is the real log used by the integration tests; they are
// skipped when it is absent. Its recording player ran the addon with three
// squad members sharing their stats.
const samplePath = "../../tests_fixtures/sabetha-05-fd9b6f3a.zevtc"

// Players of the sample log, by their index in Stats.Players().
var (
	sampleRecorded = []int{0, 1, 7, 9} // the recording player first
	sampleHealer   = 4                 // top healer, without the addon
	sampleBarrier  = 9                 // target of the first heal, a barrier
)

func loadSample(tb testing.TB) *Stats {
	tb.Helper()
	if _, err := os.Stat(samplePath); err != nil {
		tb.Skipf("%s not available", samplePath)
	}
	tl, err := timeline.ParseFile(samplePath)
	if err != nil {
		tb.Fatal(err)
	}
	s := Of(tl)
	if s == nil {
		tb.Fatal("the sample log carries no healing stats")
	}
	return s
}

func TestSampleOverview(t *testing.T) {
	s := loadSample(t)
	tl := s.Timeline
	if s.Version != "2.18rc1" || s.Revision != 2 || s.Extension.Time <= 0 || s.Extension.Events().Count() != 2325 {
		t.Errorf("version %q revision %d, %d events", s.Version, s.Revision, s.Extension.Events().Count())
	}
	if s.Heals().Count() != 2185 || s.Merged != 140 || s.Heals().Count()+s.Merged != s.Extension.Events().Count() {
		t.Errorf("heals %d merged %d", s.Heals().Count(), s.Merged)
	}
	var names, want []string
	for _, p := range s.Recorded {
		names = append(names, p.Name)
	}
	for _, i := range sampleRecorded {
		want = append(want, s.players[i].Name)
	}
	if !slices.Equal(names, want) {
		t.Errorf("recorded = %v, want %v", names, want)
	}
	pov := s.Agent(tl.POV)
	if pov == nil || pov != s.Players().First() || !pov.Recorded || pov.Heals().Count() != 561 || pov.HealsTaken().Count() != 543 || pov.HealsCredited().Count() != 561 || pov.Heals().Healed() != 92624 {
		t.Errorf("pov = %v: %d heals, %d taken, %d healed", pov, pov.Heals().Count(), pov.HealsTaken().Count(), pov.Heals().Healed())
	}
	q := s.Heals()
	if q.Barrier().Count() != 889 || q.BarrierGiven() != 880974 || q.Downed().Count() != 34 || q.Self().Count() != 531 || q.Ticks().Count() != 788 || s.Unknown.Heals().Count() != 0 {
		t.Errorf("barrier %d for %d, downed %d, self %d, ticks %d, unknown %d", q.Barrier().Count(), q.BarrierGiven(), q.Downed().Count(), q.Self().Count(), q.Ticks().Count(), s.Unknown.Heals().Count())
	}
	if withCast := q.Where(func(h *Heal) bool { return h.Cast != nil }).Count(); withCast != 639 {
		t.Errorf("heals attributed to a cast = %d", withCast)
	}
	first := q.First()
	if first.Time != 796*time.Millisecond || first.Src.Name != "Mech de jade CJ-1" || first.Dst.Name != s.players[sampleBarrier].Name || first.Skill.Name != "Explosion de barrière" || first.Amount != 455 || !first.IsBarrier || first.Credited().Player == nil || first.Credited() == first.Src {
		t.Errorf("first heal = %+v credited to %v", first, first.Credited())
	}
	if last := q.Last(); last.Time != 315101*time.Millisecond {
		t.Errorf("last heal at %v", last.Time)
	}
	checkInvariants(t, s)
}

func TestSampleRankings(t *testing.T) {
	s := loadSample(t)
	tl := s.Timeline
	healer := s.players[sampleHealer]
	healers := s.Heals().Healing().PerAgent()
	if len(healers) < 5 || healers[0].Agent.Name != healer.Name || healers[0].Heals.Count() != 256 || healers[0].Heals.Healed() != 245625 {
		t.Errorf("%d healers, top %v with %d heals for %d", len(healers), healers[0].Agent, healers[0].Heals.Count(), healers[0].Heals.Healed())
	}
	if hps := healers[0].Heals.HPS(tl.Interval()); hps < 769 || hps > 771 {
		t.Errorf("top healer HPS = %v", hps)
	}
	if healer.HealsCredited().Healing().Healed() != 245625 || healer.Recorded {
		t.Errorf("top healer healed %d, recorded %v", healer.HealsCredited().Healing().Healed(), healer.Recorded)
	}
	skills := s.Heals().PerSkill()
	if len(skills) == 0 || skills[0].Skill.Name != "Chant de récupération" || skills[0].Heals.Count() != 82 || skills[0].Heals.Amount() != 313877 {
		t.Errorf("top skill = %v", skills[0])
	}
	// A heal written by both clients keeps the record of the recording
	// player and the copy of the squad member.
	merged := s.Heals().Where(func(h *Heal) bool { return h.PeerEvent != nil })
	if merged.Count() != 140 {
		t.Fatalf("merged heals = %d", merged.Count())
	}
	pov := tl.POV.Ref()
	for h := range merged.Seq() {
		local := (h.Event.IsOffcycle&flagFromSrc != 0 && credited(h.Src.Agent) == pov) || (h.Event.IsOffcycle&flagFromDst != 0 && credited(h.Dst.Agent) == pov)
		peer := (h.PeerEvent.IsOffcycle&flagFromSrc != 0 && credited(h.Src.Agent) == pov) || (h.PeerEvent.IsOffcycle&flagFromDst != 0 && credited(h.Dst.Agent) == pov)
		// The record of the recording player's client is kept; between two
		// squad members sharing their stats, the earlier record is.
		if peer || !h.SrcRecorded || !h.DstRecorded || (!local && h.Event.Time > h.PeerEvent.Time) {
			t.Errorf("merged heal at %v keeps the wrong record: %+v", h.Time, h)
		}
	}
}

func BenchmarkDecodeSample(b *testing.B) {
	s := loadSample(b)
	x := s.Extension
	b.ReportAllocs()
	for b.Loop() {
		build(x)
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(x.Events().Count()), "ns/event")
}

func BenchmarkHealsFiltered(b *testing.B) {
	s := loadSample(b)
	pov := s.Agent(s.Timeline.POV)
	b.ReportAllocs()
	for b.Loop() {
		pov.HealsTaken().Healing().Others().Healed()
	}
}
