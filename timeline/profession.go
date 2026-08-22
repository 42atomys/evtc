package timeline

import "fmt"

// Profession is the profession of a player, with the ids of the GW2 API,
// which the arcdps agent table uses as well.
type Profession uint32

// Professions, in the order of their GW2 API ids.
const (
	ProfessionUnknown Profession = iota // Unknown, or not a player.
	ProfessionGuardian
	ProfessionWarrior
	ProfessionEngineer
	ProfessionRanger
	ProfessionThief
	ProfessionElementalist
	ProfessionMesmer
	ProfessionNecromancer
	ProfessionRevenant
)

var professionNames = []string{"Unknown", "Guardian", "Warrior", "Engineer", "Ranger", "Thief", "Elementalist", "Mesmer", "Necromancer", "Revenant"}

// String returns the profession name.
func (p Profession) String() string { return enumString(professionNames, "Profession", int(p)) }

// EliteSpec is the elite specialization of a player, with the ids of the
// GW2 API. EliteNone is a core build.
type EliteSpec uint32

const (
	EliteNone         EliteSpec = 0  // Core build.
	EliteDruid        EliteSpec = 5  // Ranger.
	EliteDaredevil    EliteSpec = 7  // Thief.
	EliteBerserker    EliteSpec = 18 // Warrior.
	EliteDragonhunter EliteSpec = 27 // Guardian.
	EliteReaper       EliteSpec = 34 // Necromancer.
	EliteChronomancer EliteSpec = 40 // Mesmer.
	EliteScrapper     EliteSpec = 43 // Engineer.
	EliteTempest      EliteSpec = 48 // Elementalist.
	EliteHerald       EliteSpec = 52 // Revenant.
	EliteSoulbeast    EliteSpec = 55 // Ranger.
	EliteWeaver       EliteSpec = 56 // Elementalist.
	EliteHolosmith    EliteSpec = 57 // Engineer.
	EliteDeadeye      EliteSpec = 58 // Thief.
	EliteMirage       EliteSpec = 59 // Mesmer.
	EliteScourge      EliteSpec = 60 // Necromancer.
	EliteSpellbreaker EliteSpec = 61 // Warrior.
	EliteFirebrand    EliteSpec = 62 // Guardian.
	EliteRenegade     EliteSpec = 63 // Revenant.
	EliteHarbinger    EliteSpec = 64 // Necromancer.
	EliteWillbender   EliteSpec = 65 // Guardian.
	EliteVirtuoso     EliteSpec = 66 // Mesmer.
	EliteCatalyst     EliteSpec = 67 // Elementalist.
	EliteBladesworn   EliteSpec = 68 // Warrior.
	EliteVindicator   EliteSpec = 69 // Revenant.
	EliteMechanist    EliteSpec = 70 // Engineer.
	EliteSpecter      EliteSpec = 71 // Thief.
	EliteUntamed      EliteSpec = 72 // Ranger.
	EliteTroubadour   EliteSpec = 73 // Mesmer.
	EliteParagon      EliteSpec = 74 // Warrior.
	EliteAmalgam      EliteSpec = 75 // Engineer.
	EliteRitualist    EliteSpec = 76 // Necromancer.
	EliteAntiquary    EliteSpec = 77 // Thief.
	EliteGaleshot     EliteSpec = 78 // Ranger.
	EliteConduit      EliteSpec = 79 // Revenant.
	EliteEvoker       EliteSpec = 80 // Elementalist.
	EliteLuminary     EliteSpec = 81 // Guardian.
)

// eliteInfo names an elite specialization and ties it to its profession.
type eliteInfo struct {
	name string
	prof Profession
}

// eliteSpecs is indexed by specialization id; ids that are not elite
// specializations hold an empty entry.
var eliteSpecs = [...]eliteInfo{
	EliteDruid:        {"Druid", ProfessionRanger},
	EliteDaredevil:    {"Daredevil", ProfessionThief},
	EliteBerserker:    {"Berserker", ProfessionWarrior},
	EliteDragonhunter: {"Dragonhunter", ProfessionGuardian},
	EliteReaper:       {"Reaper", ProfessionNecromancer},
	EliteChronomancer: {"Chronomancer", ProfessionMesmer},
	EliteScrapper:     {"Scrapper", ProfessionEngineer},
	EliteTempest:      {"Tempest", ProfessionElementalist},
	EliteHerald:       {"Herald", ProfessionRevenant},
	EliteSoulbeast:    {"Soulbeast", ProfessionRanger},
	EliteWeaver:       {"Weaver", ProfessionElementalist},
	EliteHolosmith:    {"Holosmith", ProfessionEngineer},
	EliteDeadeye:      {"Deadeye", ProfessionThief},
	EliteMirage:       {"Mirage", ProfessionMesmer},
	EliteScourge:      {"Scourge", ProfessionNecromancer},
	EliteSpellbreaker: {"Spellbreaker", ProfessionWarrior},
	EliteFirebrand:    {"Firebrand", ProfessionGuardian},
	EliteRenegade:     {"Renegade", ProfessionRevenant},
	EliteHarbinger:    {"Harbinger", ProfessionNecromancer},
	EliteWillbender:   {"Willbender", ProfessionGuardian},
	EliteVirtuoso:     {"Virtuoso", ProfessionMesmer},
	EliteCatalyst:     {"Catalyst", ProfessionElementalist},
	EliteBladesworn:   {"Bladesworn", ProfessionWarrior},
	EliteVindicator:   {"Vindicator", ProfessionRevenant},
	EliteMechanist:    {"Mechanist", ProfessionEngineer},
	EliteSpecter:      {"Specter", ProfessionThief},
	EliteUntamed:      {"Untamed", ProfessionRanger},
	EliteTroubadour:   {"Troubadour", ProfessionMesmer},
	EliteParagon:      {"Paragon", ProfessionWarrior},
	EliteAmalgam:      {"Amalgam", ProfessionEngineer},
	EliteRitualist:    {"Ritualist", ProfessionNecromancer},
	EliteAntiquary:    {"Antiquary", ProfessionThief},
	EliteGaleshot:     {"Galeshot", ProfessionRanger},
	EliteConduit:      {"Conduit", ProfessionRevenant},
	EliteEvoker:       {"Evoker", ProfessionElementalist},
	EliteLuminary:     {"Luminary", ProfessionGuardian},
}

// info returns the table entry of the specialization, empty when the id is
// not a known elite specialization.
func (e EliteSpec) info() eliteInfo {
	if uint64(e) < uint64(len(eliteSpecs)) {
		return eliteSpecs[e]
	}
	return eliteInfo{}
}

// String returns the specialization name, "None" for a core build.
func (e EliteSpec) String() string {
	if e == EliteNone {
		return "None"
	}
	if info := e.info(); info.name != "" {
		return info.name
	}
	return fmt.Sprintf("EliteSpec(%d)", uint32(e))
}

// Profession returns the profession the specialization belongs to,
// ProfessionUnknown for a core build or an unknown id.
func (e EliteSpec) Profession() Profession { return e.info().prof }

// Spec returns the name of the elite specialization of the player, or the
// name of the profession for a core build or an unknown specialization.
func (p *Player) Spec() string {
	if info := p.EliteSpec.info(); info.name != "" {
		return info.name
	}
	return p.Profession.String()
}
