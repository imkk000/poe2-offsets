package gamestate

import "encoding/binary"

const (
	statsItemsPtrOff       = 0x160
	statsBuffsPtrOff       = 0x198
	statsBaseResistsPtrOff = 0x1C8
	statsVecBeginOff       = 0xF8
	statsVecEndOff         = 0x100

	keyHPMax     = 239
	keyMPMax     = 240
	keyESMax     = 241
	keyMoveSpeed = 236
	keyEvasion   = 276
	keyArmour    = 235
	keyStr       = 563
	keyInt       = 566
	keyDex       = 569

	keyColdResCap  = 298
	keyFireResCap  = 299
	keyLightResCap = 300
	keyChaosResCap = 301
	keyFireResUC   = 2032
	keyColdResUC   = 2033
	keyLightResUC  = 2034
	keyChaosResUC  = 2035

	keySpirit             = 16094
	keySpiritReservation  = 16114
	keySpiritReservation2 = 16112

	keyItemRarity       = 806
	keyItemQuantity     = 805
	keyCooldownRecovery = 3225

	keyKillCount = 15256

	statsMaxVecBytes = 0x4000
)

type PlayerStats struct {
	HPMax           int          `json:"hp_max,omitempty"`
	MPMax           int          `json:"mp_max,omitempty"`
	ESMax           int          `json:"es_max,omitempty"`
	Evasion         int          `json:"evasion,omitempty"`
	Armour          int          `json:"armour,omitempty"`
	Spirit          int          `json:"spirit,omitempty"`
	SpiritCurrent   int          `json:"spirit_current,omitempty"`
	SpiritReserved  int          `json:"spirit_reserved,omitempty"`
	SpiritReserved2 int          `json:"spirit_reserved2,omitempty"`
	FireCap         int          `json:"fire_cap,omitempty"`
	ColdCap         int          `json:"cold_cap,omitempty"`
	LightCap        int          `json:"light_cap,omitempty"`
	ChaosCap        int          `json:"chaos_cap,omitempty"`
	FireUC          int          `json:"fire_uc,omitempty"`
	ColdUC          int          `json:"cold_uc,omitempty"`
	LightUC         int          `json:"light_uc,omitempty"`
	ChaosUC         int          `json:"chaos_uc,omitempty"`
	Str             int          `json:"str,omitempty"`
	Dex             int          `json:"dex,omitempty"`
	Int             int          `json:"int,omitempty"`
	MoveSpeed       int          `json:"move_speed,omitempty"`
	ItemRarity      int          `json:"item_rarity,omitempty"`
	ItemQuantity    int          `json:"item_quantity,omitempty"`
	CooldownRecov   int          `json:"cooldown_recovery,omitempty"`
	KillCount       int          `json:"kill_count,omitempty"`
	Buffs           []PlayerBuff `json:"buffs,omitempty"`
}

func ReadStatsComponent(r Reader, statsComp uint64) (PlayerStats, bool) {
	var s PlayerStats
	if statsComp < HeapLo || statsComp >= HeapHi {
		return s, false
	}
	gotFireCap, gotColdCap, gotLightCap, gotChaosCap := false, false, false, false
	gotFireUC, gotColdUC, gotLightUC, gotChaosUC := false, false, false, false
	gotStr, gotDex, gotInt := false, false, false
	if !walkStatsVec(r, statsComp, statsItemsPtrOff, func(key uint32, val int32) {
		switch key {
		case keyHPMax:
			s.HPMax = int(val)
		case keyMPMax:
			s.MPMax = int(val)
		case keyESMax:
			s.ESMax = int(val)
		case keyMoveSpeed:
			s.MoveSpeed = int(val) / 100
		case keyEvasion:
			s.Evasion = int(val)
		case keyArmour:
			s.Armour = int(val)
		case keyStr:
			s.Str, gotStr = int(val), true
		case keyDex:
			s.Dex, gotDex = int(val), true
		case keyInt:
			s.Int, gotInt = int(val), true
		case keySpirit:
			s.Spirit = int(val)
		case keySpiritReservation:
			s.SpiritReserved = int(val)
		case keySpiritReservation2:
			s.SpiritReserved2 = int(val)
		case keyFireResCap:
			s.FireCap, gotFireCap = int(val), true
		case keyColdResCap:
			s.ColdCap, gotColdCap = int(val), true
		case keyLightResCap:
			s.LightCap, gotLightCap = int(val), true
		case keyChaosResCap:
			s.ChaosCap, gotChaosCap = int(val), true
		case keyFireResUC:
			s.FireUC, gotFireUC = int(val), true
		case keyColdResUC:
			s.ColdUC, gotColdUC = int(val), true
		case keyLightResUC:
			s.LightUC, gotLightUC = int(val), true
		case keyChaosResUC:
			s.ChaosUC, gotChaosUC = int(val), true
		}
	}) {
		return s, false
	}

	_ = walkStatsVec(r, statsComp, statsBaseResistsPtrOff, func(key uint32, val int32) {
		switch key {
		case keyFireResCap:
			if !gotFireCap {
				s.FireCap = int(val)
			}
		case keyColdResCap:
			if !gotColdCap {
				s.ColdCap = int(val)
			}
		case keyLightResCap:
			if !gotLightCap {
				s.LightCap = int(val)
			}
		case keyChaosResCap:
			if !gotChaosCap {
				s.ChaosCap = int(val)
			}
		case keyFireResUC:
			if !gotFireUC {
				s.FireUC = int(val)
			}
		case keyColdResUC:
			if !gotColdUC {
				s.ColdUC = int(val)
			}
		case keyLightResUC:
			if !gotLightUC {
				s.LightUC = int(val)
			}
		case keyChaosResUC:
			if !gotChaosUC {
				s.ChaosUC = int(val)
			}

		case keyStr:
			if !gotStr {
				s.Str = int(val)
			}
		case keyDex:
			if !gotDex {
				s.Dex = int(val)
			}
		case keyInt:
			if !gotInt {
				s.Int = int(val)
			}

		case keyArmour:
			if s.Armour == 0 {
				s.Armour = int(val)
			}

		case keyItemRarity:
			s.ItemRarity = int(val)
		case keyItemQuantity:
			s.ItemQuantity = int(val)
		case keyCooldownRecovery:
			s.CooldownRecov = int(val)
		case keyKillCount:
			s.KillCount = int(val)
		}
	})
	return s, true
}

