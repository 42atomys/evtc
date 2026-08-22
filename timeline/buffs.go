package timeline

// Buff ids of the GW2 API for the boons, conditions and common effects
// every log carries. The skill table only names them in the language of
// the recording client, so these ids are the stable way to ask for them:
// tl.Buff(BuffMight), player.Stacks().OfBuff(BuffQuickness).
const (
	BuffMight        uint32 = 740
	BuffFury         uint32 = 725
	BuffQuickness    uint32 = 1187
	BuffAlacrity     uint32 = 30328
	BuffProtection   uint32 = 717
	BuffRegeneration uint32 = 718
	BuffVigor        uint32 = 726
	BuffSwiftness    uint32 = 719
	BuffStability    uint32 = 1122
	BuffResistance   uint32 = 26980
	BuffAegis        uint32 = 743
	BuffResolution   uint32 = 873

	BuffBleeding      uint32 = 736
	BuffBurning       uint32 = 737
	BuffConfusion     uint32 = 861
	BuffPoison        uint32 = 723
	BuffTorment       uint32 = 19426
	BuffBlinded       uint32 = 720
	BuffChilled       uint32 = 722
	BuffCrippled      uint32 = 721
	BuffFear          uint32 = 791
	BuffImmobilized   uint32 = 727
	BuffSlow          uint32 = 26766
	BuffTaunt         uint32 = 27705
	BuffWeakness      uint32 = 742
	BuffVulnerability uint32 = 738

	BuffSuperspeed uint32 = 5974
	BuffStealth    uint32 = 13017
	BuffRevealed   uint32 = 890
)

// Boons lists the twelve boons, Conditions the fourteen conditions, in the
// order of the constants above.
var (
	Boons = []uint32{BuffMight, BuffFury, BuffQuickness, BuffAlacrity, BuffProtection, BuffRegeneration, BuffVigor, BuffSwiftness, BuffStability, BuffResistance, BuffAegis, BuffResolution}

	Conditions = []uint32{BuffBleeding, BuffBurning, BuffConfusion, BuffPoison, BuffTorment, BuffBlinded, BuffChilled, BuffCrippled, BuffFear, BuffImmobilized, BuffSlow, BuffTaunt, BuffWeakness, BuffVulnerability}
)