func walkStatsVec(r Reader, statsComp, ptrOff uint64, visit func(key uint32, val int32)) bool {
	internal := ReadU64(r, statsComp+ptrOff)
	if internal < HeapLo || internal >= HeapHi {
		return false
	}
	vecBegin := ReadU64(r, internal+statsVecBeginOff)
	vecEnd := ReadU64(r, internal+statsVecEndOff)
	if vecBegin < HeapLo || vecEnd <= vecBegin {
		return false
	}
	size := vecEnd - vecBegin
	if size > statsMaxVecBytes {
		size = statsMaxVecBytes
	}
	data, err := r.ReadBytes(vecBegin, int(size))
	if err != nil || len(data) < 8 {
		return false
	}
	for i := 0; i+8 <= len(data); i += 8 {
		key := binary.LittleEndian.Uint32(data[i : i+4])
		val := int32(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		visit(key, val)
	}
	return true
}

type StatEntry struct {
	Key   uint32
	Value int32
}

func DumpStatsVec(r Reader, statsComp uint64) []StatEntry {
	return dumpVecAt(r, statsComp, statsItemsPtrOff)
}

// Support-gem capacity per attribute colour (RE'd 2026-06-09). USED is stored as
// stats; MAX is floor(attribute / 5) computed from the virtual_*_for_gem_requirements
// attributes. Verified live: str 75 -> 15, dex 22 -> 4, int 243 -> 48.
const (
	keyUsedRedSupports    = 22264 // total_socketed_red_skill_support_gems (str)
	keyUsedGreenSupports  = 22265 // total_socketed_green_skill_support_gems (dex)
	keyUsedBlueSupports   = 22266 // total_socketed_blue_skill_support_gems (int)
	keyVirtStrForGemReqs  = 19774 // virtual_strength_for_gem_requirements
	keyVirtDexForGemReqs  = 19775 // virtual_dexterity_for_gem_requirements
	keyVirtIntForGemReqs  = 19776 // virtual_intelligence_for_gem_requirements
	supportGemAttrDivisor = 5
)

type SupportCapacity struct {
	RedUsed, RedMax     int // strength
	GreenUsed, GreenMax int // dexterity
	BlueUsed, BlueMax   int // intelligence
}

func ReadSupportCapacity(r Reader, statsComp uint64) (SupportCapacity, bool) {
	var sc SupportCapacity
	var str, dex, intl int32
	if !walkStatsVec(r, statsComp, statsItemsPtrOff, func(key uint32, val int32) {
		switch key {
		case keyUsedRedSupports:
			sc.RedUsed = int(val)
		case keyUsedGreenSupports:
			sc.GreenUsed = int(val)
		case keyUsedBlueSupports:
			sc.BlueUsed = int(val)
		case keyVirtStrForGemReqs:
			str = val
		case keyVirtDexForGemReqs:
			dex = val
		case keyVirtIntForGemReqs:
			intl = val
		}
	}) {
		return sc, false
	}
	sc.RedMax = int(str) / supportGemAttrDivisor
	sc.GreenMax = int(dex) / supportGemAttrDivisor
	sc.BlueMax = int(intl) / supportGemAttrDivisor
	return sc, true
}

func DumpStatsVecBuffs(r Reader, statsComp uint64) []StatEntry {
	return dumpVecAt(r, statsComp, statsBuffsPtrOff)
}

func dumpVecAt(r Reader, statsComp uint64, ptrOff uint64) []StatEntry {
	if statsComp < HeapLo || statsComp >= HeapHi {
		return nil
	}
	internal := ReadU64(r, statsComp+ptrOff)
	if internal < HeapLo || internal >= HeapHi {
		return nil
	}
	vecBegin := ReadU64(r, internal+statsVecBeginOff)
	vecEnd := ReadU64(r, internal+statsVecEndOff)
	if vecBegin < HeapLo || vecEnd <= vecBegin {
		return nil
	}
	size := vecEnd - vecBegin
	if size > statsMaxVecBytes {
		size = statsMaxVecBytes
	}
	data, err := r.ReadBytes(vecBegin, int(size))
	if err != nil || len(data) < 8 {
		return nil
	}
	out := make([]StatEntry, 0, len(data)/8)
	for i := 0; i+8 <= len(data); i += 8 {
		key := binary.LittleEndian.Uint32(data[i : i+4])
		val := int32(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		out = append(out, StatEntry{Key: key, Value: val})
	}
	return out
}

func IsPlayerStatsComponent(r Reader, statsComp uint64, wantHPMax int) bool {
	s, ok := ReadStatsComponent(r, statsComp)
	if !ok {
		return false
	}
	return s.HPMax == wantHPMax
}

func ReadPlayerHPMax(r Reader, entity uint64) int {
	comp := ResolveComponentByName(r, entity, "Life")
	if comp == 0 {
		return 0
	}
	return readU32(r, comp+monsterHPMaxOffset)
}
